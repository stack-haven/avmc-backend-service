#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${ROOT_DIR}/proto"
BASELINE_FILE="${PROTO_DIR}/buf-lint-baseline.txt"
CURRENT_FILE="$(mktemp)"

trap 'rm -f "${CURRENT_FILE}"' EXIT

(
  cd "${PROTO_DIR}"
  buf lint --error-format=json 2>/dev/null || true
) |
  jq -r '[.path, .type] | @tsv' |
  sort |
  uniq -c |
  sed 's/^ *//' >"${CURRENT_FILE}"

if ! diff -u "${BASELINE_FILE}" "${CURRENT_FILE}"; then
  echo "Buf lint violations differ from the approved baseline." >&2
  echo "Fix new violations. Update the baseline only when intentionally accepting or removing debt." >&2
  exit 1
fi

echo "Buf lint baseline check passed."
