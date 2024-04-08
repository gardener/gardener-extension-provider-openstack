// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	bastionctrl "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/bastion"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

var (
	authURL          = flag.String("auth-url", "", "Authorization URL for openstack")
	domainName       = flag.String("domain-name", "", "Domain name for openstack")
	floatingPoolName = flag.String("floating-pool-name", "", "Floating pool name for creating router")
	region           = flag.String("region", "", "Openstack region")
	tenantName       = flag.String("tenant-name", "", "Tenant name for openstack")
	userName         = flag.String("user-name", "", "User name for openstack")
	password         = flag.String("password", "", "Password for openstack")
	appID            = flag.String("app-id", "", "ApplicationCredentialID for openstack")
	appName          = flag.String("app-name", "", "ApplicationCredentialName for openstack")
	appSecret        = flag.String("app-secret", "", "ApplicationCredentialSecret for openstack")
	flavorRef        = flag.String("flavor-ref", "", "Operating System flavour reference for openstack")
	imageRef         = flag.String("image-ref", "", "Image reference for openstack")

	userDataConst = "IyEvYmluL2Jhc2ggLWV1CmlkIGdhcmRlbmVyIHx8IHVzZXJhZGQgZ2FyZGVuZXIgLW1VCm1rZGlyIC1wIC9ob21lL2dhcmRlbmVyLy5zc2gKZWNobyAic3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFDazYyeDZrN2orc0lkWG9TN25ITzRrRmM3R0wzU0E2UmtMNEt4VmE5MUQ5RmxhcmtoRzFpeU85WGNNQzZqYnh4SzN3aWt0M3kwVTBkR2h0cFl6Vjh3YmV3Z3RLMWJBWnl1QXJMaUhqbnJnTFVTRDBQazNvWGh6RkpKN0MvRkxNY0tJZFN5bG4vMENKVkVscENIZlU5Y3dqQlVUeHdVQ2pnVXRSYjdZWHN6N1Y5dllIVkdJKzRLaURCd3JzOWtVaTc3QWMyRHQ1UzBJcit5dGN4b0p0bU5tMWgxTjNnNzdlbU8rWXhtWEo4MzFXOThoVFVTeFljTjNXRkhZejR5MWhrRDB2WHE1R1ZXUUtUQ3NzRE1wcnJtN0FjQTBCcVRsQ0xWdWl3dXVmTEJLWGhuRHZRUEQrQ2Jhbk03bUZXRXdLV0xXelZHME45Z1VVMXE1T3hhMzhvODUgbWVAbWFjIiA+IC9ob21lL2dhcmRlbmVyLy5zc2gvYXV0aG9yaXplZF9rZXlzCmNob3duIGdhcmRlbmVyOmdhcmRlbmVyIC9ob21lL2dhcmRlbmVyLy5zc2gvYXV0aG9yaXplZF9rZXlzCmVjaG8gImdhcmRlbmVyIEFMTD0oQUxMKSBOT1BBU1NXRDpBTEwiID4vZXRjL3N1ZG9lcnMuZC85OS1nYXJkZW5lci11c2VyCg=="
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
	if len(*flavorRef) == 0 {
		panic("--flavorRef flag is not specified")
	}
	if len(*imageRef) == 0 {
		panic("--imageRef flag is not specified")
	}
	err := openstack.ValidateSecrets(*userName, *password, *appID, *appName, *appSecret)
	if err != nil {
		panic(fmt.Errorf("flag error: %w", err))
	}
}

