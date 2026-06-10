// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

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
		scheme         *runtime.Scheme
		workerDelegate genericactuator.WorkerDelegate
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		osFactory = mocks.NewMockFactory(ctrl)
		computeClient = mocks.NewMockCompute(ctrl)

		scheme = runtime.NewScheme()
		_ = api.AddToScheme(scheme)
		_ = apiv1alpha1.AddToScheme(scheme)
		_ = extensionsv1alpha1.AddToScheme(scheme)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#ServerGroups", func() {
		var (
			namespace   string
			technicalID string
			w           *extensionsv1alpha1.Worker
			cl          client.Client
		)

		BeforeEach(func() {
			namespace = "control-plane-namespace"
			technicalID = "shoot--foobar--openstack"

			w = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker",
					Namespace: namespace,
				},
			}
			cl = fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(w).
				WithStatusSubresource(&extensionsv1alpha1.Worker{}).
				Build()
			osFactory.EXPECT().Compute(gomock.Any()).AnyTimes().Return(computeClient, nil)
		})

		Context("#PreReconcileHook", func() {
			It("should not create server groups by default", func() {
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
					osFactory,
				)

				ctx := context.Background()

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})

			It("should create server groups if specified in worker pool", func() {
				var (
					ctx              = context.Background()
					policy           = "foo"
					pool1            = "pool-1"
					serverGroupID1   = "id-1"
					serverGroupName1 string

					pool2            = "pool-2"
					serverGroupID2   = "id-2"
					serverGroupName2 string
					pools            []extensionsv1alpha1.WorkerPool
				)

				pools = append(pools, *(newWorkerPoolWithPolicy(pool1, &policy)), *(newWorkerPoolWithPolicy(pool2, &policy)))
				w.Spec.Pools = append(w.Spec.Pools, pools...)
				syncWorkerSpec(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// List server groups first (new logic checks for existing ones)
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)

				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						serverGroupName1 = name
						return &servergroups.ServerGroup{
							ID:       serverGroupID1,
							Name:     name,
							Policies: []string{policy},
						}, nil
					}).Times(1)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						serverGroupName2 = name
						return &servergroups.ServerGroup{
							ID:       serverGroupID2,
							Name:     name,
							Policies: []string{policy},
						}, nil
					}).Times(1)

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(2))
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID1),
						"PoolName": Equal(pool1),
						"Name":     Equal(serverGroupName1),
					}),
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID2),
						"PoolName": Equal(pool2),
						"Name":     Equal(serverGroupName2),
					}),
				))
			})

			It("should recreate server group if specs do not match", func() {
				var (
					ctx                = context.Background()
					poolName           = "pool"
					policy             = "foo"
					newPolicy          = "bar"
					serverGroupName    = technicalID + "-" + poolName + "-rand123"
					newServerGroupName string
				)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy("pool", &policy)))
				syncWorkerSpec(ctx, cl, w)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// First reconcile - create initial server group
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						serverGroupName = name
						return &servergroups.ServerGroup{
							ID:       "id",
							Name:     name,
							Policies: []string{policy},
						}, nil
					})

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal("id"),
						"PoolName": Equal("pool"),
						"Name":     Equal(serverGroupName),
					}),
				))

				// Second reconcile - policy changed, should recreate
				w.Spec.Pools[0] = *(newWorkerPoolWithPolicy("pool", &newPolicy))
				syncWorkerSpec(ctx, cl, w)
				computeClient.EXPECT().GetServerGroup(ctx, "id").Return(&servergroups.ServerGroup{
					ID:       "id",
					Name:     serverGroupName,
					Policies: []string{policy},
				}, nil)
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:       "id",
						Name:     serverGroupName,
						Policies: []string{policy},
					},
				}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), newPolicy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						newServerGroupName = name
						return &servergroups.ServerGroup{
							ID:       "new-id",
							Name:     name,
							Policies: []string{policy},
						}, nil
					})

				err = workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerStatus = decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal("new-id"),
						"PoolName": Equal("pool"),
						"Name":     Equal(newServerGroupName),
					}),
				))
			})
		})

		Context("#PreReconcileHook during Restore", func() {
			It("should create server groups when called for restore scenario with missing dependencies", func() {
				var (
					ctx              = context.Background()
					policy           = "soft-anti-affinity"
					pool1            = "pool-1"
					serverGroupID1   = "id-1"
					serverGroupName1 string
				)

				// Setup worker with server group configuration but no dependencies
				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy(pool1, &policy)))

				// Set the last operation type to Restore
				w.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				}

				// Empty worker status (no server group dependencies)
				// This simulates the scenario where GenerateMachineDeployments would trigger PreReconcileHook
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// Expect PreReconcileHook to list then create the server group
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						serverGroupName1 = name
						return &servergroups.ServerGroup{
							ID:       serverGroupID1,
							Name:     name,
							Policies: []string{policy},
						}, nil
					})

				// Directly call PreReconcileHook (which is what GenerateMachineDeployments would do)
				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Verify that server group dependencies were created
				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(1))
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID1),
						"PoolName": Equal(pool1),
						"Name":     Equal(serverGroupName1),
					}),
				))
			})

			It("should create server groups for multiple pools during restore scenario", func() {
				var (
					ctx              = context.Background()
					policy           = "soft-anti-affinity"
					pool1            = "pool-1"
					pool2            = "pool-2"
					serverGroupID1   = "id-1"
					serverGroupID2   = "id-2"
					serverGroupName1 string
					serverGroupName2 string
				)

				// Setup worker with multiple pools requiring server groups
				w.Spec.Pools = append(w.Spec.Pools,
					*(newWorkerPoolWithPolicy(pool1, &policy)),
					*(newWorkerPoolWithPolicy(pool2, &policy)),
				)

				// Set the last operation type to Restore
				w.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				}

				// Empty worker status (no server group dependencies)
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// PreReconcileHook should list then create server groups for both pools
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						serverGroupName1 = name
						return &servergroups.ServerGroup{
							ID:       serverGroupID1,
							Name:     name,
							Policies: []string{policy},
						}, nil
					}).Times(1)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						serverGroupName2 = name
						return &servergroups.ServerGroup{
							ID:       serverGroupID2,
							Name:     name,
							Policies: []string{policy},
						}, nil
					}).Times(1)

				// Directly call PreReconcileHook
				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Verify that server group dependencies were created for both pools
				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(2))
				Expect(workerStatus.ServerGroupDependencies).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID1),
						"PoolName": Equal(pool1),
						"Name":     Equal(serverGroupName1),
					}),
					MatchFields(IgnoreExtras, Fields{
						"ID":       Equal(serverGroupID2),
						"PoolName": Equal(pool2),
						"Name":     Equal(serverGroupName2),
					}),
				))
			})
		})

		Context("#PreReconcileHook with K8s version-based naming", func() {
			var (
				ctx       = context.Background()
				policy    = "soft-anti-affinity"
				poolName  = "pool-1"
				shootUID  = "12345678-1234-1234-1234-123456789012"
				oldSGName = technicalID + "-" + poolName
			)

			It("should use new name format for K8s >= 1.35", func() {
				k8sVersion := "1.35.0"
				pool := newWorkerPoolWithPolicy(poolName, &policy)
				pool.KubernetesVersion = &k8sVersion
				w.Spec.Pools = append(w.Spec.Pools, *pool)
				syncWorkerSpec(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				cluster.Shoot.Spec.Kubernetes.Version = k8sVersion

				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// Should list server groups and find none, then create with new name format
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						// Verify the name uses the new format (UUID prefix)
						Expect(name).To(HavePrefix(shootUID[:16]))
						return &servergroups.ServerGroup{
							ID:       "new-id",
							Name:     name,
							Policies: []string{policy},
						}, nil
					})

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(1))
				Expect(workerStatus.ServerGroupDependencies[0].ID).To(Equal("new-id"))
			})

			It("should reuse existing old name format server group for K8s < 1.35", func() {
				k8sVersion := "1.34.0"
				pool := newWorkerPoolWithPolicy(poolName, &policy)
				pool.KubernetesVersion = &k8sVersion
				w.Spec.Pools = append(w.Spec.Pools, *pool)
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						ID:       "existing-old-id",
						Name:     oldSGName,
						PoolName: poolName,
					},
				})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				cluster.Shoot.Spec.Kubernetes.Version = k8sVersion

				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				computeClient.EXPECT().GetServerGroup(ctx, gomock.Any()).DoAndReturn(
					func(_ context.Context, id string) (*servergroups.ServerGroup, error) {
						return &servergroups.ServerGroup{
							ID:       id,
							Name:     oldSGName,
							Policies: []string{policy},
						}, nil
					})

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(1))
				Expect(workerStatus.ServerGroupDependencies[0].ID).To(Equal("existing-old-id"))
				Expect(workerStatus.ServerGroupDependencies[0].Name).To(Equal(oldSGName))
			})

			It("should use new name format for K8s < 1.35 when no existing old format server group exists", func() {
				k8sVersion := "1.34.0"
				pool := newWorkerPoolWithPolicy(poolName, &policy)
				pool.KubernetesVersion = &k8sVersion
				w.Spec.Pools = append(w.Spec.Pools, *pool)
				syncWorkerSpec(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				cluster.Shoot.Spec.Kubernetes.Version = k8sVersion

				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// Should list server groups, find none with old name, then create with new name format
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{}, nil)
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), policy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						// Verify the name uses the new format (UUID prefix)
						Expect(name).To(HavePrefix(shootUID[:16]))
						return &servergroups.ServerGroup{
							ID:       "new-id",
							Name:     name,
							Policies: []string{policy},
						}, nil
					})

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(1))
				Expect(workerStatus.ServerGroupDependencies[0].ID).To(Equal("new-id"))
			})

			It("should recreate server group with new name when policy changes for K8s < 1.35", func() {
				k8sVersion := "1.34.0"
				oldPolicy := "soft-anti-affinity"
				newPolicy := "anti-affinity"
				pool := newWorkerPoolWithPolicy(poolName, &newPolicy)
				pool.KubernetesVersion = &k8sVersion
				w.Spec.Pools = append(w.Spec.Pools, *pool)
				syncWorkerSpec(ctx, cl, w)

				cluster := newClusterWithDefaultCloudProfileConfig(namespace, technicalID)
				cluster.Shoot.Spec.Kubernetes.Version = k8sVersion

				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					cluster,
					osFactory,
				)

				// Should list server groups and find existing one with old policy
				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:       "existing-old-id",
						Name:     oldSGName,
						Policies: []string{oldPolicy}, // Old policy doesn't match
					},
				}, nil)
				// Should create new server group with new name format
				computeClient.EXPECT().CreateServerGroup(ctx, gomock.Any(), newPolicy).DoAndReturn(
					func(_ context.Context, name string, policy string) (*servergroups.ServerGroup, error) {
						// Verify the name uses the new format (UUID prefix)
						Expect(name).To(HavePrefix(shootUID[:16]))
						return &servergroups.ServerGroup{
							ID:       "new-id-with-new-policy",
							Name:     name,
							Policies: []string{policy},
						}, nil
					})

				err := workerDelegate.PreReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(HaveLen(1))
				Expect(workerStatus.ServerGroupDependencies[0].ID).To(Equal("new-id-with-new-policy"))
			})
		})

		Context("#PostReconcileHook", func() {
			It("should clean server group if worker pool is deleted", func() {
				var (
					ctx             = context.Background()
					poolName        = "pool"
					serverGroupID   = "id"
					serverGroupName = technicalID + "-" + poolName + "-" + "rand"
				)

				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						PoolName: poolName,
						ID:       serverGroupID,
						Name:     serverGroupName,
					},
				})
				syncWorkerStatus(ctx, cl, w)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID,
						Name: serverGroupName,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID).Return(nil)

				err := workerDelegate.PostReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})

			It("should clean old server group if worker pool specs changed", func() {
				var (
					ctx = context.Background()

					poolName = "pool"
					policy   = "foo"

					serverGroupID   = "id"
					serverGroupName = technicalID + "-" + poolName + "-" + "rand"

					oldServerGroupID   = "old-id"
					oldServerGroupName = technicalID + "-" + poolName + "-" + "old-rand"
				)

				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy("pool", &policy)))
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						PoolName: poolName,
						ID:       serverGroupID,
						Name:     serverGroupName,
					},
				})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
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

				err := workerDelegate.PostReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
			})

			It("should clean all server groups if worker is terminating", func() {

				var (
					ctx = context.Background()

					policy           = "foo"
					poolName1        = "pool1"
					serverGroupID1   = "id1"
					serverGroupName1 = technicalID + "-" + poolName1 + "-rand1"
					poolName2        = "pool2"
					serverGroupID2   = "id2"
					serverGroupName2 = technicalID + "-" + poolName2 + "-rand2"
				)

				deletionTime := metav1.NewTime(time.Now())
				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy(poolName1, &policy)), *(newWorkerPoolWithPolicy(poolName2, &policy)))
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						PoolName: poolName1,
						ID:       serverGroupID1,
						Name:     serverGroupName1,
					},
					{
						PoolName: poolName2,
						ID:       serverGroupID2,
						Name:     serverGroupName2,
					},
				})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)
				w.DeletionTimestamp = &deletionTime
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID1,
						Name: serverGroupName1,
					},
					{
						ID:   serverGroupID2,
						Name: serverGroupName2,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID1).Return(nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID2).Return(nil)

				err := workerDelegate.PostReconcileHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})
		})

		Context("#PostDeleteHook", func() {
			It("should clean server group if worker pool is deleted", func() {
				var (
					ctx             = context.Background()
					poolName        = "pool"
					serverGroupID   = "id"
					serverGroupName = technicalID + "-" + poolName + "-" + "rand"
				)

				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						PoolName: poolName,
						ID:       serverGroupID,
						Name:     serverGroupName,
					},
				})
				syncWorkerStatus(ctx, cl, w)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID,
						Name: serverGroupName,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID).Return(nil)

				err := workerDelegate.PostDeleteHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).To(BeEmpty())
			})

			It("should clean old server group if worker pool specs changed", func() {
				var (
					ctx = context.Background()

					poolName = "pool"
					policy   = "foo"

					serverGroupID   = "id"
					serverGroupName = technicalID + "-" + poolName + "-" + "rand"

					oldServerGroupID   = "old-id"
					oldServerGroupName = technicalID + "-" + poolName + "-" + "old-rand"
				)

				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy("pool", &policy)))
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						PoolName: poolName,
						ID:       serverGroupID,
						Name:     serverGroupName,
					},
				})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
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

				err := workerDelegate.PostDeleteHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
				Expect(workerStatus.ServerGroupDependencies).NotTo(BeEmpty())
			})

			It("should clean all server groups if worker is terminating", func() {

				var (
					ctx = context.Background()

					policy           = "foo"
					poolName1        = "pool1"
					serverGroupID1   = "id1"
					serverGroupName1 = technicalID + "-" + poolName1 + "-rand1"
					poolName2        = "pool2"
					serverGroupID2   = "id2"
					serverGroupName2 = technicalID + "-" + poolName2 + "-rand2"
				)

				deletionTime := metav1.NewTime(time.Now())
				w.Spec.Pools = append(w.Spec.Pools, *(newWorkerPoolWithPolicy(poolName1, &policy)), *(newWorkerPoolWithPolicy(poolName2, &policy)))
				w.Status.ProviderStatus = workerProviderStatusRaw([]apiv1alpha1.ServerGroupDependency{
					{
						PoolName: poolName1,
						ID:       serverGroupID1,
						Name:     serverGroupName1,
					},
					{
						PoolName: poolName2,
						ID:       serverGroupID2,
						Name:     serverGroupName2,
					},
				})
				syncWorkerSpec(ctx, cl, w)
				syncWorkerStatus(ctx, cl, w)
				w.DeletionTimestamp = &deletionTime
				workerDelegate, _ = worker.NewWorkerDelegate(
					cl,
					scheme,
					nil,
					w,
					newClusterWithDefaultCloudProfileConfig(namespace, technicalID),
					osFactory,
				)

				computeClient.EXPECT().ListServerGroups(ctx).Return([]servergroups.ServerGroup{
					{
						ID:   serverGroupID1,
						Name: serverGroupName1,
					},
					{
						ID:   serverGroupID2,
						Name: serverGroupName2,
					},
				}, nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID1).Return(nil)
				computeClient.EXPECT().DeleteServerGroup(ctx, serverGroupID2).Return(nil)

				err := workerDelegate.PostDeleteHook(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerStatus(w)
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

func newClusterWithDefaultCloudProfileConfig(name, technicalID string) *extensionscontroller.Cluster {
	cloudProfileConfig := &api.CloudProfileConfig{
		ServerGroupPolicies: []string{"foo", "bar"},
	}

	cpJson, err := json.Marshal(cloudProfileConfig)
	Expect(err).NotTo(HaveOccurred())

	return &extensionscontroller.Cluster{
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
		Seed: nil,
		Shoot: &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				UID: "12345678-1234-1234-1234-123456789012",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.32.0",
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: technicalID,
			},
		},
	}
}

// decodeWorkerStatus decodes the WorkerStatus from the Worker's ProviderStatus.
// After a fakeclient Status().Patch(), the Object field is nil and the status is
// stored as raw JSON bytes — use this helper instead of a direct type assertion.
// This helper decodes the raw JSON bytes into a WorkerStatus struct for easier assertions in tests.
func decodeWorkerStatus(w *extensionsv1alpha1.Worker) *apiv1alpha1.WorkerStatus {
	Expect(w.Status.ProviderStatus).NotTo(BeNil())
	raw, err := w.Status.ProviderStatus.MarshalJSON()
	Expect(err).NotTo(HaveOccurred())
	status := &apiv1alpha1.WorkerStatus{}
	Expect(json.Unmarshal(raw, status)).To(Succeed())
	return status
}

// workerProviderStatusRaw serialises a WorkerStatus to a RawExtension for use in
// test setup (fakeclient stores objects as JSON, so Object must be pre-serialised).
func workerProviderStatusRaw(deps []apiv1alpha1.ServerGroupDependency) *runtime.RawExtension {
	raw, err := json.Marshal(&apiv1alpha1.WorkerStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
			Kind:       "WorkerStatus",
		},
		ServerGroupDependencies: deps,
	})
	Expect(err).NotTo(HaveOccurred())
	return &runtime.RawExtension{Raw: raw}
}

// syncWorkerSpec pushes w's spec and metadata (but not status) into the fakeclient
// store so that subsequent Status().Patch() calls do not clobber in-memory spec changes.
// It saves and restores the in-memory w.Status so that a subsequent syncWorkerStatus call
// can still push the intended status.
func syncWorkerSpec(ctx context.Context, cl client.Client, w *extensionsv1alpha1.Worker) {
	savedStatus := w.Status.DeepCopy()
	Expect(cl.Update(ctx, w)).To(Succeed())
	w.Status = *savedStatus
}

// syncWorkerStatus pushes w.Status into the fakeclient store so that
// the WorkerDelegate's seedClient can read it back via the status subresource.
func syncWorkerStatus(ctx context.Context, cl client.Client, w *extensionsv1alpha1.Worker) {
	Expect(cl.Status().Update(ctx, w)).To(Succeed())
}
