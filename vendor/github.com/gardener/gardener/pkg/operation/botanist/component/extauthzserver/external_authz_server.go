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

package extauthzserver

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	protobuftypes "github.com/gogo/protobuf/types"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AuthServerPort is the port exposed by the external authorization server
	AuthServerPort = 9001
	// DeploymentName is the name of the external authorization server deployment.
	DeploymentName = "reversed-vpn-auth-server"
	// ServiceName is the name of the external authorization server service.
	ServiceName = DeploymentName
)

// NewExtAuthServer creates a new instance of DeployWaiter for the auth-server.
func NewExtAuthServer(
	client client.Client,
	namespace string,
	imageExtAuthzServer string,
	replicas int32,
) component.DeployWaiter {
	return &authServer{
		client:              client,
		namespace:           namespace,
		imageExtAuthzServer: imageExtAuthzServer,
		replicas:            replicas,
	}
}

type authServer struct {
	client              client.Client
	namespace           string
	imageExtAuthzServer string
	replicas            int32
}

func (a *authServer) Deploy(ctx context.Context) error {
	var (
		deployment      = a.emptyDeployment()
		destinationRule = a.emptyDestinationRule()
		service         = a.emptyService()
		virtualService  = a.emptyVirtualService()
		vpa             = a.emptyVPA()
		pdb             = a.emptyPDB()
		pc              = a.emptyPC()

		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto
	)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, pc, func() error {
		pc.Description = "This class is used to ensure that the reversed-vpn-auth-server has a high priority and is not preempted in favor of other pods."
		pc.GlobalDefault = false
		pc.Value = 1000000000
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, deployment, func() error {
		maxSurge := intstr.FromInt(100)
		maxUnavailable := intstr.FromInt(0)
		deployment.Labels = map[string]string{
			v1beta1constants.LabelApp: DeploymentName,
		}
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas:             pointer.Int32(a.replicas),
			RevisionHistoryLimit: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				v1beta1constants.LabelApp: DeploymentName,
			}},
			Strategy: appsv1.DeploymentStrategy{
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1beta1constants.LabelApp: DeploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: "In",
												Values:   []string{DeploymentName},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					AutomountServiceAccountToken: pointer.Bool(false),
					PriorityClassName:            pc.Name,
					DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
					Containers: []corev1.Container{
						{
							Name:            DeploymentName,
							Image:           a.imageExtAuthzServer,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc-authz",
									ContainerPort: 9001,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
						},
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, destinationRule, func() error {
		destinationRule.Spec = istionetworkingv1beta1.DestinationRule{
			ExportTo: []string{"*"},
			Host:     fmt.Sprintf("%s.%s.svc.%s", DeploymentName, a.namespace, gardencorev1beta1.DefaultDomain),
			TrafficPolicy: &istionetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istionetworkingv1beta1.ConnectionPoolSettings{
					Tcp: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
						MaxConnections: 5000,
						TcpKeepalive: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
							Interval: &protobuftypes.Duration{
								Seconds: 75,
							},
							Time: &protobuftypes.Duration{
								Seconds: 7200,
							},
						},
					},
				},
				Tls: &istionetworkingv1beta1.ClientTLSSettings{
					Mode: istionetworkingv1beta1.ClientTLSSettings_DISABLE,
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, service, func() error {
		service.Annotations = map[string]string{
			"networking.istio.io/exportTo": "*",
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "grpc-authz",
				Port:       AuthServerPort,
				TargetPort: intstr.FromInt(AuthServerPort),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		service.Spec.Selector = map[string]string{
			v1beta1constants.LabelApp: DeploymentName,
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, virtualService, func() error {
		virtualService.Spec = istionetworkingv1beta1.VirtualService{
			ExportTo: []string{"*"},
			Hosts:    []string{fmt.Sprintf("%s.%s.svc.%s", DeploymentName, a.namespace, gardencorev1beta1.DefaultDomain)},
			Http: []*istionetworkingv1beta1.HTTPRoute{{
				Route: []*istionetworkingv1beta1.HTTPRouteDestination{{
					Destination: &istionetworkingv1beta1.Destination{
						Host: DeploymentName,
						Port: &istionetworkingv1beta1.PortSelector{Number: AuthServerPort},
					},
				}},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       DeploymentName,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		vpa.Spec.ResourcePolicy = &autoscalingv1beta2.PodResourcePolicy{
			ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
				{
					ContainerName: DeploymentName,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, pdb, func() error {
		maxUnavailable := intstr.FromInt(1)
		pdb.Labels = getLabels()
		pdb.Spec.MaxUnavailable = &maxUnavailable
		pdb.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: getLabels(),
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (a *authServer) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(
		ctx,
		a.client,
		a.emptyDeployment(),
		a.emptyDestinationRule(),
		a.emptyService(),
		a.emptyVirtualService(),
		a.emptyVPA(),
		a.emptyPDB(),
		a.emptyPC(),
	)
}

func (a *authServer) Wait(_ context.Context) error        { return nil }
func (a *authServer) WaitCleanup(_ context.Context) error { return nil }

func (a *authServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: a.namespace}}
}

func (a *authServer) emptyDestinationRule() *networkingv1beta1.DestinationRule {
	return &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: a.namespace}}
}

func (a *authServer) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: a.namespace}}
}

func (a *authServer) emptyVirtualService() *networkingv1beta1.VirtualService {
	return &networkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: a.namespace}}
}

func (a *authServer) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName + "-vpa", Namespace: a.namespace}}
}

func (a *authServer) emptyPDB() *policyv1beta1.PodDisruptionBudget {
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName + "-pdb", Namespace: a.namespace}}
}

func (a *authServer) emptyPC() *schedulingv1.PriorityClass {
	return &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: DeploymentName,
	}
}
