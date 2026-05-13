<p>Packages:</p>
<ul>
<li>
<a href="#openstack.provider.extensions.gardener.cloud%2fv1alpha1">openstack.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="openstack.provider.extensions.gardener.cloud/v1alpha1">openstack.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>

</p>

<h3 id="csimanila">CSIManila
</h3>


<p>
(<em>Appears on:</em><a href="#storage">Storage</a>)
</p>

<p>
CSIManila contains configuration for CSI Manila driver (support for NFS volumes)
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled is the switch to enable the CSI Manila driver support</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudcontrollermanagerconfig">CloudControllerManagerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>)
</p>

<p>
CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofileconfig">CloudProfileConfig
</h3>


<p>
CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
resource.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#constraints">Constraints</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the control plane config.</p>
</td>
</tr>
<tr>
<td>
<code>dnsServers</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSServers is a list of IPs of DNS servers used while creating subnets.</p>
</td>
</tr>
<tr>
<td>
<code>dhcpDomain</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for<br />Kubernetes 1.10.1+. See https://github.com/kubernetes/kubernetes/pull/61890 for details.</p>
</td>
</tr>
<tr>
<td>
<code>keystoneURL</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeyStoneURL is the URL for auth\{n,z\} in OpenStack (pointing to KeyStone).</p>
</td>
</tr>
<tr>
<td>
<code>keystoneCACert</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeystoneCACert is the CA Bundle for the KeyStoneURL.</p>
</td>
</tr>
<tr>
<td>
<code>keystoneForceInsecure</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeyStoneForceInsecure is a flag to control whether the OpenStack client should perform no certificate validation.</p>
</td>
</tr>
<tr>
<td>
<code>keystoneURLs</code></br>
<em>
<a href="#keystoneurl">KeyStoneURL</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeyStoneURLs is a region-URL mapping for auth\{n,z\} in OpenStack (pointing to KeyStone).</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#machineimages">MachineImages</a> array
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps<br />logical names and versions to provider-specific identifiers.</p>
</td>
</tr>
<tr>
<td>
<code>requestTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RequestTimeout specifies the HTTP timeout against the OpenStack API.</p>
</td>
</tr>
<tr>
<td>
<code>rescanBlockStorageOnResize</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>RescanBlockStorageOnResize specifies whether the storage plugin scans and checks new block device size before it resizes<br />the filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>ignoreVolumeAZ</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>IgnoreVolumeAZ specifies whether the volumes AZ should be ignored when scheduling to nodes,<br />to allow for differences between volume and compute zone naming.</p>
</td>
</tr>
<tr>
<td>
<code>nodeVolumeAttachLimit</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeVolumeAttachLimit specifies how many volumes can be attached to a node.</p>
</td>
</tr>
<tr>
<td>
<code>useSNAT</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>UseSNAT specifies whether S-NAT is supposed to be used for the Gardener managed OpenStack router.</p>
</td>
</tr>
<tr>
<td>
<code>serverGroupPolicies</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServerGroupPolicies specify the allowed server group policies for worker groups.</p>
</td>
</tr>
<tr>
<td>
<code>resolvConfOptions</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResolvConfOptions specifies options to be added to /etc/resolv.conf on workers</p>
</td>
</tr>
<tr>
<td>
<code>storageClasses</code></br>
<em>
<a href="#storageclassdefinition">StorageClassDefinition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>StorageClasses defines storageclasses for the shoot</p>
</td>
</tr>

</tbody>
</table>


<h3 id="constraints">Constraints
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
Constraints is an object containing constraints for the shoots.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>floatingPools</code></br>
<em>
<a href="#floatingpool">FloatingPool</a> array
</em>
</td>
<td>
<p>FloatingPools contains constraints regarding allowed values of the 'floatingPoolName' block in the control plane config.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerProviders</code></br>
<em>
<a href="#loadbalancerprovider">LoadBalancerProvider</a> array
</em>
</td>
<td>
<p>LoadBalancerProviders contains constraints regarding allowed values of the 'loadBalancerProvider' block in the control plane config.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplaneconfig">ControlPlaneConfig
</h3>


