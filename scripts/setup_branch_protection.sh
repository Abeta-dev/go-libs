#!/usr/bin/env bash
# ==============================================================================
# setup_branch_protection.sh
# Configures GitHub branch protection and repository access for umesh0492/go-libs.
# Enforces CI status checks, PR approvals, admin enforcement, and public community access.
# ==============================================================================

set -euo pipefail

REPO_OWNER="${REPO_OWNER:-umesh0492}"
REPO_NAME="${REPO_NAME:-go-libs}"
BRANCH="${BRANCH:-main}"
TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}"

usage() {
  cat << USAGE
Usage: $(basename "$0") [OPTIONS]

Applies branch protection rules and community settings to ${REPO_OWNER}/${REPO_NAME}.

Options:
  -o, --owner OWNER       Repository owner (default: ${REPO_OWNER})
  -r, --repo REPO         Repository name (default: ${REPO_NAME})
  -b, --branch BRANCH     Branch to protect (default: ${BRANCH})
  -h, --help              Show this help message

Authentication:
  Requires GITHUB_TOKEN or GH_TOKEN with 'repo' / 'admin:org' scope.

Examples:
  export GITHUB_TOKEN="ghp_yourTokenHere"
  ./scripts/setup_branch_protection.sh
USAGE
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--owner)
      REPO_OWNER="$2"
      shift 2
      ;;
    -r|--repo)
      REPO_NAME="$2"
      shift 2
      ;;
    -b|--branch)
      BRANCH="$2"
      shift 2
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

if [[ -z "${TOKEN}" ]]; then
  echo "❌ Error: GITHUB_TOKEN or GH_TOKEN environment variable is not set." >&2
  echo "   Export a token with repo admin scope: export GITHUB_TOKEN='ghp_xxx'" >&2
  exit 1
fi

echo "========================================================"
echo "🛡️ Configuring GitHub Repository & Branch Protection"
echo "   Target:  ${REPO_OWNER}/${REPO_NAME} (${BRANCH})"
echo "========================================================"

# Step 1: Configure Repository Settings (Public Issues, Forking, Discussions)
echo ""
echo "⚙️ [1/2] Updating repository settings (Public Issues, Discussions, Forking)..."
REPO_PAYLOAD=$(cat << JSON
{
  "has_issues": true,
  "has_discussions": true,
  "has_projects": false,
  "has_wiki": false,
  "allow_forking": true,
  "allow_squash_merge": true,
  "allow_merge_commit": false,
  "allow_rebase_merge": true,
  "delete_branch_on_merge": true
}
JSON
)

HTTP_STATUS=$(curl -sSL -o /tmp/repo_settings_res.json -w "%{http_code}" \
  -X PATCH \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}" \
  -d "${REPO_PAYLOAD}")

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  echo "   ✅ Repository features configured successfully (HTTP ${HTTP_STATUS})."
  echo "      - Public Issues: Enabled"
  echo "      - Discussions:   Enabled"
  echo "      - Forking:       Enabled"
  echo "      - Linear Merge:  Enforced (Squash/Rebase only)"
else
  echo "   ⚠️ Warning: Repository settings update returned HTTP ${HTTP_STATUS}:"
  cat /tmp/repo_settings_res.json || true
  echo ""
fi

# Step 2: Apply Main Branch Protection
echo ""
echo "🔒 [2/2] Enforcing branch protection on '${BRANCH}'..."
PROTECTION_PAYLOAD=$(cat << JSON
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "CI / Test & Race (1.25.x)",
      "CI / GolangCI-Lint",
      "CI / Documentation & Version Gate",
      "CI / Scale & Load Tests (-tags=scale)"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": true,
    "required_approving_review_count": 1,
    "require_last_push_approval": true
  },
  "restrictions": {
    "users": ["umesh0492"],
    "teams": [],
    "apps": []
  },
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON
)

HTTP_STATUS=$(curl -sSL -o /tmp/branch_protection_res.json -w "%{http_code}" \
  -X PUT \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/branches/${BRANCH}/protection" \
  -d "${PROTECTION_PAYLOAD}")

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  echo "   ✅ Branch protection applied successfully (HTTP ${HTTP_STATUS})!"
  echo "      - Mandatory Status Checks: CI / Test & Race (1.25.x), CI / GolangCI-Lint, CI / Documentation & Version Gate, CI / Scale & Load Tests (-tags=scale)"
  echo "      - Enforce Admins:          true"
  echo "      - Require PR Reviews:      1 approval (stale review dismissal + Code Owners)"
  echo "      - Direct Push Access:      Restricted to repository collaborators only"
  echo "      - Linear History:          Enforced"
  echo "      - Force Pushes:            Blocked"
else
  echo "   ❌ Error applying branch protection (HTTP ${HTTP_STATUS}):"
  cat /tmp/branch_protection_res.json || true
  exit 1
fi

echo ""
echo "✨ Branch protection and repository policies are active for ${REPO_OWNER}/${REPO_NAME}:${BRANCH}."
