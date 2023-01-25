// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ControllerConfiguration initializes an empty CSI config object.
func SetDefaults_ControllerConfiguration(obj *ControllerConfiguration) {
	if obj.CSI == nil {
		obj.CSI = &CSI{}
	}
}

// SetDefaults_CSI initializes an empty CSIAttacher object.
func SetDefaults_CSI(obj *CSI) {
	if obj.CSIAttacher == nil {
		obj.CSIAttacher = &CSIAttacher{}
	}
}

// SetDefaults_CSIAttacher sets the default timeout of the csi-attacher component to 3m and verbosity to 5.
func SetDefaults_CSIAttacher(obj *CSIAttacher) {
	if obj.Timeout == nil {
		obj.Timeout = &metav1.Duration{Duration: time.Minute * 3}
	}
	if obj.Verbosity == nil {
		obj.Verbosity = pointer.Int32(5)
	}
}
