package infrastructure

import (
	"context"

	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		nw = mocks.NewMockNetworking(ctrl)
		ctx = context.TODO()
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
