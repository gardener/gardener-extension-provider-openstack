// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	namespace                        = "test"
	authURL                          = "someurl"
	region                           = "europe"
	technicalID                      = "shoot--dev--test"
	genericTokenKubeconfigSecretName = "generic-token-kubeconfig-92e9ae14"
)

var (
	dhcpDomain     = ptr.To("dhcp-domain")
	requestTimeout = &metav1.Duration{
		Duration: func() time.Duration { d, _ := time.ParseDuration("5m"); return d }(),
	}
)

func defaultControlPlane() *extensionsv1alpha1.ControlPlane {
	return defaultControlPlaneWithManila(false)
}

func defaultControlPlaneWithManila(csiManila bool) *extensionsv1alpha1.ControlPlane {
	cpConfig := &api.ControlPlaneConfig{
		LoadBalancerProvider: "load-balancer-provider",
		CloudControllerManager: &api.CloudControllerManagerConfig{
			FeatureGates: map[string]bool{
				"SomeKubernetesFeature": true,
			},
		},
	}
	var status *api.ShareNetworkStatus
	if csiManila {
		cpConfig.Storage = &api.Storage{CSIManila: &api.CSIManila{Enabled: true}}
		status = &api.ShareNetworkStatus{
			ID:   "1111-2222-3333-4444",
			Name: "sharenetwork",
		}
	}
	cp := controlPlane("floating-network-id", cpConfig, status)
	return cp
}

func controlPlane(floatingPoolID string, cfg *api.ControlPlaneConfig, status *api.ShareNetworkStatus) *extensionsv1alpha1.ControlPlane {
	return &extensionsv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "control-plane",
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.ControlPlaneSpec{
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(cfg),
				},
			},
			Region: region,
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: encode(&api.InfrastructureStatus{
					Networks: api.NetworkStatus{
						Name: technicalID,
						FloatingPool: api.FloatingPoolStatus{
							ID: floatingPoolID,
						},
						Router: api.RouterStatus{
							ID: "routerID",
						},
						Subnets: []api.Subnet{
							{
								ID:      "subnet-acbd1234",
								Purpose: api.PurposeNodes,
							},
						},
						ShareNetwork: status,
					},
				}),
			},
		},
	}
}

