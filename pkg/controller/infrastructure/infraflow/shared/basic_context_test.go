// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
)

type testFlowContext struct {
	shared.BasicFlowContext
	state shared.Whiteboard
}

func newTestFlowContext(log logr.Logger, state shared.Whiteboard, persistor flow.TaskFn) *testFlowContext {
	return &testFlowContext{
		BasicFlowContext: *shared.NewBasicFlowContext().WithLogger(log).WithPersist(persistor),
		state:            state,
	}
}

var _ = Describe("BasicFlowContext", func() {
	It("should create and run a graph flow", func() {
		var (
			logBuffer           bytes.Buffer
			log                 = zap.New(zap.WriteTo(&logBuffer))
			state               = shared.NewWhiteboard()
			forcePersistorError = false
			forceTask3Error     = false
			persistedData       shared.FlatMap
			persistCallCount    = 0
			persistor           = func(_ context.Context) error {
				if forcePersistorError {
					return fmt.Errorf("forced persistor error")
				}
				persistedData = shared.FlatMap{}
				for k, v := range state.ExportAsFlatMap() {
					persistedData[k] = v
				}
				persistCallCount++
				return nil
			}
			ctx = context.Background()
			err error
		)

		c := newTestFlowContext(log, state, persistor)
		By("create a new graph", func() {
			g := flow.NewGraph("test")
			task1 := c.AddTask(g, "task1",
				func(_ context.Context) error {
					c.state.Set("task1", "done")
					return nil
				},
				shared.DoIf(false), shared.DoIf(true))
			task2 := c.AddTask(g, "task2",
				func(ctx context.Context) error {
					c.state.Set("task2", "done")
					log := shared.LogFromContext(ctx)
					log.Info("task2:foo")
					return nil
				},
				shared.Dependencies(task1))
			_ = c.AddTask(g, "task3",
				func(_ context.Context) error {
					c.state.SetPtr("afterTask2", c.state.Get("task2"))
					c.state.Set("task3", "done")
					if forceTask3Error {
						return fmt.Errorf("forceTask3Error")
					}
					return nil
				},
				shared.DoIf(true), shared.Dependencies(task2), shared.DoIf(true))

			f := g.Compile()
			Expect(f.Len()).To(Equal(3))
			err = f.Run(ctx, flow.Opts{Log: log})
			Expect(err).ToNot(HaveOccurred())

			Expect(c.state.Get("task1")).To(BeNil())
			Expect(logBuffer.String()).To(ContainSubstring("task2:foo"))

			Expect(c.state.Get("task2")).To(Equal(ptr.To("done")))
			Expect(c.state.Get("afterTask2")).To(Equal(c.state.Get("task2")))
			Expect(c.state.Get("task3")).To(Equal(ptr.To("done")))

			Expect(persistCallCount).To(Equal(2))
			Expect(persistedData["task2"]).To(Equal("done"))
			Expect(persistedData["afterTask2"]).To(Equal(*c.state.Get("task2")))
			Expect(persistedData["task3"]).To(Equal("done"))
		})
	})
})
