// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"context"
	"fmt"
	"strings"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	workerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// GardenPurposeMachineClass is a constant for the 'machineclass' value in a label.
const GardenPurposeMachineClass = "machineclass"

type genericActuator struct {
	logger logr.Logger

	delegateFactory DelegateFactory
	mcmName         string
	mcmSeedChart    chart.Interface
	mcmShootChart   chart.Interface
	imageVector     imagevector.ImageVector

	client               client.Client
	clientset            kubernetes.Interface
	reader               client.Reader
	scheme               *runtime.Scheme
	decoder              runtime.Decoder
	gardenerClientset    gardenerkubernetes.Interface
	chartApplier         gardenerkubernetes.ChartApplier
	chartRendererFactory extensionscontroller.ChartRendererFactory
}

// NewActuator creates a new Actuator that reconciles
// Worker resources of Gardener's `extensions.gardener.cloud` API group.
// It provides a default implementation that allows easier integration of providers.
func NewActuator(logger logr.Logger, delegateFactory DelegateFactory, mcmName string, mcmSeedChart, mcmShootChart chart.Interface, imageVector imagevector.ImageVector, chartRendererFactory extensionscontroller.ChartRendererFactory) worker.Actuator {
	return &genericActuator{
		logger: logger.WithName("worker-actuator"),

		delegateFactory:      delegateFactory,
		mcmName:              mcmName,
		mcmSeedChart:         mcmSeedChart,
		mcmShootChart:        mcmShootChart,
		imageVector:          imageVector,
		chartRendererFactory: chartRendererFactory,
	}
}

func (a *genericActuator) InjectFunc(f inject.Func) error {
	return f(a.delegateFactory)
}

func (a *genericActuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

func (a *genericActuator) InjectAPIReader(reader client.Reader) error {
	a.reader = reader
	return nil
}

func (a *genericActuator) InjectScheme(scheme *runtime.Scheme) error {
	a.scheme = scheme
	a.decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
	return nil
}

func (a *genericActuator) InjectConfig(config *rest.Config) error {
	var err error

	a.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "could not create Kubernetes client")
	}

	a.gardenerClientset, err = gardenerkubernetes.NewWithConfig(gardenerkubernetes.WithRESTConfig(config))
	if err != nil {
		return errors.Wrap(err, "could not create Gardener client")
	}

	a.chartApplier = a.gardenerClientset.ChartApplier()

	return nil
}

func (a *genericActuator) cleanupMachineDeployments(ctx context.Context, logger logr.Logger, existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Cleaning up machine deployments")
	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if !wantedMachineDeployments.HasDeployment(existingMachineDeployment.Name) {
			if err := a.client.Delete(ctx, &existingMachineDeployment); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *genericActuator) listMachineClassNames(ctx context.Context, namespace string, machineClassList client.ObjectList) (sets.String, error) {
	if err := a.client.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	classNames := sets.NewString()

	if err := meta.EachListItem(machineClassList, func(machineClass runtime.Object) error {
		accessor, err := meta.Accessor(machineClass)
		if err != nil {
			return err
		}

		classNames.Insert(accessor.GetName())
		return nil
	}); err != nil {
		return nil, err
	}

	return classNames, nil
}

func (a *genericActuator) cleanupMachineClasses(ctx context.Context, logger logr.Logger, namespace string, machineClassList client.ObjectList, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Cleaning up machine classes")
	if err := a.client.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
		return err
	}

	return meta.EachListItem(machineClassList, func(obj runtime.Object) error {
		machineClass := obj.(client.Object)
		if !wantedMachineDeployments.HasClass(machineClass.GetName()) {
			logger.Info("Deleting machine class", "machineClass", machineClass)
			if err := a.client.Delete(ctx, machineClass); err != nil {
				return err
			}
		}

		return nil
	})
}

func getMachineClassSecretLabels() map[string]string {
	return map[string]string{v1beta1constants.GardenerPurpose: GardenPurposeMachineClass}
}

func (a *genericActuator) listMachineClassSecrets(ctx context.Context, namespace string) (*corev1.SecretList, error) {
	secretList := &corev1.SecretList{}
	if err := a.client.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels(getMachineClassSecretLabels())); err != nil {
		return nil, err
	}

	return secretList, nil
}

