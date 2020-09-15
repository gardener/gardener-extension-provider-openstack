## Compatibility 

| Openstack Extension | Gardener | Notes |
| --- | ----------- | --- |
| `>= v1.12.0` | `>v1.10.0` | Applies if feature flag `MountHostCADirectories` in the Gardenlet is enabled. This is to prevent duplicate volume mounts to `/usr/share/ca-certificates` in the Shoot API Server.  |