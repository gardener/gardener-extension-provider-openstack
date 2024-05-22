// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultInformerPeriod = 10 * time.Second
)

// Timestamper is an interface around time package.
type Timestamper interface {
	Now() time.Time
}

// TimestamperFn is an implementation of the Timestamper interface using a function.
type TimestamperFn func() time.Time

// Now returns the value of time.Now().
func (t TimestamperFn) Now() time.Time {
	return t()
}

// DefaultTimer is the default implementation for Timestamper used in the package.
var DefaultTimer Timestamper = TimestamperFn(time.Now)

// TaskOption contains options for created flow tasks
type TaskOption struct {
	Dependencies []flow.TaskIDer
	Timeout      time.Duration
	DoIf         *bool
}

// Dependencies creates a TaskOption for dependencies
func Dependencies(dependencies ...flow.TaskIDer) TaskOption {
	return TaskOption{Dependencies: dependencies}
}

// Timeout creates a TaskOption for Timeout
func Timeout(timeout time.Duration) TaskOption {
	return TaskOption{Timeout: timeout}
}

// DoIf creates a TaskOption for DoIf
func DoIf(condition bool) TaskOption {
	return TaskOption{DoIf: ptr.To(condition)}
}

// BasicFlowContext provides logic for persisting the state and add tasks to the flow graph.
type BasicFlowContext struct {
	log           logr.Logger
	timer         Timestamper
	persistorLock sync.Mutex

	span      bool
	persistFn flow.TaskFn

	lastPersistedGeneration int64
	lastPersistedAt         time.Time
	PersistInterval         time.Duration
}

// NewBasicFlowContext creates a new `BasicFlowContext`.
func NewBasicFlowContext() *BasicFlowContext {
	flowContext := &BasicFlowContext{
		PersistInterval: 10 * time.Second,
		timer:           DefaultTimer,
	}
	return flowContext
}

// WithLogger injects the given logger into the context.
func (c *BasicFlowContext) WithLogger(log logr.Logger) *BasicFlowContext {
	c.log = log
	return c
}

// WithSpan when enabled will log the total execution time for the task on Info level.
func (c *BasicFlowContext) WithSpan() *BasicFlowContext {
	c.span = true
	return c
}

// WithPersist is the Task that will be called after each successful node directly after the node execution.
func (c *BasicFlowContext) WithPersist(task flow.TaskFn) *BasicFlowContext {
	c.persistFn = task
	return c
}

// PersistState persists the internal state to the provider status.
func (c *BasicFlowContext) PersistState(ctx context.Context) error {
	c.persistorLock.Lock()
	defer c.persistorLock.Unlock()

	return c.persistFn(ctx)
}

// AddTask adds a wrapped task for the given task function and options.
func (c *BasicFlowContext) AddTask(g *flow.Graph, name string, fn flow.TaskFn, options ...TaskOption) flow.TaskIDer {
	allOptions := TaskOption{}
	for _, opt := range options {
		if len(opt.Dependencies) > 0 {
			allOptions.Dependencies = append(allOptions.Dependencies, opt.Dependencies...)
		}
		if opt.Timeout > 0 {
			allOptions.Timeout = opt.Timeout
		}
		if opt.DoIf != nil {
			condition := true
			if allOptions.DoIf != nil {
				condition = *allOptions.DoIf
			}
			condition = condition && *opt.DoIf
			allOptions.DoIf = ptr.To(condition)
		}
	}

	tunedFn := fn
	if allOptions.Timeout > 0 {
		tunedFn = tunedFn.Timeout(allOptions.Timeout)
	}
	task := flow.Task{
		Name:   name,
		Fn:     c.wrapTaskFn(g.Name(), name, tunedFn),
		SkipIf: allOptions.DoIf != nil && !*allOptions.DoIf,
	}

	if len(allOptions.Dependencies) > 0 {
		task.Dependencies = flow.NewTaskIDs(allOptions.Dependencies...)
	}

	return g.Add(task)
}

// wrapTaskFn sets up the task function fn. It wraps it with the hooks
func (c *BasicFlowContext) wrapTaskFn(flowName, taskName string, fn flow.TaskFn) flow.TaskFn {
	return func(ctx context.Context) error {
		log := c.log.WithValues("flow", flowName, "task", taskName)
		ctx = logf.IntoContext(ctx, log)
		if c.persistFn != nil {
			defer func() {
				err := c.PersistState(ctx)
				if err != nil {
					log.Error(err, "failed to persist state")
				}
			}()
		}

		w := InformOnWaiting(log, defaultInformerPeriod, fmt.Sprintf("still trying to [%s]...", taskName))
		ctx = w.IntoContext(ctx)
		defer w.Done()

		var beforeTs time.Time
		if c.span {
			beforeTs = c.timer.Now()
		}
		err := fn(ctx)
		if c.span {
			log.Info(fmt.Sprintf("task finished - total execution time: %v", c.timer.Now().Sub(beforeTs)))
		}
		if err != nil {
			// don't wrap error with '%w', as otherwise the error context get lost
			err = fmt.Errorf("failed to %q: %s", taskName, err)
			return err
		}

		return nil
	}
}

// LogFromContext returns the log from the context when called within a task function added with the `AddTask` method. If no logger is present, a new noop-logger will be returned.
func LogFromContext(ctx context.Context) logr.Logger {
	if log, err := logr.FromContext(ctx); err == nil {
		return log
	}
	return logr.New(nil)
}