<p>
ControlPlaneConfig contains configuration settings for the control plane.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>cloudControllerManager</code></br>
<em>
<a href="#cloudcontrollermanagerconfig">CloudControllerManagerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudControllerManager contains configuration settings for the cloud-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerClasses</code></br>
<em>
<a href="#loadbalancerclass">LoadBalancerClass</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerClasses available for a dedicated Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerProvider</code></br>
<em>
string
</em>
</td>
<td>
<p>LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone is the OpenStack zone.<br />Deprecated: Don't use anymore. Will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#storage">Storage</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage contains configuration for storage in the cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="floatingpool">FloatingPool
</h3>


<p>
(<em>Appears on:</em><a href="#constraints">Constraints</a>)
</p>

<p>
FloatingPool contains constraints regarding allowed values of the 'floatingPoolName' block in the control plane config.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the floating pool.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Region is the region name.</p>
</td>
</tr>
<tr>
<td>
<code>domain</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Domain is the domain name.</p>
</td>
</tr>
<tr>
<td>
<code>defaultFloatingSubnet</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultFloatingSubnet is the default floating subnet for the floating pool.</p>
</td>
</tr>
<tr>
<td>
<code>nonConstraining</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>NonConstraining specifies whether this floating pool is not constraining, that means additionally available independent of other FP constraints.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerClasses</code></br>
<em>
<a href="#loadbalancerclass">LoadBalancerClass</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerClasses contains a list of supported labeled load balancer network settings.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="floatingpoolstatus">FloatingPoolStatus
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
FloatingPoolStatus contains information about the floating pool.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the floating pool id.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the floating pool name.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ipv6config">IPv6Config
</h3>


<p>
(<em>Appears on:</em><a href="#networks">Networks</a>)
</p>

<p>
IPv6Config contains the IPv6 CIDR configuration for nodes, pods, and services.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>subnetPoolID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubnetPoolID is the ID of the subnet pool to use for IPv6 subnet allocation.<br />Mutually exclusive with explicit CIDR fields (NodeCIDR, PodCIDR, ServiceCIDR).</p>
</td>
</tr>
<tr>
<td>
<code>nodeCIDR</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeCIDR is the CIDR of the node subnet.</p>
</td>
</tr>
<tr>
<td>
<code>podCIDR</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodCIDR is the CIDR of the pods.</p>
</td>
</tr>
<tr>
<td>
<code>serviceCIDR</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceCIDR is the CIDR of the services.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructureconfig">InfrastructureConfig
</h3>


<p>
InfrastructureConfig infrastructure configuration resource
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>floatingPoolName</code></br>
<em>
string
</em>
</td>
<td>
<p>FloatingPoolName contains the FloatingPoolName name in which LoadBalancer FIPs should be created.</p>
</td>
</tr>
<tr>
<td>
<code>floatingPoolSubnetName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingPoolSubnetName contains the fixed name of subnet or matching name pattern for subnet<br />in the Floating IP Pool where the router should be attached to.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#networks">Networks</a>
</em>
</td>
<td>
<p>Networks is the OpenStack specific network configuration</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestate">InfrastructureState
</h3>


<p>
InfrastructureState is the state which is persisted as part of the infrastructure status.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>data</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestatus">InfrastructureStatus
</h3>


<p>
InfrastructureStatus contains information about created infrastructure resources.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>networks</code></br>
<em>
<a href="#networkstatus">NetworkStatus</a>
</em>
</td>
<td>
<p>Networks contains information about the created Networks and some related resources.</p>
</td>
</tr>
<tr>
<td>
<code>node</code></br>
<em>
<a href="#nodestatus">NodeStatus</a>
</em>
</td>
<td>
<p>Node contains information about Node related resources.</p>
</td>
</tr>
<tr>
<td>
<code>securityGroups</code></br>
<em>
<a href="#securitygroup">SecurityGroup</a> array
</em>
</td>
<td>
<p>SecurityGroups is a list of security groups that have been created.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="keystoneurl">KeyStoneURL
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
KeyStoneURL is a region-URL mapping for auth{n,z} in OpenStack (pointing to KeyStone).
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the name of the region.</p>
</td>
</tr>
<tr>
<td>
<code>url</code></br>
<em>
string
</em>
</td>
<td>
<p>URL is the keystone URL.</p>
</td>
</tr>
<tr>
<td>
<code>caCert</code></br>
<em>
string
</em>
</td>
<td>
<p>CACert is the CA Bundle for the KeyStoneURL.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="loadbalancerclass">LoadBalancerClass
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>, <a href="#floatingpool">FloatingPool</a>)
</p>

