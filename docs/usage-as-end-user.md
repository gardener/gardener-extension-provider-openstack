# Using the OpenStack provider extension with Gardener as end-user

The [`core.gardener.cloud/v1beta1.Shoot` resource](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) declares a few fields that are meant to contain provider-specific configuration.

In this document we are describing how this configuration looks like for OpenStack and provide an example `Shoot` manifest with minimal configuration that you can use to create an OpenStack cluster (modulo the landscape-specific information like cloud profile names, secret binding names, etc.).

## Provider Secret Data

Every shoot cluster references a `SecretBinding` which itself references a `Secret`, and this `Secret` contains the provider credentials of your OpenStack tenant.
This `Secret` must look as follows:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: core-openstack
  namespace: garden-dev
type: Opaque
data:
  domainName: base64(domain-name)
  tenantName: base64(tenant-name)
  
  # either use username/password
  username: base64(user-name)
  password: base64(password)

  # or application credentials
  #applicationCredentialID: base64(app-credential-id)
  #applicationCredentialName: base64(app-credential-name) # optional
  #applicationCredentialSecret: base64(app-credential-secret)
```

Please look up https://docs.openstack.org/keystone/pike/admin/identity-concepts.html as well.

For authentication with username/password see [Keystone username/password](https://docs.openstack.org/keystone/latest/user/supported_clients.html)

Alternatively, for authentication with application credentials see [Keystone Application Credentials](https://docs.openstack.org/keystone/latest/user/application_credentials.html). Application Credentials are **not** supported for shoots with kubernetes versions **less than v1.19**.


⚠️ Depending on your API usage it can be problematic to reuse the same provider credentials for different Shoot clusters due to rate limits.
Please consider spreading your Shoots over multiple credentials from different tenants if you are hitting those limits.

### Managed Application Credentials

The Gardener Openstack extension is capable to manage for each Shoot cluster an own application credential that is used to interact with the Openstack layer on behalf of the provided Openstack user.

Those application credentials are managed by the Gardener Openstack extension.
The extension will take care that the managed application credential get created, deleted and constanntly rotated and exchanged (including the case when the Openstack user changes) as part of the regular Shoot operations like reconcilation or deletion.

Managed application credentials will be used by default
- if the operator of the Gardener installation enables the managed application credential usage (find more information [here](usage-as-operator.md#Managed-application-credentials))
- if the provided Openstack user itself is not an application credential
- for Shoot cluster larger or equal than `v1.19`

Using managed application credentials to interact with the Openstack layer has several advantages compared to the usage of the Openstack user itself e.g. the application credential will be still functional even if the credentials of the owning Openstack user are rotated and not propagated to the system or that a managed application credential does exclusivly belong to one Shoot cluster and does therefore not get influenced/throttled by other operation run with the same Openstack user etc.

Be aware: In case the Openstack user for a Shoot cluster is changed the managed application credential can end up in an orphan state where the Gardener Openstack extension cannot manage it anymore.
Therefore each managed application credential has set an expiration date on the Openstack layer.

## `InfrastructureConfig`

The infrastructure configuration mainly describes how the network layout looks like in order to create the shoot worker nodes in a later step, thus, prepares everything relevant to create VMs, load balancers, volumes, etc.

An example `InfrastructureConfig` for the OpenStack extension looks as follows:

```yaml
apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
kind: InfrastructureConfig
floatingPoolName: MY-FLOATING-POOL
# floatingPoolSubnetName: my-floating-pool-subnet-name
networks:
# id: 12345678-abcd-efef-08af-0123456789ab
# router:
#   id: 1234
  workers: 10.250.0.0/19
```

The `floatingPoolName` is the name of the floating pool you want to use for your shoot.
If you don't know which floating pools are available look it up in the respective `CloudProfile`.

With `floatingPoolSubnetName` you can explicitly define to which subnet in the floating pool network (defined via `floatingPoolName`) the router should be attached to.

If `networks.id` is an optional field. If it is given, you can specify the uuid of an existing private Neutron network (created manually, by other tooling, ...) that should be reused. A new subnet for the Shoot will be created in it.

The `networks.router` section describes whether you want to create the shoot cluster in an already existing router or whether to create a new one:

* If `networks.router.id` is given then you have to specify the router id of the existing router that was created by other means (manually, other tooling, ...).
If you want to get a fresh router for the shoot then just omit the `networks.router` field.

* In any case, the shoot cluster will be created in a **new** subnet. 

The `networks.workers` section describes the CIDR for a subnet that is used for all shoot worker nodes, i.e., VMs which later run your applications.

You can freely choose these CIDRs and it is your responsibility to properly design the network layout to suit your needs.

Apart from the router and the worker subnet the OpenStack extension will also create a network, router interfaces, security groups, and a key pair.

## `ControlPlaneConfig`

The control plane configuration mainly contains values for the OpenStack-specific control plane components.
Today, the only component deployed by the OpenStack extension is the `cloud-controller-manager`.

An example `ControlPlaneConfig` for the OpenStack extension looks as follows:

```yaml
apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
kind: ControlPlaneConfig
loadBalancerProvider: haproxy
loadBalancerClasses:
- name: lbclass-1
  purpose: default
  floatingNetworkID: fips-1-id
  floatingSubnetName: internet-*
- name: lbclass-2
  floatingNetworkID: fips-1-id
  floatingSubnetTags: internal,private
