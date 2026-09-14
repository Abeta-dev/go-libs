# `timeutil` Package

The `timeutil` package provides domain-agnostic time helpers for location normalization, day boundaries, and business day calculations.

## When to Use
- **Day Boundary Normalization**: Calculating start of day and end of day (`StartOfDay`, `EndOfDay`) in arbitrary timezones.
- **Business Day Calculations**: Computing delivery or SLA dates (`AddBusinessDays`) skipping weekends.
- **Timezone Normalization**: Formatting and parsing timestamps cleanly across timezones.

## Why It Is Written Like That
- **Resilient Timezone Execution**: Relies purely on standard `time.Location` pointers without hardcoding any regional timezones.
- **Zero Heap Allocations**: Day boundaries and business day arithmetic are computed via direct timestamp calculations.

---

## API

```go
import "github.com/umesh0492/go-libs/timeutil"

// Current time in a given location
loc, _ := time.LoadLocation("UTC")
now := timeutil.NowIn(loc)

// Format a time in a given location
formatted := timeutil.FormatIn(time.Now(), loc, "02 Jan 2006")

// Day boundaries
sod := timeutil.StartOfDay(time.Now(), loc) // 00:00:00.000
eod := timeutil.EndOfDay(time.Now(), loc)   // 23:59:59.999999999

// ── Business Days ─────────────────────────────────────────────────────────────

// Add 5 business days (skips Saturdays and Sundays; no public holiday awareness)
deadline := timeutil.AddBusinessDays(order.IssuedAt, 5)
```

## Domain Localization

Regional localization logic (such as Indian Standard Time or financial year bounds) belongs in domain libraries (e.g. `go-app-kit/india`).

## What it does NOT do

- **Public holidays** — `AddBusinessDays` only skips weekends. Calendar-specific holidays belong in domain modules.
- **Financial years or aging reports** — Domain-specific fiscal logic belongs in application services.

## 📊 Test Coverage Status
- **Coverage**: `100.0% of statements`
- **Tests**: `timeutil_test.go`, `example_test.go`
