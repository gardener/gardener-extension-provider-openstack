// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/charts"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
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

func shootAccessSecretsFunc(namespace string) []*gutil.AccessSecret {
	return []*gutil.AccessSecret{
		gutil.NewShootAccessSecret(openstack.CloudControllerManagerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSIProvisionerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSIAttacherName, namespace),
		gutil.NewShootAccessSecret(openstack.CSISnapshotterName, namespace),
		gutil.NewShootAccessSecret(openstack.CSIResizerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSISnapshotControllerName, namespace),
		gutil.NewShootAccessSecret(openstack.CSISnapshotValidationName, namespace),
	}
}

func makeUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return obj
}

var (
	configChart = &chart.Chart{
		Name:       openstack.CloudProviderConfigName,
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, openstack.CloudProviderConfigName),
		Objects: []*chart.Object{
			{Type: &corev1.Secret{}, Name: openstack.CloudProviderConfigName},
			{Type: &corev1.Secret{}, Name: openstack.CloudProviderDiskConfigName},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name:       "seed-controlplane",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   openstack.CloudControllerManagerName,
				Images: []string{openstack.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: openstack.CloudControllerManagerName},
					{Type: &appsv1.Deployment{}, Name: openstack.CloudControllerManagerName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: openstack.CloudControllerManagerName + "-vpa"},
					{Type: &monitoringv1.ServiceMonitor{}, Name: "shoot-cloud-controller-manager"},
					{Type: &monitoringv1.PrometheusRule{}, Name: "shoot-cloud-controller-manager"},
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
				},
			},
			{
				Name: openstack.CSIDriverManilaController,
				Images: []string{
					openstack.CSIDriverManilaImageName,
					openstack.CSIDriverNFSImageName,
					openstack.CSIProvisionerImageName,
					openstack.CSISnapshotterImageName,
					openstack.CSIResizerImageName,
					openstack.CSILivenessProbeImageName,
				},
				Objects: []*chart.Object{
					// csi-driver-manila-controller
					{Type: &appsv1.Deployment{}, Name: openstack.CSIDriverManilaController},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: openstack.CSIDriverManilaController},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name:       "shoot-system-components",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-system-components"),
		SubCharts: []*chart.Chart{
			{
				Name: openstack.CloudControllerManagerName,
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
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotValidationName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotValidationName},
				},
			},
			{
				Name: openstack.CSIDriverManila,
				Images: []string{
					openstack.CSIDriverManilaImageName,
					openstack.CSIDriverNFSImageName,
					openstack.CSINodeDriverRegistrarImageName,
					openstack.CSILivenessProbeImageName,
				},
				Objects: []*chart.Object{
					{Type: &storagev1.CSIDriver{}, Name: openstack.CSIManilaStorageProvisionerNFS},
					{Type: &corev1.Secret{}, Name: openstack.CSIManilaNFS},
					{Type: &storagev1.StorageClass{}, Name: openstack.CSIManilaNFS},
					{Type: makeUnstructured(schema.GroupVersionKind{
						Group:   "snapshot.storage.k8s.io",
						Version: "v1",
						Kind:    "VolumeSnapshotClass"}), Name: openstack.CSIManilaNFS},
					// csi-provisioner/csi-snapshotter/csi-resizer share service account with CSI cinder driver
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIManilaSecret},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIManilaSecret},
					// csi-driver-manila-node
					{Type: &appsv1.DaemonSet{}, Name: openstack.CSIManilaNodeName},
					{Type: &corev1.ServiceAccount{}, Name: openstack.CSIManilaNodeName},
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIManilaNodeName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIManilaNodeName},
					{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: openstack.CSIManilaNodeName},
				},
			},
		},
	}

	controlPlaneShootCRDsChart = &chart.Chart{
		Name:       "shoot-crds",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-crds"),
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
		Name:       "shoot-storageclasses",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-storageclasses"),
	}
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider(mgr manager.Manager) genericactuator.ValuesProvider {
	return &valuesProvider{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

// valuesProvider is a ValuesProvider that provides OpenStack-specific values for the 2 charts applied by the generic actuator.
type valuesProvider struct {
	genericactuator.NoopValuesProvider
	client  k8sclient.Client
	decoder runtime.Decoder
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	cpConfig := &api.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	infraStatus, err := vp.getInfrastructureStatus(cp)
	if err != nil {
		return nil, err
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	// Get credentials
	credentials, err := openstack.GetCredentials(ctx, vp.client, cp.Spec.SecretRef, false)
	if err != nil {
		return nil, fmt.Errorf("could not get service account from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	overlayEnabled, err := vp.isOverlayEnabled(cluster.Shoot.Spec.Networking)
	if err != nil {
		return nil, fmt.Errorf("could not determine overlay status: %v", err)
	}
	return getConfigChartValues(cpConfig, infraStatus, cloudProfileConfig, overlayEnabled, cp, credentials)
}

func (vp *valuesProvider) getInfrastructureStatus(cp *extensionsv1alpha1.ControlPlane) (*api.InfrastructureStatus, error) {
	infraStatus := &api.InfrastructureStatus{}
	if _, _, err := vp.decoder.Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
	}
	return infraStatus, nil
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
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	// TODO(rfranzke): Delete this in a future release.
	if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "csi-driver-controller-observability-config", Namespace: cp.Namespace}}); err != nil {
		return nil, fmt.Errorf("failed deleting legacy csi-driver-controller-observability-config ConfigMap: %w", err)
	}

	// TODO(rfranzke): Delete this after August 2024.
	gep19Monitoring := vp.client.Get(ctx, k8sclient.ObjectKey{Name: "prometheus-shoot", Namespace: cp.Namespace}, &appsv1.StatefulSet{}) == nil
	if gep19Monitoring {
		if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-observability-config", Namespace: cp.Namespace}}); err != nil {
			return nil, fmt.Errorf("failed deleting cloud-controller-manager-observability-config ConfigMap: %w", err)
		}
	}

	cpConfigSecret := &corev1.Secret{}

	if err := vp.client.Get(ctx, k8sclient.ObjectKey{Namespace: cp.Namespace, Name: openstack.CloudProviderConfigName}, cpConfigSecret); err != nil {
		return nil, err
	}
	checksums[openstack.CloudProviderConfigName] = gardenerutils.ComputeChecksum(cpConfigSecret.Data)

	var userAgentHeaders []string
	cpDiskConfigSecret := &corev1.Secret{}
	if err := vp.client.Get(ctx, k8sclient.ObjectKey{Namespace: cp.Namespace, Name: openstack.CloudProviderCSIDiskConfigName}, cpDiskConfigSecret); err != nil {
		return nil, err
	}
	checksums[openstack.CloudProviderCSIDiskConfigName] = gardenerutils.ComputeChecksum(cpDiskConfigSecret.Data)
	credentials, _ := vp.getCredentials(ctx, cp) // ignore missing credentials
	userAgentHeaders = vp.getUserAgentHeaders(credentials, cluster)

	return vp.getControlPlaneChartValues(cpConfig, cp, cluster, secretsReader, userAgentHeaders, checksums, scaledDown, credentials, gep19Monitoring)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &api.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}
	return vp.getControlPlaneShootChartValues(ctx, cpConfig, cp, cloudProfileConfig, cluster, secretsReader, checksums)
}

