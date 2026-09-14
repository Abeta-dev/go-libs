# `rbac` Package

The `rbac` package provides high-performance, in-memory Role-Based Access Control (RBAC) evaluation with wildcard permissions and default-deny semantics.

## When to Use
- **Route Authorization**: Enforcing permission checks on HTTP routes (`orders:read`, `users:write`).
- **Domain Authorization**: Checking if an actor possesses specific permissions before executing service actions.
- **Dynamic Role Management**: Storing and evaluating role-to-permission matrices in memory.

## Why It Is Written Like That
- **Sub-Microsecond Evaluation**: Evaluates permissions via fast O(1) exact map lookup with a single wildcard fallback (`*` or `domain:*`).
- **Default-Deny Security**: If a role or permission is not explicitly defined, access is denied immediately.
- **Wildcard Hierarchy**: A permission like `orders:*` automatically authorizes `orders:create`, `orders:read`, `orders:update`, and `orders:delete`.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/rbac` |
|---|---|---|---|
| **Casbin** | Comprehensive model support (ABAC, RBAC, ACL) | Heavyweight runtime, complicated model DSL, memory overhead | Fast, pure Go, zero-dependency engine perfectly tailored for microservice authorization |
| **OPA (Open Policy Agent)** | Declarative Rego language | Requires external sidecar or embedded WebAssembly interpreter | In-process sub-microsecond evaluation without infrastructure sidecars |
| **Hardcoded `role == "admin"`** | Simplest to code | Brittle, tightly couples role strings to handlers, cannot grant granular permissions | Granular permissions decoupled from high-level roles |

## Quickstart

```go
package main

import (
    "fmt"
    "github.com/umesh0492/go-libs/rbac"
)

func main() {
    engine := rbac.NewEngine()

    // Configure role permissions
    engine.RegisterRole("viewer", []string{"orders:read", "invoices:read"})
    engine.RegisterRole("manager", []string{"orders:*", "invoices:*"})

    // Evaluate
    canView := engine.HasPermission("viewer", "orders:read")   // true
    canCreate := engine.HasPermission("viewer", "orders:create") // false
    managerCreate := engine.HasPermission("manager", "orders:create") // true (wildcard match)

    fmt.Println("viewer can view:", canView)
    fmt.Println("viewer can create:", canCreate)
    fmt.Println("manager can create:", managerCreate)
}
```
