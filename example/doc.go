// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate sh -c "bash $GARDENER_HACK_DIR/generate-crds.sh -p 20-crd- extensions.gardener.cloud druid.gardener.cloud resources.gardener.cloud"

// Package example contains generated manifests for all CRDs and other examples.
// Useful for development purposes.
package example
