// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenv1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/v2/openstack/sharedfilesystems/v2/sharenetworks"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

const (
	defaultTimeout     = 90 * time.Second
	defaultLongTimeout = 3 * time.Minute
)

// Reconcile creates and runs the flow to reconcile the AWS infrastructure.
func (fctx *FlowContext) Reconcile(ctx context.Context) error {
	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithSpan().WithLogger(fctx.log).WithPersist(fctx.persistState)
	g := fctx.buildReconcileGraph()
	f := g.Compile()
	if err := f.Run(ctx, flow.Opts{Log: fctx.log}); err != nil {
		fctx.log.Error(err, "flow reconciliation failed")
		return errors.Join(flow.Causes(err), fctx.persistState(ctx))
	}

	state := fctx.computeInfrastructureState()
	status := fctx.computeInfrastructureStatus()
	nodeCIDR := fctx.state.Get(IdentifierNodeSubnetIPv6CIDR)
	podCIDR := fctx.state.Get(IdentifierPodSubnetIPv6CIDR)
	svcCIDR := fctx.state.Get(IdentifierServiceSubnetIPv6CIDR)
	return PatchProviderStatusAndState(ctx, fctx.client, fctx.infra, fctx.shootNetworking, status, state, nodeCIDR, podCIDR, svcCIDR)
	//return PatchProviderStatusAndState(ctx, fctx.client, fctx.infra, fctx.shootNetworking, status, state, nil, nil, nil)
}

func (fctx *FlowContext) buildReconcileGraph() *flow.Graph {
	g := flow.NewGraph("Openstack infrastructure reconciliation")

	prehook := fctx.AddTask(g, "pre-reconcile hook", func(_ context.Context) error {
		// delete unnecessary state object. RouterIP was replaced by IdentifierEgressCIDRs to handle cases where the router had multiple externalFixedIPs attached to it.
		fctx.state.Delete(RouterIP)
		return nil
	})

	ensureExternalNetwork := fctx.AddTask(g, "ensure external network",
		fctx.ensureExternalNetwork,
		shared.Timeout(defaultTimeout), shared.Dependencies(prehook))

	ensureRouter := fctx.AddTask(g, "ensure router",
		fctx.ensureRouter,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureExternalNetwork))

	ensureNetwork := fctx.AddTask(g, "ensure network",
		fctx.ensureNetwork,
		shared.Timeout(defaultTimeout), shared.Dependencies(prehook))

	ensureSubnet := fctx.AddTask(g, "ensure subnet",
		fctx.ensureSubnet,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureNetwork))

	ensureSubnetIPv6 := fctx.AddTask(g, "ensure IPv6 subnet",
		fctx.ensureSubnetIPv6,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureNetwork))

	_ = fctx.AddTask(g, "ensure router interface",
		fctx.ensureRouterInterface,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureRouter, ensureSubnet))

	_ = fctx.AddTask(g, "ensure IPv6 router interface",
		fctx.ensureRouterInterfaceIPv6,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureRouter, ensureSubnetIPv6))

	_ = fctx.AddTask(g, "ensure IPv6 CIDR services", fctx.ensureIPv6CIDRs,
		shared.Timeout(defaultTimeout),
		shared.Dependencies(ensureSubnetIPv6),
		shared.DoIf(fctx.isDualStack()),
	)

	ensureSecGroup := fctx.AddTask(g, "ensure security group",
		fctx.ensureSecGroup,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureRouter))

	_ = fctx.AddTask(g, "ensure security group rules",
		fctx.ensureSecGroupRules,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureSecGroup))

	_ = fctx.AddTask(g, "ensure ssh key pair",
		fctx.ensureSSHKeyPair,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureRouter))

	_ = fctx.AddTask(g, "ensure share network",
		fctx.ensureShareNetwork,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureSubnet),
	)

	return g
}

