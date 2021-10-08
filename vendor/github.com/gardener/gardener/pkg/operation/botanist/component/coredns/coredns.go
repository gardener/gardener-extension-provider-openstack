// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package coredns

import (
	"context"
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/api/resources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LabelKey is the key of a label used for the identification of CoreDNS pods.
	LabelKey = "k8s-app"
	// LabelValue is the value of a label used for the identification of CoreDNS pods (it's 'kube-dns' for legacy
	// reasons).
	LabelValue = "kube-dns"
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-core-coredns"
	// PortServiceServer is the service port used for the DNS server.
	PortServiceServer = 53
	// PortServer is the target port used for the DNS server.
	PortServer = 8053
	// DeploymentName is the name of the coredns Deployment.
	DeploymentName = "coredns"

	containerName = "coredns"
	serviceName   = "kube-dns" // this is due to legacy reasons

	portNameMetrics = "metrics"
	portMetrics     = 9153

	configDataKey               = "Corefile"
	volumeNameConfig            = "config-volume"
	volumeNameConfigCustom      = "custom-config-volume"
	volumeMountPathConfig       = "/etc/coredns"
	volumeMountPathConfigCustom = "/etc/coredns/custom"
)

// Interface contains functions for a CoreDNS deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	SetPodAnnotations(map[string]string)
}

// Values is a set of configuration values for the coredns component.
type Values struct {
	// APIServerHost is the host of the kube-apiserver.
	APIServerHost *string
	// ClusterDomain is the domain used for cluster-wide DNS records handled by CoreDNS.
	ClusterDomain string
	// ClusterIP is the IP address which should be used as `.spec.clusterIP` in the Service spec.
	ClusterIP string
	// Image is the container image used for CoreDNS.
	Image string
	// PodAnnotations is the set of additional annotations to be used for the pods.
	PodAnnotations map[string]string
	// PodNetworkCIDR is the CIDR of the pod network.
	PodNetworkCIDR string
	// NodeNetworkCIDR is the CIDR of the node network.
	NodeNetworkCIDR *string
}

// New creates a new instance of DeployWaiter for coredns.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &coreDNS{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type coreDNS struct {
	client    client.Client
	namespace string
	values    Values
}

func (c *coreDNS) Deploy(ctx context.Context) error {
	data, err := c.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, ManagedResourceName, false, data)
}

