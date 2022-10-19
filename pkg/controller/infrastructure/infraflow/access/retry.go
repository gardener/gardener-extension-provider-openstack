// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package access

import (
	"encoding/json"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
)

// following https://github.com/terraform-provider-openstack/terraform-provider-openstack/blob/cec35ae29769b4de7d84980b1335a2b723ffb15f/openstack/networking_v2_shared.go

type neutronErrorWrap struct {
	NeutronError neutronError
}

type neutronError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Detail  string `json:"detail"`
}

func retryOn409(log logr.Logger, err error) bool {
	switch err := err.(type) {
	case gophercloud.ErrDefault409:
		neutronError, e := decodeNeutronError(err.ErrUnexpectedResponseCode.Body)
		if e != nil {
			// retry, when error type cannot be detected
			log.Info("[DEBUG] failed to decode a neutron error", "error", e)
			return true
		}
		if neutronError.Type == "IpAddressGenerationFailure" {
			return true
		}

		// don't retry on quota or other errors
		return false
	case gophercloud.ErrDefault400:
		neutronError, e := decodeNeutronError(err.ErrUnexpectedResponseCode.Body)
		if e != nil {
			// retry, when error type cannot be detected
			log.Info("[DEBUG] failed to decode a neutron error", "error", e)
			return true
		}
		if neutronError.Type == "ExternalIpAddressExhausted" {
			return true
		}

		// don't retry on quota or other errors
		return false
	case gophercloud.ErrDefault404: // this case is handled mostly for functional tests
		return true
	}

	return false
}

func decodeNeutronError(body []byte) (*neutronError, error) {
	e := &neutronErrorWrap{}
	if err := json.Unmarshal(body, e); err != nil {
		return nil, err
	}

	return &e.NeutronError, nil
}