<p>
LoadBalancerClass defines a restricted network setting for generic LoadBalancer classes.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the LB class</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Purpose is reflecting if the loadbalancer class has a special purpose e.g. default, internal.</p>
</td>
</tr>
<tr>
<td>
<code>floatingSubnetID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingSubnetID is the subnetwork ID of a dedicated subnet in floating network pool.</p>
</td>
</tr>
<tr>
<td>
<code>floatingSubnetTags</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingSubnetTags is a list of tags which can be used to select subnets in the floating network pool.</p>
</td>
</tr>
<tr>
<td>
<code>floatingSubnetName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingSubnetName is can either be a name or a name pattern of a subnet in the floating network pool.</p>
</td>
</tr>
<tr>
<td>
<code>floatingNetworkID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingNetworkID is the network ID of the floating network pool.</p>
</td>
</tr>
<tr>
<td>
<code>subnetID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubnetID is the ID of a local subnet used for LoadBalancer provisioning. Only usable if no FloatingPool<br />configuration is done.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="loadbalancerprovider">LoadBalancerProvider
</h3>


<p>
(<em>Appears on:</em><a href="#constraints">Constraints</a>)
</p>

<p>
LoadBalancerProvider contains constraints regarding allowed values of the 'loadBalancerProvider' block in the control plane config.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the load balancer provider.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Region is the region name.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimage">MachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
MachineImage is a mapping from logical names and versions to provider-specific machine image data.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the logical version of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the name of the image.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the id of the image. (one of Image or ID must be set)</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the machine image</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities of the machine image.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageflavor">MachineImageFlavor
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
MachineImageFlavor groups all RegionAMIMappings for a specific set of capabilities.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>regions</code></br>
<em>
<a href="#regionidmapping">RegionIDMapping</a> array
</em>
</td>
<td>
<p>Regions is a mapping to the correct Image ID for the machine image in the supported regions.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the name of the image.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities that are supported by the Image ID in this set.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageversion">MachineImageVersion
</h3>


<p>
(<em>Appears on:</em><a href="#machineimages">MachineImages</a>)
</p>

<p>
MachineImageVersion contains a version and a provider-specific identifier.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version of the image.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the name of the image.</p>
</td>
</tr>
<tr>
<td>
<code>regions</code></br>
<em>
<a href="#regionidmapping">RegionIDMapping</a> array
</em>
</td>
<td>
<p>Regions is an optional mapping to the correct Image ID for the machine image in the supported regions.</p>
</td>
</tr>
<tr>
<td>
<code>capabilityFlavors</code></br>
<em>
<a href="#machineimageflavor">MachineImageFlavor</a> array
</em>
</td>
<td>
<p>CapabilityFlavors is grouping of region AMIs by capabilities.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimages">MachineImages
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
MachineImages is a mapping from logical names and versions to provider-specific identifiers.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#machineimageversion">MachineImageVersion</a> array
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinelabel">MachineLabel
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
MachineLabel define key value pair to label machines.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the machine label key</p>
</td>
</tr>
<tr>
<td>
<code>value</code></br>
<em>
string
</em>
</td>
<td>
<p>Value is the machine label value</p>
</td>
</tr>
<tr>
<td>
<code>triggerRollingOnUpdate</code></br>
<em>
boolean
</em>
</td>
<td>
<p>TriggerRollingOnUpdate controls if the machines should be rolled if the value changes</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networkstatus">NetworkStatus
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
NetworkStatus contains information about a generated Network or resources created in an existing Network.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the Network id.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the Network name.</p>
</td>
</tr>
<tr>
<td>
<code>floatingPool</code></br>
<em>
<a href="#floatingpoolstatus">FloatingPoolStatus</a>
</em>
</td>
<td>
<p>FloatingPool contains information about the floating pool.</p>
</td>
</tr>
<tr>
<td>
<code>router</code></br>
<em>
<a href="#routerstatus">RouterStatus</a>
</em>
</td>
<td>
<p>Router contains information about the Router and related resources.</p>
</td>
</tr>
<tr>
<td>
<code>subnets</code></br>
<em>
<a href="#subnet">Subnet</a> array
</em>
</td>
<td>
<p>Subnets is a list of subnets that have been created.</p>
</td>
</tr>
<tr>
<td>
<code>shareNetwork</code></br>
<em>
<a href="#sharenetworkstatus">ShareNetworkStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShareNetwork contains information about a created/provided ShareNetwork</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networks">Networks
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructureconfig">InfrastructureConfig</a>)
</p>

