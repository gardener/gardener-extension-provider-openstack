// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/uuid"
	schemev1 "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

const (
	reconcilerUseTF        string = "tf"
	reconcilerMigrateTF    string = "migrate"
	reconcilerUseFlow      string = "flow"
	reconcilerRecoverState string = "recover"
)

const (
	vpcCIDR = "10.250.0.0/16"
)

var (
	authURL          = flag.String("auth-url", "", "Authorization URL for openstack")
	domainName       = flag.String("domain-name", "", "Domain name for openstack")
	floatingPoolName = flag.String("floating-pool-name", "", "Floating pool name for creating router")
	region           = flag.String("region", "", "Openstack region")
	tenantName       = flag.String("tenant-name", "", "Tenant name for openstack")
	userName         = flag.String("user-name", "", "User name for openstack")
	password         = flag.String("password", "", "Password for openstack")
	appID            = flag.String("app-id", "", "Application Credential ID for openstack")
	appName          = flag.String("app-name", "", "Application Credential Name for openstack")
	appSecret        = flag.String("app-secret", "", "Application Credential Secret for openstack")
	reconciler       = flag.String("reconciler", reconcilerUseTF, "Set annotation to use flow for reconciliation")

	floatingPoolID string
)

func validateFlags() {
	if len(*authURL) == 0 {
		panic("--auth-url flag is not specified")
	}
	if len(*domainName) == 0 {
		panic("--domain-name flag is not specified")
	}
	if len(*floatingPoolName) == 0 {
		panic("--floating-pool-name is not specified")
	}
	if len(*region) == 0 {
		panic("--region flag is not specified")
	}
	if len(*tenantName) == 0 {
		panic("--tenant-name flag is not specified")
	}

	err := openstack.ValidateSecrets(*userName, *password, *appID, *appName, *appSecret)
	if err != nil {
		panic(fmt.Errorf("flag error: %w", err))
	}
}

var (
	ctx = context.Background()
	log logr.Logger

	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client

	decoder runtime.Decoder

	networkClient openstackclient.Networking
	computeClient openstackclient.Compute
	testId        = string(uuid.NewUUID())
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	log = logf.Log.WithName("infrastructure-test")

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_infrastructures.yaml"),
			},
		},
	}

	restConfig, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(restConfig).ToNot(BeNil())

	httpClient, err := rest.HTTPClientFor(restConfig)
	Expect(err).NotTo(HaveOccurred())
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
	Expect(err).NotTo(HaveOccurred())

	scheme := runtime.NewScheme()
	Expect(schemev1.AddToScheme(scheme)).To(Succeed())
	Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(openstackinstall.AddToScheme(scheme)).To(Succeed())

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Cache: cache.Options{
			Mapper: mapper,
			ByObject: map[client.Object]cache.ByObject{
				&extensionsv1alpha1.Infrastructure{}: {
					Label: labels.SelectorFromSet(labels.Set{"test-id": testId}),
				},
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(openstackinstall.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(infrastructure.AddToManagerWithOptions(ctx, mgr, infrastructure.AddOptions{
		// During testing in testmachinery cluster, there is no gardener-resource-manager to inject the volume mount.
		// Hence, we need to run without projected token mount.
		DisableProjectedTokenMount: true,
		Controller: controller.Options{
			MaxConcurrentReconciles: 5,
		},
	})).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	c = mgr.GetClient()
	Expect(c).NotTo(BeNil())

	decoder = serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder()

	openstackClient, err := openstackclient.NewOpenstackClientFromCredentials(&openstack.Credentials{
		AuthURL:                     *authURL,
		Username:                    *userName,
		Password:                    *password,
		DomainName:                  *domainName,
		TenantName:                  *tenantName,
		ApplicationCredentialID:     *appID,
		ApplicationCredentialName:   *appName,
		ApplicationCredentialSecret: *appSecret,
	})
	Expect(err).NotTo(HaveOccurred())

	opts := openstackclient.WithRegion(*region)
	networkClient, err = openstackClient.Networking(opts)
	Expect(err).NotTo(HaveOccurred())
	computeClient, err = openstackClient.Compute(opts)
	Expect(err).NotTo(HaveOccurred())

	// Retrieve FloatingPoolNetworkID
	externalNetwork, err := networkClient.GetExternalNetworkByName(*floatingPoolName)
	Expect(err).NotTo(HaveOccurred())
	floatingPoolID = externalNetwork.ID

	priorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.PriorityClassNameShootControlPlane300,
		},
		Description:   "PriorityClass for Shoot control plane components",
		GlobalDefault: false,
		Value:         999998300,
	}
	Expect(client.IgnoreAlreadyExists(c.Create(ctx, priorityClass))).To(BeNil())
})

