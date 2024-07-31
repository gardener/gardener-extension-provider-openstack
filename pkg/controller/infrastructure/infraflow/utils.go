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
	if len(selector) == 0 {
		if len(found) > 1 {
			return nil, fmt.Errorf("%w: found matches: %v", ErrorMultipleMatches, found)
		}
		return found[0], nil
	}

	var res *T
	for _, item := range found {
		if selector[0](item) {
			if res != nil {
				return nil, fmt.Errorf("%w: found matches: %v, %v", ErrorMultipleMatches, res, item)
			}
			res = item
		}
	}
	return res, nil
}

func sliceToPtr[T any](slice []T) []*T {
	res := make([]*T, 0)
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
