// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/dns"
	"github.com/gardener/gardener/pkg/operation/botanist/seedsystemcomponents/seedadmission"
	"github.com/gardener/gardener/pkg/operation/botanist/systemcomponents/metricsserver"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed/istio"
	"github.com/gardener/gardener/pkg/operation/seed/scheduler"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/pkg/version"

	"github.com/Masterminds/semver"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		seedObjectFunc: func() (*gardencorev1beta1.Seed, error) { return nil, fmt.Errorf("seed object is required but not set") },
	}
}

// WithSeedObject sets the seedObjectFunc attribute at the Builder.
func (b *Builder) WithSeedObject(seedObject *gardencorev1beta1.Seed) *Builder {
	b.seedObjectFunc = func() (*gardencorev1beta1.Seed, error) { return seedObject, nil }
	return b
}

// WithSeedObjectFromLister sets the seedObjectFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithSeedObjectFromLister(seedLister gardencorelisters.SeedLister, seedName string) *Builder {
	b.seedObjectFunc = func() (*gardencorev1beta1.Seed, error) { return seedLister.Get(seedName) }
	return b
}

// Build initializes a new Seed object.
func (b *Builder) Build() (*Seed, error) {
	seed := &Seed{}

	seedObject, err := b.seedObjectFunc()
	if err != nil {
		return nil, err
	}
	seed.Info = seedObject

	if seedObject.Spec.Settings != nil && seedObject.Spec.Settings.LoadBalancerServices != nil {
		seed.LoadBalancerServiceAnnotations = seedObject.Spec.Settings.LoadBalancerServices.Annotations
	}

	return seed, nil
}

const (
	caSeed = "ca-seed"
)

var wantedCertificateAuthorities = map[string]*secretsutils.CertificateSecretConfig{
	caSeed: {
		Name:       caSeed,
		CommonName: "kubernetes",
		CertType:   secretsutils.CACert,
	},
}

const (
	grafanaPrefix = "g-seed"
	grafanaTLS    = "grafana-tls"

	prometheusPrefix = "p-seed"
	prometheusTLS    = "aggregate-prometheus-tls"

	userExposedComponentTagPrefix = "user-exposed"
)

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func generateWantedSecrets(seed *Seed, certificateAuthorities map[string]*secretsutils.Certificate) ([]secretsutils.ConfigInterface, error) {
	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	endUserCrtValidity := common.EndUserCrtValidity

	secretList := []secretsutils.ConfigInterface{
		&secretsutils.CertificateSecretConfig{
			Name: common.VPASecretName,

			CommonName:   "vpa-webhook.garden.svc",
			Organization: nil,
			DNSNames:     []string{"vpa-webhook.garden.svc", "vpa-webhook"},
			IPAddresses:  nil,

			CertType:  secretsutils.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
		},
		&secretsutils.CertificateSecretConfig{
			Name: common.GrafanaTLS,

			CommonName:   "grafana",
			Organization: []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:     []string{seed.GetIngressFQDN(grafanaPrefix)},
			IPAddresses:  nil,

			CertType:  secretsutils.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
			Validity:  &endUserCrtValidity,
		},
		&secretsutils.CertificateSecretConfig{
			Name: prometheusTLS,

			CommonName:   "prometheus",
			Organization: []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:     []string{seed.GetIngressFQDN(prometheusPrefix)},
			IPAddresses:  nil,

			CertType:  secretsutils.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
			Validity:  &endUserCrtValidity,
		},
	}

	return secretList, nil
}

// deployCertificates deploys CA and TLS certificates inside the garden namespace
// It takes a map[string]*corev1.Secret object which contains secrets that have already been deployed inside that namespace to avoid duplication errors.
func deployCertificates(ctx context.Context, seed *Seed, k8sSeedClient kubernetes.Interface, existingSecretsMap map[string]*corev1.Secret) (map[string]*corev1.Secret, error) {
	_, certificateAuthorities, err := secretsutils.GenerateCertificateAuthorities(ctx, k8sSeedClient, existingSecretsMap, wantedCertificateAuthorities, v1beta1constants.GardenNamespace)
	if err != nil {
		return nil, err
	}

	wantedSecretsList, err := generateWantedSecrets(seed, certificateAuthorities)
	if err != nil {
		return nil, err
	}

	// Only necessary to renew certificates for Grafana, Prometheus
	// TODO: (timuthy) remove in future version.
	var (
		renewedLabel = "cert.gardener.cloud/renewed-endpoint"
		browserCerts = sets.NewString(grafanaTLS, prometheusTLS)
	)
	for name, secret := range existingSecretsMap {
		_, ok := secret.Labels[renewedLabel]
		if browserCerts.Has(name) && !ok {
			if err := k8sSeedClient.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
				return nil, err
			}
			delete(existingSecretsMap, name)
		}
	}

	secrets, err := secretsutils.GenerateClusterSecrets(ctx, k8sSeedClient, existingSecretsMap, wantedSecretsList, v1beta1constants.GardenNamespace)
	if err != nil {
		return nil, err
	}

	// Only necessary to renew certificates for Grafana, Prometheus
	// TODO: (timuthy) remove in future version.
	for name, secret := range secrets {
		_, ok := secret.Labels[renewedLabel]
		if browserCerts.Has(name) && !ok {
			if secret.Labels == nil {
				secret.Labels = make(map[string]string)
			}
			secret.Labels[renewedLabel] = "true"

			if err := k8sSeedClient.Client().Update(ctx, secret); err != nil {
				return nil, err
			}
		}
	}

	return secrets, nil
}