func (fctx *FlowContext) ensureExternalNetwork(ctx context.Context) error {
	externalNetwork, err := fctx.networking.GetExternalNetworkByName(ctx, fctx.config.FloatingPoolName)
	if err != nil {
		return err
	}
	if externalNetwork == nil {
		return fmt.Errorf("external network for floating pool name %s not found", fctx.config.FloatingPoolName)
	}
	fctx.state.Set(IdentifierFloatingNetwork, externalNetwork.ID)
	fctx.state.Set(NameFloatingNetwork, externalNetwork.Name)
	return nil
}

func (fctx *FlowContext) ensureRouter(ctx context.Context) error {
	externalNetworkID := fctx.state.Get(IdentifierFloatingNetwork)
	if externalNetworkID == nil {
		return fmt.Errorf("missing external network ID")
	}

	if fctx.config.Networks.Router != nil {
		return fctx.ensureConfiguredRouter(ctx)
	}
	return fctx.ensureNewRouter(ctx, *externalNetworkID)
}

func (fctx *FlowContext) ensureConfiguredRouter(ctx context.Context) error {
	router, err := fctx.access.GetRouterByID(ctx, fctx.config.Networks.Router.ID)
	if err != nil {
		fctx.state.Set(IdentifierRouter, "")
		return err
	}
	if router == nil {
		fctx.state.Set(IdentifierRouter, "")
		fctx.state.Set(RouterIP, "")
		return fmt.Errorf("missing expected router %s", fctx.config.Networks.Router.ID)
	}
	fctx.state.Set(IdentifierRouter, fctx.config.Networks.Router.ID)
	if len(router.ExternalFixedIPs) < 1 {
		return fmt.Errorf("expected at least one external fixed ip")
	}

	return fctx.ensureEgressCIDRs(router)
}

func (fctx *FlowContext) ensureNewRouter(ctx context.Context, externalNetworkID string) error {
	log := shared.LogFromContext(ctx)

	desired := &access.Router{
		Name:              fctx.defaultRouterName(),
		ExternalNetworkID: externalNetworkID,
		EnableSNAT:        fctx.cloudProfileConfig.UseSNAT,
	}
	current, err := fctx.findExistingRouter(ctx)
	if err != nil {
		return err
	}
	if current != nil {
		if len(current.ExternalFixedIPs) < 1 {
			return fmt.Errorf("expected at least one external fixed ip")
		}
		if _, current, err = fctx.access.UpdateRouter(ctx, desired, current); err != nil {
			return err
		}
		fctx.state.Set(IdentifierRouter, current.ID)
		return fctx.ensureEgressCIDRs(current)
	}

	floatingPoolSubnetName := fctx.findFloatingPoolSubnetName()
	fctx.state.SetPtr(NameFloatingPoolSubnet, floatingPoolSubnetName)
	if floatingPoolSubnetName != nil {
		log.Info("looking up floating pool subnets...")
		desired.ExternalSubnetIDs, err = fctx.access.LookupFloatingPoolSubnetIDs(ctx, externalNetworkID, *floatingPoolSubnetName)
		if err != nil {
			return err
		}
	}
	log.Info("creating...")
	// TODO: add tags to created resources
	created, err := fctx.access.CreateRouter(ctx, desired)
	if err != nil {
		return err
	}

	fctx.state.Set(IdentifierRouter, created.ID)
	return fctx.ensureEgressCIDRs(created)
}

func (fctx *FlowContext) findExistingRouter(ctx context.Context) (*access.Router, error) {
	return findExisting(ctx, fctx.state.Get(IdentifierRouter), fctx.defaultRouterName(), fctx.access.GetRouterByID, fctx.access.GetRouterByName)
}

func (fctx *FlowContext) findFloatingPoolSubnetName() *string {
	if fctx.config.FloatingPoolSubnetName != nil {
		return fctx.config.FloatingPoolSubnetName
	}

	// Second: Check if the CloudProfile contains a default floating subnet and use it.
	if floatingPool, err := helper.FindFloatingPool(fctx.cloudProfileConfig.Constraints.FloatingPools, fctx.config.FloatingPoolName, fctx.infra.Spec.Region, nil); err == nil && floatingPool.DefaultFloatingSubnet != nil {
		return floatingPool.DefaultFloatingSubnet
	}

	return nil
}

