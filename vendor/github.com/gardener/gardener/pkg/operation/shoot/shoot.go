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

package shoot

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/extension"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		shootObjectFunc: func(context.Context) (*gardencorev1beta1.Shoot, error) {
			return nil, fmt.Errorf("shoot object is required but not set")
		},
		cloudProfileFunc: func(string) (*gardencorev1beta1.CloudProfile, error) {
			return nil, fmt.Errorf("cloudprofile object is required but not set")
		},
		shootSecretFunc: func(context.Context, client.Client, string, string) (*corev1.Secret, error) {
			return nil, fmt.Errorf("shoot secret object is required but not set")
		},
	}
}

// WithShootObject sets the shootObjectFunc attribute at the Builder.
func (b *Builder) WithShootObject(shootObject *gardencorev1beta1.Shoot) *Builder {
	b.shootObjectFunc = func(context.Context) (*gardencorev1beta1.Shoot, error) { return shootObject, nil }
	return b
}

// WithShootObjectFromCluster sets the shootObjectFunc attribute at the Builder.
func (b *Builder) WithShootObjectFromCluster(seedClient kubernetes.Interface, seedNamespace string) *Builder {
	b.shootObjectFunc = func(ctx context.Context) (*gardencorev1beta1.Shoot, error) {
		cluster, err := gardenerextensions.GetCluster(ctx, seedClient.Client(), seedNamespace)
		if err != nil {
			return nil, err
		}
		return cluster.Shoot, err
	}
	return b
}

// WithShootObjectFromLister sets the shootObjectFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithShootObjectFromLister(shootLister gardencorelisters.ShootLister, namespace, name string) *Builder {
	b.shootObjectFunc = func(context.Context) (*gardencorev1beta1.Shoot, error) {
		return shootLister.Shoots(namespace).Get(name)
	}
	return b
}

// WithCloudProfileObject sets the cloudProfileFunc attribute at the Builder.
func (b *Builder) WithCloudProfileObject(cloudProfileObject *gardencorev1beta1.CloudProfile) *Builder {
	b.cloudProfileFunc = func(string) (*gardencorev1beta1.CloudProfile, error) { return cloudProfileObject, nil }
	return b
}

// WithCloudProfileObjectFromLister sets the cloudProfileFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithCloudProfileObjectFromLister(cloudProfileLister gardencorelisters.CloudProfileLister) *Builder {
	b.cloudProfileFunc = func(name string) (*gardencorev1beta1.CloudProfile, error) { return cloudProfileLister.Get(name) }
	return b
}

// WithShootSecret sets the shootSecretFunc attribute at the Builder.
func (b *Builder) WithShootSecret(secret *corev1.Secret) *Builder {
	b.shootSecretFunc = func(context.Context, client.Client, string, string) (*corev1.Secret, error) { return secret, nil }
	return b
}

// WithShootSecretFromLister sets the shootSecretFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithShootSecretFromSecretBindingLister(secretBindingLister gardencorelisters.SecretBindingLister) *Builder {
	b.shootSecretFunc = func(ctx context.Context, c client.Client, namespace, secretBindingName string) (*corev1.Secret, error) {
		binding, err := secretBindingLister.SecretBindings(namespace).Get(secretBindingName)
		if err != nil {
			return nil, err
		}

		secret := &corev1.Secret{}
		if err = c.Get(ctx, kutil.Key(binding.SecretRef.Namespace, binding.SecretRef.Name), secret); err != nil {
			return nil, err
		}
		return secret, nil
	}
	return b
}

// WithDisableDNS sets the disableDNS attribute at the Builder.
func (b *Builder) WithDisableDNS(disableDNS bool) *Builder {
	b.disableDNS = disableDNS
	return b
}

// WithProjectName sets the projectName attribute at the Builder.
func (b *Builder) WithProjectName(projectName string) *Builder {
	b.projectName = projectName
	return b
}

// WithInternalDomain sets the internalDomain attribute at the Builder.
func (b *Builder) WithInternalDomain(internalDomain *garden.Domain) *Builder {
	b.internalDomain = internalDomain
	return b
}

