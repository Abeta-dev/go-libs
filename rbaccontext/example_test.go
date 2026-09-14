// SPDX-License-Identifier: MIT

package rbaccontext_test

import (
	"context"
	"fmt"

	"github.com/umesh0492/go-libs/rbaccontext"
)

func ExampleCan() {
	ctx := context.Background()
	ctx = rbaccontext.WithPermissions(ctx, []string{"orders:read", "invoices:*"})

	fmt.Println(rbaccontext.Can(ctx, "orders:read"))
	fmt.Println(rbaccontext.Can(ctx, "orders:write"))
	fmt.Println(rbaccontext.Can(ctx, "invoices:create"))
	// Output:
	// true
	// false
	// true
}
