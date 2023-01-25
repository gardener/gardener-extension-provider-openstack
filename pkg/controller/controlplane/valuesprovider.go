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
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	calicov1alpha1 "github.com/gardener/gardener-extension-networking-calico/pkg/apis/calico/v1alpha1"
	"github.com/gardener/gardener-extension-networking-calico/pkg/calico"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/utils"
)

const (
	caNameControlPlane               = "ca-" + openstack.Name + "-controlplane"
	cloudControllerManagerServerName = openstack.CloudControllerManagerName + "-server"
	csiSnapshotValidationServerName  = openstack.CSISnapshotValidationName + "-server"
)

func secretConfigsFunc(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       caNameControlPlane,
				CommonName: caNameControlPlane,
				CertType:   secretutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        cloudControllerManagerServerName,
				CommonName:                  openstack.CloudControllerManagerName,
				DNSNames:                    kutil.DNSNamesForService(openstack.CloudControllerManagerName, namespace),
				CertType:                    secretutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        csiSnapshotValidationServerName,
				CommonName:                  openstack.UsernamePrefix + openstack.CSISnapshotValidationName,
				DNSNames:                    kutil.DNSNamesForService(openstack.CSISnapshotValidationName, namespace),
				CertType:                    secretutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			// use current CA for signing server cert to prevent mismatches when dropping the old CA from the webhook
			// config in phase Completing
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane, secretsmanager.UseCurrentCA)},
		},
	}
}

func shootAccessSecretsFunc(namespace string) []*gutil.ShootAccessSecret {
	return []*gutil.ShootAccessSecret{
		gutil.NewShootAccessSecret(openstack.CloudControllerManagerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSIProvisionerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSIAttacherName, namespace),
		gutil.NewShootAccessSecret(openstack.CSISnapshotterName, namespace),
		gutil.NewShootAccessSecret(openstack.CSIResizerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSISnapshotControllerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSISnapshotValidationName, namespace),
	}
}

var (
	configChart = &chart.Chart{
		Name: openstack.CloudProviderConfigName,
		Path: filepath.Join(openstack.InternalChartsPath, openstack.CloudProviderConfigName),
		Objects: []*chart.Object{
			{Type: &corev1.Secret{}, Name: openstack.CloudProviderConfigName},
			{Type: &corev1.Secret{}, Name: openstack.CloudProviderDiskConfigName},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name: "seed-controlplane",
		Path: filepath.Join(openstack.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   openstack.CloudControllerManagerName,
				Images: []string{openstack.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: openstack.CloudControllerManagerName},
					{Type: &appsv1.Deployment{}, Name: openstack.CloudControllerManagerName},
					{Type: &corev1.ConfigMap{}, Name: openstack.CloudControllerManagerName + "-observability-config"},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: openstack.CloudControllerManagerName + "-vpa"},
				},
			},
			{
				Name: openstack.CSIControllerName,
				Images: []string{
					openstack.CSIDriverCinderImageName,
					openstack.CSIProvisionerImageName,
					openstack.CSIAttacherImageName,
					openstack.CSISnapshotterImageName,
					openstack.CSIResizerImageName,
					openstack.CSILivenessProbeImageName,
					openstack.CSISnapshotControllerImageName,
					openstack.CSISnapshotValidationWebhookImageName,
				},
				Objects: []*chart.Object{
					// csi-driver-controller
					{Type: &appsv1.Deployment{}, Name: openstack.CSIControllerName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: openstack.CSIControllerName + "-vpa"},
					{Type: &corev1.ConfigMap{}, Name: openstack.CSIControllerName + "-observability-config"},
					// csi-snapshot-controller
					{Type: &appsv1.Deployment{}, Name: openstack.CSISnapshotControllerName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: openstack.CSISnapshotControllerName + "-vpa"},
					// csi-snapshot-validation-webhook
					{Type: &appsv1.Deployment{}, Name: openstack.CSISnapshotValidationName},
					{Type: &corev1.Service{}, Name: openstack.CSISnapshotValidationName},
					{Type: &networkingv1.NetworkPolicy{}, Name: "allow-kube-apiserver-to-csi-snapshot-validation"},
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotValidationName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotValidationName},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name: "shoot-system-components",
		Path: filepath.Join(openstack.InternalChartsPath, "shoot-system-components"),
		SubCharts: []*chart.Chart{
			{
				Name: openstack.CloudControllerManagerName,
				Path: filepath.Join(openstack.InternalChartsPath, openstack.CloudControllerManagerName),
				Objects: []*chart.Object{
					{Type: &rbacv1.ClusterRole{}, Name: "system:controller:cloud-node-controller"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "system:controller:cloud-node-controller"},
				},
			},
			{
				Name: openstack.CSINodeName,
				Images: []string{
					openstack.CSIDriverCinderImageName,
					openstack.CSINodeDriverRegistrarImageName,
					openstack.CSILivenessProbeImageName,
				},
				Objects: []*chart.Object{
					// csi-driver
					{Type: &appsv1.DaemonSet{}, Name: openstack.CSINodeName},
					{Type: &storagev1.CSIDriver{}, Name: openstack.CSIStorageProvisioner},
					{Type: &corev1.ServiceAccount{}, Name: openstack.CSIDriverName},
					{Type: &corev1.Secret{}, Name: openstack.CloudProviderConfigName},
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIDriverName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIDriverName},
					{Type: &policyv1beta1.PodSecurityPolicy{}, Name: strings.Replace(openstack.UsernamePrefix+openstack.CSIDriverName, ":", ".", -1)},
					{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: openstack.CSINodeName},
					// csi-provisioner
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					// csi-attacher
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					// csi-snapshot-controller
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					// csi-snapshotter
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					// csi-resizer
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					// csi-snapshot-validation-webhook
					{Type: &admissionregistrationv1.ValidatingWebhookConfiguration{}, Name: openstack.CSISnapshotValidationName},
				},
			},
		},
	}

	controlPlaneShootCRDsChart = &chart.Chart{
		Name: "shoot-crds",
		Path: filepath.Join(openstack.InternalChartsPath, "shoot-crds"),
		SubCharts: []*chart.Chart{
			{
				Name: "volumesnapshots",
				Objects: []*chart.Object{
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshotclasses.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshotcontents.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshots.snapshot.storage.k8s.io"},
				},
			},
		},
	}

	storageClassChart = &chart.Chart{
		Name: "shoot-storageclasses",
		Path: filepath.Join(openstack.InternalChartsPath, "shoot-storageclasses"),
	}
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider(csi *config.CSI) genericactuator.ValuesProvider {
	return &valuesProvider{
		csi: csi,
	}
}

