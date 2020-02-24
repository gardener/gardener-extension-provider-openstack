module github.com/gardener/gardener-extension-provider-openstack

go 1.13

require (
	github.com/ahmetb/gen-crd-api-reference-docs v0.1.5
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f
	github.com/gardener/gardener v1.0.1-0.20200213093126-7a6123b6ae21
	github.com/gardener/gardener-extensions v1.3.1-0.20200219155613-f5f526846320
	github.com/gardener/machine-controller-manager v0.26.0
	github.com/go-logr/logr v0.1.0
	github.com/gobuffalo/packr/v2 v2.1.0
	github.com/golang/mock v1.3.1
	github.com/gophercloud/gophercloud v0.7.0
	github.com/gophercloud/utils v0.0.0-20190527093828-25f1b77b8c03
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.0.0-20191010143144-fbf594f18f80
	k8s.io/apiextensions-apiserver v0.0.0-20190918161926-8f644eb6e783
	k8s.io/apimachinery v0.0.0-20191016060620-86f2f1b9c076
	k8s.io/apiserver v0.0.0-20191010014313-3893be10d307
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.0.0-20190912054826-cd179ad6a269
	k8s.io/component-base v0.0.0-20190918160511-547f6c5d7090
	k8s.io/gengo v0.0.0-20190826232639-a874a240740c
	k8s.io/klog v1.0.0
	k8s.io/kubelet v0.0.0-20190918162654-250a1838aa2c
	sigs.k8s.io/controller-runtime v0.4.0
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918155943-95b840bb6a1f // kubernetes-1.16.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655 // kubernetes-1.16.0
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190918160949-bfa5e2e684ad // kubernetes-1.16.0
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90 // kubernetes-1.16.0
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190912054826-cd179ad6a269 // kubernetes-1.16.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190918161219-8c8f079fddc3 // kubernetes-1.16.0
)
