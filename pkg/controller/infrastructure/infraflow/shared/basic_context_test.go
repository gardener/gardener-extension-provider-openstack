// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shared_test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
)

type testFlowContext struct {
	shared.BasicFlowContext
	state shared.Whiteboard
}

func newTestFlowContext(log logr.Logger, state shared.Whiteboard, persistor shared.FlowStatePersistor) *testFlowContext {
	return &testFlowContext{
		BasicFlowContext: *shared.NewBasicFlowContext(log, state, persistor),
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
			persistor           = func(ctx context.Context, data shared.FlatMap) error {
				if forcePersistorError {
					return fmt.Errorf("forced persistor error")
				}
				persistedData = shared.FlatMap{}
				for k, v := range data {
					persistedData[k] = v
				}
				persistCallCount++
				return nil
			}
			ctx           = context.Background()
			err           error
			expectedData1 = shared.FlatMap{
				"key1": "id1",
				"key2": "id2b",
			}
		)

		c := newTestFlowContext(log, state, persistor)
		c.PersistInterval = 10 * time.Millisecond

		By("persists only if needed", func() {
			c.state.Set("key1", "id1")
			err = c.PersistState(ctx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(persistCallCount).To(Equal(1))

			// no immediate persistence with force = false
			c.state.Set("key2", "id2")
			err = c.PersistState(ctx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(persistCallCount).To(Equal(1))

			// persist after interval
			time.Sleep(c.PersistInterval)
			err = c.PersistState(ctx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(persistCallCount).To(Equal(2))

			// immediate persistence with force = true
			c.state.Set("key2", "id2b")
			err = c.PersistState(ctx, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(persistCallCount).To(Equal(3))
			Expect(persistedData).To(Equal(expectedData1))

			// no change, no persist call in backend
			err = c.PersistState(ctx, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(persistCallCount).To(Equal(3))
		})

		By("logs with context", func() {
			g := flow.NewGraph("test")
			task1 := c.AddTask(g, "task1",
				func(ctx context.Context) error {
					c.state.Set("task1", "done")
					return nil
				},
				shared.DoIf(false), shared.DoIf(true))
			task2 := c.AddTask(g, "task2",
				func(ctx context.Context) error {
					c.state.Set("task2", "done")
					task1Log := c.LogFromContext(ctx)
					task1Log.Info("message from task2")
					return nil
				},
				shared.Dependencies(task1))
			_ = c.AddTask(g, "task3",
				func(ctx context.Context) error {
					c.state.SetPtr("afterTask2", c.state.Get("task2"))
					time.Sleep(c.PersistInterval)
					c.state.Set("task3", "done")
					if forceTask3Error {
						return fmt.Errorf("forceTask3Error")
					}
					return nil
				},
				shared.DoIf(true), shared.Dependencies(task2), shared.DoIf(true))

			f := g.Compile()
			Expect(f.Len()).To(Equal(3))

			forcePersistorError = true
			forceTask3Error = false
			err = f.Run(ctx, flow.Opts{Log: log})
			Expect(err.Error()).To(ContainSubstring(`flow "test" encountered task errors: [task "task3" failed: forced persistor error]`))

			Expect(c.state.Get("task1")).To(BeNil())
			Expect(c.state.Get("task2")).To(Equal(pointer.String("done")))
			Expect(c.state.Get("afterTask2")).To(Equal(c.state.Get("task2")))
			Expect(c.state.Get("task3")).To(Equal(pointer.String("done")))
			Expect(logBuffer.String()).To(ContainSubstring(`"task":"[Skipped] task1"`))
			Expect(logBuffer.String()).To(ContainSubstring(`"task":"task2"`))
			Expect(logBuffer.String()).To(ContainSubstring(`"msg":"message from task2"`))
			Expect(logBuffer.String()).To(ContainSubstring(`"task":"task3"`))

			forcePersistorError = false
			forceTask3Error = true
			c.state.Set("task1", "")
			err = f.Run(ctx, flow.Opts{Log: log})
			Expect(err.Error()).To(ContainSubstring(`flow "test" encountered task errors: [task "task3" failed: failed to task3: forceTask3Error]`))

			forcePersistorError = false
			forceTask3Error = false
			c.state.Set("task1", "")
			err = f.Run(ctx, flow.Opts{Log: log})
			Expect(err).To(BeNil())
		})
	})
})
