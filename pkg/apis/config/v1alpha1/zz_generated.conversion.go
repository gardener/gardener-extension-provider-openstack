//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha1

import (
	unsafe "unsafe"

	config "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	healthcheckconfig "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/config"
	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/config/v1alpha1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	componentbaseconfig "k8s.io/component-base/config"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*BastionConfig)(nil), (*config.BastionConfig)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_BastionConfig_To_config_BastionConfig(a.(*BastionConfig), b.(*config.BastionConfig), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.BastionConfig)(nil), (*BastionConfig)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_BastionConfig_To_v1alpha1_BastionConfig(a.(*config.BastionConfig), b.(*BastionConfig), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSI)(nil), (*config.CSI)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSI_To_config_CSI(a.(*CSI), b.(*config.CSI), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSI)(nil), (*CSI)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSI_To_v1alpha1_CSI(a.(*config.CSI), b.(*CSI), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSIAttacher)(nil), (*config.CSIAttacher)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSIAttacher_To_config_CSIAttacher(a.(*CSIAttacher), b.(*config.CSIAttacher), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSIAttacher)(nil), (*CSIAttacher)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSIAttacher_To_v1alpha1_CSIAttacher(a.(*config.CSIAttacher), b.(*CSIAttacher), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSIBaseArgs)(nil), (*config.CSIBaseArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(a.(*CSIBaseArgs), b.(*config.CSIBaseArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSIBaseArgs)(nil), (*CSIBaseArgs)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(a.(*config.CSIBaseArgs), b.(*CSIBaseArgs), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSIDriverCinder)(nil), (*config.CSIDriverCinder)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSIDriverCinder_To_config_CSIDriverCinder(a.(*CSIDriverCinder), b.(*config.CSIDriverCinder), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSIDriverCinder)(nil), (*CSIDriverCinder)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSIDriverCinder_To_v1alpha1_CSIDriverCinder(a.(*config.CSIDriverCinder), b.(*CSIDriverCinder), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSILivenessProbe)(nil), (*config.CSILivenessProbe)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSILivenessProbe_To_config_CSILivenessProbe(a.(*CSILivenessProbe), b.(*config.CSILivenessProbe), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSILivenessProbe)(nil), (*CSILivenessProbe)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSILivenessProbe_To_v1alpha1_CSILivenessProbe(a.(*config.CSILivenessProbe), b.(*CSILivenessProbe), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSIProvisioner)(nil), (*config.CSIProvisioner)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSIProvisioner_To_config_CSIProvisioner(a.(*CSIProvisioner), b.(*config.CSIProvisioner), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSIProvisioner)(nil), (*CSIProvisioner)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSIProvisioner_To_v1alpha1_CSIProvisioner(a.(*config.CSIProvisioner), b.(*CSIProvisioner), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSIResizer)(nil), (*config.CSIResizer)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSIResizer_To_config_CSIResizer(a.(*CSIResizer), b.(*config.CSIResizer), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSIResizer)(nil), (*CSIResizer)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSIResizer_To_v1alpha1_CSIResizer(a.(*config.CSIResizer), b.(*CSIResizer), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSISnapshotController)(nil), (*config.CSISnapshotController)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSISnapshotController_To_config_CSISnapshotController(a.(*CSISnapshotController), b.(*config.CSISnapshotController), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSISnapshotController)(nil), (*CSISnapshotController)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSISnapshotController_To_v1alpha1_CSISnapshotController(a.(*config.CSISnapshotController), b.(*CSISnapshotController), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*CSISnapshotter)(nil), (*config.CSISnapshotter)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_CSISnapshotter_To_config_CSISnapshotter(a.(*CSISnapshotter), b.(*config.CSISnapshotter), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.CSISnapshotter)(nil), (*CSISnapshotter)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_CSISnapshotter_To_v1alpha1_CSISnapshotter(a.(*config.CSISnapshotter), b.(*CSISnapshotter), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*ControllerConfiguration)(nil), (*config.ControllerConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_ControllerConfiguration_To_config_ControllerConfiguration(a.(*ControllerConfiguration), b.(*config.ControllerConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.ControllerConfiguration)(nil), (*ControllerConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_ControllerConfiguration_To_v1alpha1_ControllerConfiguration(a.(*config.ControllerConfiguration), b.(*ControllerConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*ETCD)(nil), (*config.ETCD)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_ETCD_To_config_ETCD(a.(*ETCD), b.(*config.ETCD), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.ETCD)(nil), (*ETCD)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_ETCD_To_v1alpha1_ETCD(a.(*config.ETCD), b.(*ETCD), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*ETCDBackup)(nil), (*config.ETCDBackup)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_ETCDBackup_To_config_ETCDBackup(a.(*ETCDBackup), b.(*config.ETCDBackup), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.ETCDBackup)(nil), (*ETCDBackup)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_ETCDBackup_To_v1alpha1_ETCDBackup(a.(*config.ETCDBackup), b.(*ETCDBackup), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*ETCDStorage)(nil), (*config.ETCDStorage)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_ETCDStorage_To_config_ETCDStorage(a.(*ETCDStorage), b.(*config.ETCDStorage), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*config.ETCDStorage)(nil), (*ETCDStorage)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_config_ETCDStorage_To_v1alpha1_ETCDStorage(a.(*config.ETCDStorage), b.(*ETCDStorage), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha1_BastionConfig_To_config_BastionConfig(in *BastionConfig, out *config.BastionConfig, s conversion.Scope) error {
	out.ImageRef = in.ImageRef
	out.FlavorRef = in.FlavorRef
	return nil
}

// Convert_v1alpha1_BastionConfig_To_config_BastionConfig is an autogenerated conversion function.
func Convert_v1alpha1_BastionConfig_To_config_BastionConfig(in *BastionConfig, out *config.BastionConfig, s conversion.Scope) error {
	return autoConvert_v1alpha1_BastionConfig_To_config_BastionConfig(in, out, s)
}

func autoConvert_config_BastionConfig_To_v1alpha1_BastionConfig(in *config.BastionConfig, out *BastionConfig, s conversion.Scope) error {
	out.ImageRef = in.ImageRef
	out.FlavorRef = in.FlavorRef
	return nil
}

// Convert_config_BastionConfig_To_v1alpha1_BastionConfig is an autogenerated conversion function.
func Convert_config_BastionConfig_To_v1alpha1_BastionConfig(in *config.BastionConfig, out *BastionConfig, s conversion.Scope) error {
	return autoConvert_config_BastionConfig_To_v1alpha1_BastionConfig(in, out, s)
}

func autoConvert_v1alpha1_CSI_To_config_CSI(in *CSI, out *config.CSI, s conversion.Scope) error {
	out.CSIAttacher = (*config.CSIAttacher)(unsafe.Pointer(in.CSIAttacher))
	out.CSIDriverCinder = (*config.CSIDriverCinder)(unsafe.Pointer(in.CSIDriverCinder))
	out.CSIProvisioner = (*config.CSIProvisioner)(unsafe.Pointer(in.CSIProvisioner))
	out.CSIResizer = (*config.CSIResizer)(unsafe.Pointer(in.CSIResizer))
	out.CSISnapshotController = (*config.CSISnapshotController)(unsafe.Pointer(in.CSISnapshotController))
	out.CSISnapshotter = (*config.CSISnapshotter)(unsafe.Pointer(in.CSISnapshotter))
	out.CSILivenessProbe = (*config.CSILivenessProbe)(unsafe.Pointer(in.CSILivenessProbe))
	return nil
}

// Convert_v1alpha1_CSI_To_config_CSI is an autogenerated conversion function.
func Convert_v1alpha1_CSI_To_config_CSI(in *CSI, out *config.CSI, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSI_To_config_CSI(in, out, s)
}

func autoConvert_config_CSI_To_v1alpha1_CSI(in *config.CSI, out *CSI, s conversion.Scope) error {
	out.CSIAttacher = (*CSIAttacher)(unsafe.Pointer(in.CSIAttacher))
	out.CSIDriverCinder = (*CSIDriverCinder)(unsafe.Pointer(in.CSIDriverCinder))
	out.CSIProvisioner = (*CSIProvisioner)(unsafe.Pointer(in.CSIProvisioner))
	out.CSIResizer = (*CSIResizer)(unsafe.Pointer(in.CSIResizer))
	out.CSISnapshotController = (*CSISnapshotController)(unsafe.Pointer(in.CSISnapshotController))
	out.CSISnapshotter = (*CSISnapshotter)(unsafe.Pointer(in.CSISnapshotter))
	out.CSILivenessProbe = (*CSILivenessProbe)(unsafe.Pointer(in.CSILivenessProbe))
	return nil
}

// Convert_config_CSI_To_v1alpha1_CSI is an autogenerated conversion function.
func Convert_config_CSI_To_v1alpha1_CSI(in *config.CSI, out *CSI, s conversion.Scope) error {
	return autoConvert_config_CSI_To_v1alpha1_CSI(in, out, s)
}

func autoConvert_v1alpha1_CSIAttacher_To_config_CSIAttacher(in *CSIAttacher, out *config.CSIAttacher, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	out.RetryIntervalStart = (*string)(unsafe.Pointer(in.RetryIntervalStart))
	out.RetryIntervalMax = (*string)(unsafe.Pointer(in.RetryIntervalMax))
	out.ReconcileSync = (*string)(unsafe.Pointer(in.ReconcileSync))
	return nil
}

// Convert_v1alpha1_CSIAttacher_To_config_CSIAttacher is an autogenerated conversion function.
func Convert_v1alpha1_CSIAttacher_To_config_CSIAttacher(in *CSIAttacher, out *config.CSIAttacher, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSIAttacher_To_config_CSIAttacher(in, out, s)
}

func autoConvert_config_CSIAttacher_To_v1alpha1_CSIAttacher(in *config.CSIAttacher, out *CSIAttacher, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	out.RetryIntervalStart = (*string)(unsafe.Pointer(in.RetryIntervalStart))
	out.RetryIntervalMax = (*string)(unsafe.Pointer(in.RetryIntervalMax))
	out.ReconcileSync = (*string)(unsafe.Pointer(in.ReconcileSync))
	return nil
}

// Convert_config_CSIAttacher_To_v1alpha1_CSIAttacher is an autogenerated conversion function.
func Convert_config_CSIAttacher_To_v1alpha1_CSIAttacher(in *config.CSIAttacher, out *CSIAttacher, s conversion.Scope) error {
	return autoConvert_config_CSIAttacher_To_v1alpha1_CSIAttacher(in, out, s)
}

func autoConvert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(in *CSIBaseArgs, out *config.CSIBaseArgs, s conversion.Scope) error {
	out.Timeout = (*string)(unsafe.Pointer(in.Timeout))
	out.Verbose = (*string)(unsafe.Pointer(in.Verbose))
	return nil
}

// Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs is an autogenerated conversion function.
func Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(in *CSIBaseArgs, out *config.CSIBaseArgs, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(in, out, s)
}

func autoConvert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(in *config.CSIBaseArgs, out *CSIBaseArgs, s conversion.Scope) error {
	out.Timeout = (*string)(unsafe.Pointer(in.Timeout))
	out.Verbose = (*string)(unsafe.Pointer(in.Verbose))
	return nil
}

// Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs is an autogenerated conversion function.
func Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(in *config.CSIBaseArgs, out *CSIBaseArgs, s conversion.Scope) error {
	return autoConvert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(in, out, s)
}

func autoConvert_v1alpha1_CSIDriverCinder_To_config_CSIDriverCinder(in *CSIDriverCinder, out *config.CSIDriverCinder, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_CSIDriverCinder_To_config_CSIDriverCinder is an autogenerated conversion function.
func Convert_v1alpha1_CSIDriverCinder_To_config_CSIDriverCinder(in *CSIDriverCinder, out *config.CSIDriverCinder, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSIDriverCinder_To_config_CSIDriverCinder(in, out, s)
}

func autoConvert_config_CSIDriverCinder_To_v1alpha1_CSIDriverCinder(in *config.CSIDriverCinder, out *CSIDriverCinder, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_CSIDriverCinder_To_v1alpha1_CSIDriverCinder is an autogenerated conversion function.
func Convert_config_CSIDriverCinder_To_v1alpha1_CSIDriverCinder(in *config.CSIDriverCinder, out *CSIDriverCinder, s conversion.Scope) error {
	return autoConvert_config_CSIDriverCinder_To_v1alpha1_CSIDriverCinder(in, out, s)
}

func autoConvert_v1alpha1_CSILivenessProbe_To_config_CSILivenessProbe(in *CSILivenessProbe, out *config.CSILivenessProbe, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_CSILivenessProbe_To_config_CSILivenessProbe is an autogenerated conversion function.
func Convert_v1alpha1_CSILivenessProbe_To_config_CSILivenessProbe(in *CSILivenessProbe, out *config.CSILivenessProbe, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSILivenessProbe_To_config_CSILivenessProbe(in, out, s)
}

func autoConvert_config_CSILivenessProbe_To_v1alpha1_CSILivenessProbe(in *config.CSILivenessProbe, out *CSILivenessProbe, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_CSILivenessProbe_To_v1alpha1_CSILivenessProbe is an autogenerated conversion function.
func Convert_config_CSILivenessProbe_To_v1alpha1_CSILivenessProbe(in *config.CSILivenessProbe, out *CSILivenessProbe, s conversion.Scope) error {
	return autoConvert_config_CSILivenessProbe_To_v1alpha1_CSILivenessProbe(in, out, s)
}

func autoConvert_v1alpha1_CSIProvisioner_To_config_CSIProvisioner(in *CSIProvisioner, out *config.CSIProvisioner, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_CSIProvisioner_To_config_CSIProvisioner is an autogenerated conversion function.
func Convert_v1alpha1_CSIProvisioner_To_config_CSIProvisioner(in *CSIProvisioner, out *config.CSIProvisioner, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSIProvisioner_To_config_CSIProvisioner(in, out, s)
}

func autoConvert_config_CSIProvisioner_To_v1alpha1_CSIProvisioner(in *config.CSIProvisioner, out *CSIProvisioner, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_CSIProvisioner_To_v1alpha1_CSIProvisioner is an autogenerated conversion function.
func Convert_config_CSIProvisioner_To_v1alpha1_CSIProvisioner(in *config.CSIProvisioner, out *CSIProvisioner, s conversion.Scope) error {
	return autoConvert_config_CSIProvisioner_To_v1alpha1_CSIProvisioner(in, out, s)
}

func autoConvert_v1alpha1_CSIResizer_To_config_CSIResizer(in *CSIResizer, out *config.CSIResizer, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_CSIResizer_To_config_CSIResizer is an autogenerated conversion function.
func Convert_v1alpha1_CSIResizer_To_config_CSIResizer(in *CSIResizer, out *config.CSIResizer, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSIResizer_To_config_CSIResizer(in, out, s)
}

func autoConvert_config_CSIResizer_To_v1alpha1_CSIResizer(in *config.CSIResizer, out *CSIResizer, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_CSIResizer_To_v1alpha1_CSIResizer is an autogenerated conversion function.
func Convert_config_CSIResizer_To_v1alpha1_CSIResizer(in *config.CSIResizer, out *CSIResizer, s conversion.Scope) error {
	return autoConvert_config_CSIResizer_To_v1alpha1_CSIResizer(in, out, s)
}

func autoConvert_v1alpha1_CSISnapshotController_To_config_CSISnapshotController(in *CSISnapshotController, out *config.CSISnapshotController, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_CSISnapshotController_To_config_CSISnapshotController is an autogenerated conversion function.
func Convert_v1alpha1_CSISnapshotController_To_config_CSISnapshotController(in *CSISnapshotController, out *config.CSISnapshotController, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSISnapshotController_To_config_CSISnapshotController(in, out, s)
}

func autoConvert_config_CSISnapshotController_To_v1alpha1_CSISnapshotController(in *config.CSISnapshotController, out *CSISnapshotController, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_CSISnapshotController_To_v1alpha1_CSISnapshotController is an autogenerated conversion function.
func Convert_config_CSISnapshotController_To_v1alpha1_CSISnapshotController(in *config.CSISnapshotController, out *CSISnapshotController, s conversion.Scope) error {
	return autoConvert_config_CSISnapshotController_To_v1alpha1_CSISnapshotController(in, out, s)
}

func autoConvert_v1alpha1_CSISnapshotter_To_config_CSISnapshotter(in *CSISnapshotter, out *config.CSISnapshotter, s conversion.Scope) error {
	if err := Convert_v1alpha1_CSIBaseArgs_To_config_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_CSISnapshotter_To_config_CSISnapshotter is an autogenerated conversion function.
func Convert_v1alpha1_CSISnapshotter_To_config_CSISnapshotter(in *CSISnapshotter, out *config.CSISnapshotter, s conversion.Scope) error {
	return autoConvert_v1alpha1_CSISnapshotter_To_config_CSISnapshotter(in, out, s)
}

func autoConvert_config_CSISnapshotter_To_v1alpha1_CSISnapshotter(in *config.CSISnapshotter, out *CSISnapshotter, s conversion.Scope) error {
	if err := Convert_config_CSIBaseArgs_To_v1alpha1_CSIBaseArgs(&in.CSIBaseArgs, &out.CSIBaseArgs, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_CSISnapshotter_To_v1alpha1_CSISnapshotter is an autogenerated conversion function.
func Convert_config_CSISnapshotter_To_v1alpha1_CSISnapshotter(in *config.CSISnapshotter, out *CSISnapshotter, s conversion.Scope) error {
	return autoConvert_config_CSISnapshotter_To_v1alpha1_CSISnapshotter(in, out, s)
}

func autoConvert_v1alpha1_ControllerConfiguration_To_config_ControllerConfiguration(in *ControllerConfiguration, out *config.ControllerConfiguration, s conversion.Scope) error {
	out.ClientConnection = (*componentbaseconfig.ClientConnectionConfiguration)(unsafe.Pointer(in.ClientConnection))
	if err := Convert_v1alpha1_ETCD_To_config_ETCD(&in.ETCD, &out.ETCD, s); err != nil {
		return err
	}
	out.HealthCheckConfig = (*healthcheckconfig.HealthCheckConfig)(unsafe.Pointer(in.HealthCheckConfig))
	out.BastionConfig = (*config.BastionConfig)(unsafe.Pointer(in.BastionConfig))
	out.CSI = (*config.CSI)(unsafe.Pointer(in.CSI))
	return nil
}

// Convert_v1alpha1_ControllerConfiguration_To_config_ControllerConfiguration is an autogenerated conversion function.
func Convert_v1alpha1_ControllerConfiguration_To_config_ControllerConfiguration(in *ControllerConfiguration, out *config.ControllerConfiguration, s conversion.Scope) error {
	return autoConvert_v1alpha1_ControllerConfiguration_To_config_ControllerConfiguration(in, out, s)
}

func autoConvert_config_ControllerConfiguration_To_v1alpha1_ControllerConfiguration(in *config.ControllerConfiguration, out *ControllerConfiguration, s conversion.Scope) error {
	out.ClientConnection = (*configv1alpha1.ClientConnectionConfiguration)(unsafe.Pointer(in.ClientConnection))
	if err := Convert_config_ETCD_To_v1alpha1_ETCD(&in.ETCD, &out.ETCD, s); err != nil {
		return err
	}
	out.HealthCheckConfig = (*healthcheckconfigv1alpha1.HealthCheckConfig)(unsafe.Pointer(in.HealthCheckConfig))
	out.BastionConfig = (*BastionConfig)(unsafe.Pointer(in.BastionConfig))
	out.CSI = (*CSI)(unsafe.Pointer(in.CSI))
	return nil
}

// Convert_config_ControllerConfiguration_To_v1alpha1_ControllerConfiguration is an autogenerated conversion function.
func Convert_config_ControllerConfiguration_To_v1alpha1_ControllerConfiguration(in *config.ControllerConfiguration, out *ControllerConfiguration, s conversion.Scope) error {
	return autoConvert_config_ControllerConfiguration_To_v1alpha1_ControllerConfiguration(in, out, s)
}

func autoConvert_v1alpha1_ETCD_To_config_ETCD(in *ETCD, out *config.ETCD, s conversion.Scope) error {
	if err := Convert_v1alpha1_ETCDStorage_To_config_ETCDStorage(&in.Storage, &out.Storage, s); err != nil {
		return err
	}
	if err := Convert_v1alpha1_ETCDBackup_To_config_ETCDBackup(&in.Backup, &out.Backup, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha1_ETCD_To_config_ETCD is an autogenerated conversion function.
func Convert_v1alpha1_ETCD_To_config_ETCD(in *ETCD, out *config.ETCD, s conversion.Scope) error {
	return autoConvert_v1alpha1_ETCD_To_config_ETCD(in, out, s)
}

func autoConvert_config_ETCD_To_v1alpha1_ETCD(in *config.ETCD, out *ETCD, s conversion.Scope) error {
	if err := Convert_config_ETCDStorage_To_v1alpha1_ETCDStorage(&in.Storage, &out.Storage, s); err != nil {
		return err
	}
	if err := Convert_config_ETCDBackup_To_v1alpha1_ETCDBackup(&in.Backup, &out.Backup, s); err != nil {
		return err
	}
	return nil
}

// Convert_config_ETCD_To_v1alpha1_ETCD is an autogenerated conversion function.
func Convert_config_ETCD_To_v1alpha1_ETCD(in *config.ETCD, out *ETCD, s conversion.Scope) error {
	return autoConvert_config_ETCD_To_v1alpha1_ETCD(in, out, s)
}

func autoConvert_v1alpha1_ETCDBackup_To_config_ETCDBackup(in *ETCDBackup, out *config.ETCDBackup, s conversion.Scope) error {
	out.Schedule = (*string)(unsafe.Pointer(in.Schedule))
	return nil
}

// Convert_v1alpha1_ETCDBackup_To_config_ETCDBackup is an autogenerated conversion function.
func Convert_v1alpha1_ETCDBackup_To_config_ETCDBackup(in *ETCDBackup, out *config.ETCDBackup, s conversion.Scope) error {
	return autoConvert_v1alpha1_ETCDBackup_To_config_ETCDBackup(in, out, s)
}

func autoConvert_config_ETCDBackup_To_v1alpha1_ETCDBackup(in *config.ETCDBackup, out *ETCDBackup, s conversion.Scope) error {
	out.Schedule = (*string)(unsafe.Pointer(in.Schedule))
	return nil
}

// Convert_config_ETCDBackup_To_v1alpha1_ETCDBackup is an autogenerated conversion function.
func Convert_config_ETCDBackup_To_v1alpha1_ETCDBackup(in *config.ETCDBackup, out *ETCDBackup, s conversion.Scope) error {
	return autoConvert_config_ETCDBackup_To_v1alpha1_ETCDBackup(in, out, s)
}

func autoConvert_v1alpha1_ETCDStorage_To_config_ETCDStorage(in *ETCDStorage, out *config.ETCDStorage, s conversion.Scope) error {
	out.ClassName = (*string)(unsafe.Pointer(in.ClassName))
	out.Capacity = (*resource.Quantity)(unsafe.Pointer(in.Capacity))
	return nil
}

// Convert_v1alpha1_ETCDStorage_To_config_ETCDStorage is an autogenerated conversion function.
func Convert_v1alpha1_ETCDStorage_To_config_ETCDStorage(in *ETCDStorage, out *config.ETCDStorage, s conversion.Scope) error {
	return autoConvert_v1alpha1_ETCDStorage_To_config_ETCDStorage(in, out, s)
}

func autoConvert_config_ETCDStorage_To_v1alpha1_ETCDStorage(in *config.ETCDStorage, out *ETCDStorage, s conversion.Scope) error {
	out.ClassName = (*string)(unsafe.Pointer(in.ClassName))
	out.Capacity = (*resource.Quantity)(unsafe.Pointer(in.Capacity))
	return nil
}

// Convert_config_ETCDStorage_To_v1alpha1_ETCDStorage is an autogenerated conversion function.
func Convert_config_ETCDStorage_To_v1alpha1_ETCDStorage(in *config.ETCDStorage, out *ETCDStorage, s conversion.Scope) error {
	return autoConvert_config_ETCDStorage_To_v1alpha1_ETCDStorage(in, out, s)
}
