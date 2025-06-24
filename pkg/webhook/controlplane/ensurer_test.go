// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/pkg/utils/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

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

if grep -q "# No DNS servers known." /run/systemd/resolve/resolv.conf; then
  exit 10
fi

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

		eContextK8s127 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.27.1",
						},
					},
				},
			},
		)
		eContextK8s131 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.31.1",
						},
					},
				},
			},
		)
		eContextK8s127WithResolvConfOptions = gcontext.NewInternalGardenContext(
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
							Version: "1.27.1",
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

		It("should add missing elements to kube-apiserver deployment (k8s < 1.31)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.27.1")
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.31.1")
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

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.27.1")
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

		It("should add missing elements to kube-controller-manager deployment", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.27.1")
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

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.27.1")
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

		It("should modify existing elements of kubelet.service unit options", func() {
			newUnitOptions := []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external`,
				},
				hostnamectlUnitOption,
			}

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, nil, nil, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})
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

		It("should modify existing elements of kubelet configuration", func() {
			newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
				ResolverConfig:               ptr.To("/etc/resolv-for-kubelet.conf"),
				EnableControllerAttachDetach: ptr.To(true),
			}

			kubeletConfig := *oldKubeletConfig
			err := ensurer.EnsureKubeletConfiguration(ctx, eContextK8s127, semver.MustParse("1.27.1"), &kubeletConfig, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(&kubeletConfig).To(Equal(newKubeletConfig))
		})
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
RestartForceExitStatus=10
RestartSec=15
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
			err := ensurer.EnsureAdditionalUnits(ctx, eContextK8s127, &units, nil)
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
			err := ensurer.EnsureAdditionalUnits(ctx, eContextK8s127WithResolvConfOptions, &units, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(units).To(ConsistOf(oldUnit, additionalPath, additionalUnit))
		})
	})

	Describe("#EnsureAdditionalFiles", func() {
		var (
			permissions uint32 = 0o755

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
			err := ensurer.EnsureAdditionalFiles(ctx, eContextK8s127, &files, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(files).To(ConsistOf(oldFile, additionalFileFunc(`""`)))
		})
		It("should add additional files to the current ones if resolvConfOptions field is set", func() {
			files := []extensionsv1alpha1.File{oldFile}

			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureAdditionalFiles method and check the result
			err := ensurer.EnsureAdditionalFiles(ctx, eContextK8s127WithResolvConfOptions, &files, nil)
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
			err := ensurer.EnsureAdditionalFiles(ctx, eContextK8s127WithResolvConfOptions, &files, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(files).To(ConsistOf(oldFile, additionalFile))
			Expect(files).To(HaveLen(2))
		})
	})

	Describe("#EnsureMachineControllerManagerDeployment", func() {
		var (
			ensurer    genericmutator.Ensurer
			deployment *appsv1.Deployment
			shoot      *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "foo"}}

			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{},
					},
				},
			}
		})

		BeforeEach(func() {
			ensurer = NewEnsurer(logger)
			DeferCleanup(testutils.WithVar(&ImageVector, imagevector.ImageVector{{
				Name:       "machine-controller-manager-provider-openstack",
				Repository: ptr.To("foo"),
				Tag:        ptr.To("bar"),
			}}))
		})

		It("should inject the sidecar container", func() {
			Expect(deployment.Spec.Template.Spec.Containers).To(BeEmpty())
			Expect(ensurer.EnsureMachineControllerManagerDeployment(context.TODO(), eContextK8s127, deployment, nil)).To(Succeed())
			expectedContainer := machinecontrollermanager.ProviderSidecarContainer(shoot, deployment.Namespace, "provider-openstack", "foo:bar")
			Expect(deployment.Spec.Template.Spec.Containers).To(ConsistOf(expectedContainer))
		})
	})

	Describe("#EnsureMachineControllerManagerVPA", func() {
		var (
			ensurer genericmutator.Ensurer
			vpa     *vpaautoscalingv1.VerticalPodAutoscaler
		)

		BeforeEach(func() {
			vpa = &vpaautoscalingv1.VerticalPodAutoscaler{}
		})

		BeforeEach(func() {
			ensurer = NewEnsurer(logger)
		})

		It("should inject the sidecar container policy", func() {
			Expect(vpa.Spec.ResourcePolicy).To(BeNil())
			Expect(ensurer.EnsureMachineControllerManagerVPA(context.TODO(), nil, vpa, nil)).To(Succeed())

			ccv := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
			Expect(vpa.Spec.ResourcePolicy.ContainerPolicies).To(ConsistOf(vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    "machine-controller-manager-provider-openstack",
				ControlledValues: &ccv,
			}))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).NotTo(ContainElement("--cloud-provider=openstack"))
	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	if !k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
	}
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, _ string) {
	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))

	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=openstack"))
	Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
	Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCACertificatesVolumeMount))
	Expect(dep.Spec.Template.Labels).To(BeEmpty())
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCACertificatesVolume))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
	Expect(dep.Spec.Template.Spec.Volumes).To(BeEmpty())
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
