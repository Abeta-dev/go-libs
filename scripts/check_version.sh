#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Version, Symbol & Documentation Synchronization Gate
# Guarantees that:
#   1. README install examples and changelog agree with the selected release tag.
#   2. Tagged checkouts validate their actual tag and the tag points at HEAD.
#   3. Dynamic coverage badge and README claims match actual toolchain measurements.
#   4. README package count matches actual repository package count.
#   5. Code formatting is clean across all files (gofmt).
#   6. All current toolchain claims match the go.mod baseline.
#   7. Every package and exported symbol in the selected CHANGELOG section exists.
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

README_GET_VER=$(grep -E 'go get github.com/umesh0492/go-libs@v' README.md | head -n1 | sed -E 's/.*go-libs@v([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?).*/\1/')
CURRENT_TAG="${GIT_TAG:-$(git describe --tags --exact-match 2>/dev/null || true)}"
LATEST_CHANGELOG_VER=$(sed -n -E 's/^## \[([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?)\].*/\1/p' CHANGELOG.md | head -n1)

printf '%s\n' '========================================================'
printf '%s\n' '🔒 Verifying Published Module and Checkout Identity'
printf '   - README.md go get:        v%s\n' "${README_GET_VER:-N/A}"
printf '   - Latest changelog:        v%s\n' "${LATEST_CHANGELOG_VER:-N/A}"
printf '   - Current source tag:      %s\n' "${CURRENT_TAG:-untagged}"
printf '%s\n' '========================================================'

