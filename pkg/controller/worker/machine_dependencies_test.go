// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	k8smocks "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("#MachineDependencies", func() {

	var (
		ctrl *gomock.Controller

		osFactory      *mocks.MockFactory
		computeClient  *mocks.MockCompute
		cl             *k8smocks.MockClient
		statusCl       *k8smocks.MockStatusWriter
		scheme         *runtime.Scheme
		workerDelegate genericactuator.WorkerDelegate
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		osFactory = mocks.NewMockFactory(ctrl)
		computeClient = mocks.NewMockCompute(ctrl)

		cl = k8smocks.NewMockClient(ctrl)
		statusCl = k8smocks.NewMockStatusWriter(ctrl)

		cl.EXPECT().Status().AnyTimes().Return(statusCl)

		scheme = runtime.NewScheme()
		_ = api.AddToScheme(scheme)
		_ = apiv1alpha1.AddToScheme(scheme)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#ServerGroups", func() {
		var (
			clusterName = "shoot--foobar--openstack"
			namespace   = clusterName
			w           *extensionsv1alpha1.Worker
		)

		BeforeEach(func() {
			w = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
			}
			osFactory.EXPECT().Compute(gomock.Any()).AnyTimes().Return(computeClient, nil)
		})

		Context("#PreReconcileHook", func() {
			It("should not create server groups by default", func() {
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				ctx := context.Background()
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})

			It("should create server groups if specified in worker pool", func() {
				var (
					ctx            = context.Background()
					policy         = "foo"
					pool1          = "pool-1"
					serverGroupID1 = "id-1"

					pool2          = "pool-2"
					serverGroupID2 = "id-2"
					pools          []extensionsv1alpha1.WorkerPool
				)

				pools = append(pools, *(newWorkerPoolWithPolicy(pool1, &policy)), *(newWorkerPoolWithPolicy(pool2, &policy)))
				w.Spec.Pools = append(w.Spec.Pools, pools...)

				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().CreateServerGroup(ctx, prefixMatch(serverGroupPrefix(clusterName, pool1)), policy).Return(&servergroups.ServerGroup{
					ID: serverGroupID1,
				}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, prefixMatch(serverGroupPrefix(clusterName, pool2)), policy).Return(&servergroups.ServerGroup{
					ID: serverGroupID2,
				}, nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(2))
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID1),
						"PoolName": Equal(pool1),
					}),
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID2),
						"PoolName": Equal(pool2),
					}),
				))
			})

			It("should recreate server group if specs do not match", func() {
				var (
					ctx       = context.Background()
					poolName  = "pool"
					policy    = "foo"
					newPolicy = "bar"
				)

				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy("pool", &policy)))
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().CreateServerGroup(ctx, prefixMatch(serverGroupPrefix(clusterName, poolName)), policy).Return(&servergroups.ServerGroup{
					ID: "id",
				}, nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal("id"),
						"PoolName": Equal("pool"),
					}),
				))

				w.Spec.Pools[0] = *(newWorkerPoolWithPolicy("pool", &newPolicy))
				computeClient.EXPECT().GetServerGroup(ctx, "id").Return(&servergroups.ServerGroup{
					ID:       "id",
					Policies: []string{"foo"},
				}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, prefixMatch(serverGroupPrefix(clusterName, poolName)), newPolicy).Return(&servergroups.ServerGroup{
					ID: "new-id",
				}, nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err = workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerStatus = w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal("new-id"),
						"PoolName": Equal("pool"),
					}),
				))
			})
		})

		Context("#PostReconcileHook", func() {
			It("should clean server group if worker pool is deleted", func() {
				var (
					ctx             = context.Background()
					poolName        = "pool"
					serverGroupID   = "id"
					serverGroupName = clusterName + "-" + poolName + "-" + "rand"
				)

				w.Status.ProviderStatus = &runtime.RawExtension{
					Object: &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							Kind:       "WorkerStatus",
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						},
						ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
							{
								PoolName: poolName,
								ID:       serverGroupID,
							},
						},
					},
				}
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID,
						Name: serverGroupName,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID).Return(nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PostReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})

			It("should clean old server group if worker pool specs changed", func() {
				var (
					ctx = context.Background()

					poolName = "pool"
					policy   = "foo"

					serverGroupID   = "id"
					serverGroupName = clusterName + "-" + poolName + "-" + "rand"

					oldServerGroupID   = "old-id"
					oldServerGroupName = clusterName + "-" + poolName + "-" + "old-rand"
				)

				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy("pool", &policy)))
				w.Status.ProviderStatus = &runtime.RawExtension{
					Object: &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							Kind:       "WorkerStatus",
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						},
						ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
							{
								PoolName: poolName,
								ID:       serverGroupID,
							},
						},
					},
				}
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID,
						Name: serverGroupName,
					},
					{
						ID:   oldServerGroupID,
						Name: oldServerGroupName,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, oldServerGroupID).Return(nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PostReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
			})

			It("should clean all server groups if worker is terminating", func() {

				var (
					ctx = context.Background()

					policy         = "foo"
					poolName1      = "pool1"
					serverGroupID1 = "id1"
					poolName2      = "pool2"
					serverGroupID2 = "id2"
				)

				deletionTime := metav1.NewTime(time.Now())
				w.DeletionTimestamp = &deletionTime
				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy(poolName1, &policy)), *(newWorkerPoolWithPolicy(poolName2, &policy)))
				w.Status.ProviderStatus = &runtime.RawExtension{
					Object: &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							Kind:       "WorkerStatus",
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						},
						ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
							{
								PoolName: poolName1,
								ID:       serverGroupID1,
							},
							{
								PoolName: poolName2,
								ID:       serverGroupID2,
							},
						},
					},
				}
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID1,
						Name: poolName1,
					},
					{
						ID:   serverGroupID2,
						Name: poolName2,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID1).Return(nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID2).Return(nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PostReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})
		})

		Context("#PostDeleteHook", func() {
			It("should clean server group if worker pool is deleted", func() {
				var (
					ctx             = context.Background()
					poolName        = "pool"
					serverGroupID   = "id"
					serverGroupName = clusterName + "-" + poolName + "-" + "rand"
				)

				w.Status.ProviderStatus = &runtime.RawExtension{
					Object: &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							Kind:       "WorkerStatus",
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						},
						ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
							{
								PoolName: poolName,
								ID:       serverGroupID,
							},
						},
					},
				}
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID,
						Name: serverGroupName,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID).Return(nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PostDeleteHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})

			It("should clean old server group if worker pool specs changed", func() {
				var (
					ctx = context.Background()

					poolName = "pool"
					policy   = "foo"

					serverGroupID   = "id"
					serverGroupName = clusterName + "-" + poolName + "-" + "rand"

					oldServerGroupID   = "old-id"
					oldServerGroupName = clusterName + "-" + poolName + "-" + "old-rand"
				)

				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy("pool", &policy)))
				w.Status.ProviderStatus = &runtime.RawExtension{
					Object: &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							Kind:       "WorkerStatus",
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						},
						ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
							{
								PoolName: poolName,
								ID:       serverGroupID,
							},
						},
					},
				}
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID,
						Name: serverGroupName,
					},
					{
						ID:   oldServerGroupID,
						Name: oldServerGroupName,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, oldServerGroupID).Return(nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PostDeleteHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
			})

			It("should clean all server groups if worker is terminating", func() {

				var (
					ctx = context.Background()

					policy         = "foo"
					poolName1      = "pool1"
					serverGroupID1 = "id1"
					poolName2      = "pool2"
					serverGroupID2 = "id2"
				)

				deletionTime := metav1.NewTime(time.Now())
				w.DeletionTimestamp = &deletionTime
				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy(poolName1, &policy)), *(newWorkerPoolWithPolicy(poolName2, &policy)))
				w.Status.ProviderStatus = &runtime.RawExtension{
					Object: &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							Kind:       "WorkerStatus",
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						},
						ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
							{
								PoolName: poolName1,
								ID:       serverGroupID1,
							},
							{
								PoolName: poolName2,
								ID:       serverGroupID2,
							},
						},
					},
				}
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					"",
					w,
					newClusterWithDefaultCloudProfileConfig(clusterName),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID1,
						Name: poolName1,
					},
					{
						ID:   serverGroupID2,
						Name: poolName2,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID1).Return(nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID2).Return(nil)
				expectStatusUpdateToSucceed(ctx, statusCl)

				err := workerDelegate.PostDeleteHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := w.Status.ProviderStatus.Object.(*apiv1alpha1.WorkerStatus)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})
		})
	})
})

