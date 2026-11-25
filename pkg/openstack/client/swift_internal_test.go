// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/objects"
	th "github.com/gophercloud/gophercloud/v2/testhelper"
	fakeclient "github.com/gophercloud/gophercloud/v2/testhelper/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type swiftRoundTripper func(*http.Request) (*http.Response, error)

func (f swiftRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

var _ = Describe("Swift deletion", func() {
	const container = "backups"
	var (
		server          th.FakeServer
		storage         *StorageClient
		objectNames     []string
		listStatus      int
		deleteStatuses  []int
		deleteCalls     int
		bulkStatus      int
		bulkLimit       int
		bulkLimitInBody bool
		bulkResponses   []string
		batches         [][]string
		prefixes        []string
	)

	BeforeEach(func() {
		server = th.SetupHTTP()
		storage = &StorageClient{client: fakeclient.ServiceClient(server)}
		objectNames = []string{"shoot/snapshot"}
		listStatus, bulkStatus = http.StatusOK, http.StatusOK
		bulkLimit, bulkLimitInBody = 0, false
		deleteStatuses = []int{http.StatusNoContent}
		deleteCalls = 0
		bulkResponses, batches, prefixes = nil, nil, nil

		server.Mux.HandleFunc("/"+container, func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			switch r.Method {
			case http.MethodDelete:
				status := deleteStatuses[min(deleteCalls, len(deleteStatuses)-1)]
				deleteCalls++
				// Keep a regression to unbounded recursion from hanging the suite.
				if deleteCalls > 2 {
					status = http.StatusInternalServerError
				}
				w.WriteHeader(status)
			case http.MethodGet:
				prefixes = append(prefixes, r.URL.Query().Get("prefix"))
				if listStatus != http.StatusNoContent {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(listStatus)
				if listStatus != http.StatusOK {
					return
				}
				start := 0
				if marker := r.URL.Query().Get("marker"); marker != "" {
					Expect(objectNames).To(ContainElement(marker))
					start = slices.Index(objectNames, marker) + 1
				}
				page := make([]objects.Object, 0)
				for _, name := range objectNames[start:min(start+1000, len(objectNames))] {
					page = append(page, objects.Object{Name: name})
				}
				Expect(json.NewEncoder(w).Encode(page)).To(Succeed())
			default:
				Fail("unexpected container request: " + r.Method)
			}
		})
		server.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			Expect(r.URL.Path).To(Equal("/"))
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Query().Get("bulk-delete")).To(Equal("true"))
			Expect(r.Header.Get("Accept")).To(Equal("application/json"))
			body, err := io.ReadAll(r.Body)
			Expect(err).NotTo(HaveOccurred())
			paths := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
			batches = append(batches, paths)
			w.Header().Set("Content-Type", "application/json")
			if bulkLimit > 0 && len(paths) > bulkLimit {
				if bulkLimitInBody {
					fmt.Fprint(w, `{"Response Status":"413 Request Entity Too Large","Errors":[]}`)
				} else {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
				}
				return
			}
			w.WriteHeader(bulkStatus)
			if len(bulkResponses) >= len(batches) {
				fmt.Fprint(w, bulkResponses[len(batches)-1])
				return
			}
			Expect(json.NewEncoder(w).Encode(objects.BulkDeleteResponse{
				ResponseStatus: "200 OK", NumberDeleted: len(paths),
			})).To(Succeed())
		})
	})

	AfterEach(func() { server.Teardown() })

	Describe("DeleteContainerIfExists", func() {
		DescribeTable("handles container HTTP results", func(status int, success bool) {
			deleteStatuses = []int{status}
			err := storage.DeleteContainerIfExists(context.Background(), container)
			if success {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(gophercloud.ResponseCodeIs(err, status)).To(BeTrue())
			}
			Expect(deleteCalls).To(Equal(1))
			Expect(batches).To(BeEmpty())
		},
			Entry("success", http.StatusNoContent, true),
			Entry("absent", http.StatusNotFound, true),
			Entry("forbidden", http.StatusForbidden, false),
			Entry("server failure", http.StatusInternalServerError, false),
		)

		It("propagates transport failures", func() {
			failure := errors.New("connection failed")
			storage.client.HTTPClient.Transport = swiftRoundTripper(func(*http.Request) (*http.Response, error) {
				return nil, failure
			})
			Expect(storage.DeleteContainerIfExists(context.Background(), container)).To(MatchError(failure))
		})

		DescribeTable("propagates context errors", func(deadline bool) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			expected := context.Canceled
			if deadline {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				expected = context.DeadlineExceeded
			}
			Expect(storage.DeleteContainerIfExists(ctx, container)).To(MatchError(expected))
		}, Entry("canceled", false), Entry("deadline exceeded", true))

		It("propagates reauthentication failures", func() {
			deleteStatuses = []int{http.StatusUnauthorized}
			storage.client.ReauthFunc = func(context.Context) error { return errors.New("reauthentication failed") }
			err := storage.DeleteContainerIfExists(context.Background(), container)
			var reauthErr *gophercloud.ErrUnableToReauthenticate
			Expect(errors.As(err, &reauthErr)).To(BeTrue())
		})

		DescribeTable("retries a conflict only once after cleanup", func(status int, success bool) {
			deleteStatuses = []int{http.StatusConflict, status}
			err := storage.DeleteContainerIfExists(context.Background(), container)
			if success {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(gophercloud.ResponseCodeIs(err, status)).To(BeTrue())
			}
			Expect(deleteCalls).To(Equal(2))
			Expect(batches).To(HaveLen(1))
			Expect(prefixes).To(ConsistOf("", ""))
		},
			Entry("deleted", http.StatusNoContent, true),
			Entry("concurrently deleted", http.StatusNotFound, true),
			Entry("still nonempty", http.StatusConflict, false),
			Entry("forbidden after cleanup", http.StatusForbidden, false),
		)

		It("propagates cleanup failure without retrying the container deletion", func() {
			deleteStatuses = []int{http.StatusConflict}
			bulkResponses = []string{`{"Response Status":"502 Bad Gateway","Errors":[["backups/shoot/snapshot","503 Service Unavailable"]]}`}
			Expect(storage.DeleteContainerIfExists(context.Background(), container)).To(HaveOccurred())
			Expect(deleteCalls).To(Equal(1))
		})

		It("propagates transport failure on the post-cleanup retry", func() {
			deleteStatuses = []int{http.StatusConflict}
			failure := errors.New("connection failed after cleanup")
			storage.client.HTTPClient.Transport = swiftRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodDelete && len(batches) > 0 {
					return nil, failure
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			Expect(storage.DeleteContainerIfExists(context.Background(), container)).To(MatchError(failure))
			Expect(batches).To(HaveLen(1))
			Expect(deleteCalls).To(Equal(1))
		})

		It("handles a container disappearing before cleanup", func() {
			deleteStatuses = []int{http.StatusConflict, http.StatusNotFound}
			listStatus = http.StatusNotFound
			Expect(storage.DeleteContainerIfExists(context.Background(), container)).To(Succeed())
			Expect(deleteCalls).To(Equal(2))
			Expect(batches).To(BeEmpty())
		})
	})

	Describe("DeleteObjectsWithPrefix", func() {
		DescribeTable("validates the bulk response body", func(body string, success bool) {
			bulkResponses = []string{body}
			err := storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			if success {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
			Expect(batches).To(HaveLen(1))
		},
			Entry("deleted", `{"Response Status":"200 OK","Errors":[],"Number Deleted":1,"Number Not Found":0}`, true),
			Entry("disappeared after listing", `{"Response Status":"200 OK","Errors":[],"Number Deleted":0,"Number Not Found":1}`, true),
			Entry("object failure despite successful status", `{"Response Status":"200 OK","Errors":[["backups/shoot/snapshot","403 Forbidden"]],"Number Deleted":1}`, false),
			Entry("failed operation", `{"Response Status":"502 Bad Gateway","Errors":[],"Number Deleted":1}`, false),
			Entry("oversized request", `{"Response Status":"413 Request Entity Too Large","Errors":[]}`, false),
			Entry("unrecognized status", `{"Response Status":"foo","Number Deleted":1}`, false),
			Entry("missing status", `{"Number Deleted":1}`, false),
			Entry("unaccounted object", `{"Response Status":"200 OK"}`, false),
			Entry("excess count", `{"Response Status":"200 OK","Number Deleted":2}`, false),
			Entry("negative count", `{"Response Status":"200 OK","Number Deleted":-1,"Number Not Found":2}`, false),
			Entry("negative not-found count", `{"Response Status":"200 OK","Number Deleted":2,"Number Not Found":-1}`, false),
			Entry("noncanonical object 404", `{"Response Status":"200 OK","Errors":[["backups/shoot/snapshot","404 Not Found"]],"Number Not Found":1}`, false),
			Entry("malformed error entry", `{"Response Status":"200 OK","Errors":[[]],"Number Deleted":1}`, false),
			Entry("malformed JSON", `{`, false),
			Entry("null", `null`, false),
		)

		DescribeTable("skips bulk deletion for empty or absent containers", func(status int) {
			objectNames = nil
			listStatus = status
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(Succeed())
			Expect(batches).To(BeEmpty())
		}, Entry("empty", http.StatusOK), Entry("no content", http.StatusNoContent), Entry("absent", http.StatusNotFound))

		It("accepts a mix of deleted and already absent objects", func() {
			objectNames = []string{"shoot/snapshot-1", "shoot/snapshot-2"}
			bulkResponses = []string{`{"Response Status":"200 OK","Errors":[],"Number Deleted":1,"Number Not Found":1}`}
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(Succeed())
		})

		It("includes one example object per distinct error status", func() {
			body, err := json.Marshal(objects.BulkDeleteResponse{
				ResponseStatus: "400 Bad Request",
				Errors: [][]string{
					{"backups/shoot/snapshot-1", "403 Forbidden"},
					{"backups/shoot/snapshot-2", "403 Forbidden"},
					{"backups/shoot/snapshot-3", "503 Service Unavailable"},
					{"backups/shoot/snapshot-4", "403 Forbidden"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			bulkResponses = []string{string(body)}
			err = storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			Expect(err).To(MatchError(`Swift bulk delete returned status "400 Bad Request" with 4 object errors; "403 Forbidden" (example object "backups/shoot/snapshot-1"); "503 Service Unavailable" (example object "backups/shoot/snapshot-3")`))
		})

		It("reports when more than five distinct error statuses are present", func() {
			response := objects.BulkDeleteResponse{ResponseStatus: "400 Bad Request"}
			for i, status := range []string{"400 Bad Request", "401 Unauthorized", "403 Forbidden", "409 Conflict", "500 Internal Server Error", "503 Service Unavailable", "504 Gateway Timeout"} {
				// Repeated statuses must not consume the slots for distinct statuses.
				for range 2 {
					response.Errors = append(response.Errors, []string{fmt.Sprintf("snapshot-%d", i), status})
				}
			}
			body, err := json.Marshal(response)
			Expect(err).NotTo(HaveOccurred())
			bulkResponses = []string{string(body)}
			err = storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("14 object errors"))
			Expect(err.Error()).To(ContainSubstring("2 additional distinct error statuses omitted"))
			Expect(strings.Count(err.Error(), "example object")).To(Equal(5))
			Expect(err.Error()).To(ContainSubstring(`"500 Internal Server Error" (example object "snapshot-4")`))
			Expect(err.Error()).NotTo(ContainSubstring("503 Service Unavailable"))
			Expect(err.Error()).NotTo(ContainSubstring("504 Gateway Timeout"))
		})

		It("does not truncate statuses or example object paths", func() {
			status := strings.Repeat("response", 40)
			objectStatus := "403 " + strings.Repeat("reason", 40) + "\n"
			objectPath := "backups/" + strings.Repeat("snapshot", 80) + "\n"
			body, err := json.Marshal(objects.BulkDeleteResponse{
				ResponseStatus: status,
				Errors:         [][]string{{objectPath, objectStatus}},
			})
			Expect(err).NotTo(HaveOccurred())
			bulkResponses = []string{string(body)}
			err = storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			Expect(err).To(MatchError(fmt.Sprintf("Swift bulk delete returned status %q with 1 object errors; %q (example object %q)", status, objectStatus, objectPath)))
		})

		It("reports malformed entries without hiding valid error details", func() {
			bulkResponses = []string{`{"Response Status":"400 Bad Request","Errors":[[],["snapshot"],["snapshot","403 Forbidden","extra"],["snapshot","503 Service Unavailable"]]}`}
			err := storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			Expect(err).To(MatchError(`Swift bulk delete returned status "400 Bad Request" with 4 object errors; "503 Service Unavailable" (example object "snapshot"); 3 malformed object error entries`))
		})

		It("propagates transport failures during bulk deletion", func() {
			failure := errors.New("bulk connection failed")
			storage.client.HTTPClient.Transport = swiftRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPost {
					return nil, failure
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(MatchError(failure))
		})

		It("propagates listing failures", func() {
			listStatus = http.StatusForbidden
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(HaveOccurred())
			Expect(batches).To(BeEmpty())
		})

		It("does not ignore HTTP errors at the account bulk endpoint", func() {
			bulkStatus = http.StatusNotFound
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(HaveOccurred())
		})

		It("preserves the prefix and escapes special characters in object paths", func() {
			objectNames = []string{"shoot/snapshot with space\n%"}
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(Succeed())
			Expect(prefixes).To(ConsistOf("shoot/", "shoot/"))
			Expect(batches).To(Equal([][]string{{container + "/" + url.PathEscape(objectNames[0])}}))
		})

		DescribeTable("reduces oversized batches and keeps the smaller size", func(inBody bool) {
			objectNames = []string{"shoot/1", "shoot/2", "shoot/3", "shoot/4", "shoot/5", "shoot/6", "shoot/7"}
			bulkLimit, bulkLimitInBody = 2, inBody
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(Succeed())
			Expect(batches).To(HaveLen(9))
			Expect(batches[0]).To(HaveLen(7))
			Expect(batches[1]).To(Equal(batches[0][:3]))
			for i, batch := range batches[2:] {
				Expect(batch).To(Equal([]string{container + "/" + url.PathEscape(objectNames[i])}))
			}
		}, Entry("HTTP 413", false), Entry("body-level 413", true))

		DescribeTable("returns an error when even one object is rejected", func(inBody bool) {
			objectNames = []string{"shoot/1", "shoot/2", "shoot/3", "shoot/4", "shoot/5", "shoot/6", "shoot/7"}
			if inBody {
				bulkResponses = []string{
					`{"Response Status":"413 Request Entity Too Large"}`,
					`{"Response Status":"413 Request Entity Too Large"}`,
					`{"Response Status":"413 Request Entity Too Large"}`,
				}
			} else {
				bulkStatus = http.StatusRequestEntityTooLarge
			}
			err := storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			Expect(err).To(HaveOccurred())
			if inBody {
				Expect(err.Error()).To(ContainSubstring("413 Request Entity Too Large"))
			} else {
				Expect(gophercloud.ResponseCodeIs(err, http.StatusRequestEntityTooLarge)).To(BeTrue())
			}
			Expect(batches).To(HaveLen(3))
			Expect(batches[0]).To(HaveLen(7))
			Expect(batches[1]).To(HaveLen(3))
			Expect(batches[2]).To(HaveLen(1))
		}, Entry("HTTP 413", false), Entry("body-level 413", true))

		DescribeTable("does not reduce batches for other failures", func(inBody bool) {
			objectNames = []string{"shoot/1", "shoot/2"}
			if inBody {
				bulkResponses = []string{`{"Response Status":"502 Bad Gateway","Errors":[["backups/shoot/1","503 Service Unavailable"]]}`}
			} else {
				bulkStatus = http.StatusServiceUnavailable
			}
			Expect(storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")).To(HaveOccurred())
			Expect(batches).To(HaveLen(1))
			Expect(batches[0]).To(HaveLen(2))
		}, Entry("HTTP failure", false), Entry("body-level failure", true))

		It("honors cancellation while retrying a smaller batch", func() {
			objectNames = []string{"shoot/1", "shoot/2"}
			bulkLimit = 1
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			posts := 0
			storage.client.HTTPClient.Transport = swiftRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPost {
					posts++
					if posts == 2 {
						cancel()
					}
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			Expect(storage.DeleteObjectsWithPrefix(ctx, container, "shoot/")).To(MatchError(context.Canceled))
			Expect(posts).To(Equal(2))
		})

		DescribeTable("batches objects from all listing pages and stops on failure", func(failSecond bool) {
			objectNames = make([]string, 2*swiftBulkDeleteBatchSize+1)
			for i := range objectNames {
				objectNames[i] = fmt.Sprintf("shoot/snapshot-%05d", i)
			}
			if failSecond {
				bulkResponses = []string{
					fmt.Sprintf(`{"Response Status":"200 OK","Number Deleted":%d}`, swiftBulkDeleteBatchSize),
					`{"Response Status":"502 Bad Gateway"}`,
				}
			}
			err := storage.DeleteObjectsWithPrefix(context.Background(), container, "shoot/")
			if failSecond {
				Expect(err).To(HaveOccurred())
				Expect(batches).To(HaveLen(2))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(batches).To(HaveLen(3))
				Expect(batches[2]).To(HaveLen(1))
			}
			Expect(batches[0]).To(HaveLen(swiftBulkDeleteBatchSize))
			Expect(batches[1]).To(HaveLen(swiftBulkDeleteBatchSize))
			i := 0
			for _, batch := range batches {
				for _, path := range batch {
					Expect(path).To(Equal(container + "/" + url.PathEscape(objectNames[i])))
					i++
				}
			}
		}, Entry("all succeed", false), Entry("later batch fails", true))
	})
})
