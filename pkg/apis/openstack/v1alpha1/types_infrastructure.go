// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	// Deprecated - use `workers` instead.
	Worker string `json:"worker"`
	// Workers is a CIDRs of a worker subnet (private) to create (used for the VMs).
	Workers string `json:"workers"`
	// ID is the ID of an existing private network.
	// +optional
	ID *string `json:"id,omitempty"`
	// ShareNetwork holds information about the share network (used for shared file systems like NFS)
	// +optional
	ShareNetwork *ShareNetwork `json:"shareNetwork,omitempty"`
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
	IP string `json:"ip"`
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
)

// Subnet is an OpenStack subnet related to a Network.
type Subnet struct {
	// Purpose is a logical description of the subnet.
	Purpose Purpose `json:"purpose"`
	// ID is the subnet id.
	ID string `json:"id"`
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
