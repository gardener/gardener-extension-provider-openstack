# OpenStack Resource Tagging

This document describes which OpenStack resources are tagged by
`gardener-extension-provider-openstack` and what tags are applied to each resource type.
At the moment, tagging is implemented only for worker node virtual machines.

## Overview

The extension applies metadata tags to OpenStack resources for two primary purposes:

1. **Ownership identification** — marking resources as Gardener-managed so they
   can be found, filtered, and reconciled correctly.
2. **Kubernetes node labeling** — propagating Shoot worker pool labels onto VM
   instances so that Kubernetes node labels are backed by OpenStack server metadata
   (used by the cloud controller manager).

## Tagged Resource Types

### Virtual Machines (Worker Nodes)

Worker node VMs receive metadata tags derived from the worker pool configuration
in the Shoot spec.

| Tag Key | Value | Source |
|---|---|---|
| `kubernetes.io-cluster-{technicalID}` | `"1"` | Shoot technical ID |
| `kubernetes.io-role-node` | `"1"` | Static |
| `{label-key}` | `{label-value}` | Each entry in `shoot.spec.provider.workers[].labels` |
| `{label-key}` | `{label-value}` | Each entry in `workerConfig.machineLabels` |

**Example:** Given the following worker pool configuration:

```yaml
spec:
  provider:
    workers:
    - name: production
      labels:
        worker.gardener.cloud/pool: production
        workload-type: high-memory
      providerConfig:
        apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
        kind: WorkerConfig
        machineLabels:
        - name: custom/label
          value: my-value
```

The resulting VM metadata tags are:

```text
kubernetes.io-cluster-shoot--my-project--my-cluster: "1"
kubernetes.io-role-node: "1"
worker.gardener.cloud-pool: production
workload-type: high-memory
custom-label: my-value
```

> Note: Label keys `worker.gardener.cloud/pool` and `custom/label` are sanitized
> to `worker.gardener.cloud-pool` and `custom-label` because `/` is not allowed
> in OpenStack metadata keys (see [Tag Sanitization](#tag-sanitization) below).

#### Machine Labels (`workerConfig.machineLabels`)

In addition to pool-level labels, you can define machine-specific labels via
`WorkerConfig.machineLabels`. These support an optional `triggerRollingOnUpdate`
field: when set to `true`, changing the label value causes the worker pool machines
to be rolled (replaced).

```yaml
providerConfig:
  apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
  kind: WorkerConfig
  machineLabels:
  - name: environment
    value: production
    triggerRollingOnUpdate: true
```

## Tag Sanitization

OpenStack server metadata keys do not allow certain characters that are common
in Kubernetes label keys. Worker pool label keys and machine label keys are
sanitized before being applied as OpenStack metadata tags.

**Allowed characters in tag keys:** `a-zA-Z0-9 -_:.` (alphanumeric, hyphen,
underscore, colon, period, space), see [OpenStack documentation](https://docs.openstack.org/api-ref/compute/#server-tags-servers-tags).

Any character outside this set is replaced with a hyphen (`-`).

**Examples:**

| Original Kubernetes Label Key | Sanitized OpenStack Tag Key |
|---|---|
| `worker.gardener.cloud/pool` | `worker.gardener.cloud-pool` |
| `kubernetes.io/role` | `kubernetes.io-role` |
| `my_label` | `my_label` (unchanged) |
| `label:with:colons` | `label:with:colons` (unchanged) |

Note that only tag **keys** are sanitized; tag **values** are passed through
unchanged.

### Collision Handling

If two different original keys sanitize to the same tag key, the last value
written wins.
If both are present in the same label source, one will silently overwrite the
other with no error or warning.

Avoid defining label keys that differ only in characters that are replaced by
`-`, as the resulting tag value is unpredictable.

### Precedence Across Label Sources

Tags from multiple sources are merged in the following order (later sources
overwrite earlier ones):

1. `workers[].labels` (pool-level labels)
2. `workerConfig.machineLabels` (provider-specific machine labels)
3. System tags (`kubernetes.io-cluster-*`, `kubernetes.io-role-node`)

This means `machineLabels` **take precedence** over `workers[].labels` when both
produce the same tag key after sanitization, and the system tags always win over
both user-defined sources.
