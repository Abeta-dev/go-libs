// SPDX-License-Identifier: MIT

package rbac_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/rbac"
)

func ExampleEngine() {
	engine := rbac.NewEngine()
	userPermissions := []string{"orders:read", "inventory:*"}

	canReadOrders := engine.HasPermission(userPermissions, "orders:read")
	canCreateInventory := engine.HasPermission(userPermissions, "inventory:create")
	canDeleteOrders := engine.HasPermission(userPermissions, "orders:delete")

	fmt.Printf("can read orders: %v\n", canReadOrders)
	fmt.Printf("can create inventory (wildcard): %v\n", canCreateInventory)
	fmt.Printf("can delete orders: %v\n", canDeleteOrders)

	// Output:
	// can read orders: true
	// can create inventory (wildcard): true
	// can delete orders: false
}
