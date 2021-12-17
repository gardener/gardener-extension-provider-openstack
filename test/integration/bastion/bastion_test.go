// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bastion

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"path/filepath"
	"time"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	bastionctrl "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/bastion"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	authURL          = flag.String("auth-url", "", "Authorization URL for openstack")
	domainName       = flag.String("domain-name", "", "Domain name for openstack")
	floatingPoolName = flag.String("floating-pool-name", "", "Floating pool name for creating router")
	password         = flag.String("password", "", "Password for openstack")
	region           = flag.String("region", "", "Openstack region")
	tenantName       = flag.String("tenant-name", "", "Tenant name for openstack")
	userName         = flag.String("user-name", "", "User name for openstack")
	userDataConst    = "IyEvYmluL2Jhc2ggLWV1CmlkIGdhcmRlbmVyIHx8IHVzZXJhZGQgZ2FyZGVuZXIgLW1VCm1rZGlyIC1wIC9ob21lL2dhcmRlbmVyLy5zc2gKZWNobyAic3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFDazYyeDZrN2orc0lkWG9TN25ITzRrRmM3R0wzU0E2UmtMNEt4VmE5MUQ5RmxhcmtoRzFpeU85WGNNQzZqYnh4SzN3aWt0M3kwVTBkR2h0cFl6Vjh3YmV3Z3RLMWJBWnl1QXJMaUhqbnJnTFVTRDBQazNvWGh6RkpKN0MvRkxNY0tJZFN5bG4vMENKVkVscENIZlU5Y3dqQlVUeHdVQ2pnVXRSYjdZWHN6N1Y5dllIVkdJKzRLaURCd3JzOWtVaTc3QWMyRHQ1UzBJcit5dGN4b0p0bU5tMWgxTjNnNzdlbU8rWXhtWEo4MzFXOThoVFVTeFljTjNXRkhZejR5MWhrRDB2WHE1R1ZXUUtUQ3NzRE1wcnJtN0FjQTBCcVRsQ0xWdWl3dXVmTEJLWGhuRHZRUEQrQ2Jhbk03bUZXRXdLV0xXelZHME45Z1VVMXE1T3hhMzhvODUgbWVAbWFjIiA+IC9ob21lL2dhcmRlbmVyLy5zc2gvYXV0aG9yaXplZF9rZXlzCmNob3duIGdhcmRlbmVyOmdhcmRlbmVyIC9ob21lL2dhcmRlbmVyLy5zc2gvYXV0aG9yaXplZF9rZXlzCmVjaG8gImdhcmRlbmVyIEFMTD0oQUxMKSBOT1BBU1NXRDpBTEwiID4vZXRjL3N1ZG9lcnMuZC85OS1nYXJkZW5lci11c2VyCg=="
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