var _ = Describe("Infrastructure tests", func() {
	AfterEach(func() {
		framework.RunCleanupActions()
	})

	It("minimum configuration infrastructure", func() {
		providerConfig := newProviderConfig(nil, nil, nil)
		cloudProfileConfig := newCloudProfileConfig(*region, *authURL)
		namespace, err := generateNamespaceName()
		Expect(err).NotTo(HaveOccurred())

		err = runTest(ctx, log, c, namespace, providerConfig, decoder, cloudProfileConfig)

		Expect(err).NotTo(HaveOccurred())
	})

	It("with infrastructure that uses existing router", func() {
		namespace, err := generateNamespaceName()
		Expect(err).NotTo(HaveOccurred())

		cloudRouterName := namespace + "-cloud-router"

		routerID, err := prepareNewRouter(log, cloudRouterName)
		Expect(err).NotTo(HaveOccurred())

		var cleanupHandle framework.CleanupActionHandle
		cleanupHandle = framework.AddCleanupAction(func() {
			err := teardownRouter(log, *routerID)
			Expect(err).NotTo(HaveOccurred())

			framework.RemoveCleanupAction(cleanupHandle)
		})

		providerConfig := newProviderConfig(routerID, nil, nil)
		cloudProfileConfig := newCloudProfileConfig(*region, *authURL)

		err = runTest(ctx, log, c, namespace, providerConfig, decoder, cloudProfileConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("with infrastructure that uses existing network", func() {
		namespace, err := generateNamespaceName()
		Expect(err).NotTo(HaveOccurred())

		networkName := namespace + "-network"

		networkID, err := prepareNewNetwork(log, networkName)
		Expect(err).NotTo(HaveOccurred())

		var cleanupHandle framework.CleanupActionHandle
		cleanupHandle = framework.AddCleanupAction(func() {
			err := teardownNetwork(log, *networkID)
			Expect(err).NotTo(HaveOccurred())

			framework.RemoveCleanupAction(cleanupHandle)
		})

		providerConfig := newProviderConfig(nil, networkID, nil)
		cloudProfileConfig := newCloudProfileConfig(*region, *authURL)

		err = runTest(ctx, log, c, namespace, providerConfig, decoder, cloudProfileConfig)

		Expect(err).NotTo(HaveOccurred())
	})

	It("with infrastructure that uses existing network and router", func() {
		namespace, err := generateNamespaceName()
		Expect(err).NotTo(HaveOccurred())

		networkName := namespace + "-network"
		cloudRouterName := namespace + "-cloud-router"

		networkID, err := prepareNewNetwork(log, networkName)
		Expect(err).NotTo(HaveOccurred())
		routerID, err := prepareNewRouter(log, cloudRouterName)
		Expect(err).NotTo(HaveOccurred())

		var cleanupHandle framework.CleanupActionHandle
		cleanupHandle = framework.AddCleanupAction(func() {
			By("Tearing down network")
			err := teardownNetwork(log, *networkID)
			Expect(err).NotTo(HaveOccurred())

			By("Tearing down router")
			err = teardownRouter(log, *routerID)
			Expect(err).NotTo(HaveOccurred())

			framework.RemoveCleanupAction(cleanupHandle)
		})

		providerConfig := newProviderConfig(routerID, networkID, nil)
		cloudProfileConfig := newCloudProfileConfig(*region, *authURL)

		err = runTest(ctx, log, c, namespace, providerConfig, decoder, cloudProfileConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("with infrastructure that uses existing network, subnet and router", func() {
		namespace, err := generateNamespaceName()
		Expect(err).NotTo(HaveOccurred())

		networkName := namespace + "-network"
		networkID, err := prepareNewNetwork(log, networkName)
		Expect(err).NotTo(HaveOccurred())

		subnetName := namespace + "-subnet"
		subnetID, err := prepareNewSubnet(log, subnetName, *networkID)
		Expect(err).NotTo(HaveOccurred())

		routerName := namespace + "-router"
		routerID, err := prepareNewRouter(log, routerName)
		Expect(err).NotTo(HaveOccurred())

		routerInterfacePortID, err := prepareNewRouterInterface(log, *routerID, *subnetID)
		Expect(err).NotTo(HaveOccurred())

		var cleanupHandle framework.CleanupActionHandle
		cleanupHandle = framework.AddCleanupAction(func() {
			By("Tearing down router interface")
			err := teardownRouterInterface(log, *routerID, *subnetID, *routerInterfacePortID)
			Expect(err).NotTo(HaveOccurred())
			By("Tearing down router")
			err = teardownRouter(log, *routerID)
			Expect(err).NotTo(HaveOccurred())
			By("Tearing down subnet")
			err = teardownSubnet(log, *subnetID)
			Expect(err).NotTo(HaveOccurred())
			By("Tearing down network")
			err = teardownNetwork(log, *networkID)
			Expect(err).NotTo(HaveOccurred())

			framework.RemoveCleanupAction(cleanupHandle)
		})

		providerConfig := newProviderConfig(routerID, networkID, subnetID)
		cloudProfileConfig := newCloudProfileConfig(*region, *authURL)

		err = runTest(ctx, log, c, namespace, providerConfig, decoder, cloudProfileConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("with infrastructure that uses existing network and subnet", func() {
		namespace, err := generateNamespaceName()
		Expect(err).NotTo(HaveOccurred())

		networkName := namespace + "-network"
		networkID, err := prepareNewNetwork(log, networkName)
		Expect(err).NotTo(HaveOccurred())

		subnetName := namespace + "-subnet"
		subnetID, err := prepareNewSubnet(log, subnetName, *networkID)
		Expect(err).NotTo(HaveOccurred())

		var cleanupHandle framework.CleanupActionHandle
		cleanupHandle = framework.AddCleanupAction(func() {
			By("Tearing down subnet")
			err := teardownSubnet(log, *subnetID)
			Expect(err).NotTo(HaveOccurred())
			By("Tearing down network")
			err = teardownNetwork(log, *networkID)
			Expect(err).NotTo(HaveOccurred())

			framework.RemoveCleanupAction(cleanupHandle)
		})

		providerConfig := newProviderConfig(nil, networkID, subnetID)
		cloudProfileConfig := newCloudProfileConfig(*region, *authURL)

		err = runTest(ctx, log, c, namespace, providerConfig, decoder, cloudProfileConfig)
		Expect(err).NotTo(HaveOccurred())
	})
})

func runTest(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	namespaceName string,
	providerConfig *openstackv1alpha1.InfrastructureConfig,
	decoder runtime.Decoder,
	cloudProfileConfig *openstackv1alpha1.CloudProfileConfig,
) error {
	var (
		namespace        *corev1.Namespace
		cluster          *extensionsv1alpha1.Cluster
		infra            *extensionsv1alpha1.Infrastructure
		infraIdentifiers infrastructureIdentifiers
	)

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		By("delete infrastructure")
		Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

		By("wait until infrastructure is deleted")
		err := extensions.WaitUntilExtensionObjectDeleted(
			ctx,
			c,
			log,
			infra,
			"Infrastructure",
			10*time.Second,
			16*time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())

		By("verify infrastructure deletion")
		verifyDeletion(infraIdentifiers, providerConfig)

		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())

		framework.RemoveCleanupAction(cleanupHandle)
	})

	By("create namespace for test execution")
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	if err := c.Create(ctx, namespace); err != nil {
		return err
	}

	cloudProfileConfigJSON, err := json.Marshal(&cloudProfileConfig)
	if err != nil {
		return err
	}

	cloudprofile := gardenerv1beta1.CloudProfile{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardenerv1beta1.SchemeGroupVersion.String(),
			Kind:       "CloudProfile",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
		Spec: gardenerv1beta1.CloudProfileSpec{
			ProviderConfig: &runtime.RawExtension{
				Raw: cloudProfileConfigJSON,
			},
		},
	}

	cloudProfileJSON, err := json.Marshal(&cloudprofile)
	if err != nil {
		return err
	}

	By("create cluster")
	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{
				Raw: cloudProfileJSON,
			},
			Seed: runtime.RawExtension{
				Raw: []byte("{}"),
			},
			Shoot: runtime.RawExtension{
				Raw: []byte("{}"),
			},
		},
	}
	if err := c.Create(ctx, cluster); err != nil {
		return err
	}

	By("deploy cloudprovider secret into namespace")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudprovider",
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			openstack.AuthURL:                     []byte(*authURL),
			openstack.DomainName:                  []byte(*domainName),
			openstack.Password:                    []byte(*password),
			openstack.Region:                      []byte(*region),
			openstack.TenantName:                  []byte(*tenantName),
			openstack.UserName:                    []byte(*userName),
			openstack.ApplicationCredentialID:     []byte(*appID),
			openstack.ApplicationCredentialName:   []byte(*appName),
			openstack.ApplicationCredentialSecret: []byte(*appSecret),
		},
	}
	if err := c.Create(ctx, secret); err != nil {
		return err
	}

	By("create infrastructure")
	infra, err = newInfrastructure(namespaceName, providerConfig)
	if err != nil {
		return err
	}

	if err := c.Create(ctx, infra); err != nil {
		return err
	}

	By("wait until infrastructure is created")
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		infra,
		"Infrastucture",
		10*time.Second,
		6*time.Minute,
		16*time.Minute,
		nil,
	)).To(Succeed())

	// Update the infra resource to trigger a migration.
	oldState := infra.Status.State.DeepCopy()
	if *reconciler == reconcilerMigrateTF {
		By("verifying terraform migration")
		patch := client.MergeFrom(infra.DeepCopy())
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, openstack.AnnotationKeyUseFlow, "true")
		Expect(c.Patch(ctx, infra, patch)).To(Succeed())
	} else if *reconciler == reconcilerRecoverState {
		By("drop state for testing recovery")

		patch := client.MergeFrom(infra.DeepCopy())
		infra.Status.LastOperation = nil
		infra.Status.ProviderStatus = nil
		infra.Status.State = nil
		Expect(c.Status().Patch(ctx, infra, patch)).To(Succeed())

		Expect(c.Get(ctx, client.ObjectKey{Namespace: infra.Namespace, Name: infra.Name}, infra)).To(Succeed())

		patch = client.MergeFrom(infra.DeepCopy())
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		err = c.Patch(ctx, infra, patch)
		Expect(err).To(Succeed())
	}

	By("wait until infrastructure is reconciled")
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		infra,
		"Infrastucture",
		10*time.Second,
		30*time.Second,
		16*time.Minute,
		nil,
	)).To(Succeed())

	infraIdentifiers, providerStatus := verifyCreation(infra.Status, providerConfig)

	if *reconciler == reconcilerRecoverState {
		By("check state recovery")
		Expect(infra.Status.State).To(Equal(oldState))
		newProviderStatus := openstackv1alpha1.InfrastructureStatus{}
		if _, _, err := decoder.Decode(infra.Status.ProviderStatus.Raw, nil, &newProviderStatus); err != nil {
			return err
		}
		Expect(newProviderStatus).To(Equal(providerStatus))
	}

	return nil
}

