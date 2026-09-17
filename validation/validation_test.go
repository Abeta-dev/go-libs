// SPDX-License-Identifier: MIT

package validation_test

import (
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/validation"
)

type TestUser struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Age   int    `validate:"min=18,max=120"`
	Code  string `validate:"len=5"` // Will hit default message
}

func TestFormatErrors(t *testing.T) {
	validate := validator.New()

	// 1. Valid struct
	validUser := TestUser{Name: "Alice", Email: "alice@example.com", Age: 30, Code: "12345"}
	err := validate.Struct(validUser)
	assert.NoError(t, err)

	// 2. Invalid struct
	invalidUser := TestUser{Name: "", Email: "not-an-email", Age: 10, Code: "12"}
	err = validate.Struct(invalidUser)
	assert.Error(t, err)

	errs := validation.FormatErrors(err)
	assert.Len(t, errs, 4)

	assert.Equal(t, "name", errs[0].Field)
	assert.Equal(t, "This field is required", errs[0].Message)

	assert.Equal(t, "email", errs[1].Field)
	assert.Equal(t, "Invalid email format", errs[1].Message)

	assert.Equal(t, "age", errs[2].Field)
	assert.Equal(t, "Must be at least 18", errs[2].Message)

	assert.Equal(t, "code", errs[3].Field)
	assert.Equal(t, "Invalid value", errs[3].Message)

	// Test max tag
	overAgeUser := TestUser{Name: "Bob", Email: "bob@example.com", Age: 150, Code: "12345"}
	err = validate.Struct(overAgeUser)
	errs = validation.FormatErrors(err)
	assert.Len(t, errs, 1)
	assert.Equal(t, "age", errs[0].Field)
	assert.Equal(t, "Must be at most 120", errs[0].Message)

	// 3. Fallback error (non-validator error)
	nonValErr := errors.New("some other error")
	fallbackErrs := validation.FormatErrors(nonValErr)
	assert.Len(t, fallbackErrs, 1)
	assert.Equal(t, "unknown", fallbackErrs[0].Field)
	assert.Equal(t, "some other error", fallbackErrs[0].Message)
}

func TestFormatErrors_Nil(t *testing.T) {
	errs := validation.FormatErrors(nil)
	assert.Nil(t, errs)
}
