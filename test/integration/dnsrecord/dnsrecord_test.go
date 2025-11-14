// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

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
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	dnsrecordctrl "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/dnsrecord"
	openstackext "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// IMPORTANT:
// In order for this test to work you have to pre create
// a DNS zone in the correct OpenStack project and pass its name with --existing-dns-zone

var (
	ctx       = context.Background()
	log       logr.Logger
	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client

	testName  string
	dnsClient *gophercloud.ServiceClient
	zoneID    string
	cluster   *extensionsv1alpha1.Cluster
	namespace *corev1.Namespace
	secret    *corev1.Secret

	authURL    = flag.String("auth-url", "", "Authorization URL for openstack")
	domainName = flag.String("domain-name", "", "Domain name for openstack")
	region     = flag.String("region", "", "Openstack region")
	tenantName = flag.String("tenant-name", "", "Tenant name for openstack")
	userName   = flag.String("user-name", "", "User name for openstack")
	password   = flag.String("password", "", "Password for openstack")
	appID      = flag.String("app-id", "", "Application Credential ID for openstack")
	appName    = flag.String("app-name", "", "Application Credential Name for openstack")
	appSecret  = flag.String("app-secret", "", "Application Credential Secret for openstack")
	// TODO remove default
	existingDnsZone = flag.String("existing-dns-zone", "", "Name of the dns Zone that must be pre-created in OpenStack")
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName("dnsrecord-test")

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("deleting test cluster")
		deleteCluster(ctx, c, cluster)

		By("deleting secret")
		deleteSecret(ctx, c, secret)

		By("deleting test namespace")
		deleteNamespace(ctx, c, namespace)

		By("stopping test environment")
		if testEnv != nil {
			Expect(testEnv.Stop()).To(Succeed())
		}
	})

	By("generating randomized dnsrecord test resource identifiers")
	testName = fmt.Sprintf("os-dnsrecord-it--%s", randomString())

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_dnsrecords.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
			},
		},
		ControlPlaneStopTimeout: 2 * time.Minute,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred(), "Failed to start the test environment")
	Expect(cfg).ToNot(BeNil(), "Test environment configuration is nil")
	log.Info("Test environment started successfully")

	By("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred(), "Failed to create manager for the test environment")

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed(), "Failed to add extensionsv1alpha1 scheme to manager")
	Expect(openstackinstall.AddToScheme(mgr.GetScheme())).To(Succeed(), "Failed to add OpenStack scheme to manager")

	Expect(dnsrecordctrl.AddToManagerWithOptions(ctx, mgr, dnsrecordctrl.AddOptions{})).To(Succeed(), "Failed to add DnsRecord controller to manager")

	By("starting manager")
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred(), "Failed to start the manager")
	}()

	By("getting k8s client")
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(c).NotTo(BeNil())

	By("creating OpenStack provider client and dns service")
	credentials := openstackext.Credentials{
		DomainName:                  *domainName,
		TenantName:                  *tenantName,
		Username:                    *userName,
		Password:                    *password,
		ApplicationCredentialID:     *appID,
		ApplicationCredentialName:   *appName,
		ApplicationCredentialSecret: *appSecret,
		AuthURL:                     *authURL,
	}
	providerClient := createProviderClient(ctx, &credentials)
	dnsClient = createDnsClient(providerClient, *region)

	By("creating test namespace")
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}
	createNamespace(ctx, c, namespace)

	By("creating test secret")
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dnsrecord",
			Namespace: testName,
		},
		Data: map[string][]byte{
			openstackext.AuthURL:                     []byte(*authURL),
			openstackext.DomainName:                  []byte(*domainName),
			openstackext.Password:                    []byte(*password),
			openstackext.Region:                      []byte(*region),
			openstackext.TenantName:                  []byte(*tenantName),
			openstackext.UserName:                    []byte(*userName),
			openstackext.ApplicationCredentialID:     []byte(*appID),
			openstackext.ApplicationCredentialName:   []byte(*appName),
			openstackext.ApplicationCredentialSecret: []byte(*appSecret),
		},
	}
	createSecret(ctx, c, secret)

	By("creating test cluster")
	cloudProfileConfig := openstackv1alpha1.CloudProfileConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CloudProfileConfig",
		},
		KeyStoneURLs: []openstackv1alpha1.KeyStoneURL{
			{
				Region: *region,
				URL:    *authURL,
			},
		},
	}
	cloudProfileConfigJSON, err := json.Marshal(&cloudProfileConfig)
	Expect(err).NotTo(HaveOccurred())

	cloudprofile := gardenerv1beta1.CloudProfile{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardenerv1beta1.SchemeGroupVersion.String(),
			Kind:       "CloudProfile",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: gardenerv1beta1.CloudProfileSpec{
			ProviderConfig: &runtime.RawExtension{
				Raw: cloudProfileConfigJSON,
			},
		},
	}
	cloudProfileJSON, err := json.Marshal(&cloudprofile)
	Expect(err).NotTo(HaveOccurred())

	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
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
	createCluster(ctx, c, cluster)

	By("retrieving pre created OpenStack DNS hosted zone")
	zoneID = getPreCreatedDNSHostedZone(ctx, dnsClient, *existingDnsZone)
})