var (
	ctx     = context.Background()
	log     logr.Logger
	testEnv *envtest.Environment

	extensionscluster *extensionsv1alpha1.Cluster
	controllercluster *controller.Cluster
	options           *bastionctrl.Options
	bastion           *extensionsv1alpha1.Bastion
	secret            *corev1.Secret

	mgrCancel   context.CancelFunc
	c           client.Client
	bastionName string

	networkClient openstackclient.Networking
	computeClient openstackclient.Compute
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	log = logf.Log.WithName("bastion-test")

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_bastions.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_workers.yaml"),
			},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	By("setup manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(openstackinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(bastionctrl.AddToManagerWithOptions(mgr, bastionctrl.AddOptions{
		BastionConfig: config.BastionConfig{
			ImageRef:  *imageRef,
			FlavorRef: *flavorRef,
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

	randString, err := randomString()
	Expect(err).NotTo(HaveOccurred())

	bastionName = fmt.Sprintf("openstack-it-bastion-%s", randString)

	extensionscluster, controllercluster = createClusters(bastionName)
	bastion, options = createBastion(controllercluster, bastionName)

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudprovider",
			Namespace: bastionName,
		},
		Data: map[string][]byte{
			openstack.AuthURL:                     []byte(*authURL),
			openstack.DomainName:                  []byte(*domainName),
			openstack.Region:                      []byte(*region),
			openstack.TenantName:                  []byte(*tenantName),
			openstack.UserName:                    []byte(*userName),
			openstack.Password:                    []byte(*password),
			openstack.ApplicationCredentialID:     []byte(*appID),
			openstack.ApplicationCredentialName:   []byte(*appName),
			openstack.ApplicationCredentialSecret: []byte(*appSecret),
		},
	}

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
})

var _ = AfterSuite(func() {
	defer func() {
		By("stopping manager")
		mgrCancel()
	}()

	By("running cleanup actions")
	framework.RunCleanupActions()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

var _ = Describe("Bastion tests", func() {
	It("should successfully create and delete", func() {
		cloudRouterName := bastionName + "-cloud-router"
		subnetName := bastionName + "-subnet"

		By("setup Infrastructure ")
		shootSecurityGroupID, err := prepareShootSecurityGroup(bastionName)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			By("Tearing down Shoot Security Group")
			err = teardownShootSecurityGroup(shootSecurityGroupID)
			Expect(err).NotTo(HaveOccurred())
		})

		networkID, err := prepareNewNetwork(bastionName)
		Expect(err).NotTo(HaveOccurred())

		subNetID, err := prepareSubNet(subnetName, networkID)
		Expect(err).NotTo(HaveOccurred())

		routerID, externalNetworkID, err := prepareNewRouter(cloudRouterName, subNetID)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			By("Tearing down network")
			err := teardownNetwork(networkID, routerID, subNetID)
			Expect(err).NotTo(HaveOccurred())

			By("Tearing down router")
			err = teardownRouter(routerID)
			Expect(err).NotTo(HaveOccurred())
		})

		infraStatus := createInfrastructureStatus(shootSecurityGroupID, networkID, routerID, externalNetworkID, subNetID)
		worker, err := createWorker(bastionName, infraStatus)
		Expect(err).NotTo(HaveOccurred())

		By("create namespace, cluster, secret, worker")
		setupEnvironmentObjects(ctx, c, namespace(bastionName), secret, extensionscluster, worker)

		framework.AddCleanupAction(func() {
			teardownShootEnvironment(ctx, c, namespace(bastionName), secret, extensionscluster, worker)
		})

		By("setup bastion")
		err = c.Create(ctx, bastion)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			By("Tearing down bastion")
			teardownBastion(ctx, c, bastion)
			By("verify bastion deletion")
			verifyDeletion(bastionName)
		})

		By("wait until bastion is reconciled")
		Expect(extensions.WaitUntilExtensionObjectReady(
			ctx,
			c,
			log,
			bastion,
			extensionsv1alpha1.BastionResource,
			60*time.Second,
			120*time.Second,
			10*time.Minute,
			nil,
		)).To(Succeed())

		err = retry(10, 5*time.Second, func() error {
			return verifyPort22IsOpen(ctx, c, bastion)
		})
		Expect(err).NotTo(HaveOccurred())
		verifyPort42IsClosed(ctx, c, bastion)

		By("verify cloud resources")
		verifyCreation(options)
	})
})

