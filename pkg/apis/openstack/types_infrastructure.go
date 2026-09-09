// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta
	// FloatingPoolName contains the FloatingPoolName name in which LoadBalancer FIPs should be created.
	FloatingPoolName string
	// FloatingPoolSubnetName contains the fixed name of subnet or matching name pattern for subnet
	// in the Floating IP Pool where the router should be attached to.
	FloatingPoolSubnetName *string
	// Networks is the OpenStack specific network configuration
	Networks Networks
}

// Networks holds information about the Kubernetes and infrastructure networks.
type Networks struct {
	// Router indicates whether to use an existing router or create a new one.
	Router *Router
	// Worker is a CIDRs of a worker subnet (private) to create (used for the VMs).
	// Deprecated: use `workers` instead.
	// +optional
	Worker string
	// Workers is a CIDRs of a worker subnet (private) to create (used for the VMs).
	// Mutually exclusive with SubnetPool.
	// +optional
	Workers string
	// SubnetPool specifies an OpenStack subnet pool to use for automatic CIDR allocation
	// for the worker subnet. Mutually exclusive with Workers/Worker CIDR fields.
	// +optional
	SubnetPool *SubnetPool
	// ID is the ID of an existing private network.
	ID *string
	// SubnetID is the ID of an existing subnet. If provided, the workers will be deployed into this subnet
	// instead of a new subnet being created. Requires networks.id to be set as well.
	// +optional
	SubnetID *string
	// SecurityGroupID is the ID of an existing security group to use for worker nodes.
	// When set, Gardener will not create a security group and will use this one instead.
	// Requires networks.id to be set. The security group must allow node-to-node traffic,
	// TCP/UDP on ports 30000-32767, and all egress.
	// +optional
	SecurityGroupID *string
	// ShareNetworkID is the ID of an existing Manila share network to use for NFS volumes.
	// When set, Gardener will use it instead of creating one and will not delete it on shoot teardown.
	// Requires networks.id to be set. Mutually exclusive with shareNetwork.enabled.
	// +optional
	ShareNetworkID *string
	// ShareNetwork holds information about the share network (used for shared file systems like NFS)
	ShareNetwork *ShareNetwork
	// IPv6 holds information about the IPv6 CIDRs.
	// +optional
	IPv6 *IPv6Config
}

// SubnetPool specifies an OpenStack subnet pool from which a CIDR will be automatically allocated.
type SubnetPool struct {
	// ID is the ID of the OpenStack subnet pool.
	ID string
	// PrefixLength is the prefix length (e.g. 24 for a /24 subnet) to request from the pool.
	PrefixLength int
}

// IPv6Config contains the IPv6 CIDR configuration for nodes, pods, and services.
type IPv6Config struct {
	// SubnetPoolID is the ID of the subnet pool to use for IPv6 subnet allocation.
	// Mutually exclusive with explicit CIDR fields (NodeCIDR, PodCIDR, ServiceCIDR) and NodeSubnetID.
	SubnetPoolID *string
	// NodeCIDR is the CIDR of the IPv6 node subnet to create.
	// Required when neither SubnetPoolID nor NodeSubnetID is set.
	// Mutually exclusive with NodeSubnetID.
	// +optional
	NodeCIDR string
	// PodCIDR is the IPv6 CIDR for pods.
	// Required when NodeCIDR is set or NodeSubnetID is set.
	// +optional
	PodCIDR string
	// ServiceCIDR is the IPv6 CIDR for services.
	// Required when NodeCIDR is set or NodeSubnetID is set.
	// +optional
	ServiceCIDR string
	// NodeSubnetID is the ID of an existing IPv6 subnet for worker nodes.
	// When set, Gardener will not create an IPv6 node subnet.
	// Requires networks.id and networks.router.id. Mutually exclusive with SubnetPoolID and NodeCIDR.
	// PodCIDR and ServiceCIDR must be set explicitly when NodeSubnetID is used.
	// +optional
	NodeSubnetID *string
}

// Router indicates whether to use an existing router or create a new one.
type Router struct {
	// ID is the router id of an existing OpenStack router.
	ID string
}

// ShareNetwork holds information about the share network (used for shared file systems like NFS)
type ShareNetwork struct {
	// Enabled is the switch to enable the creation of a share network
	Enabled bool
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta
	// Networks contains information about the created Networks and some related resources.
	Networks NetworkStatus
	// Node contains information about Node related resources.
	Node NodeStatus
	// SecurityGroups is a list of security groups that have been created.
	SecurityGroups []SecurityGroup
}

// NodeStatus contains information about Node related resources.
type NodeStatus struct {
	// KeyName is the name of the SSH key.
	KeyName string
}

// NetworkStatus contains information about a generated Network or resources created in an existing Network.
type NetworkStatus struct {
	// ID is the Network id.
	ID string
	// Name is the Network name.
	Name string
	// FloatingPool contains information about the floating pool.
	FloatingPool FloatingPoolStatus
	// Router contains information about the Router and related resources.
	Router RouterStatus
	// Subnets is a list of subnets that have been created.
	Subnets []Subnet
	// ShareNetwork contains information about a created/provided ShareNetwork
	ShareNetwork *ShareNetworkStatus
}

// RouterStatus contains information about a generated Router or resources attached to an existing Router.
type RouterStatus struct {
	// ID is the Router id.
	ID string
	// IP is the router ip.
	// Deprecated: use ExternalFixedIPs instead.
	IP string
	// ExternalFixedIPs is the list of the router's assigned external fixed IPs.
	ExternalFixedIPs []string
}

// FloatingPoolStatus contains information about the floating pool.
type FloatingPoolStatus struct {
	// ID is the floating pool id.
	ID string
	// Name is the floating pool name.
	Name string
}

// ShareNetworkStatus contains information about a generated ShareNetwork
type ShareNetworkStatus struct {
	// ID is the Network id.
	ID string
	// Name is the Network name.
	Name string
}

// Purpose is a purpose of resources.
type Purpose string

const (
	// PurposeNodes is a Purpose for node resources.
	PurposeNodes Purpose = "nodes"
	// PurposeNodesIPv6 is a Purpose for IPv6 node subnet resources in dual-stack clusters.
	PurposeNodesIPv6 Purpose = "nodes-ipv6"
	// PurposePods is a Purpose for pod CIDR allocation resources.
	PurposePods Purpose = "pods"
	// PurposeServices is a Purpose for service CIDR allocation resources.
	PurposeServices Purpose = "services"
)

// Subnet is an OpenStack subnet related to a Network.
type Subnet struct {
	// Purpose is a logical description of the subnet.
	Purpose Purpose
	// ID is the subnet id.
	ID string
	// CIDR is the CIDR of the subnet.
	CIDR string
}

// SecurityGroup is an OpenStack security group related to a Network.
type SecurityGroup struct {
	// Purpose is a logical description of the security group.
	Purpose Purpose
	// ID is the security group id.
	ID string
	// Name is the security group name.
	Name string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureState is the state which is persisted as part of the infrastructure status.
type InfrastructureState struct {
	metav1.TypeMeta

	Data map[string]string
}

// WorkersCIDR returns the CIDR for worker nodes.
func (cfg InfrastructureConfig) WorkersCIDR() string {
	s := cfg.Networks.Worker
	if workers := cfg.Networks.Workers; workers != "" {
		s = workers
	}
	return s
}
