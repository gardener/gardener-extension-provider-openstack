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
  username: base64(user-name)
  password: base64(password)
```

Please look up https://docs.openstack.org/keystone/pike/admin/identity-concepts.html as well.

## `InfrastructureConfig`

The infrastructure configuration mainly describes how the network layout looks like in order to create the shoot worker nodes in a later step, thus, prepares everything relevant to create VMs, load balancers, volumes, etc.

An example `InfrastructureConfig` for the OpenStack extension looks as follows:

```yaml
apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
kind: InfrastructureConfig
floatingPoolName: MY-FLOATING-POOL
networks:
# router:
#   id: 1234
  workers: 10.250.0.0/19
```

The `floatingPoolName` is the name of the floating pool you want to use for your shoot.
If you don't know which floating pools are available look it up in the respective `CloudProfile`.

The `networks.router` section describes whether you want to create the shoot cluster in an already existing router or whether to create a new one:

* If `networks.router.name` is given then you have to specify the router name of the existing router that was created by other means (manually, other tooling, ...).
If you want to get a fresh router for the shoot then just omit the `networks.router` field.

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
zone: europe-1a
cloudControllerManager:
  featureGates:
    CustomResourceValidation: true
```

The `loadBalancerProvider` is the provider name you want to use for load balancers in your shoot.
If you don't know which types are available look it up in the respective `CloudProfile`.

The `zone` field tells the cloud-controller-manager in which zone it should mainly operate.
You can still create clusters in multiple availability zones, however, the cloud-controller-manager requires one "main" zone.
:warning: You always have to specify this field!

The `cloudControllerManager.featureGates` contains a map of explicitly enabled or disabled feature gates.
For production usage it's not recommend to use this field at all as you can enable alpha features or disable beta/stable features, potentially impacting the cluster stability.
If you don't want to configure anything for the `cloudControllerManager` simply omit the key in the YAML specification.

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
      zone: europe-1a
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
