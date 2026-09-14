// SPDX-License-Identifier: MIT

package validation_test

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/umesh0492/go-libs/validation"
)

type UserRequest struct {
	Email string `validate:"required,email"`
	Age   int    `validate:"min=18"`
}

func ExampleFormatErrors() {
	validate := validator.New()
	req := UserRequest{
		Email: "invalid-email",
		Age:   16,
	}

	err := validate.Struct(req)
	fieldErrors := validation.FormatErrors(err)

	for _, fe := range fieldErrors {
		fmt.Printf("field: %s -> %s\n", fe.Field, fe.Message)
	}

	// Output:
	// field: email -> Invalid email format
	// field: age -> Must be at least 18
}
