// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"net/http"
	"os"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/containers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackext "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

func secretsFromEnv() {
	if len(*authURL) == 0 {
		authURL = ptr.To(os.Getenv("AUTH_URL"))
	}
	if len(*domainName) == 0 {
		domainName = ptr.To(os.Getenv("DOMAIN_NAME"))
	}
	if len(*tenantName) == 0 {
		tenantName = ptr.To(os.Getenv("TENANT_NAME"))
	}
	if len(*userName) == 0 {
		userName = ptr.To(os.Getenv("USER_NAME"))
	}
	if len(*password) == 0 {
		password = ptr.To(os.Getenv("PASSWORD"))
	}
	if len(*region) == 0 {
		region = ptr.To(os.Getenv("REGION"))
	}
	if len(*appID) == 0 {
		appID = ptr.To(os.Getenv("APP_ID"))
	}
	if len(*appName) == 0 {
		appName = ptr.To(os.Getenv("APP_NAME"))
	}
	if len(*appSecret) == 0 {
		appSecret = ptr.To(os.Getenv("APP_SECRET"))
	}
}

func validateFlags() {
	if len(*authURL) == 0 {
		panic("OpenStack auth URL required. Either provide it via the auth-url flag or set the AUTH_URL environment variable")
	}
	if len(*domainName) == 0 {
		panic("OpenStack domain name required. Either provide it via the domain-name flag or set the DOMAIN_NAME environment variable")
	}
	if len(*tenantName) == 0 {
		panic("OpenStack tenant name required. Either provide it via the tenant-name flag or set the TENANT_NAME environment variable")
	}
	if len(*userName) == 0 && len(*appID) == 0 {
		panic("Either OpenStack user name or application credential ID required. Provide one via the respective flags or environment variables")
	}
	if len(*password) == 0 && len(*appSecret) == 0 {
		panic("Either OpenStack password or application credential secret required. Provide one via the respective flags or environment variables")
	}
	if len(*region) == 0 {
		panic("OpenStack region required. Either provide it via the region flag or set the REGION environment variable")
	}
	if len(*logLevel) == 0 {
		logLevel = ptr.To("debug")
	} else {
		if *logLevel != "debug" && *logLevel != "info" && *logLevel != "error" {
			panic("Invalid log level: " + *logLevel)
		}
	}
}

func createNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Creating namespace", "namespace", namespace.Name)
	Expect(c.Create(ctx, namespace)).To(Succeed(), "Failed to create namespace: %s", namespace.Name)
}

func deleteNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Deleting namespace", "namespace", namespace.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func ensureGardenNamespace(ctx context.Context, c client.Client) (*corev1.Namespace, bool) {
	gardenNamespaceAlreadyExists := false
	gardenNamespace := &corev1.Namespace{}
	err := c.Get(ctx, client.ObjectKey{Name: gardenNamespaceName}, gardenNamespace)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("Garden namespace not found, creating it", "namespace", gardenNamespaceName)
			gardenNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenNamespaceName,
				},
			}
			Expect(c.Create(ctx, gardenNamespace)).To(Succeed(), "Failed to create garden namespace")
		} else {
			log.Error(err, "Failed to check for garden namespace")
			Expect(err).NotTo(HaveOccurred(), "Unexpected error while checking for garden namespace")
		}
	} else {
		gardenNamespaceAlreadyExists = true
		log.Info("Garden namespace already exists", "namespace", gardenNamespaceName)
	}
	return gardenNamespace, gardenNamespaceAlreadyExists
}

func createBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Creating secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(c.Create(ctx, secret)).To(Succeed(), "Failed to create secret: %s", secret.Name)
}

func deleteBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Deleting secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
}

func createBackupBucket(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	log.Info("Creating backupBucket", "backupBucket", backupBucket)
	Expect(c.Create(ctx, backupBucket)).To(Succeed(), "Failed to create backupBucket: %s", backupBucket.Name)
}

func deleteBackupBucket(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	log.Info("Deleting backupBucket", "backupBucket", backupBucket)
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
}

func waitUntilBackupBucketReady(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		backupBucket,
		v1alpha1.BackupBucketResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)).To(Succeed(), "BackupBucket did not become ready: %s", backupBucket.Name)
	log.Info("BackupBucket is ready", "backupBucket", backupBucket)
}

func waitUntilBackupBucketDeleted(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		backupBucket.DeepCopy(),
		v1alpha1.BackupBucketResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
	log.Info("BackupBucket successfully deleted", "backupBucket", backupBucket)
}

func getBackupBucketAndVerifyStatus(ctx context.Context, c client.Client, backupBucket *v1alpha1.BackupBucket) {
	log.Info("Verifying backupBucket", "backupBucket", backupBucket)
	Expect(c.Get(ctx, client.ObjectKey{Name: backupBucket.Name}, backupBucket)).To(Succeed())

	By("verifying LastOperation state")
	Expect(backupBucket.Status.LastOperation).NotTo(BeNil(), "LastOperation should not be nil")
	Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded), "LastOperation state should be Succeeded")
	Expect(backupBucket.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate), "LastOperation type should be Create")

	By("verifying GeneratedSecretRef")
	if backupBucket.Status.GeneratedSecretRef != nil {
		Expect(backupBucket.Status.GeneratedSecretRef.Name).NotTo(BeEmpty(), "GeneratedSecretRef name should not be empty")
		Expect(backupBucket.Status.GeneratedSecretRef.Namespace).NotTo(BeEmpty(), "GeneratedSecretRef namespace should not be empty")
	}
}

func verifyBackupBucket(ctx context.Context, storageClient *gophercloud.ServiceClient, backupBucket *v1alpha1.BackupBucket) {
	opts := containers.GetOpts{Newest: true}
	_, err := containers.Get(ctx, storageClient, backupBucket.Name, opts).Extract()
	Expect(err).NotTo(HaveOccurred(), "Failed to verify OpenStack Swift container existence", "containerName", backupBucket.Name)
}

func verifyBackupBucketDeleted(ctx context.Context, storageClient *gophercloud.ServiceClient, backupBucket *v1alpha1.BackupBucket) {
	opts := containers.GetOpts{Newest: true}
	_, err := containers.Get(ctx, storageClient, backupBucket.Name, opts).Extract()
	Expect(gophercloud.ResponseCodeIs(err, http.StatusNotFound)).To(BeTrue(),
		"Expected 404 error when verifying OpenStack Swift container deletion",
		"containerName", backupBucket.Name)
}

func newBackupBucket(name string, region string) *v1alpha1.BackupBucket {
	return &v1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.BackupBucketSpec{
			DefaultSpec: v1alpha1.DefaultSpec{
				Type: openstackext.Type,
			},
			Region: region,
			SecretRef: corev1.SecretReference{
				Name:      backupBucketSecretName,
				Namespace: name,
			},
		},
	}
}

func randomString() string {
	rs, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred(), "Failed to generate random string")
	log.Info("Generated random string", "randomString", rs)
	return rs
}

// createProviderClient creates a provider client for OpenStack
func createProviderClient(ctx context.Context, authURL, username, password, tenantName, domainName string) *gophercloud.ProviderClient {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		TenantName:       tenantName,
		DomainName:       domainName,
	}

	provider, err := openstack.AuthenticatedClient(ctx, opts)
	Expect(err).NotTo(HaveOccurred(), "Failed to create OpenStack provider client", "Error", err)

	return provider
}

// createStorageClient creates an Object Storage client for a specific region
func createStorageClient(provider *gophercloud.ProviderClient, region string) *gophercloud.ServiceClient {
	client, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to create OpenStack Object Storage client", "Error", err)

	return client
}