// valuesProvider is a ValuesProvider that provides OpenStack-specific values for the 2 charts applied by the generic actuator.
type valuesProvider struct {
	genericactuator.NoopValuesProvider
	common.ClientContext
	csi *config.CSI
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	cpConfig := &api.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	infraStatus := &api.InfrastructureStatus{}
	if _, _, err := vp.Decoder().Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	// Get credentials
	credentials, err := openstack.GetCredentials(ctx, vp.Client(), cp.Spec.SecretRef, false)
	if err != nil {
		return nil, fmt.Errorf("could not get service account from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	overlayEnabled, err := vp.isOverlayEnabled(cluster.Shoot.Spec.Networking)
	if err != nil {
		return nil, fmt.Errorf("could not determine overlay status: %v", err)
	}
	return getConfigChartValues(cpConfig, infraStatus, cloudProfileConfig, overlayEnabled, cp, credentials, cluster)
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
) (
	map[string]interface{},
	error,
) {
	// Decode providerConfig
	cpConfig := &api.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	cpConfigSecret := &corev1.Secret{}
	if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, openstack.CloudProviderConfigName), cpConfigSecret); err != nil {
		return nil, err
	}
	checksums[openstack.CloudProviderConfigName] = gardenerutils.ComputeChecksum(cpConfigSecret.Data)

	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		return nil, err
	}

	var userAgentHeaders []string
	if !k8sVersionLessThan119 {
		cpDiskConfigSecret := &corev1.Secret{}
		if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, openstack.CloudProviderCSIDiskConfigName), cpDiskConfigSecret); err != nil {
			return nil, err
		}
		checksums[openstack.CloudProviderCSIDiskConfigName] = gardenerutils.ComputeChecksum(cpDiskConfigSecret.Data)
		userAgentHeaders = vp.getUserAgentHeaders(ctx, cp, cluster)
	}

	return getControlPlaneChartValues(cpConfig, cp, cluster, secretsReader, userAgentHeaders, checksums, scaledDown, vp.csi)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
) (map[string]interface{}, error) {
	return vp.getControlPlaneShootChartValues(ctx, cp, cluster, secretsReader, checksums)
}

// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by the generic actuator.
// Currently the provider extension does not specify a control plane shoot CRDs chart. That's why we simply return empty values.
func (vp *valuesProvider) GetControlPlaneShootCRDsChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"volumesnapshots": map[string]interface{}{
			"enabled": !k8sVersionLessThan119,
		},
	}, nil
}

// GetStorageClassesChartValues returns the values for the shoot storageclasses chart applied by the generic actuator.
func (vp *valuesProvider) GetStorageClassesChartValues(
	_ context.Context,
	controlPlane *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		return nil, err
	}

	providerConfig := api.CloudProfileConfig{}
	if cluster.CloudProfile.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cluster.CloudProfile.Spec.ProviderConfig.Raw, nil, &providerConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(controlPlane), err)
		}
	}
	values := make(map[string]interface{})
	if providerConfig.StorageClasses != nil && len(providerConfig.StorageClasses) != 0 {
		allSc := make([]map[string]interface{}, len(providerConfig.StorageClasses))
		for i, sc := range providerConfig.StorageClasses {
			var storageClassValues = map[string]interface{}{
				"name": sc.Name,
			}

			if sc.Default != nil && *sc.Default {
				storageClassValues["default"] = true
			}

			if len(sc.Annotations) != 0 {
				storageClassValues["annotations"] = sc.Annotations
			}
			if len(sc.Labels) != 0 {
				storageClassValues["labels"] = sc.Labels
			}
			if len(sc.Parameters) != 0 {
				storageClassValues["parameters"] = sc.Parameters
			}

			storageClassValues["provisioner"] = openstack.CSIStorageProvisioner
			if sc.Provisioner != nil && *sc.Provisioner != "" {
				storageClassValues["provisioner"] = sc.Provisioner
			}

			if sc.ReclaimPolicy != nil && *sc.ReclaimPolicy != "" {
				storageClassValues["reclaimPolicy"] = sc.ReclaimPolicy
			}

			if sc.VolumeBindingMode != nil && *sc.VolumeBindingMode != "" {
				storageClassValues["volumeBindingMode"] = sc.VolumeBindingMode
			}

			allSc[i] = storageClassValues
		}
		values["storageclasses"] = allSc
		return values, err
	}

	storageclasses := []map[string]interface{}{
		{
			"name":              "default",
			"default":           true,
			"provisioner":       openstack.CSIStorageProvisioner,
			"volumeBindingMode": storagev1.VolumeBindingWaitForFirstConsumer,
		},
		{
			"name":              "default-class",
			"provisioner":       openstack.CSIStorageProvisioner,
			"volumeBindingMode": storagev1.VolumeBindingWaitForFirstConsumer,
		}}

	if k8sVersionLessThan119 {
		storageclasses[0]["provisioner"] = openstack.StorageProvisionerBeforeCSI
		storageclasses[1]["provisioner"] = openstack.StorageProvisionerBeforeCSI
	}
	values["storageclasses"] = storageclasses

	return values, nil
}

