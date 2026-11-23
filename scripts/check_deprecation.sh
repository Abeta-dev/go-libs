#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Strict Deprecation Window Gate (N + 2 Minor Releases)
#
# Enforces the go-libs Deprecation Policy (docs/VERSIONING.md):
#   1. Scans all production *.go files (excluding *_test.go) for
#      '// Deprecated:' annotations.
#   2. Every annotation must conform to:
#      // Deprecated: Since vX.Y.Z (remove in >= vA.B.C). Use <Replacement> instead.
#   3. Verifies the deprecation window is at least N + 2 minor releases:
#      (A > X) || (A == X && B >= Y + 2).
#   4. Compares against the previous release tag in git (when available) to
#      ensure no deprecated annotation is removed before the target version in
#      README.md / CHANGELOG.md reaches >= vA.B.C.
#
# Bash 3.2 compatible (verified by ./scripts/test_shell_compatibility.sh).
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

semver_ge() {
  local maj1="$1" min1="$2" pat1="$3"
  local maj2="$4" min2="$5" pat2="$6"
  if [ "${maj1}" -gt "${maj2}" ]; then return 0; fi
  if [ "${maj1}" -lt "${maj2}" ]; then return 1; fi
  if [ "${min1}" -gt "${min2}" ]; then return 0; fi
  if [ "${min1}" -lt "${min2}" ]; then return 1; fi
  if [ "${pat1}" -ge "${pat2}" ]; then return 0; fi
  return 1
}

TARGET_VER=""
if [ -f README.md ]; then
  TARGET_VER=$(grep -E 'go get github.com/abeta-dev/go-libs@v' README.md | head -n1 | sed -E 's/.*go-libs@v([0-9]+\.[0-9]+\.[0-9]+).*/\1/' || true)
fi
if [ -z "${TARGET_VER}" ] && [ -f CHANGELOG.md ]; then
  TARGET_VER=$(sed -n -E 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\].*/\1/p' CHANGELOG.md | head -n1 || true)
fi
if [ -z "${TARGET_VER}" ]; then
  TARGET_VER="0.0.0"
fi

TARGET_MAJOR=0
TARGET_MINOR=0
TARGET_PATCH=0
SEMVER_PART_REGEX='^([0-9]+)\.([0-9]+)\.([0-9]+)$'
if [[ "${TARGET_VER}" =~ ${SEMVER_PART_REGEX} ]]; then
  TARGET_MAJOR="${BASH_REMATCH[1]}"
  TARGET_MINOR="${BASH_REMATCH[2]}"
  TARGET_PATCH="${BASH_REMATCH[3]}"
fi

echo "========================================================"
echo "⏳ Verifying Strict N+2 Minor-Release Deprecation Policy"
echo "   - Current target version: v${TARGET_VER}"
echo "   - Required format:        // Deprecated: Since vX.Y.Z (remove in >= vA.B.C). Use <Replacement> instead."
echo "   - Minimum window:         N + 2 minor releases (B >= Y + 2 or A > X)"
echo "========================================================"

FAILED=0
ANNOTATION_COUNT=0

DEPRECATION_TRIGGER_REGEX='^[[:space:]]*//[[:space:]]*Deprecated:'
STRICT_DEPRECATION_REGEX='^[[:space:]]*//[[:space:]]*Deprecated: Since v([0-9]+)\.([0-9]+)\.([0-9]+) \(remove in >= v([0-9]+)\.([0-9]+)\.([0-9]+)\)\. Use ([^[:space:]].*) instead\.[[:space:]]*$'

GO_FILES=$(find . -type f -name '*.go' ! -name '*_test.go' ! -path './.git/*' ! -path './vendor/*' | sed 's|^\./||' | sort)

for file in ${GO_FILES}; do
  [ -f "${file}" ] || continue
  lineno=0
  while IFS= read -r line || [ -n "${line}" ]; do
    lineno=$((lineno + 1))
    if [[ "${line}" =~ ${DEPRECATION_TRIGGER_REGEX} ]]; then
      ANNOTATION_COUNT=$((ANNOTATION_COUNT + 1))
      if [[ "${line}" =~ ${STRICT_DEPRECATION_REGEX} ]]; then
        SINCE_MAJOR="${BASH_REMATCH[1]}"
        SINCE_MINOR="${BASH_REMATCH[2]}"
        SINCE_PATCH="${BASH_REMATCH[3]}"
        REMOVE_MAJOR="${BASH_REMATCH[4]}"
        REMOVE_MINOR="${BASH_REMATCH[5]}"
        REMOVE_PATCH="${BASH_REMATCH[6]}"
        REPLACEMENT="${BASH_REMATCH[7]}"

        MIN_REMOVE_MINOR=$((SINCE_MINOR + 2))
        if [ "${REMOVE_MAJOR}" -gt "${SINCE_MAJOR}" ] || { [ "${REMOVE_MAJOR}" -eq "${SINCE_MAJOR}" ] && [ "${REMOVE_MINOR}" -ge "${MIN_REMOVE_MINOR}" ]; }; then
          echo "   - ${file}:${lineno}: valid N+2 deprecation window (Since v${SINCE_MAJOR}.${SINCE_MINOR}.${SINCE_PATCH} -> remove in >= v${REMOVE_MAJOR}.${REMOVE_MINOR}.${REMOVE_PATCH}, replacement: ${REPLACEMENT}) ✅"
        else
          echo "❌ Error: ${file}:${lineno}: Deprecation window too short (Since v${SINCE_MAJOR}.${SINCE_MINOR}.${SINCE_PATCH} -> remove in >= v${REMOVE_MAJOR}.${REMOVE_MINOR}.${REMOVE_PATCH})."
          echo "   Policy requires at least N + 2 minor releases (earliest removal: >= v${SINCE_MAJOR}.${MIN_REMOVE_MINOR}.0)."
          FAILED=1
        fi
      else
        trimmed=$(printf '%s\n' "${line}" | sed -E 's/^[[:space:]]+//')
        echo "❌ Error: ${file}:${lineno}: Malformed '// Deprecated:' annotation:"
        echo "   Found:    ${trimmed}"
        echo "   Expected: // Deprecated: Since vX.Y.Z (remove in >= vA.B.C). Use <Replacement> instead."
        FAILED=1
      fi
    fi
  done < "${file}"
