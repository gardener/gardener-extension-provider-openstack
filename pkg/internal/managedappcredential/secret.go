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

package managedappcredential

import (
	"context"
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	applicationCredentialSecretName = "cloudprovider-application-credential"

	applicationCredentialSecretCreationTime = "creationTime"
	applicationCredentialSecretParentID     = "parentID"
	applicationCredentialSecretParentName   = "parentName"
	applicationCredentialSecretParentSecret = "parentSecret"
)

func readApplicationCredential(ctx context.Context, client client.Client, namespace string) (*applicationCredential, error) {
	var secret = &corev1.Secret{}
	if err := client.Get(ctx, kutil.Key(namespace, applicationCredentialSecretName), secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	creationTimeRaw := readSecretKey(secret, applicationCredentialSecretCreationTime)
	creationTime, err := time.Parse(time.RFC3339, creationTimeRaw)
	if err != nil {
		return nil, err
	}

	return &applicationCredential{
		id:           readSecretKey(secret, openstack.ApplicationCredentialID),
		name:         readSecretKey(secret, openstack.ApplicationCredentialName),
		password:     readSecretKey(secret, openstack.ApplicationCredentialSecret),
		creationTime: creationTime,
		secret:       secret,
	}, nil
}

func (m *Manager) storeApplicationCredential(ctx context.Context, appCredential *applicationCredential, parent *parent) error {
	secretData := map[string][]byte{
		openstack.ApplicationCredentialID:       []byte(appCredential.id),
		openstack.ApplicationCredentialName:     []byte(appCredential.name),
		openstack.ApplicationCredentialSecret:   []byte(appCredential.password),
		applicationCredentialSecretCreationTime: []byte(time.Now().UTC().Format(time.RFC3339)),
	}

	for k, v := range parent.toSecretData() {
		secretData[k] = v
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       applicationCredentialSecretName,
			Namespace:  m.namespace,
			Finalizers: []string{m.finalizerName},
		},
		Data: secretData,
	}

	if err := m.client.Update(ctx, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		if err := m.client.Create(ctx, secret); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) removeApplicationCredentialStore(ctx context.Context, secret *corev1.Secret) error {
	patch := client.MergeFrom(secret.DeepCopy())
	secret.ObjectMeta.Finalizers = []string{}
	if err := m.client.Patch(ctx, secret, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := m.client.Delete(ctx, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (m *Manager) updateParentPasswordIfRequired(ctx context.Context, appCredential *applicationCredential, newParent *parent) error {
	var (
		oldParentSecret = string(appCredential.secret.Data[applicationCredentialSecretParentSecret])
		oldParentID     = string(appCredential.secret.Data[applicationCredentialSecretParentID])
	)

	if oldParentID != newParent.id {
		return nil
	}

	if oldParentSecret == newParent.secret {
		return nil
	}

	patch := client.MergeFrom(appCredential.secret.DeepCopy())
	appCredential.secret.Data[applicationCredentialSecretParentSecret] = []byte(newParent.secret)

	return m.client.Patch(ctx, appCredential.secret, patch)
}

func readSecretKey(secret *corev1.Secret, key string) string {
	return string(secret.Data[key])
}
