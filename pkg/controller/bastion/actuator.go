// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

const (
	// sshPort is the default SSH Port used for bastion ingress firewall rule
	sshPort = 22
)

type actuator struct {
	client  client.Client
	decoder runtime.Decoder

	openstackClientFactory openstackclient.FactoryFactory
	bastionConfig          *controllerconfig.BastionConfig
}

func newActuator(mgr manager.Manager, openstackClientFactory openstackclient.FactoryFactory, bastionConfig *controllerconfig.BastionConfig) bastion.Actuator {
	return &actuator{
		client:                 mgr.GetClient(),
		decoder:                serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		openstackClientFactory: openstackClientFactory,
		bastionConfig:          bastionConfig,
	}
}

func getBastionInstance(ctx context.Context, client openstackclient.Compute, name string) ([]servers.Server, error) {
	return client.FindServersByName(ctx, name)
}

func createBastionInstance(ctx context.Context, client openstackclient.Compute, parameters servers.CreateOpts) (*servers.Server, error) {
	return client.CreateServer(ctx, parameters)
}

func deleteBastionInstance(ctx context.Context, client openstackclient.Compute, id string) error {
	return client.DeleteServer(ctx, id)
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

func createFloatingIP(ctx context.Context, client openstackclient.Networking, parameters floatingips.CreateOpts) (*floatingips.FloatingIP, error) {
	return client.CreateFloatingIP(ctx, parameters)
}

func deleteFloatingIP(ctx context.Context, client openstackclient.Networking, id string) error {
	return client.DeleteFloatingIP(ctx, id)
}

func getFipByName(ctx context.Context, client openstackclient.Networking, name string) ([]floatingips.FloatingIP, error) {
	return client.GetFipByName(ctx, name)
}

func createSecurityGroup(ctx context.Context, client openstackclient.Networking, createOpts groups.CreateOpts) (*groups.SecGroup, error) {
	return client.CreateSecurityGroup(ctx, createOpts)
}

func deleteSecurityGroup(ctx context.Context, client openstackclient.Networking, groupid string) error {
	return client.DeleteSecurityGroup(ctx, groupid)
}

func getSecurityGroups(ctx context.Context, client openstackclient.Networking, name string) ([]groups.SecGroup, error) {
	return client.GetSecurityGroupByName(ctx, name)
}

func createRules(ctx context.Context, client openstackclient.Networking, createOpts rules.CreateOpts) (*rules.SecGroupRule, error) {
	return client.CreateRule(ctx, createOpts)
}

func listRules(ctx context.Context, client openstackclient.Networking, secGroupID string) ([]rules.SecGroupRule, error) {
	listOpts := rules.ListOpts{
		SecGroupID: secGroupID,
	}
	return client.ListRules(ctx, listOpts)
}

func deleteRule(ctx context.Context, client openstackclient.Networking, ruleID string) error {
	return client.DeleteRule(ctx, ruleID)
}

func useBastionControllerConfig(bastionConfig *controllerconfig.BastionConfig) bool {
	if bastionConfig == nil || bastionConfig.FlavorRef == "" || bastionConfig.ImageRef == "" {
		return false
	}

	return true
}
