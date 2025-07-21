// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Terraform", func() {
	Describe("#TerraformerEnvVars", func() {
		It("should correctly create the environment variables for username/password", func() {
			secretRef := corev1.SecretReference{Name: "cloud"}
			credentials := &openstack.Credentials{}
			Expect(TerraformerEnvVars(secretRef, credentials)).To(ConsistOf(
				corev1.EnvVar{
					Name: "TF_VAR_DOMAIN_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "domainName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_TENANT_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "tenantName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_USER_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "username",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "password",
					}},
				}))
		})
		It("should correctly create the environment variables for application credentials (id + secret)", func() {
			secretRef := corev1.SecretReference{Name: "cloud"}
			credentials := &openstack.Credentials{
				ApplicationCredentialSecret: "secret",
				ApplicationCredentialID:     "appid",
			}
			Expect(TerraformerEnvVars(secretRef, credentials)).To(ConsistOf(
				corev1.EnvVar{
					Name: "TF_VAR_DOMAIN_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "domainName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_TENANT_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "tenantName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_APPLICATION_CREDENTIAL_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "applicationCredentialID",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_APPLICATION_CREDENTIAL_SECRET",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "applicationCredentialSecret",
					}},
				}))
		})
		It("should correctly create the environment variables for application credentials (username + name + secret)", func() {
			secretRef := corev1.SecretReference{Name: "cloud"}
			credentials := &openstack.Credentials{
				Username:                    "fred",
				ApplicationCredentialName:   "appname",
				ApplicationCredentialSecret: "secret",
			}
			Expect(TerraformerEnvVars(secretRef, credentials)).To(ConsistOf(
				corev1.EnvVar{
					Name: "TF_VAR_DOMAIN_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "domainName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_TENANT_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "tenantName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_APPLICATION_CREDENTIAL_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "applicationCredentialName",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_APPLICATION_CREDENTIAL_SECRET",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "applicationCredentialSecret",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_USER_NAME",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "username",
					}},
				}))
		})
	})
})
