// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2"
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

func retryOnError(log logr.Logger, err error) bool {
	var unexpectedErr gophercloud.ErrUnexpectedResponseCode
	if errors.As(err, &unexpectedErr) {
		switch unexpectedErr.Actual {
		case http.StatusConflict:
			neutronError, e := decodeNeutronError(unexpectedErr.Body)
			if e != nil {
				// retry, when error type cannot be detected
				log.V(1).Info("[DEBUG] failed to decode a neutron error", "error", e)
				return true
			}
			if neutronError.Type == "IpAddressGenerationFailure" {
				return true
			}

			// don't retry on quota or other errors
			return false
		case http.StatusBadRequest:
			neutronError, e := decodeNeutronError(unexpectedErr.Body)
			if e != nil {
				// retry, when error type cannot be detected
				log.V(1).Info("[DEBUG] failed to decode a neutron error", "error", e)
				return true
			}
			if neutronError.Type == "ExternalIpAddressExhausted" {
				return true
			}

			// don't retry on quota or other errors
			return false
		case http.StatusNotFound:
			return true
		}
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