// BootstrapCluster bootstraps a Seed cluster and deploys various required manifests.
func BootstrapCluster(ctx context.Context, k8sGardenClient, k8sSeedClient kubernetes.Interface, seed *Seed, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, componentImageVectors imagevector.ComponentImageVectors, conf *config.GardenletConfiguration) error {
	vpaGK := schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}

	vpaEnabled := seed.Info.Spec.Settings == nil || seed.Info.Spec.Settings.VerticalPodAutoscaler == nil || seed.Info.Spec.Settings.VerticalPodAutoscaler.Enabled
	if !vpaEnabled {
		// VPA is a prerequisite. If it's not enabled via the seed spec it must be provided through some other mechanism.
		if _, err := k8sSeedClient.RESTMapper().RESTMapping(vpaGK); err != nil {
			return fmt.Errorf("VPA is required for seed cluster: %s", err)
		}

		if err := common.DeleteVpa(ctx, k8sSeedClient.Client(), v1beta1constants.GardenNamespace, false); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	const chartName = "seed-bootstrap"
	var (
		loggingConfig   = conf.Logging
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.GardenNamespace,
			},
		}
	)

	if err := k8sSeedClient.Client().Create(ctx, gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err := kutil.TryUpdateNamespace(ctx, k8sSeedClient.Kubernetes(), retry.DefaultBackoff, gardenNamespace.ObjectMeta, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutil.SetMetaDataLabel(&ns.ObjectMeta, "role", v1beta1constants.GardenNamespace)
		return ns, nil
	}); err != nil {
		return err
	}
	if _, err := kutil.TryUpdateNamespace(ctx, k8sSeedClient.Kubernetes(), retry.DefaultBackoff, metav1.ObjectMeta{Name: metav1.NamespaceSystem}, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutil.SetMetaDataLabel(&ns.ObjectMeta, "role", metav1.NamespaceSystem)
		return ns, nil
	}); err != nil {
		return err
	}

	if monitoringSecrets := common.GetSecretKeysWithPrefix(common.GardenRoleGlobalMonitoring, secrets); len(monitoringSecrets) > 0 {
		for _, key := range monitoringSecrets {
			secret := secrets[key]
			secretObj := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", "seed", secret.Name),
					Namespace: "garden",
				},
			}

			if _, err := controllerutil.CreateOrUpdate(ctx, k8sSeedClient.Client(), secretObj, func() error {
				secretObj.Type = corev1.SecretTypeOpaque
				secretObj.Data = secret.Data
				return nil
			}); err != nil {
				return err
			}
		}
	}

	images, err := imagevector.FindImages(imageVector,
		[]string{
			common.AlertManagerImageName,
			common.AlpineImageName,
			common.ConfigMapReloaderImageName,
			common.LokiImageName,
			common.FluentBitImageName,
			common.FluentBitPluginInstaller,
			common.GardenerResourceManagerImageName,
			common.GrafanaImageName,
			common.PauseContainerImageName,
			common.PrometheusImageName,
			common.VpaAdmissionControllerImageName,
			common.VpaExporterImageName,
			common.VpaRecommenderImageName,
			common.VpaUpdaterImageName,
			common.HvpaControllerImageName,
			common.DependencyWatchdogImageName,
			common.KubeStateMetricsImageName,
			common.NginxIngressControllerSeedImageName,
			common.IngressDefaultBackendImageName,
		},
		imagevector.RuntimeVersion(k8sSeedClient.Version()),
		imagevector.TargetVersion(k8sSeedClient.Version()),
	)
	if err != nil {
		return err
	}

	// HVPA feature gate
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if !hvpaEnabled {
		if err := common.DeleteHvpa(ctx, k8sSeedClient, v1beta1constants.GardenNamespace); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	// Fetch component-specific dependency-watchdog configuration
	var dependencyWatchdogConfigurations []string
	for _, componentFn := range []component.DependencyWatchdogConfiguration{
		func() (string, error) { return etcd.DependencyWatchdogConfiguration(v1beta1constants.ETCDRoleMain) },
	} {
		dwdConfig, err := componentFn()
		if err != nil {
			return err
		}
		dependencyWatchdogConfigurations = append(dependencyWatchdogConfigurations, dwdConfig)
	}

	// Fetch component-specific central monitoring configuration
	var (
		centralScrapeConfigs                            = strings.Builder{}
		centralCAdvisorScrapeConfigMetricRelabelConfigs = strings.Builder{}
	)

	for _, componentFn := range []component.CentralMonitoringConfiguration{} {
		centralMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range centralMonitoringConfig.ScrapeConfigs {
			centralScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		for _, config := range centralMonitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs {
			centralCAdvisorScrapeConfigMetricRelabelConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	// Logging feature gate
	var (
		loggingEnabled                    = gardenletfeatures.FeatureGate.Enabled(features.Logging)
		existingSecretsMap                = map[string]*corev1.Secret{}
		filters                           = strings.Builder{}
		parsers                           = strings.Builder{}
		fluentBitConfigurationsOverwrites = map[string]interface{}{}
		lokiValues                        = map[string]interface{}{}
	)

	lokiValues["enabled"] = loggingEnabled

	if loggingEnabled {
		lokiValues["authEnabled"] = false
		lokiValues["hvpa"] = map[string]interface{}{
			"enabled": hvpaEnabled,
		}

		lokiVpa := &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "loki-vpa", Namespace: v1beta1constants.GardenNamespace}}
		if err := k8sSeedClient.Client().Delete(ctx, lokiVpa); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}

		if hvpaEnabled {
			currentResources, err := common.GetContainerResourcesInStatefulSet(ctx, k8sSeedClient.Client(), kutil.Key(v1beta1constants.GardenNamespace, "loki"))
			if err != nil {
				return err
			}
			if len(currentResources) != 0 && currentResources[0] != nil {
				lokiValues["resources"] = currentResources[0]
			}
		}

		componentsFunctions := []component.CentralLoggingConfiguration{
			// seed system components
			seedadmission.CentralLoggingConfiguration,
			// shoot control plane components
			etcd.CentralLoggingConfiguration,
			clusterautoscaler.CentralLoggingConfiguration,
			kubescheduler.CentralLoggingConfiguration,
			kubecontrollermanager.CentralLoggingConfiguration,
			// shoot system components
			metricsserver.CentralLoggingConfiguration,
		}
		userAllowedComponents := []string{v1beta1constants.DeploymentNameKubeAPIServer}

		// Fetch component specific logging configurations
		for _, componentFn := range componentsFunctions {
			loggingConfig, err := componentFn()
			if err != nil {
				return err
			}

			filters.WriteString(fmt.Sprintln(loggingConfig.Filters))
			parsers.WriteString(fmt.Sprintln(loggingConfig.Parsers))

			if loggingConfig.UserExposed {
				userAllowedComponents = append(userAllowedComponents, loggingConfig.PodPrefix)
			}
		}

		loggingRewriteTagFilter := `[FILTER]
    Name          rewrite_tag
    Match         kubernetes.*
    Rule          $tag ^kubernetes\.var\.log\.containers\.(` + strings.Join(userAllowedComponents, "-.+?|") + `-.+?)_ ` + userExposedComponentTagPrefix + `.$TAG true
    Emitter_Name  re_emitted
`
		filters.WriteString(fmt.Sprintln(loggingRewriteTagFilter))

		// Read extension provider specific logging configuration
		existingConfigMaps := &corev1.ConfigMapList{}
		if err = k8sSeedClient.Client().List(ctx, existingConfigMaps,
			client.InNamespace(v1beta1constants.GardenNamespace),
			client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelLogging}); err != nil {
			return err
		}

		// Need stable order before passing the dashboards to Grafana config to avoid unnecessary changes
		kutil.ByName().Sort(existingConfigMaps)

		// Read all filters and parsers coming from the extension provider configurations
		for _, cm := range existingConfigMaps.Items {
			filters.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapKubernetesFilter]))
			parsers.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapParser]))
		}

		if loggingConfig != nil && loggingConfig.FluentBit != nil {
			fbConfig := loggingConfig.FluentBit

			if fbConfig.ServiceSection != nil {
				fluentBitConfigurationsOverwrites["service"] = *fbConfig.ServiceSection
			}
			if fbConfig.InputSection != nil {
				fluentBitConfigurationsOverwrites["input"] = *fbConfig.InputSection
			}
			if fbConfig.OutputSection != nil {
				fluentBitConfigurationsOverwrites["output"] = *fbConfig.OutputSection
			}
		}
	} else {
		if err := common.DeleteSeedLoggingStack(ctx, k8sSeedClient.Client()); err != nil {
			return err
		}
	}

	existingSecrets := &corev1.SecretList{}
	if err = k8sSeedClient.Client().List(ctx, existingSecrets, client.InNamespace(v1beta1constants.GardenNamespace)); err != nil {
		return err
	}

	for _, secret := range existingSecrets.Items {
		secretObj := secret
		existingSecretsMap[secret.ObjectMeta.Name] = &secretObj
	}

	deployedSecretsMap, err := deployCertificates(ctx, seed, k8sSeedClient, existingSecretsMap)
	if err != nil {
		return err
	}
	jsonString, err := json.Marshal(deployedSecretsMap[common.VPASecretName].Data)
	if err != nil {
		return err
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": seed.GetValidVolumeSize("1Gi"),
	}

	alertingSMTPKeys := common.GetSecretKeysWithPrefix(common.GardenRoleAlerting, secrets)

	if seedWantsAlertmanager(alertingSMTPKeys, secrets) {
		emailConfigs := make([]map[string]interface{}, 0, len(alertingSMTPKeys))
		for _, key := range alertingSMTPKeys {
			if string(secrets[key].Data["auth_type"]) == "smtp" {
				secret := secrets[key]
				emailConfigs = append(emailConfigs, map[string]interface{}{
					"to":            string(secret.Data["to"]),
					"from":          string(secret.Data["from"]),
					"smarthost":     string(secret.Data["smarthost"]),
					"auth_username": string(secret.Data["auth_username"]),
					"auth_identity": string(secret.Data["auth_identity"]),
					"auth_password": string(secret.Data["auth_password"]),
				})
				alertManagerConfig["enabled"] = true
				alertManagerConfig["emailConfigs"] = emailConfigs
				break
			}
		}
	} else {
		alertManagerConfig["enabled"] = false
		if err := common.DeleteAlertmanager(ctx, k8sSeedClient.Client(), v1beta1constants.GardenNamespace); err != nil {
			return err
		}
	}

	if !seed.Info.Spec.Settings.ExcessCapacityReservation.Enabled {
		if err := common.DeleteReserveExcessCapacity(ctx, k8sSeedClient.Client()); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	chartApplier := k8sSeedClient.ChartApplier()

	var (
		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
		hvpaGK                = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK              = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}
		grafanaHost           = seed.GetIngressFQDN(grafanaPrefix)
		prometheusHost        = seed.GetIngressFQDN(prometheusPrefix)
		monitoringCredentials = existingSecretsMap["seed-monitoring-ingress-credentials"]
		monitoringBasicAuth   string
	)

	if monitoringCredentials != nil {
		monitoringBasicAuth = utils.CreateSHA1Secret(monitoringCredentials.Data[secretsutils.DataKeyUserName], monitoringCredentials.Data[secretsutils.DataKeyPassword])
	}
	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation

	networks := []string{
		seed.Info.Spec.Networks.Pods,
		seed.Info.Spec.Networks.Services,
	}
	if v := seed.Info.Spec.Networks.Nodes; v != nil {
		networks = append(networks, *v)
	}

	privateNetworks, err := common.ToExceptNetworks(common.AllPrivateNetworkBlocks(), networks...)
	if err != nil {
		return err
	}

	var (
		grafanaTLSOverride    = grafanaTLS
		prometheusTLSOverride = prometheusTLS
	)

	wildcardCert, err := GetWildcardCertificate(ctx, k8sSeedClient.Client())
	if err != nil {
		return err
	}

	if wildcardCert != nil {
		grafanaTLSOverride = wildcardCert.GetName()
		prometheusTLSOverride = wildcardCert.GetName()
	}

	imageVectorOverwrites := make(map[string]string, len(componentImageVectors))
	for name, data := range componentImageVectors {
		imageVectorOverwrites[name] = data
	}

	anySNI, err := controlplane.AnyDeployedSNI(ctx, k8sSeedClient.Client())
	if err != nil {
		return err
	}

	if gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio) {
		istiodImage, err := imageVector.FindImage(common.IstioIstiodImageName)
		if err != nil {
			return err
		}

		igwImage, err := imageVector.FindImage(common.IstioProxyImageName)
		if err != nil {
			return err
		}

		chartApplier := k8sSeedClient.ChartApplier()
		istioCRDs := istio.NewIstioCRD(chartApplier, common.ChartPath, k8sSeedClient.Client())
		istiod := istio.NewIstiod(
			&istio.IstiodValues{
				TrustDomain: "cluster.local",
				Image:       istiodImage.String(),
			},
			common.IstioNamespace,
			chartApplier,
			common.ChartPath,
			k8sSeedClient.Client(),
		)

		igwConfig := &istio.IngressValues{
			TrustDomain:     "cluster.local",
			Image:           igwImage.String(),
			IstiodNamespace: common.IstioNamespace,
			Annotations:     seed.LoadBalancerServiceAnnotations,
			Ports:           []corev1.ServicePort{},
		}

		// even if SNI is being disabled, the existing ports must stay the same
		// until all APIServer SNI resources are removed.
		if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) || anySNI {
			igwConfig.Ports = append(
				igwConfig.Ports,
				corev1.ServicePort{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt(8443)},
				corev1.ServicePort{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(9443)},
			)

			if gardenletfeatures.FeatureGate.Enabled(features.KonnectivityTunnel) {
				igwConfig.Ports = append(
					igwConfig.Ports,
					corev1.ServicePort{Name: "tls-tunnel", Port: 8132, TargetPort: intstr.FromInt(8132)},
				)
			}
		}

		igw := istio.NewIngressGateway(
			igwConfig,
			*conf.SNI.Ingress.Namespace,
			chartApplier,
			common.ChartPath,
			k8sSeedClient.Client(),
		)

		if err := component.OpWaiter(istioCRDs, istiod, igw).Deploy(ctx); err != nil {
			return err
		}
	}

	proxy := istio.NewProxyProtocolGateway(*conf.SNI.Ingress.Namespace, chartApplier, common.ChartPath)

	if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		if err := proxy.Deploy(ctx); err != nil {
			return err
		}
	} else {
		if err := proxy.Destroy(ctx); err != nil {
			return err
		}
	}

	if seed.Info.Status.ClusterIdentity == nil {
		seedClusterIdentity, err := determineClusterIdentity(ctx, k8sSeedClient.Client())
		if err != nil {
			return err
		}

		seed.Info.Status.ClusterIdentity = &seedClusterIdentity
		if err := k8sGardenClient.Client().Status().Update(ctx, seed.Info); err != nil {
			return err
		}
	}

	// .spec.selector of a Deployment is immutable. If Deployment's .spec.selector contains
	// the deprecated role label key, we delete it and let it to be re-created below with the chart apply.
	// TODO: remove in a future version
	deploymentKeys := []client.ObjectKey{
		kutil.Key(v1beta1constants.GardenNamespace, "vpa-exporter"),
	}
	if vpaEnabled {
		deploymentKeys = append(deploymentKeys,
			kutil.Key(v1beta1constants.GardenNamespace, "vpa-updater"),
			kutil.Key(v1beta1constants.GardenNamespace, "vpa-recommender"),
			kutil.Key(v1beta1constants.GardenNamespace, "vpa-admission-controller"),
		)
	}
	if err := common.DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, k8sSeedClient.Client(), deploymentKeys); err != nil {
		return err
	}

	values := kubernetes.Values(map[string]interface{}{
		"priorityClassName": v1beta1constants.PriorityClassNameShootControlPlane,
		"global": map[string]interface{}{
			"ingressClass": getIngressClass(managedIngress(seed)),
			"images":       chart.ImageMapToValues(images),
		},
		"reserveExcessCapacity": seed.Info.Spec.Settings.ExcessCapacityReservation.Enabled,
		"replicas": map[string]interface{}{
			"reserve-excess-capacity": DesiredExcessCapacity(),
		},
		"prometheus": map[string]interface{}{
			"storage":                 seed.GetValidVolumeSize("10Gi"),
			"additionalScrapeConfigs": centralScrapeConfigs.String(),
			"additionalCAdvisorScrapeConfigMetricRelabelConfigs": centralCAdvisorScrapeConfigMetricRelabelConfigs.String(),
		},
		"aggregatePrometheus": map[string]interface{}{
			"storage":    seed.GetValidVolumeSize("20Gi"),
			"seed":       seed.Info.Name,
			"hostName":   prometheusHost,
			"secretName": prometheusTLSOverride,
		},
		"grafana": map[string]interface{}{
			"hostName":   grafanaHost,
			"secretName": grafanaTLSOverride,
		},
		"fluent-bit": map[string]interface{}{
			"enabled":                           loggingEnabled,
			"additionalParsers":                 parsers.String(),
			"additionalFilters":                 filters.String(),
			"fluentBitConfigurationsOverwrites": fluentBitConfigurationsOverwrites,
			"exposedComponentsTagPrefix":        userExposedComponentTagPrefix,
		},
		"loki":         lokiValues,
		"alertmanager": alertManagerConfig,
		"vpa": map[string]interface{}{
			"enabled": vpaEnabled,
			"runtime": map[string]interface{}{
				"admissionController": map[string]interface{}{
					"podAnnotations": map[string]interface{}{
						"checksum/secret-vpa-tls-certs": utils.ComputeSHA256Hex(jsonString),
					},
				},
			},
			"application": map[string]interface{}{
				"admissionController": map[string]interface{}{
					"controlNamespace": v1beta1constants.GardenNamespace,
					"caCert":           deployedSecretsMap[common.VPASecretName].Data[secretsutils.DataKeyCertificateCA],
				},
			},
		},
		"dependency-watchdog": map[string]interface{}{
			"config": strings.Join(dependencyWatchdogConfigurations, "\n"),
		},
		"nginx-ingress": computeNginxIngress(seed),
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
		"istio": map[string]interface{}{
			"enabled": gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio),
		},
		"global-network-policies": map[string]interface{}{
			"denyAll":         false,
			"privateNetworks": privateNetworks,
			"sniEnabled":      gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) || anySNI,
		},
		"gardenerResourceManager": map[string]interface{}{
			"resourceClass": v1beta1constants.SeedResourceManagerClass,
		},
		"ingress": map[string]interface{}{
			"basicAuthSecret": monitoringBasicAuth,
		},
		"cluster-identity": map[string]interface{}{"clusterIdentity": &seed.Info.Status.ClusterIdentity},
	})

	if err := chartApplier.Apply(ctx, filepath.Join(common.ChartPath, chartName), v1beta1constants.GardenNamespace, chartName, values, applierOptions); err != nil {
		return err
	}

	if err := handleDNSProvider(ctx, k8sGardenClient.Client(), k8sSeedClient.Client(), seed.Info.Spec.DNS); err != nil {
		return err
	}

	// managed nginx-ingress handling
	if err := handleIngressDNSEntry(ctx, k8sSeedClient, chartApplier, seed); err != nil {
		return err
	}

	var ingressClass string
	if managedIngress(seed) {
		ingressClass = v1beta1constants.SeedNginxIngressClass
	} else {
		ingressClass = v1beta1constants.ShootNginxIngressClass
		if err := deleteIngressController(ctx, k8sSeedClient.Client()); err != nil {
			return err
		}
	}
	if err := migrateIngressClassForShootIngresses(ctx, k8sGardenClient.Client(), k8sSeedClient.Client(), seed, ingressClass); err != nil {
		return err
	}

	// Deploy component specific resources
	bootstrapComponents, err := bootstrapComponents(k8sSeedClient, v1beta1constants.GardenNamespace, imageVector, imageVectorOverwrites)
	if err != nil {
		return err
	}

	var bootstrapFunctions []flow.TaskFn
	for _, componentFn := range bootstrapComponents {
		fn := componentFn
		bootstrapFunctions = append(bootstrapFunctions, func(ctx context.Context) error {
			return component.OpWaiter(fn).Deploy(ctx)
		})
	}

	return flow.Parallel(bootstrapFunctions...)(ctx)
}

