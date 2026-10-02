# uuidutil

Package `uuidutil` provides domain-agnostic UUID generation, parsing, pointer transformation, and validation helpers wrapping `github.com/google/uuid`.

## Usage

```go
import "github.com/umesh0492/go-libs/uuidutil"

id := uuidutil.New()
str := uuidutil.NewString()

ptr, err := uuidutil.ParsePtr(optionalIDString)
if uuidutil.IsValid(str) {
    // Valid UUID
}
```
