// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	"k8s.io/utils/ptr"
)

func TestCreateServerGroup(t *testing.T) {
	for _, test := range []struct {
		name             string
		policy           string
		maxServerPerHost *int32
		microversion     string
		body             string
	}{
		{
			name:   "legacy affinity",
			policy: "affinity",
			body:   `{"server_group":{"name":"workers","policies":["affinity"]}}`,
		},
		{
			name:   "legacy anti-affinity",
			policy: "anti-affinity",
			body:   `{"server_group":{"name":"workers","policies":["anti-affinity"]}}`,
		},
		{
			name:         "soft anti-affinity",
			policy:       "soft-anti-affinity",
			microversion: "2.15",
			body:         `{"server_group":{"name":"workers","policies":["soft-anti-affinity"]}}`,
		},
		{
			name:         "soft affinity",
			policy:       "soft-affinity",
			microversion: "2.15",
			body:         `{"server_group":{"name":"workers","policies":["soft-affinity"]}}`,
		},
		{
			name:             "two servers per host",
			policy:           "anti-affinity",
			maxServerPerHost: ptr.To[int32](2),
			microversion:     "2.64",
			body:             `{"server_group":{"name":"workers","policy":"anti-affinity","rules":{"max_server_per_host":2}}}`,
		},
		{
			name:             "explicit one server per host",
			policy:           "anti-affinity",
			maxServerPerHost: ptr.To[int32](1),
			microversion:     "2.64",
			body:             `{"server_group":{"name":"workers","policy":"anti-affinity","rules":{"max_server_per_host":1}}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := newServerGroupTestClient(func(request *http.Request) (*http.Response, error) {
				requests++
				if request.Method != http.MethodPost || request.URL.Path != "/os-server-groups" {
					t.Errorf("unexpected request: %s %s", request.Method, request.URL)
				}
				if got := request.Header.Get("X-OpenStack-Nova-API-Version"); got != test.microversion {
					t.Errorf("microversion = %q, want %q", got, test.microversion)
				}
				var actual, expected any
				if err := json.NewDecoder(request.Body).Decode(&actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(test.body), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("body = %#v, want %#v", actual, expected)
				}
				return serverGroupTestResponse(http.StatusOK, `{"server_group":{"id":"group-id","name":"workers"}}`), nil
			})

			group, err := client.CreateServerGroup(context.Background(), "workers", test.policy, test.maxServerPerHost)
			if err != nil {
				t.Fatal(err)
			}
			if group.ID != "group-id" {
				t.Errorf("group ID = %q, want group-id", group.ID)
			}
			if requests != 1 {
				t.Errorf("requests = %d, want 1", requests)
			}
			if client.client.Microversion != "2.7" {
				t.Errorf("shared microversion changed to %q", client.client.Microversion)
			}
		})
	}
}

func TestCreateServerGroupDoesNotDropRulesOnUnsupportedMicroversion(t *testing.T) {
	requests := 0
	client := newServerGroupTestClient(func(_ *http.Request) (*http.Response, error) {
		requests++
		return serverGroupTestResponse(http.StatusNotAcceptable, `{}`), nil
	})
	_, err := client.CreateServerGroup(context.Background(), "workers", "anti-affinity", ptr.To[int32](2))
	if !gophercloud.ResponseCodeIs(err, http.StatusNotAcceptable) {
		t.Fatalf("error = %v, want HTTP 406", err)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1 (no retry without rules)", requests)
	}
	if client.client.Microversion != "2.7" {
		t.Errorf("shared microversion changed to %q", client.client.Microversion)
	}
}

func TestReadServerGroups(t *testing.T) {
	for _, operation := range []struct {
		name string
		path string
		wrap func(string) string
		read func(*ComputeClient) ([]servergroups.ServerGroup, error)
	}{
		{
			name: "get",
			path: "/os-server-groups/group-id",
			wrap: func(group string) string { return `{"server_group":` + group + `}` },
			read: func(client *ComputeClient) ([]servergroups.ServerGroup, error) {
				group, err := client.GetServerGroup(context.Background(), "group-id")
				if err != nil {
					return nil, err
				}
				return []servergroups.ServerGroup{*group}, nil
			},
		},
		{
			name: "list",
			path: "/os-server-groups",
			wrap: func(group string) string { return `{"server_groups":[` + group + `]}` },
			read: func(client *ComputeClient) ([]servergroups.ServerGroup, error) {
				return client.ListServerGroups(context.Background())
			},
		},
	} {
		for _, test := range []struct {
			name           string
			status         int
			fallbackStatus int
			invalidBody    bool
		}{
			{name: "modern rules", status: http.StatusOK},
			{name: "legacy fallback", status: http.StatusNotAcceptable, fallbackStatus: http.StatusOK},
			{name: "forbidden", status: http.StatusForbidden},
			{name: "not found", status: http.StatusNotFound},
			{name: "server error", status: http.StatusInternalServerError},
			{name: "invalid response", status: http.StatusOK, invalidBody: true},
			{name: "failed fallback", status: http.StatusNotAcceptable, fallbackStatus: http.StatusServiceUnavailable},
		} {
			t.Run(operation.name+"/"+test.name, func(t *testing.T) {
				requests := 0
				client := newServerGroupTestClient(func(request *http.Request) (*http.Response, error) {
					requests++
					if request.Method != http.MethodGet || request.URL.Path != operation.path {
						t.Errorf("unexpected request: %s %s", request.Method, request.URL)
					}
					version := "2.64"
					status := test.status
					body := operation.wrap(`{"id":"group-id","policy":"anti-affinity","rules":{"max_server_per_host":2}}`)
					if requests > 1 {
						version = ""
						status = test.fallbackStatus
						body = operation.wrap(`{"id":"group-id","policies":["anti-affinity"]}`)
					}
					if got := request.Header.Get("X-OpenStack-Nova-API-Version"); got != version {
						t.Errorf("microversion = %q, want %q", got, version)
					}
					if test.invalidBody {
						body = "{"
					}
					return serverGroupTestResponse(status, body), nil
				})

				groups, err := operation.read(client)
				status, expectedRequests := test.status, 1
				if test.fallbackStatus != 0 {
					status, expectedRequests = test.fallbackStatus, 2
				}
				if requests != expectedRequests {
					t.Errorf("requests = %d, want %d", requests, expectedRequests)
				}
				if client.client.Microversion != "2.7" {
					t.Errorf("shared microversion changed to %q", client.client.Microversion)
				}
				if test.invalidBody {
					if err == nil {
						t.Fatal("expected invalid JSON error")
					}
					return
				}
				if status != http.StatusOK {
					if !gophercloud.ResponseCodeIs(err, status) {
						t.Fatalf("error = %v, want HTTP %d", err, status)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(groups) != 1 || groups[0].ID != "group-id" {
					t.Fatalf("unexpected groups: %#v", groups)
				}
				group := groups[0]
				if test.fallbackStatus == 0 {
					if group.Policy == nil || *group.Policy != "anti-affinity" || group.Rules == nil || group.Rules.MaxServerPerHost != 2 {
						t.Errorf("modern policy and rules not retained: %#v", group)
					}
				} else if group.Rules != nil || !reflect.DeepEqual(group.Policies, []string{"anti-affinity"}) {
					t.Errorf("unexpected legacy group: %#v", group)
				}
			})
		}
	}
}

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