// DebootstrapCluster deletes certain resources from the seed cluster.
func DebootstrapCluster(ctx context.Context, k8sSeedClient kubernetes.Interface) error {
	bootstrapComponents, err := bootstrapComponents(k8sSeedClient, v1beta1constants.GardenNamespace, nil, nil)
	if err != nil {
		return err
	}

	// Delete component specific resources
	var debootstrapFunctions []flow.TaskFn
	for _, componentFn := range bootstrapComponents {
		fn := componentFn
		debootstrapFunctions = append(debootstrapFunctions, func(ctx context.Context) error {
			return component.OpDestroyAndWait(fn).Destroy(ctx)
		})
	}

	return flow.Parallel(debootstrapFunctions...)(ctx)
}

func bootstrapComponents(c kubernetes.Interface, namespace string, imageVector imagevector.ImageVector, imageVectorOverwrites map[string]string) ([]component.DeployWaiter, error) {
	var components []component.DeployWaiter

	kubernetesVersion, err := semver.NewVersion(c.Version())
	if err != nil {
		return nil, err
	}

	// cluster-autoscaler
	components = append(components, clusterautoscaler.NewBootstrapper(c.Client(), namespace))

	// etcd
	var (
		etcdImage                string
		etcdImageVectorOverwrite *string
	)
	if imageVector != nil {
		image, err := imageVector.FindImage(common.EtcdDruidImageName, imagevector.RuntimeVersion(c.Version()), imagevector.TargetVersion(c.Version()))
		if err != nil {
			return nil, err
		}
		etcdImage = image.String()
	}
	if imageVectorOverwrites != nil {
		if val, ok := imageVectorOverwrites[etcd.Druid]; ok {
			etcdImageVectorOverwrite = &val
		}
	}
	components = append(components, etcd.NewBootstrapper(c.Client(), namespace, etcdImage, kubernetesVersion, etcdImageVectorOverwrite))

	// gardener-seed-admission-controller
	var gsacImage imagevector.Image
	if imageVector != nil {
		gardenerSeedAdmissionControllerImage, err := imageVector.FindImage(common.GardenerSeedAdmissionControllerImageName)
		if err != nil {
			return nil, err
		}
		var (
			repository = gardenerSeedAdmissionControllerImage.String()
			tag        = version.Get().GitVersion
		)
		if gardenerSeedAdmissionControllerImage.Tag != nil {
			repository = gardenerSeedAdmissionControllerImage.Repository
			tag = *gardenerSeedAdmissionControllerImage.Tag
		}
		gsacImage = imagevector.Image{
			Repository: repository,
			Tag:        &tag,
		}
	}
	components = append(components, seedadmission.New(c.Client(), namespace, gsacImage.String(), kubernetesVersion))

	// kube-scheduler for shoot control plane pods
	var schedulerImage *imagevector.Image
	if imageVector != nil {
		schedulerImage, err = imageVector.FindImage(common.KubeSchedulerImageName, imagevector.TargetVersion(kubernetesVersion.String()))
		if err != nil {
			return nil, err
		}
	}
	sched, err := scheduler.Bootstrap(c.DirectClient(), namespace, schedulerImage, kubernetesVersion)
	if err != nil {
		return nil, err
	}
	components = append(components, sched)

	return components, nil
}

