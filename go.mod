module github.com/gardener/gardener-extension-provider-openstack

go 1.16

require (
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/ahmetb/gen-crd-api-reference-docs v0.2.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/gardener/etcd-druid v0.7.0
	github.com/gardener/gardener v1.43.2
	github.com/gardener/machine-controller-manager v0.41.0
	github.com/go-logr/logr v1.2.0
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.1.2
	github.com/gophercloud/gophercloud v0.7.0
	github.com/gophercloud/utils v0.0.0-20200204043447-9864b6f1f12f
	github.com/onsi/ginkgo/v2 v2.1.0
	github.com/onsi/gomega v1.18.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/tools v0.1.9
	k8s.io/api v0.23.3
	k8s.io/apiextensions-apiserver v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/apiserver v0.23.3
	k8s.io/autoscaler/vertical-pod-autoscaler v0.0.0-00010101000000-000000000000
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.23.3
	k8s.io/component-base v0.23.3
	k8s.io/kubelet v0.23.3
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/controller-tools v0.8.0
)

replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.0 // keep this value in sync with sigs.k8s.io/controller-runtime
	k8s.io/api => k8s.io/api v0.23.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.3
	k8s.io/apiserver => k8s.io/apiserver v0.23.3
	k8s.io/autoscaler => k8s.io/autoscaler v0.0.0-20201008123815-1d78814026aa // translates to k8s.io/autoscaler/vertical-pod-autoscaler@v0.9.0
	k8s.io/autoscaler/vertical-pod-autoscaler => k8s.io/autoscaler/vertical-pod-autoscaler v0.9.0
	k8s.io/client-go => k8s.io/client-go v0.23.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.23.3
	k8s.io/code-generator => k8s.io/code-generator v0.23.3
	k8s.io/component-base => k8s.io/component-base v0.23.3
	k8s.io/helm => k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.3
)
