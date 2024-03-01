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
	"k8s.io/utils/pointer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

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
	return TaskOption{DoIf: pointer.Bool(condition)}
}

// FlowStatePersistor persists the flat map to the provider status
type FlowStatePersistor func(ctx context.Context, flatMap FlatMap) error

// BasicFlowContext provides logic for persisting the state and add tasks to the flow graph.
type BasicFlowContext struct {
	Log logr.Logger

	exporter                StateExporter
	persistorLock           sync.Mutex
	flowStatePersistor      FlowStatePersistor
	lastPersistedGeneration int64
	lastPersistedAt         time.Time
	PersistInterval         time.Duration
}

// StateExporter knows how to export the internal state to a flat string map.
type StateExporter interface {
	// CurrentGeneration is a counter which increments on changes of the internal state.
	CurrentGeneration() int64
	// Exports all or parts of the internal state to a flat string map.
	ExportAsFlatMap() FlatMap
}

// NewBasicFlowContext creates a new `BasicFlowContext`.
func NewBasicFlowContext(log logr.Logger, exporter StateExporter, persistor FlowStatePersistor) *BasicFlowContext {
	flowContext := &BasicFlowContext{
		Log:                log,
		exporter:           exporter,
		flowStatePersistor: persistor,
		PersistInterval:    10 * time.Second,
	}
	return flowContext
}

// PersistState persists the internal state to the provider status if it has changed and force is true
// or it has not been persisted during the `PersistInterval`.
func (c *BasicFlowContext) PersistState(ctx context.Context, force bool) error {
	c.persistorLock.Lock()
	defer c.persistorLock.Unlock()

	if !force && c.lastPersistedAt.Add(c.PersistInterval).After(time.Now()) {
		return nil
	}
	currentGeneration := c.exporter.CurrentGeneration()
	if c.lastPersistedGeneration == currentGeneration {
		return nil
	}
	if c.flowStatePersistor != nil {
		newState := c.exporter.ExportAsFlatMap()
		if err := c.flowStatePersistor(ctx, newState); err != nil {
			return err
		}
	}
	c.lastPersistedGeneration = currentGeneration
	c.lastPersistedAt = time.Now()
	return nil
}

// LogFromContext returns the log from the context when called within a task function added with the `AddTask` method.
func (c *BasicFlowContext) LogFromContext(ctx context.Context) logr.Logger {
	if log, err := logr.FromContext(ctx); err != nil {
		return c.Log
	} else {
		return log
	}
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
			allOptions.DoIf = pointer.Bool(condition)
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

func (c *BasicFlowContext) wrapTaskFn(flowName, taskName string, fn flow.TaskFn) flow.TaskFn {
	return func(ctx context.Context) error {
		taskCtx := logf.IntoContext(ctx, c.Log.WithValues("flow", flowName, "task", taskName))
		err := fn(taskCtx)
		if err != nil {
			// don't wrap error with '%w', as otherwise the error context get lost
			err = fmt.Errorf("failed to %s: %s", taskName, err)
		}
		if perr := c.PersistState(taskCtx, false); perr != nil {
			if err != nil {
				c.Log.Error(perr, "persisting state failed")
			} else {
				err = perr
			}
		}
		return err
	}
}
