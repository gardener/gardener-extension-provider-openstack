# Using the OpenStack provider extension with Gardener as operator

The [`core.gardener.cloud/v1alpha1.CloudProfile` resource](https://github.com/gardener/gardener/blob/master/example/30-cloudprofile.yaml) declares a `providerConfig` field that is meant to contain provider-specific configuration.

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

The cloud profile config also contains constraints for floating pools and load balancer providers that can be used in shoots.

An example `CloudProfileConfig` for the OpenStack extension looks as follows:

```yaml
apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
kind: CloudProfileConfig
machineImages:
- name: coreos
  versions:
  - version: 2135.6.0
    image: coreos-2135.6.0
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
# rescanBlockStorageOnResize: true
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
#   region: europe
# - name: f5
#   region: asia
```

Please note that it is possible to configure a region mapping for keystone URLs, floating pools, and load balancer providers.
Additionally, floating pools can be constrainted to a keystone domain by specifying the `domain` field.
Floating pool names may also contains simple wildcard expressions, like `*` or `fp-pool-*` or `*-fp-pool`. Please note that the `*` must be either single or at the beginning or at the end. Consequently, `fp-*-pool` is not possible/allowed.
The default behavior is that, if found, the regional (and/or domain restricted) entry is taken.
If no entry for the given region exists then the fallback value is the most matching entry (w.r.t. wildcard matching) in the list without a `region` field (or the `keystoneURL` value for the keystone URLs).
If an additional floating pool should be selectable for a region and/or domain, you can mark it as non constraining
with setting the optional field `nonConstraining` to `true`.

Some OpenStack environments don't need these regional mappings, hence, the `region` and `keystoneURLs` fields are optional.
If your OpenStack environment only has regional values and it doesn't make sense to provide a (non-regional) fallback then simply
omit `keystoneURL` and always specify `region`.

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
    - version: 1.16.1
    - version: 1.16.0
      expirationDate: "2020-04-05T01:02:03Z"
  machineImages:
  - name: coreos
    versions:
    - version: 2135.6.0
  machineTypes:
  - name: medium_4_8
    cpu: "4"
    gpu: "0"
    memory: 8Gi
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
        image: coreos-2135.6.0
    keystoneURL: https://url-to-keystone/v3/
    constraints:
      floatingPools:
      - name: fp-pool-1
      loadBalancerProviders:
      - name: haproxy
```