func (fctx *FlowContext) ensureNetwork(ctx context.Context) error {
	if fctx.config.Networks.ID != nil {
		return fctx.ensureConfiguredNetwork(ctx)
	}
	return fctx.ensureNewNetwork(ctx)
}

func (fctx *FlowContext) ensureConfiguredNetwork(ctx context.Context) error {
	networkId := *fctx.config.Networks.ID
	network, err := fctx.access.GetNetworkByID(ctx, networkId)
	if err != nil {
		fctx.state.Set(IdentifierNetwork, "")
		fctx.state.Set(NameNetwork, "")
		return err
	}
	if network == nil {
		return gardenv1beta1helper.NewErrorWithCodes(
			fmt.Errorf("network with ID '%s' was not found", networkId),
			gardencorev1beta1.ErrorInfraDependencies,
		)
	}
	fctx.state.Set(IdentifierNetwork, networkId)
	fctx.state.Set(NameNetwork, network.Name)
	return nil
}

func (fctx *FlowContext) ensureNewNetwork(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	desired := &access.Network{
		Name:         fctx.defaultNetworkName(),
		AdminStateUp: true,
	}
	current, err := fctx.findExistingNetwork(ctx)
	if err != nil {
		return err
	}
	if current != nil {
		fctx.state.Set(IdentifierNetwork, current.ID)
		fctx.state.Set(NameNetwork, current.Name)
		if _, err := fctx.access.UpdateNetwork(ctx, desired, current); err != nil {
			return err
		}
	} else {
		log.Info("creating...")
		created, err := fctx.access.CreateNetwork(ctx, desired)
		if err != nil {
			return err
		}
		fctx.state.Set(IdentifierNetwork, created.ID)
		fctx.state.Set(NameNetwork, created.Name)
	}

	return nil
}

func (fctx *FlowContext) findExistingNetwork(ctx context.Context) (*access.Network, error) {
	return findExisting(ctx, fctx.state.Get(IdentifierNetwork), fctx.defaultNetworkName(), fctx.access.GetNetworkByID, fctx.access.GetNetworkByName)
}

func (fctx *FlowContext) getNetworkID(ctx context.Context) (*string, error) {
	if fctx.config.Networks.ID != nil {
		return fctx.config.Networks.ID, nil
	}
	networkID := fctx.state.Get(IdentifierNetwork)
	if networkID != nil {
		return networkID, nil
	}
	network, err := fctx.findExistingNetwork(ctx)
	if err != nil {
		return nil, err
	}
	if network != nil {
		fctx.state.Set(IdentifierNetwork, network.ID)
		return &network.ID, nil
	}
	return nil, nil
}

func (fctx *FlowContext) ensureSubnet(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	if fctx.state.Get(IdentifierNetwork) == nil {
		return fmt.Errorf("missing cluster network ID")
	}
	networkID := ptr.Deref(fctx.state.Get(IdentifierNetwork), "")
	// Backwards compatibility - remove this code in a future version.
	desired := &subnets.Subnet{
		Name:           fctx.defaultSubnetName(),
		NetworkID:      networkID,
		CIDR:           fctx.workersCIDR(),
		IPVersion:      4,
		DNSNameservers: fctx.cloudProfileConfig.DNSServers,
	}
	current, err := fctx.findExistingSubnet(ctx)
	if err != nil {
		return err
	}
	if current != nil {
		fctx.state.Set(IdentifierSubnet, current.ID)
		log.Info("updating...")
		if _, err := fctx.access.UpdateSubnet(ctx, desired, current); err != nil {
			return err
		}
	} else {
		log.Info("creating...")
		created, err := fctx.access.CreateSubnet(ctx, desired)
		if err != nil {
			return err
		}
		fctx.state.Set(IdentifierSubnet, created.ID)
	}
	return nil
}

