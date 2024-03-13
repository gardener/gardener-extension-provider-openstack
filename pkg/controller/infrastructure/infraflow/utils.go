// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/atomic"
)

func findExisting[T any](id *string, name string,
	getter func(id string) (*T, error),
	finder func(name string) ([]*T, error),
	selector ...func(item *T) bool) (*T, error) {

	if id != nil {
		found, err := getter(*id)
		if err != nil {
			return nil, err
		}
		if found != nil && (len(selector) == 0 || selector[0](found)) {
			return found, nil
		}
	}

	found, err := finder(name)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	if len(selector) > 0 {
		for _, item := range found {
			if selector[0](item) {
				return item, nil
			}
		}
		return nil, nil
	}
	return found[0], nil
}

type waiter struct {
	log           logr.Logger
	start         time.Time
	period        time.Duration
	message       atomic.String
	keysAndValues []any
	done          chan struct{}
}

func informOnWaiting(log logr.Logger, period time.Duration, message string, keysAndValues ...any) *waiter {
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
	ticker := time.NewTicker(w.period)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			delta := int(time.Since(w.start).Seconds())
			w.log.Info(fmt.Sprintf("%s [%ds]", w.message.Load(), delta), w.keysAndValues...)
		}
	}
}

func (w *waiter) Done(err error) {
	w.done <- struct{}{}
	if err != nil {
		w.log.Info("failed: " + err.Error())
	} else {
		w.log.Info("succeeded")
	}
}

func copyMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := map[string]string{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func sliceToPtr[T any](slice []T) []*T {
	res := make([]*T, len(slice))
	for _, t := range slice {
		res = append(res, &t)
	}
	return res
}
