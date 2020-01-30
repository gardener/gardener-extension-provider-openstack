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

package genericmutator

import (
	"context"

	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	"github.com/gardener/gardener-extensions/pkg/controller/operatingsystemconfig/oscommon/cloudinit"
	"github.com/gardener/gardener-extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener-extensions/pkg/webhook"
	"github.com/gardener/gardener-extensions/pkg/webhook/controlplane"

	"github.com/coreos/go-systemd/unit"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// EnsurerContext wraps the actual context and cluster object.
type EnsurerContext interface {
	GetCluster(ctx context.Context) (*extensionscontroller.Cluster, error)
}

// Ensurer ensures that various standard Kubernets controlplane objects conform to the provider requirements.
// If they don't initially, they are mutated accordingly.
type Ensurer interface {
	// EnsureKubeAPIServerService ensures that the kube-apiserver service conforms to the provider requirements.
	EnsureKubeAPIServerService(context.Context, EnsurerContext, *corev1.Service) error
	// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
	EnsureKubeAPIServerDeployment(context.Context, EnsurerContext, *appsv1.Deployment) error
	// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
	EnsureKubeControllerManagerDeployment(context.Context, EnsurerContext, *appsv1.Deployment) error
	// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
	EnsureKubeSchedulerDeployment(context.Context, EnsurerContext, *appsv1.Deployment) error
	// EnsureETCDStatefulSet ensures that the etcd stateful sets conform to the provider requirements.
	EnsureETCDStatefulSet(context.Context, EnsurerContext, *appsv1.StatefulSet) error
	// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
	EnsureKubeletServiceUnitOptions(context.Context, EnsurerContext, []*unit.UnitOption) ([]*unit.UnitOption, error)
	// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
	EnsureKubeletConfiguration(context.Context, EnsurerContext, *kubeletconfigv1beta1.KubeletConfiguration) error
	// EnsureKubernetesGeneralConfiguration ensures that the kubernetes general configuration conforms to the provider requirements.
	EnsureKubernetesGeneralConfiguration(context.Context, EnsurerContext, *string) error
	// ShouldProvisionKubeletCloudProviderConfig returns true if the cloud provider config file should be added to the kubelet configuration.
	ShouldProvisionKubeletCloudProviderConfig() bool
	// EnsureKubeletCloudProviderConfig ensures that the cloud provider config file content conforms to the provider requirements.
	EnsureKubeletCloudProviderConfig(context.Context, EnsurerContext, *string, string) error
	// EnsureAdditionalUnits ensures additional systemd units
	EnsureAdditionalUnits(context.Context, EnsurerContext, *[]extensionsv1alpha1.Unit) error
	// EnsureAdditionalFile ensures additional systemd files
	EnsureAdditionalFiles(context.Context, EnsurerContext, *[]extensionsv1alpha1.File) error
}

// NewMutator creates a new controlplane mutator.
func NewMutator(
	ensurer Ensurer,
	unitSerializer controlplane.UnitSerializer,
	kubeletConfigCodec controlplane.KubeletConfigCodec,
	fciCodec controlplane.FileContentInlineCodec,
	logger logr.Logger,
) extensionswebhook.Mutator {
	return &mutator{
		ensurer:            ensurer,
		unitSerializer:     unitSerializer,
		kubeletConfigCodec: kubeletConfigCodec,
		fciCodec:           fciCodec,
		logger:             logger.WithName("mutator"),
	}
}

type mutator struct {
	client             client.Client
	ensurer            Ensurer
	unitSerializer     controlplane.UnitSerializer
	kubeletConfigCodec controlplane.KubeletConfigCodec
	fciCodec           controlplane.FileContentInlineCodec
	logger             logr.Logger
}

// InjectClient injects the given client into the ensurer.
// TODO Replace this with the more generic InjectFunc when controller runtime supports it
func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	if _, err := inject.ClientInto(client, m.ensurer); err != nil {
		return errors.Wrap(err, "could not inject the client into the ensurer")
	}
	return nil
}

