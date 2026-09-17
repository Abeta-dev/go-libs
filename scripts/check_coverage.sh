#!/usr/bin/env bash
set -euo pipefail

# Scripts directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

GLOBAL_FLOOR=90.0
PACKAGE_FLOOR=85.0
GINMW_REQUIRED=100.0

command -v bc >/dev/null || { echo "bc must be available on PATH" >&2; exit 1; }

echo "========================================================"
echo "🛡️ Running Comprehensive Statement Coverage Gate"
echo "   - Global Floor:       >= ${GLOBAL_FLOOR}%"
echo "   - Per-Package Floor:  >= ${PACKAGE_FLOOR}%"
echo "   - ginmw Gate:         == ${GINMW_REQUIRED}%"
echo "========================================================"

# List library packages, excluding the root documentation-test package and load generators.
PACKAGES=$(go list ./... | grep -vE '^github\.com/umesh0492/go-libs$|/loadgen$')
PKGS_COMMA=$(echo "${PACKAGES}" | tr '\n' ',' | sed 's/,$//')

# Generate merged coverage profile across all packages. Never continue with a
# stale ignored profile when the test command or output write fails.
rm -f coverage.out
go test -coverprofile=coverage.out ${PACKAGES}
test -s coverage.out

# Extract global coverage percentage
GLOBAL_COV_STR=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
GLOBAL_COV=$(echo "${GLOBAL_COV_STR}" | tr -d '%')
echo "==> Global Statement Coverage: ${GLOBAL_COV_STR}"

# Per-package check and table generation
TABLE_ROWS=""
FAILED_PACKAGES=()

# Run per-package coverage check
PER_PKG_OUTPUT=$(go test -cover ${PACKAGES})

while IFS= read -r line; do
    if [[ "${line}" =~ coverage:\ ([0-9.]+)%\ of\ statements ]]; then
        PKG_NAME=$(echo "${line}" | awk '{print $2}' | sed 's|github.com/umesh0492/go-libs/||')
        PKG_COV="${BASH_REMATCH[1]}"
        TABLE_ROWS="${TABLE_ROWS}\n| \`${PKG_NAME}\` | **${PKG_COV}%** |"
        
        # Check if ginmw meets 100%
        if [ "${PKG_NAME}" = "ginmw" ]; then
            if (( $(echo "${PKG_COV} < ${GINMW_REQUIRED}" | bc -l) )); then
                echo "❌ Package ${PKG_NAME} failed strict requirement: ${PKG_COV}% < ${GINMW_REQUIRED}%"
                FAILED_PACKAGES+=("${PKG_NAME} (${PKG_COV}% < ${GINMW_REQUIRED}%)")
            fi
        fi

        # Check per-package floor
        if (( $(echo "${PKG_COV} < ${PACKAGE_FLOOR}" | bc -l) )); then
            echo "❌ Package ${PKG_NAME} failed floor: ${PKG_COV}% < ${PACKAGE_FLOOR}%"
            FAILED_PACKAGES+=("${PKG_NAME} (${PKG_COV}% < ${PACKAGE_FLOOR}%)")
        fi
    elif [[ "${line}" =~ coverage:\ \[no\ statements\] ]]; then
        PKG_NAME=$(echo "${line}" | awk '{print $2}' | sed 's|github.com/umesh0492/go-libs/||')
        TABLE_ROWS="${TABLE_ROWS}\n| \`${PKG_NAME}\` | *No statements (interfaces/types)* |"
    fi
done <<< "${PER_PKG_OUTPUT}"

# Verify README.md per-package coverage table synchronization
echo ""
echo "========================================================"
echo "📑 Verifying README.md Per-Package Coverage Table Synchronization"
echo "========================================================"

