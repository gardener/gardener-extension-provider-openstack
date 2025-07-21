// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

type actuator struct {
	backupbucket.Actuator
	client client.Client
}

func newActuator(mgr manager.Manager) backupbucket.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	openstackClient, err := openstackclient.NewStorageClientFromSecretRef(ctx, a.client, bb.Spec.SecretRef, bb.Spec.Region)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	return util.DetermineError(openstackClient.CreateContainerIfNotExists(ctx, bb.Name), helper.KnownCodes)
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	openstackClient, err := openstackclient.NewStorageClientFromSecretRef(ctx, a.client, bb.Spec.SecretRef, bb.Spec.Region)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	return util.DetermineError(openstackClient.DeleteContainerIfExists(ctx, bb.Name), helper.KnownCodes)
}
