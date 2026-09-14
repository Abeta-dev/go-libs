// SPDX-License-Identifier: MIT

package rbac_test

import (
	"testing"

	"github.com/umesh0492/go-libs/rbac"
)

func BenchmarkRBAC_ExactMatch(b *testing.B) {
	engine := rbac.NewEngine()
	pattern := "order:create"
	req := "order:create"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = engine.Match(pattern, req)
	}
}

func BenchmarkRBAC_WildcardMatch(b *testing.B) {
	engine := rbac.NewEngine()
	pattern := "billing:invoice:*"
	req := "billing:invoice:download"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = engine.Match(pattern, req)
	}
}

func BenchmarkRBAC_HasPermission(b *testing.B) {
	engine := rbac.NewEngine()
	userPerms := []string{
		"user:read",
		"order:list",
		"order:view",
		"product:read",
		"billing:invoice:*",
		"report:export",
	}
	req := "billing:invoice:download"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = engine.HasPermission(userPerms, req)
	}
}

func BenchmarkRBAC_HasAllPermissions(b *testing.B) {
	engine := rbac.NewEngine()
	userPerms := []string{
		"user:*",
		"order:*",
		"billing:*",
	}
	required := []string{"user:read", "order:view", "billing:invoice:read"}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = engine.HasAllPermissions(userPerms, required...)
	}
}