func managedIngress(seed *Seed) bool {
	return seed.Info.Spec.Ingress != nil
}

// DesiredExcessCapacity computes the required resources (CPU and memory) required to deploy new shoot control planes
// (on the seed) in terms of reserve-excess-capacity deployment replicas. Each deployment replica currently
// corresponds to resources of (request/limits) 2 cores of CPU and 6Gi of RAM.
// This roughly corresponds to a single, moderately large control-plane.
// The logic for computation of desired excess capacity corresponds to deploying 2 such shoot control planes.
// This excess capacity can be used for hosting new control planes or newly vertically scaled old control-planes.
func DesiredExcessCapacity() int {
	var (
		replicasToSupportSingleShoot = 1
		effectiveExcessCapacity      = 2
	)

	return effectiveExcessCapacity * replicasToSupportSingleShoot
}

// GetIngressFQDNDeprecated returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
// Only necessary to renew certificates for Alertmanager, Grafana, Prometheus
// TODO: (timuthy) remove in future version.
func (s *Seed) GetIngressFQDNDeprecated(subDomain, shootName, projectName string) string {
	ingressDomain := s.IngressDomain()

	if shootName == "" {
		return fmt.Sprintf("%s.%s.%s", subDomain, projectName, ingressDomain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", subDomain, shootName, projectName, ingressDomain)
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
func (s *Seed) GetIngressFQDN(subDomain string) string {
	return fmt.Sprintf("%s.%s", subDomain, s.IngressDomain())
}

// IngressDomain returns the ingress domain for the seed.
func (s *Seed) IngressDomain() string {
	if s.Info.Spec.DNS.IngressDomain != nil {
		return *s.Info.Spec.DNS.IngressDomain
	} else if s.Info.Spec.Ingress != nil {
		return s.Info.Spec.Ingress.Domain
	}
	return ""
}

// CheckMinimumK8SVersion checks whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
func (s *Seed) CheckMinimumK8SVersion(seedClient kubernetes.Interface) (string, error) {
	// We require CRD status subresources for the extension controllers that we install into the seeds.
	minSeedVersion := "1.11"

	version := seedClient.Version()

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minSeedVersion)
	if err != nil {
		return "<unknown>", err
	}
	if !seedVersionOK {
		return "<unknown>", fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minSeedVersion)
	}
	return version, nil
}

// GetValidVolumeSize is to get a valid volume size.
// If the given size is smaller than the minimum volume size permitted by cloud provider on which seed cluster is running, it will return the minimum size.
func (s *Seed) GetValidVolumeSize(size string) string {
	if s.Info.Spec.Volume == nil || s.Info.Spec.Volume.MinimumSize == nil {
		return size
	}

	qs, err := resource.ParseQuantity(size)
	if err == nil && qs.Cmp(*s.Info.Spec.Volume.MinimumSize) < 0 {
		return s.Info.Spec.Volume.MinimumSize.String()
	}

	return size
}

func seedWantsAlertmanager(keys []string, secrets map[string]*corev1.Secret) bool {
	for _, key := range keys {
		if string(secrets[key].Data["auth_type"]) == "smtp" {
			return true
		}
	}
	return false
}

// GetWildcardCertificate gets the wildcard certificate for the seed's ingress domain.
// Nil is returned if no wildcard certificate is configured.
func GetWildcardCertificate(ctx context.Context, c client.Client) (*corev1.Secret, error) {
	wildcardCerts := &corev1.SecretList{}
	if err := c.List(
		ctx,
		wildcardCerts,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert},
	); err != nil {
		return nil, err
	}

	if len(wildcardCerts.Items) > 1 {
		return nil, fmt.Errorf("misconfigured seed cluster: not possible to provide more than one secret with annotation %s", common.ControlPlaneWildcardCert)
	}

	if len(wildcardCerts.Items) == 1 {
		return &wildcardCerts.Items[0], nil
	}
	return nil, nil
}

