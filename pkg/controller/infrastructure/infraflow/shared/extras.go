// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/atomic"
)

type waiter struct {
	log           logr.Logger
	start         time.Time
	period        time.Duration
	message       atomic.String
	keysAndValues []any
	once          sync.Once
	done          chan struct{}

	ticker *time.Ticker
}

// InformOnWaiting periodically prints a message until stopped.
func InformOnWaiting(log logr.Logger, period time.Duration, message string, keysAndValues ...any) *waiter {
	w := &waiter{
		log:           log,
		start:         time.Now(),
		period:        period,
		keysAndValues: keysAndValues,
		done:          make(chan struct{}),
	}
	w.message.Store(message)
	go w.run()
	return w
}

func (w *waiter) UpdateMessage(message string) {
	w.message.Store(message)
}

func (w *waiter) run() {
	w.ticker = time.NewTicker(w.period)
	defer w.ticker.Stop()
OUTER:
	for {
		select {
		case <-w.done:
			break OUTER
		case <-w.ticker.C:
			delta := time.Since(w.start)
			w.log.Info(fmt.Sprintf("[%.fs] %s", delta.Seconds(), w.message.Load()), w.keysAndValues...)
		}
	}
}

func (w *waiter) IntoContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, w)
}

func (w *waiter) Done() {
	w.once.Do(
		func() {
			close(w.done)
		})
}

type contextKey struct{}

// FromContext retrieves a waiter from the current context or returns nil if there is none.
func FromContext(ctx context.Context) *waiter {
	v := ctx.Value(contextKey{})
	if v == nil {
		return nil
	}

	w, ok := v.(*waiter)
	if !ok {
		return nil
	}
	return w
}
