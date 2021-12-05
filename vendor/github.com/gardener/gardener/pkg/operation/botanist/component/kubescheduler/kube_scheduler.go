// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubescheduler

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ServiceName is the name of the service of the kube-scheduler.
	ServiceName = "kube-scheduler"
	// SecretName is a constant for the secret name for the kube-scheduler's kubeconfig secret.
	SecretName = "kube-scheduler"
	// SecretNameServer is the name of the kube-scheduler server certificate secret.
	SecretNameServer = "kube-scheduler-server"

	// LabelRole is a constant for the value of a label with key 'role'.
	LabelRole = "scheduler"

	managedResourceName    = "shoot-core-kube-scheduler"
	containerName          = v1beta1constants.DeploymentNameKubeScheduler
	portNameMetrics        = "metrics"
	dataKeyComponentConfig = "config.yaml"

	volumeNameConfig          = "kube-scheduler-config"
	volumeMountPathKubeconfig = "/var/lib/kube-scheduler"
	volumeMountPathServer     = "/var/lib/kube-scheduler-server"
	volumeMountPathConfig     = "/var/lib/kube-scheduler-config"

	componentConfigTmpl = `apiVersion: {{ .apiVersion }}
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: ` + volumeMountPathKubeconfig + "/" + secrets.DataKeyKubeconfig + `
leaderElection:
  leaderElect: true`
)

// Interface contains functions for a kube-scheduler deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
}

// New creates a new instance of DeployWaiter for the kube-scheduler.
func New(
	client client.Client,
	namespace string,
	version *semver.Version,
	image string,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) Interface {
	return &kubeScheduler{
		client:    client,
		namespace: namespace,
		version:   version,
		image:     image,
		replicas:  replicas,
		config:    config,
	}
}

type kubeScheduler struct {
	client    client.Client
	namespace string
	version   *semver.Version
	image     string
	replicas  int32
	config    *gardencorev1beta1.KubeSchedulerConfig

	secrets Secrets
}

func (k *kubeScheduler) Deploy(ctx context.Context) error {
	if k.secrets.Kubeconfig.Name == "" || k.secrets.Kubeconfig.Checksum == "" {
		return fmt.Errorf("missing kubeconfig secret information")
	}
	if k.secrets.Server.Name == "" || k.secrets.Server.Checksum == "" {
		return fmt.Errorf("missing server secret information")
	}

	componentConfigYAML, err := k.computeComponentConfig()
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler-config",
			Namespace: k.namespace,
		},
		Data: map[string]string{dataKeyComponentConfig: componentConfigYAML},
	}
	utilruntime.Must(kutil.MakeUnique(configMap))

	var (
		vpa        = k.emptyVPA()
		service    = k.emptyService()
		deployment = k.emptyDeployment()

		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto

		port           int32 = 10259
		probeURIScheme       = corev1.URISchemeHTTPS
		env                  = k.computeEnvironmentVariables()
		command              = k.computeCommand(port)
	)

	if err := k.client.Create(ctx, configMap); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, service, func() error {
		service.Labels = getLabels()
		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		desiredPorts := []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			},
		}
		service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		})
		deployment.Spec.Replicas = &k.replicas
		deployment.Spec.RevisionHistoryLimit = pointer.Int32(1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"checksum/secret-" + k.secrets.Kubeconfig.Name: k.secrets.Kubeconfig.Checksum,
					"checksum/secret-" + k.secrets.Server.Name:     k.secrets.Server.Checksum,
				},
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:         "true",
					v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           k.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         command,
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: probeURIScheme,
									Port:   intstr.FromInt(int(port)),
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    2,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          portNameMetrics,
								ContainerPort: port,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env: env,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("23m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("400m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      k.secrets.Kubeconfig.Name,
								MountPath: volumeMountPathKubeconfig,
							},
							{
								Name:      k.secrets.Server.Name,
								MountPath: volumeMountPathServer,
							},
							{
								Name:      volumeNameConfig,
								MountPath: volumeMountPathConfig,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: k.secrets.Kubeconfig.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: k.secrets.Kubeconfig.Name,
							},
						},
					},
					{
						Name: k.secrets.Server.Name,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: k.secrets.Server.Name,
							},
						},
					},
					{
						Name: volumeNameConfig,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				},
			},
		}

		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameKubeScheduler,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &autoscalingv1beta2.PodResourcePolicy{
			ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
				{
					ContainerName: autoscalingv1beta2.DefaultContainerResourcePolicy,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if err := k.reconcileShootResources(ctx); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObject(ctx, k.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-config", Namespace: k.namespace}})
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: LabelRole,
	}
}

func (k *kubeScheduler) Destroy(_ context.Context) error     { return nil }
func (k *kubeScheduler) Wait(_ context.Context) error        { return nil }
func (k *kubeScheduler) WaitCleanup(_ context.Context) error { return nil }
func (k *kubeScheduler) SetSecrets(secrets Secrets)          { k.secrets = secrets }

func (k *kubeScheduler) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-vpa", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyManagedResourceSecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedresources.SecretName(managedResourceName, true), Namespace: k.namespace}}
}