// WithDefaultDomains sets the defaultDomains attribute at the Builder.
func (b *Builder) WithDefaultDomains(defaultDomains []*garden.Domain) *Builder {
	b.defaultDomains = defaultDomains
	return b
}

// Build initializes a new Shoot object.
func (b *Builder) Build(ctx context.Context, c client.Client) (*Shoot, error) {
	shoot := &Shoot{}

	shootObject, err := b.shootObjectFunc(ctx)
	if err != nil {
		return nil, err
	}
	shoot.Info = shootObject

	cloudProfile, err := b.cloudProfileFunc(shootObject.Spec.CloudProfileName)
	if err != nil {
		return nil, err
	}
	shoot.CloudProfile = cloudProfile

	secret, err := b.shootSecretFunc(ctx, c, shootObject.Namespace, shootObject.Spec.SecretBindingName)
	if err != nil {
		return nil, err
	}
	shoot.Secret = secret

	shoot.DisableDNS = b.disableDNS
	shoot.OperatingSystemConfigsMap = make(map[string]OperatingSystemConfigs, len(shoot.GetWorkerNames()))
	shoot.HibernationEnabled = gardencorev1beta1helper.HibernationIsEnabled(shootObject)
	shoot.SeedNamespace = ComputeTechnicalID(b.projectName, shootObject)
	shoot.InternalClusterDomain = ConstructInternalClusterDomain(shootObject.Name, b.projectName, b.internalDomain)
	shoot.ExternalClusterDomain = ConstructExternalClusterDomain(shootObject)
	shoot.IgnoreAlerts = gardencorev1beta1helper.ShootIgnoresAlerts(shootObject)
	shoot.WantsAlertmanager = gardencorev1beta1helper.ShootWantsAlertManager(shootObject)
	shoot.WantsVerticalPodAutoscaler = gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(shootObject)
	shoot.Components = &Components{
		Extensions: &Extensions{
			DNS: &DNS{},
		},
		ControlPlane:     &ControlPlane{},
		SystemComponents: &SystemComponents{},
	}

	extensions, err := calculateExtensions(ctx, c, shootObject, shoot.SeedNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate required extensions for shoot %s: %v", shootObject.Name, err)
	}
	shoot.Extensions = extensions

	// Determine information about external domain for shoot cluster.
	externalDomain, err := ConstructExternalDomain(ctx, c, shootObject, secret, b.defaultDomains)
	if err != nil {
		return nil, err
	}
	shoot.ExternalDomain = externalDomain

	// Store the Kubernetes version in the format <major>.<minor> on the Shoot object.
	kubernetesVersion, err := semver.NewVersion(shootObject.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}
	shoot.KubernetesMajorMinorVersion = fmt.Sprintf("%d.%d", kubernetesVersion.Major(), kubernetesVersion.Minor())
	shoot.KubernetesVersion = kubernetesVersion

	gardenerVersion, err := semver.NewVersion(shootObject.Status.Gardener.Version)
	if err != nil {
		return nil, err
	}
	shoot.GardenerVersion = gardenerVersion

	kubernetesVersionGeq118, err := versionutils.CheckVersionMeetsConstraint(shoot.KubernetesMajorMinorVersion, ">= 1.18")
	if err != nil {
		return nil, err
	}

	shoot.KonnectivityTunnelEnabled = gardenletfeatures.FeatureGate.Enabled(features.KonnectivityTunnel) && kubernetesVersionGeq118
	if konnectivityTunnelEnabled, err := strconv.ParseBool(shoot.Info.Annotations[v1beta1constants.AnnotationShootKonnectivityTunnel]); err == nil && kubernetesVersionGeq118 {
		shoot.KonnectivityTunnelEnabled = konnectivityTunnelEnabled
	}

	needsClusterAutoscaler, err := gardencorev1beta1helper.ShootWantsClusterAutoscaler(shootObject)
	if err != nil {
		return nil, err
	}
	shoot.WantsClusterAutoscaler = needsClusterAutoscaler

	networks, err := ToNetworks(shootObject)
	if err != nil {
		return nil, err
	}
	shoot.Networks = networks

	shoot.ResourceRefs = getResourceRefs(shootObject)
	shoot.NodeLocalDNSEnabled = gardenletfeatures.FeatureGate.Enabled(features.NodeLocalDNS)
	shoot.Purpose = gardencorev1beta1helper.GetPurpose(shootObject)

	return shoot, nil
}

