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

package client

import (
	"context"
	"reflect"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/dns/v2/recordsets"
	"github.com/gophercloud/gophercloud/openstack/dns/v2/zones"
)

// GetZones returns a map of all zone names mapped to their IDs.
func (c *DNSClient) GetZones(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	allPages, err := zones.List(c.client, zones.ListOpts{}).AllPages()
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
	rs, err := c.getRecordSet(zoneID, name, recordType)
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
			_, err := recordsets.Update(c.client, zoneID, rs.ID, updateOpts).Extract()
			return err
		}
	} else {
		createOpts := recordsets.CreateOpts{
			Name:    ensureTrailingDot(name),
			Type:    recordType,
			Records: records,
			TTL:     ttl,
		}
		_, err := recordsets.Create(c.client, zoneID, createOpts).Extract()
		return err
	}
	return nil
}

// DeleteRecordSet deletes the recordset with the given name and record type in the zone with the given zone ID.
func (c *DNSClient) DeleteRecordSet(ctx context.Context, zoneID, name, recordType string) error {
	rs, err := c.getRecordSet(zoneID, name, recordType)
	if err != nil {
		return err
	}
	if rs != nil {
		if err := recordsets.Delete(c.client, zoneID, rs.ID).ExtractErr(); !IsNotFoundError(err) {
			return err
		}
	}
	return nil
}

func (c *DNSClient) getRecordSet(zoneID, name, recordType string) (*recordsets.RecordSet, error) {
	listOpts := recordsets.ListOpts{
		Name: ensureTrailingDot(name),
		Type: recordType,
	}
	allPages, err := recordsets.ListByZone(c.client, zoneID, listOpts).AllPages()
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
