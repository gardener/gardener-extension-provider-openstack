// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"context"

	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
)

// Get will return an application credential based a given id and on the id of
// the parent user/application credential to which the application credential belongs to.
func (c *IdentityClient) GetApplicationCredential(ctx context.Context, parentUserID, applicationCredentialID string) (*applicationcredentials.ApplicationCredential, error) {
	return applicationcredentials.Get(c.client, parentUserID, applicationCredentialID).Extract()
}

// ListApplicationCredentials lists all application credentials based on a given id of
// the parent user/application credential to which the application credential belongs to.
func (c *IdentityClient) ListApplicationCredentials(ctx context.Context, parentUserID string) ([]applicationcredentials.ApplicationCredential, error) {
	page, err := applicationcredentials.List(c.client, parentUserID, applicationcredentials.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}
	return applicationcredentials.ExtractApplicationCredentials(page)
}

// CreateApplicationCredential creates an application credential belonging to
// a given parent user/application credential with a randomized name based on a given cluster name.
func (c *IdentityClient) CreateApplicationCredential(ctx context.Context, parentUserID, name, description, expirationTime string) (*applicationcredentials.ApplicationCredential, error) {
	return applicationcredentials.Create(c.client, parentUserID, applicationcredentials.CreateOpts{
		Name:         name,
		Description:  description,
		Unrestricted: false,
		ExpiresAt:    expirationTime,
	}).Extract()
}

// Delete will delete an application credential based a given id and on the id of
// the parent user/application credential to which the application credential belongs to.
func (c *IdentityClient) DeleteApplicationCredential(ctx context.Context, parentUserID, applicationCredentialID string) error {
	return applicationcredentials.Delete(c.client, parentUserID, applicationCredentialID).ExtractErr()
}

// LookupClientUserID will try to lookup the id of the user that configure the identity client.
func (c *IdentityClient) LookupClientUserID() (string, error) {
	result := tokens.Get(c.client, c.client.Token())
	if result.Err != nil {
		return "", result.Err
	}

	user, err := result.ExtractUser()
	if err != nil {
		return "", err
	}

	return user.ID, nil
}

// GetClientUser return information about the keystone user which is used
// to configure the identiy client. This will contain information like
// the user id, name and the domain the user is associated to.
func (c *IdentityClient) GetClientUser() (*tokens.User, error) {
	token := tokens.Get(c.client, c.client.Token())
	if token.Err != nil {
		return nil, token.Err
	}

	return token.ExtractUser()
}
