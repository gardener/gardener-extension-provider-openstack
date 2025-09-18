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
	// genericNameRegex is used to validate openstack resource names. There are not many restrictions on the character set, but we will constrain brackets,braces and newlines.
	genericNameRegex     = `^[^{}\[\]\n]+$`
	validateResourceName = combineValidationFuncs(regex(genericNameRegex), notEmpty, maxLength(255))
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
// regex allows empty strings to pass through to allow combining with other checks for clearer error messages. Use
// notEmpty() to check for emptiness.
func regex(regex string) validateFunc[string] {
	compiled := regexp.MustCompile(regex)
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if name == "" {
			// allow empty strings to pass through, use notEmpty() to check for emptiness.
			return allErrs
		}
		if !compiled.MatchString(name) {
			allErrs = append(allErrs, field.Invalid(fld, name, fmt.Sprintf("does not match expected regex %s", compiled.String())))
		}
		return allErrs
	}
}

func notEmpty(name string, fld *field.Path) field.ErrorList {
	if utf8.RuneCountInString(name) == 0 {
		return field.ErrorList{field.Required(fld, name)}
	}
	return nil
}

func maxLength(max int) validateFunc[string] {
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if l := utf8.RuneCountInString(name); l > max {
			return field.ErrorList{field.Invalid(fld, name, fmt.Sprintf("must not be more than %d characters, got %d", max, l))}
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