func calculateExtensions(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, seedNamespace string) (map[string]extension.Extension, error) {
	controllerRegistrations := &gardencorev1beta1.ControllerRegistrationList{}
	if err := gardenClient.List(ctx, controllerRegistrations); err != nil {
		return nil, err
	}
	return MergeExtensions(controllerRegistrations.Items, shoot.Spec.Extensions, seedNamespace)
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Shoot cluster. The
// end result is '<subDomain>.<ingressPrefix>.<clusterDomain>'.
func (s *Shoot) GetIngressFQDN(subDomain string) string {
	if s.Info.Spec.DNS == nil || s.Info.Spec.DNS.Domain == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", subDomain, common.IngressPrefix, *(s.Info.Spec.DNS.Domain))
}

// GetWorkerNames returns a list of names of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkerNames() []string {
	var workerNames []string
	for _, worker := range s.Info.Spec.Provider.Workers {
		workerNames = append(workerNames, worker.Name)
	}
	return workerNames
}

// GetMinNodeCount returns the sum of all 'minimum' fields of all worker groups of the Shoot.
func (s *Shoot) GetMinNodeCount() int32 {
	var nodeCount int32
	for _, worker := range s.Info.Spec.Provider.Workers {
		nodeCount += worker.Minimum
	}
	return nodeCount
}

// GetMaxNodeCount returns the sum of all 'maximum' fields of all worker groups of the Shoot.
func (s *Shoot) GetMaxNodeCount() int32 {
	var nodeCount int32
	for _, worker := range s.Info.Spec.Provider.Workers {
		nodeCount += worker.Maximum
	}
	return nodeCount
}

// GetNodeNetwork returns the nodes network CIDR for the Shoot cluster. If the infrastructure extension
// controller has generated a nodes network then this CIDR will take priority. Otherwise, the nodes network
// CIDR specified in the shoot will be returned (if possible). If no CIDR was specified then nil is returned.
func (s *Shoot) GetNodeNetwork() *string {
	if val := s.Info.Spec.Networking.Nodes; val != nil {
		return val
	}
	return nil
}

// ComputeCloudConfigSecretName computes the name for a secret which contains the original cloud config for
// the worker group with the given <workerName>. It is build by the cloud config secret prefix, the worker
// name itself and a hash of the minor Kubernetes version of the Shoot cluster.
func (s *Shoot) ComputeCloudConfigSecretName(workerName string) string {
	return fmt.Sprintf("%s-%s-%s", common.CloudConfigPrefix, workerName, utils.ComputeSHA256Hex([]byte(s.KubernetesMajorMinorVersion))[:5])
}

// GetReplicas returns the given <wokenUp> number if the shoot is not hibernated, or zero otherwise.
func (s *Shoot) GetReplicas(wokenUp int32) int32 {
	if s.HibernationEnabled {
		return 0
	}
	return wokenUp
}

// ComputeInClusterAPIServerAddress returns the internal address for the shoot API server depending on whether
// the caller runs in the shoot namespace or not.
func (s *Shoot) ComputeInClusterAPIServerAddress(runsInShootNamespace bool) string {
	url := v1beta1constants.DeploymentNameKubeAPIServer
	if !runsInShootNamespace {
		url = fmt.Sprintf("%s.%s.svc", url, s.SeedNamespace)
	}
	return url
}