var _ = Describe("Bastion tests", func() {
	var (
		ctx    = context.Background()
		logger *logrus.Entry

		extensionscluster *extensionsv1alpha1.Cluster
		controllercluster *controller.Cluster
		options           *bastionctrl.Options
		bastion           *extensionsv1alpha1.Bastion
		secret            *corev1.Secret

		testEnv   *envtest.Environment
		mgrCancel context.CancelFunc
		c         client.Client

		openstackClient    *OpenstackClient
		internalChartsPath string
	)

	randString, err := randomString()
	Expect(err).NotTo(HaveOccurred())

	// bastion name prefix
	name := fmt.Sprintf("openstack-it-bastion-%s", randString)

	BeforeSuite(func() {
		flag.Parse()
		validateFlags()

		internalChartsPath = openstack.InternalChartsPath
		repoRoot := filepath.Join("..", "..", "..")
		openstack.InternalChartsPath = filepath.Join(repoRoot, openstack.InternalChartsPath)

		// enable manager logs
		logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

		log := logrus.New()
		log.SetOutput(GinkgoWriter)
		logger = logrus.NewEntry(log)

		By("starting test environment")
		testEnv = &envtest.Environment{
			UseExistingCluster: pointer.BoolPtr(true),
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
					filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_bastions.yaml"),
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

		Expect(bastionctrl.AddToManager(mgr)).To(Succeed())

		var mgrContext context.Context
		mgrContext, mgrCancel = context.WithCancel(ctx)

		By("start manager")
		go func() {
			err := mgr.Start(mgrContext)
			Expect(err).NotTo(HaveOccurred())
		}()

		c = mgr.GetClient()
		Expect(c).NotTo(BeNil())

		extensionscluster, controllercluster = createClusters(name)
		bastion, options = createBastion(controllercluster, name)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cloudprovider",
				Namespace: name,
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

		openstackClient, err = NewOpenstackClient(*authURL, *domainName, *password, *region, *tenantName, *userName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterSuite(func() {
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

	It("should successfully create and delete", func() {
		cloudRouterName := name + "-cloud-router"
		subnetName := name + "-subnet"

		By("setup Infrastructure ")
		shootSecurityGroupID, err := prepareShootSecurityGroup(logger, name, openstackClient)
		Expect(err).NotTo(HaveOccurred())

		networkID, err := prepareNewNetwork(logger, name, openstackClient)
		Expect(err).NotTo(HaveOccurred())

		subNetID, err := prepareSubNet(logger, subnetName, *networkID, openstackClient)
		Expect(err).NotTo(HaveOccurred())

		routerID, err := prepareNewRouter(logger, cloudRouterName, *subNetID, openstackClient)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			By("Tearing down Shoot Security Group")
			err = teardownShootSecurityGroup(logger, *shootSecurityGroupID, openstackClient)
			Expect(err).NotTo(HaveOccurred())

			By("Tearing down network")
			err := teardownNetwork(logger, *networkID, *routerID, *subNetID, openstackClient)
			Expect(err).NotTo(HaveOccurred())

			By("Tearing down router")
			err = teardownRouter(logger, *routerID, openstackClient)
			Expect(err).NotTo(HaveOccurred())

		})

		By("create namespace for test execution")
		setupEnvironmentObjects(ctx, c, namespace(name), secret, extensionscluster)
		framework.AddCleanupAction(func() {
			teardownShootEnvironment(ctx, c, namespace(name), secret, extensionscluster)
		})

		By("setup bastion")
		err = c.Create(ctx, bastion)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			teardownBastion(ctx, logger, c, bastion)

			By("verify bastion deletion")
			verifyDeletion(openstackClient, name)
		})

		By("wait until bastion is reconciled")
		Expect(extensions.WaitUntilExtensionObjectReady(
			ctx,
			c,
			logger,
			bastion,
			extensionsv1alpha1.BastionResource,
			60*time.Second,
			120*time.Second,
			10*time.Minute,
			nil,
		)).To(Succeed())

		time.Sleep(10 * time.Second)
		verifyPort22IsOpen(ctx, c, bastion)
		verifyPort42IsClosed(ctx, c, bastion)

		By("verify cloud resources")
		verifyCreation(openstackClient, options)
	})
})

func randomString() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return suffix, nil
}

func verifyPort22IsOpen(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("check connection to port 22 open should not error")
	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "22")
	conn, err := net.DialTimeout("tcp4", address, 60*time.Second)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(conn).NotTo(BeNil())
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

func prepareNewRouter(logger *logrus.Entry, routerName, subnetID string, openstackClient *OpenstackClient) (*string, error) {
	logger.Infof("Waiting until router '%s' is created...", routerName)

	allPages, err := networks.List(openstackClient.NetworkingClient, external.ListOptsExt{
		ListOptsBuilder: networks.ListOpts{
			Name: "FloatingIP-external-monsoon3-02"},
		External: pointer.Bool(true),
	}).AllPages()
	Expect(err).NotTo(HaveOccurred())

	externalNetworks, err := networks.ExtractNetworks(allPages)
	Expect(err).NotTo(HaveOccurred())

	createOpts := routers.CreateOpts{
		Name:         routerName,
		AdminStateUp: pointer.BoolPtr(true),
		GatewayInfo: &routers.GatewayInfo{
			NetworkID: externalNetworks[0].ID,
		},
	}
	router, err := routers.Create(openstackClient.NetworkingClient, createOpts).Extract()
	Expect(err).NotTo(HaveOccurred())

	intOpts := routers.AddInterfaceOpts{
		SubnetID: subnetID,
	}
	_, err = routers.AddInterface(openstackClient.NetworkingClient, router.ID, intOpts).Extract()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Router '%s' is created...", routerName)
	return &router.ID, nil
}

func teardownRouter(logger *logrus.Entry, routerID string, openstackClient *OpenstackClient) error {
	logger.Infof("Waiting until router '%s' is deleted...", routerID)

	err := routers.Delete(openstackClient.NetworkingClient, routerID).ExtractErr()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Router '%s' is deleted...", routerID)
	return nil
}

func prepareNewNetwork(logger *logrus.Entry, networkName string, openstackClient *OpenstackClient) (*string, error) {
	logger.Infof("Waiting until network '%s' is created...", networkName)

	network, err := networks.Create(openstackClient.NetworkingClient, networks.CreateOpts{Name: networkName}).Extract()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Network '%s' is created...", networkName)
	return &network.ID, nil
}

func prepareSubNet(logger *logrus.Entry, subnetName, networkid string, openstackClient *OpenstackClient) (*string, error) {
	logger.Infof("Waiting until Subnet '%s' is created...", subnetName)

	createOpts := subnets.CreateOpts{
		Name:      subnetName,
		NetworkID: networkid,
		IPVersion: 4,
		CIDR:      "10.180.0.0/16",
		GatewayIP: pointer.String("10.180.0.1"),
		AllocationPools: []subnets.AllocationPool{
			{
				Start: "10.180.0.2",
				End:   "10.180.255.254",
			},
		},
	}
	subnet, err := subnets.Create(openstackClient.NetworkingClient, createOpts).Extract()
	Expect(err).NotTo(HaveOccurred())
	logger.Infof("Subnet '%s' is created...", subnetName)
	return &subnet.ID, nil
}

// prepareShootSecurityGroup create fake shoot security group which will be used in EgressAllowSSHToWorker remoteGroupID
func prepareShootSecurityGroup(logger *logrus.Entry, shootSgName string, openstackClient *OpenstackClient) (*string, error) {
	logger.Infof("Waiting until Shoot Security Group '%s' is created...", shootSgName)

	sgroups, err := groups.Create(openstackClient.NetworkingClient, groups.CreateOpts{Name: shootSgName, Description: shootSgName}).Extract()
	Expect(err).NotTo(HaveOccurred())
	logger.Infof("Shoot Security Group '%s' is created...", shootSgName)
	return &sgroups.ID, nil
}

func teardownShootSecurityGroup(logger *logrus.Entry, groupID string, openstackClient *OpenstackClient) error {
	err := groups.Delete(openstackClient.NetworkingClient, groupID).ExtractErr()
	Expect(err).NotTo(HaveOccurred())
	logger.Infof("Shoot Security Group '%s' is deleted...", groupID)
	return nil
}

func teardownNetwork(logger *logrus.Entry, networkID, routerID, subnetID string, openstackClient *OpenstackClient) error {
	logger.Infof("Waiting until network '%s' is deleted...", networkID)

	_, err := routers.RemoveInterface(openstackClient.NetworkingClient, routerID, routers.RemoveInterfaceOpts{SubnetID: subnetID}).Extract()
	Expect(err).NotTo(HaveOccurred())

	err = networks.Delete(openstackClient.NetworkingClient, networkID).ExtractErr()
	Expect(err).NotTo(HaveOccurred())

	logger.Infof("Network '%s' is deleted...", networkID)
	return nil
}

func setupEnvironmentObjects(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster) {
	Expect(c.Create(ctx, namespace)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
	Expect(c.Create(ctx, secret)).To(Succeed())
}

func teardownShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster) {
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
		},
	}

	cluster := &controller.Cluster{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Shoot:        shoot,
		CloudProfile: cloudProfile,
	}
	return extensionscluster, cluster
}