- name: lbclass-3
  purpose: private
  subnetID: internal-id
cloudControllerManager:
  featureGates:
    CustomResourceValidation: true
```

The `loadBalancerProvider` is the provider name you want to use for load balancers in your shoot.
If you don't know which types are available look it up in the respective `CloudProfile`.

The `loadBalancerClasses` field contains an optional list of load balancer classes which will be available in the cluster. Each entry can have the following fields:
- `name` to select the load balancer class via the kubernetes [service annotations](https://github.com/kubernetes/cloud-provider-openstack/blob/master/docs/openstack-cloud-controller-manager/expose-applications-using-loadbalancer-type-service.md#switching-between-floating-subnets-by-using-preconfigured-classes) `loadbalancer.openstack.org/class=name`
- `purpose` with values `default` or `private`
  - The configuration of the `default` load balancer class will be used as default for all other kubernetes loadbalancer services without a class annotation
  - The configuration of the `private` load balancer class will be also set to the global loadbalancer configuration of the cluster, but will be overridden by the `default` purpose
- `floatingNetworkID` can be specified to receive an ip from an floating/external network, additionally the subnet in this network can be selected via
  - `floatingSubnetName` can be either a full subnet name or a regex/glob to match subnet name
  - `floatingSubnetTags` a comma seperated list of subnet tags
  - `floatingSubnetID` the id of a specific subnet
- `subnetID` can be specified by to receive an ip from an internal subnet (will not have an effect in combination with floating/external network configuration)


The `cloudControllerManager.featureGates` contains a map of explicitly enabled or disabled feature gates.
For production usage it's not recommended to use this field at all as you can enable alpha features or disable beta/stable features, potentially impacting the cluster stability.
If you don't want to configure anything for the `cloudControllerManager` simply omit the key in the YAML specification.

## `WorkerConfig`

Each worker group in a shoot may contain provider-specific configurations and options. These are contained in the `providerConfig` section of a worker group and can be configured using a `WorkerConfig` object.
An example of a `WorkerConfig` looks as follows:

```yaml
apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
kind: WorkerConfig
serverGroup:
  policy: soft-anti-affinity
```

When you specify the `serverGroup` section in your worker group configuration, a new server group will be created with the configured policy for each worker group that enabled this setting and all machines managed by this worker group will be assigned as members of the created server group.

For users to have access to the server group feature, it must be enabled on the `CloudProfile` by your operator. 
Existing clusters can take advantage of this feature by updating the server group configuration of their respective worker groups. Worker groups that are already configured with server groups can update their setting to change the policy used, or remove it altogether at any time.

Users must be aware that **any change to the server group settings will result in a rolling deployment of new nodes for the affected worker group**.


Please note the following restrictions when deploying workers with server groups:
+ The `serverGroup` section is optional, but if it is included in the worker configuration, it must contain a valid policy value.
+ The available `policy` values that can be used, are defined in the provider specific section of `CloudProfile` by your operator.
+ Certain policy values may induce further constraints. Using the `affinity` policy is only allowed when the worker group utilizes a single zone.

## Example `Shoot` manifest (one availability zone)

Please find below an example `Shoot` manifest for one availability zone:

```yaml
apiVersion: core.gardener.cloud/v1alpha1
kind: Shoot
metadata:
  name: johndoe-openstack
  namespace: garden-dev
spec:
  cloudProfileName: openstack
  region: europe-1
  secretBindingName: core-openstack
  provider:
    type: openstack
    infrastructureConfig:
      apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      floatingPoolName: MY-FLOATING-POOL
      networks:
        workers: 10.250.0.0/19
    controlPlaneConfig:
      apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
      kind: ControlPlaneConfig
      loadBalancerProvider: haproxy
    workers:
    - name: worker-xoluy
      machine:
        type: medium_4_8
      minimum: 2
      maximum: 2
      zones:
      - europe-1a
  networking:
    nodes: 10.250.0.0/16
    type: calico
  kubernetes:
    version: 1.16.1
  maintenance:
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
  addons:
    kubernetes-dashboard:
      enabled: true
    nginx-ingress:
      enabled: true
```

## CSI volume provisioners

Every OpenStack shoot cluster that has at least Kubernetes v1.19 will be deployed with the OpenStack Cinder CSI driver.
It is compatible with the legacy in-tree volume provisioner that was deprecated by the Kubernetes community and will be removed in future versions of Kubernetes.
End-users might want to update their custom `StorageClass`es to the new `cinder.csi.openstack.org` provisioner.
Shoot clusters with Kubernetes v1.18 or less will use the in-tree `kubernetes.io/cinder` volume provisioner in the kube-controller-manager and the kubelet.

## Kubernetes Versions per Worker Pool

This extension supports `gardener/gardener`'s `WorkerPoolKubernetesVersion` feature gate, i.e., having [worker pools with overridden Kubernetes versions](https://github.com/gardener/gardener/blob/8a9c88866ec5fce59b5acf57d4227eeeb73669d7/example/90-shoot.yaml#L69-L70) since `gardener-extension-provider-openstack@v1.23`.
Note that this feature is only usable for `Shoot`s whose `.spec.kubernetes.version` is greater or equal than the CSI migration version (`1.19`).

## Shoot CA Certificate and `ServiceAccount` Signing Key Rotation

This extension supports `gardener/gardener`'s `ShootCARotation` and `ShootSARotation` feature gates since `gardener-extension-provider-openstack@v1.26`.