// ComputeOutOfClusterAPIServerAddress returns the external address for the shoot API server depending on whether
// the caller wants to use the internal cluster domain and whether DNS is disabled on this seed.
func (s *Shoot) ComputeOutOfClusterAPIServerAddress(apiServerAddress string, useInternalClusterDomain bool) string {
	if s.DisableDNS {
		return apiServerAddress
	}

	if gardencorev1beta1helper.ShootUsesUnmanagedDNS(s.Info) {
		return common.GetAPIServerDomain(s.InternalClusterDomain)
	}

	if useInternalClusterDomain {
		return common.GetAPIServerDomain(s.InternalClusterDomain)
	}

	return common.GetAPIServerDomain(*s.ExternalClusterDomain)
}

// IPVSEnabled returns true if IPVS is enabled for the shoot.
func (s *Shoot) IPVSEnabled() bool {
	return s.Info.Spec.Kubernetes.KubeProxy != nil &&
		s.Info.Spec.Kubernetes.KubeProxy.Mode != nil &&
		*s.Info.Spec.Kubernetes.KubeProxy.Mode == gardencorev1beta1.ProxyModeIPVS
}

// TechnicalIDPrefix is a prefix used for a shoot's technical id.
const TechnicalIDPrefix = "shoot--"

// ComputeTechnicalID determines the technical id of that Shoot which is later used for the name of the
// namespace and for tagging all the resources created in the infrastructure.
func ComputeTechnicalID(projectName string, shoot *gardencorev1beta1.Shoot) string {
	// Use the stored technical ID in the Shoot's status field if it's there.
	// For backwards compatibility we keep the pattern as it was before we had to change it
	// (double hyphens).
	if len(shoot.Status.TechnicalID) > 0 {
		return shoot.Status.TechnicalID
	}

	// New clusters shall be created with the new technical id (double hyphens).
	return fmt.Sprintf("%s%s--%s", TechnicalIDPrefix, projectName, shoot.Name)
}

