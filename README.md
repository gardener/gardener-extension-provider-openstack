# [Gardener Extension for OpenStack provider](https://gardener.cloud)

[![CI Build status](https://concourse.ci.gardener.cloud/api/v1/teams/gardener/pipelines/gardener-extension-provider-openstack-master/jobs/master-head-update-job/badge)](https://concourse.ci.gardener.cloud/teams/gardener/pipelines/gardener-extension-provider-openstack-master/jobs/master-head-update-job)
[![Go Report Card](https://goreportcard.com/badge/github.com/gardener/gardener-extension-provider-openstack)](https://goreportcard.com/report/github.com/gardener/gardener-extension-provider-openstack)

Project Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service.
Its main principle is to leverage Kubernetes concepts for all of its tasks.

Recently, most of the vendor specific logic has been developed [in-tree](https://github.com/gardener/gardener).
However, the project has grown to a size where it is very hard to extend, maintain, and test.
With [GEP-1](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md) we have proposed how the architecture can be changed in a way to support external controllers that contain their very own vendor specifics.
This way, we can keep Gardener core clean and independent.

This controller implements Gardener's extension contract for the OpenStack provider.

An example for a `ControllerRegistration` resource that can be used to register this controller to Gardener can be found [here](example/controller-registration.yaml).

Please find more information regarding the extensibility concepts and a detailed proposal [here](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md).

## Supported Kubernetes versions

This extension controller supports the following Kubernetes versions:

| Version         | Support     | Conformance test results |
| --------------- | ----------- | ------------------------ |
| Kubernetes 1.25 | 1.25.0+     | N/A |
| Kubernetes 1.24 | 1.24.0+     | [![Gardener v1.24 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.24%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.24%20OpenStack) |
| Kubernetes 1.23 | 1.23.0+     | [![Gardener v1.23 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.23%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.23%20OpenStack) |
| Kubernetes 1.22 | 1.22.0+     | [![Gardener v1.22 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.22%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.22%20OpenStack) |
| Kubernetes 1.21 | 1.21.0+     | [![Gardener v1.21 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.21%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.21%20OpenStack) |
| Kubernetes 1.20 | 1.20.0+     | [![Gardener v1.20 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.20%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.20%20OpenStack) |
| Kubernetes 1.19 | 1.19.0+     | [![Gardener v1.19 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.19%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.19%20OpenStack) |
| Kubernetes 1.18 | 1.18.0+     | [![Gardener v1.18 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.18%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.18%20OpenStack) |
| Kubernetes 1.17 | 1.17.0+     | [![Gardener v1.17 Conformance Tests](https://testgrid.k8s.io/q/summary/conformance-gardener/Gardener,%20v1.17%20OpenStack/tests_status?style=svg)](https://testgrid.k8s.io/conformance-gardener#Gardener,%20v1.17%20OpenStack) |

Please take a look [here](https://github.com/gardener/gardener/blob/master/docs/usage/supported_k8s_versions.md) to see which versions are supported by Gardener in general.

----

## Compatibility

The following lists known compatibility issues of this extension controller with other Gardener components.

| OpenStack Extension | Gardener | Action | Notes |
| ----- | ----- | --- |  --------------- |
| `< v1.12.0` | `> v1.10.0` |  Please update the provider version to `>= v1.12.0` or disable the feature gate `MountHostCADirectories` in the Gardenlet. | Applies if feature flag `MountHostCADirectories` in the Gardenlet is enabled. This is to prevent duplicate volume mounts to `/usr/share/ca-certificates` in the Shoot API Server. |

## How to start using or developing this extension controller locally

You can run the controller locally on your machine by executing `make start`.

Static code checks and tests can be executed by running `make verify`. We are using Go modules for Golang package dependency management and [Ginkgo](https://github.com/onsi/ginkgo)/[Gomega](https://github.com/onsi/gomega) for testing.

## Feedback and Support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/gardener/gardener-extension-provider-openstack/issues) or join our [Slack channel #gardener](https://kubernetes.slack.com/messages/gardener) (please invite yourself to the Kubernetes workspace [here](http://slack.k8s.io)).

## Learn more!

Please find further resources about out project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)
* [GEP-1 (Gardener Enhancement Proposal) on extensibility](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md)
* [GEP-4 (New `core.gardener.cloud/v1beta1` API)](https://github.com/gardener/gardener/blob/master/docs/proposals/04-new-core-gardener-cloud-apis.md)
* [Extensibility API documentation](https://github.com/gardener/gardener/tree/master/docs/extensions)
* [Gardener Extensions Golang library](https://godoc.org/github.com/gardener/gardener/extensions/pkg)
* [Gardener API Reference](https://gardener.cloud/api-reference/)
