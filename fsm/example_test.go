// SPDX-License-Identifier: MIT

package fsm_test

import (
	"fmt"

	"github.com/abeta-dev/go-libs/fsm"
)

func ExampleNewMachine() {
	machine := fsm.NewMachine([]fsm.Transition{
		{From: "PENDING", To: "PAID", Action: "pay"},
		{From: "PAID", To: "SHIPPED", Action: "ship"},
	})

	fmt.Println("PENDING -> PAID via pay:", machine.CanTransition("PENDING", "PAID", "pay"))
	fmt.Println("PENDING -> SHIPPED via ship:", machine.CanTransition("PENDING", "SHIPPED", "ship"))
	// Output:
	// PENDING -> PAID via pay: true
	// PENDING -> SHIPPED via ship: false
}
