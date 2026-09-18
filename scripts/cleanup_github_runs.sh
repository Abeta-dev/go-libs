#!/usr/bin/env bash
# ==============================================================================
# cleanup_github_runs.sh
# Automated cleanup utility for GitHub Actions historical workflow runs and draft releases.
# Supports both GitHub CLI (`gh`) and standard `curl` with $GITHUB_TOKEN / $GH_TOKEN.
# ==============================================================================

set -euo pipefail

DEFAULT_REPO="Abeta-dev/go-libs"
REPO="${DEFAULT_REPO}"
DRY_RUN="false"
PURGE_RUNS="true"
PURGE_DRAFTS="true"

# Auto-detect repo from git remote if available
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  REMOTE_URL=$(git config --get remote.origin.url || true)
  if [[ "${REMOTE_URL}" =~ github\.com[:/]([^/]+/[^/.]+)(\.git)?$ ]]; then
    REPO="${BASH_REMATCH[1]}"
  fi
fi

usage() {
  cat << USAGE
Usage: $(basename "$0") [OPTIONS]

Purges historical workflow runs and draft releases from a GitHub repository.

Options:
  -r, --repo OWNER/REPO   Target repository (default: ${REPO})
  -d, --dry-run           Log candidate items without deleting them
      --runs-only         Purge completed/failed/cancelled workflow runs only
      --drafts-only       Purge draft releases only
  -h, --help              Show this help message

Authentication:
  Requires GITHUB_TOKEN or GH_TOKEN environment variable (with 'repo' / 'actions' scope).
  Alternatively, authenticated GitHub CLI ('gh auth login') can be used.

Examples:
  GITHUB_TOKEN="ghp_xxx" ./scripts/cleanup_github_runs.sh --dry-run
  ./scripts/cleanup_github_runs.sh --repo Abeta-dev/go-libs
USAGE
  exit 0
}

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    -r|--repo)
      REPO="$2"
      shift 2
      ;;
    -d|--dry-run)
      DRY_RUN="true"
      shift
      ;;
    --runs-only)
      PURGE_RUNS="true"
      PURGE_DRAFTS="false"
      shift
      ;;
    --drafts-only)
      PURGE_RUNS="false"
      PURGE_DRAFTS="true"
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "❌ Unknown option: $1" >&2
      usage
      ;;
  esac
done

TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
HAS_GH="false"
if command -v gh >/dev/null 2>&1; then
  if gh auth status >/dev/null 2>&1; then
    HAS_GH="true"
  fi
fi

if [[ -z "${TOKEN}" && "${HAS_GH}" != "true" ]]; then
  echo "❌ Error: Neither GITHUB_TOKEN (or GH_TOKEN) is set, nor is 'gh' CLI authenticated." >&2
  echo "   Please export GITHUB_TOKEN='ghp_yourToken' before running this script." >&2
  exit 1
fi

echo "========================================================"
echo "🧹 GitHub Actions & Draft Releases Purge Utility"
echo "   Target Repository: ${REPO}"
echo "   Dry Run Mode:      ${DRY_RUN}"
echo "   Purge Runs:        ${PURGE_RUNS}"
echo "   Purge Drafts:      ${PURGE_DRAFTS}"
echo "   Engine:            $([ "${HAS_GH}" = "true" ] && echo "GitHub CLI (gh)" || echo "GitHub REST API (curl + jq)")"
echo "========================================================"

api_get() {
  local endpoint="$1"
  if [[ "${HAS_GH}" == "true" ]]; then
    gh api -H "Accept: application/vnd.github+json" "${endpoint}"
  else
    curl -sSL -H "Accept: application/vnd.github+json" \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "https://api.github.com/${endpoint}"
  fi
}

api_delete() {
  local endpoint="$1"
  if [[ "${HAS_GH}" == "true" ]]; then
    gh api -X DELETE -H "Accept: application/vnd.github+json" "${endpoint}" >/dev/null
  else
    curl -sSL -X DELETE -H "Accept: application/vnd.github+json" \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "https://api.github.com/${endpoint}" >/dev/null
  fi
}

