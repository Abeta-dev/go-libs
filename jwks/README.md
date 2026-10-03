# jwks

Package `jwks` provides an in-memory cached JSON Web Key Set (JWKS) resolver for cryptographic public keys, featuring automatic TTL refresh and resilient stale fallback during network interruptions.

## Usage

```go
import "github.com/umesh0492/go-libs/jwks"

// Retrieve ECDSA P-256 public key by KID
pubKey, err := jwks.GetECPublicKey("my-kid", "https://auth.example.com/.well-known/jwks.json")
```
