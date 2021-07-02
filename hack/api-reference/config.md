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
<a href="https://github.com/gardener/gardener/extensions/pkg/controller/healthcheck/config">
github.com/gardener/gardener/extensions/pkg/controller/healthcheck/config/v1alpha1.HealthCheckConfig
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
<code>csi</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">
CSI
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSI is the config for the csi components</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.ControllerConfiguration">ControllerConfiguration</a>)
</p>
<p>
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
<code>csiAttacher</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIAttacher">
CSIAttacher
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIAttacher is the configuration for the csi-attacher</p>
</td>
</tr>
<tr>
<td>
<code>csiDriverCinder</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIDriverCinder">
CSIDriverCinder
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIDriverCinder is the configuration for the csi-driver-cinder</p>
</td>
</tr>
<tr>
<td>
<code>csiProvisioner</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIProvisioner">
CSIProvisioner
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIProvisioner is the configuration for the csi-provisioner</p>
</td>
</tr>
<tr>
<td>
<code>csiResizer</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIResizer">
CSIResizer
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSIResizer is the configuration for the csi-resizer</p>
</td>
</tr>
<tr>
<td>
<code>csiSnapshotController</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSISnapshotController">
CSISnapshotController
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSISnapshotController is the configuration for the csi-shapshot-controller</p>
</td>
</tr>
<tr>
<td>
<code>csiSnapshotter</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSISnapshotter">
CSISnapshotter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSISnapshotter is the configuration for the csi-snapshotter</p>
</td>
</tr>
<tr>
<td>
<code>csiLivenessProbe</code></br>
<em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSILivenessProbe">
CSILivenessProbe
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSILivenessProbe is the configuration for the csi-liveness-probe</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIAttacher">CSIAttacher
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>retryIntervalStart</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetryIntervalStart The exponential backoff for failures.</p>
</td>
</tr>
<tr>
<td>
<code>retryIntervalMax</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetryIntervalMax The exponential backoff maximum value.</p>
</td>
</tr>
<tr>
<td>
<code>reconcileSync</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReconcileSync Resync frequency of the attached volumes with the driver.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbose The verbosity level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIDriverCinder">CSIDriverCinder
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbose The verbosity level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSILivenessProbe">CSILivenessProbe
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbose The verbosity level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIProvisioner">CSIProvisioner
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbose The verbosity level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSIResizer">CSIResizer
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CSITimeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbose The verbosity level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSISnapshotController">CSISnapshotController
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbose The verbosity level.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSISnapshotter">CSISnapshotter
</h3>
<p>
(<em>Appears on:</em>
<a href="#openstack.provider.extensions.config.gardener.cloud/v1alpha1.CSI">CSI</a>)
</p>
<p>
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
<code>timeout</code></br>
<em>
string
</em>
</td>
<td>
<p>Timeout Timeout of all calls to the container storage interface driver.</p>
</td>
</tr>
<tr>
<td>
<code>verbose</code></br>
<em>
string
</em>
</td>
<td>
<p>Verbose The verbosity level.</p>
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
