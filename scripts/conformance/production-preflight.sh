#!/usr/bin/env bash
set -euo pipefail

failures=()
require_value() {
  local name="$1"
  [[ -n "${!name:-}" ]] || failures+=("missing ${name}")
}

[[ "$(uname -s)" == "Linux" ]] || failures+=("Production Conformance requires Linux")
for command in docker go git jq rg curl; do
  command -v "${command}" >/dev/null 2>&1 || failures+=("missing command ${command}")
done
if command -v docker >/dev/null 2>&1; then
  docker info --format '{{json .Runtimes}}' 2>/dev/null | rg -q '"runsc"' || failures+=("Docker runtime runsc is unavailable")
fi

for name in \
  CONFORMANCE_REPOSITORY_URL CONFORMANCE_BASE_BRANCH \
  CONFORMANCE_WORK_ROOT CONFORMANCE_EVIDENCE_ROOT \
  CONFORMANCE_CLAUDE_IMAGE CONFORMANCE_CLAUDE_MODEL CONFORMANCE_CLAUDE_CREDENTIAL_DIR \
  CONFORMANCE_CODEX_IMAGE CONFORMANCE_CODEX_MODEL CONFORMANCE_CODEX_CREDENTIAL_DIR \
  CONFORMANCE_HERMES_IMAGE CONFORMANCE_HERMES_MODEL CONFORMANCE_HERMES_CREDENTIAL_DIR \
  CONFORMANCE_OPENCLAW_IMAGE CONFORMANCE_OPENCLAW_MODEL CONFORMANCE_OPENCLAW_CREDENTIAL_DIR \
  ALIYUN_OSS_ENDPOINT ALIYUN_OSS_ACCESS_KEY ALIYUN_OSS_SECRET_KEY ALIYUN_OSS_BUCKET; do
  require_value "${name}"
done

for runtime in CLAUDE CODEX HERMES OPENCLAW; do
  image_name="CONFORMANCE_${runtime}_IMAGE"
  image="${!image_name:-}"
  if [[ -n "${image}" && ! "${image}" =~ ^[^[:space:]@]+@sha256:[a-f0-9]{64}$ ]]; then
    failures+=("${image_name} must be an immutable repo digest")
  fi
  credential_name="CONFORMANCE_${runtime}_CREDENTIAL_DIR"
  credential_dir="${!credential_name:-}"
  if [[ -n "${credential_dir}" ]]; then
    [[ "${credential_dir}" == /* && -d "${credential_dir}" ]] || failures+=("${credential_name} must be an existing absolute directory")
    [[ -f "${credential_dir}/git/id_ed25519" ]] || failures+=("${credential_name} is missing git/id_ed25519")
    [[ -f "${credential_dir}/git/known_hosts" ]] || failures+=("${credential_name} is missing git/known_hosts")
    [[ -f "${credential_dir}/env/CONFORMANCE_CANARY_SECRET" ]] || failures+=("${credential_name} is missing env/CONFORMANCE_CANARY_SECRET")
    if [[ -d "${credential_dir}/env" ]] && ! find "${credential_dir}/env" -type f -mindepth 1 -maxdepth 1 | read -r; then
      failures+=("${credential_name}/env contains no model credential")
    fi
  fi
done

for root_name in CONFORMANCE_WORK_ROOT CONFORMANCE_EVIDENCE_ROOT; do
  root="${!root_name:-}"
  [[ -z "${root}" || "${root}" == /* ]] || failures+=("${root_name} must be absolute")
done
[[ "${CONFORMANCE_WORK_ROOT:-}" != "${CONFORMANCE_EVIDENCE_ROOT:-}" ]] || failures+=("work and evidence roots must differ")

network="${AGENT_EGRESS_NETWORK:-agent-public-egress}"
if command -v docker >/dev/null 2>&1; then
  docker network inspect "${network}" >/dev/null 2>&1 || failures+=("Docker network ${network} is unavailable")
fi

if ((${#failures[@]} > 0)); then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi

echo "Production Conformance preflight passed"
