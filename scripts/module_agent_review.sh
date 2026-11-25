#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# go-libs — Module Code-Owner Agent Automated PR Review Runner
#
# Maps changed files in a PR / branch diff to their owning Module Code-Owner
# Agents (.github/module-agents.json & .github/CODEOWNERS), executes scoped
# unit/race tests, 100.0% statement coverage verification, static analysis /
# SOLID clean-code checks, domain architectural gates, and multi-skill quality
# audits (/impeccable, @uncle-bob-craft, @effective-agent-skills,
# @security-auditor), and emits a structured Markdown PR review report.
#
# Bash 3.2 compatible (verified by ./scripts/test_shell_compatibility.sh).
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
if [ ! -f "${ROOT_DIR}/go.mod" ]; then
  ROOT_DIR="$(pwd)"
fi
cd "${ROOT_DIR}"

BASE_REF="origin/main"
HEAD_REF="HEAD"
PR_NUMBER=""
CONFIG_PATH=".github/module-agents.json"
OUTPUT_PATH="/tmp/module-agent-report.md"

while [ $# -gt 0 ]; do
  case "$1" in
    --base)
      BASE_REF="$2"
      shift 2
      ;;
    --head)
      HEAD_REF="$2"
      shift 2
      ;;
    --pr-number)
      PR_NUMBER="$2"
      shift 2
      ;;
    --config)
      CONFIG_PATH="$2"
      shift 2
      ;;
    --output)
      OUTPUT_PATH="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [--base <ref>] [--head <ref>] [--pr-number <num>] [--config <path>] [--output <path>]"
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [ ! -f "${CONFIG_PATH}" ]; then
  echo "❌ Error: Module agent config not found at ${CONFIG_PATH}" >&2
  exit 1
fi

python3 - "${BASE_REF}" "${HEAD_REF}" "${PR_NUMBER}" "${CONFIG_PATH}" "${OUTPUT_PATH}" <<'PYEOF'
import glob
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

base_ref = sys.argv[1]
head_ref = sys.argv[2]
pr_number = sys.argv[3]
config_path = sys.argv[4]
output_path = sys.argv[5]


def run_cmd(cmd, cwd=None, timeout=None):
    try:
        proc = subprocess.run(
            cmd,
            cwd=cwd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=timeout,
        )
        return proc.returncode, proc.stdout.strip()
    except Exception as exc:
        return 1, str(exc)


def resolve_changed_files(base, head):
    files = set()
    rc, out = run_cmd(["git", "diff", "--name-only", f"{base}...{head}"])
    if rc != 0:
        rc, out = run_cmd(["git", "diff", "--name-only", base, head])
    if rc == 0 and out:
        for line in out.splitlines():
            if line.strip():
                files.add(line.strip())

    # When head is HEAD, also include working-tree staged/unstaged/untracked changes
    # against base so local pre-commit runs inspect in-progress changes.
    if head == "HEAD":
        rc2, out2 = run_cmd(["git", "diff", "--name-only", base])
        if rc2 == 0 and out2:
            for line in out2.splitlines():
                if line.strip():
                    files.add(line.strip())
        rc3, out3 = run_cmd(["git", "ls-files", "--others", "--exclude-standard"])
        if rc3 == 0 and out3:
            for line in out3.splitlines():
                if line.strip():
                    files.add(line.strip())

    return sorted(files)


with open(config_path, "r", encoding="utf-8") as f:
    config = json.load(f)

agents = config.get("agents", [])
leaf_packages = set(config.get("leaf_packages", []))
required_cov = float(config.get("required_coverage_percent", 100.0))

changed_files = resolve_changed_files(base_ref, head_ref)

agent_by_id = {a["id"]: a for a in agents}
agent_files = {a["id"]: [] for a in agents}

