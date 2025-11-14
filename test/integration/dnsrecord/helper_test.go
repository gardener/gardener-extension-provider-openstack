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
	if len(*region) == 0 {
		panic("--region flag is not specified")
	}
	if len(*tenantName) == 0 {
		panic("--tenant-name flag is not specified")
	}
	if len(*existingDnsZone) == 0 {
		panic("--existing-dns-zone flag is not specified")
	}

	By("validating OpenStack credentials")
	err := openstackext.ValidateSecrets(*userName, *password, *appID, *appName, *appSecret)
	if err != nil {
		Fail(fmt.Sprintf("Failed to validate OpenStack credentials: %s", err.Error()))
	}
}

// Kube client helper functions
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

func createSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Creating secret", "secret", secret.Name)
	Expect(c.Create(ctx, secret)).To(Succeed())
}

func deleteSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	if c == nil || secret == nil {
		return
	}
	log.Info("Deleting secret", "secret", secret.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
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

func newDNSRecord(recordType extensionsv1alpha1.DNSRecordType, values []string, ttl *int64) *extensionsv1alpha1.DNSRecord {
	recordName := "dnsrecord-" + randomString()

	return &extensionsv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recordName,
			Namespace: testName,
		},
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: openstackext.DNSType,
			},
			SecretRef: corev1.SecretReference{
				Name:      "dnsrecord",
				Namespace: testName,
			},
			Zone:       &zoneID,
			Name:       recordName + "." + *existingDnsZone,
			RecordType: recordType,
			Values:     values,
			TTL:        ttl,
		},
	}
}

func getDNSRecordAndVerifyStatus(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord, zoneID string) {
	Expect(c.Get(ctx, client.ObjectKey{Namespace: dns.Namespace, Name: dns.Name}, dns)).To(Succeed())
	Expect(dns.Status.Zone).To(PointTo(Equal(zoneID)))
}

func verifyDNSRecordSet(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) {
	recordSets := getDnsRecordSetByName(ctx, dnsService, dns)
	Expect(recordSets).To(HaveLen(1))
	rrs := recordSets[0]
	Expect(rrs.Name).To(Equal(ensureTrailingDot(dns.Spec.Name)))
	Expect(rrs.Type).To(Equal(string(dns.Spec.RecordType)))
	Expect(int64(rrs.TTL)).To(Equal(ptr.Deref(dns.Spec.TTL, 120)))

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

// extensionsv1alpha1.DNSRecord helper functions
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

func verifyDNSRecordSetDeleted(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) {
	recordSets := getDnsRecordSetByName(ctx, dnsService, dns)
	Expect(recordSets).To(HaveLen(0))
}

func createDNSRecordSet(ctx context.Context, dnsService *gophercloud.ServiceClient,
	name, recordType string, ttl int, records []string) {
	createOpts := recordsets.CreateOpts{
		Name:    name,
		Records: records,
		TTL:     ttl,
		Type:    recordType,
	}

	_, err := recordsets.Create(ctx, dnsService, zoneID, createOpts).Extract()
	Expect(err).NotTo(HaveOccurred())
}

func updateDNSRecordSet(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord, newRecords []string) {
	recordSets := getDnsRecordSetByName(ctx, dnsService, dns)
	Expect(recordSets).To(HaveLen(1))
	rrs := recordSets[0]

	updateOpts := recordsets.UpdateOpts{
		Records: newRecords,
	}

	_, err := recordsets.Update(ctx, dnsService, zoneID, rrs.ID, updateOpts).Extract()
	Expect(err).NotTo(HaveOccurred())
}

func deleteDNSRecordSet(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) {
	recordSets := getDnsRecordSetByName(ctx, dnsService, dns)
	if len(recordSets) > 0 {
		Expect(recordSets).To(HaveLen(1))
		rrs := recordSets[0]
		err := recordsets.Delete(ctx, dnsService, zoneID, rrs.ID).ExtractErr()
		if openstackclient.IsNotFoundError(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())
	}
}

func getDnsRecordSetByName(ctx context.Context, dnsService *gophercloud.ServiceClient, dns *extensionsv1alpha1.DNSRecord) []recordsets.RecordSet {
	page, err := recordsets.ListByZone(dnsService, zoneID, recordsets.ListOpts{
		Name: dns.Spec.Name,
	}).AllPages(ctx)
	Expect(err).NotTo(HaveOccurred())
	recordSets, err := recordsets.ExtractRecordSets(page)
	Expect(err).NotTo(HaveOccurred())
	return recordSets
}

// gophercloud helper functions
func createDnsClient(providerClient *gophercloud.ProviderClient, region string) *gophercloud.ServiceClient {
	opts := gophercloud.EndpointOpts{}
	opts.Region = region

	newDnsClient, err := openstack.NewDNSV2(providerClient, opts)
	Expect(err).NotTo(HaveOccurred(), "Failed to create OpenStack DNS client", "Error", err)

	return newDnsClient
}

func createProviderClient(ctx context.Context, credentials *openstackext.Credentials) *gophercloud.ProviderClient {
	provider, err := openstackclient.NewProviderClient(ctx, credentials)
	Expect(err).NotTo(HaveOccurred())

	provider.UserAgent.Prepend("Gardener Extension for OpenStack DNS Entry test provider")

	return provider
}

func getPreCreatedDNSHostedZone(ctx context.Context, dnsService *gophercloud.ServiceClient, zoneName string) string {
	page, err := zones.List(dnsService, zones.ListOpts{
		Name: zoneName,
	}).AllPages(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(page).NotTo(BeNil())
	zonesList, err := zones.ExtractZones(page)
	Expect(err).NotTo(HaveOccurred())
	Expect(zonesList).To(HaveLen(1))
	return zonesList[0].ID
}

// other helper functions
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