// GetStorageClassesChartValues returns the values for the shoot storageclasses chart applied by the generic actuator.
func (vp *valuesProvider) GetStorageClassesChartValues(
	_ context.Context,
	controlPlane *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	providerConfig := api.CloudProfileConfig{}
	if cluster.CloudProfile.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cluster.CloudProfile.Spec.ProviderConfig.Raw, nil, &providerConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(controlPlane), err)
		}
	}
	values := make(map[string]interface{})
	if len(providerConfig.StorageClasses) != 0 {
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
		return values, nil
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

	values["storageclasses"] = storageclasses

	return values, nil
}

func (vp *valuesProvider) getCredentials(ctx context.Context, cp *extensionsv1alpha1.ControlPlane) (*openstack.Credentials, error) {
	return openstack.GetCredentials(ctx, vp.client, cp.Spec.SecretRef, false)
}

func (vp *valuesProvider) getUserAgentHeaders(
	credentials *openstack.Credentials,
	cluster *extensionscontroller.Cluster,
) []string {
	headers := []string{}

	// Add the domain and project/tenant to the useragent headers if the secret
	// could be read and the respective fields in secret are not empty.
	if credentials != nil {
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
) (map[string]interface{}, error) {
	subnet, err := helper.FindSubnetByPurpose(infraStatus.Networks.Subnets, api.PurposeNodes)
	if err != nil {
		return nil, fmt.Errorf("could not determine subnet from infrastructureProviderStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
	}

	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil - cannot determine keystone URL and other parameters")
	}

	values := map[string]interface{}{
		"domainName":                  c.DomainName,
		"tenantName":                  c.TenantName,
		"username":                    c.Username,
		"password":                    c.Password,
		"insecure":                    c.Insecure,
		"authUrl":                     c.AuthURL,
		"applicationCredentialID":     c.ApplicationCredentialID,
		"applicationCredentialName":   c.ApplicationCredentialName,
		"applicationCredentialSecret": c.ApplicationCredentialSecret,
		"region":                      cp.Spec.Region,
		"lbProvider":                  cpConfig.LoadBalancerProvider,
		"floatingNetworkID":           infraStatus.Networks.FloatingPool.ID,
		"subnetID":                    subnet.ID,
		"dhcpDomain":                  cloudProfileConfig.DHCPDomain,
		"requestTimeout":              cloudProfileConfig.RequestTimeout,
		"useOctavia":                  cloudProfileConfig.UseOctavia != nil && *cloudProfileConfig.UseOctavia,
		"ignoreVolumeAZ":              cloudProfileConfig.IgnoreVolumeAZ != nil && *cloudProfileConfig.IgnoreVolumeAZ,
		// detect internal network.
		// See https://github.com/kubernetes/cloud-provider-openstack/blob/v1.22.1/docs/openstack-cloud-controller-manager/using-openstack-cloud-controller-manager.md#networking
		"internalNetworkName": infraStatus.Networks.Name,
	}

	if !isUsingOverlay {
		values["routerID"] = infraStatus.Networks.Router.ID
	}

	if len(c.CACert) > 0 {
		values["caCert"] = c.CACert
	}

	loadBalancerClassesFromCloudProfile := []api.LoadBalancerClass{}
	if floatingPool, err := helper.FindFloatingPool(cloudProfileConfig.Constraints.FloatingPools, infraStatus.Networks.FloatingPool.Name, cp.Spec.Region, nil); err == nil {
		loadBalancerClassesFromCloudProfile = floatingPool.LoadBalancerClasses
	}

	// The LoadBalancerClasses from the CloudProfile will be configured by default.
	// In case the user specifies own LoadBalancerClasses via the ControlPlaneConfig
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
	// add it to the list of available LoadBalancerClasses.
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
	// configured one will be treated as default LoadBalancerClass.
	if lbClassPurpose == api.DefaultLoadBalancerClass && firstLoadBalancerClass != nil {
		return firstLoadBalancerClass
	}

	return nil
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func (vp *valuesProvider) getControlPlaneChartValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	userAgentHeaders []string,
	checksums map[string]string,
	scaledDown bool,
	credentials *openstack.Credentials,
	gep19Monitoring bool,
) (
	map[string]interface{},
	error,
) {
	ccm, err := getCCMChartValues(cpConfig, cp, cluster, secretsReader, userAgentHeaders, checksums, scaledDown, gep19Monitoring)
	if err != nil {
		return nil, err
	}

	csiCinder, err := getCSIControllerChartValues(cluster, secretsReader, userAgentHeaders, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	csiManila, err := vp.getCSIManilaControllerChartValues(cpConfig, cp, cluster, userAgentHeaders, checksums, scaledDown, credentials)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"genericTokenKubeconfigSecretName": extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster),
		},
		openstack.CloudControllerManagerName: ccm,
		openstack.CSIControllerName:          csiCinder,
		openstack.CSIManilaControllerName:    csiManila,
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
	gep19Monitoring bool,
) (map[string]interface{}, error) {
	serverSecret, found := secretsReader.Get(cloudControllerManagerServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", cloudControllerManagerServerName)
	}

	values := map[string]interface{}{
		"enabled":           true,
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        strings.Join(extensionscontroller.GetPodNetwork(cluster), ","),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/secret-" + openstack.CloudProviderConfigName:        checksums[openstack.CloudProviderConfigName],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
		"tlsCipherSuites": kutil.TLSCipherSuites,
		"secrets": map[string]interface{}{
			"server": serverSecret.Name,
		},
		"gep19Monitoring": gep19Monitoring,
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
) (map[string]interface{}, error) {
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
			"topologyAwareRoutingEnabled": gardencorev1beta1helper.IsTopologyAwareRoutingForShootControlPlaneEnabled(cluster.Seed, cluster.Shoot),
		},
	}
	if userAgentHeaders != nil {
		values["userAgentHeaders"] = userAgentHeaders
	}
	return values, nil
}