<p>
Networks holds information about the Kubernetes and infrastructure networks.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>router</code></br>
<em>
<a href="#router">Router</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Router indicates whether to use an existing router or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>worker</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Worker is a CIDRs of a worker subnet (private) to create (used for the VMs).<br />Deprecated: use `workers` instead.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workers is a CIDRs of a worker subnet (private) to create (used for the VMs).<br />Mutually exclusive with SubnetPool.</p>
</td>
</tr>
<tr>
<td>
<code>subnetPool</code></br>
<em>
<a href="#subnetpool">SubnetPool</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubnetPool specifies an OpenStack subnet pool to use for automatic CIDR allocation<br />for the worker subnet. Mutually exclusive with Workers/Worker CIDR fields.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ID is the ID of an existing private network.</p>
</td>
</tr>
<tr>
<td>
<code>shareNetwork</code></br>
<em>
<a href="#sharenetwork">ShareNetwork</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShareNetwork holds information about the share network (used for shared file systems like NFS)</p>
</td>
</tr>
<tr>
<td>
<code>ipv6</code></br>
<em>
<a href="#ipv6config">IPv6Config</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPv6 holds information about the IPv6 CIDRs.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="nodestatus">NodeStatus
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
NodeStatus contains information about Node related resources.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>keyName</code></br>
<em>
string
</em>
</td>
<td>
<p>KeyName is the name of the SSH key.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="purpose">Purpose
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#securitygroup">SecurityGroup</a>, <a href="#subnet">Subnet</a>)
</p>

<p>
Purpose is a purpose of a resource.
</p>


<h3 id="regionidmapping">RegionIDMapping
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageflavor">MachineImageFlavor</a>, <a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
RegionIDMapping is a mapping to the correct ID for the machine image in the given region.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the region.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the ID for the machine image in the given region.</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the machine image</p>
</td>
</tr>

</tbody>
</table>


<h3 id="router">Router
</h3>


<p>
(<em>Appears on:</em><a href="#networks">Networks</a>)
</p>

<p>
Router indicates whether to use an existing router or create a new one.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the router id of an existing OpenStack router.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="routerstatus">RouterStatus
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
RouterStatus contains information about a generated Router or resources attached to an existing Router.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the Router id.</p>
</td>
</tr>
<tr>
<td>
<code>ip</code></br>
<em>
string
</em>
</td>
<td>
<p>IP is the router ip.<br />Deprecated: use ExternalFixedIPs instead.</p>
</td>
</tr>
<tr>
<td>
<code>externalFixedIP</code></br>
<em>
string array
</em>
</td>
<td>
<p>ExternalFixedIPs is the list of the router's assigned external fixed IPs.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="securitygroup">SecurityGroup
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
SecurityGroup is an OpenStack security group related to a Network.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#purpose">Purpose</a>
</em>
</td>
<td>
<p>Purpose is a logical description of the security group.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the security group id.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the security group name.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="servergroup">ServerGroup
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
ServerGroup contains configuration data for setting up a server group.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>policy</code></br>
<em>
string
</em>
</td>
<td>
<p>Policy describes the kind of affinity policy for instances of the server group.<br />https://docs.openstack.org/python-openstackclient/ussuri/cli/command-objects/server-group.html</p>
</td>
</tr>

</tbody>
</table>


<h3 id="servergroupdependency">ServerGroupDependency
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
ServerGroupDependency is a reference to an external machine dependency of OpenStack server groups.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>poolName</code></br>
<em>
string
</em>
</td>
<td>
<p>PoolName identifies the worker pool that this dependency belongs</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the provider's generated ID for a server group</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the server group</p>
</td>
</tr>