COVERAGE_TABLE=$(sed -n '/| Package | Purpose | Statement Coverage |/,/| \*\*Total Statement Coverage\*\*/p' "${ROOT_DIR}/README.md")
TABLE_PKG_COUNT=$(echo "${COVERAGE_TABLE}" | grep -E '^\| `[a-zA-Z0-9_/]+` \|' | wc -l | tr -d ' ')
MEASURED_PKG_COUNT=$(echo "${PACKAGES}" | wc -w | tr -d ' ')

if [ "${TABLE_PKG_COUNT}" != "${MEASURED_PKG_COUNT}" ]; then
    echo "❌ Error: README.md coverage table row count (${TABLE_PKG_COUNT}) does not match measured packages (${MEASURED_PKG_COUNT})"
    FAILED_PACKAGES+=("README table row count mismatch (${TABLE_PKG_COUNT} != ${MEASURED_PKG_COUNT})")
fi

while IFS= read -r line; do
    if [[ "${line}" =~ coverage:\ ([0-9.]+)%\ of\ statements ]]; then
        PKG_NAME=$(echo "${line}" | awk '{print $2}' | sed 's|github.com/umesh0492/go-libs/||')
        PKG_COV="${BASH_REMATCH[1]}"
        
        README_ROW=$(echo "${COVERAGE_TABLE}" | grep -E "^\| \`${PKG_NAME}\` \|" || true)
        if [ -z "${README_ROW}" ]; then
            echo "❌ Package '${PKG_NAME}' missing from README.md coverage table"
            FAILED_PACKAGES+=("${PKG_NAME} (missing from README.md table)")
        else
            README_COV_VAL=$(echo "${README_ROW}" | awk -F'|' '{print $4}' | sed -E 's/^[[:space:]]+//;s/[[:space:]]+$//')
            README_PCT=$(echo "${README_COV_VAL}" | grep -oE '[0-9]+\.[0-9]+' || true)
            if [ -n "${README_PCT}" ]; then
                if [ "${README_PCT}" != "${PKG_COV}" ]; then
                    echo "❌ Coverage drift for package '${PKG_NAME}': README claims ${README_PCT}%, measured is ${PKG_COV}%"
                    FAILED_PACKAGES+=("${PKG_NAME} (drift: README=${README_PCT}%, measured=${PKG_COV}%)")
                else
                    echo "   - ${PKG_NAME}: measured ${PKG_COV}% == README ${README_COV_VAL} ✅"
                fi
            else
                echo "❌ Expected numeric coverage for package '${PKG_NAME}' in README.md, found: ${README_COV_VAL}"
                FAILED_PACKAGES+=("${PKG_NAME} (drift: README has non-numeric '${README_COV_VAL}', measured is ${PKG_COV}%)")
            fi
        fi
    elif [[ "${line}" =~ coverage:\ \[no\ statements\] ]]; then
        PKG_NAME=$(echo "${line}" | awk '{print $2}' | sed 's|github.com/umesh0492/go-libs/||')
        README_ROW=$(echo "${COVERAGE_TABLE}" | grep -E "^\| \`${PKG_NAME}\` \|" || true)
        if [ -z "${README_ROW}" ]; then
            echo "❌ Package '${PKG_NAME}' missing from README.md coverage table"
            FAILED_PACKAGES+=("${PKG_NAME} (missing from README.md table)")
        else
            README_COV_VAL=$(echo "${README_ROW}" | awk -F'|' '{print $4}' | sed -E 's/^[[:space:]]+//;s/[[:space:]]+$//')
            if echo "${README_COV_VAL}" | grep -qiE 'N/A|no statements'; then
                echo "   - ${PKG_NAME}: measured [no statements] == README ${README_COV_VAL} ✅"
            else
                echo "❌ Coverage drift for package '${PKG_NAME}': README claims ${README_COV_VAL}, but package has no statements"
                FAILED_PACKAGES+=("${PKG_NAME} (drift: README=${README_COV_VAL}, measured=[no statements])")
            fi
        fi
    fi
done <<< "${PER_PKG_OUTPUT}"

