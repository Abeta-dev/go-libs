#!/usr/bin/env bash
set -euo pipefail

# Guard the release workflow's idempotent manifest behavior without requiring a
# GitHub token or creating a release during local verification.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW="${ROOT_DIR}/.github/workflows/release.yml"

require() {
  local description="$1"
  local pattern="$2"
  if ! grep -Fq -- "${pattern}" "${WORKFLOW}"; then
    printf 'ERROR: release workflow must %s\n' "${description}" >&2
    exit 1
  fi
}

require 'retain the Actions release-manifest artifact on every outcome' 'if: always()'
require 'upload release-manifest.json as an Actions artifact' 'path: release-manifest.json'
require 'retain the Actions manifest artifact for 90 days' 'retention-days: 90'
require 'attach release-manifest.json when creating a release' 'gh release create "$TAG" --verify-tag --title "$TAG" --notes-file "$NOTES" release-manifest.json'
require 'replace only the manifest asset when a release already exists' 'gh release upload "$TAG" release-manifest.json --clobber'

if grep -Fq -- 'gh release delete' "${WORKFLOW}"; then
  echo 'ERROR: release workflow must never delete an existing GitHub release.' >&2
  exit 1
fi

echo 'Release workflow manifest assertions passed.'
