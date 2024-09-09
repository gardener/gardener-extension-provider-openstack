// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"
	"fmt"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

const (
	// requeueAfterOnProviderError is a value for RequeueAfter to be returned on provider errors
	// in order to prevent quick retries that could quickly exhaust the account rate limits in case of e.g.
	// configuration issues.
	requeueAfterOnProviderError = 30 * time.Second
)

type actuator struct {
	client                 k8sclient.Client
	openstackClientFactory openstackclient.FactoryFactory
}

// NewActuator creates a new dnsrecord.Actuator.
func NewActuator(mgr manager.Manager, openstackClientFactory openstackclient.FactoryFactory) dnsrecord.Actuator {
	return &actuator{
		client:                 mgr.GetClient(),
		openstackClientFactory: openstackClientFactory,
	}
}

// Reconcile reconciles the DNSRecord.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	// Create Openstack DNS client
	credentials, err := openstack.GetCredentials(ctx, a.client, dns.Spec.SecretRef, true)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}
	openstackClientFactory, err := a.openstackClientFactory.NewFactory(credentials)
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create Openstack client factory: %w", err), helper.KnownCodes)
	}
	dnsClient, err := openstackClientFactory.DNS()
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create Openstack DNS client: %w", err), helper.KnownCodes)
	}

	// Determine DNS zone ID
	zone, err := a.getZone(ctx, log, dns, dnsClient)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// Create or update DNS recordset
	ttl := extensionsv1alpha1helper.GetDNSRecordTTL(dns.Spec.TTL)
	log.Info("Creating or updating DNS recordset", "zone", zone, "name", dns.Spec.Name, "type", dns.Spec.RecordType, "values", dns.Spec.Values, "dnsrecord", k8sclient.ObjectKeyFromObject(dns))
	if err := dnsClient.CreateOrUpdateRecordSet(ctx, zone, dns.Spec.Name, string(dns.Spec.RecordType), dns.Spec.Values, int(ttl)); err != nil {
		return &reconcilerutils.RequeueAfterError{
			Cause:        fmt.Errorf("could not create or update DNS recordset in zone %s with name %s, type %s, and values %v: %+v", zone, dns.Spec.Name, dns.Spec.RecordType, dns.Spec.Values, err),
			RequeueAfter: requeueAfterOnProviderError,
		}
	}

	// Update resource status
	patch := k8sclient.MergeFrom(dns.DeepCopy())
	dns.Status.Zone = &zone
	return a.client.Status().Patch(ctx, dns, patch)
}

// Delete deletes the DNSRecord.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	// Create Openstack DNS client
	credentials, err := openstack.GetCredentials(ctx, a.client, dns.Spec.SecretRef, true)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %+v", err)
	}
	openstackClientFactory, err := a.openstackClientFactory.NewFactory(credentials)
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create Openstack client factory: %+v", err), helper.KnownCodes)
	}
	dnsClient, err := openstackClientFactory.DNS()
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create Openstack DNS client: %+v", err), helper.KnownCodes)
	}

	// Determine DNS zone ID
	zone, err := a.getZone(ctx, log, dns, dnsClient)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// Delete DNS recordset
	log.Info("Deleting DNS recordset", "zone", zone, "name", dns.Spec.Name, "type", dns.Spec.RecordType, "dnsrecord", k8sclient.ObjectKeyFromObject(dns))
	if err := dnsClient.DeleteRecordSet(ctx, zone, dns.Spec.Name, string(dns.Spec.RecordType)); err != nil {
		return &reconcilerutils.RequeueAfterError{
			Cause:        fmt.Errorf("could not delete DNS recordset in zone %s with name %s and type %s: %+v", zone, dns.Spec.Name, dns.Spec.RecordType, err),
			RequeueAfter: requeueAfterOnProviderError,
		}
	}

	return nil
}

// ForceDelete forcefully deletes the DNSRecord.
func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, dns, cluster)
}

// Restore restores the DNSRecord.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, dns, cluster)
}

// Migrate migrates the DNSRecord.
func (a *actuator) Migrate(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	return nil
}

func (a *actuator) getZone(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, dnsClient openstackclient.DNS) (string, error) {
	switch {
	case dns.Spec.Zone != nil && *dns.Spec.Zone != "":
		return *dns.Spec.Zone, nil
	case dns.Status.Zone != nil && *dns.Status.Zone != "":
		return *dns.Status.Zone, nil
	default:
		// The zone is not specified in the resource status or spec. Try to determine the zone by
		// getting all zones of the account and searching for the longest zone name that is a suffix of dns.spec.Name
		zones, err := dnsClient.GetZones(ctx)
		if err != nil {
			return "", &reconcilerutils.RequeueAfterError{
				Cause:        fmt.Errorf("could not get DNS zones: %+v", err),
				RequeueAfter: requeueAfterOnProviderError,
			}
		}
		log.Info("Got DNS zones", "zones", zones, "dnsrecord", k8sclient.ObjectKeyFromObject(dns))
		zone := dnsrecord.FindZoneForName(zones, dns.Spec.Name)
		if zone == "" {
			return "", fmt.Errorf("could not find DNS zone for name %s", dns.Spec.Name)
		}
		return zone, nil
	}
}