func randomString() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return suffix, nil
}

func verifyPort22IsOpen(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) error {
	By("check connection to port 22 open should not error")
	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "22")
	conn, err := net.DialTimeout("tcp4", address, 10*time.Second)
	if err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("connection should not be nil")
	}
	return nil
}

func verifyPort42IsClosed(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("check connection to port 42 which should fail")

	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "42")
	conn, err := net.DialTimeout("tcp4", address, 3*time.Second)
	Expect(err).Should(HaveOccurred())
	Expect(conn).To(BeNil())
}

func prepareNewRouter(routerName, subnetID string) (routerID, floatingPoolID string, err error) {
	log.Info("Waiting until router is created", "routerName", routerName)

	externalNetwork, err := networkClient.GetExternalNetworkByName(*floatingPoolName)
	Expect(err).NotTo(HaveOccurred())

	createOpts := routers.CreateOpts{
		Name:         routerName,
		AdminStateUp: ptr.To(true),
		GatewayInfo: &routers.GatewayInfo{
			NetworkID: externalNetwork.ID,
		},
	}
	router, err := networkClient.CreateRouter(createOpts)
	Expect(err).NotTo(HaveOccurred())

	intOpts := routers.AddInterfaceOpts{
		SubnetID: subnetID,
	}
	_, err = networkClient.AddRouterInterface(router.ID, intOpts)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Router is created", "routerName", routerName)
	return router.ID, externalNetwork.ID, nil
}

func teardownRouter(routerID string) error {
	log.Info("Waiting until router is deleted", "routerID", routerID)

	err := networkClient.DeleteRouter(routerID)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Router is deleted", "routerID", routerID)
	return nil
}

func prepareNewNetwork(networkName string) (string, error) {
	log.Info("Waiting until network is created", "networkName", networkName)

	network, err := networkClient.CreateNetwork(networks.CreateOpts{
		Name: networkName,
	})
	Expect(err).NotTo(HaveOccurred())

	log.Info("Network is created", "networkName", networkName)
	return network.ID, nil
}

func prepareSubNet(subnetName, networkID string) (string, error) {
	log.Info("Waiting until Subnet is created", "subnetName", subnetName)

	createOpts := subnets.CreateOpts{
		Name:      subnetName,
		NetworkID: networkID,
		IPVersion: 4,
		CIDR:      "10.180.0.0/16",
		GatewayIP: ptr.To("10.180.0.1"),
		AllocationPools: []subnets.AllocationPool{
			{
				Start: "10.180.0.2",
				End:   "10.180.255.254",
			},
		},
	}
	subnet, err := networkClient.CreateSubnet(createOpts)
	Expect(err).NotTo(HaveOccurred())
	log.Info("Subnet is created", "subnetName", subnetName)
	return subnet.ID, nil
}

// prepareShootSecurityGroup create fake shoot security group which will be used in EgressAllowSSHToWorker remoteGroupID
func prepareShootSecurityGroup(shootSgName string) (string, error) {
	log.Info("Waiting until Shoot Security Group is created", "shootSecurityGroupName", shootSgName)

	sGroup, err := networkClient.CreateSecurityGroup(groups.CreateOpts{
		Name: shootSgName,
	})
	Expect(err).NotTo(HaveOccurred())
	log.Info("Shoot Security Group is created", "shootSecurityGroupName", shootSgName)
	return sGroup.ID, nil
}

func teardownShootSecurityGroup(groupID string) error {
	err := networkClient.DeleteSecurityGroup(groupID)
	Expect(err).NotTo(HaveOccurred())
	log.Info("Shoot Security Group is deleted", "shootSecurityGroupID", groupID)
	return nil
}

