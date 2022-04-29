// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"context"
	"encoding/json"
	"flag"
	"path/filepath"
	"time"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	vpcCIDR = "10.250.0.0/16"
)

var (
	authURL          = flag.String("auth-url", "", "Authorization URL for openstack")
	domainName       = flag.String("domain-name", "", "Domain name for openstack")
	floatingPoolName = flag.String("floating-pool-name", "", "Floating pool name for creating router")
	password         = flag.String("password", "", "Password for openstack")
	region           = flag.String("region", "", "Openstack region")
	tenantName       = flag.String("tenant-name", "", "Tenant name for openstack")
	userName         = flag.String("user-name", "", "User name for openstack")
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
	if len(*password) == 0 {
		panic("--password flag is not specified")
	}
	if len(*region) == 0 {
		panic("--region flag is not specified")
	}
	if len(*tenantName) == 0 {
		panic("--tenant-name flag is not specified")
	}
	if len(*userName) == 0 {
		panic("--user-name flag is not specified")
	}
}

var (
	ctx    = context.Background()
	logger *logrus.Entry

	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client

	decoder         runtime.Decoder
	openstackClient *OpenstackClient
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	internalChartsPath := openstack.InternalChartsPath
	repoRoot := filepath.Join("..", "..", "..")
	openstack.InternalChartsPath = filepath.Join(repoRoot, openstack.InternalChartsPath)

	// enable manager logs
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	log := logrus.New()
	log.SetOutput(GinkgoWriter)
	logger = logrus.NewEntry(log)

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())

		openstack.InternalChartsPath = internalChartsPath
	})

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_infrastructures.yaml"),
			},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	By("setup manager")
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(openstackinstall.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(infrastructure.AddToManagerWithOptions(mgr, infrastructure.AddOptions{
		// During testing in testmachinery cluster, there is no gardener-resource-manager to inject the volume mount.
		// Hence, we need to run without projected token mount.
		DisableProjectedTokenMount: true,
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

	openstackClient, err = NewOpenstackClient(*authURL, *domainName, *floatingPoolName, *password, *region, *tenantName, *userName)
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Infrastructure tests", func() {
	Context("with infrastructure that requests new private network", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			providerConfig := newProviderConfig("", nil)
			cloudProfileConfig := newCloudProfileConfig(openstackClient.Region, openstackClient.AuthURL)
			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			err = runTest(ctx, logger, c, namespace, providerConfig, decoder, openstackClient, cloudProfileConfig)

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with infrastructure that uses existing router", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			cloudRouterName := namespace + "-cloud-router"

			routerID, err := prepareNewRouter(ctx, logger, cloudRouterName, openstackClient)
			Expect(err).NotTo(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				err := teardownRouter(ctx, logger, *routerID, openstackClient)
				Expect(err).NotTo(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			providerConfig := newProviderConfig(*routerID, nil)
			cloudProfileConfig := newCloudProfileConfig(openstackClient.Region, openstackClient.AuthURL)

			err = runTest(ctx, logger, c, namespace, providerConfig, decoder, openstackClient, cloudProfileConfig)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with infrastructure that uses existing network", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			networkName := namespace + "-network"

			networkID, err := prepareNewNetwork(ctx, logger, networkName, openstackClient)
			Expect(err).NotTo(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				err := teardownNetwork(ctx, logger, *networkID, openstackClient)
				Expect(err).NotTo(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			providerConfig := newProviderConfig("", networkID)
			cloudProfileConfig := newCloudProfileConfig(openstackClient.Region, openstackClient.AuthURL)

			err = runTest(ctx, logger, c, namespace, providerConfig, decoder, openstackClient, cloudProfileConfig)

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with infrastructure that uses existing network and router", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			networkName := namespace + "-network"
			cloudRouterName := namespace + "-cloud-router"

			networkID, err := prepareNewNetwork(ctx, logger, networkName, openstackClient)
			Expect(err).NotTo(HaveOccurred())
			routerID, err := prepareNewRouter(ctx, logger, cloudRouterName, openstackClient)
			Expect(err).NotTo(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				By("Tearing down network")
				err := teardownNetwork(ctx, logger, *networkID, openstackClient)
				Expect(err).NotTo(HaveOccurred())

				By("Tearing down router")
				err = teardownRouter(ctx, logger, *routerID, openstackClient)
				Expect(err).NotTo(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			providerConfig := newProviderConfig(*routerID, networkID)
			cloudProfileConfig := newCloudProfileConfig(openstackClient.Region, openstackClient.AuthURL)

			err = runTest(ctx, logger, c, namespace, providerConfig, decoder, openstackClient, cloudProfileConfig)

			Expect(err).NotTo(HaveOccurred())

		})
	})
})

func runTest(
	ctx context.Context,
	logger *logrus.Entry,
	c client.Client,
	namespaceName string,
	providerConfig *openstackv1alpha1.InfrastructureConfig,
	decoder runtime.Decoder,
	openstackClient *OpenstackClient,
	cloudProfileConfig *openstackv1alpha1.CloudProfileConfig,
) error {
	var (
		namespace                 *corev1.Namespace
		cluster                   *extensionsv1alpha1.Cluster
		infra                     *extensionsv1alpha1.Infrastructure
		infrastructureIdentifiers infrastructureIdentifiers
	)

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		By("delete infrastructure")
		Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

		By("wait until infrastructure is deleted")
		err := extensions.WaitUntilExtensionObjectDeleted(
			ctx,
			c,
			logger,
			infra,
			"Infrastructure",
			10*time.Second,
			16*time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())

		By("verify infrastructure deletion")
		verifyDeletion(ctx, openstackClient, infrastructureIdentifiers, providerConfig)

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
			openstack.AuthURL:    []byte(*authURL),
			openstack.DomainName: []byte(*domainName),
			openstack.Password:   []byte(*password),
			openstack.Region:     []byte(*region),
			openstack.TenantName: []byte(*tenantName),
			openstack.UserName:   []byte(*userName),
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
	if err := extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		logger,
		infra,
		extensionsv1alpha1.InfrastructureResource,
		10*time.Second,
		30*time.Second,
		16*time.Minute,
		nil,
	); err != nil {
		return err
	}

	By("decode infrastucture status")
	if err := c.Get(ctx, client.ObjectKey{Namespace: infra.Namespace, Name: infra.Name}, infra); err != nil {
		return err
	}

	providerStatus := &openstackv1alpha1.InfrastructureStatus{}
	if _, _, err := decoder.Decode(infra.Status.ProviderStatus.Raw, nil, providerStatus); err != nil {
		return err
	}

	By("verify infrastructure creation")
	infrastructureIdentifiers = verifyCreation(ctx, openstackClient, infra, providerStatus, providerConfig, pointer.StringPtr(vpcCIDR))

	return nil
}

// newProviderConfig creates a providerConfig with the network and router details.
// If routerID is set to "", it requests a new router creation.
// Else it reuses the suppiled routerID.
func newProviderConfig(routerID string, networkID *string) *openstackv1alpha1.InfrastructureConfig {
	var router *openstackv1alpha1.Router

	if routerID != "" {
		router = &openstackv1alpha1.Router{ID: routerID}
	}

	return &openstackv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		FloatingPoolName: *floatingPoolName,
		Networks: openstackv1alpha1.Networks{
			ID:      networkID,
			Router:  router,
			Workers: vpcCIDR,
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

	return &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infrastructure",
			Namespace: namespace,
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
	}, nil
}

func generateNamespaceName() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return "openstack--infra-it--" + suffix, nil
}

func prepareNewRouter(ctx context.Context, logger *logrus.Entry, routerName string, openstackClient *OpenstackClient) (*string, error) {
	logger.Infof("Waiting until router '%s' is created...", routerName)

	createOpts := routers.CreateOpts{
		Name: routerName,
	}
	router, err := routers.Create(openstackClient.NetworkingClient, createOpts).Extract()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Router '%s' is created...", routerName)
	return &router.ID, nil
}

func teardownRouter(ctx context.Context, logger *logrus.Entry, routerID string, openstackClient *OpenstackClient) error {
	logger.Infof("Waiting until router '%s' is deleted...", routerID)

	err := routers.Delete(openstackClient.NetworkingClient, routerID).ExtractErr()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Router '%s' is deleted...", routerID)
	return nil
}

func prepareNewNetwork(ctx context.Context, logger *logrus.Entry, networkName string, openstackClient *OpenstackClient) (*string, error) {
	logger.Infof("Waiting until network '%s' is created...", networkName)

	createOpts := networks.CreateOpts{
		Name: networkName,
	}
	network, err := networks.Create(openstackClient.NetworkingClient, createOpts).Extract()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Network '%s' is created...", networkName)
	return &network.ID, nil
}

func teardownNetwork(ctx context.Context, logger *logrus.Entry, networkID string, openstackClient *OpenstackClient) error {
	logger.Infof("Waiting until network '%s' is deleted...", networkID)

	err := networks.Delete(openstackClient.NetworkingClient, networkID).ExtractErr()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Network '%s' is deleted...", networkID)
	return nil
}

type infrastructureIdentifiers struct {
	networkID  *string
	keyPair    *string
	subnetID   *string
	secGroupID *string
	routerID   *string
}

func verifyCreation(
	ctx context.Context,
	openstackClient *OpenstackClient,
	infra *extensionsv1alpha1.Infrastructure,
	infraStatus *openstackv1alpha1.InfrastructureStatus,
	providerConfig *openstackv1alpha1.InfrastructureConfig,
	cidr *string,
) (infrastructureIdentifier infrastructureIdentifiers) {
	// router exists
	router, err := routers.Get(openstackClient.NetworkingClient, infraStatus.Networks.Router.ID).Extract()
	Expect(err).NotTo(HaveOccurred())
	Expect(router.Status).To(Equal("ACTIVE"))
	infrastructureIdentifier.routerID = &router.ID

	// network is created
	net, err := networks.Get(openstackClient.NetworkingClient, infraStatus.Networks.ID).Extract()
	Expect(err).NotTo(HaveOccurred())

	if providerConfig.Networks.ID != nil {
		Expect(net.ID).To(Equal(*providerConfig.Networks.ID))
	}
	infrastructureIdentifier.networkID = &net.ID

	// subnet is created
	subnet, err := subnets.Get(openstackClient.NetworkingClient, infraStatus.Networks.Subnets[0].ID).Extract()
	Expect(err).NotTo(HaveOccurred())
	Expect(subnet.CIDR).To(Equal(providerConfig.Networks.Workers))
	infrastructureIdentifier.subnetID = &subnet.ID

	// security group is created
	secGroup, err := groups.Get(openstackClient.NetworkingClient, infraStatus.SecurityGroups[0].ID).Extract()
	Expect(err).NotTo(HaveOccurred())
	Expect(secGroup.Name).To(Equal(infraStatus.SecurityGroups[0].Name))
	infrastructureIdentifier.secGroupID = &secGroup.ID

	// keypair is created
	keyPair, err := keypairs.Get(openstackClient.ComputeClient, infraStatus.Node.KeyName).Extract()
	Expect(err).NotTo(HaveOccurred())
	infrastructureIdentifier.keyPair = &keyPair.Name

	return infrastructureIdentifier
}

func verifyDeletion(
	ctx context.Context,
	openstackClient *OpenstackClient,
	infrastructureIdentifier infrastructureIdentifiers,
	providerConfig *openstackv1alpha1.InfrastructureConfig,
) {
	// keypair doesn't exist
	_, err := keypairs.Get(openstackClient.ComputeClient, *infrastructureIdentifier.keyPair).Extract()
	Expect(err).To(HaveOccurred())

	_, err = networks.Get(openstackClient.NetworkingClient, *infrastructureIdentifier.networkID).Extract()
	if providerConfig.Networks.ID == nil {
		// make sure network doesn't exist, if it wasn't present before
		Expect(err).To(HaveOccurred())
	}

	// subnet doesn't exist
	_, err = subnets.Get(openstackClient.NetworkingClient, *infrastructureIdentifier.subnetID).Extract()
	Expect(err).To(HaveOccurred())

	// security group doesn't exist
	_, err = groups.Get(openstackClient.NetworkingClient, *infrastructureIdentifier.secGroupID).Extract()
	Expect(err).To(HaveOccurred())

	_, err = routers.Get(openstackClient.NetworkingClient, *infrastructureIdentifier.routerID).Extract()
	if providerConfig.Networks.Router == nil {
		// make sure router doesn't exist, if it wasn't present in the start of test
		Expect(err).To(HaveOccurred())
	}
}