func (k *kubeScheduler) reconcileShootResources(ctx context.Context) error {
	return kutil.DeleteObjects(ctx, k.client, k.emptyManagedResource(), k.emptyManagedResourceSecret())
}

func (k *kubeScheduler) computeEnvironmentVariables() []corev1.EnvVar {
	if k.config != nil && k.config.KubeMaxPDVols != nil {
		return []corev1.EnvVar{{
			Name:  "KUBE_MAX_PD_VOLS",
			Value: *k.config.KubeMaxPDVols,
		}}
	}
	return nil
}

func (k *kubeScheduler) computeComponentConfig() (string, error) {
	var apiVersion string
	if version.ConstraintK8sGreaterEqual122.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1beta2"
	} else if version.ConstraintK8sGreaterEqual119.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1beta1"
	} else if version.ConstraintK8sGreaterEqual118.Check(k.version) {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha2"
	} else {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha1"
	}

	var componentConfigYAML bytes.Buffer
	if err := componentConfigTemplate.Execute(&componentConfigYAML, map[string]string{"apiVersion": apiVersion}); err != nil {
		return "", err
	}

	return componentConfigYAML.String(), nil
}

func (k *kubeScheduler) computeCommand(port int32) []string {
	var command []string

	if version.ConstraintK8sGreaterEqual117.Check(k.version) {
		command = append(command, "/usr/local/bin/kube-scheduler")
	} else {
		command = append(command, "/hyperkube", "kube-scheduler")
	}

	command = append(command, fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyComponentConfig))

	command = append(command,
		fmt.Sprintf("--authentication-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
		fmt.Sprintf("--authorization-kubeconfig=%s/%s", volumeMountPathKubeconfig, secrets.DataKeyKubeconfig),
		fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.ControlPlaneSecretDataKeyCertificatePEM(SecretNameServer)),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.ControlPlaneSecretDataKeyPrivateKey(SecretNameServer)),
		fmt.Sprintf("--secure-port=%d", port),
		"--port=0",
	)

	if k.config != nil {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}

	command = append(command, "--v=2")
	return command
}

var componentConfigTemplate *template.Template

func init() {
	var err error

	componentConfigTemplate, err = template.New("config").Parse(componentConfigTmpl)
	utilruntime.Must(err)
}

// Secrets is collection of secrets for the kube-scheduler.
type Secrets struct {
	// Kubeconfig is a secret which can be used by the kube-scheduler to communicate to the kube-apiserver.
	Kubeconfig component.Secret
	// Server is a secret for the HTTPS server inside the kube-scheduler (which is used for metrics and health checks).
	Server component.Secret
}
