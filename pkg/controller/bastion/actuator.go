// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"errors"
	"fmt"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/go-logr/logr"
	computefip "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// sshPort is the default SSH Port used for bastion ingress firewall rule
	sshPort = 22
)

type actuator struct {
	common.ClientContext
	client client.Client

	openstackClientFactory openstackclient.FactoryFactory
	logger                 logr.Logger
	bastionConfig          *controllerconfig.BastionConfig
}

func newActuator(openstackClientFactory openstackclient.FactoryFactory, bastionConfig *controllerconfig.BastionConfig) bastion.Actuator {
	return &actuator{
		openstackClientFactory: openstackClientFactory,
		logger:                 logger,
		bastionConfig:          bastionConfig,
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

func getBastionInstance(client openstackclient.Compute, name string) ([]servers.Server, error) {
	return client.FindServersByName(name)
}

func createBastionInstance(client openstackclient.Compute, parameters servers.CreateOpts) (*servers.Server, error) {
	return client.CreateServer(parameters)
}

func deleteBastionInstance(client openstackclient.Compute, id string) error {
	return client.DeleteServer(id)
}

// GetIPs return privateip, publicip
func GetIPs(s *servers.Server, opt *Options) (string, string, error) {
	var privateIP, publicIp string

	type InstanceNic struct {
		MacAddr string `json:"OS-EXT-IPS-MAC:mac_addr"`
		Version int    `json:"version"`
		Addr    string `json:"addr"`
		Type    string `json:"OS-EXT-IPS:type"`
	}

	instanceNic := []InstanceNic{}

	if len(s.Addresses) == 0 {
		return "", "", fmt.Errorf("NIC not ready yet")
	}

	bytes, err := json.Marshal(s.Addresses[opt.ShootName])
	if err != nil {
		return "", "", err
	}
	err = json.Unmarshal(bytes, &instanceNic)
	if err != nil {
		return "", "", err
	}

	for i, v := range instanceNic {
		if v.Type == "fixed" {
			privateIP = instanceNic[i].Addr
		} else {
			publicIp = instanceNic[i].Addr
		}
	}

	return privateIP, publicIp, nil
}

func createFloatingIP(client openstackclient.Networking, parameters floatingips.CreateOpts) (*floatingips.FloatingIP, error) {
	return client.CreateFloatingIP(parameters)
}

func deleteFloatingIP(client openstackclient.Networking, id string) error {
	return client.DeleteFloatingIP(id)
}

func associateFIPWithInstance(client openstackclient.Compute, id string, parameter computefip.AssociateOpts) error {
	return client.AssociateFIPWithInstance(id, parameter)
}

func findFloatingIDByInstanceID(client openstackclient.Compute, id string) (string, error) {
	return client.FindFloatingIDByInstanceID(id)
}

func getFipByName(client openstackclient.Networking, name string) ([]floatingips.FloatingIP, error) {
	return client.GetFipByName(name)
}

func createSecurityGroup(client openstackclient.Networking, createOpts groups.CreateOpts) (*groups.SecGroup, error) {
	return client.CreateSecurityGroup(createOpts)
}

func deleteSecurityGroup(client openstackclient.Networking, groupid string) error {
	return client.DeleteSecurityGroup(groupid)
}

func getSecurityGroups(client openstackclient.Networking, name string) ([]groups.SecGroup, error) {
	return client.GetSecurityGroupByName(name)
}

func createRules(client openstackclient.Networking, createOpts rules.CreateOpts) (*rules.SecGroupRule, error) {
	return client.CreateRule(createOpts)
}

func listRules(client openstackclient.Networking, secGroupID string) ([]rules.SecGroupRule, error) {
	listOpts := rules.ListOpts{
		SecGroupID: secGroupID,
	}
	return client.ListRules(listOpts)
}

func deleteRule(client openstackclient.Networking, ruleID string) error {
	return client.DeleteRule(ruleID)
}

func bastionConfigCheck(bastionConfig *controllerconfig.BastionConfig) error {
	if bastionConfig == nil {
		return errors.New("bastionConfig must not be empty")
	}

	if bastionConfig.FlavorRef == "" {
		return errors.New("bastion not supported as no flavor is configured for the bastion host machine")
	}

	if bastionConfig.ImageRef == "" {
		return errors.New("bastion not supported as no Image is configured for the bastion host machine")
	}
	return nil
}