// getCSIManilaControllerChartValues collects and returns the CSIController chart values.
func (vp *valuesProvider) getCSIManilaControllerChartValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	userAgentHeaders []string,
	checksums map[string]string,
	scaledDown bool,
	credentials *openstack.Credentials,
) (map[string]interface{}, error) {
	csiManilaEnabled := vp.isCSIManilaEnabled(cpConfig)
	values := map[string]interface{}{
		"enabled": csiManilaEnabled,
	}

	if csiManilaEnabled {
		values["replicas"] = extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1)
		values["podAnnotations"] = map[string]interface{}{
			"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
		}
		if userAgentHeaders != nil {
			values["userAgentHeaders"] = userAgentHeaders
		}
		if err := vp.addCSIManilaValues(values, cp, cluster, credentials); err != nil {
			return nil, err
		}
	}

	return values, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func (vp *valuesProvider) getControlPlaneShootChartValues(
	ctx context.Context,
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cloudProfileConfig *api.CloudProfileConfig,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
) (
	map[string]interface{},
	error,
) {
	var (
		cloudProviderDiskConfig []byte
		userAgentHeader         []string
		csiNodeDriverValues     map[string]interface{}
		caBundle                string
	)

	secret := &corev1.Secret{}
	if err := vp.client.Get(ctx, k8sclient.ObjectKey{Namespace: cp.Namespace, Name: openstack.CloudProviderCSIDiskConfigName}, secret); err != nil {
		return nil, err
	}

	cloudProviderDiskConfig = secret.Data[openstack.CloudProviderConfigDataKey]
	checksums[openstack.CloudProviderCSIDiskConfigName] = gardenerutils.ComputeChecksum(secret.Data)
	credentials, _ := vp.getCredentials(ctx, cp) // ignore missing credentials
	userAgentHeader = vp.getUserAgentHeaders(credentials, cluster)

	caSecret, found := secretsReader.Get(caNameControlPlane)
	if !found {
		return nil, fmt.Errorf("secret %q not found", caNameControlPlane)
	}
	caBundle = string(caSecret.Data[secretutils.DataKeyCertificateBundle])

	csiNodeDriverValues = map[string]interface{}{
		"enabled":    true,
		"vpaEnabled": gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
		},
		"cloudProviderConfig": cloudProviderDiskConfig,
		"webhookConfig": map[string]interface{}{
			"url":      "https://" + openstack.CSISnapshotValidationName + "." + cp.Namespace + "/volumesnapshot",
			"caBundle": caBundle,
		},

		"rescanBlockStorageOnResize": cloudProfileConfig.RescanBlockStorageOnResize != nil && *cloudProfileConfig.RescanBlockStorageOnResize,
		"nodeVolumeAttachLimit":      cloudProfileConfig.NodeVolumeAttachLimit,
	}

	// add keystone CA bundle
	if keystoneCA, ok := secret.Data[openstack.CloudProviderConfigKeyStoneCAKey]; ok && len(keystoneCA) > 0 {
		csiNodeDriverValues["keystoneCACert"] = keystoneCA
	}

	if userAgentHeader != nil {
		csiNodeDriverValues["userAgentHeaders"] = userAgentHeader
	}

	csiDriverManilaValues, err := vp.getControlPlaneShootChartCSIManilaValues(cpConfig, cp, cluster, credentials)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		openstack.CloudControllerManagerName: map[string]interface{}{"enabled": true},
		openstack.CSINodeName:                csiNodeDriverValues,
		openstack.CSIDriverManila:            csiDriverManilaValues,
	}, nil
}

