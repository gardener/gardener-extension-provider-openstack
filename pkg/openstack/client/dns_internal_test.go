// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	th "github.com/gophercloud/gophercloud/v2/testhelper"
	fakeclient "github.com/gophercloud/gophercloud/v2/testhelper/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNS", func() {
	const zoneID = "2150b1bf-dee2-4221-9d85-11f7886fb15f"

	var (
		fakeServer th.FakeServer
		dnsClient  *DNSClient
	)

	BeforeEach(func() {
		fakeServer = th.SetupHTTP()
		dnsClient = &DNSClient{client: fakeclient.ServiceClient(fakeServer)}
	})

	AfterEach(func() {
		fakeServer.Teardown()
	})

	// handleRecordSetList registers a handler for the recordsets list endpoint returning the
	// given JSON array of recordsets, mimicking Designate's server-side LIKE filtering which
	// returns siblings sharing the suffix in addition to (or instead of) the exact match.
	handleRecordSetList := func(recordsetsJSON string) {
		fakeServer.Mux.HandleFunc(fmt.Sprintf("/zones/%s/recordsets", zoneID), func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"recordsets": [%s], "metadata": {"total_count": 0}}`, recordsetsJSON)
		})
	}

	recordset := func(id, name, recType string, records ...string) string {
		recordsJSON := ""
		for i, rec := range records {
			if i > 0 {
				recordsJSON += ", "
			}
			recordsJSON += fmt.Sprintf("%q", rec)
		}
		return fmt.Sprintf(`{"id": %q, "name": %q, "type": %q, "records": [%s], "ttl": 300}`, id, name, recType, recordsJSON)
	}

	Describe("getRecordSet", func() {
		It("should return the recordset whose name exactly matches", func() {
			handleRecordSetList(strings.Join([]string{
				recordset("id-abc", "abc.sub.domain.tld.", "A", "1.1.1.1"),
				recordset("id-wildcard", "*.sub.domain.tld.", "A", "2.2.2.2"),
				recordset("id-def", "def.sub.domain.tld.", "A", "1.1.1.1"),
			}, ", "))

			rs, err := dnsClient.getRecordSet(context.Background(), zoneID, "*.sub.domain.tld", "A")
			Expect(err).NotTo(HaveOccurred())
			Expect(rs).NotTo(BeNil())
			Expect(rs.ID).To(Equal("id-wildcard"))
			Expect(rs.Records).To(Equal([]string{"2.2.2.2"}))
		})

		It("should match a wildcard returned in the escaped \\052 form", func() {
			handleRecordSetList(strings.Join([]string{
				recordset("id-abc", "abc.sub.domain.tld.", "A", "1.1.1.1"),
				recordset("id-wildcard", "\\052.sub.domain.tld.", "A", "2.2.2.2"),
			}, ", "))

			rs, err := dnsClient.getRecordSet(context.Background(), zoneID, "*.sub.domain.tld", "A")
			Expect(err).NotTo(HaveOccurred())
			Expect(rs).NotTo(BeNil())
			Expect(rs.ID).To(Equal("id-wildcard"))
		})

		It("should return nil when only siblings are returned but the exact name is absent", func() {
			// This is the bug in issue #1434: Designate's LIKE filter returns siblings even
			// though the wildcard recordset does not exist. We must not treat that as a match.
			handleRecordSetList(strings.Join([]string{
				recordset("id-abc", "abc.sub.domain.tld.", "A", "1.1.1.1"),
				recordset("id-def", "def.sub.domain.tld.", "A", "1.1.1.1"),
			}, ", "))

			rs, err := dnsClient.getRecordSet(context.Background(), zoneID, "*.sub.domain.tld", "A")
			Expect(err).NotTo(HaveOccurred())
			Expect(rs).To(BeNil())
		})

		It("should return nil when no recordsets are returned", func() {
			handleRecordSetList("")

			rs, err := dnsClient.getRecordSet(context.Background(), zoneID, "abc.sub.domain.tld", "A")
			Expect(err).NotTo(HaveOccurred())
			Expect(rs).To(BeNil())
		})
	})
})