// newProviderConfig creates a providerConfig with the network and router details.
// If routerID is set to "", it requests a new router creation.
// Else it reuses the supplied routerID.
func newProviderConfig(routerID *string, networkID *string, subnetID *string) *openstackv1alpha1.InfrastructureConfig {
	var router *openstackv1alpha1.Router

	if routerID != nil {
		router = &openstackv1alpha1.Router{ID: *routerID}
	}

	return &openstackv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		FloatingPoolName: *floatingPoolName,
		Networks: openstackv1alpha1.Networks{
			ID:       networkID,
			Router:   router,
			Workers:  vpcCIDR,
			SubnetID: subnetID,
		},
	}
}

func newCloudProfileConfig(region string, authURL string) *openstackv1alpha1.CloudProfileConfig {
	return &openstackv1alpha1.CloudProfileConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CloudProfileConfig",
		},
		KeyStoneURLs: []openstackv1alpha1.KeyStoneURL{
			{
				Region: region,
				URL:    authURL,
			},
		},
	}
}

func newInfrastructure(namespace string, providerConfig *openstackv1alpha1.InfrastructureConfig) (*extensionsv1alpha1.Infrastructure, error) {
	const sshPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== your_email@example.com"

	providerConfigJSON, err := json.Marshal(&providerConfig)
	if err != nil {
		return nil, err
	}

	infra := &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infrastructure",
			Namespace: namespace,
			Labels: map[string]string{
				"test-id": testId,
			},
		},
		Spec: extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: openstack.Type,
				ProviderConfig: &runtime.RawExtension{
					Raw: providerConfigJSON,
				},
			},
			SecretRef: corev1.SecretReference{
				Name:      "cloudprovider",
				Namespace: namespace,
			},
			Region:       *region,
			SSHPublicKey: []byte(sshPublicKey),
		},
	}
	if usesFlow(reconciler) {
		infra.Annotations = map[string]string{openstack.AnnotationKeyUseFlow: "true"}
	}
	return infra, nil
}

