// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/config.yaml --renderer=markdown --templates-dir=$GARDENER_HACK_DIR/api-reference/template --log-level=ERROR --output-path=../../../../hack/api-reference/config.md

// Package v1alpha1 contains the OpenStack provider configuration API resources.
// +groupName=openstack.provider.extensions.config.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config/v1alpha1"
