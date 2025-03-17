// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"reflect"
	"strings"

	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/zones"
)

// GetZones returns a map of all zone names mapped to their IDs.
func (c *DNSClient) GetZones(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	allPages, err := zones.List(c.client, zones.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	zones, err := zones.ExtractZones(allPages)
	if err != nil {
		return nil, err
	}
	for _, zone := range zones {
		result[normalizeName(zone.Name)] = zone.ID
	}
	return result, nil
}

// CreateOrUpdateRecordSet creates or updates the recordset with the given name, record type, records, and ttl
// in the zone with the given zone ID.
func (c *DNSClient) CreateOrUpdateRecordSet(ctx context.Context, zoneID, name, recordType string, records []string, ttl int) error {
	rs, err := c.getRecordSet(ctx, zoneID, name, recordType)
	if err != nil {
		return err
	}
	if recordType == "CNAME" {
		records = []string{ensureTrailingDot(records[0])}
	}
	if rs != nil {
		if !reflect.DeepEqual(rs.Records, records) || rs.TTL != ttl {
			updateOpts := recordsets.UpdateOpts{
				Records: records,
				TTL:     &ttl,
			}
			_, err := recordsets.Update(ctx, c.client, zoneID, rs.ID, updateOpts).Extract()
			return err
		}
		return nil
	}
	createOpts := recordsets.CreateOpts{
		Name:    ensureTrailingDot(name),
		Type:    recordType,
		Records: records,
		TTL:     ttl,
	}
	_, err = recordsets.Create(ctx, c.client, zoneID, createOpts).Extract()
	return err
}

// DeleteRecordSet deletes the recordset with the given name and record type in the zone with the given zone ID.
func (c *DNSClient) DeleteRecordSet(ctx context.Context, zoneID, name, recordType string) error {
	rs, err := c.getRecordSet(ctx, zoneID, name, recordType)
	if err != nil {
		return err
	}
	if rs != nil {
		if err := recordsets.Delete(ctx, c.client, zoneID, rs.ID).ExtractErr(); !IsNotFoundError(err) {
			return err
		}
	}
	return nil
}

func (c *DNSClient) getRecordSet(ctx context.Context, zoneID, name, recordType string) (*recordsets.RecordSet, error) {
	listOpts := recordsets.ListOpts{
		Name: ensureTrailingDot(name),
		Type: recordType,
	}
	allPages, err := recordsets.ListByZone(c.client, zoneID, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	rss, err := recordsets.ExtractRecordSets(allPages)
	if err != nil {
		return nil, err
	}
	if len(rss) > 0 {
		return &rss[0], nil
	}
	return nil, nil
}

func normalizeName(name string) string {
	if strings.HasPrefix(name, "\\052.") {
		name = "*" + name[4:]
	}
	if strings.HasSuffix(name, ".") {
		return name[:len(name)-1]
	}
	return name
}

func ensureTrailingDot(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}
