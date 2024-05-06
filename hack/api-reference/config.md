<p>Packages:</p>
<ul>
<li>
<a href="#openstack.provider.extensions.config.gardener.cloud%2fv1alpha1">openstack.provider.extensions.config.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1">openstack.provider.extensions.config.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the OpenStack provider configuration API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ControllerConfiguration">ControllerConfiguration</a>
</li></ul>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.ControllerConfiguration">ControllerConfiguration
</h3>
<p>
<p>ControllerConfiguration defines the configuration for the OpenStack provider.</p>
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
openstack.provider.extensions.config.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControllerConfiguration</code></td>
</tr>
<tr>
<td>
<code>clientConnection</code></br>
<em>
<a href="https://godoc.org/k8s.io/component-base/config/v1alpha1#ClientConnectionConfiguration">
Kubernetes v1alpha1.ClientConnectionConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientConnection specifies the kubeconfig file and client connection
settings for the proxy server to use when communicating with the apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCD">
ETCD
</a>
</em>
</td>
<td>
<p>ETCD is the etcd configuration.</p>
</td>
</tr>
<tr>
<td>
<code>healthCheckConfig</code></br>
<em>
<a href="https://github.com/gardener/gardener/extensions/pkg/apis/config">
github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1.HealthCheckConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HealthCheckConfig is the config for the health check controller</p>
</td>
</tr>
<tr>
<td>
<code>bastionConfig</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.BastionConfig">
BastionConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>BastionConfig the config for the Bastion</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.BastionConfig">BastionConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ControllerConfiguration">ControllerConfiguration</a>)
</p>
<p>
<p>BastionConfig is the config for the Bastion</p>
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
<code>imageRef</code></br>
<em>
string
</em>
</td>
<td>
<p>ImageRef is the openstack image reference (name or id)</p>
</td>
</tr>
<tr>
<td>
<code>flavorRef</code></br>
<em>
string
</em>
</td>
<td>
<p>FlavorRef is the openstack flavorRef reference</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCD">ETCD
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ControllerConfiguration">ControllerConfiguration</a>)
</p>
<p>
<p>ETCD is an etcd configuration.</p>
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
<code>storage</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCDStorage">
ETCDStorage
</a>
</em>
</td>
<td>
<p>ETCDStorage is the etcd storage configuration.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCDBackup">
ETCDBackup
</a>
</em>
</td>
<td>
<p>ETCDBackup is the etcd backup configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCDBackup">ETCDBackup
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCD">ETCD</a>)
</p>
<p>
<p>ETCDBackup is an etcd backup configuration.</p>
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
<code>schedule</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Schedule is the etcd backup schedule.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCDStorage">ETCDStorage
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ETCD">ETCD</a>)
</p>
<p>
<p>ETCDStorage is an etcd storage configuration.</p>
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
<code>className</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClassName is the name of the storage class used in etcd-main volume claims.</p>
</td>
</tr>
<tr>
<td>
<code>capacity</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capacity is the storage capacity used in etcd-main volume claims.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
