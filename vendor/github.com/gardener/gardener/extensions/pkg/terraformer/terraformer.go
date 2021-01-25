// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package terraformer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
)

const (
	// LabelKeyName is a key for label on a Terraformer Pod indicating the Terraformer name.
	LabelKeyName = "terraformer.gardener.cloud/name"
	// LabelKeyPurpose is a key for label on a Terraformer Pod indicating the Terraformer purpose.
	LabelKeyPurpose = "terraformer.gardener.cloud/purpose"
)

type factory struct{}

func (factory) NewForConfig(logger logr.Logger, config *rest.Config, purpose, namespace, name, image string) (Terraformer, error) {
	return NewForConfig(logger, config, purpose, namespace, name, image)
}

func (f factory) New(logger logr.Logger, client client.Client, coreV1Client corev1client.CoreV1Interface, purpose, namespace, name, image string) Terraformer {
	return New(logger, client, coreV1Client, purpose, namespace, name, image)
}

func (f factory) DefaultInitializer(c client.Client, main, variables string, tfVars []byte, stateInitializer StateConfigMapInitializer) Initializer {
	return DefaultInitializer(c, main, variables, tfVars, stateInitializer)
}

// DefaultFactory returns the default factory.
func DefaultFactory() Factory {
	return factory{}
}

// NewForConfig creates a new Terraformer and its dependencies from the given configuration.
func NewForConfig(
	logger logr.Logger,
	config *rest.Config,
	purpose,
	namespace,
	name,
	image string,
) (
	Terraformer,
	error,
) {
	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	coreV1Client, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return New(logger, c, coreV1Client, purpose, namespace, name, image), nil
}

// New takes a <logger>, a <k8sClient>, a string <purpose>, which describes for what the
// Terraformer is used, a <name>, a <namespace> in which the Terraformer will run, and the
// <image> name for the to-be-used Docker image. It returns a Terraformer interface with initialized
// values for the namespace and the names which will be used for all the stored resources like
// ConfigMaps/Secrets.
func New(
	logger logr.Logger,
	c client.Client,
	coreV1Client corev1client.CoreV1Interface,
	purpose,
	namespace,
	name,
	image string,
) Terraformer {
	var prefix = fmt.Sprintf("%s.%s", name, purpose)

	return &terraformer{
		logger:       logger.WithName("terraformer"),
		client:       c,
		coreV1Client: coreV1Client,

		name:      name,
		namespace: namespace,
		purpose:   purpose,
		image:     image,

		configName:    prefix + ConfigSuffix,
		variablesName: prefix + VariablesSuffix,
		stateName:     prefix + StateSuffix,

		logLevel:                      "info",
		terminationGracePeriodSeconds: int64(3600),

		deadlineCleaning: 20 * time.Minute,
		deadlinePod:      20 * time.Minute,
	}
}

// Apply executes a Terraform Pod by running the 'terraform apply' command.
func (t *terraformer) Apply(ctx context.Context) error {
	if !t.configurationDefined {
		return errors.New("terraformer configuration has not been defined, cannot execute Terraformer")
	}
	return t.execute(ctx, "apply")
}

// Destroy executes a Terraform Pod by running the 'terraform destroy' command.
func (t *terraformer) Destroy(ctx context.Context) error {
	if err := t.execute(ctx, "destroy"); err != nil {
		return err
	}
	return t.CleanupConfiguration(ctx)
}

