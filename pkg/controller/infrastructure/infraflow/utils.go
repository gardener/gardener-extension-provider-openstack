// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"
)

// ErrorMultipleMatches is returned when the findExisting finds multiple resources matching a name.
var ErrorMultipleMatches = fmt.Errorf("error multiple matches")

func findExisting[T any](ctx context.Context, id *string, name string,
	getter func(ctx context.Context, id string) (*T, error),
	finder func(ctx context.Context, name string) ([]*T, error)) (*T, error) {
	if id != nil {
		found, err := getter(ctx, *id)
		if err != nil {
			return nil, err
		}
		if found != nil {
			return found, nil
		}
	}

	found, err := finder(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	if len(found) > 1 {
		return nil, fmt.Errorf("%w: found %d matches for name %q", ErrorMultipleMatches, len(found), name)
	}
	return found[0], nil
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
