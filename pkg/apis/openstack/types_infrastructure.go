// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	// Deprecated - use `workers` instead.
	Worker string
	// Workers is a CIDRs of a worker subnet (private) to create (used for the VMs).
	Workers string
	// ID is the ID of an existing private network.
	ID *string
	// ShareNetwork holds information about the share network (used for shared file systems like NFS)
	ShareNetwork *ShareNetwork
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
	IP string
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
)

// Subnet is an OpenStack subnet related to a Network.
type Subnet struct {
	// Purpose is a logical description of the subnet.
	Purpose Purpose
	// ID is the subnet id.
	ID string
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