// execute creates a Terraform Pod which runs the provided scriptName (apply or destroy), waits for the Pod to be completed
// (either successful or not), prints its logs, deletes it and returns whether it was successful or not.
func (t *terraformer) execute(ctx context.Context, command string) error {
	logger := t.logger.WithValues("command", command)

	var allConfigurationResourcesExist = false
	logger.V(1).Info("Waiting until all configuration resources exist")

	// We should retry the preparation check in order to allow the kube-apiserver to actually create the ConfigMaps.
	if err := retry.UntilTimeout(ctx, 5*time.Second, 30*time.Second, func(ctx context.Context) (done bool, err error) {
		numberOfExistingResources, err := t.NumberOfResources(ctx)
		if err != nil {
			return retry.SevereError(err)
		}
		if numberOfExistingResources == 0 {
			logger.Info("All ConfigMaps and Secrets missing, can not execute Terraformer pod")
			return retry.Ok()
		} else if numberOfExistingResources == numberOfConfigResources {
			logger.Info("All ConfigMaps and Secrets exist, will execute Terraformer pod")
			allConfigurationResourcesExist = true
			return retry.Ok()
		} else {
			logger.Error(fmt.Errorf("ConfigMaps or Secrets are missing"), "Cannot execute Terraformer pod")
			return retry.MinorError(fmt.Errorf("%d/%d Terraform resources are missing", numberOfConfigResources-numberOfExistingResources, numberOfConfigResources))
		}
	}); err != nil {
		return err
	}
	if !allConfigurationResourcesExist {
		return nil
	}

	// Check if an existing Terraformer pod is still running. If yes, then adopt it. If no, then deploy a new pod (if
	// necessary).
	var (
		pod          *corev1.Pod
		deployNewPod = true
	)

	podList, err := t.listPods(ctx)
	if err != nil {
		return err
	}

	switch {
	case len(podList.Items) == 1:
		if cmd := getTerraformerCommand(&podList.Items[0]); cmd == command {
			// adopt still existing pod
			pod = &podList.Items[0]
			deployNewPod = false
		} else {
			// delete still existing pod and wait until it's gone
			logger.Info(fmt.Sprintf("still existing Terraform pod with command %q found - ensuring the cleanup before starting a new Terraform pod with command %q", cmd, command))
			if err := t.EnsureCleanedUp(ctx); err != nil {
				return err
			}
		}

	case len(podList.Items) > 1:
		// unreachable
		logger.Error(fmt.Errorf("too many Terraformer pods"), "unexpected number of still existing Terraformer pods: %d", len(podList.Items))
		if err := t.EnsureCleanedUp(ctx); err != nil {
			return err
		}
	}

	// In case of command == 'destroy', we need to first check whether the Terraform state contains
	// something at all. If it does not contain anything, then the 'apply' could never be executed, probably
	// because of syntax errors. In this case, we want to skip the Terraform destroy pod (as it wouldn't do anything
	// anyway) and just delete the related ConfigMaps/Secrets.
	if command == "destroy" {
		deployNewPod = !t.IsStateEmpty(ctx)
	}

	if deployNewPod {
		// Create Terraform Pod which executes the provided command
		generateName := t.computePodGenerateName(command)

		logger.Info("Deploying Terraformer pod", "generateName", generateName)
		pod, err = t.deployTerraformerPod(ctx, generateName, command)
		if err != nil {
			return fmt.Errorf("failed to deploy the Terraformer pod with .meta.generateName %q: %w", generateName, err)
		}
		logger.Info("Successfully created Terraformer pod", "pod", kutil.KeyFromObject(pod))
	}

	if pod != nil {
		// TODO: remove after several releases
		// ensure ownerRef for already existing state configmaps
		if err := t.ensureStateHasOwnerRef(ctx); err != nil {
			return fmt.Errorf("failed to ensure owner reference for the state configmap: %w", err)
		}

		// Wait for the Terraform apply/destroy Pod to be completed
		exitCode := t.waitForPod(ctx, logger, pod, t.deadlinePod)
		succeeded := exitCode == 0
		if succeeded {
			logger.Info("Terraformer pod finished successfully")
		} else {
			logger.Info("Terraformer pod finished with error", "exitCode", exitCode)
		}

		// Retrieve the logs of the pod
		logger.V(1).Info("Fetching the logs for Terraformer pod")
		logs, err := t.retrievePodLogs(ctx, logger, pod)
		if err != nil {
			logger.Error(err, "Could not retrieve the logs of the Terraformer pod")
			return err
		}
		logger.V(1).Info("Logs of Terraformer pod: "+logs, "pod", client.ObjectKey{Namespace: t.namespace, Name: pod.Name})

		// Delete the Terraformer Pod
		logger.Info("Cleaning up Terraformer pod")
		if err := t.client.Delete(ctx, pod); client.IgnoreNotFound(err) != nil {
			return err
		}

		// Evaluate whether the execution was successful or not
		logger.Info("Terraformer execution has been completed")
		if !succeeded {
			errorMessage := fmt.Sprintf("Terraform execution for command '%s' could not be completed.", command)
			if terraformErrors := retrieveTerraformErrors(pod.Name, logs); terraformErrors != nil {
				errorMessage += fmt.Sprintf(" The following issues have been found in the logs:\n\n%s", strings.Join(terraformErrors, "\n\n"))
			}
			return gardencorev1beta1helper.DetermineError(errors.New(errorMessage), errorMessage)
		}
	}

	return nil
}

const (
	name     = "terraformer"
	rbacName = "gardener.cloud:system:terraformer"
)

func (t *terraformer) createOrUpdateServiceAccount(ctx context.Context) error {
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, t.client, serviceAccount, func() error {
		return nil
	})
	return err
}

func (t *terraformer) createOrUpdateRole(ctx context.Context) error {
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: rbacName}}
	_, err := controllerutil.CreateOrUpdate(ctx, t.client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"configmaps", "secrets"},
			Verbs:     []string{"*"},
		}}
		return nil
	})
	return err
}

func (t *terraformer) createOrUpdateRoleBinding(ctx context.Context) error {
	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: t.namespace, Name: rbacName}}
	_, err := controllerutil.CreateOrUpdate(ctx, t.client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     rbacName,
		}
		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: t.namespace,
		}}
		return nil
	})
	return err
}