if ! [[ "${README_GET_VER}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "❌ Error: README.md must contain a SemVer go get version; found v${README_GET_VER:-N/A}"
  exit 1
fi
if [ "${LATEST_CHANGELOG_VER}" != "${README_GET_VER}" ]; then
  echo "❌ Error: latest CHANGELOG.md release (v${LATEST_CHANGELOG_VER:-N/A}) does not match README.md go get version (v${README_GET_VER})"
  exit 1
fi

if [ -n "${CURRENT_TAG}" ]; then
  if ! [[ "${CURRENT_TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
    echo "❌ Error: current tag is not a SemVer release tag: ${CURRENT_TAG}"
    exit 1
  fi
  if [ "${CURRENT_TAG#v}" != "${README_GET_VER}" ]; then
    echo "❌ Error: tagged checkout ${CURRENT_TAG} does not match README.md go get version (v${README_GET_VER})"
    exit 1
  fi
  if [ "$(git rev-parse "${CURRENT_TAG}^{}")" != "$(git rev-parse HEAD)" ]; then
    echo "❌ Error: tagged checkout ${CURRENT_TAG} does not resolve to HEAD"
    exit 1
  fi
  echo "✅ Tagged checkout ${CURRENT_TAG} matches README, CHANGELOG, and HEAD."
else
  echo "✅ Untagged checkout references the latest documented release v${README_GET_VER}."
fi

echo "========================================================"
echo "📦 Verifying Package Inventory & Count Synchronization"
echo "========================================================"

EXPECTED_PKG_COUNT=31
MEASURED_PKGS=$(go list ./... | grep -vE '^github\.com/umesh0492/go-libs$|/(loadgen|tests?)$' | wc -l | tr -d ' ')
README_PKG_COUNT=$(grep -oE '[0-9]+ packages' README.md | awk '{print $1}' | head -n1 || true)
TABLE_PKG_COUNT=$(sed -n '/| Package | Purpose | Statement Coverage |/,/| \*\*Total Statement Coverage\*\*/p' README.md | grep -E '^\| `[a-zA-Z0-9_/]+` \|' | wc -l | tr -d ' ')

echo "   - Expected Packages:                 $EXPECTED_PKG_COUNT"
echo "   - Measured library packages (go list): $MEASURED_PKGS"
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

STALE_TAG_REFS=$(grep -n -E "@v[0-9]+[a-zA-Z0-9._-]*" README.md CONTRIBUTING.md docs/*.md docs/adr/*.md 2>/dev/null | grep -v "@v${README_GET_VER}" || true)
if [ -n "${STALE_TAG_REFS}" ]; then
  echo "❌ Error: Found stale or invalid module tag references (must match @v${README_GET_VER}):"
  echo "${STALE_TAG_REFS}"
  exit 1
fi
echo "✅ Module import tags strictly match @v${README_GET_VER}."

echo "========================================================"
echo "🔍 Verifying Selected CHANGELOG Exported Symbols"
echo "========================================================"

CHANGELOG_FILE="CHANGELOG.md"
UNRELEASED_CONTENT=$(awk '
  /^## \[Unreleased\]/{in_section=1; next}
  in_section && /^## \[/{exit}
  in_section && $0 !~ /^[[:space:]]*$/ {print}
' "${CHANGELOG_FILE}")

if [ -n "${UNRELEASED_CONTENT}" ]; then
  SECTION_NAME='Unreleased'
  SECTION_CONTENT="${UNRELEASED_CONTENT}"
else
  SECTION_NAME="v${LATEST_CHANGELOG_VER}"
  SECTION_CONTENT=$(awk '
    /^## \[[0-9]+\.[0-9]+\.[0-9]+/{if (!found) {found=1; next}}
    found && /^## \[/{exit}
    found {print}
  ' "${CHANGELOG_FILE}")
fi

if [ -z "${SECTION_CONTENT}" ]; then
  echo "❌ Error: selected CHANGELOG section ${SECTION_NAME} is empty or missing"
  exit 1
fi

FAILED_SYMBOLS=0
CHECKED_SYMBOLS=0
CHECKED_PACKAGES=0
EXPORTED_BULLETS=0
while IFS= read -r line; do
  [[ "${line}" =~ ^-[[:space:]]+ ]] || continue

  PKG=''
  LEAD_SYMBOL=''
  PKG_DOT_REGEX='^-[[:space:]]+`?([a-zA-Z0-9_]+)\.([A-Z][a-zA-Z0-9_]*)`?'
  PKG_COLON_REGEX='^-[[:space:]]+`?([a-zA-Z0-9_]+)`?:'
  if [[ "${line}" =~ ${PKG_DOT_REGEX} ]]; then
    PKG="${BASH_REMATCH[1]}"
    LEAD_SYMBOL="${BASH_REMATCH[2]}"
  elif [[ "${line}" =~ ${PKG_COLON_REGEX} ]]; then
    PKG="${BASH_REMATCH[1]}"
  else
    continue
  fi

  RAW_SYMBOLS=$(printf '%s\n' "${line}" | grep -oE '`[^`]+`' | tr -d '`' || true)
  SYMBOLS="${LEAD_SYMBOL}"
  for symbol in ${RAW_SYMBOLS}; do
    if [[ "${symbol}" =~ ^${PKG}\.([A-Z][a-zA-Z0-9_]*)$ ]]; then
      SYMBOLS="${SYMBOLS} ${BASH_REMATCH[1]}"
    elif [[ "${symbol}" =~ ^[A-Z][a-zA-Z0-9_]*$ ]]; then
      SYMBOLS="${SYMBOLS} ${symbol}"
    fi
  done
  SYMBOLS=$(printf '%s\n' "${SYMBOLS}" | xargs)
  [ -n "${SYMBOLS}" ] || continue

  EXPORTED_BULLETS=$((EXPORTED_BULLETS + 1))
  if [ ! -d "${PKG}" ]; then
    echo "❌ Error: package directory '${PKG}' does not exist for CHANGELOG bullet: ${line}"
    FAILED_SYMBOLS=1
    continue
  fi
  CHECKED_PACKAGES=$((CHECKED_PACKAGES + 1))

  for symbol in ${SYMBOLS}; do
    CHECKED_SYMBOLS=$((CHECKED_SYMBOLS + 1))
    if ! git grep -q -w "${symbol}" -- "${PKG}" ':(exclude)**/*_test.go'; then
      echo "❌ Error: exported symbol '${symbol}' referenced under '${PKG}' does not exist in production source"
      FAILED_SYMBOLS=1
    else
      echo "   - ${PKG}.${symbol}: verified ✅"
    fi
  done
done <<< "${SECTION_CONTENT}"

if [ "${EXPORTED_BULLETS}" -gt 0 ] && { [ "${CHECKED_PACKAGES}" -eq 0 ] || [ "${CHECKED_SYMBOLS}" -eq 0 ]; }; then
  echo "❌ Error: ${SECTION_NAME} has exported-symbol bullets but zero packages or symbols were checked"
  exit 1
fi
if [ "${FAILED_SYMBOLS}" -ne 0 ]; then
  echo "❌ Error: CHANGELOG symbol verification failed for ${SECTION_NAME}!"
  exit 1
fi
echo "✅ ${SECTION_NAME} CHANGELOG symbols exist in the repository (${CHECKED_PACKAGES} packages checked, ${CHECKED_SYMBOLS} symbols verified)."

echo "========================================================"
echo "✅ All versions, metrics, and symbols are strictly verified!"
echo "========================================================"
