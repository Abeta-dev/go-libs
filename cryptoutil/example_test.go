// SPDX-License-Identifier: MIT

package cryptoutil_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/cryptoutil"
)

func ExampleHashPassword() {
	hash, err := cryptoutil.HashPassword("SuperSecret123!")
	if err != nil {
		panic(err)
	}

	valid := cryptoutil.IsCorrectPassword(hash, "SuperSecret123!")
	invalid := cryptoutil.IsCorrectPassword(hash, "WrongPassword")

	fmt.Printf("valid password matches: %v\n", valid)
	fmt.Printf("invalid password matches: %v\n", invalid)

	// Output:
	// valid password matches: true
	// invalid password matches: false
}

func ExampleGenerateTempPassword() {
	pass, err := cryptoutil.GenerateTempPassword(16)
	if err != nil {
		panic(err)
	}

	fmt.Printf("generated length: %d\n", len(pass))

	// Output:
	// generated length: 16
}