// ConstructInternalClusterDomain constructs the internal base domain pof this shoot cluster.
// It is only used for internal purposes (all kubeconfigs except the one which is received by the
// user will only talk with the kube-apiserver via a DNS record of domain). In case the given <internalDomain>
// already contains "internal", the result is constructed as "<shootName>.<shootProject>.<internalDomain>."
// In case it does not, the word "internal" will be appended, resulting in
// "<shootName>.<shootProject>.internal.<internalDomain>".
func ConstructInternalClusterDomain(shootName, shootProject string, internalDomain *garden.Domain) string {
	if internalDomain == nil {
		return ""
	}
	if strings.Contains(internalDomain.Domain, common.InternalDomainKey) {
		return fmt.Sprintf("%s.%s.%s", shootName, shootProject, internalDomain.Domain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", shootName, shootProject, common.InternalDomainKey, internalDomain.Domain)
}

// ConstructExternalClusterDomain constructs the external Shoot cluster domain, i.e. the domain which will be put
// into the Kubeconfig handed out to the user.
func ConstructExternalClusterDomain(shoot *gardencorev1beta1.Shoot) *string {
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return nil
	}
	return shoot.Spec.DNS.Domain
}

// ConstructExternalDomain constructs an object containing all relevant information of the external domain that
// shall be used for a shoot cluster - based on the configuration of the Garden cluster and the shoot itself.
func ConstructExternalDomain(ctx context.Context, client client.Client, shoot *gardencorev1beta1.Shoot, shootSecret *corev1.Secret, defaultDomains []*garden.Domain) (*garden.Domain, error) {
	externalClusterDomain := ConstructExternalClusterDomain(shoot)
	if externalClusterDomain == nil {
		return nil, nil
	}

	var (
		externalDomain  = &garden.Domain{Domain: *shoot.Spec.DNS.Domain}
		defaultDomain   = garden.DomainIsDefaultDomain(*externalClusterDomain, defaultDomains)
		primaryProvider = gardencorev1beta1helper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	)

	switch {
	case defaultDomain != nil:
		externalDomain.SecretData = defaultDomain.SecretData
		externalDomain.Provider = defaultDomain.Provider
		externalDomain.IncludeDomains = defaultDomain.IncludeDomains
		externalDomain.ExcludeDomains = defaultDomain.ExcludeDomains
		externalDomain.IncludeZones = defaultDomain.IncludeZones
		externalDomain.ExcludeZones = defaultDomain.ExcludeZones

	case primaryProvider != nil:
		if primaryProvider.SecretName != nil {
			secret := &corev1.Secret{}
			if err := client.Get(ctx, kutil.Key(shoot.Namespace, *primaryProvider.SecretName), secret); err != nil {
				return nil, fmt.Errorf("could not get dns provider secret %q: %+v", *shoot.Spec.DNS.Providers[0].SecretName, err)
			}
			externalDomain.SecretData = secret.Data
		} else {
			externalDomain.SecretData = shootSecret.Data
		}
		if primaryProvider.Type != nil {
			externalDomain.Provider = *primaryProvider.Type
		}
		if domains := primaryProvider.Domains; domains != nil {
			externalDomain.IncludeDomains = domains.Include
			externalDomain.ExcludeDomains = domains.Exclude
		}
		if zones := primaryProvider.Zones; zones != nil {
			externalDomain.IncludeZones = zones.Include
			externalDomain.ExcludeZones = zones.Exclude
		}

	default:
		return nil, &IncompleteDNSConfigError{}
	}

	return externalDomain, nil
}

// MergeExtensions merges the given controller registrations with the given extensions, expecting that each type in
// extensions is also represented in the registration. It ignores all extensions that were explicitly disabled in the
// shoot spec.
func MergeExtensions(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, namespace string) (map[string]extension.Extension, error) {
	var (
		typeToExtension    = make(map[string]extension.Extension)
		requiredExtensions = make(map[string]extension.Extension)
	)

	// Extensions enabled by default for all Shoot clusters.
	for _, reg := range registrations {
		for _, res := range reg.Spec.Resources {
			if res.Kind != extensionsv1alpha1.ExtensionResource {
				continue
			}

			timeout := extension.DefaultTimeout
			if res.ReconcileTimeout != nil {
				timeout = res.ReconcileTimeout.Duration
			}

			typeToExtension[res.Type] = extension.Extension{
				Extension: extensionsv1alpha1.Extension{
					ObjectMeta: metav1.ObjectMeta{
						Name:      res.Type,
						Namespace: namespace,
					},
					Spec: extensionsv1alpha1.ExtensionSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type: res.Type,
						},
					},
				},
				Timeout: timeout,
			}

			if res.GloballyEnabled != nil && *res.GloballyEnabled {
				requiredExtensions[res.Type] = typeToExtension[res.Type]
			}
		}
	}

	// Extensions defined in Shoot resource.
	for _, extension := range extensions {
		if obj, ok := typeToExtension[extension.Type]; ok {
			if utils.IsTrue(extension.Disabled) {
				delete(requiredExtensions, extension.Type)
				continue
			}

			obj.Spec.ProviderConfig = extension.ProviderConfig
			requiredExtensions[extension.Type] = obj
			continue
		}
	}

	return requiredExtensions, nil
}

// ToNetworks return a network with computed cidrs and ClusterIPs
// for a Shoot
func ToNetworks(s *gardencorev1beta1.Shoot) (*Networks, error) {
	if s.Spec.Networking.Services == nil {
		return nil, fmt.Errorf("shoot's service cidr is empty")
	}

	if s.Spec.Networking.Pods == nil {
		return nil, fmt.Errorf("shoot's pods cidr is empty")
	}

	_, svc, err := net.ParseCIDR(*s.Spec.Networking.Services)
	if err != nil {
		return nil, fmt.Errorf("cannot parse shoot's network cidr %v", err)
	}

	_, pods, err := net.ParseCIDR(*s.Spec.Networking.Pods)
	if err != nil {
		return nil, fmt.Errorf("cannot parse shoot's network cidr %v", err)
	}

	apiserver, err := common.ComputeOffsetIP(svc, 1)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate default/kubernetes ClusterIP: %v", err)
	}

	coreDNS, err := common.ComputeOffsetIP(svc, 10)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate CoreDNS ClusterIP: %v", err)
	}

	return &Networks{
		CoreDNS:   coreDNS,
		Pods:      pods,
		Services:  svc,
		APIServer: apiserver,
	}, nil
}