func (fctx *FlowContext) ensureSubnetIPv6(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	if !fctx.isDualStack() {
		return nil
	}
	if fctx.state.Get(IdentifierNetwork) == nil {
		return fmt.Errorf("missing cluster network ID")
	}
	networkID := ptr.Deref(fctx.state.Get(IdentifierNetwork), "")

	// Use configured subnet pool ID if provided
	var subnetPoolID string
	if fctx.config.SubnetPoolID != nil {
		subnetPoolID = *fctx.config.SubnetPoolID
	}

	desired := []subnets.Subnet{{
		Name:            fctx.defaultSubnetIPv6Name(),
		NetworkID:       networkID,
		IPVersion:       6,
		DNSNameservers:  fctx.cloudProfileConfig.DNSServers,
		IPv6RAMode:      "slaac",
		IPv6AddressMode: "slaac",
		SubnetPoolID:    subnetPoolID,
	},
		{
			Name:           fctx.defaultSubnetIPv6Name() + "-pod",
			NetworkID:      networkID,
			IPVersion:      6,
			DNSNameservers: fctx.cloudProfileConfig.DNSServers,
			// IPv6RAMode:      "none",
			// IPv6AddressMode: "none",
			SubnetPoolID: subnetPoolID,
		},
		{
			Name:           fctx.defaultSubnetIPv6Name() + "-svc",
			NetworkID:      networkID,
			IPVersion:      6,
			DNSNameservers: fctx.cloudProfileConfig.DNSServers,
			// IPv6RAMode:      "none",
			// IPv6AddressMode: "none",
			SubnetPoolID: subnetPoolID,
		}}

	for _, desiredSubnet := range desired {
		current, err := fctx.findExistingSubnetIPv6(ctx, desiredSubnet.Name)
		if err != nil {
			return err
		}

		if current != nil {
			fctx.state.Set(getSubnetIdentifierBySuffix(desiredSubnet.Name), current.ID)
			log.Info("updating...")
			if _, err := fctx.access.UpdateSubnet(ctx, &desiredSubnet, current); err != nil {
				return err
			}
		} else {
			log.Info("creating...")
			created, err := fctx.access.CreateSubnet(ctx, &desiredSubnet)
			if err != nil {
				return err
			}
			fctx.state.Set(getSubnetIdentifierBySuffix(desiredSubnet.Name), created.ID)
		}
	}
	return nil
}

func (fctx *FlowContext) ensureIPv6CIDRs(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	if !fctx.isDualStack() {
		return nil
	}

	cidrIdentifiers := []string{IdentifierNodeSubnetIPv6CIDR, IdentifierPodSubnetIPv6CIDR, IdentifierServiceSubnetIPv6CIDR}
	for index, identifier := range []string{IdentifierSubnetIPv6, IdentifierSubnetIPv6Pod, IdentifierSubnetIPv6Svc} {
		// Get the actual IPv6 CIDR from the created subnet
		subnetIPv6ID := fctx.state.Get(identifier)
		if subnetIPv6ID == nil {
			return fmt.Errorf("missing IPv6 subnet ID for identifier %s", identifier)
		}

		// Wait for the IPv6 CIDR to be allocated by the subnet pool
		var actualIPv6CIDR string

		log.V(1).Info("waiting for IPv6 CIDR allocation from subnet pool...")
		err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
			subnet, err := fctx.access.GetSubnetByID(ctx, *subnetIPv6ID)
			if err != nil {
				log.Error(err, "failed to get IPv6 subnet during CIDR wait")
				return false, err
			}
			if subnet == nil {
				return false, fmt.Errorf("IPv6 subnet not found for id %s", *subnetIPv6ID)
			}

			// Check if CIDR has been allocated
			if subnet.CIDR != "" {
				actualIPv6CIDR = subnet.CIDR
				log.Info("IPv6 CIDR allocated from subnet pool", "cidr", actualIPv6CIDR)
				return true, nil
			}

			log.V(1).Info("IPv6 CIDR not yet allocated, continuing to wait")
			return false, nil
		})

		if err != nil {
			return fmt.Errorf("failed waiting for IPv6 CIDR allocation for identifier %s: %w", identifier, err)
		}

		if cidrIdentifiers[index] == IdentifierServiceSubnetIPv6CIDR {
			ip, _, err := net.ParseCIDR(actualIPv6CIDR)
			if err != nil {
				return fmt.Errorf("failed to parse allocated IPv6 service CIDR %s: %w", actualIPv6CIDR, err)
			}
			actualIPv6CIDR = fmt.Sprintf("%s/112", ip.String())
		}

		fctx.state.Set(cidrIdentifiers[index], actualIPv6CIDR)
	}
	return nil
}

