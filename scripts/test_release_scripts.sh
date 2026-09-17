#!/usr/bin/env bash
set -euo pipefail

# Exercises release-script behavior without altering tags in the working checkout.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
command -v git >/dev/null || { echo 'git must be available on PATH' >&2; exit 1; }
command -v go >/dev/null || { echo 'go must be available on PATH' >&2; exit 1; }

fixture_dir="$(mktemp -d)"
trap 'chmod -R u+w "${fixture_dir}" 2>/dev/null || true; rm -rf "${fixture_dir}"' EXIT

git clone --quiet --no-local "${ROOT_DIR}" "${fixture_dir}/tagged-checkout"
# Exercise the current gate against an isolated checkout; uncommitted review fixes
# deliberately remain in the source checkout until they are validated.
cp "${ROOT_DIR}/scripts/check_version.sh" "${fixture_dir}/tagged-checkout/scripts/check_version.sh"
chmod +x "${fixture_dir}/tagged-checkout/scripts/check_version.sh"
cd "${fixture_dir}/tagged-checkout"
# The fixture has its own refs. This does not move, delete, or recreate tags in
# the source checkout, including the known conflicting local v0.2.1 tag.
TARGET_TAG="v$(grep -E 'go get github.com/umesh0492/go-libs@v' "${ROOT_DIR}/README.md" | head -n1 | sed -E 's/.*go-libs@v([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?).*/\1/')"
git tag -d "${TARGET_TAG}" >/dev/null 2>&1 || true
git -c user.name='go-libs release fixture' \
  -c user.email='release-fixture@invalid.example' \
  tag -a "${TARGET_TAG}" -m 'tagged-checkout fixture' HEAD

GIT_TAG="${TARGET_TAG}" ./scripts/check_version.sh >/dev/null

# A successful baseline audit must remain successful after Go creates read-only
# files in its temporary module cache. The baseline script uses the source
# checkout's configured remote, so run it there rather than in the fixture clone.
cd "${ROOT_DIR}"
./scripts/verify_release_baseline.sh >/dev/null

cd "${fixture_dir}/tagged-checkout"
# A populated Unreleased section must take precedence over the latest release.
python3 - "${TARGET_TAG#v}" <<'PY'
import sys
from pathlib import Path
target_ver = sys.argv[1]
path = Path("CHANGELOG.md")
text = path.read_text()
path.write_text(text.replace(f"## [{target_ver}]", f"## [Unreleased]\n\n### Added\n- logger: `WithSampler` remains available.\n\n## [{target_ver}]", 1))
PY
GIT_TAG="${TARGET_TAG}" ./scripts/check_version.sh >/dev/null

if GIT_TAG="${TARGET_TAG}-not-semver" ./scripts/check_version.sh >/dev/null 2>&1; then
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
