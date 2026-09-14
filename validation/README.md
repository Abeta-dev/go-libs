# `validation` Package

The `validation` package provides declarative struct validation error formatting powered by `github.com/go-playground/validator/v10`.

## When to Use
- **Inbound HTTP Request Payloads**: Validating JSON/form request bodies in Gin handlers (`c.ShouldBindJSON`).
- **Standardized Error Formatting**: Translating raw Go validator error strings into clean, user-friendly JSON payloads for frontend display.

## Why It Is Written Like That
- **The Raw Validator Problem**: Go's `validator` package returns verbose, internal error structures like `Key: 'CreateUserRequest.Email' Error:Field validation for 'Email' failed on the 'email' tag`. Sending this to mobile/web clients results in poor UX and exposes internal struct naming.
- **Frontend-Friendly Schema**: `validation.FormatErrors(err)` maps validator failures to an array of `{field, message}` objects with lowercase field names and human-readable explanations (e.g. `"This field is required"`, `"Invalid email format"`, `"Must be at least 18"`).
- **Graceful Fallback**: If an error passed to `FormatErrors` is not a `validator.ValidationErrors` type, it gracefully wraps the raw error under `{"field": "unknown", "message": err.Error()}` instead of panicking.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/validation` |
|---|---|---|---|
| **Manual `if req.Email == "" { ... }` checks** | Zero dependencies | Massive repetitive boilerplate, inconsistent messages, prone to missing fields | Declarative struct tags (`validate:"required,email"`) keep models clean |
| **Raw Gin Binding Errors** | Built-in | Emits ugly, unformatted strings to frontend clients | `FormatErrors` produces uniform, predictable error envelopes |
| **`asaskevich/govalidator`** | Lightweight | Lacks advanced tag rules, unmaintained compared to `v10` | `go-playground/validator/v10` is the industry standard Go validation engine |

## Quickstart

### 1. Define Request Struct with Validation Tags
```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18,max=120"`
}
```

### 2. Validate and Format in Gin Handler
```go
import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/go-playground/validator/v10"
    "github.com/umesh0492/go-libs/validation"
)

var validate = validator.New()

func CreateUserHandler(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Malformed JSON payload"})
        return
    }

    if err := validate.Struct(req); err != nil {
        errs := validation.FormatErrors(err)
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "Validation Failed",
            "code":    "VALIDATION_ERROR",
            "details": errs,
        })
        return
    }

    // Process valid user...
}
```

### 3. Example JSON Error Output
```json
{
  "error": "Validation Failed",
  "code": "VALIDATION_ERROR",
  "details": [
    {
      "field": "email",
      "message": "Invalid email format"
    },
    {
      "field": "age",
      "message": "Must be at least 18"
    }
  ]
}
```
