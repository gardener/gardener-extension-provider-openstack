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

package vpnseedserver

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	protobuftypes "github.com/gogo/protobuf/types"
	istionetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// GatewayPort is the port exposed by the istio ingress gateway
	GatewayPort = 8132
	// VpnSeedServerTLSAuth is the name of seed server tlsauth Secret.
	VpnSeedServerTLSAuth = "vpn-seed-server-tlsauth"
	// vpnSeedServerDH is the name of seed server DH Secret.
	vpnSeedServerDH = "vpn-seed-server-dh"
	// VpnShootSecretName is the name of the shoot secret.
	VpnShootSecretName = "vpn-shoot-client"
	// DeploymentName is the name of vpn seed server deployment.
	DeploymentName = v1beta1constants.DeploymentNameVPNSeedServer
	// ServiceName is the name of the vpn seed server service running internally on the control plane in seed.
	ServiceName = DeploymentName
	// EnvoyPort is the port exposed by the envoy proxy on which it receives http proxy/connect requests.
	EnvoyPort = 9443

	envoyConfigDir          = "/etc/envoy"
	envoyConfigFileName     = "envoy.yaml"
	envoyTLSConfigDir       = "/etc/tls"
	envoyProxyContainerName = "envoy-proxy"
	openVPNPort             = 1194
	mountPathVpnSeedServer  = "/srv/secrets/vpn-server"
	mountPathTLSAuth        = "/srv/secrets/tlsauth"
	mountPathDH             = "/srv/secrets/dh"
	volumeNameEnvoyConfig   = "envoy-config"
)

// Interface contains functions for a vpn-seed-server deployer.
type Interface interface {
	component.DeployWaiter

	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
	// SetSeedNamespaceObjectUID sets UID for the namespace
	SetSeedNamespaceObjectUID(namespaceUID types.UID)

	// SetExposureClassHandlerName sets the name of the ExposureClass handler.
	SetExposureClassHandlerName(string)

	// SetSNIConfig set the sni config.
	SetSNIConfig(*config.SNI)
}

// Secrets is collection of secrets for the vpn-seed-server.
type Secrets struct {
	// TLSAuth is a secret containing the TLSAuth certificate.
	TLSAuth component.Secret
	// Server is a secret containing the server certificate and key.
	Server component.Secret
	// DiffieHellmanKey is a secret containing the diffie hellman key.
	DiffieHellmanKey component.Secret
}

// IstioIngressGateway contains the values for istio ingress gateway configuration.
type IstioIngressGateway struct {
	Namespace string
	Labels    map[string]string
}

// New creates a new instance of DeployWaiter for the vpn-seed-server.
func New(
	client client.Client,
	namespace string,
	imageAPIServerProxy string,
	imageVPNSeedServer string,
	kubeAPIServerHost *string,
	serviceNetwork string,
	podNetwork string,
	nodeNetwork *string,
	replicas int32,
	istioIngressGateway IstioIngressGateway,
) Interface {
	return &vpnSeedServer{
		client:              client,
		namespace:           namespace,
		imageAPIServerProxy: imageAPIServerProxy,
		imageVPNSeedServer:  imageVPNSeedServer,
		kubeAPIServerHost:   kubeAPIServerHost,
		serviceNetwork:      serviceNetwork,
		podNetwork:          podNetwork,
		nodeNetwork:         nodeNetwork,
		replicas:            replicas,
		istioIngressGateway: istioIngressGateway,
	}
}

type vpnSeedServer struct {
	client              client.Client
	namespace           string
	namespaceUID        types.UID
	imageAPIServerProxy string
	imageVPNSeedServer  string
	kubeAPIServerHost   *string
	serviceNetwork      string
	podNetwork          string
	nodeNetwork         *string
	replicas            int32

	istioIngressGateway      IstioIngressGateway
	exposureClassHandlerName *string
	sniConfig                *config.SNI
	secrets                  Secrets
}

