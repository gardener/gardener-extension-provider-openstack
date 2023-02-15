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

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
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

	err = removeBastionInstance(log, computeClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove bastion instance: %w", err), helper.KnownCodes)
	}

	err = removePublicIPAddress(log, networkingClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove public ip address: %w", err), helper.KnownCodes)
	}

	deleted, err := isInstanceDeleted(computeClient, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to check for bastion instance: %w", err), helper.KnownCodes)
	}

	if !deleted {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: 10 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	return util.DetermineError(removeSecurityGroup(networkingClient, opt), helper.KnownCodes)
}

func removeBastionInstance(log logr.Logger, client openstackclient.Compute, opt *Options) error {
	instances, err := getBastionInstance(client, opt.BastionInstanceName)
	if openstackclient.IgnoreNotFoundError(err) != nil {
		return err
	}

	if len(instances) == 0 {
		return nil
	}

	err = deleteBastionInstance(client, instances[0].ID)
	if err != nil {
		return fmt.Errorf("failed to terminate bastion instance: %w", err)
	}

	log.Info("Instance removed", "instance", opt.BastionInstanceName)
	return nil
}

func removePublicIPAddress(log logr.Logger, client openstackclient.Networking, opt *Options) error {
	fips, err := getFipByName(client, opt.BastionInstanceName)
	if err != nil {
		return err
	}

	if len(fips) == 0 {
		return nil
	}

	err = deleteFloatingIP(client, fips[0].ID)
	if err != nil {
		return fmt.Errorf("failed to terminate bastion Public IP: %w", err)
	}

	log.Info("Public IP removed", "public IP ID", fips[0].ID)
	return nil
}

func removeSecurityGroup(client openstackclient.Networking, opt *Options) error {
	bastionSecurityGroups, err := getSecurityGroups(client, opt.SecurityGroup)
	if err != nil {
		return err
	}

	if len(bastionSecurityGroups) == 0 {
		return nil
	}

	return deleteSecurityGroup(client, bastionSecurityGroups[0].ID)
}

func isInstanceDeleted(client openstackclient.Compute, opt *Options) (bool, error) {
	instances, err := getBastionInstance(client, opt.BastionInstanceName)
	if openstackclient.IgnoreNotFoundError(err) != nil {
		return false, err
	}

	if len(instances) == 0 {
		return true, err
	}

	return false, err
}