func newWorkerPoolWithPolicy(name string, policy *string) *extensionsv1alpha1.WorkerPool {
	pool := &extensionsv1alpha1.WorkerPool{
		Name: name,
	}

	if policy != nil {
		workerConfig := apiv1alpha1.WorkerConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
				Kind:       "WorkerConfig",
			},
			ServerGroup: &apiv1alpha1.ServerGroup{
				Policy: *policy,
			},
		}

		wppcJson, err := json.Marshal(workerConfig)
		Expect(err).NotTo(HaveOccurred())

		pool.ProviderConfig = &runtime.RawExtension{
			Raw: wppcJson,
		}
	}
	return pool
}

func newClusterWithDefaultCloudProfileConfig(name string) *controller.Cluster {
	cloudProfileConfig := &api.CloudProfileConfig{
		ServerGroupPolicies: []string{"foo", "bar"},
	}

	cpJson, err := json.Marshal(cloudProfileConfig)
	Expect(err).NotTo(HaveOccurred())

	return &controller.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		CloudProfile: &gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cpJson,
				},
			},
		},
		Seed:  nil,
		Shoot: nil,
	}
}

func expectStatusUpdateToSucceed(ctx context.Context, statusWriter *k8smocks.MockStatusWriter) {
	statusWriter.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.Any()).Return(nil)
}

type prefixMatcher struct {
	prefix string
}

func (p *prefixMatcher) Matches(x interface{}) bool {
	s, ok := x.(string)
	if !ok {
		return false
	}

	return strings.HasPrefix(s, p.prefix)
}

func (p *prefixMatcher) String() string {
	return fmt.Sprintf("doesn't match prefix %s", p.prefix)
}

func prefixMatch(prefix string) gomock.Matcher {
	return &prefixMatcher{
		prefix: prefix,
	}
}

func serverGroupPrefix(clusterName, poolName string) string {
	return fmt.Sprintf("%s-%s", clusterName, poolName)
}
