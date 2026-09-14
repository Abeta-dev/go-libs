# `db` Package

The `db` package manages PostgreSQL connection pooling via `pgxpool`, standardizing connection limits, health checks, and lifecycle parameters across microservices.

## When to Use
- **PostgreSQL Database Access**: Establishing managed connection pools for transactional services using `pgxpool`.
- **Connection Resource Governance**: Enforcing uniform `MaxConns`, `MinConns`, `MaxConnLifetime`, and `MaxConnIdleTime` constraints across containerized pods.
- **Health Checks & Liveness Probes**: Supplying ping and pool status metrics to orchestration systems and load balancers.

## Why It Is Written Like That
- **Direct `pgxpool` Driver**: Uses the native PostgreSQL binary protocol (`github.com/jackc/pgx/v5/pgxpool`) rather than generic `database/sql`, yielding higher performance, native array parsing, and structured logging.
- **Functional Options Pattern**: Configured using composable options (`db.WithMaxConns`, `db.WithMinConns`, `db.WithMaxConnLifetime`), making settings explicit, safe, and easily testable.
- **Warm Pool Warm-up**: Enforces `MinConns` to maintain warm background sockets, preventing TCP handshake latency spikes on traffic bursts.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/db` |
|---|---|---|---|
| **Standard `database/sql` + `lib/pq`** | Built-in Go interface | `lib/pq` is officially in maintenance mode; lacks binary protocol optimizations and native pgvector/copy features | Modern `pgx/v5` driver with binary protocol performance and robust connection pool governance |
| **GORM DB Connect directly** | Full ORM feature set | Hides pool settings under ORM wrappers; difficult to fine-tune socket timeouts and lifecycle hooks | Exposes explicit pool configuration while retaining compatibility with raw SQL and query builders |
| **Ad-hoc Per-Service Connections** | Total freedom per service | Leads to TCP socket exhaustion, unconfigured idle leaks, and accidental database cluster overloading | Centralized pool parameters guarantee database reliability under Kubernetes autoscaling |

---

## How to Use It

```go
import (
    "context"
    "os"
    "time"
    "github.com/umesh0492/go-libs/db"
)

// Initialize with production pool limits matching container resources:
pool, err := db.Connect(
    context.Background(),
    os.Getenv("DATABASE_URL"),
    db.WithMaxConns(50),
    db.WithMinConns(10), // Prevents cold-start latency spikes
    db.WithMaxConnLifetime(1*time.Hour),
    db.WithMaxConnIdleTime(30*time.Minute),
)
if err != nil {
    log.Fatalf("failed to connect to postgres: %v", err)
}
defer pool.Close()
```

---

## 🛡️ Edge Cases Handled
- **Cold Boot Thrashing**: Integrating `db.WithMinConns` allows instances to establish socket connections prior to traffic ingress, eliminating first-request latency spikes.
- **Stale TCP Connection Recycling**: `MaxConnLifetime` and `MaxConnIdleTime` periodically cycle old connections to prevent silent TCP drops across AWS/GCP NAT gateways.

## 📊 Test Coverage Status
- **Coverage**: `100.0% of statements`
- **Tests**: `db_test.go`, `export_test.go`
