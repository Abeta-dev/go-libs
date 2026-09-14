// SPDX-License-Identifier: MIT

package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEngine_Match(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name      string
		pattern   string
		requested string
		expected  bool
	}{
		// Exact Match
		{"exact match", "orders:create", "orders:create", true},
		{"exact match diff case", "Orders:Create", "orders:create", true},
		{"exact match diff case 2", "orders:create", "ORDERS:CREATE", true},
		{"exact match no match", "orders:create", "orders:view", false},

		// Single Wildcard
		{"wildcard match all", "*", "orders:create", true},
		{"wildcard match all multi-segment", "*", "domain:resource:action", true},
		{"wildcard colon match all", "*:*", "orders:create", true},

		// Resource Wildcards
		{"resource wildcard", "orders:*", "orders:create", true},
		{"resource wildcard multi-segment", "orders:*", "orders:create:approved", true},
		{"resource wildcard no match", "orders:*", "documents:create", false},

		// Action Wildcards
		{"action wildcard", "*:view", "orders:view", true},
		{"action wildcard no match", "*:view", "orders:create", false},

		// Multi-segment Wildcards
		{"multi-segment middle wildcard", "domain:*:action", "domain:resource:action", true},
		{"multi-segment middle wildcard no match", "domain:*:action", "other:resource:action", false},

		// Edge Cases
		{"empty pattern", "", "orders:create", false},
		{"empty requested", "orders:create", "", false},
		{"pattern longer than requested", "orders:create:approved", "orders:create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := e.Match(tt.pattern, tt.requested)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestEngine_HasPermission(t *testing.T) {
	e := NewEngine()
	userPerms := []string{
		"orders:create",
		"documents:*",
		"*:view",
	}

	assert.True(t, e.HasPermission(userPerms, "orders:create"))
	assert.True(t, e.HasPermission(userPerms, "documents:approve"))
	assert.True(t, e.HasPermission(userPerms, "reports:view"))
	assert.False(t, e.HasPermission(userPerms, "orders:approve"))
	assert.False(t, e.HasPermission(userPerms, "reports:create"))
}

func TestEngine_HasAnyPermission(t *testing.T) {
	e := NewEngine()
	userPerms := []string{
		"orders:create",
		"documents:*",
	}

	assert.True(t, e.HasAnyPermission(userPerms, "orders:create", "orders:approve"))
	assert.True(t, e.HasAnyPermission(userPerms, "orders:approve", "documents:create"))
	assert.False(t, e.HasAnyPermission(userPerms, "orders:approve", "reports:create"))
}

func TestEngine_HasAllPermissions(t *testing.T) {
	e := NewEngine()
	userPerms := []string{
		"orders:create",
		"documents:*",
	}

	assert.True(t, e.HasAllPermissions(userPerms, "orders:create", "documents:create"))
	assert.False(t, e.HasAllPermissions(userPerms, "orders:create", "orders:approve"))
}
