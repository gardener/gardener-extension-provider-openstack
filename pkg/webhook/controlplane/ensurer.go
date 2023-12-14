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
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-openstack/imagevector"
	apisopenstack "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("openstack-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	logger logr.Logger
}

// ImageVector is exposed for testing.
var ImageVector = imagevector.ImageVector()

// EnsureMachineControllerManagerDeployment ensures that the machine-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerDeployment(_ context.Context, _ gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	image, err := ImageVector.FindImage(openstack.MachineControllerManagerProviderOpenStackImageName)
	if err != nil {
		return err
	}

	newObj.Spec.Template.Spec.Containers = extensionswebhook.EnsureContainerWithName(
		newObj.Spec.Template.Spec.Containers,
		machinecontrollermanager.ProviderSidecarContainer(newObj.Namespace, openstack.Name, image.String()),
	)
	return nil
}

// EnsureMachineControllerManagerVPA ensures that the machine-controller-manager VPA conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerVPA(_ context.Context, _ gcontext.GardenContext, newObj, _ *vpaautoscalingv1.VerticalPodAutoscaler) error {
	var (
		minAllowed = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}
		maxAllowed = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("5G"),
		}
	)

	if newObj.Spec.ResourcePolicy == nil {
		newObj.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{}
	}

	newObj.Spec.ResourcePolicy.ContainerPolicies = extensionswebhook.EnsureVPAContainerResourcePolicyWithName(
		newObj.Spec.ResourcePolicy.ContainerPolicies,
		machinecontrollermanager.ProviderSidecarVPAContainerPolicy(openstack.Name, minAllowed, maxAllowed),
	)
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, gctx gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	template := &newObj.Spec.Template
	ps := &template.Spec

	// TODO: This label approach is deprecated and no longer needed in the future. Remove it as soon as gardener/gardener@v1.75 has been released.
	metav1.SetMetaDataLabel(&newObj.Spec.Template.ObjectMeta, gutil.NetworkPolicyLabel(openstack.CSISnapshotValidationName, 443), v1beta1constants.LabelNetworkPolicyAllowed)

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		ensureKubeAPIServerCommandLineArgs(c, k8sVersion)
	}

	return nil
}

// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeControllerManagerDeployment(ctx context.Context, gctx gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	template := &newObj.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-controller-manager"); c != nil {
		ensureKubeControllerManagerCommandLineArgs(c, k8sVersion)
		ensureKubeControllerManagerVolumeMounts(c)
	}

	ensureKubeControllerManagerLabels(template)
	ensureKubeControllerManagerVolumes(ps)
	return nil
}

// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeSchedulerDeployment(ctx context.Context, gctx gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	template := &newObj.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-scheduler"); c != nil {
		ensureKubeSchedulerCommandLineArgs(c, k8sVersion)
	}
	return nil
}

// EnsureClusterAutoscalerDeployment ensures that the cluster-autoscaler deployment conforms to the provider requirements.
func (e *ensurer) EnsureClusterAutoscalerDeployment(ctx context.Context, gctx gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	template := &newObj.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "cluster-autoscaler"); c != nil {
		ensureClusterAutoscalerCommandLineArgs(c, k8sVersion)
	}
	return nil
}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	if versionutils.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
	}
	if versionutils.ConstraintK8sLess126.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginOpenStackUnregister"+"=true", ",")
	}
	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-provider=")
	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
	c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
		"PersistentVolumeLabel", ",")
	c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
		"PersistentVolumeLabel", ",")
}

func ensureKubeControllerManagerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "external")
	if versionutils.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
	}
	if versionutils.ConstraintK8sLess126.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginOpenStackUnregister"+"=true", ",")
	}
	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--external-cloud-volume-plugin=")
}

func ensureKubeSchedulerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	if versionutils.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
	}
	if versionutils.ConstraintK8sLess126.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginOpenStackUnregister"+"=true", ",")
	}
}

// ensureClusterAutoscalerCommandLineArgs ensures the cluster-autoscaler command line args.
func ensureClusterAutoscalerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	if versionutils.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
	}
	if versionutils.ConstraintK8sLess126.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginOpenStackUnregister"+"=true", ",")
	}
}

func ensureKubeControllerManagerLabels(t *corev1.PodTemplateSpec) {
	// TODO: This can be removed in a future version.
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToBlockedCIDRs)

	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks)
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks)
}

var (
	etcSSLName        = "etc-ssl"
	etcSSLVolumeMount = corev1.VolumeMount{
		Name:      etcSSLName,
		MountPath: "/etc/ssl",
		ReadOnly:  true,
	}
	directoryOrCreate = corev1.HostPathDirectoryOrCreate
	etcSSLVolume      = corev1.Volume{
		Name: etcSSLName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/etc/ssl",
				Type: &directoryOrCreate,
			},
		},
	}

	usrShareCACertificatesName        = "usr-share-ca-certificates"
	usrShareCACertificatesVolumeMount = corev1.VolumeMount{
		Name:      usrShareCACertificatesName,
		MountPath: "/usr/share/ca-certificates",
		ReadOnly:  true,
	}
	usrShareCACertificatesVolume = corev1.Volume{
		Name: usrShareCACertificatesName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/usr/share/ca-certificates",
			},
		},
	}
)

func ensureKubeControllerManagerVolumeMounts(c *corev1.Container) {
	c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount.Name)
	c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, usrShareCACertificatesVolumeMount.Name)
}

func ensureKubeControllerManagerVolumes(ps *corev1.PodSpec) {
	ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, etcSSLVolume.Name)
	ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, usrShareCACertificatesVolume.Name)
}

// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
func (e *ensurer) EnsureKubeletServiceUnitOptions(_ context.Context, _ gcontext.GardenContext, _ *semver.Version, newObj, _ []*unit.UnitOption) ([]*unit.UnitOption, error) {
	if opt := extensionswebhook.UnitOptionWithSectionAndName(newObj, "Service", "ExecStart"); opt != nil {
		command := extensionswebhook.DeserializeCommandLine(opt.Value)
		command = ensureKubeletCommandLineArgs(command)
		opt.Value = extensionswebhook.SerializeCommandLine(command, 1, " \\\n    ")
	}

	newObj = extensionswebhook.EnsureUnitOption(newObj, &unit.UnitOption{
		Section: "Service",
		Name:    "ExecStartPre",
		Value:   `/bin/sh -c 'hostnamectl set-hostname $(cat /etc/hostname | cut -d '.' -f 1)'`,
	})
	return newObj, nil
}

func ensureKubeletCommandLineArgs(command []string) []string {
	return extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "external")
}

// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ gcontext.GardenContext, kubeletVersion *semver.Version, newObj, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	if newObj.FeatureGates == nil {
		newObj.FeatureGates = make(map[string]bool)
	}

	if versionutils.ConstraintK8sLess127.Check(kubeletVersion) {
		newObj.FeatureGates["CSIMigration"] = true
	}
	if versionutils.ConstraintK8sLess126.Check(kubeletVersion) {
		newObj.FeatureGates["CSIMigrationOpenStack"] = true
		// kubelets of new worker nodes can directly be started with the <csiMigrationCompleteFeatureGate> feature gate
		newObj.FeatureGates["InTreePluginOpenStackUnregister"] = true
	}

	newObj.EnableControllerAttachDetach = pointer.Bool(true)

	// resolv-for-kubelet.conf is created by update-resolv-conf.service
	newObj.ResolverConfig = pointer.String("/etc/resolv-for-kubelet.conf")

	return nil
}

// EnsureAdditionalUnits ensures that additional required system units are added.
func (e *ensurer) EnsureAdditionalUnits(_ context.Context, _ gcontext.GardenContext, newObj, _ *[]extensionsv1alpha1.Unit) error {
	e.addAdditionalUnitsForResolvConfOptions(newObj)
	return nil
}

// addAdditionalUnitsForResolvConfOptions installs a systemd service to update `resolv-for-kubelet.conf`
// after each change of `/run/systemd/resolve/resolv.conf`.
func (e *ensurer) addAdditionalUnitsForResolvConfOptions(new *[]extensionsv1alpha1.Unit) {
	var (
		trueVar           = true
		customPathContent = `[Path]
PathChanged=/run/systemd/resolve/resolv.conf

[Install]
WantedBy=multi-user.target
`
		customUnitContent = `[Unit]
Description=update /etc/resolv-for-kubelet.conf on start and after each change of /run/systemd/resolve/resolv.conf
After=network.target
StartLimitIntervalSec=0

[Service]
Type=oneshot
ExecStart=/opt/bin/update-resolv-conf.sh
`
	)

	extensionswebhook.AppendUniqueUnit(new, extensionsv1alpha1.Unit{
		Name:    "update-resolv-conf.path",
		Enable:  &trueVar,
		Content: &customPathContent,
	})
	extensionswebhook.AppendUniqueUnit(new, extensionsv1alpha1.Unit{
		Name:    "update-resolv-conf.service",
		Enable:  &trueVar,
		Content: &customUnitContent,
	})
}

// EnsureAdditionalFiles ensures that additional required system files are added.
func (e *ensurer) EnsureAdditionalFiles(ctx context.Context, gctx gcontext.GardenContext, newObj, _ *[]extensionsv1alpha1.File) error {
	cloudProfileConfig, err := getCloudProfileConfig(ctx, gctx)
	if err != nil {
		return err
	}
	e.addAdditionalFilesForResolvConfOptions(getResolveConfOptions(cloudProfileConfig), newObj)
	return nil
}

// addAdditionalFilesForResolvConfOptions writes the script to update `/etc/resolv.conf` from
// `/run/systemd/resolve/resolv.conf` and adds an options line to it.
func (e *ensurer) addAdditionalFilesForResolvConfOptions(options []string, newObj *[]extensionsv1alpha1.File) {
	var (
		permissions int32 = 0o755
		template          = `#!/bin/sh

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
	)

	optionLine := ""
	if len(options) > 0 {
		optionLine = fmt.Sprintf("options %s", strings.Join(options, " "))
	}
	content := fmt.Sprintf(template, optionLine)
	file := extensionsv1alpha1.File{
		Path:        "/opt/bin/update-resolv-conf.sh",
		Permissions: &permissions,
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "",
				Data:     content,
			},
		},
	}
	appendUniqueFile(newObj, file)
}

func getCloudProfileConfig(ctx context.Context, gctx gcontext.GardenContext) (*apisopenstack.CloudProfileConfig, error) {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return nil, err
	}
	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}

func getResolveConfOptions(cloudProfileConfig *apisopenstack.CloudProfileConfig) []string {
	if cloudProfileConfig == nil {
		return nil
	}
	return cloudProfileConfig.ResolvConfOptions
}

// appendUniqueFile appends a unit file only if it does not exist, otherwise overwrite content of previous files
func appendUniqueFile(files *[]extensionsv1alpha1.File, file extensionsv1alpha1.File) {
	resFiles := make([]extensionsv1alpha1.File, 0, len(*files))

	for _, f := range *files {
		if f.Path != file.Path {
			resFiles = append(resFiles, f)
		}
	}

	*files = append(resFiles, file)
}
