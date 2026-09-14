# `env` Package

The `env` package provides zero-dependency typed environment variable retrieval with fallback defaults and fail-fast startup panic gates.

## When to Use
- **Service Initialization**: Reading ports, connection pools, timeouts, and feature flags during application startup.
- **Twelve-Factor App Configuration**: Parsing environment variables in Kubernetes pods or Docker containers.
- **Fail-Fast Validation**: Enforcing mandatory environment variables (`DATABASE_URL`, `JWT_SECRET`) with `Must*` functions.

## Why It Is Written Like That
- **Zero Boilerplate**: Avoids repetitive `os.Getenv()`, `strconv.Atoi()`, `time.ParseDuration()`, and nil check error handling.
- **Safe Fallbacks**: Returns explicit default values whenever variables are unset, empty, or unparseable.
- **Fail-Fast `Must*` Functions**: For non-negotiable configuration, `MustString` and `MustInt` panic immediately with clear error messages before listeners or background pools open.

## Available Functions

- `String(key, defaultVal string) string`
- `Int(key string, defaultVal int) int`
- `Bool(key string, defaultVal bool) bool` (accepts `1`, `t`, `true`, `TRUE`, `0`, `f`, `false`, `FALSE`)
- `Duration(key string, defaultVal time.Duration) time.Duration` (accepts standard Go duration strings like `5s`, `10m`)
- `MustString(key string) string` (panics if unset or empty)
- `MustInt(key string) int` (panics if unset or not an integer)

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/env` |
|---|---|---|---|
| **`spf13/viper`** | Extremely powerful, multi-format (YAML/JSON/etcd) | Very heavy dependency tree, reflection-heavy, slow startup | Microservices in Kubernetes rely primarily on environment variables; `env` is sub-microsecond and zero-dependency |
| **`caarlos0/env`** | Struct tag-based parsing | Uses reflection on every startup | Explicit procedural parsing is compile-time verifiable, reflection-free, and easy to trace |
| **Raw `os.Getenv`** | Built into Go | Requires 5-10 lines of parsing and fallback logic per configuration key | Concise 1-liners with type safety and default values |

## Quickstart

```go
package main

import (
    "time"
    "github.com/umesh0492/go-libs/env"
)

type Config struct {
    Port        string
    Workers     int
    Debug       bool
    ReadTimeout time.Duration
    DatabaseURL string
}

func LoadConfig() Config {
    return Config{
        Port:        env.String("PORT", ":8080"),
        Workers:     env.Int("WORKER_COUNT", 8),
        Debug:       env.Bool("DEBUG_MODE", false),
        ReadTimeout: env.Duration("READ_TIMEOUT", 5*time.Second),
        DatabaseURL: env.MustString("DATABASE_URL"), // Required! Panics if missing
    }
}
```
