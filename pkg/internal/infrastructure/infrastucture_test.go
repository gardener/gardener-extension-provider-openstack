package infrastructure

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("Infrastructure", func() {
	var (
		ctrl          *gomock.Controller
		nw            *mocks.MockNetworking
		ctx           context.Context
		routerID      string
		defaultWorker = "10.0.0.0/16"
		router        *routers.Router
		clusterName   = "foo-bar"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		nw = mocks.NewMockNetworking(ctrl)
		ctx = context.TODO()
	})

	Context("Route deletion", func() {
		BeforeEach(func() {
			routerID = "router"
			router = &routers.Router{
				ID: routerID,
			}
		})

		type args struct {
			workers string
			prep    func()
		}

		prepRoutes := func(routes ...routers.Route) {
			router.Routes = routes
			nw.EXPECT().GetRouterByID(routerID).Return(router, nil)
		}

		DescribeTable("#RouteCleanup", func(a args, expErr error) {
			a.prep()
			err := CleanupKubernetesRoutes(ctx, nw, routerID, a.workers)
			if expErr == nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err).To(Equal(expErr))
			}
		},
			Entry("no update request if no routes exist", args{
				workers: defaultWorker,
				prep:    func() { prepRoutes() },
			}, nil),
			Entry("no update request if no routes need change", args{
				workers: defaultWorker,
				prep: func() {
					prepRoutes(
						routers.Route{NextHop: "10.11.0.0"},
						routers.Route{NextHop: "10.12.0.0"},
						routers.Route{NextHop: "10.13.0.0"},
					)
				}}, nil),
			Entry("expect update request", args{
				workers: defaultWorker,
				prep: func() {
					prepRoutes(
						routers.Route{NextHop: "10.0.0.0"},
						routers.Route{NextHop: "10.0.1.0"},
						routers.Route{NextHop: "10.0.2.0"},
						// plus one more that needs to be preserved
						routers.Route{NextHop: "10.11.2.0"},
					)
					nw.EXPECT().UpdateRoutesForRouter([]routers.Route{{NextHop: "10.11.2.0"}}, routerID).Return(router, nil)
				}}, nil),
		)
	})

	Context("Loadbalancer deletion", func() {
		var (
			lbclient *mocks.MockLoadbalancing
			svcName  string
			log      logr.Logger
			subnetID string
			lbs      []loadbalancers.LoadBalancer
		)
		BeforeEach(func() {
			lbclient = mocks.NewMockLoadbalancing(ctrl)
			svcName = "nginx"
			log = logf.Log.WithName("bastion-test")
			lbs = []loadbalancers.LoadBalancer{
				{
					ProvisioningStatus: "ACTIVE",
					Name:               fmt.Sprintf("kube_service_%s_%s", clusterName, svcName),
					VipSubnetID:        subnetID,
					ID:                 "k8s",
				},
				{
					ProvisioningStatus: "ACTIVE",
					Name:               "baz",
					ID:                 "not-k8s",
				},
			}
		})

		It("should delete all the kubernetes loadbalancers", func() {
			lbclient.EXPECT().ListLoadbalancers(gomock.Any()).Return(lbs, nil)
			lbclient.EXPECT().DeleteLoadbalancer("k8s", loadbalancers.DeleteOpts{Cascade: true}).Return(nil)
			// first call to Get will return active state
			gomock.InOrder(
				lbclient.EXPECT().GetLoadbalancer("k8s").Return(&lbs[0], nil),
				lbclient.EXPECT().GetLoadbalancer("k8s").Return(nil, nil),
			)
			err := CleanupKubernetesLoadbalancers(ctx, log, lbclient, subnetID, clusterName)
			Expect(err).To(BeNil())
		})
	})
})