var _ = Describe("ValuesProvider", func() {
	var (
		ctx = context.TODO()

		ctrl *gomock.Controller

		scheme = runtime.NewScheme()
		_      = api.AddToScheme(scheme)

		fakeClient         client.Client
		fakeSecretsManager secretsmanager.Interface

		vp  genericactuator.ValuesProvider
		c   *mockclient.MockClient
		mgr *mockmanager.MockManager

		cp = defaultControlPlane()

		cidr                             = "10.250.0.0/19"
		rescanBlockStorageOnResize       = true
		ignoreVolumeAZ                   = true
		nodeVoluemAttachLimit      int32 = 25
		technicalID                      = technicalID

		cloudProfileConfig = &api.CloudProfileConfig{
			KeyStoneURL:                authURL,
			DHCPDomain:                 dhcpDomain,
			RequestTimeout:             requestTimeout,
			RescanBlockStorageOnResize: ptr.To(rescanBlockStorageOnResize),
			IgnoreVolumeAZ:             ptr.To(ignoreVolumeAZ),
			NodeVolumeAttachLimit:      ptr.To[int32](nodeVoluemAttachLimit),
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)

		cluster = &extensionscontroller.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"generic-token-kubeconfig.secret.gardener.cloud/name": genericTokenKubeconfigSecretName,
				},
			},
			CloudProfile: &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: cloudProfileConfigJSON,
					},
				},
			},
			Seed: &gardencorev1beta1.Seed{},
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.30.14",
					},
					Provider: gardencorev1beta1.Provider{
						InfrastructureConfig: &runtime.RawExtension{
							Raw: encode(&openstackv1alpha1.InfrastructureConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
									Kind:       "InfrastructureConfig",
								},
								Networks: openstackv1alpha1.Networks{
									Workers: "10.200.0.0/19",
								},
							}),
						},
						Workers: []gardencorev1beta1.Worker{
							{
								Name:  "worker",
								Zones: []string{"zone2", "zone1"},
							},
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					TechnicalID: technicalID,
				},
			},
		}
		clusterNoOverlay = &extensionscontroller.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"generic-token-kubeconfig.secret.gardener.cloud/name": genericTokenKubeconfigSecretName,
				},
			},
			CloudProfile: &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: cloudProfileConfigJSON,
					},
				},
			},
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To("calico"),
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"overlay":{"enabled": false}}`),
						},
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.31.1",
						VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
							Enabled: true,
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					TechnicalID: technicalID,
				},
			},
		}

		domainName  = "domain-name"
		tenantName  = "tenant-name"
		cpSecretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpSecret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"domainName": []byte(domainName),
				"tenantName": []byte(tenantName),
				"username":   []byte(`username`),
				"password":   []byte(`password`),
				"authURL":    []byte(authURL),
			},
		}

		cpConfigKey = client.ObjectKey{Namespace: namespace, Name: openstack.CloudProviderConfigName}
		cpConfig    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openstack.CloudProviderConfigName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.CloudProviderConfigDataKey: []byte("some data"),
			},
		}

		cloudProviderDiskConfig = []byte("foo")
		cpCSIDiskConfigKey      = client.ObjectKey{Namespace: namespace, Name: openstack.CloudProviderCSIDiskConfigName}
		cpCSIDiskConfig         = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openstack.CloudProviderCSIDiskConfigName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.CloudProviderConfigDataKey: cloudProviderDiskConfig,
			},
		}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			openstack.CloudProviderConfigName:        "bf19236c3ff3be18cf28cb4f58532bda4fd944857dd163baa05d23f952550392",
			openstack.CloudProviderCSIDiskConfigName: "77627eb2343b9f2dc2fca3cce35f2f9eec55783aa5f7dac21c473019e5825de2",
		}

		enabledTrue  = map[string]interface{}{"enabled": true}
		enabledFalse = map[string]interface{}{"enabled": false}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		fakeClient = fakeclient.NewClientBuilder().Build()
		fakeSecretsManager = fakesecretsmanager.New(fakeClient, namespace)

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)
		mgr.EXPECT().GetScheme().Return(scheme)
		vp = NewValuesProvider(mgr)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		configChartValues := map[string]interface{}{
			"domainName":                  "domain-name",
			"tenantName":                  "tenant-name",
			"username":                    "username",
			"password":                    "password",
			"region":                      region,
			"subnetID":                    "subnet-acbd1234",
			"lbProvider":                  "load-balancer-provider",
			"floatingNetworkID":           "floating-network-id",
			"insecure":                    false,
			"authUrl":                     authURL,
			"dhcpDomain":                  dhcpDomain,
			"requestTimeout":              requestTimeout,
			"ignoreVolumeAZ":              ignoreVolumeAZ,
			"applicationCredentialID":     "",
			"applicationCredentialSecret": "",
			"applicationCredentialName":   "",
			"internalNetworkName":         technicalID,
		}

		It("should return correct config chart values", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(configChartValues))
		})

		It("should return correct config chart values with load balancer classes", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			var (
				floatingNetworkID  = "4711"
				fsid               = "0815"
				floatingSubnetID2  = "pub0815"
				floatingSubnetName = "*public*"
				floatingSubnetTags = "tag1,tag2"
				subnetID           = "priv"
				floatingSubnetID   = "default-floating-subnet-id"
				cp                 = controlPlane(
					floatingNetworkID,
					&api.ControlPlaneConfig{
						LoadBalancerProvider: "load-balancer-provider",
						LoadBalancerClasses: []api.LoadBalancerClass{
							{
								Name:              "test",
								FloatingNetworkID: &floatingNetworkID,
								FloatingSubnetID:  &fsid,
								SubnetID:          nil,
							},
							{
								Name:             "default",
								FloatingSubnetID: &floatingSubnetID,
								SubnetID:         nil,
							},
							{
								Name:             "public",
								FloatingSubnetID: &floatingSubnetID2,
								SubnetID:         nil,
							},
							{
								Name:               "fip-subnet-by-name",
								FloatingSubnetName: &floatingSubnetName,
							},
							{
								Name:               "fip-subnet-by-tags",
								FloatingSubnetTags: &floatingSubnetTags,
							},
							{
								Name:     "other",
								SubnetID: &subnetID,
							},
						},
						CloudControllerManager: &api.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"SomeKubernetesFeature": true,
							},
						},
					},
					nil,
				)

				expectedValues = utils.MergeMaps(configChartValues, map[string]interface{}{
					"floatingNetworkID": floatingNetworkID,
					"floatingSubnetID":  floatingSubnetID,
					"floatingClasses": []map[string]interface{}{
						{
							"name":              "test",
							"floatingNetworkID": floatingNetworkID,
							"floatingSubnetID":  fsid,
						},
						{
							"name":             "default",
							"floatingSubnetID": floatingSubnetID,
						},
						{
							"name":             "public",
							"floatingSubnetID": floatingSubnetID2,
						},
						{
							"name":               "fip-subnet-by-name",
							"floatingSubnetName": floatingSubnetName,
						},
						{
							"name":               "fip-subnet-by-tags",
							"floatingSubnetTags": floatingSubnetTags,
						},
						{
							"name":     "other",
							"subnetID": subnetID,
						},
					},
				})
			)

			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(expectedValues))
		})

		It("should return correct config chart values with load balancer classes with purpose", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			var (
				floatingNetworkID = "fip1"
				cp                = controlPlane(
					floatingNetworkID,
					&api.ControlPlaneConfig{
						LoadBalancerProvider: "load-balancer-provider",
						LoadBalancerClasses: []api.LoadBalancerClass{
							{
								Name:             "default",
								FloatingSubnetID: ptr.To("fip-subnet-1"),
							},
							{
								Name:             "real-default",
								FloatingSubnetID: ptr.To("fip-subnet-2"),
								Purpose:          ptr.To("default"),
							},
						},
						CloudControllerManager: &api.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"SomeKubernetesFeature": true,
							},
						},
					},
					nil,
				)

				expectedValues = utils.MergeMaps(configChartValues, map[string]interface{}{
					"floatingNetworkID": floatingNetworkID,
					"floatingSubnetID":  "fip-subnet-2",
					"floatingClasses": []map[string]interface{}{
						{
							"name":             "default",
							"floatingSubnetID": "fip-subnet-1",
						},
						{
							"name":             "real-default",
							"floatingSubnetID": "fip-subnet-2",
						},
					},
				})
			)

			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(expectedValues))
		})

		It("should return correct config chart values with application credentials", func() {
			secret2 := *cpSecret
			secret2.Data = map[string][]byte{
				"domainName":                  []byte(domainName),
				"tenantName":                  []byte(tenantName),
				"applicationCredentialID":     []byte(`app-id`),
				"applicationCredentialSecret": []byte(`app-secret`),
				"authURL":                     []byte(authURL),
			}

			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(&secret2))

			expectedValues := utils.MergeMaps(configChartValues, map[string]interface{}{
				"username":                    "",
				"password":                    "",
				"applicationCredentialID":     "app-id",
				"applicationCredentialSecret": "app-secret",
				"insecure":                    false,
			})
			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(expectedValues))
		})

		It("should configure cloud routes when not using overlay", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			expectedValues := utils.MergeMaps(configChartValues, map[string]interface{}{
				"routerID": "routerID",
			})
			values, err := vp.GetConfigChartValues(ctx, cp, clusterNoOverlay)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(expectedValues))
		})

		It("should return correct config chart values with KeyStone CA Cert", func() {
			secret2 := cpSecret.DeepCopy()
			caCert := "custom-cert"
			secret2.Data["caCert"] = []byte(caCert)
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret2))
			expectedValues := utils.MergeMaps(configChartValues, map[string]interface{}{
				"caCert": caCert,
			})
			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(expectedValues))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		ccmChartValues := utils.MergeMaps(enabledTrue, map[string]interface{}{
			"replicas":    1,
			"clusterName": namespace,
			"podNetwork":  cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
				"checksum/secret-" + openstack.CloudProviderConfigName:        checksums[openstack.CloudProviderConfigName],
			},
			"podLabels": map[string]interface{}{
				"maintenance.gardener.cloud/restart": "true",
			},
			"featureGates": map[string]bool{
				"SomeKubernetesFeature": true,
			},
			"tlsCipherSuites": []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			},
			"secrets": map[string]interface{}{
				"server": "cloud-controller-manager-server",
			},
		})

		BeforeEach(func() {
			c.EXPECT().Get(ctx, cpConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfig))

			c.EXPECT().Delete(context.TODO(), &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "csi-snapshot-validation", Namespace: namespace}})
			c.EXPECT().Delete(context.TODO(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "csi-snapshot-validation", Namespace: namespace}})
			c.EXPECT().Delete(context.TODO(), &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "csi-snapshot-webhook-vpa", Namespace: namespace}})
			c.EXPECT().Delete(context.TODO(), &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "csi-snapshot-validation", Namespace: namespace}})

			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-openstack-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())
		})

		It("should return correct control plane chart values", func() {
			c.EXPECT().Get(ctx, cpCSIDiskConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpCSIDiskConfig))
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				openstack.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"userAgentHeaders": []string{domainName, tenantName, technicalID},
				}),
				openstack.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas":   1,
					"maxEntries": 1000,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
					},
					"userAgentHeaders": []string{domainName, tenantName, technicalID},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
				}),
				openstack.CSIManilaControllerName: enabledFalse,
			}))
		})

		It("should return correct control plane chart values if CSI Manila is enabled", func() {
			c.EXPECT().Get(ctx, cpCSIDiskConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpCSIDiskConfig))
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			cpManila := defaultControlPlaneWithManila(true)
			values, err := vp.GetControlPlaneChartValues(ctx, cpManila, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				openstack.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"userAgentHeaders": []string{domainName, tenantName, technicalID},
				}),
				openstack.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
					},
					"maxEntries":       1000,
					"userAgentHeaders": []string{domainName, tenantName, technicalID},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
				}),
				openstack.CSIManilaControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
					},
					"userAgentHeaders": []string{domainName, tenantName, technicalID},
					"csimanila": map[string]interface{}{
						"clusterID": "test",
					},
					"openstack": map[string]interface{}{
						"projectName":                 "tenant-name",
						"userName":                    "username",
						"password":                    "password",
						"applicationCredentialID":     "",
						"applicationCredentialName":   "",
						"availabilityZones":           []string{"zone1", "zone2"},
						"authURL":                     authURL,
						"region":                      "europe",
						"applicationCredentialSecret": "",
						"shareClient":                 "10.200.0.0/19",
						"shareNetworkID":              "1111-2222-3333-4444",
						"domainName":                  "domain-name",
						"tlsInsecure":                 "",
						"caCert":                      "",
					},
				}),
			}))
		})

		DescribeTable("topologyAwareRoutingEnabled value",
			func(seedSettings *gardencorev1beta1.SeedSettings, shootControlPlane *gardencorev1beta1.ControlPlane) {
				c.EXPECT().Get(ctx, cpCSIDiskConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpCSIDiskConfig))
				c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

				cluster.Seed = &gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Settings: seedSettings,
					},
				}
				cluster.Shoot.Spec.ControlPlane = shootControlPlane

				values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(HaveKey(openstack.CSIControllerName))
			},

			Entry("seed setting is nil, shoot control plane is not HA",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is disabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is enabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry("seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry("seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
		)
	})

	Describe("#GetControlPlaneShootChartValues", func() {
		BeforeEach(func() {
			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-openstack-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())
		})

		Context("shoot control plane chart values", func() {
			It("should return correct shoot control plane chart when ca is secret found", func() {
				c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

				values, err := vp.GetControlPlaneShootChartValues(ctx, cp, cluster, fakeSecretsManager, map[string]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					openstack.CloudControllerManagerName: enabledTrue,
					openstack.CSINodeName: utils.MergeMaps(enabledTrue, map[string]interface{}{
						"rescanBlockStorageOnResize": rescanBlockStorageOnResize,
						"nodeVolumeAttachLimit":      ptr.To[int32](nodeVoluemAttachLimit),
						"userAgentHeaders":           []string{domainName, tenantName, technicalID},
					}),
					openstack.CSIDriverManila:          enabledFalse,
					"calico-mutating-admission-policy": enabledFalse,
				}))
			})

			It("should return correct shoot control plane chart if CSI Manila is enabled", func() {
				c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

				cpManila := defaultControlPlaneWithManila(true)
				values, err := vp.GetControlPlaneShootChartValues(ctx, cpManila, cluster, fakeSecretsManager, map[string]string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					openstack.CloudControllerManagerName: enabledTrue,
					openstack.CSINodeName: utils.MergeMaps(enabledTrue, map[string]interface{}{
						"rescanBlockStorageOnResize": rescanBlockStorageOnResize,
						"nodeVolumeAttachLimit":      ptr.To[int32](nodeVoluemAttachLimit),
						"userAgentHeaders":           []string{domainName, tenantName, technicalID},
					}),
					openstack.CSIDriverManila: utils.MergeMaps(enabledTrue, map[string]interface{}{
						"csimanila": map[string]interface{}{
							"clusterID": "test",
						},
						"openstack": map[string]interface{}{
							"projectName":                 "tenant-name",
							"userName":                    "username",
							"password":                    "password",
							"applicationCredentialID":     "",
							"applicationCredentialName":   "",
							"availabilityZones":           []string{"zone1", "zone2"},
							"authURL":                     authURL,
							"region":                      "europe",
							"applicationCredentialSecret": "",
							"shareClient":                 "10.200.0.0/19",
							"shareNetworkID":              "1111-2222-3333-4444",
							"domainName":                  "domain-name",
							"tlsInsecure":                 "",
							"caCert":                      "",
						},
					}),
					"calico-mutating-admission-policy": enabledFalse,
				}))
			})
		})
	})

	Describe("#GetStorageClassesChartValues", func() {
		It("should return correct storage class chart values", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values["storageclasses"]).To(HaveLen(2))
			Expect(values["storageclasses"].([]map[string]interface{})[0]["provisioner"]).To(Equal(openstack.CSIStorageProvisioner))
			Expect(values["storageclasses"].([]map[string]interface{})[1]["provisioner"]).To(Equal(openstack.CSIStorageProvisioner))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func clientGet(result runtime.Object) interface{} {
	return func(_ context.Context, _ client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		}
		return nil
	}
}
