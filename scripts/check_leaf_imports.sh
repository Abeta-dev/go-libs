#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Leaf Package Import Isolation & Minimal Footprint Gate
#
# Guarantees that:
#   Importing any lightweight "leaf" package in isolation does NOT drag heavy
#   third-party dependencies (gin, pgx, opentelemetry) into a consumer's
#   go.sum or compiled binary.
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

LEAF_PACKAGES=(
  "shutdown"
  "retry"
  "sliceutil"
  "env"
  "apperror"
  "cache"
  "circuitbreaker"
  "ratelimit"
  "workerpool"
  "cryptoutil"
  "stringutil"
  "timeutil"
  "maputil"
  "clock"
)

FORBIDDEN_PATTERN='github.com/gin-gonic/gin|github.com/jackc/pgx|go.opentelemetry.io/otel\b'

echo "========================================================"
echo "🍃 Verifying Minimal-Footprint Leaf Import Isolation"
echo "   Checking ${#LEAF_PACKAGES[@]} leaf packages against heavy dependencies:"
echo "   - gin (github.com/gin-gonic/gin)"
echo "   - pgx (github.com/jackc/pgx)"
echo "   - otel (go.opentelemetry.io/otel)"
echo "========================================================"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

FAILED_COUNT=0

for pkg in "${LEAF_PACKAGES[@]}"; do
  PKG_DIR="${TMP_DIR}/${pkg}"
  mkdir -p "${PKG_DIR}"
  (
    cd "${PKG_DIR}"
    go mod init "verify_leaf_${pkg}" >/dev/null 2>&1
    cat << EOF > main.go
package main

import (
	_ "github.com/umesh0492/go-libs/${pkg}"
)

func main() {}
EOF
    go mod edit -replace "github.com/umesh0492/go-libs=${ROOT_DIR}" >/dev/null 2>&1
    if ! TIDY_ERR=$(go mod tidy 2>&1); then
      echo "❌ Error: 'go mod tidy' failed for '${pkg}': ${TIDY_ERR}"
      exit 1
    fi

    if [ -f go.sum ] && grep -E "${FORBIDDEN_PATTERN}" go.sum >/dev/null 2>&1; then
      LEAKED=$(grep -E "${FORBIDDEN_PATTERN}" go.sum | awk '{print $1}' | sort -u | tr '\n' ' ')
      echo "❌ Error: '${pkg}' leaked heavy dependencies into go.sum: ${LEAKED}"
      exit 1
    else
      echo "   - ${pkg}: clean leaf import ✅ (zero heavy dependencies in go.sum)"
    fi
  ) || FAILED_COUNT=$((FAILED_COUNT + 1))
done

echo "========================================================"
if [ "${FAILED_COUNT}" -ne 0 ]; then
  echo "❌ Error: ${FAILED_COUNT} leaf package(s) failed dependency isolation!"
  exit 1
fi

echo "✅ All ${#LEAF_PACKAGES[@]} leaf packages resolve independently with minimal footprint!"
echo "========================================================"
