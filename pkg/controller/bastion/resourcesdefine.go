// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
)

// IngressAllowSSH ingress allow ssh
func IngressAllowSSH(opts Options, etherType rules.RuleEtherType, secGroupID, cidr, remoteGroupID string) rules.CreateOpts {
	return rules.CreateOpts{
		Direction:      "ingress",
		Description:    ingressAllowSSHResourceName(opts.BastionInstanceName),
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
func EgressAllowSSHToWorker(opts Options, secGroupID string, remoteGroupID string) rules.CreateOpts {
	return rules.CreateOpts{
		Direction:     "egress",
		Description:   egressAllowOnlyResourceName(opts.BastionInstanceName),
		PortRangeMin:  sshPort,
		EtherType:     rules.EtherType4,
		PortRangeMax:  sshPort,
		Protocol:      "tcp",
		SecGroupID:    secGroupID,
		RemoteGroupID: remoteGroupID,
	}
}
