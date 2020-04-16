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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	"github.com/coreos/go-systemd/unit"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const csiMigrationVersion = "1.19"

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("openstack-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	client client.Client
	logger logr.Logger
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		ensureKubeAPIServerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
		ensureVolumeMounts(c, csiEnabled, csiMigrationComplete)
	}

	ensureVolumes(ps, csiEnabled, csiMigrationComplete)
	return e.ensureChecksumAnnotations(ctx, &new.Spec.Template, new.Namespace, csiEnabled, csiMigrationComplete)
}

// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeControllerManagerDeployment(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-controller-manager"); c != nil {
		ensureKubeControllerManagerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
		ensureVolumeMounts(c, csiEnabled, csiMigrationComplete)
	}

	ensureKubeControllerManagerLabels(template, csiEnabled, csiMigrationComplete)
	ensureVolumes(ps, csiEnabled, csiMigrationComplete)
	return e.ensureChecksumAnnotations(ctx, &new.Spec.Template, new.Namespace, csiEnabled, csiMigrationComplete)
}

// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeSchedulerDeployment(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-scheduler"); c != nil {
		ensureKubeSchedulerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
	}
	return nil
}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"CSIMigrationOpenStackComplete=true", ",")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-provider=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
				"PersistentVolumeLabel", ",")
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
				"PersistentVolumeLabel", ",")
			return
		}
	}

	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "openstack")
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-config=",
		"/etc/kubernetes/cloudprovider/cloudprovider.conf")
	c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
		"PersistentVolumeLabel", ",")
	c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
		"PersistentVolumeLabel", ",")
}

func ensureKubeControllerManagerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "external")

	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"CSIMigrationOpenStackComplete=true", ",")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--external-cloud-volume-plugin=")
			return
		}
	}

	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-config=",
		"/etc/kubernetes/cloudprovider/cloudprovider.conf")
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--external-cloud-volume-plugin=", "openstack")
}

func ensureKubeSchedulerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationOpenStack=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"CSIMigrationOpenStackComplete=true", ",")
			return
		}
	}
}

func ensureKubeControllerManagerLabels(t *corev1.PodTemplateSpec, csiEnabled, csiMigrationComplete bool) {
	// TODO: This can be removed in a future version.
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToBlockedCIDRs)

	if csiEnabled && csiMigrationComplete {
		delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks)
		delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks)
		return
	}

	t.Labels = extensionswebhook.EnsureAnnotationOrLabel(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks, v1beta1constants.LabelNetworkPolicyAllowed)
	t.Labels = extensionswebhook.EnsureAnnotationOrLabel(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks, v1beta1constants.LabelNetworkPolicyAllowed)
}

var (
	etcSSLName        = "etc-ssl"
	etcSSLVolumeMount = corev1.VolumeMount{
		Name:      etcSSLName,
		MountPath: "/etc/ssl",
		ReadOnly:  true,
	}
	etcSSLVolume = corev1.Volume{
		Name: etcSSLName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/etc/ssl",
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

	cloudProviderDiskConfigVolumeMount = corev1.VolumeMount{
		Name:      openstack.CloudProviderDiskConfigName,
		MountPath: "/etc/kubernetes/cloudprovider",
	}
	cloudProviderDiskConfigVolume = corev1.Volume{
		Name: openstack.CloudProviderDiskConfigName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: openstack.CloudProviderDiskConfigName,
			},
		},
	}
)

func ensureVolumeMounts(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderDiskConfigVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, usrShareCACertificatesVolumeMount.Name)
		return
	}

	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderDiskConfigVolumeMount)
	// Host certificates are mounted to accommodate OpenStack endpoints that might be served with a certificate
	// signed by a CA that is not globally trusted.
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount)
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, usrShareCACertificatesVolumeMount)
}

