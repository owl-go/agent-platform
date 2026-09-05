#!/usr/bin/env bash
set -euo pipefail

registry="${RUNTIME_IMAGE_REGISTRY:-agent-platform}"

check_version() {
  local runtime="$1"
  local version="$2"
  local output
  output="$(docker run --rm --network none "${registry}/${runtime}:${version}" --version)"
  if [[ "${output}" != *"${version}"* ]]; then
    echo "${runtime} version mismatch: ${output}" >&2
    exit 1
  fi
  uid="$(docker run --rm --network none --entrypoint id "${registry}/${runtime}:${version}" -u)"
  if [[ "${uid}" == "0" ]]; then
    echo "${runtime} image runs as root" >&2
    exit 1
  fi
  docker run --rm --network none --mount type=volume,dst=/workspace \
    --entrypoint sh "${registry}/${runtime}:${version}" -ceu '
      test "$(id -u)" = "65532"
      test -w /workspace
      touch /workspace/runtime-write-probe
    '
}

check_version claude 2.1.233
check_version codex 0.147.0
check_version hermes 0.19.0
check_version openclaw 2026.7.1-2
check_version pi 0.84.4

builder_uid="$(docker run --rm --network none --entrypoint id "${registry}/cli-builder:1.0.0" -u)"
if [[ "${builder_uid}" != "65532" ]]; then
  echo "CLI Builder image must run as UID 65532, got ${builder_uid}" >&2
  exit 1
fi

echo "runtime and CLI Builder image smoke tests passed"
