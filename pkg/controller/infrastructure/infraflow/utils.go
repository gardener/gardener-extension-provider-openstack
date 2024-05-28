// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"fmt"
)

// ErrorMultipleMatches is returned when the findExisting finds multiple resources matching a name.
var ErrorMultipleMatches = fmt.Errorf("error multiple matches")

func findExisting[T any](id *string, name string,
	getter func(id string) (*T, error),
	finder func(name string) ([]*T, error),
	selector ...func(item *T) bool) (*T, error) {

	if id != nil {
		found, err := getter(*id)
		if err != nil {
			return nil, err
		}
		if found != nil && (len(selector) == 0 || selector[0](found)) {
			return found, nil
		}
	}

	found, err := finder(name)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}

	// TODO: check if this makes sense
	if len(found) > 1 {
		return nil, ErrorMultipleMatches
	}

	if len(selector) > 0 {
		for _, item := range found {
			if selector[0](item) {
				return item, nil
			}
		}
		return nil, nil
	}
	return found[0], nil
}

func copyMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := map[string]string{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func sliceToPtr[T any](slice []T) []*T {
	res := make([]*T, len(slice))
	for _, t := range slice {
		res = append(res, &t)
	}
	return res
}

func (fctx *FlowContext) defaultRouterName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSSHKeypairName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultNetworkName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSubnetName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSecurityGroupName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSharedNetworkName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) workerCIDR() string {
	s := fctx.config.Networks.Worker
	if workers := fctx.config.Networks.Workers; workers != "" {
		s = workers
	}

	return s
}