</tbody>
</table>


<h3 id="sharenetwork">ShareNetwork
</h3>


<p>
(<em>Appears on:</em><a href="#networks">Networks</a>)
</p>

<p>
ShareNetwork holds information about the share network (used for shared file systems like NFS)
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled is the switch to enable the creation of a share network</p>
</td>
</tr>

</tbody>
</table>


<h3 id="sharenetworkstatus">ShareNetworkStatus
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
ShareNetworkStatus contains information about a generated ShareNetwork
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the Network id.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the Network name.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="storage">Storage
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>)
</p>

<p>
Storage contains configuration for storage in the cluster.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>csiManila</code></br>
<em>
<a href="#csimanila">CSIManila</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIManila contains configuration for CSI Manila driver (support for NFS volumes)</p>
</td>
</tr>

</tbody>
</table>


<h3 id="storageclassdefinition">StorageClassDefinition
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
StorageClassDefinition is a definition of a storageClass
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the storageclass</p>
</td>
</tr>
<tr>
<td>
<code>default</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default set the storageclass to the default one</p>
</td>
</tr>
<tr>
<td>
<code>provisioner</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provisioner set the Provisioner inside the storageclass</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters adds parameters to the storageclass (storageclass.parameters)</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations sets annotations for the storageclass</p>
</td>
</tr>
<tr>
<td>
<code>labels</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels sets labels for the storageclass</p>
</td>
</tr>
<tr>
<td>
<code>reclaimPolicy</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReclaimPolicy sets reclaimPolicy for the storageclass</p>
</td>
</tr>
<tr>
<td>
<code>volumeBindingMode</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeBindingMode sets bindingMode for the storageclass</p>
</td>
</tr>

</tbody>
</table>


<h3 id="subnet">Subnet
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
Subnet is an OpenStack subnet related to a Network.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#purpose">Purpose</a>
</em>
</td>
<td>
<p>Purpose is a logical description of the subnet.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the subnet id.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<p>CIDR is the CIDR of the subnet.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="subnetpool">SubnetPool
</h3>


<p>
(<em>Appears on:</em><a href="#networks">Networks</a>)
</p>

<p>
SubnetPool specifies an OpenStack subnet pool from which a CIDR will be automatically allocated.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the ID of the OpenStack subnet pool.</p>
</td>
</tr>
<tr>
<td>
<code>prefixLength</code></br>
<em>
integer
</em>
</td>
<td>
<p>PrefixLength is the prefix length (e.g. 24 for a /24 subnet) to request from the pool.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerconfig">WorkerConfig
</h3>


<p>
WorkerConfig contains configuration data for a worker pool.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>nodeTemplate</code></br>
<em>
<a href="#nodetemplate">NodeTemplate</a>
</em>
</td>
<td>
<p>NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate<br />nodeTemplate during scaling a nodeGroup from zero.</p>
</td>
</tr>
<tr>
<td>
<code>serverGroup</code></br>
<em>
<a href="#servergroup">ServerGroup</a>
</em>
</td>
<td>
<p>ServerGroup contains configuration data for the worker pool's server group. If this object is present,<br />OpenStack provider extension will try to create a new server group for instances of this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>machineLabels</code></br>
<em>
<a href="#machinelabel">MachineLabel</a> array
</em>
</td>
<td>
<p>MachineLabels define key value pairs to add to machines.</p>
</td>
</tr>
<tr>
<td>
<code>additionalSecurityGroups</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalSecurityGroups is a list of names of pre-existing OpenStack security<br />groups to attach to every node in this worker pool, in addition to the<br />auto-managed "nodes" security group.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerstatus">WorkerStatus
</h3>


<p>
WorkerStatus contains information about created worker resources.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#machineimage">MachineImage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller<br />gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if<br />a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`<br />resources that are still using this version. Hence, it stores the used versions in the provider status to ensure<br />reconciliation is possible.</p>
</td>
</tr>
<tr>
<td>
<code>serverGroupDependencies</code></br>
<em>
<a href="#servergroupdependency">ServerGroupDependency</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServerGroupDependencies is a list of external server group dependencies.</p>
</td>
</tr>

</tbody>
</table>


