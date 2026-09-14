// SPDX-License-Identifier: MIT

package apperror_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/apperror"
)

func ExampleNotFound() {
	err := apperror.NotFound("user not found")
	fmt.Println(err.Error())
	fmt.Println(err.GetStatus())
	fmt.Println(apperror.Is(err, apperror.CodeNotFound))
	// Output:
	// NOT_FOUND: user not found
	// 404
	// true
}

func ExampleIs() {
	err := apperror.Conflict("duplicate email")
	fmt.Println(apperror.Is(err, apperror.CodeConflict))
	fmt.Println(apperror.Is(err, apperror.CodeNotFound))
	// Output:
	// true
	// false
}