func (fctx *FlowContext) findExistingSubnet(ctx context.Context) (*subnets.Subnet, error) {
	networkID, err := fctx.getNetworkID(ctx)
	if err != nil {
		return nil, err
	}
	if networkID == nil {
		return nil, nil
	}
	getByName := func(ctx context.Context, name string) ([]*subnets.Subnet, error) {
		return fctx.access.GetSubnetByName(ctx, *networkID, name)
	}
	return findExisting(ctx, fctx.state.Get(IdentifierSubnet), fctx.defaultSubnetName(), fctx.access.GetSubnetByID, getByName)
}

// getSubnetIdentifierBySuffix returns the appropriate subnet identifier based on subnet name suffix
func getSubnetIdentifierBySuffix(subnetName string) string {
	if strings.HasSuffix(subnetName, "-pod") {
		return IdentifierSubnetIPv6Pod
	} else if strings.HasSuffix(subnetName, "-svc") {
		return IdentifierSubnetIPv6Svc
	}
	return IdentifierSubnetIPv6
}

func (fctx *FlowContext) findExistingSubnetIPv6(ctx context.Context, subnetName string) (*subnets.Subnet, error) {
	networkID, err := fctx.getNetworkID(ctx)
	if err != nil {
		return nil, err
	}
	if networkID == nil {
		return nil, nil
	}
	getByName := func(ctx context.Context, name string) ([]*subnets.Subnet, error) {
		return fctx.access.GetSubnetByName(ctx, *networkID, name)
	}
	identifier := getSubnetIdentifierBySuffix(subnetName)

	return findExisting(ctx, fctx.state.Get(identifier), subnetName, fctx.access.GetSubnetByID, getByName)
}

func (fctx *FlowContext) ensureRouterInterface(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	routerID := fctx.state.Get(IdentifierRouter)
	if routerID == nil {
		return fmt.Errorf("internal error: missing routerID")
	}
	subnetID := fctx.state.Get(IdentifierSubnet)
	if subnetID == nil {
		return fmt.Errorf("internal error: missing subnetID")
	}
	portID, err := fctx.access.GetRouterInterfacePortID(ctx, *routerID, *subnetID)
	if err != nil {
		return err
	}
	if portID != nil {
		return nil
	}
	log.Info("creating...")
	return fctx.access.AddRouterInterfaceAndWait(ctx, *routerID, *subnetID)
}

func (fctx *FlowContext) ensureRouterInterfaceIPv6(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	if !fctx.isDualStack() {
		return nil
	}
	routerID := fctx.state.Get(IdentifierRouter)
	if routerID == nil {
		return fmt.Errorf("internal error: missing routerID")
	}
	subnetIPv6ID := fctx.state.Get(IdentifierSubnetIPv6)
	if subnetIPv6ID == nil {
		return fmt.Errorf("internal error: missing IPv6 subnetID")
	}
	portID, err := fctx.access.GetRouterInterfacePortID(ctx, *routerID, *subnetIPv6ID)
	if err != nil {
		return err
	}
	if portID != nil {
		return nil
	}
	log.Info("creating...")
	return fctx.access.AddRouterInterfaceAndWait(ctx, *routerID, *subnetIPv6ID)
}

