// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot_suite_test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	_ "github.com/gardener/gardener-extension-provider-openstack/test/integration/healthcheck"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/config"
	"github.com/gardener/gardener/test/framework/reporter"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	. "github.com/onsi/gomega"
)

var (
	configFilePath = flag.String("config", "", "Specify the configuration file")
	esIndex        = flag.String("es-index", "gardener-testsuite", "Specify the elastic search index where the report should be ingested")
	reportFilePath = flag.String("report-file", "/tmp/shoot_res.json", "Specify the file to write the test results")
)

func TestMain(m *testing.M) {
	framework.RegisterShootFrameworkFlags()
	flag.Parse()

	if err := config.ParseConfigForFlags(*configFilePath, flag.CommandLine); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	RegisterFailHandler(Fail)

	AfterSuite(func() {
		framework.CommonAfterSuite()
	})

	os.Exit(m.Run())
}

func TestGardenerSuite(t *testing.T) {
	RunSpecs(t, "Provider-openstack Test Suite")
}

var _ = ReportAfterSuite("Report to Elasticsearch", func(report Report) {
	//nolint:staticcheck // Ignore SA1019 until NewDeprecatedGardenerESReporter is reworked to be compatible with ginkgo v2 reporting.
	reporters.ReportViaDeprecatedReporter(reporter.NewDeprecatedGardenerESReporter(*reportFilePath, *esIndex), report)
})
