// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"errors"
	"testing"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func legacyArtifacts() []client.Object {
	return []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "infrastructure.infra.tf-state"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "infrastructure.infra.tf-vars"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "infrastructure.infra.tf-config"}},
	}
}

func TestMigrateCleansLegacyArtifacts(t *testing.T) {
	const finalizer = "gardener.cloud/terraformer"
	for _, tc := range []struct {
		name          string
		finalizers    []string
		deleting      bool
		missingSecret bool
	}{
		{name: "with finalizers", finalizers: []string{finalizer}},
		{name: "without finalizers"},
		{name: "already deleting", finalizers: []string{finalizer}, deleting: true},
		{name: "partially cleaned up", finalizers: []string{finalizer}, missingSecret: true},
		{name: "preserves other finalizers", finalizers: []string{finalizer, "example.com/other-controller"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()
			c := fakeclient.NewClientBuilder().Build()
			objects := legacyArtifacts()
			for _, obj := range objects {
				if _, secret := obj.(*corev1.Secret); secret && tc.missingSecret {
					continue
				}
				obj.SetFinalizers(tc.finalizers)
				g.Expect(c.Create(ctx, obj)).To(Succeed())
				if tc.deleting {
					g.Expect(c.Delete(ctx, obj)).To(Succeed())
				}
			}
			unrelated := []client.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "other-namespace", Name: "infrastructure.infra.tf-state", Finalizers: []string{finalizer}}},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "other.infra.tf-config", Finalizers: []string{finalizer}}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "cloudprovider"}},
			}
			for _, obj := range unrelated {
				g.Expect(c.Create(ctx, obj)).To(Succeed())
			}
			infra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "infrastructure"}}
			infra.Status.State = &runtime.RawExtension{Raw: []byte(`{"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState","data":{"Network":"network-1"}}`)}
			before := infra.DeepCopy()
			a := &actuator{client: c}
			g.Expect(a.Migrate(ctx, logr.Discard(), infra, nil)).To(Succeed())
			g.Expect(infra).To(Equal(before))
			for _, obj := range objects {
				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
				if len(tc.finalizers) == 2 {
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(obj.GetFinalizers()).To(ConsistOf("example.com/other-controller"))
					g.Expect(obj.GetDeletionTimestamp()).NotTo(BeNil())
				} else {
					g.Expect(err).To(BeNotFoundError())
				}
			}
			for _, obj := range unrelated {
				before := obj.DeepCopyObject()
				g.Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
				g.Expect(obj).To(Equal(before))
			}
			g.Expect(a.Migrate(ctx, logr.Discard(), infra, nil)).To(Succeed())
		})
	}
}

func TestMigratePropagatesCleanupFailures(t *testing.T) {
	for _, operation := range []string{"get", "patch", "delete"} {
		t.Run(operation, func(t *testing.T) {
			g := NewWithT(t)
			objects := legacyArtifacts()
			for _, obj := range objects {
				obj.SetFinalizers([]string{"gardener.cloud/terraformer"})
			}
			fakeErr := errors.New("API unavailable")
			c := fakeclient.NewClientBuilder().WithObjects(objects...).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if operation == "get" {
						return fakeErr
					}
					return c.Get(ctx, key, obj, opts...)
				},
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					if operation == "patch" {
						return fakeErr
					}
					return c.Patch(ctx, obj, patch, opts...)
				},
				Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if operation == "delete" {
						return fakeErr
					}
					return c.Delete(ctx, obj, opts...)
				},
			}).Build()
			a := &actuator{client: c}
			infra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: "shoot--test--openstack", Name: "infrastructure"}}
			g.Expect(a.Migrate(t.Context(), logr.Discard(), infra, nil)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})
	}
}

func TestActuatorRejectsUnsupportedStateBeforeAccessingCloud(t *testing.T) {
	a := &actuator{}
	for name, operation := range map[string]func(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error{
		"reconcile": a.Reconcile,
		"restore":   a.Restore,
		"delete":    a.Delete,
	} {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			infra := &extensionsv1alpha1.Infrastructure{}
			infra.Status.State = &runtime.RawExtension{Raw: []byte(`{"data":"{}","encoding":"none"}`)}
			before := infra.DeepCopy()
			g.Expect(operation(t.Context(), logr.Discard(), infra, nil)).To(HaveOccurred())
			g.Expect(infra).To(Equal(before))
		})
	}
}
