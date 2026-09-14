# `cryptoutil` Package

The `cryptoutil` package provides audited cryptographic helpers for secure password hashing, verification, and cryptographically secure random temporary password generation.

## When to Use
- **Authentication & User Registration**: Hashing user passwords securely before persisting them into database records.
- **Credential Verification**: Validating user login credentials against stored cryptographic hashes in constant time.
- **Temporary Credential Generation**: Generating high-entropy, URL-safe temporary passwords or reset tokens for user onboarding emails.

## Why It Is Written Like That
- **Bcrypt Cost Factor 12**: Balances verification latency (~250ms on modern CPUs) against brute-force resistance, making offline dictionary attacks computationally prohibitive.
- **Cryptographic Randomness**: Utilizes `crypto/rand` (system CSPRNG) rather than `math/rand`, guaranteeing non-deterministic entropy for generated credentials.
- **Timing Attack Defense**: Leverages constant-time hash comparison to eliminate side-channel timing attacks during password authentication.

## Alternatives Evaluated

| Alternative | Pros | Cons | Why We Chose `go-libs/cryptoutil` |
|---|---|---|---|
| **SHA-256 / SHA-512** | Fast computation; built into standard library | Vulnerable to GPU/ASIC acceleration; completely insecure for password hashing without salts and stretching | Bcrypt includes salt and adaptive work factor designed specifically for passwords |
| **Argon2id** | Modern memory-hard winner of password competition | Substantial memory tuning complexity; heavier CPU/RAM footprint in shared containers | Bcrypt is mature, widely supported across DB tools, and provides optimal protection for web auth |
| **Ad-hoc Custom Hashing** | No shared library dependency | Prone to salt omission, low iterations, or timing leaks | Audited, centralized implementation with 100% test coverage |

---

## API

```go
import "github.com/umesh0492/go-libs/cryptoutil"

// Generate a cryptographically random temporary password of the given length
// Uses crypto/rand with a safe 70-character charset (excludes ambiguous 0, O, I, l).
// Recommended minimum: length=16, for onboarding emails: length=12.
pwd, err := cryptoutil.GenerateTempPassword(16)
// pwd → "k9#mP2$xR8!wA4^z"  (example)

// Hash a plaintext password with bcrypt cost=12
hash, err := cryptoutil.HashPassword("user-supplied-password")

// Compare a plaintext password to a stored bcrypt hash
// Returns nil on match, error on mismatch or invalid hash
err = cryptoutil.ComparePassword(storedHash, candidatePassword)
if err != nil {
    return domain.ErrInvalidCredentials
}
```

## Usage in Authentication Services

```go
// Onboarding — generate and hash temp password
tempPwd, _ := cryptoutil.GenerateTempPassword(16)
hash, _     := cryptoutil.HashPassword(tempPwd)
// Store hash in DB; send tempPwd in welcome email (never store plaintext)

// Login — verify
if err := cryptoutil.ComparePassword(user.PasswordHash, req.Password); err != nil {
    return ErrInvalidCredentials
}
```

## Security Notes

- Cost 12 means ~250ms on commodity hardware — adequate for login, keeps brute-force expensive.
- `GenerateTempPassword` uses `crypto/rand`, never `math/rand`.
- Never log the plaintext password or the bcrypt hash.

## 📊 Test Coverage Status
- **Coverage**: `100.0% of statements`
- **Tests**: `cryptoutil_test.go`, `export_test.go`
