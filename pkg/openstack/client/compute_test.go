// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("Compute", func() {
	Describe("CreateServerGroup", func() {
		DescribeTable("should send the appropriate policy and rules without changing the shared microversion", func(policy string, maxServerPerHost *int32, microversion, expectedBody string) {
			requests := 0
			client := newServerGroupTestClient(func(request *http.Request) (*http.Response, error) {
				requests++
				Expect(request.Method).To(Equal(http.MethodPost))
				Expect(request.URL.Path).To(Equal("/os-server-groups"))
				Expect(request.Header.Get("X-OpenStack-Nova-API-Version")).To(Equal(microversion))
				body, err := io.ReadAll(request.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body).To(MatchJSON(expectedBody))
				return serverGroupTestResponse(http.StatusOK, `{"server_group":{"id":"group-id","name":"workers"}}`), nil
			})

			group, err := client.CreateServerGroup(context.Background(), "workers", policy, maxServerPerHost)
			Expect(err).NotTo(HaveOccurred())
			Expect(group).NotTo(BeNil())
			Expect(group.ID).To(Equal("group-id"))
			Expect(requests).To(Equal(1))
			Expect(client.client.Microversion).To(Equal("2.7"))
		},
			Entry("legacy affinity", "affinity", (*int32)(nil), "", `{"server_group":{"name":"workers","policies":["affinity"]}}`),
			Entry("legacy anti-affinity", "anti-affinity", (*int32)(nil), "", `{"server_group":{"name":"workers","policies":["anti-affinity"]}}`),
			Entry("soft anti-affinity", "soft-anti-affinity", (*int32)(nil), "2.15", `{"server_group":{"name":"workers","policies":["soft-anti-affinity"]}}`),
			Entry("soft affinity", "soft-affinity", (*int32)(nil), "2.15", `{"server_group":{"name":"workers","policies":["soft-affinity"]}}`),
			Entry("two servers per host", "anti-affinity", ptr.To[int32](2), "2.64", `{"server_group":{"name":"workers","policy":"anti-affinity","rules":{"max_server_per_host":2}}}`),
			Entry("explicit one server per host", "anti-affinity", ptr.To[int32](1), "2.64", `{"server_group":{"name":"workers","policy":"anti-affinity","rules":{"max_server_per_host":1}}}`),
		)

		It("should not drop rules when the microversion is unsupported", func() {
			requests := 0
			client := newServerGroupTestClient(func(_ *http.Request) (*http.Response, error) {
				requests++
				return serverGroupTestResponse(http.StatusNotAcceptable, `{}`), nil
			})
			_, err := client.CreateServerGroup(context.Background(), "workers", "anti-affinity", ptr.To[int32](2))
			Expect(err).To(HaveOccurred())
			Expect(gophercloud.ResponseCodeIs(err, http.StatusNotAcceptable)).To(BeTrue())
			Expect(requests).To(Equal(1), "must not retry without rules")
			Expect(client.client.Microversion).To(Equal("2.7"))
		})
	})

	for _, operation := range []struct {
		name string
		path string
		wrap func(string) string
		read func(*ComputeClient) ([]servergroups.ServerGroup, error)
	}{
		{
			name: "GetServerGroup",
			path: "/os-server-groups/group-id",
			wrap: func(group string) string { return `{"server_group":` + group + `}` },
			read: func(client *ComputeClient) ([]servergroups.ServerGroup, error) {
				group, err := client.GetServerGroup(context.Background(), "group-id")
				if err != nil {
					return nil, err
				}
				Expect(group).NotTo(BeNil())
				return []servergroups.ServerGroup{*group}, nil
			},
		},
		{
			name: "ListServerGroups",
			path: "/os-server-groups",
			wrap: func(group string) string { return `{"server_groups":[` + group + `]}` },
			read: func(client *ComputeClient) ([]servergroups.ServerGroup, error) {
				return client.ListServerGroups(context.Background())
			},
		},
	} {
		Describe(operation.name, func() {
			DescribeTable("should preserve rules and errors, falling back only for an unsupported microversion", func(status, fallbackStatus int, invalidBody bool) {
				requests := 0
				client := newServerGroupTestClient(func(request *http.Request) (*http.Response, error) {
					requests++
					Expect(request.Method).To(Equal(http.MethodGet))
					Expect(request.URL.Path).To(Equal(operation.path))
					version := "2.64"
					responseStatus := status
					body := operation.wrap(`{"id":"group-id","policy":"anti-affinity","rules":{"max_server_per_host":2}}`)
					if requests > 1 {
						version = ""
						responseStatus = fallbackStatus
						body = operation.wrap(`{"id":"group-id","policies":["anti-affinity"]}`)
					}
					Expect(request.Header.Get("X-OpenStack-Nova-API-Version")).To(Equal(version))
					if invalidBody {
						body = "{"
					}
					return serverGroupTestResponse(responseStatus, body), nil
				})

				groups, err := operation.read(client)
				expectedRequests := 1
				if fallbackStatus != 0 {
					status, expectedRequests = fallbackStatus, 2
				}
				Expect(requests).To(Equal(expectedRequests))
				Expect(client.client.Microversion).To(Equal("2.7"))
				if invalidBody {
					Expect(err).To(HaveOccurred())
					return
				}
				if status != http.StatusOK {
					Expect(err).To(HaveOccurred())
					Expect(gophercloud.ResponseCodeIs(err, status)).To(BeTrue())
					return
				}
				Expect(err).NotTo(HaveOccurred())
				Expect(groups).To(HaveLen(1))
				group := groups[0]
				Expect(group.ID).To(Equal("group-id"))
				if fallbackStatus == 0 {
					Expect(group.Policy).To(Equal(ptr.To("anti-affinity")))
					Expect(group.Rules).To(Equal(&servergroups.Rules{MaxServerPerHost: 2}))
				} else {
					Expect(group.Rules).To(BeNil())
					Expect(group.Policies).To(Equal([]string{"anti-affinity"}))
				}
			},
				Entry("modern rules", http.StatusOK, 0, false),
				Entry("legacy fallback", http.StatusNotAcceptable, http.StatusOK, false),
				Entry("forbidden", http.StatusForbidden, 0, false),
				Entry("not found", http.StatusNotFound, 0, false),
				Entry("server error", http.StatusInternalServerError, 0, false),
				Entry("invalid response", http.StatusOK, 0, true),
				Entry("failed fallback", http.StatusNotAcceptable, http.StatusServiceUnavailable, false),
			)
		})
	}
})

type serverGroupRoundTripper func(*http.Request) (*http.Response, error)

func (transport serverGroupRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport(request)
	if response != nil {
		response.Request = request
	}
	return response, err
}

func newServerGroupTestClient(transport serverGroupRoundTripper) *ComputeClient {
	return &ComputeClient{client: &gophercloud.ServiceClient{
		ProviderClient: &gophercloud.ProviderClient{HTTPClient: http.Client{Transport: transport}},
		Endpoint:       "https://compute.example.com/",
		Type:           "compute",
		Microversion:   "2.7",
	}}
}

func serverGroupTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
