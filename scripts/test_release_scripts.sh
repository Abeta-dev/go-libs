#!/usr/bin/env bash
set -euo pipefail

# Exercises release-script behavior without altering tags in the working checkout.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
command -v git >/dev/null || { echo 'git must be available on PATH' >&2; exit 1; }
command -v go >/dev/null || { echo 'go must be available on PATH' >&2; exit 1; }

fixture_dir="$(mktemp -d)"
trap 'rm -rf "${fixture_dir}"' EXIT

git clone --quiet --no-local "${ROOT_DIR}" "${fixture_dir}/tagged-checkout"
# Exercise the current gate against an isolated checkout; uncommitted review fixes
# deliberately remain in the source checkout until they are validated.
cp "${ROOT_DIR}/scripts/check_version.sh" "${fixture_dir}/tagged-checkout/scripts/check_version.sh"
chmod +x "${fixture_dir}/tagged-checkout/scripts/check_version.sh"
cd "${fixture_dir}/tagged-checkout"
# The fixture has its own refs. This does not move, delete, or recreate tags in
# the source checkout, including the known conflicting local v0.2.1 tag.
git tag -d v0.2.1 >/dev/null 2>&1 || true
git -c user.name='go-libs release fixture' \
  -c user.email='release-fixture@invalid.example' \
  tag -a v0.2.1 -m 'tagged-checkout fixture' HEAD

GIT_TAG=v0.2.1 ./scripts/check_version.sh >/dev/null

# A successful baseline audit must remain successful after Go creates read-only
# files in its temporary module cache. The baseline script uses the source
# checkout's configured remote, so run it there rather than in the fixture clone.
cd "${ROOT_DIR}"
./scripts/verify_release_baseline.sh >/dev/null

cd "${fixture_dir}/tagged-checkout"
# A populated Unreleased section must take precedence over the latest release.
python3 - <<'PY'
from pathlib import Path
path = Path("CHANGELOG.md")
text = path.read_text()
path.write_text(text.replace("## [0.2.1]", "## [Unreleased]\n\n### Added\n- logger: `WithSampler` remains available.\n\n## [0.2.1]", 1))
PY
GIT_TAG=v0.2.1 ./scripts/check_version.sh >/dev/null

if GIT_TAG=v0.2.1-not-semver ./scripts/check_version.sh >/dev/null 2>&1; then
  echo 'check_version.sh accepted an invalid tagged-checkout version' >&2
  exit 1
fi

cd "${ROOT_DIR}"

# Coverage generation must fail closed instead of accepting a stale profile.
coverage_fixture="${fixture_dir}/coverage-fixture"
mkdir -p "${coverage_fixture}/bin"
cp "${ROOT_DIR}/scripts/check_coverage.sh" "${coverage_fixture}/check_coverage.sh"
sed -i.bak '/^"${SCRIPT_DIR}\/check_version.sh"$/d' "${coverage_fixture}/check_coverage.sh"
rm -f "${coverage_fixture}/check_coverage.sh.bak"
printf 'mode: set\nstale.go:1.1,1.2 1 1\n' >"${coverage_fixture}/coverage.out"
cat >"${coverage_fixture}/bin/go" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then
  echo 'example.invalid/package'
  exit 0
fi
if [[ "$1" == "test" && "$2" == -coverprofile=* ]]; then
  exit 1
fi
exit 1
EOF
chmod +x "${coverage_fixture}/bin/go"
if (cd "${coverage_fixture}" && PATH="${coverage_fixture}/bin:${PATH}" ./check_coverage.sh >/dev/null 2>&1); then
  echo 'check_coverage.sh accepted a failed coverage run and stale profile' >&2
  exit 1
fi

rm -f release-manifest.json
if ./scripts/verify_release.sh invalid >/dev/null 2>&1; then
  echo 'verify_release.sh accepted an invalid release tag' >&2
  exit 1
fi
test -s release-manifest.json
python3 -m json.tool release-manifest.json >/dev/null
rm -f release-manifest.json

echo 'release script fixtures passed'
