#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Version, Symbol & Documentation Synchronization Gate
# Guarantees that:
#   1. README install examples use the verified published module version.
#   2. The current source checkout does not pretend to be a published tag.
#   3. Dynamic coverage badge and README claims match actual toolchain measurements.
#   4. README package count matches actual repository package count.
#   5. Code formatting is clean across all files (gofmt).
#   6. All current toolchain claims match the go.mod baseline.
#   7. Every package and exported symbol in the unreleased CHANGELOG section exists.
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

PUBLISHED_VER="0.2.1"

README_GET_VER=$(grep -E 'go get github.com/umesh0492/go-libs@v' README.md | head -n1 | sed -E 's/.*go-libs@v([0-9]+\.[0-9]+\.[0-9]+).*/\1/')
CURRENT_TAG="${GIT_TAG:-$(git describe --tags --exact-match 2>/dev/null || true)}"

printf '%s\n' '========================================================'
printf '%s\n' '🔒 Verifying Published Module and Checkout Identity'
printf '   - Published module version: v%s\n' "${PUBLISHED_VER}"
printf '   - README.md go get:        v%s\n' "${README_GET_VER:-N/A}"
printf '   - Current source tag:      %s\n' "${CURRENT_TAG:-untagged}"
printf '%s\n' '========================================================'

if [ "${README_GET_VER}" != "${PUBLISHED_VER}" ]; then
  echo "❌ Error: README.md go get version (v${README_GET_VER}) does not match the verified published module version (v${PUBLISHED_VER})"
  exit 1
fi

if [ -n "${CURRENT_TAG}" ]; then
  echo "❌ Error: source checkout is tagged (${CURRENT_TAG}); release verification must run from an untagged reconciliation commit before creating a new version"
  exit 1
fi

echo "✅ Published module reference and untagged checkout identity are correct."

echo "========================================================"
echo "📦 Verifying Package Inventory & Count Synchronization"
echo "========================================================"

EXPECTED_PKG_COUNT=31
MEASURED_PKGS=$(go list ./... | grep -v loadgen | wc -l | tr -d ' ')
README_PKG_COUNT=$(grep -oE '[0-9]+ packages' README.md | awk '{print $1}' | head -n1 || true)
TABLE_PKG_COUNT=$(sed -n '/| Package | Purpose | Statement Coverage |/,/| \*\*Total Statement Coverage\*\*/p' README.md | grep -E '^\| `[a-zA-Z0-9_/]+` \|' | wc -l | tr -d ' ')

echo "   - Expected Packages:            $EXPECTED_PKG_COUNT"
echo "   - Measured Packages (go list):  $MEASURED_PKGS"
echo "   - README.md packages claimed:   ${README_PKG_COUNT:-N/A}"
echo "   - README.md coverage rows:      $TABLE_PKG_COUNT"

if [ "$MEASURED_PKGS" -ne "$EXPECTED_PKG_COUNT" ]; then
  echo "❌ Error: Measured packages ($MEASURED_PKGS) does not match expected package count ($EXPECTED_PKG_COUNT)"
  exit 1
fi

if [ -n "$README_PKG_COUNT" ] && [ "$README_PKG_COUNT" != "$EXPECTED_PKG_COUNT" ]; then
  echo "❌ Error: README.md package count claim ($README_PKG_COUNT packages) does not match expected package count ($EXPECTED_PKG_COUNT)"
  exit 1
fi

if [ "$TABLE_PKG_COUNT" != "$EXPECTED_PKG_COUNT" ]; then
  echo "❌ Error: README.md coverage table row count ($TABLE_PKG_COUNT) does not match expected package count ($EXPECTED_PKG_COUNT)"
  exit 1
fi
echo "✅ Package counts are strictly synchronized ($EXPECTED_PKG_COUNT packages)."

MEASURED_COV=""
if [ -f coverage.out ]; then
  MEASURED_COV=$(go tool cover -func=coverage.out 2>/dev/null | grep total | awk '{print $3}' || true)
fi

BADGE_COV=""
if [ -f .github/badges/coverage.json ]; then
  BADGE_COV=$(grep -oE '[0-9]+\.[0-9]+%' .github/badges/coverage.json | head -n1 || true)
fi

README_CLAIM=$(grep -oE 'Overall Repository Statement Coverage: [0-9]+\.[0-9]+%' README.md | grep -oE '[0-9]+\.[0-9]+%' | head -n1 || true)
README_TABLE_TOTAL=$(grep -E '^\| \*\*Total Statement Coverage\*\* \|' README.md | grep -oE '[0-9]+\.[0-9]+%' | head -n1 || true)

DYNAMIC_BADGE_URL="https://raw.githubusercontent.com/umesh0492/go-libs/main/.github/badges/coverage.json"
HAS_DYNAMIC_BADGE=$(grep -F "${DYNAMIC_BADGE_URL}" README.md || true)