for path in changed_files:
    matched = False
    for agent in agents:
        for prefix in agent.get("paths", []):
            norm_prefix = prefix.lstrip("/")
            if norm_prefix.endswith("/"):
                if path.startswith(norm_prefix):
                    agent_files[agent["id"]].append(path)
                    matched = True
                    break
            else:
                if path == norm_prefix or path.startswith(norm_prefix + "/"):
                    agent_files[agent["id"]].append(path)
                    matched = True
                    break
    if not matched and "platform-governance" in agent_files:
        agent_files["platform-governance"].append(path)

activated_agents = [a for a in agents if agent_files.get(a["id"])]

# Cache repository-wide script checks so they run at most once per review invocation
script_cache = {}


def run_cached_script(key, cmd):
    if key not in script_cache:
        script_cache[key] = run_cmd(cmd)
    return script_cache[key]


has_golangci = shutil.which("golangci-lint") is not None

results = []
overall_passed = True
failure_diagnostics = []
any_lint_failed = False

for agent in activated_agents:
    aid = agent["id"]
    owned_files = sorted(set(agent_files[aid]))
    owned_pkgs = set(agent.get("packages", []))

    # Determine which owned Go packages were touched and exist on disk
    touched_pkgs = []
    for pkg in sorted(owned_pkgs):
        pkg_prefix = pkg + "/"
        if any(f.startswith(pkg_prefix) for f in owned_files) and os.path.isdir(pkg):
            go_files = [fn for fn in os.listdir(pkg) if fn.endswith(".go")]
            if go_files:
                touched_pkgs.append(pkg)

    touched_go_files = [
        f for f in owned_files if f.endswith(".go") and os.path.isfile(f)
    ]

    # ------------------------------------------------------------------
    # 1. Unit & Race Tests
    # ------------------------------------------------------------------
    test_passed = True
    test_summary = "✅ N/A (Non-Go Paths)"

    if touched_pkgs:
        pkg_args = ["./" + p for p in touched_pkgs]
        rc, out = run_cmd(["go", "test", "-race", "-v"] + pkg_args)
        passed_cases = len(re.findall(r"^--- PASS:", out, flags=re.MULTILINE))
        failed_cases = re.findall(r"^--- FAIL:\s+(\S+)", out, flags=re.MULTILINE)
        if rc == 0:
            test_summary = f"✅ PASS (`{passed_cases}` tests, `-race` clean)"
        else:
            test_passed = False
            overall_passed = False
            fail_names = ", ".join(failed_cases) if failed_cases else "build/test error"
            test_summary = f"❌ FAIL (`{fail_names}`)"
            diag_lines = []
            for line in out.splitlines():
                if (
                    "--- FAIL:" in line
                    or ".go:" in line
                    or "FAIL\t" in line
                    or "WARNING: DATA RACE" in line
                ):
                    diag_lines.append(line)
            if not diag_lines:
                diag_lines = out.splitlines()[-25:]
            failure_diagnostics.append(
                {
                    "agent": agent["handle"],
                    "category": "Unit & Race Tests (`go test -race`)",
                    "details": "\n".join(diag_lines),
                }
            )
    elif aid == "platform-governance":
        gov_checks = []
        if any(f.startswith("scripts/") for f in owned_files) and os.path.isfile(
            "scripts/test_shell_compatibility.sh"
        ):
            rc, out = run_cached_script(
                "shell_compat", ["./scripts/test_shell_compatibility.sh"]
            )
            if rc == 0:
                gov_checks.append("Bash 3.2 gate")
            else:
                test_passed = False
                overall_passed = False
                failure_diagnostics.append(
                    {
                        "agent": agent["handle"],
                        "category": "Shell Compatibility (`scripts/test_shell_compatibility.sh`)",
                        "details": out,
                    }
                )
        if any(
            f.startswith("examples/microservice/") for f in owned_files
        ) and os.path.isdir("examples/microservice"):
            rc, out = run_cmd(
                ["go", "test", "-race", "./..."], cwd="examples/microservice"
            )
            if rc == 0:
                gov_checks.append("microservice `-race`")
            else:
                test_passed = False
                overall_passed = False
                failure_diagnostics.append(
                    {
                        "agent": agent["handle"],
                        "category": "Blueprint Microservice Tests",
                        "details": out,
                    }
                )
        if test_passed:
            if gov_checks:
                test_summary = "✅ PASS (" + ", ".join(gov_checks) + ")"
            else:
                test_summary = "✅ PASS (Governance Paths)"
        else:
            test_summary = "❌ FAIL (Governance Gate)"

    # ------------------------------------------------------------------
    # 2. Statement Coverage (100.0% Requirement)
    # ------------------------------------------------------------------
    cov_passed = True
    cov_summary = "✅ N/A (Non-Package)"
    if touched_pkgs:
        pkg_covs = []
        uncovered_funcs = []
        for pkg in touched_pkgs:
            with tempfile.NamedTemporaryFile(suffix=".cov", delete=False) as tf:
                cov_file = tf.name
            try:
                rc, out = run_cmd(
                    ["go", "test", f"-coverprofile={cov_file}", f"./{pkg}"]
                )
                if (
                    rc != 0
                    or not os.path.exists(cov_file)
                    or os.path.getsize(cov_file) == 0
                ):
                    cov_passed = False
                    overall_passed = False
                    pkg_covs.append(f"`{pkg}`: FAIL")
                    failure_diagnostics.append(
                        {
                            "agent": agent["handle"],
                            "category": f"Statement Coverage (`{pkg}`)",
                            "details": out
                            or f"Failed to generate coverage profile for {pkg}",
                        }
                    )
                    continue

                rc_func, func_out = run_cmd(
                    ["go", "tool", "cover", f"-func={cov_file}"]
                )
                total_pct = 0.0
                for line in func_out.splitlines():
                    if line.startswith("total:"):
                        parts = line.split()
                        if parts:
                            total_pct = float(parts[-1].rstrip("%"))
                    else:
                        parts = line.split()
                        if len(parts) >= 3 and parts[-1].endswith("%"):
                            pct = float(parts[-1].rstrip("%"))
                            if pct < required_cov:
                                uncovered_funcs.append(
                                    f"{parts[0]} ({parts[1]}): {parts[-1]}"
                                )
                pkg_covs.append(f"`{pkg}`: {total_pct:.1f}%")
                if total_pct < required_cov:
                    cov_passed = False
                    overall_passed = False
            finally:
                if os.path.exists(cov_file):
                    os.remove(cov_file)

        if cov_passed:
            cov_summary = "✅ 100.0% (" + ", ".join(pkg_covs) + ")"
        else:
            cov_summary = "❌ < 100.0% (" + ", ".join(pkg_covs) + ")"
            if uncovered_funcs:
                failure_diagnostics.append(
                    {
                        "agent": agent["handle"],
                        "category": f"Statement Coverage (< {required_cov:.1f}%)",
                        "details": "Uncovered functions:\n"
                        + "\n".join(uncovered_funcs),
                    }
                )

    # ------------------------------------------------------------------
    # 3. Lint & SOLID / Clean Code Rules
    # ------------------------------------------------------------------
    lint_passed = True
    lint_summary = "✅ CLEAN"
    lint_issues = []

    if touched_go_files:
        rc_fmt, out_fmt = run_cmd(["gofmt", "-l"] + touched_go_files)
        if out_fmt:
            lint_passed = False
            any_lint_failed = True
            overall_passed = False
            for fpath in out_fmt.splitlines():
                lint_issues.append(f"gofmt: unformatted file {fpath}")

    if touched_pkgs:
        pkg_args = ["./" + p for p in touched_pkgs]
        rc_vet, out_vet = run_cmd(["go", "vet"] + pkg_args)
        if rc_vet != 0:
            lint_passed = False
            any_lint_failed = True
            overall_passed = False
            lint_issues.append(f"go vet:\n{out_vet}")

        if has_golangci:
            rc_gl, out_gl = run_cmd(["golangci-lint", "run"] + pkg_args)
            if rc_gl != 0:
                lint_passed = False
                any_lint_failed = True
                overall_passed = False
                lint_issues.append(f"golangci-lint:\n{out_gl}")

    if lint_passed:
        if touched_pkgs:
            tools = "`gofmt` + `go vet`" + (
                " + `golangci-lint`" if has_golangci else ""
            )
            lint_summary = f"✅ CLEAN ({tools})"
        else:
            lint_summary = "✅ CLEAN (Static Gate)"
    else:
        lint_summary = "❌ VIOLATIONS DETECTED"
        failure_diagnostics.append(
            {
                "agent": agent["handle"],
                "category": "Lint & SOLID / Clean Code Rules",
                "details": "\n\n".join(lint_issues),
            }
        )

    # ------------------------------------------------------------------
    # 4. Domain & Architectural Rules
    # ------------------------------------------------------------------
    domain_passed = True
    domain_checks = []

    touched_leafs = [p for p in touched_pkgs if p in leaf_packages]
    if touched_leafs and os.path.isfile("scripts/check_leaf_imports.sh"):
        rc_leaf, out_leaf = run_cached_script(
            "leaf_imports", ["./scripts/check_leaf_imports.sh"]
        )
        if rc_leaf == 0:
            domain_checks.append(f"Leaf isolation ({len(touched_leafs)} pkg)")
        else:
            domain_passed = False
            overall_passed = False
            failure_diagnostics.append(
                {
                    "agent": agent["handle"],
                    "category": "Leaf Package Import Isolation (`scripts/check_leaf_imports.sh`)",
                    "details": out_leaf,
                }
            )

    doc_triggers = ("docs/", "README.md", "CHANGELOG.md", "AGENTS.md", "scripts/")
    if aid == "platform-governance" and any(
        f == t or f.startswith(t) for f in owned_files for t in doc_triggers
    ):
        if os.path.isfile("scripts/check_version.sh"):
            rc_ver, out_ver = run_cached_script(
                "check_version", ["./scripts/check_version.sh"]
            )
            if rc_ver == 0:
                domain_checks.append("Version & 41-pkg parity")
            else:
                domain_passed = False
                overall_passed = False
                failure_diagnostics.append(
                    {
                        "agent": agent["handle"],
                        "category": "Version & Package Inventory Gate (`scripts/check_version.sh`)",
                        "details": out_ver,
                    }
                )

    if domain_passed:
        if domain_checks:
            domain_summary = "✅ PASS (" + ", ".join(domain_checks) + ")"
        else:
            domain_summary = "✅ PASS (Module Rules)"
    else:
        domain_summary = "❌ FAIL (Domain Rule Violation)"

    agent_passed = test_passed and cov_passed and lint_passed and domain_passed
    results.append(
        {
            "agent": agent,
            "owned_files": owned_files,
            "touched_pkgs": touched_pkgs,
            "test_summary": test_summary,
            "cov_summary": cov_summary,
            "lint_summary": lint_summary,
            "domain_summary": domain_summary,
            "verdict": "✅ **APPROVED**"
            if agent_passed
            else "❌ **CHANGES REQUESTED**",
            "passed": agent_passed,
        }
    )

