// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/api.yaml --renderer=markdown --templates-dir=$GARDENER_HACK_DIR/api-reference/template --log-level=ERROR --output-path=../../../../hack/api-reference/api.md

// Package v1alpha1 contains the OpenStack provider API resources.
// +groupName=openstack.provider.extensions.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