README_TABLE_TOTAL=$(echo "${COVERAGE_TABLE}" | grep -E '^\| \*\*Total Statement Coverage\*\* \|' | grep -oE '[0-9]+\.[0-9]+%' | head -n1 || true)
if [ -n "${README_TABLE_TOTAL}" ] && [ "${README_TABLE_TOTAL}" != "${GLOBAL_COV_STR}" ]; then
    echo "❌ Total statement coverage drift: README table claims ${README_TABLE_TOTAL}, measured is ${GLOBAL_COV_STR}"
    FAILED_PACKAGES+=("Total coverage drift: README=${README_TABLE_TOTAL} vs measured=${GLOBAL_COV_STR}")
fi

# Verify docs/VERSIONING.md per-package coverage table synchronization
echo ""
echo "========================================================"
echo "📑 Verifying docs/VERSIONING.md Per-Package Coverage Table Synchronization"
echo "========================================================"

VERSIONING_TABLE=$(sed -n '/| Package | Maturity Tier | Measured Coverage |/,/^---/p' "${ROOT_DIR}/docs/VERSIONING.md")
VERSIONING_PKG_COUNT=$(echo "${VERSIONING_TABLE}" | grep -E '^\| `[a-zA-Z0-9_/]+` \|' | wc -l | tr -d ' ')

if [ "${VERSIONING_PKG_COUNT}" != "${MEASURED_PKG_COUNT}" ]; then
    echo "❌ Error: docs/VERSIONING.md coverage table row count (${VERSIONING_PKG_COUNT}) does not match measured packages (${MEASURED_PKG_COUNT})"
    FAILED_PACKAGES+=("docs/VERSIONING.md table row count mismatch (${VERSIONING_PKG_COUNT} != ${MEASURED_PKG_COUNT})")
fi

while IFS= read -r line; do
    if [[ "${line}" =~ coverage:\ ([0-9.]+)%\ of\ statements ]]; then
        PKG_NAME=$(echo "${line}" | awk '{print $2}' | sed 's|github.com/umesh0492/go-libs/||')
        PKG_COV="${BASH_REMATCH[1]}"
        
        VER_ROW=$(echo "${VERSIONING_TABLE}" | grep -E "^\| \`${PKG_NAME}\` \|" || true)
        if [ -z "${VER_ROW}" ]; then
            echo "❌ Package '${PKG_NAME}' missing from docs/VERSIONING.md coverage table"
            FAILED_PACKAGES+=("${PKG_NAME} (missing from docs/VERSIONING.md table)")
        else
            VER_COV_VAL=$(echo "${VER_ROW}" | awk -F'|' '{print $4}' | sed -E 's/^[[:space:]]+//;s/[[:space:]]+$//')
            VER_PCT=$(echo "${VER_COV_VAL}" | grep -oE '[0-9]+\.[0-9]+' || true)
            if [ -n "${VER_PCT}" ]; then
                if [ "${VER_PCT}" != "${PKG_COV}" ]; then
                    echo "❌ Coverage drift for package '${PKG_NAME}': docs/VERSIONING.md claims ${VER_PCT}%, measured is ${PKG_COV}%"
                    FAILED_PACKAGES+=("${PKG_NAME} (drift: docs/VERSIONING.md=${VER_PCT}%, measured=${PKG_COV}%)")
                else
                    echo "   - ${PKG_NAME}: measured ${PKG_COV}% == docs/VERSIONING.md ${VER_COV_VAL} ✅"
                fi
            else
                echo "❌ Expected numeric coverage for package '${PKG_NAME}' in docs/VERSIONING.md, found: ${VER_COV_VAL}"
                FAILED_PACKAGES+=("${PKG_NAME} (drift: docs/VERSIONING.md has non-numeric '${VER_COV_VAL}', measured is ${PKG_COV}%)")
            fi
        fi
    elif [[ "${line}" =~ coverage:\ \[no\ statements\] ]]; then
        PKG_NAME=$(echo "${line}" | awk '{print $2}' | sed 's|github.com/umesh0492/go-libs/||')
        VER_ROW=$(echo "${VERSIONING_TABLE}" | grep -E "^\| \`${PKG_NAME}\` \|" || true)
        if [ -z "${VER_ROW}" ]; then
            echo "❌ Package '${PKG_NAME}' missing from docs/VERSIONING.md coverage table"
            FAILED_PACKAGES+=("${PKG_NAME} (missing from docs/VERSIONING.md table)")
        else
            VER_COV_VAL=$(echo "${VER_ROW}" | awk -F'|' '{print $4}' | sed -E 's/^[[:space:]]+//;s/[[:space:]]+$//')
            if echo "${VER_COV_VAL}" | grep -qiE 'interfaces|n/a|no statements'; then
                echo "   - ${PKG_NAME}: measured [no statements] == docs/VERSIONING.md ${VER_COV_VAL} ✅"
            else
                echo "❌ Coverage drift for package '${PKG_NAME}': docs/VERSIONING.md claims ${VER_COV_VAL}, but package has no statements"
                FAILED_PACKAGES+=("${PKG_NAME} (drift: docs/VERSIONING.md=${VER_COV_VAL}, measured=[no statements])")
            fi
        fi
    fi