echo "========================================================"
echo "🛡️ Verifying Dynamic Coverage Synchronization"
if [ -n "$MEASURED_COV" ]; then
  echo "   - coverage.out:        $MEASURED_COV"
fi
echo "   - coverage.json:       $BADGE_COV"
echo "   - README.md claim:     $README_CLAIM"
echo "   - README table total:  $README_TABLE_TOTAL"
if [ -n "$HAS_DYNAMIC_BADGE" ]; then
  echo "   - README.md badge:     Dynamic Shields endpoint wired ✅"
else
  echo "   - README.md badge:     Static or missing ❌"
fi
echo "========================================================"

if [ -z "$HAS_DYNAMIC_BADGE" ]; then
  echo "❌ Error: README.md does not wire the dynamic shields endpoint: ${DYNAMIC_BADGE_URL}"
  exit 1
fi

if [ -n "$MEASURED_COV" ] && [ -n "$BADGE_COV" ] && [ "$MEASURED_COV" != "$BADGE_COV" ]; then
  echo "❌ Error: Coverage mismatch between measured coverage.out ($MEASURED_COV) and .github/badges/coverage.json ($BADGE_COV)"
  echo "::error title=Version Gate Failure::Coverage mismatch between measured coverage.out ($MEASURED_COV) and .github/badges/coverage.json ($BADGE_COV)"
  exit 1
fi

if [ -n "$BADGE_COV" ] && [ -n "$README_CLAIM" ] && [ "$BADGE_COV" != "$README_CLAIM" ]; then
  echo "❌ Error: Coverage mismatch between .github/badges/coverage.json ($BADGE_COV) and README.md summary claim ($README_CLAIM)"
  echo "::error title=Version Gate Failure::Coverage mismatch between .github/badges/coverage.json ($BADGE_COV) and README.md summary claim ($README_CLAIM)"
  exit 1
fi

if [ -n "$MEASURED_COV" ] && [ -n "$README_CLAIM" ] && [ "$MEASURED_COV" != "$README_CLAIM" ]; then
  echo "❌ Error: Coverage mismatch between measured coverage.out ($MEASURED_COV) and README.md summary claim ($README_CLAIM)"
  echo "::error title=Version Gate Failure::Coverage mismatch between measured coverage.out ($MEASURED_COV) and README.md summary claim ($README_CLAIM)"
  exit 1
fi

if [ -n "$MEASURED_COV" ] && [ -n "$README_TABLE_TOTAL" ] && [ "$MEASURED_COV" != "$README_TABLE_TOTAL" ]; then
  echo "❌ Error: Coverage mismatch between measured coverage.out ($MEASURED_COV) and README.md table total ($README_TABLE_TOTAL)"
  echo "::error title=Version Gate Failure::Coverage mismatch between measured coverage.out ($MEASURED_COV) and README.md table total ($README_TABLE_TOTAL)"
  exit 1
fi

echo "========================================================"
echo "🎨 Verifying Code Formatting (gofmt)"
echo "========================================================"

UNFORMATTED=$(gofmt -l $(git ls-files '*.go' 2>/dev/null | while read -r f; do [ -f "$f" ] && echo "$f"; done))
if [ -n "$UNFORMATTED" ]; then
  echo "❌ Error: Unformatted files detected by gofmt:"
  echo "$UNFORMATTED"
  echo "Run 'gofmt -w .' to fix."
  exit 1
fi
echo "✅ Code formatting is clean across all files."

echo "========================================================"
echo "🔍 Verifying Documentation Toolchain Baseline"
echo "========================================================"