func (vp *valuesProvider) getUserAgentHeaders(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) []string {
	headers := []string{}

	// Add the domain and project/tenant to the useragent headers if the secret
	// could be read and the respective fields in secret are not empty.
	if credentials, err := openstack.GetCredentials(ctx, vp.Client(), cp.Spec.SecretRef, false); err == nil && credentials != nil {
		if credentials.DomainName != "" {
			headers = append(headers, credentials.DomainName)
		}
		if credentials.TenantName != "" {
			headers = append(headers, credentials.TenantName)
		}
	}

	if cluster.Shoot != nil {
		headers = append(headers, cluster.Shoot.Status.TechnicalID)
	}

	return headers
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(
	cpConfig *api.ControlPlaneConfig,
	infraStatus *api.InfrastructureStatus,
	cloudProfileConfig *api.CloudProfileConfig,
	isUsingOverlay bool,
	cp *extensionsv1alpha1.ControlPlane,
	c *openstack.Credentials,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	subnet, err := helper.FindSubnetByPurpose(infraStatus.Networks.Subnets, api.PurposeNodes)
	if err != nil {
		return nil, fmt.Errorf("could not determine subnet from infrastructureProviderStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil - cannot determine keystone URL and other parameters")
	}

	keyStoneURL, err := helper.FindKeyStoneURL(cloudProfileConfig.KeyStoneURLs, cloudProfileConfig.KeyStoneURL, cp.Spec.Region)
	if err != nil {
		return nil, err
	}

	values := map[string]interface{}{
		"kubernetesVersion":           cluster.Shoot.Spec.Kubernetes.Version,
		"domainName":                  c.DomainName,
		"tenantName":                  c.TenantName,
		"username":                    c.Username,
		"password":                    c.Password,
		"applicationCredentialID":     c.ApplicationCredentialID,
		"applicationCredentialName":   c.ApplicationCredentialName,
		"applicationCredentialSecret": c.ApplicationCredentialSecret,
		"region":                      cp.Spec.Region,
		"lbProvider":                  cpConfig.LoadBalancerProvider,
		"floatingNetworkID":           infraStatus.Networks.FloatingPool.ID,
		"subnetID":                    subnet.ID,
		"authUrl":                     keyStoneURL,
		"dhcpDomain":                  cloudProfileConfig.DHCPDomain,
		"requestTimeout":              cloudProfileConfig.RequestTimeout,
		"useOctavia":                  cloudProfileConfig.UseOctavia != nil && *cloudProfileConfig.UseOctavia,
		"rescanBlockStorageOnResize":  cloudProfileConfig.RescanBlockStorageOnResize != nil && *cloudProfileConfig.RescanBlockStorageOnResize,
		"ignoreVolumeAZ":              cloudProfileConfig.IgnoreVolumeAZ != nil && *cloudProfileConfig.IgnoreVolumeAZ,
		"nodeVolumeAttachLimit":       cloudProfileConfig.NodeVolumeAttachLimit,
		// detect internal network.
		// See https://github.com/kubernetes/cloud-provider-openstack/blob/v1.22.1/docs/openstack-cloud-controller-manager/using-openstack-cloud-controller-manager.md#networking
		"internalNetworkName": infraStatus.Networks.Name,
	}

	if !isUsingOverlay {
		values["routerID"] = infraStatus.Networks.Router.ID
	}

	loadBalancerClassesFromCloudProfile := []api.LoadBalancerClass{}
	if floatingPool, err := helper.FindFloatingPool(cloudProfileConfig.Constraints.FloatingPools, infraStatus.Networks.FloatingPool.Name, cp.Spec.Region, nil); err == nil {
		loadBalancerClassesFromCloudProfile = floatingPool.LoadBalancerClasses
	}

	// The LoadBalancerClasses from the CloudProfile will be configured by default.
	// In case the user specifies own LoadBalancerClasses via via the ControlPlaneConfig
	// then the ones from the CloudProfile will be overridden.
	loadBalancerClasses := loadBalancerClassesFromCloudProfile
	if cpConfig.LoadBalancerClasses != nil {
		loadBalancerClasses = cpConfig.LoadBalancerClasses
	}

	// If a private LoadBalancerClass is provided then set its configuration for
	// the global loadbalancer configuration in the cloudprovider config.
	if privateLoadBalancerClass := lookupLoadBalancerClass(loadBalancerClasses, api.PrivateLoadBalancerClass); privateLoadBalancerClass != nil {
		utils.SetStringValue(values, "subnetID", privateLoadBalancerClass.SubnetID)
	}

	// If a default LoadBalancerClass is provided then set its configuration for
	// the global loadbalancer configuration in the cloudprovider config.
	if defaultLoadBalancerClass := lookupLoadBalancerClass(loadBalancerClasses, api.DefaultLoadBalancerClass); defaultLoadBalancerClass != nil {
		utils.SetStringValue(values, "floatingNetworkID", defaultLoadBalancerClass.FloatingNetworkID)
		utils.SetStringValue(values, "floatingSubnetID", defaultLoadBalancerClass.FloatingSubnetID)
		utils.SetStringValue(values, "floatingSubnetName", defaultLoadBalancerClass.FloatingSubnetName)
		utils.SetStringValue(values, "floatingSubnetTags", defaultLoadBalancerClass.FloatingSubnetTags)
		utils.SetStringValue(values, "subnetID", defaultLoadBalancerClass.SubnetID)
	}

	// Check if there is a dedicated vpn LoadBalancerClass in the CloudProfile and
	// add its to the list of available LoadBalancerClasses.
	if vpnLoadBalancerClass := lookupLoadBalancerClass(loadBalancerClassesFromCloudProfile, api.VPNLoadBalancerClass); vpnLoadBalancerClass != nil {
		loadBalancerClasses = append(loadBalancerClasses, *vpnLoadBalancerClass)
	}

	if loadBalancerClassValues := generateLoadBalancerClassValues(loadBalancerClasses, infraStatus); len(loadBalancerClassValues) > 0 {
		values["floatingClasses"] = loadBalancerClassValues
	}

	return values, nil
}

func generateLoadBalancerClassValues(lbClasses []api.LoadBalancerClass, infrastructureStatus *api.InfrastructureStatus) []map[string]interface{} {
	loadBalancerClassValues := []map[string]interface{}{}

	for _, lbClass := range lbClasses {
		values := map[string]interface{}{"name": lbClass.Name}

		utils.SetStringValue(values, "floatingNetworkID", lbClass.FloatingNetworkID)
		if !utils.IsEmptyString(lbClass.FloatingNetworkID) && infrastructureStatus.Networks.FloatingPool.ID != "" {
			values["floatingNetworkID"] = infrastructureStatus.Networks.FloatingPool.ID
		}
		utils.SetStringValue(values, "floatingSubnetID", lbClass.FloatingSubnetID)
		utils.SetStringValue(values, "floatingSubnetName", lbClass.FloatingSubnetName)
		utils.SetStringValue(values, "floatingSubnetTags", lbClass.FloatingSubnetTags)
		utils.SetStringValue(values, "subnetID", lbClass.SubnetID)

		loadBalancerClassValues = append(loadBalancerClassValues, values)
	}

	return loadBalancerClassValues
}

func lookupLoadBalancerClass(lbClasses []api.LoadBalancerClass, lbClassPurpose string) *api.LoadBalancerClass {
	var firstLoadBalancerClass *api.LoadBalancerClass

	// First: Check if the requested LoadBalancerClass can be looked up by purpose.
	for i, class := range lbClasses {
		if i == 0 {
			classRef := &class
			firstLoadBalancerClass = classRef
		}

		if class.Purpose != nil && *class.Purpose == lbClassPurpose {
			return &class
		}
	}

	// The vpn class can only be selected by purpose and not by name.
	if lbClassPurpose == api.VPNLoadBalancerClass {
		return nil
	}

	// Second: Check if the requested LoadBalancerClass can be looked up by name.
	for _, class := range lbClasses {
		if class.Name == lbClassPurpose {
			return &class
		}
	}

	// If a default LoadBalancerClass was requested, but not found then the first
	// configured one will be trated as default LoadBalancerClass.
	if lbClassPurpose == api.DefaultLoadBalancerClass && firstLoadBalancerClass != nil {
		return firstLoadBalancerClass
	}

	return nil
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func getControlPlaneChartValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	userAgentHeaders []string,
	checksums map[string]string,
	scaledDown bool,
	csi *config.CSI,
) (
	map[string]interface{},
	error,
) {
	ccm, err := getCCMChartValues(cpConfig, cp, cluster, secretsReader, userAgentHeaders, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	csiChart, err := getCSIControllerChartValues(cluster, secretsReader, userAgentHeaders, checksums, scaledDown, csi)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"genericTokenKubeconfigSecretName": extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster),
		},
		openstack.CloudControllerManagerName: ccm,
		openstack.CSIControllerName:          csiChart,
	}, nil
}