type ensurerContext struct {
	client  client.Client
	object  metav1.Object
	cluster *extensionscontroller.Cluster
}

// NewEnsurerContext creates an ensurer context object.
func NewEnsurerContext(client client.Client, object metav1.Object) EnsurerContext {
	return &ensurerContext{
		client: client,
		object: object,
	}
}

// NewInternalEnsurerContext creates an ensurer context object.
func NewInternalEnsurerContext(cluster *extensionscontroller.Cluster) EnsurerContext {
	return &ensurerContext{
		cluster: cluster,
	}
}

// GetCluster returns the cluster object.
func (c *ensurerContext) GetCluster(ctx context.Context) (*extensionscontroller.Cluster, error) {
	if c.cluster == nil {
		cluster, err := extensionscontroller.GetCluster(ctx, c.client, c.object.GetNamespace())
		if err != nil {
			return nil, errors.Wrapf(err, "could not get cluster for namespace '%s'", c.object.GetNamespace())
		}
		c.cluster = cluster
	}
	return c.cluster, nil
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(ctx context.Context, obj runtime.Object) error {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return errors.Wrapf(err, "could not create accessor during webhook")
	}
	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if acc.GetDeletionTimestamp() != nil {
		return nil
	}
	o, ok := obj.(metav1.Object)
	if !ok {
		return errors.Wrapf(err, "could not cast runtime object to metav1 object")
	}
	ectx := NewEnsurerContext(m.client, o)

	switch x := obj.(type) {
	case *corev1.Service:
		switch x.Name {
		case v1beta1constants.DeploymentNameKubeAPIServer:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeAPIServerService(ctx, ectx, x)
		}
	case *appsv1.Deployment:
		switch x.Name {
		case v1beta1constants.DeploymentNameKubeAPIServer:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeAPIServerDeployment(ctx, ectx, x)
		case v1beta1constants.DeploymentNameKubeControllerManager:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeControllerManagerDeployment(ctx, ectx, x)
		case v1beta1constants.DeploymentNameKubeScheduler:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeSchedulerDeployment(ctx, ectx, x)
		}
	case *appsv1.StatefulSet:
		switch x.Name {
		case v1beta1constants.ETCDMain, v1beta1constants.ETCDEvents:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureETCDStatefulSet(ctx, ectx, x)
		}
	case *extensionsv1alpha1.OperatingSystemConfig:
		if x.Spec.Purpose == extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.mutateOperatingSystemConfig(ctx, ectx, x)
		}
		return nil
	}
	return nil
}

func (m *mutator) mutateOperatingSystemConfig(ctx context.Context, ectx EnsurerContext, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	// Mutate kubelet.service unit, if present
	if u := extensionswebhook.UnitWithName(osc.Spec.Units, v1beta1constants.OperatingSystemConfigUnitNameKubeletService); u != nil && u.Content != nil {
		if err := m.ensureKubeletServiceUnitContent(ctx, ectx, u.Content); err != nil {
			return err
		}
	}

	// Mutate kubelet configuration file, if present
	if f := extensionswebhook.FileWithPath(osc.Spec.Files, v1beta1constants.OperatingSystemConfigFilePathKubeletConfig); f != nil && f.Content.Inline != nil {
		if err := m.ensureKubeletConfigFileContent(ctx, ectx, f.Content.Inline); err != nil {
			return err
		}
	}

	// Mutate 99 kubernetes general configuration file, if present
	if f := extensionswebhook.FileWithPath(osc.Spec.Files, v1beta1constants.OperatingSystemConfigFilePathKernelSettings); f != nil && f.Content.Inline != nil {
		if err := m.ensureKubernetesGeneralConfiguration(ctx, ectx, f.Content.Inline); err != nil {
			return err
		}
	}

	// Check if cloud provider config needs to be ensured
	if m.ensurer.ShouldProvisionKubeletCloudProviderConfig() {
		if err := m.ensureKubeletCloudProviderConfig(ctx, ectx, osc); err != nil {
			return err
		}
	}

	if err := m.ensurer.EnsureAdditionalFiles(ctx, ectx, &osc.Spec.Files); err != nil {
		return err
	}

	if err := m.ensurer.EnsureAdditionalUnits(ctx, ectx, &osc.Spec.Units); err != nil {
		return err
	}

	return nil
}