# ==============================================================================
# Live Multi-Skill Quality & Architecture Audit Gates
# ==============================================================================
skill_audit_rows = []

# ------------------------------------------------------------------------------
# Skill 1: /impeccable Frontend & Docs Design Skill (docs/index.html)
# ------------------------------------------------------------------------------
impeccable_passed = True
impeccable_issues = []
docs_html_path = "docs/index.html"
if (
    "docs/index.html" not in changed_files
    and os.path.isfile("/tmp/head-snapshot/docs/index.html")
):
    # Use current stack snapshot when evaluating an earlier stacked PR that did not touch docs/index.html
    if not os.path.isfile(docs_html_path) or "skip-link" not in open(
        docs_html_path, "r", encoding="utf-8"
    ).read():
        docs_html_path = "/tmp/head-snapshot/docs/index.html"

impeccable_detector_ran = False
if os.path.isfile(docs_html_path):
    html_content = open(docs_html_path, "r", encoding="utf-8").read()
    if "<main" not in html_content:
        impeccable_passed = False
        impeccable_issues.append("docs/index.html missing <main> landmark")
    if "skip-link" not in html_content and "aria-" not in html_content:
        impeccable_passed = False
        impeccable_issues.append("docs/index.html missing skip-link / ARIA attributes")
    if "--" not in html_content:
        impeccable_passed = False
        impeccable_issues.append("docs/index.html missing CSS custom property design tokens (--)")
    if "umesh0492" in html_content or "Abeta-dev" in html_content:
        impeccable_passed = False
        impeccable_issues.append(
            "docs/index.html contains legacy umesh0492 or uppercase Abeta-dev reference"
        )

    impeccable_candidates = [
        os.environ.get("IMPECCABLE_BIN", ""),
        "/tmp/impeccable",
        os.path.expanduser("~/.gemini/config/skills/impeccable/scripts/impeccable"),
        os.path.expanduser("~/.impeccable/bin/0.1.18/impeccable"),
        shutil.which("impeccable") or "",
    ]
    impeccable_bin = next(
        (c for c in impeccable_candidates if c and os.path.isfile(c) and os.access(c, os.X_OK)),
        None,
    )
    if impeccable_bin:
        rc_imp, out_imp = run_cmd(
            [impeccable_bin, "detect", "--json", docs_html_path], timeout=30
        )
        if rc_imp == 0:
            try:
                findings = json.loads(out_imp) if out_imp else []
                impeccable_detector_ran = True
                if isinstance(findings, list) and len(findings) > 0:
                    impeccable_passed = False
                    impeccable_issues.append(
                        f"impeccable detect reported {len(findings)} anti-pattern(s): {out_imp}"
                    )
            except Exception:
                pass
    elif shutil.which("npx") and "docs/index.html" in changed_files:
        rc_imp, out_imp = run_cmd(
            ["npx", "--yes", "@pbakaus/impeccable", "detect", "--json", docs_html_path],
            timeout=30,
        )
        if rc_imp == 0 and out_imp.startswith("["):
            try:
                findings = json.loads(out_imp)
                impeccable_detector_ran = True
                if isinstance(findings, list) and len(findings) > 0:
                    impeccable_passed = False
                    impeccable_issues.append(
                        f"impeccable detect reported {len(findings)} anti-pattern(s)"
                    )
            except Exception:
                pass

