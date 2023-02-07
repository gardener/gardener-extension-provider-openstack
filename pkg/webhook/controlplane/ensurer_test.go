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

package controlplane

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

const namespace = "test"

const updateResolvConfScript = `#!/bin/sh

tmp=/etc/resolv-for-kubelet.conf.new
dest=/etc/resolv-for-kubelet.conf
line=%q

is_systemd_resolved_system()
{
    if [ -f /run/systemd/resolve/resolv.conf ]; then
      return 0
    else
      return 1
    fi
}

rm -f "$tmp"
if is_systemd_resolved_system; then
  if [ "$line" = "" ]; then
    ln -s /run/systemd/resolve/resolv.conf "$tmp"
  else
    cp /run/systemd/resolve/resolv.conf "$tmp"
    echo "" >> "$tmp"
    echo "# updated by update-resolv-conf.service (installed by gardener-extension-provider-openstack)" >> "$tmp"
    echo "$line" >> "$tmp"
  fi
else
  ln -s /etc/resolv.conf "$tmp"
fi
mv "$tmp" "$dest" && echo updated "$dest"
`

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctx = context.TODO()

		ctrl *gomock.Controller

		ensurer genericmutator.Ensurer

		dummyContext   = gcontext.NewGardenContext(nil, nil)
		eContextK8s120 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.20.0",
						},
					},
				},
			},
		)
		eContextK8s121 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.21.0",
						},
					},
				},
			},
		)
		eContextK8s126 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.26.0",
						},
					},
				},
			},
		)
		eContextK8s121WithResolvConfOptions = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				CloudProfile: &gardencorev1beta1.CloudProfile{
					Spec: gardencorev1beta1.CloudProfileSpec{
						ProviderConfig: &runtime.RawExtension{
							Raw: encode(&api.CloudProfileConfig{
								Constraints: api.Constraints{
									FloatingPools:         []api.FloatingPool{},
									LoadBalancerProviders: []api.LoadBalancerProvider{},
								},
								ResolvConfOptions: []string{"rotate", "timeout:1"},
							}),
						},
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.21.0",
						},
					},
				},
			},
		)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ensurer = NewEnsurer(logger)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureKubeAPIServerDeployment", func() {
		var dep *appsv1.Deployment

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-apiserver",
								},
							},
						},
					},
				},
			}
		})

		It("should add missing elements to kube-apiserver deployment (k8s = 1.20)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.20.0")
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.21)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.21.0")
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.26)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.26.0")
		})

		It("should modify existing elements of kube-apiserver deployment", func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-apiserver",
									Command: []string{
										"--cloud-provider=?",
										"--cloud-config=?",
										"--enable-admission-plugins=Priority,NamespaceLifecycle",
										"--disable-admission-plugins=PersistentVolumeLabel",
									},
								},
							},
						},
					},
				},
			}

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.20.0")
		})
	})

	Describe("#EnsureKubeControllerManagerDeployment", func() {
		var dep *appsv1.Deployment

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-controller-manager",
								},
							},
						},
					},
				},
			}
		})

		It("should add missing elements to kube-controller-manager deployment (k8s = 1.20)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.20.0")
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.21)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.21.0")
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.26)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.26.0")
		})

		It("should modify existing elements of kube-controller-manager deployment", func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToBlockedCIDRs: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-controller-manager",
									Command: []string{
										"--cloud-provider=?",
										"--cloud-config=?",
										"--external-cloud-volume-plugin=?",
									},
									VolumeMounts: []corev1.VolumeMount{
										{Name: usrShareCACertificatesName, MountPath: "?"},
										{Name: etcSSLName, MountPath: "?"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{Name: usrShareCACertificatesName},
								{Name: etcSSLName},
							},
						},
					},
				},
			}

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.20.0")
		})
	})

	Describe("#EnsureKubeSchedulerDeployment", func() {
		var (
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeScheduler},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-scheduler",
								},
							},
						},
					},
				},
			}

			ensurer = NewEnsurer(logger)
		})

		It("should add missing elements to kube-scheduler deployment (k8s = 1.20)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.20.0")
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.21)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.21.0")
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.26)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.26.0")
		})
	})

	Describe("#EnsureClusterAutoscalerDeployment", func() {
		var (
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameClusterAutoscaler},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "cluster-autoscaler",
								},
							},
						},
					},
				},
			}

			ensurer = NewEnsurer(logger)
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s 1.20)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.20.0")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.21)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.21.0")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.26)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.26.0")
		})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var (
			oldUnitOptions        []*unit.UnitOption
			hostnamectlUnitOption *unit.UnitOption
		)

		BeforeEach(func() {
			oldUnitOptions = []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
				},
			}

			hostnamectlUnitOption = &unit.UnitOption{
				Section: "Service",
				Name:    "ExecStartPre",
				Value:   `/bin/sh -c 'hostnamectl set-hostname $(cat /etc/hostname | cut -d '.' -f 1)'`,
			}
		})

		DescribeTable("should modify existing elements of kubelet.service unit options",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, cloudProvider string, withControllerAttachDetachFlag bool) {
				newUnitOptions := []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
					},
					hostnamectlUnitOption,
				}

				if cloudProvider != "" {
					newUnitOptions[0].Value += ` \
    --cloud-provider=` + cloudProvider

					if cloudProvider != "external" {
						newUnitOptions[0].Value += ` \
    --cloud-config=/var/lib/kubelet/cloudprovider.conf`
					}
				}

				if withControllerAttachDetachFlag {
					newUnitOptions[0].Value += ` \
    --enable-controller-attach-detach=true`
				}

				opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, kubeletVersion, oldUnitOptions, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(opts).To(Equal(newUnitOptions))
			},

			Entry("1.20 <= kubelet version < 1.23", dummyContext, semver.MustParse("1.20.0"), "external", true),
			Entry("kubelet version >= 1.23", dummyContext, semver.MustParse("1.23.0"), "external", false),
		)
	})

	Describe("#EnsureKubeletConfiguration", func() {
		var oldKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration

		BeforeEach(func() {
			oldKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
			}
		})

		DescribeTable("should modify existing elements of kubelet configuration",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, featureGates []string, enableControllerAttachDetach *bool) {
				newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo": true,
					},
					ResolverConfig:               pointer.String("/etc/resolv-for-kubelet.conf"),
					EnableControllerAttachDetach: enableControllerAttachDetach,
				}

				for _, featureGate := range featureGates {
					newKubeletConfig.FeatureGates[featureGate] = true
				}

				kubeletConfig := *oldKubeletConfig

				err := ensurer.EnsureKubeletConfiguration(ctx, gctx, kubeletVersion, &kubeletConfig, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(&kubeletConfig).To(Equal(newKubeletConfig))
			},

			Entry("control plane = 1.20, kubelet < 1.21", eContextK8s120, semver.MustParse("1.20.0"), []string{"CSIMigration", "CSIMigrationOpenStack", "CSIMigrationOpenStackComplete"}, nil),
			Entry("control plane >= 1.21, kubelet < 1.21", eContextK8s121, semver.MustParse("1.20.0"), []string{"CSIMigration", "CSIMigrationOpenStack", "CSIMigrationOpenStackComplete"}, nil),
			Entry("1.21 <= kubelet < 1.23", eContextK8s121, semver.MustParse("1.22.0"), []string{"CSIMigration", "CSIMigrationOpenStack", "InTreePluginOpenStackUnregister"}, nil),
			Entry("kubelet >= 1.23", eContextK8s121, semver.MustParse("1.23.0"), []string{"CSIMigration", "CSIMigrationOpenStack", "InTreePluginOpenStackUnregister"}, pointer.Bool(true)),
			Entry("kubelet >= 1.26", eContextK8s126, semver.MustParse("1.26.0"), []string{"CSIMigration"}, pointer.Bool(true)),
		)
	})

	Describe("#EnsureAdditionalUnits", func() {
		var (
			customUnitContent = `[Unit]
Description=update /etc/resolv-for-kubelet.conf on start and after each change of /run/systemd/resolve/resolv.conf
After=network.target
StartLimitIntervalSec=0

[Service]
Type=oneshot
ExecStart=/opt/bin/update-resolv-conf.sh
`

			customPathContent = `[Path]
PathChanged=/run/systemd/resolve/resolv.conf

[Install]
WantedBy=multi-user.target
`
			trueVar        = true
			oldUnit        = extensionsv1alpha1.Unit{Name: "oldunit"}
			additionalPath = extensionsv1alpha1.Unit{Name: "update-resolv-conf.path", Enable: &trueVar, Content: &customPathContent}
			additionalUnit = extensionsv1alpha1.Unit{Name: "update-resolv-conf.service", Enable: &trueVar, Content: &customUnitContent}
			units          = []extensionsv1alpha1.Unit{oldUnit}
		)

		It("should add additional units if resolvConfOptions field is not set", func() {
			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureAdditionalUnits method and check the result
			err := ensurer.EnsureAdditionalUnits(ctx, eContextK8s121, &units, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(units).To(ConsistOf(oldUnit, additionalPath, additionalUnit))
		})

		It("should add additional units to the current ones if resolvConfOptions field is set", func() {
			var (
				oldUnit        = extensionsv1alpha1.Unit{Name: "oldunit"}
				additionalPath = extensionsv1alpha1.Unit{Name: "update-resolv-conf.path", Enable: &trueVar, Content: &customPathContent}
				additionalUnit = extensionsv1alpha1.Unit{Name: "update-resolv-conf.service", Enable: &trueVar, Content: &customUnitContent}

				units = []extensionsv1alpha1.Unit{oldUnit}
			)

			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureAdditionalUnits method and check the result
			err := ensurer.EnsureAdditionalUnits(ctx, eContextK8s121WithResolvConfOptions, &units, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(units).To(ConsistOf(oldUnit, additionalPath, additionalUnit))
		})
	})

	Describe("#EnsureAdditionalFiles", func() {
		var (
			permissions int32 = 0o755

			filePath = "/opt/bin/update-resolv-conf.sh"

			oldFile            = extensionsv1alpha1.File{Path: "oldpath"}
			additionalFileFunc = func(options string) extensionsv1alpha1.File {
				return extensionsv1alpha1.File{
					Path:        filePath,
					Permissions: &permissions,
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "",
							Data:     strings.Replace(updateResolvConfScript, "%q", options, 1),
						},
					},
				}
			}
		)

		It("should add additional files to the current ones if resolvConfOptions field is not set", func() {
			files := []extensionsv1alpha1.File{oldFile}
			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureAdditionalFiles method and check the result
			err := ensurer.EnsureAdditionalFiles(ctx, eContextK8s121, &files, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(files).To(ConsistOf(oldFile, additionalFileFunc(`""`)))
		})
		It("should add additional files to the current ones if resolvConfOptions field is set", func() {
			files := []extensionsv1alpha1.File{oldFile}

			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureAdditionalFiles method and check the result
			err := ensurer.EnsureAdditionalFiles(ctx, eContextK8s121WithResolvConfOptions, &files, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(files).To(ConsistOf(oldFile, additionalFileFunc(`"options rotate timeout:1"`)))
		})

		It("should overwrite existing files of the current ones", func() {
			var (
				additionalFile = additionalFileFunc(`"options rotate timeout:1"`)
				files          = []extensionsv1alpha1.File{oldFile, additionalFile}
			)

			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureAdditionalFiles method and check the result
			err := ensurer.EnsureAdditionalFiles(ctx, eContextK8s121WithResolvConfOptions, &files, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(files).To(ConsistOf(oldFile, additionalFile))
			Expect(files).To(HaveLen(2))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")
	k8sVersionAtLeast126, _ := version.CompareVersions(k8sVersion, ">=", "1.26")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast126 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true"))
	} else if k8sVersionAtLeast121 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,InTreePluginOpenStackUnregister=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
	}
	Expect(c.Command).NotTo(ContainElement("--cloud-provider=openstack"))
	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
	Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")
	k8sVersionAtLeast126, _ := version.CompareVersions(k8sVersion, ">=", "1.26")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))

	if k8sVersionAtLeast126 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true"))
	} else if k8sVersionAtLeast121 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,InTreePluginOpenStackUnregister=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
	}
	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=openstack"))
	Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
	Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCACertificatesVolumeMount))
	Expect(dep.Spec.Template.Labels).To(BeEmpty())
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCACertificatesVolume))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
	Expect(dep.Spec.Template.Spec.Volumes).To(BeEmpty())
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")
	k8sVersionAtLeast126, _ := version.CompareVersions(k8sVersion, ">=", "1.26")

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast126 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true"))
	} else if k8sVersionAtLeast121 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,InTreePluginOpenStackUnregister=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
	}
}

func checkClusterAutoscalerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	if k8sVersionAtLeast120, _ := version.CompareVersions(k8sVersion, ">=", "1.20"); !k8sVersionAtLeast120 {
		return
	}
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")
	k8sVersionAtLeast126, _ := version.CompareVersions(k8sVersion, ">=", "1.26")

	// Check that the cluster-autoscaler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "cluster-autoscaler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast126 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true"))
	} else if k8sVersionAtLeast121 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,InTreePluginOpenStackUnregister=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
	}
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
