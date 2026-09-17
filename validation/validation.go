// SPDX-License-Identifier: MIT

// Package validation provides helpers to format and transform go-playground validator errors.
package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FieldError represents a single validation error on a field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FormatErrors takes an error (expected to be validator.ValidationErrors)
// and returns a structured slice of FieldErrors.
func FormatErrors(err error) []FieldError {
	if err == nil {
		return nil
	}

	var fieldErrs []FieldError

	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		for _, e := range validationErrors {
			fieldErrs = append(fieldErrs, FieldError{
				Field:   strings.ToLower(e.Field()),
				Message: msgForTag(e),
			})
		}
		return fieldErrs
	}

	// Fallback for non-validation errors
	return []FieldError{
		{
			Field:   "unknown",
			Message: err.Error(),
		},
	}
}

func msgForTag(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("Must be at least %s", e.Param())
	case "max":
		return fmt.Sprintf("Must be at most %s", e.Param())
	default:
		return "Invalid value"
	}
}
