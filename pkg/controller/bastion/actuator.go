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
	"k8s.io/utils/ptr"
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
	client client.Client

	openstackClientFactory openstackclient.FactoryFactory
	bastionConfig          *controllerconfig.BastionConfig
}

func newActuator(mgr manager.Manager, openstackClientFactory openstackclient.FactoryFactory, bastionConfig *controllerconfig.BastionConfig) bastion.Actuator {
	return &actuator{
		client:                 mgr.GetClient(),
		openstackClientFactory: openstackClientFactory,
		bastionConfig:          bastionConfig,
	}
}

func findBastionInstances(ctx context.Context, client openstackclient.Compute, name string) ([]servers.Server, error) {
	return client.FindServersByName(ctx, name)
}

func getBastionInstance(ctx context.Context, client openstackclient.Compute, name string) (servers.Server, error) {
	instances, err := findBastionInstances(ctx, client, name)
	if err != nil {
		return servers.Server{}, fmt.Errorf("failed to get bastion instance: %w", err)
	}
	if len(instances) != 1 {
		return servers.Server{}, fmt.Errorf("expected exactly one bastion instance, got %d", len(instances))
	}
	return instances[0], nil
}

func createBastionInstance(ctx context.Context, client openstackclient.Compute, parameters servers.CreateOpts) (servers.Server, error) {
	server, err := client.CreateServer(ctx, parameters)
	return ptr.Deref(server, servers.Server{}), err
}

func deleteBastionInstance(ctx context.Context, client openstackclient.Compute, id string) error {
	return client.DeleteServer(ctx, id)
}

// GetIPs returns the first found private and public IPs for the given server and options.
func GetIPs(s servers.Server, opts Options) (string, string, error) {
	var privateIP, publicIP string

	type InstanceNic struct {
		MacAddr string `json:"OS-EXT-IPS-MAC:mac_addr"`
		Version int    `json:"version"`
		Addr    string `json:"addr"`
		Type    string `json:"OS-EXT-IPS:type"`
	}

	var instanceNic []InstanceNic

	addresses, ok := s.Addresses[opts.ShootName]
	if !ok {
		return "", "", fmt.Errorf("network %q not found in server addresses", opts.ShootName)
	}
	bytes, err := json.Marshal(addresses)
	if err != nil {
		return "", "", err
	}
	err = json.Unmarshal(bytes, &instanceNic)
	if err != nil {
		return "", "", err
	}

	for _, v := range instanceNic {
		switch v.Type {
		case "fixed":
			if privateIP == "" {
				privateIP = v.Addr
			}
		case "floating":
			if publicIP == "" {
				publicIP = v.Addr
			}
		}
	}

	if privateIP == "" {
		return "", "", fmt.Errorf("no private IP found")
	}
	if publicIP == "" {
		return "", "", fmt.Errorf("no public IP found")
	}

	return privateIP, publicIP, nil
}

func createFloatingIP(ctx context.Context, client openstackclient.Networking, parameters floatingips.CreateOpts) (floatingips.FloatingIP, error) {
	fip, err := client.CreateFloatingIP(ctx, parameters)
	return ptr.Deref(fip, floatingips.FloatingIP{}), err
}

func deleteFloatingIP(ctx context.Context, client openstackclient.Networking, id string) error {
	return client.DeleteFloatingIP(ctx, id)
}

func findFipByName(ctx context.Context, client openstackclient.Networking, name string) ([]floatingips.FloatingIP, error) {
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
