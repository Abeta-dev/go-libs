# `health` Package

The `health` package provides standardized liveness and readiness probe handlers designed for Kubernetes and cloud load balancers.

## When to Use
- **Kubernetes Liveness Probes (`/healthz` or `/liveness`)**: Verifying that the Go runtime and HTTP server process are active and not deadlocked.
- **Kubernetes Readiness Probes (`/ready` or `/readiness`)**: Verifying that essential backing dependencies (such as PostgreSQL database pools) are reachable before routing client traffic to the pod.
- **Traffic Draining**: Failing readiness checks during graceful shutdown so load balancers stop forwarding new requests before container termination.

## Why It Is Written Like That
- **Separation of Liveness & Readiness**: Liveness never checks external dependencies (preventing cascading container restart storms when a database experiences transient latency). Readiness actively probes critical sockets using short, bounded timeouts.
- **Pluggable Pinger Interface**: Accepts any dependency implementing `Ping(ctx context.Context) error` (such as `*pgxpool.Pool` or `*sql.DB`), keeping the health package fully decoupled from specific drivers.
- **Bounded Diagnostic Timeouts**: Probes execute with strict timeout bounds (e.g. 2s) so health checks never hang or block container orchestration workers.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/health` |
|---|---|---|---|
| **Ad-hoc Custom `/ping` Handlers** | Simple one-liner in main.go | Omits database readiness; ignores dependency failures; inconsistent response formats across services | Standardized JSON payload with status codes, timestamp, and dependency breakdown |
| **Heavy Health Check Libraries** | Many third-party integrations | Heavy dependency trees; complicated DSLs; high memory overhead | Lightweight, zero-dependency implementation using Go standard library |
| **Checking DB in Liveness Probe** | Detects broken DB | When the database hiccups, all microservice pods get killed simultaneously by K8s (catastrophic restart storm) | Strict architectural split between liveness (process alive) and readiness (ready for traffic) |

---

## Quickstart

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/umesh0492/go-libs/health"
)

r := gin.New()

// 1. Liveness: returns 200 OK immediately if the HTTP process is responsive
r.GET("/liveness", health.LivenessProbe)

// 2. Readiness: checks DB ping before signaling ready to receive traffic
r.GET("/readiness", health.ReadinessProbe(dbPool))
```

---

## 🛡️ Edge Cases Handled
- **DB Connection Zombie State**: If the database pool drops or hangs, `/readiness` enforces an explicit context deadline on `.Ping(ctx)`. If the ping fails or exceeds the deadline, it returns HTTP 503, signaling Kubernetes to stop routing traffic until the connection recovers.
- **Cascading Restart Storm Prevention**: Liveness checks never touch external databases or networks, ensuring Kubernetes does not restart healthy containers during downstream outages.