func (vp *valuesProvider) isOverlayEnabled(network *v1beta1.Networking) (bool, error) {
	if network == nil || network.ProviderConfig == nil {
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
	var networkConfig map[string]interface{}
	if err := json.Unmarshal(networkProviderConfig, &networkConfig); err != nil {
		return false, err
	}
	if overlay, ok := networkConfig["overlay"].(map[string]interface{}); ok {
		return overlay["enabled"].(bool), nil
	}
	return true, nil
}

func (vp *valuesProvider) isCSIManilaEnabled(cpConfig *api.ControlPlaneConfig) bool {
	return cpConfig.Storage != nil && cpConfig.Storage.CSIManila != nil && cpConfig.Storage.CSIManila.Enabled
}

func (vp *valuesProvider) getControlPlaneShootChartCSIManilaValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	credentials *openstack.Credentials,
) (map[string]interface{}, error) {

	csiManilaEnabled := vp.isCSIManilaEnabled(cpConfig)
	values := map[string]interface{}{
		"enabled": csiManilaEnabled,
	}

	if csiManilaEnabled {
		values["vpaEnabled"] = gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot)

		if err := vp.addCSIManilaValues(values, cp, cluster, credentials); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (vp *valuesProvider) addCSIManilaValues(
	values map[string]interface{},
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	credentials *openstack.Credentials,
) error {
	values["csimanila"] = map[string]interface{}{
		"clusterID": cp.Namespace,
	}

	infraConfig, err := helper.InfrastructureConfigFromRawExtension(cluster.Shoot.Spec.Provider.InfrastructureConfig)
	if err != nil {
		return fmt.Errorf("could not decode infrastructure config of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
	}
	infraStatus, err := vp.getInfrastructureStatus(cp)
	if err != nil {
		return err
	}

	var authURL, domainName, projectName, username, password,
		applicationCredentialID, applicationCredentialName, applicationCredentialSecret,
		caCert, insecure, shareNetworkID string
	if credentials != nil {
		authURL = credentials.AuthURL
		domainName = credentials.DomainName
		projectName = credentials.TenantName
		username = credentials.Username
		password = credentials.Password
		applicationCredentialID = credentials.ApplicationCredentialID
		applicationCredentialName = credentials.ApplicationCredentialName
		applicationCredentialSecret = credentials.ApplicationCredentialSecret
		caCert = credentials.CACert
		if credentials.Insecure {
			insecure = "true"
		}
	}
	if infraStatus.Networks.ShareNetwork != nil {
		shareNetworkID = infraStatus.Networks.ShareNetwork.ID
	}
	values["openstack"] = map[string]interface{}{
		"availabilityZones":           vp.getAllWorkerPoolsZones(cluster),
		"shareNetworkID":              shareNetworkID,
		"shareClient":                 infrastructure.WorkersCIDR(infraConfig),
		"authURL":                     authURL,
		"region":                      cp.Spec.Region,
		"domainName":                  domainName,
		"projectName":                 projectName,
		"userName":                    username,
		"password":                    password,
		"applicationCredentialID":     applicationCredentialID,
		"applicationCredentialName":   applicationCredentialName,
		"applicationCredentialSecret": applicationCredentialSecret,
		"tlsInsecure":                 insecure,
		"caCert":                      caCert,
	}

	return nil
}

func (vp *valuesProvider) getAllWorkerPoolsZones(cluster *extensionscontroller.Cluster) []string {
	zones := sets.NewString()
	for _, worker := range cluster.Shoot.Spec.Provider.Workers {
		zones.Insert(worker.Zones...)
	}
	list := zones.UnsortedList()
	sort.Strings(list)
	return list
}