done

# Check previous release tag(s) in git to prevent premature removal of deprecated symbols
PREV_TAG="${PREV_RELEASE_TAG:-}"
if [ -z "${PREV_TAG}" ] && git rev-parse --git-dir >/dev/null 2>&1; then
  SEMVER_TAGS=$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | sed 's/^/v/' || true)
  if [ -n "${SEMVER_TAGS}" ]; then
    LATEST_TAG=$(printf '%s\n' "${SEMVER_TAGS}" | tail -n1)
    HEAD_SHA=$(git rev-parse HEAD 2>/dev/null || true)
    LATEST_TAG_SHA=$(git rev-parse "${LATEST_TAG}^{}" 2>/dev/null || true)
    DIRTY_GO=$(git status --porcelain -- '*.go' 2>/dev/null || true)
    if [ "${LATEST_TAG_SHA}" = "${HEAD_SHA}" ] && [ -z "${DIRTY_GO}" ]; then
      TAG_COUNT=$(printf '%s\n' "${SEMVER_TAGS}" | wc -l | tr -d ' ')
      if [ "${TAG_COUNT}" -ge 2 ]; then
        PREV_TAG=$(printf '%s\n' "${SEMVER_TAGS}" | tail -n2 | head -n1)
      else
        PREV_TAG="${LATEST_TAG}"
      fi
    else
      PREV_TAG="${LATEST_TAG}"
    fi
  fi
fi

if [ -n "${PREV_TAG}" ] && git rev-parse "${PREV_TAG}^{}" >/dev/null 2>&1; then
  echo "   - Comparing against previous release tag: ${PREV_TAG}"
  PREV_DEPRECATIONS=$(git grep -n 'Deprecated:' "${PREV_TAG}" -- '*.go' ':(exclude)*_test.go' 2>/dev/null || true)
  if [ -n "${PREV_DEPRECATIONS}" ]; then
    while IFS= read -r prev_entry; do
      [ -n "${prev_entry}" ] || continue
      # Format: <tag>:<file>:<lineno>:<content>
      rest="${prev_entry#${PREV_TAG}:}"
      prev_file="${rest%%:*}"
      rest="${rest#*:}"
      prev_lineno="${rest%%:*}"
      prev_line="${rest#*:}"

      if [[ "${prev_line}" =~ ${STRICT_DEPRECATION_REGEX} ]]; then
        SINCE_MAJOR="${BASH_REMATCH[1]}"
        SINCE_MINOR="${BASH_REMATCH[2]}"
        SINCE_PATCH="${BASH_REMATCH[3]}"
        REMOVE_MAJOR="${BASH_REMATCH[4]}"
        REMOVE_MINOR="${BASH_REMATCH[5]}"
        REMOVE_PATCH="${BASH_REMATCH[6]}"
        dep_marker="Deprecated: Since v${SINCE_MAJOR}.${SINCE_MINOR}.${SINCE_PATCH} (remove in >= v${REMOVE_MAJOR}.${REMOVE_MINOR}.${REMOVE_PATCH})"

        if [ ! -f "${prev_file}" ] || ! grep -Fq -- "${dep_marker}" "${prev_file}"; then
          if ! semver_ge "${TARGET_MAJOR}" "${TARGET_MINOR}" "${TARGET_PATCH}" "${REMOVE_MAJOR}" "${REMOVE_MINOR}" "${REMOVE_PATCH}"; then
            echo "❌ Error: Deprecated annotation from ${PREV_TAG} (${prev_file}:${prev_lineno}) was removed prematurely:"
            echo "   Annotation: ${dep_marker}"
            echo "   Current target version v${TARGET_VER} is below earliest removal version v${REMOVE_MAJOR}.${REMOVE_MINOR}.${REMOVE_PATCH}."
            FAILED=1
          else
            echo "   - ${prev_file}: deprecated symbol removal allowed at v${TARGET_VER} (>= v${REMOVE_MAJOR}.${REMOVE_MINOR}.${REMOVE_PATCH}) ✅"
          fi
        fi
      fi
    done <<< "${PREV_DEPRECATIONS}"
  fi
fi

if [ "${FAILED}" -ne 0 ]; then
  echo "========================================================"
  echo "❌ Error: Deprecation policy check failed!"
  exit 1
fi

echo "✅ Deprecation policy verified (${ANNOTATION_COUNT} active deprecation annotation(s) scanned; 0 violations)."
echo "========================================================"
