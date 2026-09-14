# Branch Protection & Community Configuration for `main`

This document details the branch protection rules, community access policies, and automated configuration for the `main` branch of `github.com/umesh0492/go-libs`.

---

## Protection Policy Summary

To maintain truth-gate integrity and ensure no regression enters production, `main` enforces the following rules:

1. **Require Pull Request Reviews Before Merging**:
   - Minimum required approvals: `1`
   - Dismiss stale pull request approvals when new commits are pushed: `true`
   - Require review from Code Owners (`CODEOWNERS`): `true`
   - Require approval of the most recent reviewable push: `true`
2. **Require Status Checks to Pass Before Merging**:
   - Require branches to be up to date before merging: `true`
   - Required status checks (matching CI workflow context in `.github/workflows/ci.yml`):
     - `CI / Test & Race (1.25.x)`
     - `CI / GolangCI-Lint`
     - `CI / Documentation & Version Gate`
     - `CI / Scale & Load Tests (-tags=scale)`
3. **Branch Access & Contributor Restrictions**:
   - Direct push access to `main` is disabled; all updates must arrive via approved Pull Requests.
   - Branch creation on the origin repository is restricted to authorized repository contributors and collaborators.
4. **Require Conversation Resolution**:
   - All review comments and discussions must be resolved before merging: `true`
5. **Require Linear History**:
   - Merge commits are disabled in favor of squash or rebase merges: `true`
6. **Include Administrators**:
   - Enforcement applies to repository administrators (`enforce_admins: true`) to guarantee zero bypassing of truth gates.

---

## Public Community Access Model

As a public open-source project, `umesh0492/go-libs` provides:

- **Public Issues**: Enabled and open for all community members to file bug reports, feature requests, and security observations.
- **GitHub Discussions**: Enabled for architectural questions, ideas, and ecosystem discussion.
- **Fork-Based Contributions**: External contributors fork the repository and submit Pull Requests against `main`. All CI status checks run automatically on PRs.

---

## Automated Configuration via Helper Script

Maintainers can apply the full suite of branch protection and community access rules with a single command:

```bash
export GITHUB_TOKEN="ghp_yourAdminTokenHere"
./scripts/setup_branch_protection.sh
```

---

## Configuration via GitHub REST API (Ready-to-Run `curl`)

Repository administrators can also apply these settings directly using `curl` and a Personal Access Token (PAT) with `repo` scope:

### 1. Enable Issues, Discussions, and Forking Settings

```bash
export GITHUB_TOKEN="ghp_yourAdminTokenHere"
export REPO_OWNER="umesh0492"
export REPO_NAME="go-libs"

curl -L \
  -X PATCH \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${GITHUB_TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME} \
  -d '{
    "has_issues": true,
    "has_discussions": true,
    "allow_forking": true,
    "allow_squash_merge": true,
    "allow_merge_commit": false,
    "allow_rebase_merge": true,
    "delete_branch_on_merge": true
  }'
```

### 2. Apply Main Branch Protection

```bash
curl -L \
  -X PUT \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${GITHUB_TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/branches/main/protection \
  -d '{
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
      "users": [],
      "teams": [],
      "apps": []
    },
    "required_linear_history": true,
    "allow_force_pushes": false,
    "allow_deletions": false,
    "required_conversation_resolution": true
  }'
```

---

## Configuration via GitHub Web UI

1. Navigate to repository **Settings** -> **Branches**.
2. Click **Add branch ruleset** or **Add branch protection rule**.
3. Set **Branch name pattern** to `main`.
4. Enable:
   - **Require a pull request before merging**:
     - Require approvals: `1`
     - Dismiss stale pull request approvals when new commits are pushed
     - Require review from Code Owners
   - **Require status checks to pass before merging**:
     - Require branches to be up to date before merging
     - Search and select:
       * `CI / Test & Race (1.25.x)`
       * `CI / GolangCI-Lint`
       * `CI / Documentation & Version Gate`
       * `CI / Scale & Load Tests (-tags=scale)`
   - **Require conversation resolution before merging**
   - **Require linear history**
   - **Do not allow bypassing the above settings** (Enforce for administrators)
   - **Restrict who can push to matching branches**
5. Navigate to **Settings** -> **General** -> **Features**:
   - Check **Issues**
   - Check **Discussions**
6. Click **Save changes**.