func generateNamespaceName() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return "openstack--infra-it--" + suffix, nil
}

func prepareNewRouter(log logr.Logger, routerName string) (*string, error) {
	log.Info("Waiting until router is created", "routerName", routerName)

	createOpts := routers.CreateOpts{
		Name: routerName,
		GatewayInfo: &routers.GatewayInfo{
			NetworkID: floatingPoolID,
		},
	}
	router, err := networkClient.CreateRouter(createOpts)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Router is created", "routerName", routerName)
	return &router.ID, nil
}

func teardownRouter(log logr.Logger, routerID string) error {
	log.Info("Waiting until router is deleted", "routerID", routerID)

	err := networkClient.DeleteRouter(routerID)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Router is deleted", "routerID", routerID)
	return nil
}

func prepareNewSubnet(log logr.Logger, subnetName string, networkID string) (*string, error) {
	log.Info("Waiting until subnet is created", "subnetName", subnetName)

	createOpts := subnets.CreateOpts{
		Name:      subnetName,
		NetworkID: networkID,
		IPVersion: gophercloud.IPv4,
		CIDR:      vpcCIDR,
	}
	subnet, err := networkClient.CreateSubnet(createOpts)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Subnet is created", "subnetName", subnet)
	return &subnet.ID, nil
}

func teardownSubnet(log logr.Logger, subnetID string) error {
	log.Info("Waiting until subnet is deleted", "subnetID", subnetID)

	err := networkClient.DeleteSubnet(subnetID)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Subnet is deleted", "subnetID", subnetID)
	return nil
}

