# uuidutil

Package `uuidutil` provides domain-agnostic UUID generation, parsing, pointer transformation, and validation helpers wrapping `github.com/google/uuid`.

## Usage

```go
import "github.com/Abeta-dev/go-libs/uuidutil"

id := uuidutil.New()
str := uuidutil.NewString()
v7 := uuidutil.NewV7String() // Time-ordered UUIDv7 for B-Tree primary keys

ptr, err := uuidutil.ParsePtr(optionalIDString)
if uuidutil.IsValid(str) {
    // Valid UUID
}
```