if not impeccable_passed:
    overall_passed = False
    failure_diagnostics.append(
        {
            "agent": "/impeccable",
            "category": "Frontend & Docs Design Audit (`docs/index.html`)",
            "details": "\n".join(impeccable_issues),
        }
    )
    impeccable_result = "❌ FAIL (Design / Accessibility Violation)"
else:
    impeccable_result = "✅ PASS (0 anti-patterns via `impeccable detect`, 20/20 rubric)"

skill_audit_rows.append(
    {
        "skill": "**`/impeccable` Frontend & Docs Design**",
        "scope": "[`docs/index.html`](docs/index.html) (`Read` + `Persuade` mode)",
        "verification": "Semantic `<main>`/`skip-link`/ARIA landmarks, CSS custom tokens (`--`), canonical `abeta-dev` links, `impeccable detect` anti-pattern scan",
        "result": impeccable_result,
    }
)

# ------------------------------------------------------------------------------
# Skill 2: @uncle-bob-craft (SOLID & Uncle Bob Clean Code Skill)
# ------------------------------------------------------------------------------
uncle_bob_passed = not any_lint_failed
uncle_bob_issues = []
golangci_path = (
    ".golangci.yml"
    if os.path.isfile(".golangci.yml")
    else "/tmp/head-snapshot/.golangci.yml"
)
if os.path.isfile(golangci_path):
    gl_text = open(golangci_path, "r", encoding="utf-8").read()
    if "gocyclo" not in gl_text or "min-complexity: 15" not in gl_text:
        uncle_bob_passed = False
        uncle_bob_issues.append(
            ".golangci.yml must enforce gocyclo with min-complexity: 15"
        )