func ensureVolumes(ps *corev1.PodSpec, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderDiskConfigVolumeMount.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, etcSSLVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, usrShareCACertificatesVolume.Name)
		return
	}

	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderDiskConfigVolume)
	// Host certificates are mounted to accommodate OpenStack endpoints that might be served with a certificate
	// signed by a CA that is not globally trusted.
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, etcSSLVolume)
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, usrShareCACertificatesVolume)
}

func (e *ensurer) ensureChecksumAnnotations(ctx context.Context, template *corev1.PodTemplateSpec, namespace string, csiEnabled, csiMigrationComplete bool) error {
	if csiEnabled && csiMigrationComplete {
		delete(template.Annotations, "checksum/secret-"+v1beta1constants.SecretNameCloudProvider)
		delete(template.Annotations, "checksum/secret-"+openstack.CloudProviderConfigName)
		return nil
	}

	return controlplane.EnsureSecretChecksumAnnotation(ctx, template, e.client, namespace, openstack.CloudProviderConfigName)
}

// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
func (e *ensurer) EnsureKubeletServiceUnitOptions(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ []*unit.UnitOption) ([]*unit.UnitOption, error) {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return nil, err
	}

	if opt := extensionswebhook.UnitOptionWithSectionAndName(new, "Service", "ExecStart"); opt != nil {
		command := extensionswebhook.DeserializeCommandLine(opt.Value)
		command = ensureKubeletCommandLineArgs(command, csiEnabled)
		opt.Value = extensionswebhook.SerializeCommandLine(command, 1, " \\\n    ")
	}

	new = extensionswebhook.EnsureUnitOption(new, &unit.UnitOption{
		Section: "Service",
		Name:    "ExecStartPre",
		Value:   `/bin/sh -c 'hostnamectl set-hostname $(cat /etc/hostname | cut -d '.' -f 1)'`,
	})
	return new, nil
}

func ensureKubeletCommandLineArgs(command []string, csiEnabled bool) []string {
	if csiEnabled {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "external")
		command = extensionswebhook.EnsureStringWithPrefix(command, "--enable-controller-attach-detach=", "true")
	} else {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "openstack")
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-config=", "/var/lib/kubelet/cloudprovider.conf")
	}
	return command
}

// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
func (e *ensurer) EnsureKubeletConfiguration(ctx context.Context, ectx genericmutator.EnsurerContext, new, old *kubeletconfigv1beta1.KubeletConfiguration) error {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if csiEnabled {
		if new.FeatureGates == nil {
			new.FeatureGates = make(map[string]bool)
		}

		new.FeatureGates["CSIMigration"] = true
		new.FeatureGates["CSIMigrationOpenStack"] = true
		// kubelets of new worker nodes can directly be started with the the `CSIMigrationOpenStackComplete` feature gate
		new.FeatureGates["CSIMigrationOpenStackComplete"] = true
	}

	return nil
}

// ShouldProvisionKubeletCloudProviderConfig returns true if the cloud provider config file should be added to the kubelet configuration.
func (e *ensurer) ShouldProvisionKubeletCloudProviderConfig(ctx context.Context, ectx genericmutator.EnsurerContext) bool {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return false
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return false
	}

	return !csiEnabled
}

// EnsureKubeletCloudProviderConfig ensures that the cloud provider config file conforms to the provider requirements.
func (e *ensurer) EnsureKubeletCloudProviderConfig(ctx context.Context, ectx genericmutator.EnsurerContext, data *string, namespace string) error {
	secret := corev1.Secret{}
	if err := e.client.Get(ctx, kutil.Key(namespace, openstack.CloudProviderDiskConfigName), &secret); err != nil {
		if apierrors.IsNotFound(err) {
			e.logger.Info("configmap not found", "name", openstack.CloudProviderDiskConfigName, "namespace", namespace)
			return nil
		}
		return errors.Wrapf(err, "could not get secret '%s/%s'", namespace, openstack.CloudProviderDiskConfigName)
	}

	if secret.Data == nil || secret.Data[openstack.CloudProviderConfigDataKey] == nil {
		return nil
	}

	*data = string(secret.Data[openstack.CloudProviderConfigDataKey])
	return nil
}
