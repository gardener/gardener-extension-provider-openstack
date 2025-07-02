//  SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	guuid "github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	// genericNameRegex is used to validate openstack resource names. They generally do not impose any restrictions on the characters used, but we constrain curly braces or newlines.
	genericNameRegex     = `^[^{}\[\]\n]+$`
	validateResourceName = combineValidationFuncs(regex(genericNameRegex), maxLength(255))
)

type validateFunc[T any] func(T, *field.Path) field.ErrorList

// combineValidationFuncs validates a value against a list of filters.
func combineValidationFuncs[T any](filters ...validateFunc[T]) validateFunc[T] {
	return func(t T, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		for _, f := range filters {
			allErrs = append(allErrs, f(t, fld)...)
		}
		return allErrs
	}
}

// regex returns a filterFunc that validates a string against a regular expression.
func regex(regex string) validateFunc[string] {
	compiled := regexp.MustCompile(regex)
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if name == "" {
			return allErrs // Allow empty strings to pass through
		}
		if !compiled.MatchString(name) {
			allErrs = append(allErrs, field.Invalid(fld, name, fmt.Sprintf("does not match expected regex %s", compiled.String())))
		}
		return allErrs
	}
}

func maxLength(max int) validateFunc[string] {
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if utf8.RuneCountInString(name) > max {
			return field.ErrorList{field.Invalid(fld, name, fmt.Sprintf("must not be more than %d characters, got %d", max, len(name)))}
		}
		return allErrs
	}
}

func uuid(u string, fld *field.Path) field.ErrorList {
	if _, err := guuid.Parse(u); err != nil {
		return field.ErrorList{field.Invalid(fld, u, fmt.Sprintf("must be a valid UUID: %v", err))}
	}
	return nil
}