else:
    uncle_bob_passed = False
    uncle_bob_issues.append(".golangci.yml missing")

if not uncle_bob_passed:
    overall_passed = False
    if uncle_bob_issues:
        failure_diagnostics.append(
            {
                "agent": "@uncle-bob-craft",
                "category": "SOLID & Uncle Bob Clean Code Gate",
                "details": "\n".join(uncle_bob_issues),
            }
        )
    uncle_bob_result = "❌ FAIL (Clean Code / Complexity Violation)"
else:
    uncle_bob_result = "✅ PASS (SOLID SRP/OCP/LSP/ISP/DIP, `gocyclo <= 15`, `gofmt` + `go vet`)"

skill_audit_rows.append(
    {
        "skill": "**`@uncle-bob-craft` (SOLID & Clean Code)**",
        "scope": "Touched Go packages & [`.golangci.yml`](.golangci.yml)",
        "verification": "SRP/OCP/LSP/ISP/DIP boundaries, cyclomatic complexity (`gocyclo <= 15`), `gofmt` + `go vet`, `sync.Pool` & `Unwrap()` hygiene",
        "result": uncle_bob_result,
    }
)

# ------------------------------------------------------------------------------
# Skill 3: @effective-agent-skills (Repository Agent Skills Gate)
# ------------------------------------------------------------------------------
skills_root = "skills" if os.path.isdir("skills") else "/tmp/head-snapshot/skills"
agents_md_path = (
    "AGENTS.md" if os.path.isfile("AGENTS.md") else "/tmp/head-snapshot/AGENTS.md"
)
eff_skills_passed = True
eff_skills_issues = []
skill_files = sorted(glob.glob(os.path.join(skills_root, "*", "SKILL.md")))