func prepareNewNetwork(log logr.Logger, networkName string) (*string, error) {
	log.Info("Waiting until network is created", "networkName", networkName)

	createOpts := networks.CreateOpts{
		Name: networkName,
	}
	network, err := networkClient.CreateNetwork(createOpts)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Network is created", "networkName", networkName)
	return &network.ID, nil
}

func teardownNetwork(log logr.Logger, networkID string) error {
	log.Info("Waiting until network is deleted", "networkID", networkID)

	err := networkClient.DeleteNetwork(networkID)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Network is deleted", "networkID", networkID)
	return nil
}

func prepareNewRouterInterface(log logr.Logger, routerID, subnetID string) (*string, error) {
	log.Info("Waiting until router interface is created", "routerID", routerID, "subnetID", subnetID)

	routerInterface, err := networkClient.AddRouterInterface(routerID, routers.AddInterfaceOpts{SubnetID: subnetID})
	Expect(err).NotTo(HaveOccurred())

	log.Info("Router interface is created", "interfaceID", routerInterface.ID)
	return &routerInterface.PortID, nil
}

func teardownRouterInterface(log logr.Logger, routerID, subnetID, portID string) error {
	log.Info("Waiting until router interface is deleted", "routerID", routerID, "subnetID", subnetID, "portID", portID)

	_, err := networkClient.RemoveRouterInterface(routerID, routers.RemoveInterfaceOpts{SubnetID: subnetID, PortID: portID})
	Expect(err).NotTo(HaveOccurred())

	log.Info("Router interface is deleted", "routerID", routerID)
	return nil
}

type infrastructureIdentifiers struct {
	networkID  *string
	keyPair    *string
	subnetID   *string
	secGroupID *string
	routerID   *string
}