func (fctx *FlowContext) ensureSecGroup(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	desired := &groups.SecGroup{
		Name:        fctx.defaultSecurityGroupName(),
		Description: "Cluster Nodes",
	}
	current, err := findExisting(ctx, fctx.state.Get(IdentifierSecGroup), fctx.defaultSecurityGroupName(), fctx.access.GetSecurityGroupByID, fctx.access.GetSecurityGroupByName)
	if err != nil {
		return err
	}

	if current != nil {
		fctx.state.Set(IdentifierSecGroup, current.ID)
		fctx.state.Set(NameSecGroup, current.Name)
		fctx.state.SetObject(ObjectSecGroup, current)
		return nil
	}

	log.Info("creating...")
	created, err := fctx.access.CreateSecurityGroup(ctx, desired)
	if err != nil {
		return err
	}
	fctx.state.Set(IdentifierSecGroup, created.ID)
	fctx.state.Set(NameSecGroup, created.Name)
	fctx.state.SetObject(ObjectSecGroup, created)
	return nil
}

func (fctx *FlowContext) ensureSecGroupRules(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	obj := fctx.state.GetObject(ObjectSecGroup)
	if obj == nil {
		return fmt.Errorf("internal error: security group object not found")
	}
	group, ok := obj.(*groups.SecGroup)
	if !ok {
		return fmt.Errorf("internal error: casting to SecGroup failed")
	}

	desiredRules := []rules.SecGroupRule{
		{
			Direction:     string(rules.DirIngress),
			EtherType:     string(rules.EtherType4),
			RemoteGroupID: access.SecurityGroupIDSelf,
			Description:   "IPv4: allow all incoming traffic within the same security group",
		},
		{
			Direction:   string(rules.DirEgress),
			EtherType:   string(rules.EtherType4),
			Description: "IPv4: allow all outgoing traffic",
		},
		{
			Direction:   string(rules.DirEgress),
			EtherType:   string(rules.EtherType6),
			Description: "IPv6: allow all outgoing traffic",
		},
		{
			Direction:      string(rules.DirIngress),
			EtherType:      string(rules.EtherType4),
			Protocol:       string(rules.ProtocolTCP),
			PortRangeMin:   30000,
			PortRangeMax:   32767,
			RemoteIPPrefix: "0.0.0.0/0",
			Description:    "IPv4: allow all incoming tcp traffic with port range 30000-32767",
		},
		{
			Direction:      string(rules.DirIngress),
			EtherType:      string(rules.EtherType4),
			Protocol:       string(rules.ProtocolUDP),
			PortRangeMin:   30000,
			PortRangeMax:   32767,
			RemoteIPPrefix: "0.0.0.0/0",
			Description:    "IPv4: allow all incoming udp traffic with port range 30000-32767",
		},
	}

	if fctx.isDualStack() {
		desiredRules = append(desiredRules, []rules.SecGroupRule{
			{
				Direction:     string(rules.DirIngress),
				EtherType:     string(rules.EtherType6),
				RemoteGroupID: access.SecurityGroupIDSelf,
				Description:   "IPv6: allow all incoming traffic within the same security group",
			},
			{
				Direction:      string(rules.DirIngress),
				EtherType:      string(rules.EtherType6),
				Protocol:       string(rules.ProtocolTCP),
				PortRangeMin:   30000,
				PortRangeMax:   32767,
				RemoteIPPrefix: "::/0",
				Description:    "IPv6: allow all incoming tcp traffic with port range 30000-32767",
			},
			{
				Direction:      string(rules.DirIngress),
				EtherType:      string(rules.EtherType6),
				Protocol:       string(rules.ProtocolUDP),
				PortRangeMin:   30000,
				PortRangeMax:   32767,
				RemoteIPPrefix: "::/0",
				Description:    "IPv6: allow all incoming udp traffic with port range 30000-32767",
			},
		}...)
	}

	if modified, err := fctx.access.UpdateSecurityGroupRules(ctx, group, desiredRules, func(_ *rules.SecGroupRule) bool {
		// Do NOT delete unknown rules to keep permissive behaviour as with terraform.
		// As we don't store the role ids in the state, this function needs to be adjusted
		// if values in existing rules are changed to identify them for update by replacement.
		return false
	}); err != nil {
		return err
	} else if modified {
		log.Info("updated rules")
	}
	return nil
}