// ComputeRequiredExtensions compute the extension kind/type combinations that are required for the
// reconciliation flow.
func ComputeRequiredExtensions(shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, controllerRegistrationList []*gardencorev1beta1.ControllerRegistration, internalDomain, externalDomain *garden.Domain) sets.String {
	requiredExtensions := sets.NewString()

	if seed.Spec.Backup != nil {
		requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.BackupBucketResource, seed.Spec.Backup.Provider))
		requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.BackupEntryResource, seed.Spec.Backup.Provider))
	}
	// Hint: This is actually a temporary work-around to request the control plane extension of the seed provider type as
	// it might come with webhooks that are configuring the exposure of shoot control planes. The ControllerRegistration resource
	// does not reflect this today.
	requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.ControlPlaneResource, seed.Spec.Provider.Type))

	requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.ControlPlaneResource, shoot.Spec.Provider.Type))
	requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.InfrastructureResource, shoot.Spec.Provider.Type))
	requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.NetworkResource, shoot.Spec.Networking.Type))
	requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.WorkerResource, shoot.Spec.Provider.Type))

	var disabledExtensions = sets.NewString()
	for _, extension := range shoot.Spec.Extensions {
		id := common.ExtensionID(extensionsv1alpha1.ExtensionResource, extension.Type)

		if utils.IsTrue(extension.Disabled) {
			disabledExtensions.Insert(id)
		} else {
			requiredExtensions.Insert(id)
		}
	}

	for _, pool := range shoot.Spec.Provider.Workers {
		if pool.Machine.Image != nil {
			requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.OperatingSystemConfigResource, pool.Machine.Image.Name))
		}
		if pool.CRI != nil {
			for _, cr := range pool.CRI.ContainerRuntimes {
				requiredExtensions.Insert(common.ExtensionID(extensionsv1alpha1.ContainerRuntimeResource, cr.Type))
			}
		}
	}

	if seed.Spec.Settings.ShootDNS.Enabled {
		if shoot.Spec.DNS != nil {
			for _, provider := range shoot.Spec.DNS.Providers {
				if provider.Type != nil && *provider.Type != core.DNSUnmanaged {
					requiredExtensions.Insert(common.ExtensionID(dnsv1alpha1.DNSProviderKind, *provider.Type))
				}
			}
		}

		if internalDomain != nil && internalDomain.Provider != core.DNSUnmanaged {
			requiredExtensions.Insert(common.ExtensionID(dnsv1alpha1.DNSProviderKind, internalDomain.Provider))
		}

		if externalDomain != nil && externalDomain.Provider != core.DNSUnmanaged {
			requiredExtensions.Insert(common.ExtensionID(dnsv1alpha1.DNSProviderKind, externalDomain.Provider))
		}
	}

	for _, controllerRegistration := range controllerRegistrationList {
		for _, resource := range controllerRegistration.Spec.Resources {
			id := common.ExtensionID(extensionsv1alpha1.ExtensionResource, resource.Type)
			if resource.Kind == extensionsv1alpha1.ExtensionResource && resource.GloballyEnabled != nil && *resource.GloballyEnabled && !disabledExtensions.Has(id) {
				requiredExtensions.Insert(id)
			}
		}
	}

	return requiredExtensions
}

// getResourceRefs returns resource references from the Shoot spec as map[string]autoscalingv1.CrossVersionObjectReference.
func getResourceRefs(shoot *gardencorev1beta1.Shoot) map[string]autoscalingv1.CrossVersionObjectReference {
	resourceRefs := make(map[string]autoscalingv1.CrossVersionObjectReference)
	for _, r := range shoot.Spec.Resources {
		resourceRefs[r.Name] = r.ResourceRef
	}
	return resourceRefs
}
