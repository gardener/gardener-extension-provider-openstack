// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
	apisopenstack "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Shoot validator", func() {
	Describe("#Validate", func() {
		const namespace = "garden-dev"

		var (
			shootValidator extensionswebhook.Validator

			shoot *core.Shoot

			ctx = context.Background()

			regionName   string
			imageName    string
			imageVersion string
			architecture *string

			scheme *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(apisopenstack.AddToScheme(scheme)).To(Succeed())
			Expect(apiv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			regionName = "eu-de-1"
			imageName = "Foo"
			imageVersion = "1.0.0"
			architecture = ptr.To("analog")

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "cloudProfile",
					},
					Provider: core.Provider{
						Type: "openstack",
						Workers: []core.Worker{
							{
								Name: "worker-1",
								Volume: &core.Volume{
									VolumeSize: "50Gi",
									Type:       ptr.To("volumeType"),
								},
								Zones: []string{"zone1"},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Name:    imageName,
										Version: imageVersion,
									},
									Architecture: architecture,
								},
							},
						},
						InfrastructureConfig: &runtime.RawExtension{
							Raw: encode(&apiv1alpha1.InfrastructureConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
									Kind:       "InfrastructureConfig",
								},
								Networks: apiv1alpha1.Networks{
									Workers: "10.250.0.0/19",
								},
								FloatingPoolName: "pool-1",
							}),
						},
						ControlPlaneConfig: &runtime.RawExtension{
							Raw: encode(&apiv1alpha1.ControlPlaneConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
									Kind:       "ControlPlaneConfig",
								},
								LoadBalancerProvider: "haproxy",
							}),
						},
					},
					Region: "eu-de-1",
					Networking: &core.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
					},
				},
			}
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil

				// No cloud profile lookup happens for workerless shoots; build a minimal validator.
				clientScheme := runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(clientScheme)).To(Succeed())
				c := fakeclient.NewClientBuilder().WithScheme(clientScheme).Build()

				apiReaderScheme := runtime.NewScheme()
				Expect(corev1.AddToScheme(apiReaderScheme)).To(Succeed())
				apiReader := fakeclient.NewClientBuilder().WithScheme(apiReaderScheme).Build()

				mgr := test.FakeManager{Scheme: scheme, Client: c, APIReader: apiReader}
				shootValidator = validator.NewShootValidator(mgr)
			})

			It("should not validate", func() {
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Shoot creation", func() {
			var (
				cloudProfile           *gardencorev1beta1.CloudProfile
				namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
			)

			BeforeEach(func() {
				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openstack",
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						Regions: []gardencorev1beta1.Region{
							{
								Name: regionName,
								Zones: []gardencorev1beta1.AvailabilityZone{
									{
										Name: "zone1",
									},
									{
										Name: "zone2",
									},
								},
							},
						},
						ProviderConfig: &runtime.RawExtension{
							Raw: encode(&apiv1alpha1.CloudProfileConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
									Kind:       "CloudProfileConfig",
								},
								MachineImages: []apiv1alpha1.MachineImages{
									{
										Name: imageName,
										Versions: []apiv1alpha1.MachineImageVersion{
											{
												Version: imageVersion,
												Regions: []apiv1alpha1.RegionIDMapping{
													{
														Name:         regionName,
														ID:           "Bar",
														Architecture: architecture,
													},
												},
											},
										},
									},
								},
							}),
						},
					},
				}

				namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "openstack-nscpfl",
						Namespace: namespace,
					},
					Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
						Parent: gardencorev1beta1.CloudProfileReference{
							Kind: "CloudProfile",
							Name: "openstack",
						},
					},
					Status: gardencorev1beta1.NamespacedCloudProfileStatus{
						CloudProfileSpec: cloudProfile.Spec,
					},
				}
			})

			// buildValidator constructs a shootValidator with a fakeclient containing the given objects.
			// Call this at the start of each It that needs the current state of cloudProfile/namespacedCloudProfile.
			buildValidator := func(objects ...client.Object) extensionswebhook.Validator {
				clientScheme := runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(clientScheme)).To(Succeed())
				c := fakeclient.NewClientBuilder().WithScheme(clientScheme).WithObjects(objects...).Build()

				apiReaderScheme := runtime.NewScheme()
				Expect(corev1.AddToScheme(apiReaderScheme)).To(Succeed())
				apiReader := fakeclient.NewClientBuilder().WithScheme(apiReaderScheme).Build()

				mgr := test.FakeManager{Scheme: scheme, Client: c, APIReader: apiReader}
				return validator.NewShootValidator(mgr)
			}

			It("should work for CloudProfile referenced from Shoot", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "openstack",
				}
				v := buildValidator(cloudProfile)

				err := v.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should work for CloudProfile referenced from cloudProfileName", func() {
				shoot.Spec.CloudProfileName = ptr.To("openstack")
				shoot.Spec.CloudProfile = nil
				v := buildValidator(cloudProfile)

				err := v.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should work for NamespacedCloudProfile referenced from Shoot", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "openstack-nscpfl",
				}
				v := buildValidator(namespacedCloudProfile)

				err := v.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail for a missing cloud profile provider config", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "openstack-nscpfl",
				}
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = nil
				v := buildValidator(namespacedCloudProfile)

				err := v.Validate(ctx, shoot, nil)
				Expect(err).To(MatchError(And(
					ContainSubstring("providerConfig is not given for cloud profile"),
					ContainSubstring("NamespacedCloudProfile"),
					ContainSubstring("openstack-nscpfl"),
				)))
			})

			Context("", func() {
				BeforeEach(func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "openstack",
					}
				})

				JustBeforeEach(func() {
					shootValidator = buildValidator(cloudProfile)
				})

				It("should succeed when networking is configured with dual-stack and subnetPoolID", func() {
					shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
						Raw: encode(&apiv1alpha1.InfrastructureConfig{
							TypeMeta: metav1.TypeMeta{
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								Kind:       "InfrastructureConfig",
							},
							Networks: apiv1alpha1.Networks{
								Workers: "10.250.0.0/19",
								IPv6: &apiv1alpha1.IPv6Config{
									SubnetPoolID: ptr.To("subnet-pool-id"),
								},
							},
							FloatingPoolName: "pool-1",
						}),
					}

					err := shootValidator.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should succeed when networking is configured with dual-stack and IPv6 config", func() {
					shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
						Raw: encode(&apiv1alpha1.InfrastructureConfig{
							TypeMeta: metav1.TypeMeta{
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								Kind:       "InfrastructureConfig",
							},
							Networks: apiv1alpha1.Networks{
								Workers: "10.250.0.0/19",
								IPv6: &apiv1alpha1.IPv6Config{
									NodeCIDR:    "2001:db8:1::/64",
									PodCIDR:     "2001:db8:2::/64",
									ServiceCIDR: "2001:db8:3::/112",
								},
							},
							FloatingPoolName: "pool-1",
						}),
					}

					err := shootValidator.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return err when networking is configured to use dual-stack without subnetPoolID or IPv6 config", func() {
					shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}

					err := shootValidator.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.provider.infrastructureConfig"),
					}))))
				})

				It("should return err when networking is configured to use IPv6-only", func() {
					shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}

					err := shootValidator.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.networking.ipFamilies"),
						})),
					))
				})
			})
		})

		Context("DNS validation", func() {
			var (
				cloudProfile *gardencorev1beta1.CloudProfile
				dnsSecret    *corev1.Secret
			)

			BeforeEach(func() {
				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openstack",
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						Regions: []gardencorev1beta1.Region{
							{
								Name: regionName,
								Zones: []gardencorev1beta1.AvailabilityZone{
									{Name: "zone1"},
									{Name: "zone2"},
								},
							},
						},
						ProviderConfig: &runtime.RawExtension{
							Raw: encode(&apiv1alpha1.CloudProfileConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
									Kind:       "CloudProfileConfig",
								},
								MachineImages: []apiv1alpha1.MachineImages{
									{
										Name: imageName,
										Versions: []apiv1alpha1.MachineImageVersion{
											{
												Version: imageVersion,
												Regions: []apiv1alpha1.RegionIDMapping{
													{
														Name:         regionName,
														ID:           "Bar",
														Architecture: architecture,
													},
												},
											},
										},
									},
								},
							}),
						},
					},
				}

				dnsSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dns-secret",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						openstack.AuthURL:    []byte("https://keystone.example.com:5000/v3"),
						openstack.DomainName: []byte("my-domain"),
						openstack.TenantName: []byte("my-tenant"),
						openstack.UserName:   []byte("my-user"),
						openstack.Password:   []byte("my-password"),
					},
				}

				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "openstack",
				}
			})

			// buildDNSValidator constructs a shootValidator with cloudProfile in the main client
			// and the given apiReader objects (DNS secrets, workload identities, etc.).
			buildDNSValidator := func(apiReaderObjects ...client.Object) extensionswebhook.Validator {
				clientScheme := runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(clientScheme)).To(Succeed())
				c := fakeclient.NewClientBuilder().WithScheme(clientScheme).WithObjects(cloudProfile).Build()

				apiReaderScheme := runtime.NewScheme()
				Expect(corev1.AddToScheme(apiReaderScheme)).To(Succeed())
				Expect(securityv1alpha1.AddToScheme(apiReaderScheme)).To(Succeed())
				apiReader := fakeclient.NewClientBuilder().WithScheme(apiReaderScheme).WithObjects(apiReaderObjects...).Build()

				mgr := test.FakeManager{Scheme: scheme, Client: c, APIReader: apiReader}
				return validator.NewShootValidator(mgr)
			}

			It("should pass when shoot has no DNS configuration", func() {
				shoot.Spec.DNS = nil
				v := buildDNSValidator()

				err := v.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("primaryProvider", func() {
				It("should pass when shoot has DNS but no OpenStack providers", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To("aws-route53"),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "aws-dns-secret",
								},
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should pass when shoot has non-primary OpenStack DNS provider", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(false),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should pass when shoot has OpenStack DNS provider with Primary=nil", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: nil,
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should pass when all OpenStack DNS providers are non-primary", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(false),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "secret1",
								},
							},
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(false),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "secret2",
								},
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should pass with multiple DNS providers (only validates primary OpenStack)", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To("aws-route53"),
								Primary: ptr.To(false),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "aws-secret",
								},
							},
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
							},
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(false),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "other-secret",
								},
							},
						},
					}
					v := buildDNSValidator(dnsSecret)

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should report errors against the correct provider index", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(false),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "non-primary-secret",
								},
							},
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.providers[1].credentialsRef"),
					}))))
				})
			})

			Context("credentialsRef", func() {
				It("should fail when credentialsRef is missing", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.dns.providers[0].credentialsRef"),
						"Detail": ContainSubstring("credentialsRef must be specified"),
					}))))
				})

				It("should fail when credentialsRef points to an unsupported GVK", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "foo.bar/v1",
									Kind:       "Baz",
									Name:       "dns-baz-ref",
								},
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInternal),
						"Field":  Equal("spec.dns.providers[0].credentialsRef"),
						"Detail": ContainSubstring("unsupported credentials reference"),
					}))))
				})

				It("should fail when credentialsRef points to a WorkloadIdentity", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "security.gardener.cloud/v1alpha1",
									Kind:       "WorkloadIdentity",
									Name:       "dns-workload-identity",
								},
							},
						},
					}
					v := buildDNSValidator(&securityv1alpha1.WorkloadIdentity{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dns-workload-identity",
							Namespace: namespace,
						},
					})

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.providers[0].credentialsRef"),
						"Detail": ContainSubstring("supported credentials type is Secret"),
					}))))
				})

				It("should fail when DNS credentials do not exist", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
							},
						},
					}
					v := buildDNSValidator() // no dnsSecret → not found

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotFound),
						"Field": Equal("spec.dns.providers[0].credentialsRef"),
					}))))
				})

				It("should fail when DNS secret has validation errors", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
							},
						},
					}
					// Invalid secret - missing authURL (required for DNS)
					dnsSecret.Data = map[string][]byte{
						openstack.DomainName: []byte("my-domain"),
						openstack.TenantName: []byte("my-tenant"),
						openstack.UserName:   []byte("my-user"),
						openstack.Password:   []byte("my-password"),
					}
					v := buildDNSValidator(dnsSecret)

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.providers[0].credentialsRef.data[authURL]"),
					}))))
				})

				It("should pass with valid primary OpenStack DNS provider", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
							},
						},
					}
					v := buildDNSValidator(dnsSecret)

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			// TODO(@wpross): Remove this context once support for Kubernetes 1.34 is dropped and secretName is no longer accepted.
			Context("secretName", func() {
				It("should require credentialsRef (not secretName) when both are unset, to push migration", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
							},
						},
					}
					v := buildDNSValidator()

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.dns.providers[0].credentialsRef"),
						"Detail": ContainSubstring("credentialsRef must be specified"),
					}))))
				})

				It("should pass with valid primary OpenStack DNS provider using secretName", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:       ptr.To(openstack.DNSType),
								Primary:    ptr.To(true),
								SecretName: ptr.To("dns-secret"),
							},
						},
					}
					v := buildDNSValidator(dnsSecret)

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should fail when DNS secret referenced by secretName does not exist", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:       ptr.To(openstack.DNSType),
								Primary:    ptr.To(true),
								SecretName: ptr.To("dns-secret"),
							},
						},
					}
					v := buildDNSValidator() // no dnsSecret → not found

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotFound),
						"Field": Equal("spec.dns.providers[0].secretName"),
					}))))
				})

				It("should fail when DNS secret referenced by secretName has validation errors", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:       ptr.To(openstack.DNSType),
								Primary:    ptr.To(true),
								SecretName: ptr.To("dns-secret"),
							},
						},
					}
					// Invalid secret - missing authURL (required for DNS)
					dnsSecret.Data = map[string][]byte{
						openstack.DomainName: []byte("my-domain"),
						openstack.TenantName: []byte("my-tenant"),
						openstack.UserName:   []byte("my-user"),
						openstack.Password:   []byte("my-password"),
					}
					v := buildDNSValidator(dnsSecret)

					err := v.Validate(ctx, shoot, nil)
					Expect(err).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.providers[0].secretName.data[authURL]"),
					}))))
				})

				It("should prefer credentialsRef when both credentialsRef and secretName are set", func() {
					shoot.Spec.DNS = &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    ptr.To(openstack.DNSType),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "dns-secret",
								},
								SecretName: ptr.To("other-secret"),
							},
						},
					}
					v := buildDNSValidator(dnsSecret)

					err := v.Validate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