// determineClusterIdentity determines the identity of a cluster, in cases where the identity was
// created manually or the Seed was created as Shoot, and later registered as Seed and already has
// an identity, it should not be changed.
func determineClusterIdentity(ctx context.Context, c client.Client) (string, error) {
	clusterIdentity := &corev1.ConfigMap{}
	if err := c.Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ClusterIdentity), clusterIdentity); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		gardenNamespace := &corev1.Namespace{}
		if err := c.Get(ctx, kutil.Key(metav1.NamespaceSystem), gardenNamespace); err != nil {
			return "", err
		}
		return string(gardenNamespace.UID), nil
	}
	return clusterIdentity.Data[v1beta1constants.ClusterIdentity], nil
}

func getIngressClass(seedIngressEnabled bool) string {
	if seedIngressEnabled {
		return v1beta1constants.SeedNginxIngressClass
	}
	return v1beta1constants.ShootNginxIngressClass
}

func handleDNSProvider(ctx context.Context, gardenClient, seedClient client.Client, dnsConfig gardencorev1beta1.SeedDNS) error {
	var (
		dnsProvider = &dnsv1alpha1.DNSProvider{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1beta1constants.GardenNamespace,
				Name:      "seed",
			},
		}
		providerSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1beta1constants.GardenNamespace,
				Name:      "dnsprovider-seed",
			},
		}
	)

	if dnsConfig.Provider == nil {
		return kutil.DeleteObjects(ctx, seedClient, dnsProvider, providerSecret)
	}

	cloudProviderSecret := kutil.Key(dnsConfig.Provider.SecretRef.Namespace, dnsConfig.Provider.SecretRef.Name)
	if err := copySecretFromGardenerToSeed(ctx, gardenClient, seedClient, cloudProviderSecret, providerSecret); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, seedClient, dnsProvider, func() error {
		dnsProvider.Spec = dnsv1alpha1.DNSProviderSpec{
			Type: dnsConfig.Provider.Type,
			SecretRef: &corev1.SecretReference{
				Namespace: providerSecret.Namespace,
				Name:      providerSecret.Name,
			},
		}

		if dnsConfig.Provider.Domains != nil {
			dnsProvider.Spec.Domains = &dnsv1alpha1.DNSSelection{
				Include: dnsConfig.Provider.Domains.Include,
				Exclude: dnsConfig.Provider.Domains.Exclude,
			}
		}

		if dnsConfig.Provider.Zones != nil {
			dnsProvider.Spec.Zones = &dnsv1alpha1.DNSSelection{
				Include: dnsConfig.Provider.Zones.Include,
				Exclude: dnsConfig.Provider.Zones.Exclude,
			}
		}

		return nil
	})
	return err
}

