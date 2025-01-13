// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenv1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	infrainternal "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
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

	status := fctx.computeInfrastructureStatus()
	state := fctx.computeInfrastructureState()
	return infrainternal.PatchProviderStatusAndState(ctx, fctx.client, fctx.infra, status, state)
}

func (fctx *FlowContext) buildReconcileGraph() *flow.Graph {
	g := flow.NewGraph("Openstack infrastructure reconciliation")

	ensureExternalNetwork := fctx.AddTask(g, "ensure external network",
		fctx.ensureExternalNetwork,
		shared.Timeout(defaultTimeout))

	ensureRouter := fctx.AddTask(g, "ensure router",
		fctx.ensureRouter,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureExternalNetwork))

	ensureNetwork := fctx.AddTask(g, "ensure network",
		fctx.ensureNetwork,
		shared.Timeout(defaultTimeout))

	ensureSubnet := fctx.AddTask(g, "ensure subnet",
		fctx.ensureSubnet,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureNetwork))

	_ = fctx.AddTask(g, "ensure router interface",
		fctx.ensureRouterInterface,
		shared.Timeout(defaultTimeout), shared.Dependencies(ensureRouter, ensureSubnet))

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

func (fctx *FlowContext) ensureExternalNetwork(_ context.Context) error {
	externalNetwork, err := fctx.networking.GetExternalNetworkByName(fctx.config.FloatingPoolName)
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

func (fctx *FlowContext) ensureConfiguredRouter(_ context.Context) error {
	router, err := fctx.access.GetRouterByID(fctx.config.Networks.Router.ID)
	if err != nil {
		fctx.state.Set(IdentifierRouter, "")
		fctx.state.Set(RouterIP, "")
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
	fctx.state.Set(RouterIP, router.ExternalFixedIPs[0].IPAddress)
	return nil
}

func (fctx *FlowContext) ensureNewRouter(ctx context.Context, externalNetworkID string) error {
	log := shared.LogFromContext(ctx)

	desired := &access.Router{
		Name:              fctx.defaultRouterName(),
		ExternalNetworkID: externalNetworkID,
		EnableSNAT:        fctx.cloudProfileConfig.UseSNAT,
	}
	current, err := fctx.findExistingRouter()
	if err != nil {
		return err
	}
	if current != nil {
		if len(current.ExternalFixedIPs) < 1 {
			return fmt.Errorf("expected at least one external fixed ip")
		}
		fctx.state.Set(IdentifierRouter, current.ID)
		fctx.state.Set(RouterIP, current.ExternalFixedIPs[0].IPAddress)
		_, err := fctx.access.UpdateRouter(desired, current)
		return err
	}

	floatingPoolSubnetName := fctx.findFloatingPoolSubnetName()
	fctx.state.SetPtr(NameFloatingPoolSubnet, floatingPoolSubnetName)
	if floatingPoolSubnetName != nil {
		log.Info("looking up floating pool subnets...")
		desired.ExternalSubnetIDs, err = fctx.access.LookupFloatingPoolSubnetIDs(externalNetworkID, *floatingPoolSubnetName)
		if err != nil {
			return err
		}
	}
	log.Info("creating...")
	// TODO: add tags to created resources
	created, err := fctx.access.CreateRouter(desired)
	if err != nil {
		return err
	}
	fctx.state.Set(IdentifierRouter, created.ID)
	fctx.state.Set(RouterIP, created.ExternalFixedIPs[0].IPAddress)

	return nil
}

func (fctx *FlowContext) findExistingRouter() (*access.Router, error) {
	return findExisting(fctx.state.Get(IdentifierRouter), fctx.defaultRouterName(), fctx.access.GetRouterByID, fctx.access.GetRouterByName)
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

func (fctx *FlowContext) ensureConfiguredNetwork(_ context.Context) error {
	networkId := *fctx.config.Networks.ID
	network, err := fctx.access.GetNetworkByID(networkId)
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
	current, err := fctx.findExistingNetwork()
	if err != nil {
		return err
	}
	if current != nil {
		fctx.state.Set(IdentifierNetwork, current.ID)
		fctx.state.Set(NameNetwork, current.Name)
		if _, err := fctx.access.UpdateNetwork(desired, current); err != nil {
			return err
		}
	} else {
		log.Info("creating...")
		created, err := fctx.access.CreateNetwork(desired)
		if err != nil {
			return err
		}
		fctx.state.Set(IdentifierNetwork, created.ID)
		fctx.state.Set(NameNetwork, created.Name)
	}

	return nil
}

func (fctx *FlowContext) findExistingNetwork() (*access.Network, error) {
	return findExisting(fctx.state.Get(IdentifierNetwork), fctx.defaultNetworkName(), fctx.access.GetNetworkByID, fctx.access.GetNetworkByName)
}

func (fctx *FlowContext) getNetworkID() (*string, error) {
	if fctx.config.Networks.ID != nil {
		return fctx.config.Networks.ID, nil
	}
	networkID := fctx.state.Get(IdentifierNetwork)
	if networkID != nil {
		return networkID, nil
	}
	network, err := fctx.findExistingNetwork()
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
		CIDR:           fctx.workerCIDR(),
		IPVersion:      4,
		DNSNameservers: fctx.cloudProfileConfig.DNSServers,
	}
	current, err := fctx.findExistingSubnet()
	if err != nil {
		return err
	}
	if current != nil {
		fctx.state.Set(IdentifierSubnet, current.ID)
		log.Info("updating...")
		if _, err := fctx.access.UpdateSubnet(desired, current); err != nil {
			return err
		}
	} else {
		log.Info("creating...")
		created, err := fctx.access.CreateSubnet(desired)
		if err != nil {
			return err
		}
		fctx.state.Set(IdentifierSubnet, created.ID)
	}
	return nil
}

func (fctx *FlowContext) findExistingSubnet() (*subnets.Subnet, error) {
	networkID, err := fctx.getNetworkID()
	if err != nil {
		return nil, err
	}
	if networkID == nil {
		return nil, nil
	}
	getByName := func(name string) ([]*subnets.Subnet, error) {
		return fctx.access.GetSubnetByName(*networkID, name)
	}
	return findExisting(fctx.state.Get(IdentifierSubnet), fctx.defaultSubnetName(), fctx.access.GetSubnetByID, getByName)
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
	portID, err := fctx.access.GetRouterInterfacePortID(*routerID, *subnetID)
	if err != nil {
		return err
	}
	if portID != nil {
		return nil
	}
	log.Info("creating...")
	return fctx.access.AddRouterInterfaceAndWait(ctx, *routerID, *subnetID)
}

func (fctx *FlowContext) ensureSecGroup(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	desired := &groups.SecGroup{
		Name:        fctx.defaultSecurityGroupName(),
		Description: "Cluster Nodes",
	}
	current, err := findExisting(fctx.state.Get(IdentifierSecGroup), fctx.defaultSecurityGroupName(), fctx.access.GetSecurityGroupByID, fctx.access.GetSecurityGroupByName)
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
	created, err := fctx.access.CreateSecurityGroup(desired)
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

	if modified, err := fctx.access.UpdateSecurityGroupRules(group, desiredRules, func(_ *rules.SecGroupRule) bool {
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

	keyPair, err := fctx.compute.GetKeyPair(fctx.defaultSSHKeypairName())
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
		if err := fctx.compute.DeleteKeyPair(fctx.defaultSSHKeypairName()); client.IgnoreNotFoundError(err) != nil {
			return err
		}
		keyPair = nil
		fctx.state.Set(NameKeyPair, "")
	}

	log.Info("creating SSH key pair")
	if keyPair, err = fctx.compute.CreateKeyPair(fctx.defaultSSHKeypairName(), string(fctx.infra.Spec.SSHPublicKey)); err != nil {
		return err
	}
	fctx.state.Set(NameKeyPair, keyPair.Name)
	return nil
}

func (fctx *FlowContext) ensureShareNetwork(ctx context.Context) error {
	if sn := fctx.config.Networks.ShareNetwork; sn == nil || !sn.Enabled {
		return nil
	}

	log := shared.LogFromContext(ctx)
	networkID := ptr.Deref(fctx.state.Get(IdentifierNetwork), "")
	subnetID := ptr.Deref(fctx.state.Get(IdentifierSubnet), "")
	current, err := findExisting(fctx.state.Get(IdentifierShareNetwork),
		fctx.defaultSharedNetworkName(),
		fctx.sharedFilesystem.GetShareNetwork,
		func(name string) ([]*sharenetworks.ShareNetwork, error) {
			list, err := fctx.sharedFilesystem.ListShareNetworks(sharenetworks.ListOpts{
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
	created, err := fctx.sharedFilesystem.CreateShareNetwork(sharenetworks.CreateOpts{
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