func createInfrastructureConfig() *openstackv1alpha1.InfrastructureConfig {
	return &openstackv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		FloatingPoolSubnetName: pointer.String("FloatingIP-external-monsoon3-02"),
	}
}

func createShoot(infrastructureConfig []byte) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core.gardener.cloud/v1beta1",
			Kind:       "Shoot",
		},
		Spec: gardencorev1beta1.ShootSpec{
			Region:            *region,
			SecretBindingName: v1beta1constants.SecretNameCloudProvider,
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
		},
	}

	options, err := bastionctrl.DetermineOptions(bastion, cluster)
	Expect(err).NotTo(HaveOccurred())

	return bastion, options
}

func teardownBastion(ctx context.Context, logger *logrus.Entry, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("delete bastion")
	Expect(client.IgnoreNotFound(c.Delete(ctx, bastion))).To(Succeed())

	By("wait until bastion is deleted")
	err := extensions.WaitUntilExtensionObjectDeleted(ctx, c, logger, bastion, extensionsv1alpha1.BastionResource, 10*time.Second, 16*time.Minute)
	Expect(err).NotTo(HaveOccurred())
}

func verifyDeletion(openstackClient *OpenstackClient, name string) {
	// bastion public ip should be gone
	_, err := floatingips.List(openstackClient.NetworkingClient, floatingips.ListOpts{Description: name}).AllPages()
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())

	// bastion Security group should be gone
	_, err = groups.List(openstackClient.NetworkingClient, groups.ListOpts{Name: fmt.Sprintf("%s-sg", name)}).AllPages()
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())

	// bastion instance should be terminated and not found
	_, err = servers.List(openstackClient.NetworkingClient, servers.ListOpts{Name: name}).AllPages()
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())
}

