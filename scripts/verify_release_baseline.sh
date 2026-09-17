#!/usr/bin/env bash
set -euo pipefail

# Audits the historical v0.2.1 publication whose local tag conflicts with origin.
# This is migration evidence, not a future-release gate. Use verify_release.sh
# with the release tag for recurring release validation.

readonly MODULE='github.com/umesh0492/go-libs'
readonly VERSION='v0.2.1'
readonly REMOTE="${RELEASE_BASELINE_REMOTE:-${GIT_REMOTE:-origin}}"
readonly PROXY_URL="${GOPROXY_URL:-https://proxy.golang.org}"
readonly EXPECTED_TAG_OBJECT='6df2ebb2d229dd82b2f95a6c9cc5a6b3b4542b43'
readonly EXPECTED_COMMIT='b75acc6d82e47189ef174a4ea80134fed8cd392f'
readonly EXPECTED_ZIP_SHA256='d519b6624138503f1cbe8f3de071f0fe685b521ad5f1f9634a0a85a1992fc9db'
readonly EXPECTED_MOD_SHA256='3625f2188bcc9a45a7bbf32241641dd317890d5a44c0bd32748cf12d6eb4ac47'
readonly EXPECTED_SUM='h1:9Fzm2GZMkd+rnFAFOd5MURnOorqM98O4H/zcVUb6uoY='
readonly EXPECTED_GOMOD_SUM='h1:R7gQaadUNwpnavd5P96ThNbhYyUUKyVbfKCR/mu29/o='

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

for command in go git curl; do
  command -v "${command}" >/dev/null || {
    echo "${command} must be available on PATH" >&2
    exit 1
  }
done

sha256() {
  if command -v sha256sum >/dev/null; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo 'either sha256sum or shasum -a 256 must be available' >&2
    exit 1
  fi
}

fetch_with_retry() {
  local url="$1"
  local output="$2"
  local attempt=1
  local max_attempts=4
  local delay=1
  local curl_error_file="${tmp_dir}/curl-error"

  while (( attempt <= max_attempts )); do
    rm -f "${output}" "${curl_error_file}"
    if curl --fail --silent --show-error --location --retry 0 "${url}" -o "${output}" 2>"${curl_error_file}"; then
      return 0
    fi
    if (( attempt == max_attempts )); then
      echo "unable to fetch ${url} after ${max_attempts} attempts: $(tr '\n' ' ' < "${curl_error_file}")" >&2
      return 1
    fi
    echo "Retrying ${url} in ${delay}s (attempt ${attempt}/${max_attempts}): $(tr '\n' ' ' < "${curl_error_file}")" >&2
    sleep "${delay}"
    delay=$((delay * 2))
    attempt=$((attempt + 1))
  done
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
cleanup() {
  local exit_code=$?
  chmod -R u+w "${tmp_dir}" 2>/dev/null || true
  rm -rf "${tmp_dir}" 2>/dev/null || true
  trap - EXIT
  exit "${exit_code}"
}
trap cleanup EXIT

remote_tag_object="$(git ls-remote --tags --refs "${REMOTE}" "refs/tags/${VERSION}" | awk 'NR == 1 { print $1 }')"
remote_commit="$(git ls-remote "${REMOTE}" "refs/tags/${VERSION}^{}" | awk 'NR == 1 { print $1 }')"
assert_equals 'remote tag object' "${EXPECTED_TAG_OBJECT}" "${remote_tag_object}"
assert_equals 'remote tagged commit' "${EXPECTED_COMMIT}" "${remote_commit}"

proxy_base="${PROXY_URL%/}/${MODULE}/@v/${VERSION}"
fetch_with_retry "${proxy_base}.zip" "${tmp_dir}/module.zip"
fetch_with_retry "${proxy_base}.mod" "${tmp_dir}/module.mod"
assert_equals 'proxy zip SHA-256' "${EXPECTED_ZIP_SHA256}" "$(sha256 "${tmp_dir}/module.zip")"
assert_equals 'proxy module file SHA-256' "${EXPECTED_MOD_SHA256}" "$(sha256 "${tmp_dir}/module.mod")"

download_module_with_retry() {
  local attempt=1
  local max_attempts=4
  local delay=1
  local module_cache
  local download_error_file="${tmp_dir}/go-download-error"

  module_json=''
  while (( attempt <= max_attempts )); do
    module_cache="${tmp_dir}/gomodcache-${attempt}"
    rm -rf "${module_cache}" "${download_error_file}"
    if module_json="$(GOWORK=off GOMODCACHE="${module_cache}" GOPROXY="${PROXY_URL}" go mod download -json "${MODULE}@${VERSION}" 2>"${download_error_file}")"; then
      rm -rf "${module_cache}"
      return 0
    fi
    rm -rf "${module_cache}"
    if (( attempt == max_attempts )); then
      echo "Go proxy resolution failed after ${max_attempts} attempts: $(tr '\n' ' ' < "${download_error_file}")" >&2
      return 1
    fi
    echo "Retrying Go proxy resolution in ${delay}s (attempt ${attempt}/${max_attempts}): $(tr '\n' ' ' < "${download_error_file}")" >&2
    sleep "${delay}"
    delay=$((delay * 2))
    attempt=$((attempt + 1))
  done
}

if ! download_module_with_retry; then
  echo "Go could not resolve ${MODULE}@${VERSION}" >&2
  exit 1
fi
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

printf 'Historical release baseline audit passed for %s@%s.\n' "${MODULE}" "${VERSION}"
