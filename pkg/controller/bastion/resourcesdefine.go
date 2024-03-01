// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
)

// IngressAllowSSH ingress allow ssh
func IngressAllowSSH(opt *Options, etherType rules.RuleEtherType, secGroupID, cidr, remoteGroupID string) rules.CreateOpts {
	return rules.CreateOpts{
		Direction:      "ingress",
		Description:    ingressAllowSSHResourceName(opt.BastionInstanceName),
		PortRangeMin:   sshPort,
		EtherType:      etherType,
		PortRangeMax:   sshPort,
		Protocol:       "tcp",
		SecGroupID:     secGroupID,
		RemoteIPPrefix: cidr,
		RemoteGroupID:  remoteGroupID,
	}
}

// EgressAllowSSHToWorker egress allow ssh to worker
func EgressAllowSSHToWorker(opt *Options, secGroupID string, remoteGroupID string) rules.CreateOpts {
	return rules.CreateOpts{
		Direction:     "egress",
		Description:   egressAllowOnlyResourceName(opt.BastionInstanceName),
		PortRangeMin:  sshPort,
		EtherType:     rules.EtherType4,
		PortRangeMax:  sshPort,
		Protocol:      "tcp",
		SecGroupID:    secGroupID,
		RemoteGroupID: remoteGroupID,
	}
}
