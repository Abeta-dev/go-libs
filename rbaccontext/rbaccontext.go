// SPDX-License-Identifier: MIT

// Package rbaccontext attaches and verifies RBAC permissions within standard Go context.Context.
package rbaccontext

import (
	"context"

	"github.com/umesh0492/go-libs/rbac"
)

type contextKey struct{}

// Singleton rbac engine for wildcard evaluation
var engine = rbac.NewEngine()

// permSet is the internal O(1) permission store.
type permSet struct {
	m map[string]struct{}
}

// WithPermissions returns a new context carrying an O(1) permission map.
// Called once per request by the RBAC middleware.
func WithPermissions(ctx context.Context, perms []string) context.Context {
	ps := &permSet{m: make(map[string]struct{}, len(perms))}
	for _, p := range perms {
		ps.m[p] = struct{}{}
	}
	return context.WithValue(ctx, contextKey{}, ps)
}

// Permissions retrieves the full permission slice from ctx.
// Returns nil if the RBAC middleware has not run (unauthenticated routes).
// Order of returned slice is non-deterministic — use for logging/debug only.
func Permissions(ctx context.Context) []string {
	ps, ok := ctx.Value(contextKey{}).(*permSet)
	if !ok || ps == nil {
		return nil
	}
	out := make([]string, 0, len(ps.m))
	for p := range ps.m {
		out = append(out, p)
	}
	return out
}

// Can returns true if the context contains the given "resource:action" permission, supporting wildcards.
// Fail-safe: returns false when no permissions are loaded (unauthenticated routes).
func Can(ctx context.Context, permission string) bool {
	ps, ok := ctx.Value(contextKey{}).(*permSet)
	if !ok || ps == nil {
		return false
	}
	// Fast-path exact match
	if _, found := ps.m[permission]; found {
		return true
	}
	// Wildcard matching fallback
	return engine.HasPermission(Permissions(ctx), permission)
}

// CanAny returns true if the context contains at least one of the given permissions, supporting wildcards.
func CanAny(ctx context.Context, permissions ...string) bool {
	ps, ok := ctx.Value(contextKey{}).(*permSet)
	if !ok || ps == nil {
		return false
	}
	return engine.HasAnyPermission(Permissions(ctx), permissions...)
}

// CanAll returns true only if the context contains every given permission, supporting wildcards.
func CanAll(ctx context.Context, permissions ...string) bool {
	ps, ok := ctx.Value(contextKey{}).(*permSet)
	if !ok || ps == nil {
		return false
	}
	return engine.HasAllPermissions(Permissions(ctx), permissions...)
}
