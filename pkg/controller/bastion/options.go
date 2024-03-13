// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"

	"github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	corev1 "k8s.io/api/core/v1"
)

const (
	//maxLengthForBaseName for "base" name due to fact that we use this name to name other openstack resources,
	maxLengthForBaseName = 33
)

// Options contains provider-related information required for setting up
// a bastion instance. This struct combines precomputed values like the
// bastion instance name with the IDs of pre-existing cloud provider
// resources, like the nic name, subnet name etc.
type Options struct {
	BastionInstanceName string
	Region              string
	ShootName           string
	SecretReference     corev1.SecretReference
	SecurityGroup       string
	UserData            []byte
}

// DetermineOptions determines the required information that are required to reconcile a Bastion on Openstack. This
// function does not create any IaaS resources.
func DetermineOptions(bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) (*Options, error) {
	clusterName := cluster.ObjectMeta.Name
	region := cluster.Shoot.Spec.Region

	baseResourceName, err := generateBastionBaseResourceName(clusterName, bastion.Name)
	if err != nil {
		return nil, err
	}

	secretReference := corev1.SecretReference{
		Namespace: clusterName,
		Name:      v1beta1constants.SecretNameCloudProvider,
	}

	return &Options{
		ShootName:           clusterName,
		BastionInstanceName: baseResourceName,
		SecretReference:     secretReference,
		SecurityGroup:       securityGroupName(baseResourceName),
		Region:              region,
		UserData:            []byte(base64.StdEncoding.EncodeToString(bastion.Spec.UserData)),
	}, nil
}

func generateBastionBaseResourceName(clusterName string, bastionName string) (string, error) {
	if clusterName == "" {
		return "", fmt.Errorf("clusterName can't be empty")
	}
	if bastionName == "" {
		return "", fmt.Errorf("bastionName can't be empty")
	}

	staticName := clusterName + "-" + bastionName
	h := sha256.New()
	_, err := h.Write([]byte(staticName))
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	if len([]rune(staticName)) > maxLengthForBaseName {
		staticName = staticName[:maxLengthForBaseName]
	}
	return fmt.Sprintf("%s-bastion-%s", staticName, hash[:5]), nil
}

// IngressPermission hold the IPv4 and IPv6 ranges that should be allowed to access the bastion.
type IngressPermission struct {
	// EtherType describes the rules.RuleEtherType of the CIDR.
	EtherType rules.RuleEtherType

	// CIDR holds the IPv4 or IPv6 range, depending on EtherType.
	CIDR string
}

func ingressPermissions(bastion *extensionsv1alpha1.Bastion) ([]IngressPermission, error) {
	var perms []IngressPermission

	for _, ingress := range bastion.Spec.Ingress {
		cidr := ingress.IPBlock.CIDR
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid ingress CIDR %q: %w", cidr, err)
		}

		normalisedCIDR := ipNet.String()

		if ip.To4() != nil {
			perms = append(perms, IngressPermission{EtherType: rules.EtherType4, CIDR: normalisedCIDR})
		} else if ip.To16() != nil {
			perms = append(perms, IngressPermission{EtherType: rules.EtherType6, CIDR: normalisedCIDR})
		}

	}
	return perms, nil
}

// securityGroupName is Security Group resource name
func securityGroupName(baseName string) string {
	return fmt.Sprintf("%s-sg", baseName)
}

// ingressAllowSSHResourceName is Firewall ingress allow SSH rule resource name
func ingressAllowSSHResourceName(baseName string) string {
	return fmt.Sprintf("%s-allow-ssh", baseName)
}

// egressAllowOnlyResourceName is Security group egress allow only rule name
func egressAllowOnlyResourceName(baseName string) string {
	return fmt.Sprintf("%s-egress-worker", baseName)
}
