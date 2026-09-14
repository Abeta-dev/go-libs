// SPDX-License-Identifier: MIT

package env_test

import (
	"fmt"
	"time"

	"github.com/umesh0492/go-libs/env"
)

func ExampleString() {
	port := env.String("SERVER_PORT", "8080")
	fmt.Println("Port:", port)
	// Output:
	// Port: 8080
}

func ExampleDuration() {
	timeout := env.Duration("HTTP_TIMEOUT", 30*time.Second)
	fmt.Println("Timeout:", timeout)
	// Output:
	// Timeout: 30s
}
