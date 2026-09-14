// SPDX-License-Identifier: MIT

package stringutil_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/stringutil"
)

func ExampleOrDefault() {
	fmt.Println(stringutil.OrDefault("", "default_value"))
	fmt.Println(stringutil.OrDefault("custom_value", "default_value"))
	// Output:
	// default_value
	// custom_value
}

func ExampleMaskEmail() {
	fmt.Println(stringutil.MaskEmail("alice.smith@example.com"))
	// Output:
	// al*********@example.com
}
