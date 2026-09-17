#!/usr/bin/env bash
set -euo pipefail

# Verifies immutable upstream release identity without trusting local tags.
# The local v0.2.1 ref may be stale or conflicting; this script never modifies it.

readonly MODULE='github.com/umesh0492/go-libs'
readonly VERSION='v0.2.1'
readonly REMOTE='origin'
readonly EXPECTED_TAG_OBJECT='6df2ebb2d229dd82b2f95a6c9cc5a6b3b4542b43'
readonly EXPECTED_COMMIT='b75acc6d82e47189ef174a4ea80134fed8cd392f'
readonly EXPECTED_ZIP_SHA256='d519b6624138503f1cbe8f3de071f0fe685b521ad5f1f9634a0a85a1992fc9db'
readonly EXPECTED_MOD_SHA256='3625f2188bcc9a45a7bbf32241641dd317890d5a44c0bd32748cf12d6eb4ac47'
readonly EXPECTED_SUM='h1:9Fzm2GZMkd+rnFAFOd5MURnOorqM98O4H/zcVUb6uoY='
readonly EXPECTED_GOMOD_SUM='h1:R7gQaadUNwpnavd5P96ThNbhYyUUKyVbfKCR/mu29/o='

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

command -v go >/dev/null || {
  echo 'go must be available on PATH' >&2
  exit 1
}
command -v curl >/dev/null || {
  echo 'curl must be available on PATH' >&2
  exit 1
}

sha256() {
  sha256sum "$1" | awk '{print $1}'
}

assert_equals() {
  local label="$1"
  local want="$2"
  local got="$3"

  if [[ "${got}" != "${want}" ]]; then
    printf 'ERROR: %s mismatch\n  expected: %s\n  got:      %s\n' "${label}" "${want}" "${got}" >&2
    exit 1
  fi
  printf 'OK: %s: %s\n' "${label}" "${got}"
}

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

remote_tag_object="$(git ls-remote --tags --refs "${REMOTE}" "refs/tags/${VERSION}" | awk 'NR == 1 { print $1 }')"
remote_commit="$(git ls-remote "${REMOTE}" "refs/tags/${VERSION}^{}" | awk 'NR == 1 { print $1 }')"
assert_equals 'remote tag object' "${EXPECTED_TAG_OBJECT}" "${remote_tag_object}"
assert_equals 'remote tagged commit' "${EXPECTED_COMMIT}" "${remote_commit}"

proxy_base="https://proxy.golang.org/${MODULE}/@v/${VERSION}"
curl --fail --silent --show-error --location "${proxy_base}.zip" -o "${tmp_dir}/module.zip"
curl --fail --silent --show-error --location "${proxy_base}.mod" -o "${tmp_dir}/module.mod"
assert_equals 'proxy zip SHA-256' "${EXPECTED_ZIP_SHA256}" "$(sha256 "${tmp_dir}/module.zip")"
assert_equals 'proxy module file SHA-256' "${EXPECTED_MOD_SHA256}" "$(sha256 "${tmp_dir}/module.mod")"

module_json="$(GOWORK=off go mod download -json "${MODULE}@${VERSION}")"
resolved_sum="$(printf '%s\n' "${module_json}" | sed -n 's/^[[:space:]]*"Sum": "\([^"]*\)".*/\1/p')"
resolved_gomod_sum="$(printf '%s\n' "${module_json}" | sed -n 's/^[[:space:]]*"GoModSum": "\([^"]*\)".*/\1/p')"
assert_equals 'Go module checksum' "${EXPECTED_SUM}" "${resolved_sum}"
assert_equals 'Go module file checksum' "${EXPECTED_GOMOD_SUM}" "${resolved_gomod_sum}"

if git rev-parse --verify --quiet "refs/tags/${VERSION}" >/dev/null; then
  local_tag_object="$(git rev-parse "refs/tags/${VERSION}")"
  local_tag_commit="$(git rev-parse "${VERSION}^{}")"
  if [[ "${local_tag_object}" == "${EXPECTED_TAG_OBJECT}" && "${local_tag_commit}" == "${EXPECTED_COMMIT}" ]]; then
    printf 'OK: local %s matches the published artifact\n' "${VERSION}"
  else
    printf 'NOTICE: local %s differs from the published artifact; it was not modified.\n' "${VERSION}"
  fi
fi

printf 'Release baseline verification passed for %s@%s.\n' "${MODULE}" "${VERSION}"