func checkSecurityRuleslExists(openstackClient *OpenstackClient, securityRuleName string) {
	allPages, err := rules.List(openstackClient.NetworkingClient, rules.ListOpts{Description: securityRuleName}).AllPages()
	Expect(err).NotTo(HaveOccurred())
	rule, err := rules.ExtractRules(allPages)
	Expect(err).NotTo(HaveOccurred())
	Expect(rule[0].Description).To(Equal(securityRuleName))
}

func verifyCreation(openstackClient *OpenstackClient, options *bastionctrl.Options) {
	By("checkSecurityGroupExists")
	allPages, err := groups.List(openstackClient.NetworkingClient, groups.ListOpts{Name: options.SecurityGroup}).AllPages()
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())

	securityGroup, err := groups.ExtractGroups(allPages)
	Expect(openstackclient.IgnoreNotFoundError(err)).To(Succeed())
	Expect(securityGroup[0].Description).To(Equal(options.SecurityGroup))

	By("checkNSGExists")
	checkSecurityRuleslExists(openstackClient, bastionctrl.IngressAllowSSH(options, "", "", "").Description)

	By("checking bastion instance")
	allPages, err = servers.List(openstackClient.ComputeClient, servers.ListOpts{Name: options.BastionInstanceName}).AllPages()
	Expect(err).To(Succeed())
	allServers, err := servers.ExtractServers(allPages)
	Expect(err).To(Succeed())
	Expect(allServers[0].Name).To(Equal(options.BastionInstanceName))

	By("checking bastion ingress IPs exist")
	privateIP, externalIP, err := bastionctrl.GetIPs(&allServers[0], options)
	Expect(err).To(Succeed())
	Expect(privateIP).NotTo(BeNil())
	Expect(externalIP).NotTo(BeNil())
}
