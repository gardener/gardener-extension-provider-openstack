// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ClusterToControlPlaneMapper returns a mapper that returns requests for ControlPlanes whose
// referenced clusters have been modified.
func ClusterToControlPlaneMapper(predicates []predicate.Predicate) extensionshandler.Mapper {
	return extensionshandler.ClusterToObjectMapper(func() client.ObjectList { return &extensionsv1alpha1.ControlPlaneList{} }, predicates)
}
