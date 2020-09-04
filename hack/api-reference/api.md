<p>Packages:</p>
<ul>
<li>
<a href="#openstack.provider.extensions.gardener.cloud%2fv1alpha1">openstack.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="openstack.provider.extensions.gardener.cloud/v1alpha1">openstack.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the OpenStack provider API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>
</li><li>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>
</li><li>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>
</li><li>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus</a>
</li></ul>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig
</h3>
<p>
<p>CloudProfileConfig contains provider-specific configuration that is embedded into Gardener&rsquo;s <code>CloudProfile</code>
resource.</p>
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
<code>apiVersion</code></br>
string</td>
<td>
<code>
openstack.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>CloudProfileConfig</code></td>
</tr>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Constraints">
Constraints
</a>
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
[]string
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
<p>DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for
Kubernetes 1.10.1+. See <a href="https://github.com/kubernetes/kubernetes/pull/61890">https://github.com/kubernetes/kubernetes/pull/61890</a> for details.</p>
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
<p>KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).</p>
</td>
</tr>
<tr>
<td>
<code>keystoneURLs</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.KeyStoneURL">
[]KeyStoneURL
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeyStoneURLs is a region-URL mapping for auth{n,z} in OpenStack (pointing to KeyStone).</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImages">
[]MachineImages
</a>
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps
logical names and versions to provider-specific identifiers.</p>
</td>
</tr>
<tr>
<td>
<code>requestTimeout</code></br>
<em>
string
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>RescanBlockStorageOnResize specifies whether the storage plugin scans and checks new block device size before it resizes
the filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>useOctavia</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>UseOctavia specifies whether the OpenStack Octavia network load balancing is used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig
</h3>
<p>
<p>ControlPlaneConfig contains configuration settings for the control plane.</p>
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
<code>apiVersion</code></br>
string</td>
<td>
<code>
openstack.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControlPlaneConfig</code></td>
</tr>
<tr>
<td>
<code>cloudControllerManager</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.CloudControllerManagerConfig">
CloudControllerManagerConfig
</a>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.LoadBalancerClass">
[]LoadBalancerClass
</a>
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
<p>Zone is the OpenStack zone.
Deprecated: Don&rsquo;t use anymore. Will be removed in a future version.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig
</h3>
<p>
<p>InfrastructureConfig infrastructure configuration resource</p>
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
<code>apiVersion</code></br>
string</td>
<td>
<code>
openstack.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>InfrastructureConfig</code></td>
</tr>
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
<p>FloatingPoolSubnetName contains the name of a subnet in the Floating IP Pool where the router should be attached to.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Networks">
Networks
</a>
</em>
</td>
<td>
<p>Networks is the OpenStack specific network configuration</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus
</h3>
<p>
<p>WorkerStatus contains information about created worker resources.</p>
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
<code>apiVersion</code></br>
string</td>
<td>
<code>
openstack.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkerStatus</code></td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImage">
[]MachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller
gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if
a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing <code>Worker</code>
resources that are still using this version. Hence, it stores the used versions in the provider status to ensure
reconciliation is possible.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.CloudControllerManagerConfig">CloudControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>)
</p>
<p>
<p>CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.</p>
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
map[string]bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.Constraints">Constraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>Constraints is an object containing constraints for the shoots.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.FloatingPool">
[]FloatingPool
</a>
</em>
</td>
<td>
<p>FloatingPools contains constraints regarding allowed values of the &lsquo;floatingPoolName&rsquo; block in the control plane config.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerProviders</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.LoadBalancerProvider">
[]LoadBalancerProvider
</a>
</em>
</td>
<td>
<p>LoadBalancerProviders contains constraints regarding allowed values of the &lsquo;loadBalancerProvider&rsquo; block in the control plane config.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.FloatingPool">FloatingPool
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Constraints">Constraints</a>)
</p>
<p>
<p>FloatingPool contains constraints regarding allowed values of the &lsquo;floatingPoolName&rsquo; block in the control plane config.</p>
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
<code>nonConstraining</code></br>
<em>
bool
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.LoadBalancerClass">
[]LoadBalancerClass
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerClasses contains a list of supported labeled load balancer network settings.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.FloatingPoolStatus">FloatingPoolStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>FloatingPoolStatus contains information about the floating pool.</p>
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
<tr>
<td>
<code>subnetID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubnetID is the id of floating pool subnet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus
</h3>
<p>
<p>InfrastructureStatus contains information about created infrastructure resources.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">
NetworkStatus
</a>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.NodeStatus">
NodeStatus
</a>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.SecurityGroup">
[]SecurityGroup
</a>
</em>
</td>
<td>
<p>SecurityGroups is a list of security groups that have been created.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.KeyStoneURL">KeyStoneURL
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>KeyStoneURL is a region-URL mapping for auth{n,z} in OpenStack (pointing to KeyStone).</p>
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
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.LoadBalancerClass">LoadBalancerClass
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>, 
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.FloatingPool">FloatingPool</a>)
</p>
<p>
<p>LoadBalancerClass defines a restricted network setting for generic LoadBalancer classes.</p>
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
<p>SubnetID is the ID of a local subnet used for LoadBalancer provisioning. Only usable if no FloatingPool
configuration is done.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.LoadBalancerProvider">LoadBalancerProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Constraints">Constraints</a>)
</p>
<p>
<p>LoadBalancerProvider contains constraints regarding allowed values of the &lsquo;loadBalancerProvider&rsquo; block in the control plane config.</p>
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
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImage">MachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus</a>)
</p>
<p>
<p>MachineImage is a mapping from logical names and versions to provider-specific machine image data.</p>
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
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">MachineImageVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImages">MachineImages</a>)
</p>
<p>
<p>MachineImageVersion contains a version and a provider-specific identifier.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.RegionIDMapping">
[]RegionIDMapping
</a>
</em>
</td>
<td>
<p>Regions is an optional mapping to the correct Image ID for the machine image in the supported regions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImages">MachineImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>MachineImages is a mapping from logical names and versions to provider-specific identifiers.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">
[]MachineImageVersion
</a>
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>NetworkStatus contains information about a generated Network or resources created in an existing Network.</p>
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
<code>floatingPool</code></br>
<em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.FloatingPoolStatus">
FloatingPoolStatus
</a>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.RouterStatus">
RouterStatus
</a>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Subnet">
[]Subnet
</a>
</em>
</td>
<td>
<p>Subnets is a list of subnets that have been created.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.Networks">Networks
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>)
</p>
<p>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Router">
Router
</a>
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
<p>Worker is a CIDRs of a worker subnet (private) to create (used for the VMs).
Deprecated - use <code>workers</code> instead.</p>
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
<p>Workers is a CIDRs of a worker subnet (private) to create (used for the VMs).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.NodeStatus">NodeStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>NodeStatus contains information about Node related resources.</p>
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
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.Purpose">Purpose
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.SecurityGroup">SecurityGroup</a>, 
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Subnet">Subnet</a>)
</p>
<p>
<p>Purpose is a purpose of a resource.</p>
</p>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.RegionIDMapping">RegionIDMapping
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">MachineImageVersion</a>)
</p>
<p>
<p>RegionIDMapping is a mapping to the correct ID for the machine image in the given region.</p>
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
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.Router">Router
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Networks">Networks</a>)
</p>
<p>
<p>Router indicates whether to use an existing router or create a new one.</p>
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
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.RouterStatus">RouterStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>RouterStatus contains information about a generated Router or resources attached to an existing Router.</p>
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
</tbody>
</table>
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.SecurityGroup">SecurityGroup
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>SecurityGroup is an OpenStack security group related to a Network.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Purpose">
Purpose
</a>
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
<h3 id="openstack.provider.extensions.gardener.cloud/v1alpha1.Subnet">Subnet
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>Subnet is an OpenStack subnet related to a Network.</p>
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
<a href="#openstack.provider.extensions.gardener.cloud/v1alpha1.Purpose">
Purpose
</a>
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
</tbody>
</table>
<hr/>
