#!/usr/bin/env bash
set -euo pipefail

# Verifies the immutable source and public Go-module artifact for a release tag.
# Unlike verify_release_baseline.sh, this is intentionally tag-parameterized and
# is safe to invoke from every future release workflow.

readonly MODULE='github.com/umesh0492/go-libs'
readonly DEFAULT_REMOTE='origin'
readonly DEFAULT_PROXY='https://proxy.golang.org'
readonly DEFAULT_MANIFEST='release-manifest.json'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

TAG="${1:-${GIT_TAG:-}}"
REMOTE="${RELEASE_REMOTE:-${GIT_REMOTE:-${DEFAULT_REMOTE}}}"
PROXY_URL="${GOPROXY_URL:-${DEFAULT_PROXY}}"
MANIFEST_PATH="${RELEASE_MANIFEST:-${DEFAULT_MANIFEST}}"

for command in go git curl; do
  command -v "${command}" >/dev/null || {
    echo "${command} must be available on PATH" >&2
    exit 1
  }
done

status='failed'
remote_tag_object=''
remote_commit=''
local_commit=''
proxy_zip_sha256=''
proxy_mod_sha256=''
module_sum=''
gomod_sum=''
error_message=''

write_manifest() {
  python3 - "${MANIFEST_PATH}" <<'PY'
import json
import os
import sys
from pathlib import Path

payload = {
    "module": os.environ["MANIFEST_MODULE"],
    "tag": os.environ["MANIFEST_TAG"],
    "remote": os.environ["MANIFEST_REMOTE"],
    "proxy": os.environ["MANIFEST_PROXY"],
    "status": os.environ["MANIFEST_STATUS"],
    "remote_tag_object": os.environ["MANIFEST_REMOTE_TAG_OBJECT"],
    "remote_commit": os.environ["MANIFEST_REMOTE_COMMIT"],
    "local_commit": os.environ["MANIFEST_LOCAL_COMMIT"],
    "proxy_zip_sha256": os.environ["MANIFEST_PROXY_ZIP_SHA256"],
    "proxy_mod_sha256": os.environ["MANIFEST_PROXY_MOD_SHA256"],
    "module_sum": os.environ["MANIFEST_MODULE_SUM"],
    "gomod_sum": os.environ["MANIFEST_GOMOD_SUM"],
    "error": os.environ["MANIFEST_ERROR"],
}
Path(sys.argv[1]).write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
PY
}

finish() {
  local exit_code=$?
  if [[ ${exit_code} -eq 0 ]]; then
    status='passed'
  fi
  MANIFEST_MODULE="${MODULE}" \
  MANIFEST_TAG="${TAG}" \
  MANIFEST_REMOTE="${REMOTE}" \
  MANIFEST_PROXY="${PROXY_URL}" \
  MANIFEST_STATUS="${status}" \
  MANIFEST_REMOTE_TAG_OBJECT="${remote_tag_object}" \
  MANIFEST_REMOTE_COMMIT="${remote_commit}" \
  MANIFEST_LOCAL_COMMIT="${local_commit}" \
  MANIFEST_PROXY_ZIP_SHA256="${proxy_zip_sha256}" \
  MANIFEST_PROXY_MOD_SHA256="${proxy_mod_sha256}" \
  MANIFEST_MODULE_SUM="${module_sum}" \
  MANIFEST_GOMOD_SUM="${gomod_sum}" \
  MANIFEST_ERROR="${error_message}" \
  write_manifest
  rm -rf "${tmp_dir:-}"
  trap - EXIT
  exit "${exit_code}"
}
trap finish EXIT

if [[ -z "${TAG}" ]]; then
  error_message='usage: verify_release.sh vMAJOR.MINOR.PATCH[-PRERELEASE]'
  echo "${error_message}" >&2
  exit 2
