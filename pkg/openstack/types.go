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

package openstack

import (
	"path/filepath"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// Name is the name of the OpenStack provider.
	Name = "provider-openstack"

	// CloudControllerManagerImageName is the name of the cloud-controller-manager image.
	CloudControllerManagerImageName = "cloud-controller-manager"
	// CSIDriverCinderImageName is the name of the csi-driver-cinder image.
	CSIDriverCinderImageName = "csi-driver-cinder"
	// CSIProvisionerImageName is the name of the csi-provisioner image.
	CSIProvisionerImageName = "csi-provisioner"
	// CSIAttacherImageName is the name of the csi-attacher image.
	CSIAttacherImageName = "csi-attacher"
	// CSISnapshotterImageName is the name of the csi-snapshotter image.
	CSISnapshotterImageName = "csi-snapshotter"
	// CSIResizerImageName is the name of the csi-resizer image.
	CSIResizerImageName = "csi-resizer"
	// CSINodeDriverRegistrarImageName is the name of the csi-node-driver-registrar image.
	CSINodeDriverRegistrarImageName = "csi-node-driver-registrar"
	// CSILivenessProbeImageName is the name of the csi-liveness-probe image.
	CSILivenessProbeImageName = "csi-liveness-probe"
	// CSISnapshotControllerImageName is the name of the csi-snapshot-controller image.
	CSISnapshotControllerImageName = "csi-snapshot-controller"
	// MachineControllerManagerImageName is the name of the MachineControllerManager image.
	MachineControllerManagerImageName                  = "machine-controller-manager"
	MachineControllerManagerProviderOpenStackImageName = "machine-controller-manager-provider-openstack"

	// AuthURL is a constant for the key in a cloud provider secret that holds the OpenStack auth url.
	AuthURL = "authURL"
	// DomainName is a constant for the key in a cloud provider secret that holds the OpenStack domain name.
	DomainName = "domainName"
	// TenantName is a constant for the key in a cloud provider secret that holds the OpenStack tenant name.
	TenantName = "tenantName"
	// UserName is a constant for the key in a cloud provider secret and backup secret that holds the OpenStack username.
	UserName = "username"
	// Password is a constant for the key in a cloud provider secret and backup secret that holds the OpenStack password.
	Password = "password"
	// ApplicationCredentialID is a constant for the key in a cloud provider secret and backup secret that holds the OpenStack application credential id.
	ApplicationCredentialID = "applicationCredentialID"
	// ApplicationCredentialName is a constant for the key in a cloud provider secret and backup secret that holds the OpenStack application credential name.
	ApplicationCredentialName = "applicationCredentialName"
	// ApplicationCredentialSecret is a constant for the key in a cloud provider secret and backup secret that holds the OpenStack application credential secret.
	ApplicationCredentialSecret = "applicationCredentialSecret"
	// Region is a constant for the key in a backup secret that holds the Openstack region.
	Region = "region"
	// Insecure is a constant for the key in a cloud provider secret that configures whether the OpenStack client verifies the server's certificate.
	Insecure = "insecure"

	// DNSAuthURL is a constant for the key in a DNS secret that holds the OpenStack auth url.
	DNSAuthURL = "OS_AUTH_URL"
	// DNSDomainName is a constant for the key in a DNS secret that holds the OpenStack domain name.
	DNSDomainName = "OS_DOMAIN_NAME"
	// DNSTenantName is a constant for the key in a DNS secret that holds the OpenStack tenant name.
	DNSTenantName = "OS_PROJECT_NAME"
	// DNSUserName is a constant for the key in a DNS secret that holds the OpenStack username.
	DNSUserName = "OS_USERNAME"
	// DNSPassword is a constant for the key in a DNS secret that holds the OpenStack password.
	DNSPassword = "OS_PASSWORD"

	// CloudProviderConfigName is the name of the secret containing the cloud provider config.
	CloudProviderConfigName = "cloud-provider-config"
	// CloudProviderDiskConfigName is the name of the secret containing the cloud provider config for disk/volume handling. It is used by kube-controller-manager.
	CloudProviderDiskConfigName = "cloud-provider-disk-config"
	// CloudProviderCSIDiskConfigName is the name of the secret containing the cloud provider config for disk/volume handling. It is used by csi-driver-controller.
	CloudProviderCSIDiskConfigName = "cloud-provider-disk-config-csi"
	// CloudProviderConfigDataKey is the key storing the cloud provider config as value in the cloud provider secret.
	CloudProviderConfigDataKey = "cloudprovider.conf"
	// CloudControllerManagerName is a constant for the name of the CloudController deployed by the worker controller.
	CloudControllerManagerName = "cloud-controller-manager"
	// CSIControllerName is a constant for the chart name for a CSI controller deployment in the seed.
	CSIControllerName = "csi-driver-controller"
	// CSIControllerCinderName is a constant for the name of the Cinder CSI controller deployment in the seed.
	CSIControllerCinderName = "csi-driver-controller-cinder"
	// CSINodeName is a constant for the chart name for a CSI node deployment in the shoot.
	CSINodeName = "csi-driver-node"
	// CSINodeCinderName is a constant for the name of the Cinder CSI node deployment in the shoot.
	CSINodeCinderName = "csi-driver-node-cinder"
	// CSIDriverName is a constant for the name of the csi-driver component.
	CSIDriverName = "csi-driver"
	// CSIProvisionerName is a constant for the name of the csi-provisioner component.
	CSIProvisionerName = "csi-provisioner"
	// CSIAttacherName is a constant for the name of the csi-attacher component.
	CSIAttacherName = "csi-attacher"
	// CSISnapshotterName is a constant for the name of the csi-snapshotter component.
	CSISnapshotterName = "csi-snapshotter"
	// CSIResizerName is a constant for the name of the csi-resizer component.
	CSIResizerName = "csi-resizer"
	// CSINodeDriverRegistrarName is a constant for the name of the csi-node-driver-registrar component.
	CSINodeDriverRegistrarName = "csi-node-driver-registrar"
	// CSILivenessProbeName is a constant for the name of the csi-liveness-probe component.
	CSILivenessProbeName = "csi-liveness-probe"
	// CSISnapshotControllerName is a constant for the name of the csi-snapshot-controller component.
	CSISnapshotControllerName = "csi-snapshot-controller"
	// MachineControllerManagerName is a constant for the name of the machine-controller-manager.
	MachineControllerManagerName = "machine-controller-manager"
	// MachineControllerManagerVpaName is the name of the VerticalPodAutoscaler of the machine-controller-manager deployment.
	MachineControllerManagerVpaName = "machine-controller-manager-vpa"
	// MachineControllerManagerMonitoringConfigName is the name of the ConfigMap containing monitoring stack configurations for machine-controller-manager.
	MachineControllerManagerMonitoringConfigName = "machine-controller-manager-monitoring-config"
)

var (
	// ChartsPath is the path to the charts
	ChartsPath = filepath.Join("charts")
	// InternalChartsPath is the path to the internal charts
	InternalChartsPath = filepath.Join(ChartsPath, "internal")

	// UsernamePrefix is a constant for the username prefix of components deployed by OpenStack.
	UsernamePrefix = extensionsv1alpha1.SchemeGroupVersion.Group + ":" + Name + ":"
)