func verifyCreation(infraStatus extensionsv1alpha1.InfrastructureStatus, providerConfig *openstackv1alpha1.InfrastructureConfig) (infrastructureIdentifier infrastructureIdentifiers, providerStatus openstackv1alpha1.InfrastructureStatus) {
	_, _, err := decoder.Decode(infraStatus.ProviderStatus.Raw, nil, &providerStatus)
	Expect(err).NotTo(HaveOccurred())

	// router exists
	router, err := networkClient.GetRouterByID(providerStatus.Networks.Router.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(router.Status).To(Equal("ACTIVE"))
	infrastructureIdentifier.routerID = ptr.To(router.ID)

	// verify router ip in status
	Expect(router.GatewayInfo.ExternalFixedIPs).NotTo(BeEmpty())
	Expect(providerStatus.Networks.Router.IP).To(Equal(router.GatewayInfo.ExternalFixedIPs[0].IPAddress))

	// network is created
	net, err := networkClient.GetNetworkByID(providerStatus.Networks.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(net).NotTo(BeNil())

	if providerConfig.Networks.ID != nil {
		Expect(net.ID).To(Equal(*providerConfig.Networks.ID))
	}
	infrastructureIdentifier.networkID = ptr.To(net.ID)

	// subnet exists
	subnet, err := networkClient.GetSubnetByID(providerStatus.Networks.Subnets[0].ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(subnet.CIDR).To(Equal(providerConfig.Networks.Workers))
	infrastructureIdentifier.subnetID = ptr.To(subnet.ID)

	// router interface exists
	port, err := networkClient.GetRouterInterfacePort(router.ID, subnet.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(port).NotTo(BeNil())

	// security group is created
	secGroup, err := networkClient.GetSecurityGroup(providerStatus.SecurityGroups[0].ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(secGroup.Name).To(Equal(providerStatus.SecurityGroups[0].Name))
	infrastructureIdentifier.secGroupID = ptr.To(secGroup.ID)

	// keypair is created
	keyPair, err := computeClient.GetKeyPair(providerStatus.Node.KeyName)
	Expect(err).NotTo(HaveOccurred())
	infrastructureIdentifier.keyPair = ptr.To(keyPair.Name)

	// verify egressCIDRs
	expectedCIDRDs := []string{providerStatus.Networks.Router.IP + "/32"}
	Expect(infraStatus.EgressCIDRs).To(Equal(expectedCIDRDs))

	return infrastructureIdentifier, providerStatus
}

func verifyDeletion(infrastructureIdentifier infrastructureIdentifiers, providerConfig *openstackv1alpha1.InfrastructureConfig) {
	// keypair doesn't exist
	keyPair, _ := computeClient.GetKeyPair(ptr.Deref(infrastructureIdentifier.keyPair, ""))
	Expect(keyPair).To(BeNil())

	if infrastructureIdentifier.subnetID != nil {
		// subnet doesn't exist
		subnetsOpts := subnets.ListOpts{ID: ptr.Deref(infrastructureIdentifier.subnetID, "")}
		subnets, err := networkClient.ListSubnets(subnetsOpts)

		if providerConfig.Networks.SubnetID != nil {
			Expect(subnets).To(HaveLen(1))
			Expect(subnets[0].ID).To(Equal(*providerConfig.Networks.SubnetID))

		} else {
			Expect(openstackclient.IgnoreNotFoundError(err)).NotTo(HaveOccurred())
			Expect(subnets).To(BeEmpty())
		}
	}

	if infrastructureIdentifier.networkID != nil {
		if providerConfig.Networks.ID == nil {
			// make sure network doesn't exist, if it wasn't present before
			opts := networks.ListOpts{ID: ptr.Deref(infrastructureIdentifier.networkID, "")}
			networks, err := networkClient.ListNetwork(opts)
			Expect(openstackclient.IgnoreNotFoundError(err)).NotTo(HaveOccurred())
			Expect(networks).To(BeEmpty())
		}
	}

	if infrastructureIdentifier.secGroupID != nil {
		// security group doesn't exist
		sGroupsOpts := groups.ListOpts{ID: ptr.Deref(infrastructureIdentifier.secGroupID, "")}
		sGroups, err := networkClient.ListSecurityGroup(sGroupsOpts)
		Expect(openstackclient.IgnoreNotFoundError(err)).NotTo(HaveOccurred())
		Expect(sGroups).To(BeEmpty())
	}

	if infrastructureIdentifier.routerID != nil {
		if providerConfig.Networks.Router == nil {
			// make sure router doesn't exist, if it wasn't present in the start of test
			opts := routers.ListOpts{ID: ptr.Deref(infrastructureIdentifier.routerID, "")}
			routers, err := networkClient.ListRouters(opts)
			Expect(openstackclient.IgnoreNotFoundError(err)).NotTo(HaveOccurred())
			Expect(routers).To(BeEmpty())
		}
	}
}

func usesFlow(reconciler *string) bool {
	if rec := ptr.Deref(reconciler, reconcilerUseTF); rec == reconcilerUseTF || rec == reconcilerMigrateTF {
		return false
	}

	return true
}