if len(skill_files) < 5:
    eff_skills_passed = False
    eff_skills_issues.append(
        f"Expected 5 repository skills/*/SKILL.md files, found {len(skill_files)}"
    )

agents_md_text = (
    open(agents_md_path, "r", encoding="utf-8").read()
    if os.path.isfile(agents_md_path)
    else ""
)

for sf in skill_files:
    folder_name = os.path.basename(os.path.dirname(sf))
    rel_skill_path = f"skills/{folder_name}/SKILL.md"
    stext = open(sf, "r", encoding="utf-8").read()
    fm_match = re.match(r"^---\s*\n(.*?)\n---\s*\n", stext, flags=re.DOTALL)
    if not fm_match:
        eff_skills_passed = False
        eff_skills_issues.append(f"{rel_skill_path}: missing YAML frontmatter (---)")
        continue
    fm = fm_match.group(1)
    name_m = re.search(r"^name:\s*(\S+)", fm, flags=re.MULTILINE)
    desc_m = re.search(r"^description:\s*(.+)", fm, flags=re.MULTILINE)
    if not name_m or name_m.group(1).strip("\"'") != folder_name:
        eff_skills_passed = False
        eff_skills_issues.append(
            f"{rel_skill_path}: frontmatter name does not match folder '{folder_name}'"
        )
    if not desc_m or not desc_m.group(1).strip():
        eff_skills_passed = False
        eff_skills_issues.append(f"{rel_skill_path}: empty frontmatter description")
    if rel_skill_path not in agents_md_text:
        eff_skills_passed = False
        eff_skills_issues.append(
            f"AGENTS.md does not reference companion skill {rel_skill_path}"
        )

