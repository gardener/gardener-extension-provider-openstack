// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	utilsnet "k8s.io/utils/net"
)

// IsEmptyString checks whether a string is empty
func IsEmptyString(s *string) bool {
	return s == nil || *s == ""
}

// IsStringPtrValueEqual checks whether the value of string pointer `a` is equal to value of string `b`.
func IsStringPtrValueEqual(a *string, b string) bool {
	return a != nil && *a == b
}

// StringEqual compares to strings
func StringEqual(a, b *string) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

// SetStringValue sets an optional string value in a string map
// if the value is defined and not empty
func SetStringValue(values map[string]interface{}, key string, value *string) {
	if !IsEmptyString(value) {
		values[key] = *value
	}
}

// SimpleMatch returns whether the given pattern matches the given text.
// It also returns a score indicating the match between `pattern` and `text`. The higher the score the higher the match.
// Only simple wildcard patterns are supposed to be passed, e.g. '*', 'tex*'.
func SimpleMatch(pattern, text string) (bool, int) {
	const wildcard = "*"
	if pattern == wildcard {
		return true, 0
	}
	if pattern == text {
		return true, len(text)
	}
	if strings.HasSuffix(pattern, wildcard) && strings.HasPrefix(text, pattern[:len(pattern)-1]) {
		s := strings.SplitAfterN(text, pattern[:len(pattern)-1], 2)
		return true, len(s[0])
	}
	if strings.HasPrefix(pattern, wildcard) && strings.HasSuffix(text, pattern[1:]) {
		i := strings.LastIndex(text, pattern[1:])
		return true, len(text) - i
	}

	return false, 0
}

// ComputeEgressCIDRs converts an IP to a CIDR depending on the IP family.
func ComputeEgressCIDRs(ips []string) []string {
	var result []string
	for _, ip := range ips {
		switch {
		case utilsnet.IsIPv4String(ip):
			result = append(result, fmt.Sprintf("%s/32", ip))
		case utilsnet.IsIPv6String(ip):
			result = append(result, fmt.Sprintf("%s/128", ip))
		}
	}
	return result
}

// Retry performs a function with retries, delay, and a max number of attempts
func Retry(maxRetries int, delay time.Duration, log logr.Logger, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		log.Info(fmt.Sprintf("Attempt %d failed, retrying in %v: %v", i+1, delay, err))
		time.Sleep(delay)
	}
	return err
}
