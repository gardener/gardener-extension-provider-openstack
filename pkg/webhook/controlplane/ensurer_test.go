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
	"testing"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	"github.com/coreos/go-systemd/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const namespace = "test"

var cloudProviderConfigContent = []byte("[Global]\nauth-url: https://path-to-keystone:5000/v3/\n")

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctx = context.TODO()

		ctrl *gomock.Controller

		c       *mockclient.MockClient
		ensurer genericmutator.Ensurer

		dummyContext   = genericmutator.NewEnsurerContext(nil, nil)
		eContextK8s116 = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.16.0",
						},
					},
				},
			},
		)
		eContextK8s119 = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.19.0",
						},
					},
				},
			},
		)
		eContextK8s119WithCSIAnnotation = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						csimigration.AnnotationKeyNeedsComplete: "true",
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.19.0",
						},
					},
				},
			},
		)

		cpConfigSecretKey     = client.ObjectKey{Namespace: namespace, Name: openstack.CloudProviderConfigName}
		cpDiskConfigSecretKey = client.ObjectKey{Namespace: namespace, Name: openstack.CloudProviderDiskConfigName}
		cpConfigSecret        = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: openstack.CloudProviderConfigName},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"abc": []byte("xyz"), openstack.CloudProviderConfigDataKey: cloudProviderConfigContent},
		}

		annotations = map[string]string{
			"checksum/secret-" + openstack.CloudProviderConfigName: "ceeecafb44c8f5c2eb4ef77d08b85c04facb083a6b1571164d2ec3857c0bc1b4",
		}

		kubeControllerManagerLabels = map[string]string{
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		ensurer = NewEnsurer(logger)
		err := ensurer.(inject.Client).InjectClient(c)
		Expect(err).To(Not(HaveOccurred()))
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

		It("should add missing elements to kube-apiserver deployment (k8s < 1.17)", func() {
			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.16.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.19)", func() {
			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s119, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.19.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.19 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s119WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, nil, "1.19.0", true)
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

			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.16.0", false)
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

		It("should add missing elements to kube-controller-manager deployment (k8s < 1.17)", func() {
			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.16.0", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.19 w/o CSI annotation)", func() {
			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s119, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.19.0", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.19 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s119WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, nil, nil, "1.19.0", true)
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
										{Name: openstack.CloudProviderDiskConfigName, MountPath: "?"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{Name: openstack.CloudProviderDiskConfigName},
							},
						},
					},
				},
			}

			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.16.0", false)
		})
	})

	Describe("#EnsureKubeSchedulerDeployment", func() {
		var (
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
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

		It("should add missing elements to kube-scheduler deployment (k8s < 1.19)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.17.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.19 w/o CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s119, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.19.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.19 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s119WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.19.0", true)
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

		It("should modify existing elements of kubelet.service unit options (k8s < 1.19)", func() {
			newUnitOptions := []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=openstack \
    --cloud-config=/var/lib/kubelet/cloudprovider.conf`,
				},
				hostnamectlUnitOption,
			}

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, eContextK8s116, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})

		It("should modify existing elements of kubelet.service unit options (k8s >= 1.19)", func() {
			newUnitOptions := []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external \
    --enable-controller-attach-detach=true`,
				},
				hostnamectlUnitOption,
			}

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, eContextK8s119, oldUnitOptions, nil)
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

		It("should modify existing elements of kubelet configuration (k8s < 1.19)", func() {
			newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
			}
			kubeletConfig := *oldKubeletConfig

			err := ensurer.EnsureKubeletConfiguration(ctx, eContextK8s116, &kubeletConfig, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(&kubeletConfig).To(Equal(newKubeletConfig))
		})

		It("should modify existing elements of kubelet configuration (k8s >= 1.19)", func() {
			newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo":                           true,
					"CSIMigration":                  true,
					"CSIMigrationOpenStack":         true,
					"CSIMigrationOpenStackComplete": true,
				},
			}
			kubeletConfig := *oldKubeletConfig

			err := ensurer.EnsureKubeletConfiguration(ctx, eContextK8s119, &kubeletConfig, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(&kubeletConfig).To(Equal(newKubeletConfig))
		})
	})

	Describe("#ShouldProvisionKubeletCloudProviderConfig", func() {
		It("should return true (k8s < 1.19)", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s116)).To(BeTrue())
		})

		It("should return false (k8s >= 1.19)", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s119)).To(BeFalse())
		})
	})

	Describe("#EnsureKubeletCloudProviderConfig", func() {
		var (
			existingData = util.StringPtr("[LoadBalancer]\nlb-version=v2\nlb-provider:\n")
			emptydata    = util.StringPtr("")
		)

		It("cloud provider config secret does not exist", func() {
			c.EXPECT().Get(ctx, cpDiskConfigSecretKey, &corev1.Secret{}).Return(errors.NewNotFound(schema.GroupResource{}, cpConfigSecret.Name))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(""))
		})

		It("should create element containing cloud provider config content", func() {
			c.EXPECT().Get(ctx, cpDiskConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(string(cloudProviderConfigContent)))
		})

		It("should modify existing element containing cloud provider config content", func() {
			c.EXPECT().Get(ctx, cpDiskConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, existingData, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*existingData).To(Equal(string(cloudProviderConfigContent)))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, annotations map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionAtLeast119, _ := version.CompareVersions(k8sVersion, ">=", "1.19")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-provider=openstack"))
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(Not(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ",")))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderDiskConfigVolumeMount))
		Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
		Expect(c.VolumeMounts).To(ContainElement(usrShareCACertificatesVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderDiskConfigVolume))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(usrShareCACertificatesVolume))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		if k8sVersionAtLeast119 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true"))
		}
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-provider=openstack"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderDiskConfigVolumeMount))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCACertificatesVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderDiskConfigVolume))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCACertificatesVolume))
		Expect(dep.Spec.Template.Annotations).To(BeNil())
	}
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, annotations, labels map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionAtLeast119, _ := version.CompareVersions(k8sVersion, ">=", "1.19")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(ContainElement("--external-cloud-volume-plugin=openstack"))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderDiskConfigVolumeMount))
		Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
		Expect(c.VolumeMounts).To(ContainElement(usrShareCACertificatesVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderDiskConfigVolume))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(usrShareCACertificatesVolume))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(dep.Spec.Template.Labels).To(Equal(labels))
		if k8sVersionAtLeast119 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true"))
		}
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=openstack"))
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderDiskConfigVolumeMount))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCACertificatesVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderDiskConfigVolume))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCACertificatesVolume))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
		Expect(dep.Spec.Template.Labels).To(BeNil())
		Expect(dep.Spec.Template.Spec.Volumes).To(BeNil())
	}
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	if k8sVersionAtLeast119, _ := version.CompareVersions(k8sVersion, ">=", "1.19"); !k8sVersionAtLeast119 {
		return
	}

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationOpenStack=true,CSIMigrationOpenStackComplete=true"))
	}
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