func (t *terraformer) createOrUpdateTerraformerAuth(ctx context.Context) error {
	if err := t.createOrUpdateServiceAccount(ctx); err != nil {
		return err
	}
	if err := t.createOrUpdateRole(ctx); err != nil {
		return err
	}
	return t.createOrUpdateRoleBinding(ctx)
}

func (t *terraformer) ensureStateHasOwnerRef(ctx context.Context) error {
	if t.ownerRef == nil {
		return nil
	}

	configMap := &corev1.ConfigMap{}
	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.stateName), configMap); err != nil {
		return err
	}

	oldConfigMap := configMap.DeepCopy()
	configMap.SetOwnerReferences(kutil.MergeOwnerReferences(configMap.OwnerReferences, *t.ownerRef))

	return t.client.Patch(ctx, configMap, client.MergeFromWithOptions(
		oldConfigMap,
		client.MergeFromWithOptimisticLock{},
	))
}

func (t *terraformer) deployTerraformerPod(ctx context.Context, generateName, command string) (*corev1.Pod, error) {
	if err := t.createOrUpdateTerraformerAuth(ctx); err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    t.namespace,
			Labels: map[string]string{
				// Terraformer labels
				LabelKeyName:    t.name,
				LabelKeyPurpose: t.purpose,
				// Network policy labels
				v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToSeedAPIServer:   v1beta1constants.LabelNetworkPolicyAllowed,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "terraform",
				Image:           t.image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         t.computeTerraformerCommand(command),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1.5Gi"),
					},
				},
				Env: t.env(),
			}},
			RestartPolicy:                 corev1.RestartPolicyNever,
			ServiceAccountName:            name,
			TerminationGracePeriodSeconds: pointer.Int64Ptr(t.terminationGracePeriodSeconds),
		},
	}

	err := t.client.Create(ctx, pod)
	return pod, err
}

func (t *terraformer) computeTerraformerCommand(command string) []string {
	if t.useV2 {
		return []string{
			"/terraformer",
			command,
			"--zap-log-level=" + t.logLevel,
			"--configuration-configmap-name=" + t.configName,
			"--state-configmap-name=" + t.stateName,
			"--variables-secret-name=" + t.variablesName,
		}
	}

	return []string{
		"/terraformer.sh",
		command,
	}
}

func getTerraformerCommand(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	if len(pod.Spec.Containers) != 1 {
		return ""
	}
	if len(pod.Spec.Containers[0].Command) < 2 {
		return ""
	}
	return pod.Spec.Containers[0].Command[1]
}

func (t *terraformer) env() []corev1.EnvVar {
	var envVars []corev1.EnvVar

	if !t.useV2 {
		envVars = append(envVars, []corev1.EnvVar{
			{Name: "MAX_BACKOFF_SEC", Value: "60"},
			{Name: "MAX_TIME_SEC", Value: "1800"},
			{Name: "TF_CONFIGURATION_CONFIG_MAP_NAME", Value: t.configName},
			{Name: "TF_STATE_CONFIG_MAP_NAME", Value: t.stateName},
			{Name: "TF_VARIABLES_SECRET_NAME", Value: t.variablesName},
		}...)
	}

	return append(envVars, t.envVars...)
}

// listTerraformerPods lists all pods in the Terraformer namespace which have labels 'terraformer.gardener.cloud/name'
// and 'terraformer.gardener.cloud/purpose' matching the current Terraformer name and purpose.
func (t *terraformer) listPods(ctx context.Context) (*corev1.PodList, error) {
	var (
		podList = &corev1.PodList{}
		labels  = map[string]string{LabelKeyName: t.name, LabelKeyPurpose: t.purpose}
	)

	if err := t.client.List(ctx, podList, client.InNamespace(t.namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return podList, nil
}

// retrievePodLogs fetches the logs of the created Pods by the Terraformer and returns them as a map whose
// keys are pod names and whose values are the corresponding logs.
func (t *terraformer) retrievePodLogs(ctx context.Context, logger logr.Logger, pod *corev1.Pod) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logs, err := kubernetes.GetPodLogs(ctx, t.coreV1Client.Pods(pod.Namespace), pod.Name, &corev1.PodLogOptions{})
	if err != nil {
		logger.Error(err, "Could not retrieve the logs of Terraformer pod", "pod", kutil.KeyFromObject(pod))
		return "", err
	}

	return string(logs), nil
}

func (t *terraformer) deleteTerraformerPods(ctx context.Context, podList *corev1.PodList) error {
	for _, pod := range podList.Items {
		if err := t.client.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (t *terraformer) computePodGenerateName(command string) string {
	return fmt.Sprintf("%s.%s.tf-%s-", t.name, t.purpose, command)
}