func handleIngressDNSEntry(ctx context.Context, c kubernetes.Interface, chartApplier kubernetes.ChartApplier, seed *Seed) error {
	var (
		seedLogger = logger.Logger.WithField("seed", seed.Info.Name)
		values     = &dns.EntryValues{Name: "ingress"}
	)

	if managedIngress(seed) {
		loadBalancerAddress, err := kutil.WaitUntilLoadBalancerIsReady(
			ctx,
			c,
			v1beta1constants.GardenNamespace,
			"nginx-ingress-controller",
			time.Minute,
			seedLogger,
		)
		if err != nil {
			return err
		}

		values.DNSName = seed.GetIngressFQDN("*")
		values.Targets = []string{loadBalancerAddress}
	}

	dnsEntry := dns.NewDNSEntry(
		values,
		v1beta1constants.GardenNamespace,
		chartApplier,
		common.ChartPath,
		seedLogger,
		c.Client(),
		nil,
	)

	if managedIngress(seed) {
		return dnsEntry.Deploy(ctx)
	}
	return dnsEntry.Destroy(ctx)
}

const annotationSeedIngressClass = "seed.gardener.cloud/ingress-class"

func migrateIngressClassForShootIngresses(ctx context.Context, gardenClient, seedClient client.Client, seed *Seed, newClass string) error {
	if oldClass, ok := seed.Info.Annotations[annotationSeedIngressClass]; ok && oldClass == newClass {
		return nil
	}

	shootNamespaces := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, shootNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return err
	}

	for _, ns := range shootNamespaces.Items {
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "alertmanager"), newClass); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "prometheus"), newClass); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "grafana-operators"), newClass); err != nil {
			return err
		}
		if err := switchIngressClass(ctx, seedClient, kutil.Key(ns.Name, "grafana-users"), newClass); err != nil {
			return err
		}
	}

	metav1.SetMetaDataAnnotation(&seed.Info.ObjectMeta, annotationSeedIngressClass, newClass)
	return gardenClient.Update(ctx, seed.Info)
}

