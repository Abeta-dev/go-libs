// SPDX-License-Identifier: MIT

package rbaccontext_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/umesh0492/go-libs/rbaccontext"
)

func TestCan_FalseWhenNoPermissionsLoaded(t *testing.T) {
	assert.False(t, rbaccontext.Can(context.Background(), "users:create"))
}

func TestCan_TrueForLoadedPermission(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:create", "invoices:read"})
	assert.True(t, rbaccontext.Can(ctx, "users:create"))
	assert.True(t, rbaccontext.Can(ctx, "invoices:read"))
}

func TestCan_FalseForMissingPermission(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:create"})
	assert.False(t, rbaccontext.Can(ctx, "invoices:approve"))
}

func TestCanAny_TrueWhenOneMatches(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:read"})
	assert.True(t, rbaccontext.CanAny(ctx, "invoices:read", "users:read"))
}

func TestCanAny_FalseWhenNoneMatch(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:read"})
	assert.False(t, rbaccontext.CanAny(ctx, "invoices:read", "approvals:approve"))
}

func TestCanAll_TrueWhenAllMatch(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:read", "invoices:read"})
	assert.True(t, rbaccontext.CanAll(ctx, "users:read", "invoices:read"))
}

func TestCanAll_FalseWhenOneMissing(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:read"})
	assert.False(t, rbaccontext.CanAll(ctx, "users:read", "invoices:approve"))
}

func TestWithPermissions_EmptySlice_AllDenied(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{})
	assert.False(t, rbaccontext.Can(ctx, "users:create"))
}

func TestWithPermissions_DoesNotPolluteSiblingContexts(t *testing.T) {
	parent := context.Background()
	ctx1 := rbaccontext.WithPermissions(parent, []string{"users:create"})
	ctx2 := rbaccontext.WithPermissions(parent, []string{"invoices:read"})
	assert.True(t, rbaccontext.Can(ctx1, "users:create"))
	assert.False(t, rbaccontext.Can(ctx1, "invoices:read"))
	assert.True(t, rbaccontext.Can(ctx2, "invoices:read"))
	assert.False(t, rbaccontext.Can(ctx2, "users:create"))
}

func TestPermissions_NilWhenNotSet(t *testing.T) {
	assert.Nil(t, rbaccontext.Permissions(context.Background()))
}

func TestPermissions_ReturnsSetPermissions(t *testing.T) {
	ctx := rbaccontext.WithPermissions(context.Background(), []string{"users:create", "invoices:read"})
	perms := rbaccontext.Permissions(ctx)
	assert.NotNil(t, perms)
	assert.ElementsMatch(t, []string{"users:create", "invoices:read"}, perms)
}

func TestCanAny_FalseWhenNotSet(t *testing.T) {
	assert.False(t, rbaccontext.CanAny(context.Background(), "users:create"))
}

func TestCanAll_FalseWhenNotSet(t *testing.T) {
	assert.False(t, rbaccontext.CanAll(context.Background(), "users:create"))
}
