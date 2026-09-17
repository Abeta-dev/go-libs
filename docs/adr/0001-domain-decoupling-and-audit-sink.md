# ADR 0001: Strict Domain Decoupling and Framework-Agnostic Audit Sink Abstraction

- **Status**: Accepted
- **Date**: 2026-09-08
- **Deciders**: Architecture Council, Platform Engineering
- **Consulted**: Security, Microservice Core Teams

---

## Context and Problem Statement

`go-libs` serves as the foundational nervous system and shared infrastructure layer across all backend services in the enterprise ecosystem. It provides low-level, high-performance systems engineering modules including bounded worker pools, rate limiters, circuit breakers, graceful shutdown coordinators, and OpenTelemetry instrumentation.

Historically, shared repositories in distributed microservices suffer from "domain creep" — where business-specific types (e.g. `Vendor`, `Invoice`, `PurchaseOrder`), domain validation rules, or database table DDL schemas leak into foundational libraries. This antipattern causes:
1. **Coupling & Fragility**: Any schema or business model change requires releasing and bumping the shared infrastructure BOM (`go-libs`), forcing widespread rebuilds across unrelated services.
2. **Circular Dependencies**: When higher-level domain kits (`go-app-kit`) and consumer services import `go-libs`, leaky domain abstractions produce circular import graphs and package tangles.
3. **Audit & Compliance Contamination**: Regulatory compliance frameworks (SOC2, ISO 27001, Indian IT Act) mandate immutable audit logging. Storing audit schemas or business state diffs inside `go-libs` pollutes a zero-dependency transport/telemetry module with database persistence schemas.

## Decision Drivers

- **Zero Business Logic Mandate**: Maintain `go-libs` exclusively for infrastructure telemetry, network wrappers, and concurrency primitives (as mandated in `ARCHITECTURE_BOUNDARIES.md`).
- **Ports & Adapters (Hexagonal Architecture)**: Foundational libraries must define abstract interfaces (ports) without dictating concrete business implementations or persistence tables.
- **Independent Evolution**: Microservice domain teams must be able to evolve schemas, entities, and compliance diff engines without touching shared systems infrastructure.
- **Maintain High Test Coverage**: `go-libs` enforces an immutable statement coverage gate (>=90% global, 100% `ginmw`). Domain models would inflate unexercised statement branches.

## Considered Options

1. **Option 1**: Allow domain audit structs and partitioned PostgreSQL DDL schemas directly inside `go-libs/audit`.
2. **Option 2**: Enforce strict domain decoupling: keep `go-libs` purely interface-driven for logging/telemetry, and delegate concrete domain entities, state diff engines, and audit DDL schemas to `go-app-kit/audit`.
3. **Option 3**: Create a monolithic single repository containing all domain entities, microservices, and infrastructure packages.

## Decision Outcome

**Chosen Option: Option 2 (Strict Domain Decoupling with Abstract Audit Sinks).**

### Architectural Rules Enforced

1. **No Domain Entities in `go-libs`**:
   - `go-libs` shall never define application entities (`Vendor`, `User`, `Invoice`) or domain error codes.
   - All errors must use canonical infrastructure codes defined in `go-libs/apperror` (e.g., `CodeNotFound`, `CodeUnauthorized`, `CodeConflict`).

2. **Abstract Sink Interfaces (Ports)**:
   - For auditing and event dispatch, `go-libs` provides abstract sinks and context propagation tools (such as `requestid.ContextWithRequestID`, `logger.FromContext`, and pluggable metric collectors).
   - Downstream consumers or domain libraries define concrete persistence sinks (e.g., `audit.Store` or `outbox.Store` in `go-app-kit`) that accept domain events and execute database transactions.

3. **Separation of Concerns**:
   - `go-libs`: Pure infrastructure (resilience, concurrency, HTTP middleware, telemetry, zero business logic).
   - `go-app-kit`: Enterprise domain capabilities (GSTIN/PAN/Aadhaar compliance, partitioned audit tables, transactional outbox engine, PDF invoice generation).
   - Microservices: Business workflow orchestration, entity state machines, domain controllers.

## Consequences

### Positive
- **Stable Foundation**: `go-libs` releases are decoupled from business schema lifecycles.
- **Clean Dependency Graph**: Consuming microservices can freely import `go-libs` alongside `go-app-kit` without diamond dependency conflicts or circular references.
- **Audit Flexibility**: Audit logging backends (PostgreSQL partitioned tables, Kafka event streams, S3 archive sinks) can be swapped or modified in `go-app-kit` without recompiling `go-libs`.
- **Enforced Security Boundaries**: RBAC permissions are evaluated server-side via runtime context hooks (`PermissionProvider`), avoiding baked-in domain assumptions in JWT tokens.

### Negative / Trade-offs
- Consuming services must construct adapter bridges to connect `go-libs` context utilities (`requestid`, `logger`, `telemetry`) to their concrete domain audit recorders.
- Code duplication between foundational workerpool primitives and standalone application modules requires careful maintenance (remediated via strict contract tests).