# 1. Purge Historical Workflow Runs
if [[ "${PURGE_RUNS}" == "true" ]]; then
  echo ""
  echo "🔍 [1/2] Scanning for completed/failed/cancelled workflow runs..."
  PAGE=1
  PER_PAGE=100
  TOTAL_RUNS_DELETED=0

  while true; do
    RESPONSE=$(api_get "repos/${REPO}/actions/runs?per_page=${PER_PAGE}&page=${PAGE}")
    RUNS=$(echo "${RESPONSE}" | jq -c '.workflow_runs[]? | select(.status == "completed" or .status == "cancelled" or .status == "failure" or .conclusion != null) | {id: .id, name: .name, status: .status, conclusion: .conclusion, created_at: .created_at}')

    if [[ -z "${RUNS}" ]]; then
      echo "   No eligible workflow runs found on page ${PAGE}."
      break
    fi

    while IFS= read -r run; do
      RUN_ID=$(echo "${run}" | jq -r '.id')
      RUN_NAME=$(echo "${run}" | jq -r '.name')
      RUN_CONCL=$(echo "${run}" | jq -r '.conclusion')
      RUN_DATE=$(echo "${run}" | jq -r '.created_at')

      if [[ "${DRY_RUN}" == "true" ]]; then
        echo "   [DRY RUN] Would delete Run ID ${RUN_ID} ('${RUN_NAME}' - ${RUN_CONCL}, ${RUN_DATE})"
      else
        echo "   🗑️ Deleting Run ID ${RUN_ID} ('${RUN_NAME}' - ${RUN_CONCL})..."
        api_delete "repos/${REPO}/actions/runs/${RUN_ID}"
        TOTAL_RUNS_DELETED=$((TOTAL_RUNS_DELETED + 1))
      fi
    done <<< "${RUNS}"

    TOTAL_COUNT=$(echo "${RESPONSE}" | jq -r '.total_count // 0')
    if [[ "${TOTAL_COUNT}" -le $((PAGE * PER_PAGE)) ]]; then
      break
    fi
    PAGE=$((PAGE + 1))
  done

  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "✅ [1/2] Dry run complete for workflow runs."
  else
    echo "✅ [1/2] Finished workflow runs cleanup. Deleted ${TOTAL_RUNS_DELETED} runs."
  fi
fi

# 2. Purge Draft Releases
if [[ "${PURGE_DRAFTS}" == "true" ]]; then
  echo ""
  echo "🔍 [2/2] Scanning for draft releases..."
  RELEASES=$(api_get "repos/${REPO}/releases?per_page=100")
  DRAFT_RELEASES=$(echo "${RELEASES}" | jq -c '.[]? | select(.draft == true) | {id: .id, name: .name, tag_name: .tag_name}')

  TOTAL_DRAFTS_DELETED=0
  if [[ -z "${DRAFT_RELEASES}" ]]; then
    echo "   No draft releases found."
  else
    while IFS= read -r draft; do
      DRAFT_ID=$(echo "${draft}" | jq -r '.id')
      DRAFT_NAME=$(echo "${draft}" | jq -r '.name // .tag_name // "unnamed"')

      if [[ "${DRY_RUN}" == "true" ]]; then
        echo "   [DRY RUN] Would delete Draft Release ID ${DRAFT_ID} ('${DRAFT_NAME}')"
      else
        echo "   🗑️ Deleting Draft Release ID ${DRAFT_ID} ('${DRAFT_NAME}')..."
        api_delete "repos/${REPO}/releases/${DRAFT_ID}"
        TOTAL_DRAFTS_DELETED=$((TOTAL_DRAFTS_DELETED + 1))
      fi
    done <<< "${DRAFT_RELEASES}"
  fi

  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "✅ [2/2] Dry run complete for draft releases."
  else
    echo "✅ [2/2] Finished draft release cleanup. Deleted ${TOTAL_DRAFTS_DELETED} draft releases."
  fi
fi

echo ""
echo "✨ Repository cleanup workflow complete."
