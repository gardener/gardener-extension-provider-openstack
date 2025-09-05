// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate sh -c "bash $GARDENER_HACK_DIR/generate-crds.sh -p 20-crd- extensions.gardener.cloud resources.gardener.cloud"
//go:generate sh -c "$TOOLS_BIN_DIR/extension-generator --name=provider-openstack --provider-type=openstack --component-category=provider-extension --extension-oci-repository=europe-docker.pkg.dev/gardener-project/public/charts/gardener/extensions/provider-openstack:$(cat ../VERSION) --admission-runtime-oci-repository=europe-docker.pkg.dev/gardener-project/public/charts/gardener/extensions/admission-openstack-runtime:$(cat ../VERSION) --admission-application-oci-repository=europe-docker.pkg.dev/gardener-project/public/charts/gardener/extensions/admission-openstack-application:$(cat ../VERSION) --destination=./extension/base/extension.yaml"
//go:generate sh -c "$TOOLS_BIN_DIR/kustomize build ./extension -o ./extension.yaml"

// Package example contains generated manifests for all CRDs and other examples.
// Useful for development purposes.
package example
