// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInfraflow(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Infraflow Suite")
}
