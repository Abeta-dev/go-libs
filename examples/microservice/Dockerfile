# ==============================================================================
# Build Stage: Multi-stage, statically compiled, minimal attack surface
# ==============================================================================
FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /src

# Leverage Docker layer caching for dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source tree
COPY . .

# Compile static binary:
# - CGO_ENABLED=0 produces a pure Go static binary without libc runtime dependencies
# - -ldflags="-s -w" strips debug symbols and symbol table for minimal binary size
# - -trimpath removes absolute local file paths from stack traces
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o /bin/microservice \
    ./examples/microservice

# ==============================================================================
# Runtime Stage: Distroless static image (<18MB, 0 CVEs, Non-root UID 65532)
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

# Copy CA certs and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy compiled binary
COPY --from=builder --chown=nonroot:nonroot /bin/microservice /microservice

# Run as non-privileged user (UID 65532: nonroot)
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/microservice"]