func (fctx *FlowContext) ensureSSHKeyPair(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	keyPair, err := fctx.compute.GetKeyPair(ctx, fctx.defaultSSHKeypairName())
	if err != nil {
		return err
	}
	if keyPair != nil {
		// if the public keys are matching then return early. In all other cases we should be creating (or replacing) the keypair with a new one.
		if keyPair.PublicKey == string(fctx.infra.Spec.SSHPublicKey) {
			fctx.state.Set(NameKeyPair, keyPair.Name)
			return nil
		}

		log.Info("replacing SSH key pair")
		if err := fctx.compute.DeleteKeyPair(ctx, fctx.defaultSSHKeypairName()); client.IgnoreNotFoundError(err) != nil {
			return err
		}
		keyPair = nil
		fctx.state.Set(NameKeyPair, "")
	}

	log.Info("creating SSH key pair")
	if keyPair, err = fctx.compute.CreateKeyPair(ctx, fctx.defaultSSHKeypairName(), string(fctx.infra.Spec.SSHPublicKey)); err != nil {
		return err
	}
	fctx.state.Set(NameKeyPair, keyPair.Name)
	return nil
}

func (fctx *FlowContext) ensureShareNetwork(ctx context.Context) error {
	if sn := fctx.config.Networks.ShareNetwork; sn == nil || !sn.Enabled {
		return nil
	}

	sharedFilesystemClient, err := fctx.openstackClientFactory.SharedFilesystem(client.WithRegion(fctx.infra.Spec.Region))
	if err != nil {
		return err
	}

	log := shared.LogFromContext(ctx)
	networkID := ptr.Deref(fctx.state.Get(IdentifierNetwork), "")
	subnetID := ptr.Deref(fctx.state.Get(IdentifierSubnet), "")
	current, err := findExisting(ctx, fctx.state.Get(IdentifierShareNetwork),
		fctx.defaultSharedNetworkName(),
		sharedFilesystemClient.GetShareNetwork,
		func(ctx context.Context, name string) ([]*sharenetworks.ShareNetwork, error) {
			list, err := sharedFilesystemClient.ListShareNetworks(ctx, sharenetworks.ListOpts{
				Name:            name,
				NeutronNetID:    networkID,
				NeutronSubnetID: subnetID,
			})
			if err != nil {
				return nil, err
			}
			return sliceToPtr(list), nil
		})

	if err != nil {
		return err
	}

	if current != nil {
		fctx.state.Set(IdentifierShareNetwork, current.ID)
		fctx.state.Set(NameShareNetwork, current.Name)
		return nil
	}

	log.Info("creating...")
	created, err := sharedFilesystemClient.CreateShareNetwork(ctx, sharenetworks.CreateOpts{
		NeutronNetID:    networkID,
		NeutronSubnetID: subnetID,
		Name:            fctx.defaultSharedNetworkName(),
	})
	if err != nil {
		return err
	}
	fctx.state.Set(IdentifierShareNetwork, created.ID)
	fctx.state.Set(NameShareNetwork, created.Name)
	return nil
}

func (fctx *FlowContext) ensureEgressCIDRs(router *access.Router) error {
	var result []string
	for _, efip := range router.ExternalFixedIPs {
		result = append(result, efip.IPAddress)
	}
	fctx.state.SetObject(IdentifierEgressCIDRs, result)
	return nil
}
