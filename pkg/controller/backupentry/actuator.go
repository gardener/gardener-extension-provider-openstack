// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) genericactuator.BackupEntryDelegate {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) GetETCDSecretData(_ context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry, backupSecretData map[string][]byte) (map[string][]byte, error) {
	backupSecretData[openstack.Region] = []byte(be.Spec.Region)
	return backupSecretData, nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	openstackClient, err := openstackclient.NewStorageClientFromSecretRef(ctx, a.client, be.Spec.SecretRef, be.Spec.Region)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	return util.DetermineError(openstackClient.DeleteObjectsWithPrefix(ctx, be.Spec.BucketName, fmt.Sprintf("%s/", be.Name)), helper.KnownCodes)
}