var runTest = func(dns *extensionsv1alpha1.DNSRecord, newValues []string, beforeCreate, beforeUpdate, beforeDelete func()) {
	if beforeCreate != nil {
		beforeCreate()
	}

	By("creating dnsrecord")
	createDNSRecord(ctx, c, dns)

	defer func() {
		if beforeDelete != nil {
			beforeDelete()
		}

		By("deleting dnsrecord")
		deleteDNSRecord(ctx, c, dns)

		By("waiting until dnsrecord is deleted")
		waitUntilDNSRecordDeleted(ctx, c, log, dns)

		By("verifying that the OpenStack DNS recordset does not exist")
		verifyDNSRecordSetDeleted(ctx, dnsClient, dns)
	}()

	framework.AddCleanupAction(func() {
		By("deleting the OpenStack DNS recordset if it still exists")
		deleteDNSRecordSet(ctx, dnsClient, dns)
	})

	By("waiting until dnsrecord is ready")
	waitUntilDNSRecordReady(ctx, c, log, dns)

	By("getting dnsrecord and verifying its status")
	getDNSRecordAndVerifyStatus(ctx, c, dns, zoneID)

	By("verifying that the OpenStack DNS recordset exists and matches dnsrecord")
	verifyDNSRecordSet(ctx, dnsClient, dns)

	if len(newValues) > 0 {
		if beforeUpdate != nil {
			beforeUpdate()
		}

		dns.Spec.Values = newValues
		metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

		By("updating dnsrecord")
		updateDNSRecord(ctx, c, dns)

		By("waiting until dnsrecord is ready")
		waitUntilDNSRecordReady(ctx, c, log, dns)

		By("getting dnsrecord and verifying its status")
		getDNSRecordAndVerifyStatus(ctx, c, dns, zoneID)

		By("verifying that the OpenStack DNS recordset exists and matches dnsrecord")
		verifyDNSRecordSet(ctx, dnsClient, dns)
	}
}

var _ = Describe("DNSRecord tests", func() {
	Context("when a DNS recordset doesn't exist and is not changed or deleted before dnsrecord deletion", func() {
		It("should successfully create and delete a dnsrecord of type A", func() {
			dns := newDNSRecord(extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))
			runTest(dns, nil, nil, nil, nil)
		})

		It("should successfully create and delete a dnsrecord of type CNAME", func() {
			dns := newDNSRecord(extensionsv1alpha1.DNSRecordTypeCNAME, []string{"foo.example.com."}, ptr.To[int64](600))
			runTest(dns, nil, nil, nil, nil)
		})

		It("should successfully create and delete a dnsrecord of type TXT", func() {
			dns := newDNSRecord(extensionsv1alpha1.DNSRecordTypeTXT, []string{"foo", "bar"}, nil)
			runTest(dns, nil, nil, nil, nil)
		})
	})

	Context("when a DNS recordset exists and is changed before dnsrecord update and deletion", func() {
		It("should successfully create, update, and delete a dnsrecord", func() {
			dns := newDNSRecord(extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))

			runTest(
				dns,
				[]string{"3.3.3.3", "1.1.1.1"},
				nil,
				func() {
					By("creating OpenStack DNS recordset")
					updateDNSRecordSet(ctx, dnsClient, dns, []string{"8.8.8.8"})
				},
				func() {
					By("creating OpenStack DNS recordset")
					updateDNSRecordSet(ctx, dnsClient, dns, []string{"8.8.8.8"})
				},
			)
		})
	})

	Context("when a DNS recordset exists and is deleted before dnsrecord deletion", func() {
		It("should successfully create and delete a dnsrecord", func() {
			dns := newDNSRecord(extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))

			runTest(
				dns,
				nil,
				func() {
					By("creating OpenStack DNS recordset")
					createDNSRecordSet(ctx, dnsClient, dns.Spec.Name,
						string(dns.Spec.RecordType), 120, dns.Spec.Values)
				},
				nil,
				func() {
					By("deleting OpenStack DNS recordset")
					deleteDNSRecordSet(ctx, dnsClient, dns)
				},
			)
		})
	})
})
