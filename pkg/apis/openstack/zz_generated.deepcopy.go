// +build !ignore_autogenerated

/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by deepcopy-gen. DO NOT EDIT.

package openstack

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudControllerManagerConfig) DeepCopyInto(out *CloudControllerManagerConfig) {
	*out = *in
	if in.FeatureGates != nil {
		in, out := &in.FeatureGates, &out.FeatureGates
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudControllerManagerConfig.
func (in *CloudControllerManagerConfig) DeepCopy() *CloudControllerManagerConfig {
	if in == nil {
		return nil
	}
	out := new(CloudControllerManagerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudProfileConfig) DeepCopyInto(out *CloudProfileConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Constraints.DeepCopyInto(&out.Constraints)
	if in.DNSServers != nil {
		in, out := &in.DNSServers, &out.DNSServers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DHCPDomain != nil {
		in, out := &in.DHCPDomain, &out.DHCPDomain
		*out = new(string)
		**out = **in
	}
	if in.KeyStoneURLs != nil {
		in, out := &in.KeyStoneURLs, &out.KeyStoneURLs
		*out = make([]KeyStoneURL, len(*in))
		copy(*out, *in)
	}
	if in.MachineImages != nil {
		in, out := &in.MachineImages, &out.MachineImages
		*out = make([]MachineImages, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.RequestTimeout != nil {
		in, out := &in.RequestTimeout, &out.RequestTimeout
		*out = new(v1.Duration)
		**out = **in
	}
	if in.RescanBlockStorageOnResize != nil {
		in, out := &in.RescanBlockStorageOnResize, &out.RescanBlockStorageOnResize
		*out = new(bool)
		**out = **in
	}
	if in.IgnoreVolumeAZ != nil {
		in, out := &in.IgnoreVolumeAZ, &out.IgnoreVolumeAZ
		*out = new(bool)
		**out = **in
	}
	if in.NodeVolumeAttachLimit != nil {
		in, out := &in.NodeVolumeAttachLimit, &out.NodeVolumeAttachLimit
		*out = new(int32)
		**out = **in
	}
	if in.UseOctavia != nil {
		in, out := &in.UseOctavia, &out.UseOctavia
		*out = new(bool)
		**out = **in
	}
	if in.UseSNAT != nil {
		in, out := &in.UseSNAT, &out.UseSNAT
		*out = new(bool)
		**out = **in
	}
	if in.ServerGroupPolicies != nil {
		in, out := &in.ServerGroupPolicies, &out.ServerGroupPolicies
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ResolvConfOptions != nil {
		in, out := &in.ResolvConfOptions, &out.ResolvConfOptions
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudProfileConfig.
func (in *CloudProfileConfig) DeepCopy() *CloudProfileConfig {
	if in == nil {
		return nil
	}
	out := new(CloudProfileConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CloudProfileConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Constraints) DeepCopyInto(out *Constraints) {
	*out = *in
	if in.FloatingPools != nil {
		in, out := &in.FloatingPools, &out.FloatingPools
		*out = make([]FloatingPool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LoadBalancerProviders != nil {
		in, out := &in.LoadBalancerProviders, &out.LoadBalancerProviders
		*out = make([]LoadBalancerProvider, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Constraints.
func (in *Constraints) DeepCopy() *Constraints {
	if in == nil {
		return nil
	}
	out := new(Constraints)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneConfig) DeepCopyInto(out *ControlPlaneConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.CloudControllerManager != nil {
		in, out := &in.CloudControllerManager, &out.CloudControllerManager
		*out = new(CloudControllerManagerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.LoadBalancerClasses != nil {
		in, out := &in.LoadBalancerClasses, &out.LoadBalancerClasses
		*out = make([]LoadBalancerClass, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Zone != nil {
		in, out := &in.Zone, &out.Zone
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneConfig.
func (in *ControlPlaneConfig) DeepCopy() *ControlPlaneConfig {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ControlPlaneConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingPool) DeepCopyInto(out *FloatingPool) {
	*out = *in
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.Domain != nil {
		in, out := &in.Domain, &out.Domain
		*out = new(string)
		**out = **in
	}
	if in.DefaultFloatingSubnet != nil {
		in, out := &in.DefaultFloatingSubnet, &out.DefaultFloatingSubnet
		*out = new(string)
		**out = **in
	}
	if in.NonConstraining != nil {
		in, out := &in.NonConstraining, &out.NonConstraining
		*out = new(bool)
		**out = **in
	}
	if in.LoadBalancerClasses != nil {
		in, out := &in.LoadBalancerClasses, &out.LoadBalancerClasses
		*out = make([]LoadBalancerClass, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingPool.
func (in *FloatingPool) DeepCopy() *FloatingPool {
	if in == nil {
		return nil
	}
	out := new(FloatingPool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingPoolStatus) DeepCopyInto(out *FloatingPoolStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingPoolStatus.
func (in *FloatingPoolStatus) DeepCopy() *FloatingPoolStatus {
	if in == nil {
		return nil
	}
	out := new(FloatingPoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfrastructureConfig) DeepCopyInto(out *InfrastructureConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.FloatingPoolSubnetName != nil {
		in, out := &in.FloatingPoolSubnetName, &out.FloatingPoolSubnetName
		*out = new(string)
		**out = **in
	}
	in.Networks.DeepCopyInto(&out.Networks)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfrastructureConfig.
func (in *InfrastructureConfig) DeepCopy() *InfrastructureConfig {
	if in == nil {
		return nil
	}
	out := new(InfrastructureConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfrastructureConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfrastructureStatus) DeepCopyInto(out *InfrastructureStatus) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.Networks.DeepCopyInto(&out.Networks)
	out.Node = in.Node
	if in.SecurityGroups != nil {
		in, out := &in.SecurityGroups, &out.SecurityGroups
		*out = make([]SecurityGroup, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfrastructureStatus.
func (in *InfrastructureStatus) DeepCopy() *InfrastructureStatus {
	if in == nil {
		return nil
	}
	out := new(InfrastructureStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfrastructureStatus) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KeyStoneURL) DeepCopyInto(out *KeyStoneURL) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KeyStoneURL.
func (in *KeyStoneURL) DeepCopy() *KeyStoneURL {
	if in == nil {
		return nil
	}
	out := new(KeyStoneURL)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoadBalancerClass) DeepCopyInto(out *LoadBalancerClass) {
	*out = *in
	if in.Purpose != nil {
		in, out := &in.Purpose, &out.Purpose
		*out = new(string)
		**out = **in
	}
	if in.FloatingSubnetID != nil {
		in, out := &in.FloatingSubnetID, &out.FloatingSubnetID
		*out = new(string)
		**out = **in
	}
	if in.FloatingSubnetTags != nil {
		in, out := &in.FloatingSubnetTags, &out.FloatingSubnetTags
		*out = new(string)
		**out = **in
	}
	if in.FloatingSubnetName != nil {
		in, out := &in.FloatingSubnetName, &out.FloatingSubnetName
		*out = new(string)
		**out = **in
	}
	if in.FloatingNetworkID != nil {
		in, out := &in.FloatingNetworkID, &out.FloatingNetworkID
		*out = new(string)
		**out = **in
	}
	if in.SubnetID != nil {
		in, out := &in.SubnetID, &out.SubnetID
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoadBalancerClass.
func (in *LoadBalancerClass) DeepCopy() *LoadBalancerClass {
	if in == nil {
		return nil
	}
	out := new(LoadBalancerClass)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoadBalancerProvider) DeepCopyInto(out *LoadBalancerProvider) {
	*out = *in
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoadBalancerProvider.
func (in *LoadBalancerProvider) DeepCopy() *LoadBalancerProvider {
	if in == nil {
		return nil
	}
	out := new(LoadBalancerProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineImage) DeepCopyInto(out *MachineImage) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineImage.
func (in *MachineImage) DeepCopy() *MachineImage {
	if in == nil {
		return nil
	}
	out := new(MachineImage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineImageVersion) DeepCopyInto(out *MachineImageVersion) {
	*out = *in
	if in.Regions != nil {
		in, out := &in.Regions, &out.Regions
		*out = make([]RegionIDMapping, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineImageVersion.
func (in *MachineImageVersion) DeepCopy() *MachineImageVersion {
	if in == nil {
		return nil
	}
	out := new(MachineImageVersion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineImages) DeepCopyInto(out *MachineImages) {
	*out = *in
	if in.Versions != nil {
		in, out := &in.Versions, &out.Versions
		*out = make([]MachineImageVersion, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineImages.
func (in *MachineImages) DeepCopy() *MachineImages {
	if in == nil {
		return nil
	}
	out := new(MachineImages)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkStatus) DeepCopyInto(out *NetworkStatus) {
	*out = *in
	out.FloatingPool = in.FloatingPool
	out.Router = in.Router
	if in.Subnets != nil {
		in, out := &in.Subnets, &out.Subnets
		*out = make([]Subnet, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkStatus.
func (in *NetworkStatus) DeepCopy() *NetworkStatus {
	if in == nil {
		return nil
	}
	out := new(NetworkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Networks) DeepCopyInto(out *Networks) {
	*out = *in
	if in.Router != nil {
		in, out := &in.Router, &out.Router
		*out = new(Router)
		**out = **in
	}
	if in.ID != nil {
		in, out := &in.ID, &out.ID
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Networks.
func (in *Networks) DeepCopy() *Networks {
	if in == nil {
		return nil
	}
	out := new(Networks)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeStatus) DeepCopyInto(out *NodeStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeStatus.
func (in *NodeStatus) DeepCopy() *NodeStatus {
	if in == nil {
		return nil
	}
	out := new(NodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RegionIDMapping) DeepCopyInto(out *RegionIDMapping) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RegionIDMapping.
func (in *RegionIDMapping) DeepCopy() *RegionIDMapping {
	if in == nil {
		return nil
	}
	out := new(RegionIDMapping)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Router) DeepCopyInto(out *Router) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Router.
func (in *Router) DeepCopy() *Router {
	if in == nil {
		return nil
	}
	out := new(Router)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RouterStatus) DeepCopyInto(out *RouterStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RouterStatus.
func (in *RouterStatus) DeepCopy() *RouterStatus {
	if in == nil {
		return nil
	}
	out := new(RouterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecurityGroup) DeepCopyInto(out *SecurityGroup) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecurityGroup.
func (in *SecurityGroup) DeepCopy() *SecurityGroup {
	if in == nil {
		return nil
	}
	out := new(SecurityGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServerGroup) DeepCopyInto(out *ServerGroup) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServerGroup.
func (in *ServerGroup) DeepCopy() *ServerGroup {
	if in == nil {
		return nil
	}
	out := new(ServerGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServerGroupDependency) DeepCopyInto(out *ServerGroupDependency) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServerGroupDependency.
func (in *ServerGroupDependency) DeepCopy() *ServerGroupDependency {
	if in == nil {
		return nil
	}
	out := new(ServerGroupDependency)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Subnet) DeepCopyInto(out *Subnet) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Subnet.
func (in *Subnet) DeepCopy() *Subnet {
	if in == nil {
		return nil
	}
	out := new(Subnet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerConfig) DeepCopyInto(out *WorkerConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.ServerGroup != nil {
		in, out := &in.ServerGroup, &out.ServerGroup
		*out = new(ServerGroup)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerConfig.
func (in *WorkerConfig) DeepCopy() *WorkerConfig {
	if in == nil {
		return nil
	}
	out := new(WorkerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkerStatus) DeepCopyInto(out *WorkerStatus) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	if in.MachineImages != nil {
		in, out := &in.MachineImages, &out.MachineImages
		*out = make([]MachineImage, len(*in))
		copy(*out, *in)
	}
	if in.ServerGroupDependencies != nil {
		in, out := &in.ServerGroupDependencies, &out.ServerGroupDependencies
		*out = make([]ServerGroupDependency, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkerStatus.
func (in *WorkerStatus) DeepCopy() *WorkerStatus {
	if in == nil {
		return nil
	}
	out := new(WorkerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkerStatus) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