// getCCMChartValues collects and returns the CCM chart values.
func getCCMChartValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	userAgentHeaders []string,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	kubeVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}

	serverSecret, found := secretsReader.Get(cloudControllerManagerServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", cloudControllerManagerServerName)
	}

	values := map[string]interface{}{
		"enabled":           true,
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        extensionscontroller.GetPodNetwork(cluster),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/secret-" + openstack.CloudProviderConfigName:        checksums[openstack.CloudProviderConfigName],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
		"tlsCipherSuites": kutil.TLSCipherSuites(kubeVersion),
		"secrets": map[string]interface{}{
			"server": serverSecret.Name,
		},
	}

	if userAgentHeaders != nil {
		values["userAgentHeaders"] = userAgentHeaders
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	return values, nil
}

// getCSIControllerChartValues collects and returns the CSIController chart values.
func getCSIControllerChartValues(
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	userAgentHeaders []string,
	checksums map[string]string,
	scaledDown bool,
	csi *config.CSI,
) (map[string]interface{}, error) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		return nil, err
	}

	if k8sVersionLessThan119 {
		return map[string]interface{}{"enabled": false}, nil
	}

	serverSecret, found := secretsReader.Get(csiSnapshotValidationServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", csiSnapshotValidationServerName)
	}

	values := map[string]interface{}{
		"enabled":  true,
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
		},
		"csiSnapshotController": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		},
		"csiSnapshotValidationWebhook": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
			"secrets": map[string]interface{}{
				"server": serverSecret.Name,
			},
		},
	}

	if csi != nil {
		csiValues := make(map[string]interface{})
		if csi.CSIAttacher != nil {
			csiValues["attacher"] = makeCSIExtraArgs(getCSIAttacherArgs(csi.CSIAttacher))
		}
		values["csiDeployment"] = csiValues
	}

	if userAgentHeaders != nil {
		values["userAgentHeaders"] = userAgentHeaders
	}
	return values, nil
}

