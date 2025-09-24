// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// Name is the name of the OpenStack provider.
	Name = "provider-openstack"

	// CloudControllerManagerImageName is the name of the cloud-controller-manager image.
	CloudControllerManagerImageName = "cloud-controller-manager"
	// CSIDriverCinderImageName is the name of the csi-driver-cinder image.
	CSIDriverCinderImageName = "csi-driver-cinder"
	// CSIDriverManilaImageName is the name of the csi-driver-manila image.
	CSIDriverManilaImageName = "csi-driver-manila"
	// CSIDriverNFSImageName is the name of the csi-driver-nfs image.
	CSIDriverNFSImageName = "csi-driver-nfs"
	// CSIProvisionerImageName is the name of the csi-provisioner image.
	CSIProvisionerImageName = "csi-provisioner"
	// CSIAttacherImageName is the name of the csi-attacher image.
	CSIAttacherImageName = "csi-attacher"
	// CSIDiskDriverTopologyKey is the label on persistent volumes that represents availability by zone.
	// See https://github.com/kubernetes/cloud-provider-openstack/blob/master/examples/cinder-csi-plugin/topology/example.yaml
	// See https://gitlab.cern.ch/cloud/cloud-provider-openstack/-/blob/release-1.19/docs/using-cinder-csi-plugin.md#enable-topology-aware-dynamic-provisioning-for-cinder-volumes
	CSIDiskDriverTopologyKey = "topology.cinder.csi.openstack.org/zone"
	// CSIManilaDriverTopologyKey is the label on persistent volumes that represents availability by zone.
	CSIManilaDriverTopologyKey = "topology.manila.csi.openstack.org/zone"
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
	// MachineControllerManagerProviderOpenStackImageName is the name of the MachineControllerManager OpenStack image.
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
	// CACert is a constant for the key in a cloud provider secret that configures the CA bundle used to verify the server's certificate.
	CACert = "caCert"

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
	// DNSApplicationCredentialID is a constant for the key in a DNS secret hat holds the OpenStack application credential id.
	DNSApplicationCredentialID = "OS_APPLICATION_CREDENTIAL_ID"
	// DNSApplicationCredentialName is a constant for the key in a DNS secret  that holds the OpenStack application credential name.
	DNSApplicationCredentialName = "OS_APPLICATION_CREDENTIAL_NAME"
	// DNSApplicationCredentialSecret is a constant for the key in a DNS secret  that holds the OpenStack application credential secret.
	DNSApplicationCredentialSecret = "OS_APPLICATION_CREDENTIAL_SECRET"
	// DNS_CA_Bundle is a constant for the key in a DNS secret that holds the Openstack CA Bundle for the KeyStone server.
	DNS_CA_Bundle = "OS_CACERT"

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
	// CSIControllerName is a constant for the chart name for a CSI Cinder controller deployment in the seed.
	CSIControllerName = "csi-driver-controller"
	// CSIManilaControllerName is a constant for the chart name for a CSI Manila controller deployment in the seed.
	CSIManilaControllerName = "csi-driver-manila-controller"
	// CSINFSControllerName is a constant for the chart name for a CSI NFS controller deployment in the shoot.
	CSINFSControllerName = "csi-driver-nfs-controller"
	// CSINodeName is a constant for the chart name for a CSI Cinder node deployment in the shoot.
	CSINodeName = "csi-driver-node"
	// CSIManilaNodeName is a constant for the chart name for a CSI Manila node deployment in the shoot.
	CSIManilaNodeName = "csi-driver-manila-node"
	// CSINFSNodeName is a constant for the chart name for a CSI NFS node deployment in the shoot.
	CSINFSNodeName = "csi-driver-nfs-node"
	// CSIDriverManila is a constant for the chart name for the CSI driver Manila deployment in the shoot.
	CSIDriverManila = "csi-driver-manila"
	// CSIDriverManilaController is a constant for the chart name for the CSI driver Manila / NFS controller deployment in the seed.
	CSIDriverManilaController = "csi-driver-manila-controller"
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
	// CSISnapshotControllerName is a constant for the name of the csi-snapshot-controller component.
	CSISnapshotControllerName = "csi-snapshot-controller"
	// CSIStorageProvisioner is a constant with the storage provisioner name which is used in storageclasses.
	CSIStorageProvisioner = "cinder.csi.openstack.org"
	// CSIManilaStorageProvisionerNFS is a constant with the storage provisioner name which is used in storageclasses for Manila NFS.
	CSIManilaStorageProvisionerNFS = "nfs.manila.csi.openstack.org"
	// CSIManilaNFS is a constant for CSI Manila NFS resource objects
	CSIManilaNFS = "csi-manila-nfs"
	// CSIManilaSecret is a constant for additional role/rolebiding for CSI manila plugin secret
	CSIManilaSecret = "csi-manila-secret" // #nosec G101 -- No credential.

	// CSISnapshotValidationName is the constant for the name of the csi-snapshot-validation-webhook component.
	// TODO(AndreasBurger): Clean up once SnapshotValidation is removed everywhere
	CSISnapshotValidationName = "csi-snapshot-validation"
)

var (
	// UsernamePrefix is a constant for the username prefix of components deployed by OpenStack.
	UsernamePrefix = extensionsv1alpha1.SchemeGroupVersion.Group + ":" + Name + ":"
)