func teardownNetwork(networkID, routerID, subnetID string) error {
	log.Info("Waiting until network is deleted", "networkID", networkID)

	_, err := networkClient.RemoveRouterInterface(routerID, routers.RemoveInterfaceOpts{SubnetID: subnetID})
	Expect(err).NotTo(HaveOccurred())

	err = networkClient.DeleteNetwork(networkID)
	Expect(err).NotTo(HaveOccurred())

	log.Info("Network is deleted", "networkID", networkID)
	return nil
}

func setupEnvironmentObjects(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster, worker *extensionsv1alpha1.Worker) {
	Expect(c.Create(ctx, namespace)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
	Expect(c.Create(ctx, secret)).To(Succeed())
	Expect(c.Create(ctx, worker)).To(Succeed())
}

func teardownShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster, worker *extensionsv1alpha1.Worker) {
	workerCopy := worker.DeepCopy()
	metav1.SetMetaDataAnnotation(&worker.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
	Expect(c.Patch(ctx, worker, client.MergeFrom(workerCopy))).To(Succeed())

	Expect(client.IgnoreNotFound(c.Delete(ctx, worker))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func createClusters(name string) (*extensionsv1alpha1.Cluster, *controller.Cluster) {
	infrastructureConfig := createInfrastructureConfig()
	infrastructureConfigJSON, _ := json.Marshal(&infrastructureConfig)

	shoot := createShoot(infrastructureConfigJSON)
	shootJSON, _ := json.Marshal(shoot)

	seed := createSeed()
	seedJSON, _ := json.Marshal(seed)

	cloudProfile := createCloudProfile()
	cloudProfileJSON, _ := json.Marshal(cloudProfile)

	extensionscluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{
				Object: cloudProfile,
				Raw:    cloudProfileJSON,
			},
			Shoot: runtime.RawExtension{
				Object: shoot,
				Raw:    shootJSON,
			},
			Seed: runtime.RawExtension{
				Object: seed,
				Raw:    seedJSON,
			},
		},
	}

	cluster := &controller.Cluster{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Shoot:        shoot,
		CloudProfile: cloudProfile,
	}
	return extensionscluster, cluster
}

func createWorker(name string, infraStatus *openstackv1alpha1.InfrastructureStatus) (*extensionsv1alpha1.Worker, error) {
	infrastructureStatusJSON, err := json.Marshal(&infraStatus)
	if err != nil {
		return nil, err
	}

	return &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
		Spec: extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: openstack.Type,
			},
			Pools: []extensionsv1alpha1.WorkerPool{},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: infrastructureStatusJSON,
			},
			Region: *region,
			SecretRef: corev1.SecretReference{
				Name:      name,
				Namespace: name,
			},
		},
	}, nil
}

func createInfrastructureConfig() *openstackv1alpha1.InfrastructureConfig {
	return &openstackv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		FloatingPoolSubnetName: floatingPoolName,
	}
}

func createInfrastructureStatus(securityGroupID, networkID, routerID, externalNetworkID, subnetID string) *openstackv1alpha1.InfrastructureStatus {
	return &openstackv1alpha1.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
		SecurityGroups: []openstackv1alpha1.SecurityGroup{
			{
				Purpose: openstackv1alpha1.PurposeNodes,
				ID:      securityGroupID,
			},
		},
		Networks: openstackv1alpha1.NetworkStatus{
			ID: networkID,
			FloatingPool: openstackv1alpha1.FloatingPoolStatus{
				ID:   externalNetworkID,
				Name: *floatingPoolName,
			},
			Router: openstackv1alpha1.RouterStatus{
				ID: routerID,
			},
			Subnets: []openstackv1alpha1.Subnet{
				{
					ID:      subnetID,
					Purpose: openstackv1alpha1.PurposeNodes,
				},
			},
		},
	}
}

func createSeed() *gardencorev1beta1.Seed {
	return &gardencorev1beta1.Seed{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core.gardener.cloud/v1beta1",
			Kind:       "Seed",
		},
		Spec: gardencorev1beta1.SeedSpec{},
	}
}

