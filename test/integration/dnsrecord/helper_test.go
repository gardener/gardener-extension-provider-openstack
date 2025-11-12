// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	openstackext "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/zones"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	By("validating OpenStack credentials")
	err := openstackext.ValidateSecrets(*userName, *password, *appID, *appName, *appSecret)
	if err != nil {
		Fail(fmt.Sprintf("Failed to validate OpenStack credentials: %s", err.Error()))
	}
}

func createNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Creating namespace", "namespace", namespace.Name)
	Expect(c.Create(ctx, namespace)).To(Succeed(), "Failed to create namespace: %s", namespace.Name)
}

func deleteNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	if c == nil || cluster == nil {
		return
	}
	log.Info("Deleting namespace", "namespace", namespace.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func createCluster(ctx context.Context, c client.Client, cluster *extensionsv1alpha1.Cluster) {
	log.Info("Creating cluster", "cluster", cluster.Name)
	Expect(c.Create(ctx, cluster)).To(Succeed(), "Failed to create cluster: %s", cluster.Name)
}

func deleteCluster(ctx context.Context, c client.Client, cluster *extensionsv1alpha1.Cluster) {
	if c == nil || cluster == nil {
		return
	}
	log.Info("Deleting cluster", "cluster", cluster.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
}

func createDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(c.Create(ctx, dns)).To(Succeed())
}

func updateDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(c.Update(ctx, dns)).To(Succeed())
}

func deleteDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(client.IgnoreNotFound(c.Delete(ctx, dns))).To(Succeed())
}

func createDNSHostedZone(ctx context.Context, dnsService *gophercloud.ServiceClient, zoneName string) string {
	zone, err := zones.Create(ctx, dnsService, zones.CreateOpts{
		Name:        zoneName,
		Description: "DNS Test Hosted Zone",
		Email:       "test-gardener-os-extension@sap.com",
	}).Extract()
	Expect(err).NotTo(HaveOccurred())
	Expect(zone).NotTo(BeNil())
	return zone.ID
}

func deleteDNSHostedZone(ctx context.Context, dnsService *gophercloud.ServiceClient, zoneID string) {
	if dnsService == nil || zoneID == "" {
		return
	}
	_, err := zones.Delete(ctx, dnsService, zoneID).Extract()
	Expect(err).NotTo(HaveOccurred())
}

func newDNSRecord(namespace string, recordType extensionsv1alpha1.DNSRecordType, values []string, ttl *int64) *extensionsv1alpha1.DNSRecord {
	name := "dnsrecord-" + randomString()
	zone := testName + "/" + zoneName

	return &extensionsv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: openstackext.DNSType,
			},
			SecretRef: corev1.SecretReference{
				Name:      "dnsrecord",
				Namespace: namespace,
			},
			Zone:       &zone,
			Name:       name + "." + zoneName,
			RecordType: recordType,
			Values:     values,
			TTL:        ttl,
		},
	}
}

func verifyDNSRecordSet(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) {
	rrs, err := recordsets.Get(ctx, dnsService, *dns.Spec.Zone, dns.Spec.Name).Extract()
	Expect(err).NotTo(HaveOccurred())
	Expect(rrs).NotTo(BeNil())

	Expect(rrs.Name).To(Equal(ensureTrailingDot(dns.Spec.Name)))
	Expect(rrs.Type).To(Equal(string(dns.Spec.RecordType)))
	Expect(rrs.TTL).To(Equal(ptr.Deref(dns.Spec.TTL, 120)))

	Expect(rrs.Records).To(WithTransform(func(in []string) []string {
		// TODO check
		//switch dns.Spec.RecordType {
		//case extensionsv1alpha1.DNSRecordTypeTXT:
		//	out := make([]string, len(in))
		//	for i, v := range in {
		//		out[i] = strings.Trim(v, "\"")
		//	}
		//	return out
		//default:
		//	return in
		//}
		return in
	}, ConsistOf(dns.Spec.Values)))
}

func getDNSRecordAndVerifyStatus(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord, zoneID string) {
	Expect(c.Get(ctx, client.ObjectKey{Namespace: dns.Namespace, Name: dns.Name}, dns)).To(Succeed())
	Expect(dns.Status.Zone).To(PointTo(Equal(zoneID)))
}

func verifyDNSRecordSetDeleted(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) {
	// TODO check rrsetID
	_, err := recordsets.Get(ctx, dnsService, *dns.Spec.Zone, dns.Spec.Name).Extract()
	Expect(openstackclient.IsNotFoundError(err)).To(BeTrue())
}

func deleteDNSRecordSet(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) {
	// TODO check rrsetID
	err := recordsets.Delete(ctx, dnsService, *dns.Spec.Zone, dns.Spec.Name).ExtractErr()
	if openstackclient.IsNotFoundError(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
}

func waitUntilDNSRecordReady(ctx context.Context, c client.Client, log logr.Logger, dns *extensionsv1alpha1.DNSRecord) {
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		dns,
		extensionsv1alpha1.DNSRecordResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)).To(Succeed())
}

// createProviderClient creates a provider client for OpenStack
func createProviderClient(ctx context.Context, credentials *openstackext.Credentials) *gophercloud.ProviderClient {
	provider, err := openstackclient.NewProviderClient(ctx, credentials)
	Expect(err).NotTo(HaveOccurred())

	provider.UserAgent.Prepend("Gardener Extension for OpenStack DNS Entry test provider")

	return provider
}

func createDnsClient(providerClient *gophercloud.ProviderClient, region string) *gophercloud.ServiceClient {
	opts := gophercloud.EndpointOpts{}
	opts.Region = region

	newDnsClient, err := openstack.NewDNSV2(providerClient, opts)
	Expect(err).NotTo(HaveOccurred(), "Failed to create OpenStack DNS client", "Error", err)

	return newDnsClient
}

func waitUntilDNSRecordDeleted(ctx context.Context, c client.Client, log logr.Logger, dns *extensionsv1alpha1.DNSRecord) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		dns.DeepCopy(),
		extensionsv1alpha1.DNSRecordResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
}

func randomString() string {
	rs, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred())
	return rs
}

func ensureTrailingDot(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}