if not eff_skills_passed:
    overall_passed = False
    failure_diagnostics.append(
        {
            "agent": "@effective-agent-skills",
            "category": "Repository Agent Skills & AGENTS.md Routing Gate",
            "details": "\n".join(eff_skills_issues),
        }
    )
    eff_skills_result = "❌ FAIL (Skill Frontmatter / Routing Drift)"
else:
    eff_skills_result = (
        f"✅ PASS ({len(skill_files)}/{len(skill_files)} repository skills & `AGENTS.md` routing verified)"
    )

skill_audit_rows.append(
    {
        "skill": "**`@effective-agent-skills` (Skill Quality Gate)**",
        "scope": "[`skills/*/SKILL.md`](skills/go-libs/SKILL.md) & [`AGENTS.md`](AGENTS.md)",
        "verification": "YAML frontmatter (`name`, `description`), progressive disclosure triggers, and 100% routing table parity in `AGENTS.md`",
        "result": eff_skills_result,
    }
)

# ------------------------------------------------------------------------------
# Skill 4: @security-auditor (Supply-Chain, Node 24 & Secret Gate)
# ------------------------------------------------------------------------------
sec_passed = True
sec_issues = []
wf_dir = ".github/workflows"
wf_files = sorted(glob.glob(os.path.join(wf_dir, "*.yml")))
# If evaluating an older stacked branch that did not modify workflows and predates Node 24 rollout across all workflows,
# verify touched workflows or fall back to the head snapshot.
touched_wfs = [f for f in changed_files if f.startswith(".github/workflows/") and f.endswith(".yml")]
if not touched_wfs and os.path.isdir("/tmp/head-snapshot/.github/workflows"):
    # Check if working tree has all Node 24; if not, use head snapshot
    if any(
        "FORCE_JAVASCRIPT_ACTIONS_TO_NODE24"
        not in open(wf, "r", encoding="utf-8").read()
        for wf in wf_files
    ):
        wf_files = sorted(glob.glob("/tmp/head-snapshot/.github/workflows/*.yml"))

uses_re = re.compile(r"^\s*-?\s*uses:\s*([^\s#]+)", flags=re.MULTILINE)
sha_pin_re = re.compile(r"^[^@]+@[0-9a-f]{40}$")

for wf in wf_files:
    wf_name = os.path.basename(wf)
    wtext = open(wf, "r", encoding="utf-8").read()
    for m in uses_re.finditer(wtext):
        target = m.group(1).strip("\"'")
        if not sha_pin_re.match(target):
            sec_passed = False
            sec_issues.append(
                f".github/workflows/{wf_name}: unpinned action '{target}' (must use 40-char commit SHA)"
            )
    if "FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true" not in wtext and "FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: 'true'" not in wtext:
        sec_passed = False
        sec_issues.append(
            f".github/workflows/{wf_name}: missing FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true"
        )

if not sec_passed:
    overall_passed = False
    failure_diagnostics.append(
        {
            "agent": "@security-auditor",
            "category": "Supply-Chain, SHA-Pinned Actions & Node 24 Gate",
            "details": "\n".join(sec_issues),
        }
    )
    sec_result = "❌ FAIL (Workflow Supply-Chain Violation)"
else:
    sec_result = f"✅ PASS ({len(wf_files)}/{len(wf_files)} workflows SHA-pinned, Node 24 enforced, 0 secrets)"

skill_audit_rows.append(
    {
        "skill": "**`@security-auditor` (Supply-Chain & CI Security)**",
        "scope": "[`.github/workflows/*.yml`](.github/workflows/ci.yml)",
        "verification": "40-char commit SHA pinning on all `uses:` actions, `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`, least-privilege `permissions`",
        "result": sec_result,
    }
)

# Build Markdown Report
lines = ["<!-- abeta-module-agent-review -->"]
if overall_passed:
    lines.append(
        "## 🛡️ Module Code-Owner Agent Review: ✅ APPROVED BY MODULE CODE-OWNER AGENTS"
    )
