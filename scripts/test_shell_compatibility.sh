#!/usr/bin/env bash
set -euo pipefail

# Keep repository scripts runnable by macOS's system Bash 3.2. This static gate
# rejects Bash 4+ syntax while allowing indexed arrays, [[ =~ ]], and BASH_REMATCH,
# all of which are available in Bash 3.2.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

forbidden_patterns=(
  'map'"file"
  'read'"array"
  'declare -'"A"
  'typeset -'"A"
  'cop'"roc"
  'wait -'"n"
  '&'">>"
  '[|]&'
  '\$\{[^}]*,,[^}]*\}'
  '\$\{[^}]*\^\^[^}]*\}'
)

failed=0
for pattern in "${forbidden_patterns[@]}"; do
  # This script spells prohibited constructs in data to detect them, so exclude
  # it from the source scan rather than treating those literals as syntax.
  matches=$(git grep -nE "${pattern}" -- 'scripts/*.sh' ':!scripts/test_shell_compatibility.sh' || true)
  if [ -n "${matches}" ]; then
    printf 'ERROR: Bash 4+ syntax (%s) is not supported by Bash 3.2:\n%s\n' "${pattern}" "${matches}" >&2
    failed=1
  fi
done

if [ "${failed}" -ne 0 ]; then
  exit 1
fi

echo 'Bash 3.2 compatibility check passed.'
