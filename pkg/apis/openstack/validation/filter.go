// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	guuid "github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	// genericNameRegex is used to validate openstack resource names. There are not many restrictions on the character set, but we will constrain brackets,braces and newlines.
	genericNameRegex            = `^[^{}\[\]\n]+$`
	validateResourceName        = combineValidationFuncs(regex(genericNameRegex), notEmpty, maxLength(255))
	validateDomainName          = hideSensitiveValue(noWhitespace)
	validateTenantName          = combineValidationFuncs(noWhitespace, maxLength(64))
	validateUserName            = hideSensitiveValue(noWhitespace)
	validatePassword            = hideSensitiveValue(noNewlines)
	validateAppCredentialID     = hideSensitiveValue(noWhitespace)
	validateAppCredentialName   = hideSensitiveValue(noWhitespace)
	validateAppCredentialSecret = hideSensitiveValue(noWhitespace)
	validateInsecure            = allowedValues("true", "false")
)

type validateFunc[T any] func(T, *field.Path) field.ErrorList

// hideSensitiveValue wraps a validation function to hide the actual value in error messages
func hideSensitiveValue(fn validateFunc[string]) validateFunc[string] {
	return func(value string, fld *field.Path) field.ErrorList {
		errs := fn(value, fld)
		// Replace the actual value with "(hidden)" in all error messages
		for i := range errs {
			if errs[i].Type == field.ErrorTypeInvalid || errs[i].Type == field.ErrorTypeRequired {
				errs[i].BadValue = "(hidden)"
			}
		}
		return errs
	}
}

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
			allErrs = append(allErrs, field.Invalid(fld, name, fmt.Sprintf("does not match expected regex %q", compiled.String())))
		}
		return allErrs
	}
}

func notEmpty(name string, fld *field.Path) field.ErrorList {
	if utf8.RuneCountInString(name) == 0 {
		return field.ErrorList{field.Required(fld, "cannot be empty")}
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

func noWhitespace(value string, fld *field.Path) field.ErrorList {
	if value == "" {
		return nil // Allow empty
	}
	if strings.TrimSpace(value) != value {
		return field.ErrorList{field.Invalid(fld, value, "must not contain leading or trailing whitespace")}
	}
	return nil
}

func noNewlines(value string, fld *field.Path) field.ErrorList {
	if value == "" {
		return nil // Allow empty
	}
	if strings.Trim(value, "\n\r") != value {
		return field.ErrorList{field.Invalid(fld, value, "must not contain leading or trailing newlines")}
	}
	return nil
}

func allowedValues(allowed ...string) validateFunc[string] {
	allowedSet := sets.New(allowed...)
	return func(value string, fldPath *field.Path) field.ErrorList {
		if value == "" {
			return nil
		}
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !allowedSet.Has(normalized) {
			return field.ErrorList{field.NotSupported(fldPath, value, allowed)}
		}
		return nil
	}
}
