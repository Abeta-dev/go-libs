#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

BASELINE="${ROOT_DIR}/benchmarks/bench-baseline.txt"
CURRENT="${ROOT_DIR}/benchmarks/bench-current.txt"

echo "========================================================"
echo "⚡ Running Performance Benchmarks"
echo "========================================================"

# Run benchmarks across all packages (excluding scale tests by default)
go test -bench=. -benchmem -run=^$ ./... | tee "${CURRENT}"

if [ ! -f "${BASELINE}" ]; then
    echo "⚠️ Baseline file ${BASELINE} not found. Creating baseline from current run."
    cp "${CURRENT}" "${BASELINE}"
    echo "✅ Baseline created at ${BASELINE}."
    exit 0
fi

echo ""
echo "========================================================"
echo "📈 Comparing Against Baseline"
echo "========================================================"

# Check if benchstat is installed, or try to install it
if ! command -v benchstat &> /dev/null; then
    if [ -x "${HOME}/go/bin/benchstat" ]; then
        BENCHSTAT="${HOME}/go/bin/benchstat"
    else
        echo "Installing benchstat..."
        go install golang.org/x/perf/cmd/benchstat@latest || true
        BENCHSTAT="${HOME}/go/bin/benchstat"
    fi
else
    BENCHSTAT="benchstat"
fi

if command -v "${BENCHSTAT}" &> /dev/null; then
    echo "Using benchstat comparison:"
    COMPARISON=$("${BENCHSTAT}" "${BASELINE}" "${CURRENT}" 2>&1 || true)
    echo "${COMPARISON}"
    
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        {
            echo "### ⚡ Performance Benchmark Regression Analysis"
            echo '```text'
            echo "${COMPARISON}"
            echo '```'
        } >> "${GITHUB_STEP_SUMMARY}"
    fi
else
    echo "Benchstat tool not available, performing raw baseline comparison."
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        {
            echo "### ⚡ Performance Benchmark Summary"
            echo '```text'
            cat "${CURRENT}"
            echo '```'
        } >> "${GITHUB_STEP_SUMMARY}"
    fi
fi

echo "✅ Benchmark execution completed."
exit 0
