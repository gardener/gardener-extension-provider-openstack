// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta `json:",inline"`
	// FloatingPoolName contains the FloatingPoolName name in which LoadBalancer FIPs should be created.
	FloatingPoolName string `json:"floatingPoolName"`
	// FloatingPoolSubnetName contains the fixed name of subnet or matching name pattern for subnet
	// in the Floating IP Pool where the router should be attached to.
	// +optional
	FloatingPoolSubnetName *string `json:"floatingPoolSubnetName,omitempty"`
	// Networks is the OpenStack specific network configuration
	Networks Networks `json:"networks"`
}

// Networks holds information about the Kubernetes and infrastructure networks.
type Networks struct {
	// Router indicates whether to use an existing router or create a new one.
	// +optional
	Router *Router `json:"router,omitempty"`
	// Worker is a CIDRs of a worker subnet (private) to create (used for the VMs).
	// Deprecated: use `workers` instead.
	// +optional
	Worker string `json:"worker,omitempty"`
	// Workers is a CIDRs of a worker subnet (private) to create (used for the VMs).
	// Mutually exclusive with SubnetPool.
	// +optional
	Workers string `json:"workers,omitempty"`
	// SubnetPool specifies an OpenStack subnet pool to use for automatic CIDR allocation
	// for the worker subnet. Mutually exclusive with Workers/Worker CIDR fields.
	// +optional
	SubnetPool *SubnetPool `json:"subnetPool,omitempty"`
	// ID is the ID of an existing private network.
	// +optional
	ID *string `json:"id,omitempty"`
	// SubnetID is the ID of an existing subnet. If provided, the workers will be deployed into this subnet
	// instead of a new subnet being created. Requires networks.id to be set as well.
	// +optional
	SubnetID *string `json:"subnetId,omitempty"`
	// SecurityGroupID is the ID of an existing security group to use for worker nodes.
	// When set, Gardener will not create a security group and will use this one instead.
	// Requires networks.id to be set. The security group must allow node-to-node traffic,
	// TCP/UDP on ports 30000-32767, and all egress.
	// +optional
	SecurityGroupID *string `json:"securityGroupId,omitempty"`
	// ShareNetworkID is the ID of an existing Manila share network to use for NFS volumes.
	// When set, Gardener will use it instead of creating one and will not delete it on shoot teardown.
	// Requires networks.id to be set. Mutually exclusive with shareNetwork.enabled.
	// +optional
	ShareNetworkID *string `json:"shareNetworkId,omitempty"`
	// ShareNetwork holds information about the share network (used for shared file systems like NFS)
	// +optional
	ShareNetwork *ShareNetwork `json:"shareNetwork,omitempty"`
	// IPv6 holds information about the IPv6 CIDRs.
	// +optional
	IPv6 *IPv6Config `json:"ipv6,omitempty"`
}

// SubnetPool specifies an OpenStack subnet pool from which a CIDR will be automatically allocated.
type SubnetPool struct {
	// ID is the ID of the OpenStack subnet pool.
	ID string `json:"id"`
	// PrefixLength is the prefix length (e.g. 24 for a /24 subnet) to request from the pool.
	PrefixLength int `json:"prefixLength"`
}

