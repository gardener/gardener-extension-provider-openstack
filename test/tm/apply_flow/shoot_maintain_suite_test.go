// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply_flow_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShootMaintain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ShootMaintain Suite")
}
