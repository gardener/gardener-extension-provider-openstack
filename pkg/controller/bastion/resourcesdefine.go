// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