// IPv6Config contains the IPv6 CIDR configuration for nodes, pods, and services.
type IPv6Config struct {
	// SubnetPoolID is the ID of the subnet pool to use for IPv6 subnet allocation.
	// Mutually exclusive with explicit CIDR fields (NodeCIDR, PodCIDR, ServiceCIDR) and NodeSubnetID.
	// +optional
	SubnetPoolID *string `json:"subnetPoolID,omitempty"`
	// NodeCIDR is the CIDR of the IPv6 node subnet to create.
	// Required when neither subnetPoolID nor nodeSubnetId is set.
	// Mutually exclusive with nodeSubnetId.
	// +optional
	NodeCIDR string `json:"nodeCIDR,omitempty"`
	// PodCIDR is the IPv6 CIDR for pods.
	// Required when nodeCIDR is set or nodeSubnetId is set.
	// +optional
	PodCIDR string `json:"podCIDR,omitempty"`
	// ServiceCIDR is the IPv6 CIDR for services.
	// Required when nodeCIDR is set or nodeSubnetId is set.
	// +optional
	ServiceCIDR string `json:"serviceCIDR,omitempty"`
	// NodeSubnetID is the ID of an existing IPv6 subnet for worker nodes.
	// When set, Gardener will not create an IPv6 node subnet.
	// Requires networks.id and networks.router.id. Mutually exclusive with subnetPoolID and nodeCIDR.
	// podCIDR and serviceCIDR must be set explicitly when nodeSubnetId is used.
	// +optional
	NodeSubnetID *string `json:"nodeSubnetId,omitempty"`
}

// Router indicates whether to use an existing router or create a new one.
type Router struct {
	// ID is the router id of an existing OpenStack router.
	ID string `json:"id"`
}

// ShareNetwork holds information about the share network (used for shared file systems like NFS)
type ShareNetwork struct {
	// Enabled is the switch to enable the creation of a share network
	Enabled bool `json:"enabled"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta `json:",inline"`
	// Networks contains information about the created Networks and some related resources.
	Networks NetworkStatus `json:"networks"`
	// Node contains information about Node related resources.
	Node NodeStatus `json:"node"`
	// SecurityGroups is a list of security groups that have been created.
	SecurityGroups []SecurityGroup `json:"securityGroups"`
}

// NodeStatus contains information about Node related resources.
type NodeStatus struct {
	// KeyName is the name of the SSH key.
	KeyName string `json:"keyName"`
}

// NetworkStatus contains information about a generated Network or resources created in an existing Network.
type NetworkStatus struct {
	// ID is the Network id.
	ID string `json:"id"`
	// Name is the Network name.
	Name string `json:"name"`
	// FloatingPool contains information about the floating pool.
	FloatingPool FloatingPoolStatus `json:"floatingPool"`
	// Router contains information about the Router and related resources.
	Router RouterStatus `json:"router"`
	// Subnets is a list of subnets that have been created.
	Subnets []Subnet `json:"subnets"`
	// ShareNetwork contains information about a created/provided ShareNetwork
	// +optional
	ShareNetwork *ShareNetworkStatus `json:"shareNetwork,omitempty"`
}

// RouterStatus contains information about a generated Router or resources attached to an existing Router.
type RouterStatus struct {
	// ID is the Router id.
	ID string `json:"id"`
	// IP is the router ip.
	// Deprecated: use ExternalFixedIPs instead.
	IP string `json:"ip"`
	// ExternalFixedIPs is the list of the router's assigned external fixed IPs.
	ExternalFixedIPs []string `json:"externalFixedIP"`
}

// FloatingPoolStatus contains information about the floating pool.
type FloatingPoolStatus struct {
	// ID is the floating pool id.
	ID string `json:"id"`
	// Name is the floating pool name.
	Name string `json:"name"`
}

// ShareNetworkStatus contains information about a generated ShareNetwork
type ShareNetworkStatus struct {
	// ID is the Network id.
	ID string `json:"id"`
	// Name is the Network name.
	Name string `json:"name"`
}

// Purpose is a purpose of a resource.
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
	Purpose Purpose `json:"purpose"`
	// ID is the subnet id.
	ID string `json:"id"`
	// CIDR is the CIDR of the subnet.
	CIDR string `json:"cidr"`
}

// SecurityGroup is an OpenStack security group related to a Network.
type SecurityGroup struct {
	// Purpose is a logical description of the security group.
	Purpose Purpose `json:"purpose"`
	// ID is the security group id.
	ID string `json:"id"`
	// Name is the security group name.
	Name string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureState is the state which is persisted as part of the infrastructure status.
type InfrastructureState struct {
	metav1.TypeMeta

	Data map[string]string `json:"data"`
}
