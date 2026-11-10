# uuidutil

Package `uuidutil` provides domain-agnostic UUID generation, parsing, pointer transformation, and validation helpers wrapping `github.com/google/uuid`.

## Usage

```go
import "github.com/abeta-dev/go-libs/uuidutil"

id := uuidutil.New()
str := uuidutil.NewString()
v7, err := uuidutil.NewV7()  // RFC 9562 time-ordered UUIDv7 (uuid.UUID)
v7Str := uuidutil.NewV7String() // Time-ordered UUIDv7 string for B-Tree primary keys

ptr, err := uuidutil.ParsePtr(optionalIDString)
if uuidutil.IsValid(str) {
    // Valid UUID
}
```