func createShoot(infrastructureConfig []byte) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name: bastionName,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core.gardener.cloud/v1beta1",
			Kind:       "Shoot",
		},
		Spec: gardencorev1beta1.ShootSpec{
			Region:            *region,
			SecretBindingName: ptr.To(v1beta1constants.SecretNameCloudProvider),
			Provider: gardencorev1beta1.Provider{
				InfrastructureConfig: &runtime.RawExtension{
					Raw: infrastructureConfig,
				}},
		},
	}
}

func createCloudProfile() *gardencorev1beta1.CloudProfile {
	cloudProfile := &gardencorev1beta1.CloudProfile{
		Spec: gardencorev1beta1.CloudProfileSpec{},
	}
	return cloudProfile
}

func createBastion(cluster *controller.Cluster, name string) (*extensionsv1alpha1.Bastion, *bastionctrl.Options) {
	bastion := &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-bastion",
			Namespace: name,
		},
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: openstack.Type,
			},
			UserData: []byte(userDataConst),
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{
					IPBlock: networkingv1.IPBlock{
						CIDR: "0.0.0.0/0",
					},
				},
			},
		},
	}

	options, err := bastionctrl.DetermineOptions(bastion, cluster)
	Expect(err).NotTo(HaveOccurred())

	return bastion, options
}

func teardownBastion(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("delete bastion")
	Expect(client.IgnoreNotFound(c.Delete(ctx, bastion))).To(Succeed())

	By("wait until bastion is deleted")
	err := extensions.WaitUntilExtensionObjectDeleted(ctx, c, log, bastion, extensionsv1alpha1.BastionResource, 10*time.Second, 16*time.Minute)
	Expect(err).NotTo(HaveOccurred())
}

func verifyDeletion(name string) {
	// bastion public ip should be gone
	fIPs, err := networkClient.GetFipByName(name)
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())
	Expect(fIPs).To(BeEmpty())

	// bastion Security group should be gone
	sGroups, err := networkClient.GetSecurityGroupByName(fmt.Sprintf("%s-sg", name))
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())
	Expect(sGroups).To(BeEmpty())

	// bastion instance should be terminated and not found
	servers, err := computeClient.FindServersByName(name)
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())
	Expect(servers).To(BeEmpty())
}

func verifyCreation(options *bastionctrl.Options) {
	By("checkSecurityGroupExists")
	sGroups, err := networkClient.GetSecurityGroupByName(options.SecurityGroup)
	Expect(err).To(Succeed())
	Expect(sGroups).ToNot(BeEmpty())
	Expect(sGroups[0].Description).To(Equal(options.SecurityGroup))

	By("checkNSGExists")
	securityRuleName := bastionctrl.IngressAllowSSH(options, "", "", "", "").Description
	sRules, err := networkClient.ListRules(rules.ListOpts{Description: securityRuleName})
	Expect(err).NotTo(HaveOccurred())
	Expect(sRules).ToNot(BeEmpty())
	Expect(sRules[0].Description).To(Equal(securityRuleName))

	By("checking bastion instance")
	servers, err := computeClient.FindServersByName(options.BastionInstanceName)
	Expect(err).To(Succeed())
	Expect(servers[0].Name).To(Equal(options.BastionInstanceName))

	By("checking bastion ingress IPs exist")
	privateIP, externalIP, err := bastionctrl.GetIPs(&servers[0], options)
	Expect(err).To(Succeed())
	Expect(privateIP).NotTo(BeNil())
	Expect(externalIP).NotTo(BeNil())
}

// retry performs a function with retries, delay, and a max number of attempts
func retry(maxRetries int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		log.Info(fmt.Sprintf("Attempt %d failed, retrying in %v: %v", i+1, delay, err))
		time.Sleep(delay)
	}
	return err
}
