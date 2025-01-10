// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	opt, err := DetermineOptions(bastion, cluster)
	if err != nil {
		return err
	}

	credentials, err := openstack.GetCredentials(ctx, a.client, opt.SecretReference, false)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}

	openstackClientFactory, err := a.openstackClientFactory.NewFactory(credentials)
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create openstack client factory: %w", err), helper.KnownCodes)
	}

	computeClient, err := openstackClientFactory.Compute()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	networkingClient, err := openstackClientFactory.Networking()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = removeBastionInstance(ctx, log, computeClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove bastion instance: %w", err), helper.KnownCodes)
	}

	err = removePublicIPAddress(ctx, log, networkingClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove public ip address: %w", err), helper.KnownCodes)
	}

	deleted, err := isInstanceDeleted(ctx, computeClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to check for bastion instance: %w", err), helper.KnownCodes)
	}

	if !deleted {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: 10 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	// The ssh ingress rule for the bastion in the worker node security group was also deleted once the bastion security group was removed. Therefore, there's no need to manage its deletion.
	return util.DetermineError(removeSecurityGroup(ctx, networkingClient, opt), helper.KnownCodes)
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Bastion, _ *controller.Cluster) error {
	return nil
}

func removeBastionInstance(ctx context.Context, log logr.Logger, client openstackclient.Compute, opt *Options) error {
	instances, err := getBastionInstance(ctx, client, opt.BastionInstanceName)
	if openstackclient.IgnoreNotFoundError(err) != nil {
		return err
	}

	if len(instances) == 0 {
		return nil
	}

	err = deleteBastionInstance(ctx, client, instances[0].ID)
	if err != nil {
		return fmt.Errorf("failed to terminate bastion instance: %w", err)
	}

	log.Info("Instance removed", "instance", opt.BastionInstanceName)
	return nil
}

func removePublicIPAddress(ctx context.Context, log logr.Logger, client openstackclient.Networking, opt *Options) error {
	fips, err := getFipByName(ctx, client, opt.BastionInstanceName)
	if err != nil {
		return err
	}

	if len(fips) == 0 {
		return nil
	}

	err = deleteFloatingIP(ctx, client, fips[0].ID)
	if err != nil {
		return fmt.Errorf("failed to terminate bastion Public IP: %w", err)
	}

	log.Info("Public IP removed", "public IP ID", fips[0].ID)
	return nil
}

func removeSecurityGroup(ctx context.Context, client openstackclient.Networking, opt *Options) error {
	bastionSecurityGroups, err := getSecurityGroups(ctx, client, opt.SecurityGroup)
	if err != nil {
		return err
	}

	if len(bastionSecurityGroups) == 0 {
		return nil
	}

	return deleteSecurityGroup(ctx, client, bastionSecurityGroups[0].ID)
}

func isInstanceDeleted(ctx context.Context, client openstackclient.Compute, opt *Options) (bool, error) {
	instances, err := getBastionInstance(ctx, client, opt.BastionInstanceName)
	if openstackclient.IgnoreNotFoundError(err) != nil {
		return false, err
	}

	if len(instances) == 0 {
		return true, err
	}

	return false, err
}