fi
if ! [[ "${TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  error_message="release tag must use vMAJOR.MINOR.PATCH or a SemVer prerelease: ${TAG}"
  echo "${error_message}" >&2
  exit 2
fi

fail() {
  error_message="$1"
  echo "ERROR: ${error_message}" >&2
  exit 1
}

sha256() {
  if command -v sha256sum >/dev/null; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    fail 'either sha256sum or shasum -a 256 must be available'
  fi
}

fetch_with_retry() {
  local url="$1"
  local output="$2"
  local attempt=1
  local max_attempts=4
  local delay=1

  while (( attempt <= max_attempts )); do
    if curl --fail --silent --show-error --location --retry 0 "${url}" -o "${output}"; then
      return 0
    fi
    if (( attempt == max_attempts )); then
      fail "unable to fetch ${url} after ${max_attempts} attempts"
    fi
    echo "Retrying ${url} in ${delay}s (attempt ${attempt}/${max_attempts})..." >&2
    sleep "${delay}"
    delay=$((delay * 2))
    attempt=$((attempt + 1))
  done
}

tmp_dir="$(mktemp -d)"

remote_refs="$(git ls-remote --tags "${REMOTE}" "refs/tags/${TAG}" "refs/tags/${TAG}^{}")" || fail "cannot resolve ${TAG} from remote ${REMOTE}"
remote_tag_object="$(printf '%s\n' "${remote_refs}" | awk -v ref="refs/tags/${TAG}" '$2 == ref {print $1; exit}')"
remote_commit="$(printf '%s\n' "${remote_refs}" | awk -v ref="refs/tags/${TAG}^{}" '$2 == ref {print $1; exit}')"
[[ -n "${remote_tag_object}" ]] || fail "remote ${REMOTE} does not contain ${TAG}"
# Lightweight tags do not have a peeled ^{} ref; in that case the ref itself is the commit.
[[ -n "${remote_commit}" ]] || remote_commit="${remote_tag_object}"

local_commit="$(git rev-parse --verify --quiet "refs/tags/${TAG}^{}")" || fail "local checkout does not contain ${TAG}"
[[ "${local_commit}" == "${remote_commit}" ]] || fail "local ${TAG} resolves to ${local_commit}, but ${REMOTE} resolves to ${remote_commit}"

head_commit="$(git rev-parse HEAD)"
[[ "${head_commit}" == "${local_commit}" ]] || fail "HEAD (${head_commit}) is not the ${TAG} commit (${local_commit})"

proxy_base="${PROXY_URL%/}/${MODULE}/@v/${TAG}"
fetch_with_retry "${proxy_base}.zip" "${tmp_dir}/module.zip"
fetch_with_retry "${proxy_base}.mod" "${tmp_dir}/module.mod"
proxy_zip_sha256="$(sha256 "${tmp_dir}/module.zip")"
proxy_mod_sha256="$(sha256 "${tmp_dir}/module.mod")"

module_json="$(GOWORK=off GOPROXY="${PROXY_URL}" go mod download -json "${MODULE}@${TAG}")" || fail "Go could not resolve ${MODULE}@${TAG}"
module_sum="$(printf '%s\n' "${module_json}" | sed -n 's/^[[:space:]]*"Sum": "\([^"]*\)".*/\1/p')"
gomod_sum="$(printf '%s\n' "${module_json}" | sed -n 's/^[[:space:]]*"GoModSum": "\([^"]*\)".*/\1/p')"
[[ -n "${module_sum}" && -n "${gomod_sum}" ]] || fail "Go did not return checksums for ${MODULE}@${TAG}"

echo "OK: ${MODULE}@${TAG}"
echo "  remote tag object: ${remote_tag_object}"
echo "  tagged commit:     ${remote_commit}"
echo "  proxy zip SHA-256:  ${proxy_zip_sha256}"
echo "  proxy mod SHA-256:  ${proxy_mod_sha256}"
echo "  module sum:         ${module_sum}"
echo "  go.mod sum:         ${gomod_sum}"