func switchIngressClass(ctx context.Context, seedClient client.Client, ingressKey types.NamespacedName, newClass string) error {
	ingress := &extensionsv1beta1.Ingress{}
	if err := seedClient.Get(ctx, ingressKey, ingress); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	ingress.Annotations["kubernetes.io/ingress.class"] = newClass
	return seedClient.Update(ctx, ingress)
}

func copySecretFromGardenerToSeed(ctx context.Context, gardenClient, seedClient client.Client, secretKey types.NamespacedName, targetSecret *corev1.Secret) error {
	gardenSecret := &corev1.Secret{}
	if err := gardenClient.Get(ctx, secretKey, gardenSecret); err != nil {
		return err
	}

	if gardenSecret == nil {
		return errors.New("error during seed bootstrap: secret referenced in dns.provider.SecretRef not found")
	}

	_, err := controllerutil.CreateOrUpdate(ctx, seedClient, targetSecret, func() error {
		targetSecret.Type = gardenSecret.Type
		targetSecret.Data = gardenSecret.Data
		return nil
	})
	return err
}

func computeNginxIngress(seed *Seed) map[string]interface{} {
	values := map[string]interface{}{
		"enabled":      managedIngress(seed),
		"ingressClass": v1beta1constants.SeedNginxIngressClass,
	}

	if seed.Info.Spec.Ingress != nil && seed.Info.Spec.Ingress.Controller.ProviderConfig != nil {
		values["config"] = seed.Info.Spec.Ingress.Controller.ProviderConfig
	}

	return values
}

func deleteIngressController(ctx context.Context, c client.Client) error {
	return kutil.DeleteObjects(
		ctx,
		c,
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:seed:nginx-ingress"}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:seed:nginx-ingress"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-controller", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-k8s-backend", Namespace: v1beta1constants.GardenNamespace}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress", Namespace: v1beta1constants.GardenNamespace}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nginx-ingress-k8s-backend", Namespace: v1beta1constants.GardenNamespace}},
	)
}
