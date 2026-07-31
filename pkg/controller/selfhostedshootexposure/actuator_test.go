// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsselfhostedshootexposure "github.com/gardener/gardener/extensions/pkg/controller/selfhostedshootexposure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	gardenertest "github.com/gardener/gardener/pkg/utils/test"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/pools"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/selfhostedshootexposure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("Actuator", func() {
	const (
		shootName   = "shoot--project--test"
		namespace   = "shoot--project--test"
		subnetID    = "subnet-id-1"
		fipPoolID   = "fip-pool-id-1"
		fipIP       = "1.2.3.4"
		lbID        = "lb-id-1"
		vipPortID   = "vip-port-id-1"
		vipAddress  = "10.0.0.100"
		poolID      = "pool-id-1"
		fipID       = "fip-id-1"
		nodesSGID   = "nodes-sg-id-1"
		lbName      = "shoot-exposure-" + namespace
		exposureTag = "gardener.cloud/shoot-exposure=" + namespace
	)

	var (
		ctx      context.Context
		ctrl     *gomock.Controller
		cluster  *extensionscontroller.Cluster
		exposure *extensionsv1alpha1.SelfHostedShootExposure
		scheme   *runtime.Scheme

		lbClient  *mocks.MockLoadbalancing
		nwClient  *mocks.MockNetworking
		factory   *mocks.MockFactory
		ffFactory fakeFactoryFactory
	)

	buildInfraStatusRaw := func() []byte {
		status := &openstackv1alpha1.InfrastructureStatus{
			TypeMeta: metav1.TypeMeta{
				APIVersion: schema.GroupVersion{Group: openstackv1alpha1.GroupName, Version: "v1alpha1"}.String(),
				Kind:       "InfrastructureStatus",
			},
			Networks: openstackv1alpha1.NetworkStatus{
				FloatingPool: openstackv1alpha1.FloatingPoolStatus{ID: fipPoolID},
				Subnets: []openstackv1alpha1.Subnet{
					{Purpose: openstackv1alpha1.PurposeNodes, ID: subnetID},
				},
			},
			SecurityGroups: []openstackv1alpha1.SecurityGroup{
				{Purpose: openstackv1alpha1.PurposeNodes, ID: nodesSGID},
			},
		}
		raw, err := json.Marshal(status)
		Expect(err).NotTo(HaveOccurred())
		return raw
	}

	newClient := func(objects ...client.Object) client.Client {
		infra := &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: namespace},
			Status: extensionsv1alpha1.InfrastructureStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					ProviderStatus: &runtime.RawExtension{Raw: buildInfraStatusRaw()},
				},
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: namespace},
			Data: map[string][]byte{
				"domainName": []byte("domain"),
				"tenantName": []byte("tenant"),
				"username":   []byte("user"),
				"password":   []byte("pass"),
				"authURL":    []byte("https://openstack.example.com"),
			},
		}
		return fake.NewClientBuilder().WithScheme(scheme).WithObjects(append([]client.Object{infra, secret}, objects...)...).Build()
	}

	activeLB := &loadbalancers.LoadBalancer{
		ID:                 lbID,
		Name:               lbName,
		Tags:               []string{exposureTag},
		VipPortID:          vipPortID,
		VipAddress:         vipAddress,
		ProvisioningStatus: "ACTIVE",
	}

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(openstackinstall.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

		lbClient = mocks.NewMockLoadbalancing(ctrl)
		nwClient = mocks.NewMockNetworking(ctrl)
		factory = mocks.NewMockFactory(ctrl)
		factory.EXPECT().Loadbalancing().Return(lbClient, nil).AnyTimes()
		factory.EXPECT().Networking().Return(nwClient, nil).AnyTimes()
		ffFactory = fakeFactoryFactory{factory: factory}

		cluster = &extensionscontroller.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: shootName},
			Shoot: &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: namespace},
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
					},
				},
			},
		}

		exposure = &extensionsv1alpha1.SelfHostedShootExposure{
			ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: namespace},
			Spec: extensionsv1alpha1.SelfHostedShootExposureSpec{
				Port: 6443,
				Endpoints: []extensionsv1alpha1.ControlPlaneEndpoint{{
					NodeName:  "node-1",
					Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}},
				}},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	makeActuator := func(c client.Client) extensionsselfhostedshootexposure.Actuator {
		return selfhostedshootexposure.NewActuator(&gardenertest.FakeManager{Client: c}, ffFactory)
	}

	Describe("Reconcile", func() {
		It("creates the load balancer and requeues while it provisions", func() {
			c := newClient()
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return(nil, nil)
			lbClient.EXPECT().CreateLoadbalancer(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, opts loadbalancers.CreateOpts) (*loadbalancers.LoadBalancer, error) {
					Expect(opts.Name).To(Equal(lbName))
					Expect(opts.VipSubnetID).To(Equal(subnetID))
					Expect(opts.Tags).To(ConsistOf(exposureTag))
					Expect(opts.Listeners).To(HaveLen(1))
					Expect(opts.Listeners[0].ProtocolPort).To(Equal(6443))
					Expect(opts.Listeners[0].DefaultPool).NotTo(BeNil())
					Expect(opts.Listeners[0].DefaultPool.Members).To(HaveLen(1))
					Expect(opts.Listeners[0].DefaultPool.Members[0].Address).To(Equal("10.0.0.1"))
					return &loadbalancers.LoadBalancer{ID: lbID, Name: lbName, Tags: []string{exposureTag}, ProvisioningStatus: "PENDING_CREATE"}, nil
				})

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 5*time.Second)
		})

		It("requeues while an existing load balancer is still PENDING", func() {
			c := newClient()
			pendingLB := &loadbalancers.LoadBalancer{ID: lbID, Name: lbName, Tags: []string{exposureTag}, ProvisioningStatus: "PENDING_UPDATE"}
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*pendingLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(pendingLB, nil)

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 5*time.Second)
		})

		It("reconciles members and returns the floating IP once the load balancer is ACTIVE", func() {
			c := newClient()
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(activeLB, nil)
			lbClient.EXPECT().ListPools(ctx, pools.ListOpts{LoadbalancerID: lbID, Name: lbName}).Return([]pools.Pool{{ID: poolID, Name: lbName}}, nil)
			lbClient.EXPECT().BatchUpdatePoolMembers(ctx, poolID, gomock.Any()).DoAndReturn(
				func(_ context.Context, _ string, opts []pools.BatchUpdateMemberOpts) error {
					Expect(opts).To(HaveLen(1))
					Expect(opts[0].Address).To(Equal("10.0.0.1"))
					Expect(opts[0].ProtocolPort).To(Equal(6443))
					return nil
				})
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return(nil, nil)
			nwClient.EXPECT().CreateRule(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, opts rules.CreateOpts) (*rules.SecGroupRule, error) {
					Expect(opts.SecGroupID).To(Equal(nodesSGID))
					Expect(opts.RemoteIPPrefix).To(Equal("0.0.0.0/0"))
					Expect(opts.PortRangeMin).To(Equal(6443))
					Expect(opts.PortRangeMax).To(Equal(6443))
					Expect(string(opts.Direction)).To(Equal("ingress"))
					return &rules.SecGroupRule{ID: "rule-1"}, nil
				})
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return(nil, nil)
			nwClient.EXPECT().CreateFloatingIP(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, opts floatingips.CreateOpts) (*floatingips.FloatingIP, error) {
					Expect(opts.FloatingNetworkID).To(Equal(fipPoolID))
					Expect(opts.PortID).To(Equal(vipPortID))
					Expect(opts.Description).To(Equal(exposureTag))
					return &floatingips.FloatingIP{ID: fipID, FloatingIP: fipIP, PortID: vipPortID}, nil
				})

			ingress, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(ingress).To(HaveLen(1))
			Expect(ingress[0].IP).To(Equal(fipIP))
		})

		It("re-associates an existing floating IP whose port drifted", func() {
			c := newClient()
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(activeLB, nil)
			lbClient.EXPECT().ListPools(ctx, gomock.Any()).Return([]pools.Pool{{ID: poolID, Name: lbName}}, nil)
			lbClient.EXPECT().BatchUpdatePoolMembers(ctx, poolID, gomock.Any()).Return(nil)
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return([]rules.SecGroupRule{{
				ID:             "rule-1",
				Direction:      "ingress",
				EtherType:      "IPv4",
				Protocol:       "tcp",
				PortRangeMin:   6443,
				PortRangeMax:   6443,
				RemoteIPPrefix: "0.0.0.0/0",
			}}, nil)
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return([]floatingips.FloatingIP{{ID: fipID, FloatingIP: fipIP, PortID: "stale-port"}}, nil)
			nwClient.EXPECT().UpdateFIPWithPort(ctx, fipID, vipPortID).Return(nil)

			ingress, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(ingress[0].IP).To(Equal(fipIP))
		})

		It("deletes a load balancer stuck in ERROR and requeues", func() {
			c := newClient()
			errorLB := &loadbalancers.LoadBalancer{ID: lbID, Name: lbName, Tags: []string{exposureTag}, ProvisioningStatus: "ERROR"}
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*errorLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(errorLB, nil)
			lbClient.EXPECT().DeleteLoadbalancer(ctx, lbID, loadbalancers.DeleteOpts{Cascade: true}).Return(nil)

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 5*time.Second)
		})

		It("requeues when a member update conflicts with a PENDING load balancer", func() {
			c := newClient()
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(activeLB, nil)
			lbClient.EXPECT().ListPools(ctx, gomock.Any()).Return([]pools.Pool{{ID: poolID, Name: lbName}}, nil)
			lbClient.EXPECT().BatchUpdatePoolMembers(ctx, poolID, gomock.Any()).Return(conflictError())

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 5*time.Second)
		})

		It("requeues when infrastructure status is not yet populated", func() {
			infra := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: namespace}}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(infra).Build()

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 30*time.Second)
		})

		It("requeues when endpoints are not yet populated", func() {
			exposure.Spec.Endpoints = nil
			c := newClient()

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 30*time.Second)
		})

		It("requeues when no endpoint matches the primary IP family", func() {
			exposure.Spec.Endpoints = []extensionsv1alpha1.ControlPlaneEndpoint{{
				NodeName:  "node-1",
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "fd00::1"}},
			}}
			c := newClient()

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 30*time.Second)
		})

		It("selects only the primary IP family's addresses in dual-stack shoots", func() {
			cluster.Shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{
				gardencorev1beta1.IPFamilyIPv4,
				gardencorev1beta1.IPFamilyIPv6,
			}
			exposure.Spec.Endpoints = []extensionsv1alpha1.ControlPlaneEndpoint{{
				NodeName: "node-1",
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
					{Type: corev1.NodeInternalIP, Address: "fd00::1"},
				},
			}}
			c := newClient()
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(activeLB, nil)
			lbClient.EXPECT().ListPools(ctx, gomock.Any()).Return([]pools.Pool{{ID: poolID, Name: lbName}}, nil)
			lbClient.EXPECT().BatchUpdatePoolMembers(ctx, poolID, gomock.Any()).DoAndReturn(
				func(_ context.Context, _ string, opts []pools.BatchUpdateMemberOpts) error {
					Expect(opts).To(HaveLen(1))
					Expect(opts[0].Address).To(Equal("10.0.0.1"))
					return nil
				})
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return(nil, nil)
			nwClient.EXPECT().CreateRule(ctx, gomock.Any()).Return(&rules.SecGroupRule{ID: "rule-1"}, nil)
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return([]floatingips.FloatingIP{{ID: fipID, FloatingIP: fipIP, PortID: vipPortID}}, nil)

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("replaces a security group rule that drifted from the desired shape", func() {
			c := newClient()
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().GetLoadbalancer(ctx, lbID).Return(activeLB, nil)
			lbClient.EXPECT().ListPools(ctx, gomock.Any()).Return([]pools.Pool{{ID: poolID, Name: lbName}}, nil)
			lbClient.EXPECT().BatchUpdatePoolMembers(ctx, poolID, gomock.Any()).Return(nil)
			// An existing rule has a different (stale) shape; it must be deleted and recreated.
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return([]rules.SecGroupRule{{
				ID:             "stale-rule",
				Direction:      "ingress",
				EtherType:      "IPv4",
				Protocol:       "tcp",
				PortRangeMin:   6443,
				PortRangeMax:   6443,
				RemoteIPPrefix: "10.0.0.99/32",
			}}, nil)
			nwClient.EXPECT().DeleteRule(ctx, "stale-rule").Return(nil)
			nwClient.EXPECT().CreateRule(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, opts rules.CreateOpts) (*rules.SecGroupRule, error) {
					Expect(opts.RemoteIPPrefix).To(Equal("0.0.0.0/0"))
					return &rules.SecGroupRule{ID: "rule-1"}, nil
				})
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return([]floatingips.FloatingIP{{ID: fipID, FloatingIP: fipIP, PortID: vipPortID}}, nil)

			_, err := makeActuator(c).Reconcile(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Delete", func() {
		It("deletes the floating IP and load balancer, requeuing until gone", func() {
			c := newClient()
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return([]floatingips.FloatingIP{{ID: fipID}}, nil)
			nwClient.EXPECT().DeleteFloatingIP(ctx, fipID).Return(nil)
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return([]rules.SecGroupRule{{ID: "rule-1"}}, nil)
			nwClient.EXPECT().DeleteRule(ctx, "rule-1").Return(nil)
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().DeleteLoadbalancer(ctx, lbID, loadbalancers.DeleteOpts{Cascade: true}).Return(nil)

			err := makeActuator(c).Delete(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 5*time.Second)
		})

		It("succeeds when nothing exists", func() {
			c := newClient()
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return(nil, nil)
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return(nil, nil)
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return(nil, nil)

			Expect(makeActuator(c).Delete(ctx, GinkgoLogr, exposure, cluster)).To(Succeed())
		})

		It("requeues without re-issuing delete while the load balancer is PENDING_DELETE", func() {
			c := newClient()
			deletingLB := &loadbalancers.LoadBalancer{ID: lbID, Name: lbName, Tags: []string{exposureTag}, ProvisioningStatus: "PENDING_DELETE"}
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return(nil, nil)
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return(nil, nil)
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*deletingLB}, nil)

			err := makeActuator(c).Delete(ctx, GinkgoLogr, exposure, cluster)
			Expect(err).To(HaveOccurred())
			assertRequeue(err, 5*time.Second)
		})
	})

	Describe("ForceDelete", func() {
		It("tears down resources best-effort and never requeues", func() {
			c := newClient()
			nwClient.EXPECT().GetFipByName(ctx, exposureTag).Return([]floatingips.FloatingIP{{ID: fipID}}, nil)
			nwClient.EXPECT().DeleteFloatingIP(ctx, fipID).Return(errors.New("boom"))
			nwClient.EXPECT().ListRules(ctx, gomock.Any()).Return(nil, errors.New("boom"))
			lbClient.EXPECT().ListLoadbalancers(ctx, gomock.Any()).Return([]loadbalancers.LoadBalancer{*activeLB}, nil)
			lbClient.EXPECT().DeleteLoadbalancer(ctx, lbID, loadbalancers.DeleteOpts{Cascade: true}).Return(errors.New("boom"))

			Expect(makeActuator(c).ForceDelete(ctx, GinkgoLogr, exposure, cluster)).To(Succeed())
		})
	})
})

func assertRequeue(err error, after time.Duration) {
	var requeueErr *ctrlerror.RequeueAfterError
	Expect(errors.As(err, &requeueErr)).To(BeTrue(), "expected a RequeueAfterError, got %v", err)
	Expect(requeueErr.RequeueAfter).To(Equal(after))
}

func conflictError() error {
	return gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusConflict}
}