func makeCSIExtraArgs(args ...map[string]interface{}) map[string]interface{} {
	csiExtraArgs := make(map[string]interface{})

	for _, argList := range args {
		if val, ok := csiExtraArgs["extraArgs"]; ok {
			csiExtraArgs["extraArgs"] = gardenerutils.MergeMaps(val.(map[string]interface{}), argList)
		} else {
			csiExtraArgs["extraArgs"] = argList
		}
	}

	return csiExtraArgs
}

func getCSIAttacherArgs(attacher *config.CSIAttacher) map[string]interface{} {
	csiAttacherArgs := make(map[string]interface{})
	if attacher.RetryIntervalStart != nil {
		csiAttacherArgs["retry-interval-start"] = attacher.RetryIntervalStart
	}
	if attacher.ReconcileSync != nil {
		csiAttacherArgs["reconcile-sync"] = attacher.ReconcileSync
	}
	if attacher.RetryIntervalMax != nil {
		csiAttacherArgs["retry-interval-max"] = attacher.RetryIntervalMax
	}
	if attacher.Verbosity != nil {
		csiAttacherArgs["v"] = attacher.Verbosity
	}
	if attacher.Timeout != nil {
		csiAttacherArgs["timeout"] = attacher.Timeout
	}

	return csiAttacherArgs
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func (vp *valuesProvider) getControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
) (
	map[string]interface{},
	error,
) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		return nil, err
	}

	var (
		cloudProviderDiskConfig []byte
		userAgentHeader         []string
		csiNodeDriverValues     map[string]interface{}
		caBundle                string
	)

	if !k8sVersionLessThan119 {
		secret := &corev1.Secret{}
		if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, openstack.CloudProviderCSIDiskConfigName), secret); err != nil {
			return nil, err
		}

		cloudProviderDiskConfig = secret.Data[openstack.CloudProviderConfigDataKey]
		checksums[openstack.CloudProviderCSIDiskConfigName] = gardenerutils.ComputeChecksum(secret.Data)
		userAgentHeader = vp.getUserAgentHeaders(ctx, cp, cluster)

		caSecret, found := secretsReader.Get(caNameControlPlane)
		if !found {
			return nil, fmt.Errorf("secret %q not found", caNameControlPlane)
		}
		caBundle = string(caSecret.Data[secretutils.DataKeyCertificateBundle])
	}

	csiNodeDriverValues = map[string]interface{}{
		"enabled":           !k8sVersionLessThan119,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"vpaEnabled":        gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
		},
		"cloudProviderConfig": cloudProviderDiskConfig,
		"webhookConfig": map[string]interface{}{
			"url":      "https://" + openstack.CSISnapshotValidationName + "." + cp.Namespace + "/volumesnapshot",
			"caBundle": caBundle,
		},
		"pspDisabled": gardencorev1beta1helper.IsPSPDisabled(cluster.Shoot),
	}
	if userAgentHeader != nil {
		csiNodeDriverValues["userAgentHeaders"] = userAgentHeader
	}

	return map[string]interface{}{
		openstack.CloudControllerManagerName: map[string]interface{}{"enabled": true},
		openstack.CSINodeName:                csiNodeDriverValues,
	}, nil
}

func (vp *valuesProvider) isOverlayEnabled(network v1beta1.Networking) (bool, error) {
	if network.ProviderConfig == nil {
		return true, nil
	}

	// should not happen in practice because we will receive a RawExtension with Raw populated in production.
	networkProviderConfig, err := network.ProviderConfig.MarshalJSON()
	if err != nil {
		return false, err
	}
	if string(networkProviderConfig) == "null" {
		return true, nil
	}

	switch network.Type {
	case calico.ReleaseName:
		networkConfig := &calicov1alpha1.NetworkConfig{}
		if _, _, err := vp.Decoder().Decode(networkProviderConfig, nil, networkConfig); err != nil {
			return false, err
		}
		o := networkConfig.Overlay
		if o == nil {
			return true, nil
		}
		return o.Enabled, nil
	}

	return true, nil
}
