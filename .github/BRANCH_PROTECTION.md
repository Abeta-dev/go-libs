# Branch Protection & Community Access Policy

To maintain production stability, zero-regression guarantees, and security compliance for `github.com/umesh0492/go-libs`, the `main` branch is protected by automated GitHub branch protection rules, paired with open community access policies.

---

## 🛡️ Required Protection Settings for `main`

### 1. Require Pull Request Before Merging
- **Require approvals**: `1` (minimum 1 peer/maintainer approval required)
- **Dismiss stale pull request approvals when new commits are pushed**: `Enabled`
- **Require review from Code Owners**: `Enabled` (matches [`.github/CODEOWNERS`](./CODEOWNERS))
- **Require approval of the most recent reviewable push**: `Enabled`

### 2. Mandatory Status Checks
- **Require branches to be up to date before merging**: `Enabled` (Strict rebase/merge)
- **Required Status Checks**:
  1. `CI / Test & Race (1.25.x)` (unit tests with `-race`, coverage floor >= 85%, `ginmw` == 100%)
  2. `CI / GolangCI-Lint` (static analysis, formatting, complexity)
  3. `CI / Documentation & Version Gate` (strict synchronization of versions, package counts, leaf-import boundaries)
  4. `CI / Scale & Load Tests (-tags=scale)` (1000 concurrent connection load tests)

### 3. Merging & Access Constraints
- **Restrict direct push access**: Direct pushes to `main` are disabled; changes must arrive via approved Pull Requests from repository contributors or forks.
- **Restrict branch creation**: Direct branch creation on the primary repository is restricted to authorized repository contributors/collaborators.
- **Do not allow bypassing the above settings**: `Enabled` (`enforce_admins: true` — strictly enforced for Administrators).
- **Require conversation resolution before merging**: `Enabled`
- **Require linear history**: `Enabled` (Squash or Rebase merges only; merge commits disabled)
- **Allow force pushes**: `Disabled`
- **Allow deletions**: `Disabled`

---

## 🌍 Public Community Access Model

`go-libs` is an open-source project welcoming global contributions:

1. **Issues**: Open for all community members to report bugs, propose feature requests, or discuss architectural enhancements.
2. **Discussions**: Open for community questions, ideas, and architecture debates.
3. **Fork & Pull Request Workflow**:
   - Community contributors fork `https://github.com/umesh0492/go-libs`.
   - Submit Pull Requests against `main`.
   - Automated CI runs verification on PRs.
   - Maintainers review and merge once all 4 mandatory status checks pass.

---

## 🚀 Automated Setup

Repository administrators can apply these exact rules automatically using the dedicated script:

```bash
export GITHUB_TOKEN="ghp_yourAdminTokenHere"
./scripts/setup_branch_protection.sh
```

See [**`docs/BRANCH_PROTECTION_CONFIG.md`**](../docs/BRANCH_PROTECTION_CONFIG.md) for full REST API details.