func (v *vpnSeedServer) Deploy(ctx context.Context) error {
	if v.secrets.TLSAuth.Name == "" || v.secrets.TLSAuth.Checksum == "" {
		return fmt.Errorf("missing TLSAuth secret information")
	}
	if v.secrets.DiffieHellmanKey.Name == "" || v.secrets.DiffieHellmanKey.Checksum == "" {
		return fmt.Errorf("missing DH secret information")
	}
	if v.secrets.Server.Name == "" || v.secrets.Server.Checksum == "" {
		return fmt.Errorf("missing server secret information")
	}

	var (
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-seed-server-envoy-config",
				Namespace: v.namespace,
			},
			Data: map[string]string{
				envoyConfigFileName: envoyConfig,
			},
		}

		serverSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DeploymentName,
				Namespace: v.namespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: v.secrets.Server.Data,
		}

		tlsAuthSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      VpnSeedServerTLSAuth,
				Namespace: v.namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: v.secrets.TLSAuth.Data,
		}

		dhSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vpnSeedServerDH,
				Namespace: v.namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: v.secrets.DiffieHellmanKey.Data,
		}
	)

	utilruntime.Must(kutil.MakeUnique(configMap))
	utilruntime.Must(kutil.MakeUnique(serverSecret))
	utilruntime.Must(kutil.MakeUnique(tlsAuthSecret))
	utilruntime.Must(kutil.MakeUnique(dhSecret))

	var (
		service         = v.emptyService()
		deployment      = v.emptyDeployment()
		networkPolicy   = v.emptyNetworkPolicy()
		gateway         = v.emptyGateway()
		virtualService  = v.emptyVirtualService()
		destinationRule = v.emptyDestinationRule()
		vpa             = v.emptyVPA()
		envoyFilter     = v.emptyEnvoyFilter()
		igwSelectors    = v.getIngressGatewaySelectors()

		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto
	)

	if err := v.client.Create(ctx, configMap); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if err := v.client.Create(ctx, serverSecret); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if err := v.client.Create(ctx, tlsAuthSecret); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if err := v.client.Create(ctx, dhSecret); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, networkPolicy, func() error {
		networkPolicy.ObjectMeta.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: "Allows only Ingress/Egress between the kube-apiserver of the same control plane and the corresponding vpn-seed-server and Ingress from the istio ingress gateway to the vpn-seed-server.",
		}
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: GetLabels(),
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
									v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
									v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
								},
							},
						},
					},
				},
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							// we don't want to modify existing labels on the istio namespace
							NamespaceSelector: &metav1.LabelSelector{},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: igwSelectors,
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
									v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
									v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
								},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, deployment, func() error {
		maxSurge := intstr.FromInt(100)
		maxUnavailable := intstr.FromInt(0)
		deployment.Labels = map[string]string{
			v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelApp:                             DeploymentName,
			v1beta1constants.LabelNetworkPolicyFromShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		}
		deployment.Spec = appsv1.DeploymentSpec{
			Replicas:             pointer.Int32(v.replicas),
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
						v1beta1constants.GardenRole:                          v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:                            DeploymentName,
						v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: pointer.Bool(false),
					PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane,
					DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
					Containers: []corev1.Container{
						{
							Name:            DeploymentName,
							Image:           v.imageVPNSeedServer,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "tcp-tunnel",
									ContainerPort: openVPNPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN",
									},
								},
								Privileged: pointer.Bool(true),
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SERVICE_NETWORK",
									Value: v.serviceNetwork,
								},
								{
									Name:  "POD_NETWORK",
									Value: v.podNetwork,
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(openVPNPort),
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(openVPNPort),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      DeploymentName,
									MountPath: mountPathVpnSeedServer,
								},
								{
									Name:      VpnSeedServerTLSAuth,
									MountPath: mountPathTLSAuth,
								},
								{
									Name:      vpnSeedServerDH,
									MountPath: mountPathDH,
								},
							},
						},
						{
							Name:            envoyProxyContainerName,
							Image:           v.imageAPIServerProxy,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_BIND_SERVICE",
									},
								},
							},
							Command: []string{
								"envoy",
								"--concurrency",
								"2",
								"-c",
								fmt.Sprintf("%s/%s", envoyConfigDir, envoyConfigFileName),
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(EnvoyPort),
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(EnvoyPort),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("300Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameEnvoyConfig,
									MountPath: envoyConfigDir,
								},
								{
									Name:      DeploymentName,
									MountPath: envoyTLSConfigDir,
								},
							},
						},
					},
					TerminationGracePeriodSeconds: pointer.Int64(30),
					Volumes: []corev1.Volume{
						{
							Name: DeploymentName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  serverSecret.Name,
									DefaultMode: pointer.Int32Ptr(0400),
								},
							},
						},
						{
							Name: VpnSeedServerTLSAuth,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  tlsAuthSecret.Name,
									DefaultMode: pointer.Int32Ptr(0400),
								},
							},
						},
						{
							Name: vpnSeedServerDH,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  dhSecret.Name,
									DefaultMode: pointer.Int32Ptr(0400),
								},
							},
						},
						{
							Name: volumeNameEnvoyConfig,
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
			},
		}

		if v.nodeNetwork != nil {
			deployment.Spec.Template.Spec.Containers[0].Env = append(
				deployment.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "NODE_NETWORK", Value: *v.nodeNetwork},
			)
		}

		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, gateway, func() error {
		gateway.Spec = istionetworkingv1beta1.Gateway{
			Selector: igwSelectors,
			Servers: []*istionetworkingv1beta1.Server{
				{
					Hosts: []string{*v.kubeAPIServerHost},
					Port: &istionetworkingv1beta1.Port{
						Name:     "tls-tunnel",
						Number:   GatewayPort,
						Protocol: "HTTP",
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, destinationRule, func() error {
		destinationRule.Spec = istionetworkingv1beta1.DestinationRule{
			ExportTo: []string{"*"},
			Host:     fmt.Sprintf("%s.%s.svc.cluster.local", DeploymentName, v.namespace),
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
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, virtualService, func() error {
		virtualService.Spec = istionetworkingv1beta1.VirtualService{
			ExportTo: []string{"*"},
			Hosts:    []string{*v.kubeAPIServerHost},
			Gateways: []string{DeploymentName},
			Http: []*istionetworkingv1beta1.HTTPRoute{
				{
					Route: []*istionetworkingv1beta1.HTTPRouteDestination{
						{
							Destination: &istionetworkingv1beta1.Destination{
								Port: &istionetworkingv1beta1.PortSelector{
									Number: openVPNPort,
								},
								Host: DeploymentName,
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

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, service, func() error {
		service.Annotations = map[string]string{
			"networking.istio.io/exportTo": "*",
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       DeploymentName,
				Port:       openVPNPort,
				TargetPort: intstr.FromInt(openVPNPort),
			},
			{
				Name:       "http-proxy",
				Port:       EnvoyPort,
				TargetPort: intstr.FromInt(EnvoyPort),
			},
		}
		service.Spec.Selector = map[string]string{
			v1beta1constants.LabelApp: DeploymentName,
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, vpa, func() error {
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
				{
					ContainerName: envoyProxyContainerName,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20m"),
						corev1.ResourceMemory: resource.MustParse("20Mi"),
					},
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	envoyFilterSelector := v.istioIngressGateway.Labels
	if v.sniConfig != nil && v.exposureClassHandlerName != nil {
		envoyFilterSelector = igwSelectors
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, envoyFilter, func() error {
		envoyFilter.ObjectMeta.Name = envoyFilter.Name
		envoyFilter.ObjectMeta.Namespace = envoyFilter.Namespace
		envoyFilter.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion:         "v1",
				Kind:               "Namespace",
				Name:               v.namespace,
				UID:                v.namespaceUID,
				Controller:         pointer.Bool(false),
				BlockOwnerDeletion: pointer.Bool(false),
			},
		}
		envoyFilter.Spec.WorkloadSelector = &istionetworkingv1alpha3.WorkloadSelector{
			Labels: envoyFilterSelector,
		}
		envoyFilter.Spec.ConfigPatches = []*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			{
				ApplyTo: istionetworkingv1alpha3.EnvoyFilter_NETWORK_FILTER,
				Match: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
					Context: istionetworkingv1alpha3.EnvoyFilter_GATEWAY,
					ObjectTypes: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
						Listener: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch{
							Name:       fmt.Sprintf("0.0.0.0_%d", GatewayPort),
							PortNumber: GatewayPort,
							FilterChain: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
								Filter: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
									Name: "envoy.filters.network.http_connection_manager",
								},
							},
						},
					},
				},
				Patch: &istionetworkingv1alpha3.EnvoyFilter_Patch{
					Operation: istionetworkingv1alpha3.EnvoyFilter_Patch_MERGE,
					Value: &protobuftypes.Struct{
						Fields: map[string]*protobuftypes.Value{
							"name": {
								Kind: &protobuftypes.Value_StringValue{
									StringValue: "envoy.filters.network.http_connection_manager",
								},
							},
							"typed_config": {
								Kind: &protobuftypes.Value_StructValue{
									StructValue: &protobuftypes.Struct{
										Fields: map[string]*protobuftypes.Value{
											"@type": {
												Kind: &protobuftypes.Value_StringValue{
													StringValue: "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
												},
											},
											"route_config": {
												Kind: &protobuftypes.Value_StructValue{
													StructValue: &protobuftypes.Struct{
														Fields: map[string]*protobuftypes.Value{
															"virtual_hosts": {
																Kind: &protobuftypes.Value_ListValue{
																	ListValue: &protobuftypes.ListValue{
																		Values: []*protobuftypes.Value{
																			{
																				Kind: &protobuftypes.Value_StructValue{
																					StructValue: &protobuftypes.Struct{
																						Fields: map[string]*protobuftypes.Value{
																							"name": {
																								Kind: &protobuftypes.Value_StringValue{
																									StringValue: v.namespace,
																								},
																							},
																							"domains": {
																								Kind: &protobuftypes.Value_ListValue{
																									ListValue: &protobuftypes.ListValue{
																										Values: []*protobuftypes.Value{
																											{
																												Kind: &protobuftypes.Value_StringValue{
																													StringValue: fmt.Sprintf("%s:%d", *v.kubeAPIServerHost, GatewayPort),
																												},
																											},
																										},
																									},
																								},
																							},
																							"routes": {
																								Kind: &protobuftypes.Value_ListValue{
																									ListValue: &protobuftypes.ListValue{
																										Values: []*protobuftypes.Value{
																											{
																												Kind: &protobuftypes.Value_StructValue{
																													StructValue: &protobuftypes.Struct{
																														Fields: map[string]*protobuftypes.Value{
																															"match": {
																																Kind: &protobuftypes.Value_StructValue{
																																	StructValue: &protobuftypes.Struct{
																																		Fields: map[string]*protobuftypes.Value{
																																			"connect_matcher": {
																																				Kind: &protobuftypes.Value_StructValue{
																																					StructValue: &protobuftypes.Struct{},
																																				},
																																			},
																																		},
																																	},
																																},
																															},
																															"route": {
																																Kind: &protobuftypes.Value_StructValue{
																																	StructValue: &protobuftypes.Struct{
																																		Fields: map[string]*protobuftypes.Value{
																																			"cluster": {
																																				Kind: &protobuftypes.Value_StringValue{
																																					StringValue: fmt.Sprintf("outbound|%d||%s.%s.svc.cluster.local", openVPNPort, ServiceName, v.namespace),
																																				},
																																			},
																																			"upgrade_configs": {
																																				Kind: &protobuftypes.Value_ListValue{
																																					ListValue: &protobuftypes.ListValue{
																																						Values: []*protobuftypes.Value{
																																							{
																																								Kind: &protobuftypes.Value_StructValue{
																																									StructValue: &protobuftypes.Struct{
																																										Fields: map[string]*protobuftypes.Value{
																																											"upgrade_type": {
																																												Kind: &protobuftypes.Value_StringValue{
																																													StringValue: "CONNECT",
																																												},
																																											},
																																											"connect_config": {
																																												Kind: &protobuftypes.Value_StructValue{
																																													StructValue: &protobuftypes.Struct{},
																																												},
																																											},
																																										},
																																									},
																																								},
																																							},
																																						},
																																					},
																																				},
																																			},
																																		},
																																	},
																																},
																															},
																														},
																													},
																												},
																											},
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
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

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObjects(ctx, v.client,
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: "vpn-seed-server-envoy-config"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: DeploymentName}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: VpnSeedServerTLSAuth}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: vpnSeedServerDH}},
	)
}

func (v *vpnSeedServer) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(
		ctx,
		v.client,
		v.emptyNetworkPolicy(),
		v.emptyDeployment(),
		v.emptyGateway(),
		v.emptyDestinationRule(),
		v.emptyVirtualService(),
		v.emptyService(),
		v.emptyVPA(),
		v.emptyEnvoyFilter(),
		// TODO(rfranzke): Remove in a future release.
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: "vpn-seed-server-envoy-config"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: DeploymentName}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: VpnSeedServerTLSAuth}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: v.namespace, Name: vpnSeedServerDH}},
	)
}

func (v *vpnSeedServer) Wait(_ context.Context) error        { return nil }
func (v *vpnSeedServer) WaitCleanup(_ context.Context) error { return nil }

func (v *vpnSeedServer) SetSecrets(secrets Secrets) { v.secrets = secrets }

func (v *vpnSeedServer) SetSeedNamespaceObjectUID(namespaceUID types.UID) {
	v.namespaceUID = namespaceUID
}
func (v *vpnSeedServer) SetExposureClassHandlerName(handlerName string) {
	v.exposureClassHandlerName = &handlerName
}
func (v *vpnSeedServer) SetSNIConfig(cfg *config.SNI) { v.sniConfig = cfg }

func (v *vpnSeedServer) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-vpn-seed-server", Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyGateway() *networkingv1beta1.Gateway {
	return &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyVirtualService() *networkingv1beta1.VirtualService {
	return &networkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyDestinationRule() *networkingv1beta1.DestinationRule {
	return &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: DeploymentName + "-vpa", Namespace: v.namespace}}
}

func (v *vpnSeedServer) emptyEnvoyFilter() *networkingv1alpha3.EnvoyFilter {
	var namespace = v.istioIngressGateway.Namespace
	if v.sniConfig != nil && v.exposureClassHandlerName != nil {
		namespace = *v.sniConfig.Ingress.Namespace
	}
	return &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Name: v.namespace + "-vpn", Namespace: namespace}}
}

func (v *vpnSeedServer) getIngressGatewaySelectors() map[string]string {
	var defaulIgwSelectors = map[string]string{
		v1beta1constants.LabelApp: gardenletconfigv1alpha1.DefaultIngressGatewayAppLabelValue,
	}

	if v.sniConfig != nil {
		if v.exposureClassHandlerName != nil {
			return gutil.GetMandatoryExposureClassHandlerSNILabels(v.sniConfig.Ingress.Labels, *v.exposureClassHandlerName)
		}
		return utils.MergeStringMaps(v.sniConfig.Ingress.Labels, defaulIgwSelectors)
	}

	return defaulIgwSelectors
}

// GetLabels returns the labels for the vpn-seed-server
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:   DeploymentName,
	}
}

var envoyConfig = `static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: 0.0.0.0
        port_value: ` + fmt.Sprintf("%d", EnvoyPort) + `
    listener_filters:
    - name: "envoy.filters.listener.tls_inspector"
      typed_config: {}
    filter_chains:
    - transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          common_tls_context:
            tls_certificates:
            - certificate_chain: { filename: "` + envoyTLSConfigDir + `/tls.crt" }
              private_key: { filename: "` + envoyTLSConfigDir + `/tls.key" }
            validation_context:
              trusted_ca:
                filename: ` + envoyTLSConfigDir + `/ca.crt
      filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains:
              - "*"
              routes:
              - match:
                  connect_matcher: {}
                route:
                  cluster: dynamic_forward_proxy_cluster
                  upgrade_configs:
                  - upgrade_type: CONNECT
                    connect_config: {}
          http_filters:
          - name: envoy.filters.http.dynamic_forward_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.dynamic_forward_proxy.v3.FilterConfig
              dns_cache_config:
                name: dynamic_forward_proxy_cache_config
                dns_lookup_family: V4_ONLY
          - name: envoy.filters.http.router
          http_protocol_options:
            accept_http_10: true
          upgrade_configs:
          - upgrade_type: CONNECT
  clusters:
  - name: dynamic_forward_proxy_cluster
    connect_timeout: 20s
    lb_policy: CLUSTER_PROVIDED
    cluster_type:
      name: envoy.clusters.dynamic_forward_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.clusters.dynamic_forward_proxy.v3.ClusterConfig
        dns_cache_config:
          name: dynamic_forward_proxy_cache_config
          dns_lookup_family: V4_ONLY`