DOC_FILES=(README.md docs/*.md docs/adr/*.md BENCHMARKS.md CHANGELOG.md)

GO_BASELINE=$(sed -n -E 's/^go ([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/p' go.mod | head -n1)
if [ -z "${GO_BASELINE}" ]; then
  echo '❌ Error: unable to read the Go baseline from go.mod'
  exit 1
fi

STALE_GO_REFS=$(grep -n -E 'Go 1\.25\+|Go 1\.25\.0|go1\.25\+' README.md CONTRIBUTING.md docs/*.md docs/adr/*.md 2>/dev/null || true)
if [ -n "${STALE_GO_REFS}" ]; then
  echo "❌ Error: Found current documentation claims below the Go ${GO_BASELINE} baseline:"
  echo "${STALE_GO_REFS}"
  exit 1
fi
echo "✅ Current documentation claims match the Go ${GO_BASELINE} baseline."

STALE_TAG_REFS=$(grep -n -E "@v[0-9]+[a-zA-Z0-9._-]*" README.md CONTRIBUTING.md docs/*.md docs/adr/*.md 2>/dev/null | grep -v "@v${PUBLISHED_VER}" || true)
if [ -n "${STALE_TAG_REFS}" ]; then
  echo "❌ Error: Found stale or invalid module tag references (must match @v${PUBLISHED_VER}):"
  echo "${STALE_TAG_REFS}"
  exit 1
fi
echo "✅ Module import tags strictly match @v${PUBLISHED_VER}."

echo "========================================================"
echo "🔍 Verifying CHANGELOG [### Added] Exported Symbols"
echo "========================================================"

CHANGELOG_FILE="CHANGELOG.md"
IN_UNRELEASED=0
IN_ADDED=0
FAILED_SYMBOLS=0
CHECKED_SYMBOLS=0
CHECKED_PACKAGES=0

while IFS= read -r line; do
  if [[ "${line}" =~ ^"## [Unreleased]" ]]; then
    IN_UNRELEASED=1
    continue
  fi
  if [ "${IN_UNRELEASED}" -eq 1 ] && [[ "${line}" =~ ^"## " ]]; then
    IN_UNRELEASED=0
    IN_ADDED=0
  fi
  if [ "${IN_UNRELEASED}" -eq 1 ] && [[ "${line}" =~ ^"### Added" ]]; then
    IN_ADDED=1
    continue
  fi
  if [ "${IN_ADDED}" -eq 1 ] && [[ "${line}" =~ ^"### " || "${line}" =~ ^"## " ]]; then
    IN_ADDED=0
  fi
  if [ "${IN_ADDED}" -eq 1 ] && [[ "${line}" =~ ^"- " ]]; then
    PKG=""
    LEAD_SYM=""
    PKG_DOT_REGEX='^-[[:space:]]+`?([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)`?'
    PKG_COLON_REGEX='^-[[:space:]]+`?([a-zA-Z0-9_]+)`?:'

    if [[ "${line}" =~ ${PKG_DOT_REGEX} ]]; then
      PKG="${BASH_REMATCH[1]}"
      LEAD_SYM="${BASH_REMATCH[2]}"
    elif [[ "${line}" =~ ${PKG_COLON_REGEX} ]]; then
      PKG="${BASH_REMATCH[1]}"
    fi

    if [ -z "${PKG}" ]; then
      echo "❌ Error: Could not determine package in bullet: ${line}"
      FAILED_SYMBOLS=1
      continue
    fi

    if [ ! -d "${PKG}" ]; then
      echo "❌ Error: Package directory '${PKG}' does not exist in repository!"
      FAILED_SYMBOLS=1
      continue
    fi

    CHECKED_PACKAGES=$((CHECKED_PACKAGES + 1))

    RAW_SYMBOLS=$(echo "${line}" | grep -oE '`[^`]+`' | tr -d '`' || true)
    ALL_SYMBOLS="${LEAD_SYM}"
    for s in ${RAW_SYMBOLS}; do
      ALL_SYMBOLS="${ALL_SYMBOLS} ${s}"
    done

    for sym in ${ALL_SYMBOLS}; do
      [ -z "${sym}" ] && continue
      [ "${sym}" = "${PKG}" ] && continue

      if [[ "${sym}" =~ ^${PKG}\.([a-zA-Z0-9_]+)$ ]]; then
        sym="${BASH_REMATCH[1]}"
      fi

      if [[ "${sym}" =~ "/" || "${sym}" =~ ^"http." || "${sym}" =~ ^"context." || "${sym}" =~ ^"pgxpool" || "${sym}" =~ ^"log/slog" || "${sym}" =~ ^"validator." || "${sym}" =~ ^"go-playground" ]]; then
        continue
      fi

      if [[ "${sym}" =~ "-" ]]; then
        continue
      fi

      if [[ ! "${sym}" =~ ^[A-Z] ]]; then
        continue
      fi

      CHECKED_SYMBOLS=$((CHECKED_SYMBOLS + 1))

      if ! git grep -q -w "${sym}" -- "${PKG}/*.go" ":!*_test.go"; then
        echo "❌ Error: Exported symbol '${sym}' referenced under '${PKG}' does not exist in ${PKG}/*.go"
        FAILED_SYMBOLS=1
      else
        echo "   - ${PKG}.${sym}: verified ✅"
      fi
    done
  fi
done < "${CHANGELOG_FILE}"

if [ "${FAILED_SYMBOLS}" -ne 0 ]; then
  echo "❌ Error: CHANGELOG symbol verification failed!"
  exit 1
fi
echo "✅ All CHANGELOG exported symbols exist in the repository (${CHECKED_PACKAGES} packages checked, ${CHECKED_SYMBOLS} symbols verified)."

echo "========================================================"
echo "✅ All versions, metrics, and symbols are strictly verified!"
echo "========================================================"