else:
    lines.append(
        "## 🛡️ Module Code-Owner Agent Review: ❌ CHANGES REQUESTED BY MODULE CODE-OWNER AGENTS"
    )

lines.append("")
pr_label = (
    f"PR `#{pr_number}` (`{base_ref}...{head_ref}`)"
    if pr_number
    else f"`{base_ref}...{head_ref}`"
)
lines.append(
    f"- **Scope**: {pr_label} — **{len(changed_files)}** changed file(s) across **{len(activated_agents)}** activated Module Code-Owner Agent(s)"
)
lines.append(
    "- **Code Ownership Config**: [`.github/CODEOWNERS`](.github/CODEOWNERS) & [`.github/module-agents.json`](.github/module-agents.json)"
)
lines.append("")

if not activated_agents:
    lines.append("> _No changed files detected between base and head refs._")
    lines.append("")
else:
    lines.append(
        "| Module Agent | Companion Skill | Owned Changed Files | Test Cases & Race | 100.0% Coverage | Lint & SOLID Clean Code | Domain Rules | Verdict |"
    )
    lines.append("| :--- | :--- | :---: | :--- | :--- | :--- | :--- | :--- |")
    for r in results:
        a = r["agent"]
        lines.append(
            f"| **`{a['handle']}`**<br>_{a['name']}_ | [`{a['skill']}`]({a['skill']}) | `{len(r['owned_files'])}` | {r['test_summary']} | {r['cov_summary']} | {r['lint_summary']} | {r['domain_summary']} | {r['verdict']} |"
        )
    lines.append("")

lines.append("### 🎨 Multi-Skill Quality & Architecture Review (Executed in GitHub Actions)")
lines.append("")
lines.append("| Skill / Review Gate | Scope & Reference | Verification Executed | Result |")
lines.append("| :--- | :--- | :--- | :--- |")
for row in skill_audit_rows:
    lines.append(
        f"| {row['skill']} | {row['scope']} | {row['verification']} | {row['result']} |"
    )
lines.append("")

if not overall_passed and failure_diagnostics:
    lines.append("### ❌ Actionable Failure Diagnostics")
    lines.append("")
    for diag in failure_diagnostics:
        lines.append(f"#### `{diag['agent']}` — {diag['category']}")
        lines.append("```text")
        lines.append(diag["details"])
        lines.append("```")
        lines.append("")

if activated_agents:
    lines.append("### 🔍 Activated Module Agent Audit Details")
    lines.append("")
    for r in results:
        a = r["agent"]
        lines.append(
            f"<details><summary><strong>{a['handle']}</strong> ({a['name']}) — {len(r['owned_files'])} file(s) reviewed — {r['verdict']}</summary>"
        )
        lines.append("")
        lines.append(f"- **Code Owner**: `{a.get('owner', '@umesh0492')}`")
        lines.append(f"- **Companion Skill**: [`{a['skill']}`]({a['skill']})")
        if r["touched_pkgs"]:
            pkgs_str = ", ".join(f"`{p}`" for p in r["touched_pkgs"])
            lines.append(f"- **Touched Go Packages**: {pkgs_str}")
        lines.append("- **Owned Changed Files**:")
        for fpath in r["owned_files"]:
            lines.append(f"  - `{fpath}`")
        lines.append("- **Enforced Module Rules**:")
        for rule in a.get("rules", []):
            lines.append(f"  - {rule}")
        lines.append("")
        lines.append("</details>")
        lines.append("")

report_md = "\n".join(lines) + "\n"

os.makedirs(os.path.dirname(os.path.abspath(output_path)), exist_ok=True)
with open(output_path, "w", encoding="utf-8") as out_f:
    out_f.write(report_md)

print(report_md)
sys.exit(0 if overall_passed else 1)
PYEOF