func (m *mutator) ensureKubeletServiceUnitContent(ctx context.Context, ectx EnsurerContext, content *string) error {
	var opts []*unit.UnitOption
	var err error

	// Deserialize unit options
	if opts, err = m.unitSerializer.Deserialize(*content); err != nil {
		return errors.Wrap(err, "could not deserialize kubelet.service unit content")
	}

	if opts, err = m.ensurer.EnsureKubeletServiceUnitOptions(ctx, ectx, opts); err != nil {
		return err
	}

	// Serialize unit options
	if *content, err = m.unitSerializer.Serialize(opts); err != nil {
		return errors.Wrap(err, "could not serialize kubelet.service unit options")
	}

	return nil
}

func (m *mutator) ensureKubeletConfigFileContent(ctx context.Context, ectx EnsurerContext, fci *extensionsv1alpha1.FileContentInline) error {
	var kubeletConfig *kubeletconfigv1beta1.KubeletConfiguration
	var err error

	// Decode kubelet configuration from inline content
	if kubeletConfig, err = m.kubeletConfigCodec.Decode(fci); err != nil {
		return errors.Wrap(err, "could not decode kubelet configuration")
	}

	if err = m.ensurer.EnsureKubeletConfiguration(ctx, ectx, kubeletConfig); err != nil {
		return err
	}

	// Encode kubelet configuration into inline content
	var newFCI *extensionsv1alpha1.FileContentInline
	if newFCI, err = m.kubeletConfigCodec.Encode(kubeletConfig, fci.Encoding); err != nil {
		return errors.Wrap(err, "could not encode kubelet configuration")
	}
	*fci = *newFCI

	return nil
}

func (m *mutator) ensureKubernetesGeneralConfiguration(ctx context.Context, ectx EnsurerContext, fci *extensionsv1alpha1.FileContentInline) error {
	var data []byte
	var err error

	// Decode kubernetes general configuration from inline content
	if data, err = m.fciCodec.Decode(fci); err != nil {
		return errors.Wrap(err, "could not decode kubernetes general configuration")
	}

	s := string(data)
	if err = m.ensurer.EnsureKubernetesGeneralConfiguration(ctx, ectx, &s); err != nil {
		return err
	}

	// Encode kubernetes general configuration into inline content
	var newFCI *extensionsv1alpha1.FileContentInline
	if newFCI, err = m.fciCodec.Encode([]byte(s), fci.Encoding); err != nil {
		return errors.Wrap(err, "could not encode kubernetes general configuration")
	}
	*fci = *newFCI

	return nil
}

const CloudProviderConfigPath = "/var/lib/kubelet/cloudprovider.conf"

func (m *mutator) ensureKubeletCloudProviderConfig(ctx context.Context, ectx EnsurerContext, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	var err error

	// Ensure kubelet cloud provider config
	var s string
	if err = m.ensurer.EnsureKubeletCloudProviderConfig(ctx, ectx, &s, osc.Namespace); err != nil {
		return err
	}

	// Encode cloud provider config into inline content
	var fci *extensionsv1alpha1.FileContentInline
	if fci, err = m.fciCodec.Encode([]byte(s), string(cloudinit.B64FileCodecID)); err != nil {
		return errors.Wrap(err, "could not encode kubelet cloud provider config")
	}

	// Ensure the cloud provider config file is part of the OperatingSystemConfig
	osc.Spec.Files = extensionswebhook.EnsureFileWithPath(osc.Spec.Files, extensionsv1alpha1.File{
		Path:        CloudProviderConfigPath,
		Permissions: util.Int32Ptr(0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: fci,
		},
	})
	return nil
}
