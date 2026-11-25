#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Leaf Package Import Isolation & Minimal Footprint Gate
#
# Guarantees that:
#   1. Gate 1 (Depth-0 Pure Internal Leaf DAG Isolation): All 29 pure leaf
#      packages have zero internal github.com/abeta-dev/go-libs/* production
#      imports, preventing circular or accidental intra-repo coupling.
#   2. Gate 2 (Minimal-Footprint Isolated go.mod Sandbox Gate): Importing any
#      of the 35 lightweight minimal-footprint packages in isolation does NOT
#      drag heavy third-party dependencies (gin, pgx, opentelemetry) into a
#      consumer's go.sum or compiled binary.
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

PURE_LEAF_PACKAGES=(
  "apperror"
  "async"
  "bodylimit"
  "clock"
  "compress"
  "env"
  "etag"
  "fsm"
  "health"
  "httputil"
  "jwks"
  "loopbackguard"
  "maputil"
  "metrics"
  "pagination"
  "rbac"
  "recovery"
  "requestid"
  "retry"
  "securityheaders"
  "shutdown"
  "sliceutil"
  "sqlite"
  "sse"
  "stringutil"
  "telemetry"
  "timeutil"
  "uuidutil"
  "validation"
)

LEAF_PACKAGES=(
  "apperror"
  "async"
  "bodylimit"
  "cache"
  "circuitbreaker"
  "clock"
  "compress"
  "cryptoutil"
  "env"
  "etag"
  "fsm"
  "health"
  "httpmw"
  "httputil"
  "jwks"
  "logger"
  "loopbackguard"
  "maputil"
  "metrics"
  "pagination"
  "ratelimit"
  "rbac"
  "rbaccontext"
  "requestid"
  "retry"
  "securityheaders"
  "shutdown"
  "sliceutil"
  "sqlite"
  "sse"
  "stringutil"
  "timeutil"
  "uuidutil"
  "validation"
  "workerpool"
)

FORBIDDEN_PATTERN='github.com/gin-gonic/gin|github.com/jackc/pgx|go.opentelemetry.io/otel\b'

echo "========================================================"
echo "🌿 Gate 1: Verifying Depth-0 Pure Internal Leaf DAG Isolation"
echo "   Checking ${#PURE_LEAF_PACKAGES[@]} pure leaf packages for 0 internal go-libs imports"
echo "========================================================"

PURE_FAILED_COUNT=0

for pkg in "${PURE_LEAF_PACKAGES[@]}"; do
  if ! IMPORTS_OUT=$(go list -f '{{join .Imports "\n"}}' "./${pkg}" 2>&1); then
    echo "❌ Error: 'go list' failed for '${pkg}': ${IMPORTS_OUT}"
    PURE_FAILED_COUNT=$((PURE_FAILED_COUNT + 1))
    continue
  fi

  INTERNAL_IMPORTS=$(printf '%s\n' "${IMPORTS_OUT}" | grep -E '^github\.com/abeta-dev/go-libs/' | tr '\n' ' ' || true)
  if [ -n "${INTERNAL_IMPORTS}" ]; then
    echo "❌ Error: Depth-0 pure leaf '${pkg}' has forbidden internal go-libs imports: ${INTERNAL_IMPORTS}"
    PURE_FAILED_COUNT=$((PURE_FAILED_COUNT + 1))
  else
    echo "   - ${pkg}: pure Depth-0 internal leaf ✅ (0 internal go-libs imports)"
  fi
done

if [ "${PURE_FAILED_COUNT}" -ne 0 ]; then
  echo "========================================================"
  echo "❌ Error: ${PURE_FAILED_COUNT} pure leaf package(s) violated Depth-0 internal DAG isolation!"
  exit 1
fi
echo "✅ All ${#PURE_LEAF_PACKAGES[@]} Depth-0 pure leaf packages have 0 internal go-libs imports!"

echo "========================================================"
echo "🍃 Gate 2: Verifying Minimal-Footprint Leaf Import Isolation"
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
	_ "github.com/abeta-dev/go-libs/${pkg}"
)

func main() {}
EOF
    go mod edit -replace "github.com/abeta-dev/go-libs=${ROOT_DIR}" >/dev/null 2>&1
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