// cleanupMachineClassSecrets deletes all unused machine class secrets (i.e., those which are not part
// of the provided list <usedSecrets>.
func (a *genericActuator) cleanupMachineClassSecrets(ctx context.Context, logger logr.Logger, namespace string, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Cleaning up machine class secrets")
	secretList, err := a.listMachineClassSecrets(ctx, namespace)
	if err != nil {
		return err
	}

	// Cleanup all secrets which were used for machine classes that do not exist anymore.
	for _, secret := range secretList.Items {
		if !wantedMachineDeployments.HasSecret(secret.Name) {
			if err := a.client.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

// updateCloudCredentialsInAllMachineClassSecrets updates the cloud credentials
// for all existing machine class secrets.
func (a *genericActuator) updateCloudCredentialsInAllMachineClassSecrets(ctx context.Context, logger logr.Logger, cloudCredentials map[string][]byte, namespace string) error {
	logger.Info("Updating cloud credentials for existing machine class secrets")
	secretList, err := a.listMachineClassSecrets(ctx, namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to list machine class secrets in namespace %s", namespace)
	}

	for _, secret := range secretList.Items {
		secretCopy := secret.DeepCopy()
		for key, value := range cloudCredentials {
			secretCopy.Data[key] = value
		}
		if err := a.client.Patch(ctx, secretCopy, client.MergeFrom(&secret)); err != nil {
			return errors.Wrapf(err, "failed to patch secret %s/%s with cloud credentials", namespace, secret.Name)
		}
	}
	return nil
}

// shallowDeleteMachineClassSecrets deletes all unused machine class secrets (i.e., those which are not part
// of the provided list <usedSecrets>) without waiting for MCM to do this.
func (a *genericActuator) shallowDeleteMachineClassSecrets(ctx context.Context, logger logr.Logger, namespace string, wantedMachineDeployments worker.MachineDeployments) error {
	logger.Info("Shallow deleting machine class secrets")
	secretList, err := a.listMachineClassSecrets(ctx, namespace)
	if err != nil {
		return err
	}
	// Delete the finalizers to all secrets which were used for machine classes that do not exist anymore.
	for _, secret := range secretList.Items {
		if !wantedMachineDeployments.HasSecret(secret.Name) {
			if err := extensionscontroller.DeleteAllFinalizers(ctx, a.client, &secret); err != nil {
				return errors.Wrapf(err, "Error removing finalizer from MachineClassSecret: %s/%s", secret.Namespace, secret.Name)
			}
			if err := a.client.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanupMachineSets deletes MachineSets having number of desired and actual replicas equaling 0
func (a *genericActuator) cleanupMachineSets(ctx context.Context, logger logr.Logger, namespace string) error {
	logger.Info("Cleaning up machine sets")
	machineSetList := &machinev1alpha1.MachineSetList{}
	if err := a.client.List(ctx, machineSetList, client.InNamespace(namespace)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, machineSet := range machineSetList.Items {
		if machineSet.Spec.Replicas == 0 && machineSet.Status.Replicas == 0 {
			logger.Info("Deleting MachineSet as the number of desired and actual replicas is 0", "machineSet", &machineSet)
			if err := a.client.Delete(ctx, machineSet.DeepCopy()); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil
}

func (a *genericActuator) shallowDeleteAllObjects(ctx context.Context, logger logr.Logger, namespace string, objectList client.ObjectList) error {
	var objectKind interface{} = strings.TrimSuffix(fmt.Sprintf("%T", objectList), "List")
	if gvk, err := apiutil.GVKForObject(objectList, a.scheme); err == nil {
		objectKind = gvk
	}
	logger.Info("Shallow deleting all objects of kind", "kind", objectKind)

	if err := a.client.List(ctx, objectList, client.InNamespace(namespace)); err != nil {
		return err
	}

	return meta.EachListItem(objectList, func(obj runtime.Object) error {
		object := obj.(client.Object)
		if err := extensionscontroller.DeleteAllFinalizers(ctx, a.client, object); err != nil {
			return err
		}
		if err := a.client.Delete(ctx, object); client.IgnoreNotFound(err) != nil {
			return err
		}
		return nil
	})
}

// IsMachineControllerStuck determines if the machine controller pod is stuck.
func (a *genericActuator) IsMachineControllerStuck(ctx context.Context, worker *extensionsv1alpha1.Worker) (bool, *string, error) {
	machineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := a.client.List(ctx, machineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return false, nil, err
	}

	machineSets := &machinev1alpha1.MachineSetList{}
	if err := a.client.List(ctx, machineSets, client.InNamespace(worker.Namespace)); err != nil {
		return false, nil, err
	}

	isStuck, msg := isMachineControllerStuck(machineSets.Items, machineDeployments.Items)
	return isStuck, msg, nil
}

const (
	// stuckMCMThreshold defines that the machine deployment set has to be created more than
	// this duration ago to be considered for the check whether a machine controller manager pod is stuck
	stuckMCMThreshold = 2 * time.Minute
	// mcmFinalizer is the finalizer used by the machine controller manager
	// not imported from the MCM to reduce dependencies
	mcmFinalizer = "machine.sapcloud.io/machine-controller-manager"
)

// isMachineControllerStuck determines if the machine controller pod is stuck.
// A pod is assumed to be stuck if
//  - a machine deployment exists that does not have a machine set with the correct machine class
//  - the machine set does not have a status that indicates (attempted) machine creation
func isMachineControllerStuck(machineSets []machinev1alpha1.MachineSet, machineDeployments []machinev1alpha1.MachineDeployment) (bool, *string) {
	// map the owner reference to the existing machine sets
	ownerReferenceToMachineSet := workerhelper.BuildOwnerToMachineSetsMap(machineSets)

	for _, machineDeployment := range machineDeployments {
		if !controllerutils.HasFinalizer(&machineDeployment, mcmFinalizer) {
			continue
		}

		// do not consider machine deployments that have just recently been created
		if time.Now().UTC().Sub(machineDeployment.ObjectMeta.CreationTimestamp.UTC()) < stuckMCMThreshold {
			continue
		}

		machineSet := workerhelper.GetMachineSetWithMachineClass(machineDeployment.Name, machineDeployment.Spec.Template.Spec.Class.Name, ownerReferenceToMachineSet)
		if machineSet == nil {
			msg := fmt.Sprintf("missing machine set for machine deployment (%s/%s)", machineDeployment.Namespace, machineDeployment.Name)
			return true, &msg
		}
	}
	return false, nil
}
