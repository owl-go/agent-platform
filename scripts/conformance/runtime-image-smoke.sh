#!/usr/bin/env bash
set -euo pipefail

registry="${RUNTIME_IMAGE_REGISTRY:-agent-platform}"
image="${RUNTIME_IMAGE_REF:-${registry}/runtime:${RUNTIME_IMAGE_TAG_OVERRIDE:-1.0.0}}"
builder_image="${CLI_BUILDER_IMAGE_REF:-${registry}/cli-builder:1.0.0}"

check_version() {
  local runtime="$1"
  local version="$2"
  local output
  output="$(docker run --rm --network none --read-only \
    --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m \
    --mount type=volume,dst=/workspace \
    "${image}" "${runtime}" --version)"
  if [[ "${output}" != *"${version}"* ]]; then
    echo "${runtime} version mismatch: ${output}" >&2
    exit 1
  fi
}

check_version claude 2.1.233
check_version codex 0.147.0
check_version hermes 0.19.0
check_version openclaw 2026.7.1-2
check_version pi 0.84.4

docker run --rm --network none --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m \
  --mount type=volume,dst=/workspace \
  --entrypoint sh "${image}" -ceu '
    test "$(id -u)" = "65532"
    test -w /workspace
    test -x /usr/local/bin/agent-cli
    test -x /usr/local/bin/runtime-entrypoint
    test "$(node --version | cut -d. -f1)" = "v24"
    test "$(python --version 2>&1 | cut -d. -f1-2)" = "Python 3.13"
    if output="$(AGENT_PLATFORM_CLI_SOCKET=/tmp/agent-cli-protocol-smoke.sock agent-cli --connector test --capability test --identity user -- test 2>&1)"; then
      echo "agent-cli must fail closed without a broker socket" >&2
      exit 1
    fi
    case "$output" in
      *"connect ENOENT"*) ;;
      *) echo "agent-cli does not accept the current broker command protocol: $output" >&2; exit 1 ;;
    esac
    touch /workspace/runtime-write-probe
  '

builder_uid="$(docker run --rm --network none --entrypoint id "${builder_image}" -u)"
if [[ "${builder_uid}" != "65532" ]]; then
  echo "CLI Builder image must run as UID 65532, got ${builder_uid}" >&2
  exit 1
fi

echo "runtime and CLI Builder image smoke tests passed"
