# Using the OpenStack provider extension with Gardener as operator

The [`core.gardener.cloud/v1beta1.CloudProfile` resource](https://github.com/gardener/gardener/blob/master/example/30-cloudprofile.yaml) declares a `providerConfig` field that is meant to contain provider-specific configuration.

In this document we are describing how this configuration looks like for OpenStack and provide an example `CloudProfile` manifest with minimal configuration that you can use to allow creating OpenStack shoot clusters.

## `CloudProfileConfig`

The cloud profile configuration contains information about the real machine image IDs in the OpenStack environment (image names).
You have to map every version that you specify in `.spec.machineImages[].versions` here such that the OpenStack extension knows the image ID for every version you want to offer.

It also contains optional default values for DNS servers that shall be used for shoots.
In the `dnsServers[]` list you can specify IP addresses that are used as DNS configuration for created shoot subnets.

Also, you have to specify the keystone URL in the `keystoneURL` field to your environment.

Additionally, you can influence the HTTP request timeout when talking to the OpenStack API in the `requestTimeout` field.
This may help when you have for example a long list of load balancers in your environment.

In case your OpenStack system uses [Octavia](https://docs.openstack.org/octavia/latest/) for network load balancing then you have to set the `useOctavia` field to `true` such that the cloud-controller-manager for OpenStack gets correctly configured (it defaults to `false`).

Some hypervisors (especially those which are VMware-based) don't automatically send a new volume size to a Linux kernel when a volume is resized and in-use.
For those hypervisors you can enable the storage plugin interacting with Cinder to telling the SCSI block device to refresh its information to provide information about it's updated size to the kernel. You might need to enable this behavior depending on the underlying hypervisor of your OpenStack installation. The `rescanBlockStorageOnResize` field controls this. Please note that it only applies for Kubernetes versions where CSI is used.

Some openstack configurations do not allow to attach more volumes than a specific amount to a single node.
To tell the k8s scheduler to not over schedule volumes on a node, you can set `nodeVolumeAttachLimit` which defaults to 256.
Some openstack configurations have different names for volume and compute availability zones, which might cause pods to go into pending state as there are no nodes available in the detected volume AZ. To ignore the volume AZ when scheduling pods, you can set `ignoreVolumeAZ` to `true` (it defaults to `false`).
See [CSI Cinder driver](https://github.com/kubernetes/cloud-provider-openstack/blob/master/docs/cinder-csi-plugin/using-cinder-csi-plugin.md#block-storage).

The cloud profile config also contains constraints for floating pools and load balancer providers that can be used in shoots.

If your OpenStack system supports server groups, the `serverGroupPolicies` property will enable your end-users to create shoots with workers where the nodes are managed by Nova's server groups.
Specifying `serverGroupPolicies` is optional and can be omitted. If enabled, the end-user can choose whether or not to use this feature for a shoot's workers. Gardener will handle the creation of the server group and node assignment.

To enable this feature, an operator should:

+ specify the allowed policy values (e.g. `affintity`, `anti-affinity`) in this section. Only the policies in the allow-list will be available for end-users.
+ make sure that your OpenStack project has enough server group capacity. Otherwise, shoot creation will fail.

If your OpenStack system has multiple `volume-types`, the `storageClasses` property enables the creation of kubernetes `storageClasses` for shoots.
Set `storageClasses[].parameters.type` to map it with an openstack `volume-type`. Specifying `storageClasses` is optional and can be omitted.

An example `CloudProfileConfig` for the OpenStack extension looks as follows:

```yaml
apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
kind: CloudProfileConfig
machineImages:
- name: coreos
  versions:
  - version: 2135.6.0
    # Fallback to image name if no region mapping is found
    # Only works for amd64 and is strongly discouraged. Prefer image IDs!
    image: coreos-2135.6.0
    regions:
    - name: europe
      id: "1234-amd64"
      architecture: amd64 # optional, defaults to amd64
    - name: europe
      id: "1234-arm64"
      architecture: arm64
    - name: asia
      id: "5678-amd64"
      architecture: amd64
# keystoneURL: https://url-to-keystone/v3/
# keystoneURLs:
# - region: europe
#   url: https://europe.example.com/v3/
# - region: asia
#   url: https://asia.example.com/v3/
# dnsServers:
# - 10.10.10.11
# - 10.10.10.12
# requestTimeout: 60s
# useOctavia: true
# useSNAT: true
# rescanBlockStorageOnResize: true
# ignoreVolumeAZ: true
# nodeVolumeAttachLimit: 30
# serverGroupPolicies:
# - soft-anti-affinity
# - anti-affinity
# resolvConfOptions:
# - rotate
# - timeout:1
# storageClasses:
# - name: example-sc
#   default: false
#   provisioner: cinder.csi.openstack.org
#   volumeBindingMode: WaitForFirstConsumer
#   parameters:
#     type: storage_premium_perf0
constraints:
  floatingPools:
  - name: fp-pool-1
#   region: europe
#   loadBalancerClasses:
#   - name: lb-class-1
#     floatingSubnetID: "1234"
#     floatingNetworkID: "4567"
#     subnetID: "7890"
# - name: "fp-pool-*"
#   region: europe
#   loadBalancerClasses:
#   - name: lb-class-1
#     floatingSubnetID: "1234"
#     floatingNetworkID: "4567"
#     subnetID: "7890"
# - name: "fp-pool-eu-demo"
#   region: europe
#   domain: demo
#   loadBalancerClasses:
#   - name: lb-class-1
#     floatingSubnetID: "1234"
#     floatingNetworkID: "4567"
#     subnetID: "7890"
# - name: "fp-pool-eu-dev"
#   region: europe
#   domain: dev
#   nonConstraining: true
#   loadBalancerClasses:
#   - name: lb-class-1
#     floatingSubnetID: "1234"
#     floatingNetworkID: "4567"
#     subnetID: "7890"
  loadBalancerProviders:
  - name: haproxy
# - name: f5
#   region: asia
# - name: haproxy
#   region: asia
```

Please note that it is possible to configure a region mapping for keystone URLs, floating pools, and load balancer providers.
Additionally, floating pools can be constrainted to a keystone domain by specifying the `domain` field.
Floating pool names may also contains simple wildcard expressions, like `*` or `fp-pool-*` or `*-fp-pool`. Please note that the `*` must be either single or at the beginning or at the end. Consequently, `fp-*-pool` is not possible/allowed.
The default behavior is that, if found, the regional (and/or domain restricted) entry is taken.
If no entry for the given region exists then the fallback value is the most matching entry (w.r.t. wildcard matching) in the list without a `region` field (or the `keystoneURL` value for the keystone URLs).
If an additional floating pool should be selectable for a region and/or domain, you can mark it as non constraining
with setting the optional field `nonConstraining` to `true`.
Multiple `loadBalancerProviders` can be specified in the `CloudProfile`. Each provider may specify a region constraint for where it can be used.
If at least one region specific entry exists in the `CloudProfile`, the shoot's specified `loadBalancerProvider` must adhere to the list of the available providers of that region. Otherwise, one of the non-regional specific providers should be used.
Each entry in the `loadBalancerProviders` must be uniquely identified by its name and if applicable, its region.

The `loadBalancerClasses` field is an optional list of load balancer classes which can be when the corresponding floating pool network is choosen. The load balancer classes can be configured in the same way as in the `ControlPlaneConfig` in the `Shoot` resource, therefore see [here](../usage/usage.md#ControlPlaneConfig) for more details.

Some OpenStack environments don't need these regional mappings, hence, the `region` and `keystoneURLs` fields are optional.
If your OpenStack environment only has regional values and it doesn't make sense to provide a (non-regional) fallback then simply
omit `keystoneURL` and always specify `region`.

If Gardener creates and manages the router of a shoot cluster, it is additionally possible to specify that the [enable_snat](https://registry.terraform.io/providers/terraform-provider-openstack/openstack/latest/docs/resources/networking_router_v2#enable_snat) field is set to `true` via `useSNAT: true` in the `CloudProfileConfig`.

On some OpenStack enviroments, there may be the need to set options in the file `/etc/resolv.conf` on worker nodes.
If the field `resolvConfOptions` is set, a systemd service will be installed which copies `/run/systemd/resolve/resolv.conf`
on every change to `/etc/resolv.conf` and appends the given options.

## Example `CloudProfile` manifest

Please find below an example `CloudProfile` manifest:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: openstack
spec:
  type: openstack
  kubernetes:
    versions:
    - version: 1.27.3
      expirationDate: "2022-10-31T23:59:59Z"
  machineImages:
  - name: coreos
    versions:
    - version: 2135.6.0
      architectures: # optional, defaults to [amd64]
      - amd64
      - arm64
  machineTypes:
  - name: medium_4_8
    cpu: "4"
    gpu: "0"
    memory: 8Gi
    architecture: amd64 # optional, defaults to amd64
    storage:
      class: standard
      type: default
      size: 40Gi
  - name: medium_4_8_arm
    cpu: "4"
    gpu: "0"
    memory: 8Gi
    architecture: arm64
    storage:
      class: standard
      type: default
      size: 40Gi
  regions:
  - name: europe-1
    zones:
    - name: europe-1a
    - name: europe-1b
    - name: europe-1c
  providerConfig:
    apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
    kind: CloudProfileConfig
    machineImages:
    - name: coreos
      versions:
      - version: 2135.6.0
        # Fallback to image name if no region mapping is found
        # Only works for amd64 and is strongly discouraged. Prefer image IDs!
        image: coreos-2135.6.0
        regions:
        - name: europe
          id: "1234-amd64"
          architecture: amd64 # optional, defaults to amd64
        - name: europe
          id: "1234-arm64"
          architecture: arm64
        - name: asia
          id: "5678-amd64"
          architecture: amd64
    keystoneURL: https://url-to-keystone/v3/
    constraints:
      floatingPools:
      - name: fp-pool-1
      loadBalancerProviders:
      - name: haproxy
```