func (c *coreDNS) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, c.client, c.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *coreDNS) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *coreDNS) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *coreDNS) computeResourcesData() (map[string][]byte, error) {
	var (
		portAPIServer       = intstr.FromInt(kubeapiserver.Port)
		portDNSServerHost   = intstr.FromInt(53)
		portDNSServer       = intstr.FromInt(PortServer)
		portMetricsEndpoint = intstr.FromInt(portMetrics)
		protocolTCP         = corev1.ProtocolTCP
		protocolUDP         = corev1.ProtocolUDP
		intStrOne           = intstr.FromInt(1)
		intStrZero          = intstr.FromInt(0)

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:coredns",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints", "services", "pods", "namespaces"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"discovery.k8s.io"},
					Resources: []string{"endpointslices"},
					Verbs:     []string{"list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "system:coredns",
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		// We don't need to make this ConfigMap immutable since CoreDNS provides the "reload" plugins which does an
		// auto-reload if the config changes.
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				configDataKey: `.:` + strconv.Itoa(PortServer) + ` {
  errors
  log . {
      class error
  }
  health
  ready
  kubernetes ` + c.values.ClusterDomain + ` in-addr.arpa ip6.arpa {
      pods insecure
      fallthrough in-addr.arpa ip6.arpa
      ttl 30
  }
  prometheus 0.0.0.0:` + strconv.Itoa(portMetrics) + `
  forward . /etc/resolv.conf
  cache 30
  loop
  reload
  loadbalance round_robin
  import custom/*.override
}
import custom/*.server
`,
			},
		}

		configMapCustom = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "coredns-custom",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Ignore: "true"},
			},
			Data: map[string]string{
				"changeme.server":   "# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns.md",
				"changeme.override": "# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns.md",
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					LabelKey:                        LabelValue,
					"kubernetes.io/cluster-service": "true",
					"kubernetes.io/name":            "CoreDNS",
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: c.values.ClusterIP,
				Selector:  map[string]string{LabelKey: LabelValue},
				Ports: []corev1.ServicePort{
					{
						Name:       "dns",
						Port:       int32(PortServiceServer),
						TargetPort: intstr.FromInt(PortServer),
						Protocol:   corev1.ProtocolUDP,
					},
					{
						Name:       "dns-tcp",
						Port:       int32(PortServiceServer),
						TargetPort: intstr.FromInt(PortServer),
						Protocol:   corev1.ProtocolTCP,
					},
					{
						Name:       "metrics",
						Port:       int32(portMetrics),
						TargetPort: intstr.FromInt(portMetrics),
					},
				},
			},
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-dns",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: "Allows CoreDNS to lookup DNS records, talk to the API Server. " +
						"Also allows CoreDNS to be reachable via its service and its metrics endpoint.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      LabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{LabelValue},
					}},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &portAPIServer, Protocol: &protocolTCP},     // Allow communication to API Server
						{Port: &portDNSServerHost, Protocol: &protocolTCP}, // Lookup DNS due to cache miss
						{Port: &portDNSServerHost, Protocol: &protocolUDP}, // Lookup DNS due to cache miss
					},
				}},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &portMetricsEndpoint, Protocol: &protocolTCP}, // CoreDNS metrics port
						{Port: &portDNSServer, Protocol: &protocolTCP},       // CoreDNS server port
						{Port: &portDNSServer, Protocol: &protocolUDP},       // CoreDNS server port
					},
					From: []networkingv1.NetworkPolicyPeer{
						{NamespaceSelector: &metav1.LabelSelector{}, PodSelector: &metav1.LabelSelector{}},
						{IPBlock: &networkingv1.IPBlock{CIDR: c.values.PodNetworkCIDR}},
					},
				}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DeploymentName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(1),
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       &intStrOne,
						MaxUnavailable: &intStrZero,
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{LabelKey: LabelValue},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: utils.MergeStringMaps(c.values.PodAnnotations, map[string]string{
							"scheduler.alpha.kubernetes.io/critical-pod": "",
						}),
						Labels: getLabels(),
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: corev1.LabelHostname,
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{{
												Key:      LabelKey,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{LabelValue},
											}},
										},
									},
								}},
							},
						},
						PriorityClassName:  "system-cluster-critical",
						ServiceAccountName: serviceAccount.Name,
						NodeSelector:       map[string]string{v1beta1constants.LabelWorkerPoolSystemComponents: "true"},
						DNSPolicy:          corev1.DNSDefault,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: pointer.Bool(true),
							RunAsUser:    pointer.Int64(65534),
						},
						Tolerations: []corev1.Toleration{{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						}},
						Containers: []corev1.Container{{
							Name:            containerName,
							Image:           c.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{"" +
								"-conf",
								volumeMountPathConfig + "/" + configDataKey,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "dns-udp",
									Protocol:      protocolUDP,
									ContainerPort: PortServer,
								},
								{
									Name:          "dns-tcp",
									Protocol:      protocolTCP,
									ContainerPort: PortServer,
								},
								{
									Name:          portNameMetrics,
									Protocol:      protocolTCP,
									ContainerPort: portMetrics,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"all"},
								},
								ReadOnlyRootFilesystem: pointer.Bool(true),
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Scheme: corev1.URISchemeHTTP,
										Port:   intstr.FromInt(8080),
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    5,
								InitialDelaySeconds: 60,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/ready",
										Scheme: corev1.URISchemeHTTP,
										Port:   intstr.FromInt(8181),
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    1,
								InitialDelaySeconds: 30,
								TimeoutSeconds:      2,
								PeriodSeconds:       10,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("15Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameConfig,
									MountPath: volumeMountPathConfig,
									ReadOnly:  true,
								},
								{
									Name:      volumeNameConfigCustom,
									MountPath: volumeMountPathConfigCustom,
									ReadOnly:  true,
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: volumeNameConfig,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
										Items: []corev1.KeyToPath{{
											Key:  configDataKey,
											Path: configDataKey,
										}},
									},
								},
							},
							{
								Name: volumeNameConfigCustom,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapCustom.Name,
										},
										DefaultMode: pointer.Int32(420),
										Optional:    pointer.Bool(true),
									},
								},
							},
						},
					},
				},
			},
		}

		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
				Labels:    map[string]string{LabelKey: LabelValue},
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &intStrOne,
				Selector:       deployment.Spec.Selector,
			},
		}

		horizontalPodAutoscaler = &autoscalingv2beta1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: autoscalingv2beta1.HorizontalPodAutoscalerSpec{
				MinReplicas: pointer.Int32(2),
				MaxReplicas: 5,
				Metrics: []autoscalingv2beta1.MetricSpec{{
					Type: autoscalingv2beta1.ResourceMetricSourceType,
					Resource: &autoscalingv2beta1.ResourceMetricSource{
						Name:                     corev1.ResourceCPU,
						TargetAverageUtilization: pointer.Int32(70),
					},
				}},
				ScaleTargetRef: autoscalingv2beta1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
			},
		}
	)

	if c.values.APIServerHost != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: *c.values.APIServerHost,
		})
	}

	if c.values.NodeNetworkCIDR != nil {
		networkPolicy.Spec.Ingress[0].From = append(networkPolicy.Spec.Ingress[0].From, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{CIDR: *c.values.NodeNetworkCIDR},
		})
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		configMap,
		configMapCustom,
		service,
		networkPolicy,
		deployment,
		podDisruptionBudget,
		horizontalPodAutoscaler,
	)
}

func (c *coreDNS) SetPodAnnotations(v map[string]string) {
	c.values.PodAnnotations = v
}

func getLabels() map[string]string {
	return map[string]string{
		"origin":                    "gardener",
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent,
		LabelKey:                    LabelValue,
	}
}