done <<< "${PER_PKG_OUTPUT}"

# Generate Shields.io endpoint badge JSON
COLOR="brightgreen"
if (( $(echo "${GLOBAL_COV} < 90.0" | bc -l) )); then
    COLOR="yellow"
fi
if (( $(echo "${GLOBAL_COV} < 80.0" | bc -l) )); then
    COLOR="red"
fi

mkdir -p "${ROOT_DIR}/.github/badges"
cat << BADGE_EOF > "${ROOT_DIR}/.github/badges/coverage.json"
{
  "schemaVersion": 1,
  "label": "coverage",
  "message": "${GLOBAL_COV_STR}",
  "color": "${COLOR}"
}
BADGE_EOF

# Output to GitHub Step Summary if running in GitHub Actions
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    {
        echo "### 📊 Verified CI/CD Statement Coverage Gate"
        echo ""
        echo "- **Overall Repository Statement Coverage**: \`${GLOBAL_COV_STR}\` (Gate: \`>= ${GLOBAL_FLOOR}%\` ✅)"
        echo "- **Strict \`ginmw\` Gate**: \`100.0%\` ✅"
        echo "- **Per-Package Floor**: \`>= ${PACKAGE_FLOOR}%\` ✅"
        echo ""
        echo "| Package | Exact Statement Coverage |"
        echo "|---|---|"
        echo -e "${TABLE_ROWS}"
    } >> "${GITHUB_STEP_SUMMARY}"
fi

# Print summary
echo ""
echo "--------------------------------------------------------"
echo "Summary:"
echo "  Global Coverage: ${GLOBAL_COV_STR} (Floor: ${GLOBAL_FLOOR}%)"
echo "  Shields Badge:   .github/badges/coverage.json generated"
echo "--------------------------------------------------------"

# Verify global floor
if (( $(echo "${GLOBAL_COV} < ${GLOBAL_FLOOR}" | bc -l) )); then
    echo "❌ Global coverage ${GLOBAL_COV}% is below floor ${GLOBAL_FLOOR}%"
    echo "::error title=Coverage Gate Failure::Global coverage ${GLOBAL_COV}% is below floor ${GLOBAL_FLOOR}%"
    exit 1
fi

# Verify no package failed floor
if [ ${#FAILED_PACKAGES[@]} -gt 0 ]; then
    echo "❌ One or more packages failed coverage requirements:"
    for failed in "${FAILED_PACKAGES[@]}"; do
        echo "   - ${failed}"
        echo "::error title=Coverage Gate Failure::${failed}"
    done
    exit 1
fi

echo "✅ All coverage gates passed successfully!"
"${SCRIPT_DIR}/check_version.sh"
exit 0
