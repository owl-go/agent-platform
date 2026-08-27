#!/usr/bin/env bash
set -euo pipefail

failures=()
require_value() {
  local name="$1"
  [[ -n "${!name:-}" ]] || failures+=("missing ${name}")
}

require_http_url() {
  local name="$1"
  local value="${!name:-}"
  if [[ -n "${value}" && ! "${value}" =~ ^https?://[^[:space:]]+$ ]]; then
    failures+=("${name} must be an HTTP(S) URL without whitespace")
  fi
}

mode_of() {
  stat -c '%a' "$1" 2>/dev/null || true
}

owner_of() {
  stat -c '%u:%g' "$1" 2>/dev/null || true
}

[[ "$(uname -s)" == "Linux" ]] || failures+=("Production Conformance requires Linux")
for command in docker go git jq rg curl ssh ssh-keygen stat; do
  command -v "${command}" >/dev/null 2>&1 || failures+=("missing command ${command}")
done
if command -v docker >/dev/null 2>&1; then
  docker info --format '{{json .Runtimes}}' 2>/dev/null | rg -q '"runsc"' || failures+=("Docker runtime runsc is unavailable")
fi

for name in \
  CONFORMANCE_REPOSITORY_URL CONFORMANCE_BASE_BRANCH \
  CONFORMANCE_WORK_ROOT CONFORMANCE_EVIDENCE_ROOT \
  AGENT_RESOLVER_CONFIG_FILE \
  SANDBOX_REDIRECT_TEST_URL SANDBOX_REBIND_TEST_URL SANDBOX_CONTROL_PLANE_TEST_URL \
  CONFORMANCE_CLAUDE_IMAGE CONFORMANCE_CLAUDE_MODEL CONFORMANCE_CLAUDE_CREDENTIAL_DIR \
  CONFORMANCE_CODEX_IMAGE CONFORMANCE_CODEX_MODEL CONFORMANCE_CODEX_CREDENTIAL_DIR \
  CONFORMANCE_HERMES_IMAGE CONFORMANCE_HERMES_MODEL CONFORMANCE_HERMES_CREDENTIAL_DIR \
  CONFORMANCE_OPENCLAW_IMAGE CONFORMANCE_OPENCLAW_MODEL CONFORMANCE_OPENCLAW_CREDENTIAL_DIR \
  ALIYUN_OSS_ENDPOINT ALIYUN_OSS_ACCESS_KEY ALIYUN_OSS_SECRET_KEY ALIYUN_OSS_BUCKET \
  MINIO_ENDPOINT MINIO_ACCESS_KEY MINIO_SECRET_KEY MINIO_BUCKET; do
  require_value "${name}"
done

if [[ -n "${AGENT_RESOLVER_CONFIG_FILE:-}" ]]; then
  if [[ "${AGENT_RESOLVER_CONFIG_FILE}" != /* || ! -f "${AGENT_RESOLVER_CONFIG_FILE}" || -L "${AGENT_RESOLVER_CONFIG_FILE}" ]]; then
    failures+=("AGENT_RESOLVER_CONFIG_FILE must be an existing absolute regular file")
  else
    [[ "$(owner_of "${AGENT_RESOLVER_CONFIG_FILE}")" == "0:0" ]] || failures+=("AGENT_RESOLVER_CONFIG_FILE must be owned by root:root")
    [[ "$(mode_of "${AGENT_RESOLVER_CONFIG_FILE}")" == "444" ]] || failures+=("AGENT_RESOLVER_CONFIG_FILE must have mode 444")
  fi
fi

if [[ -n "${CONFORMANCE_REPOSITORY_URL:-}" && \
      ! "${CONFORMANCE_REPOSITORY_URL}" =~ ^[^@[:space:]]+@[^:[:space:]]+:.+$ && \
      ! "${CONFORMANCE_REPOSITORY_URL}" =~ ^ssh://[^[:space:]]+$ ]]; then
  failures+=("CONFORMANCE_REPOSITORY_URL must be an SSH repository URL")
fi
if [[ -n "${CONFORMANCE_BASE_BRANCH:-}" && ! "${CONFORMANCE_BASE_BRANCH}" =~ ^[A-Za-z0-9._/-]+$ ]]; then
  failures+=("CONFORMANCE_BASE_BRANCH contains unsupported characters")
fi
for url_name in SANDBOX_REDIRECT_TEST_URL SANDBOX_REBIND_TEST_URL SANDBOX_CONTROL_PLANE_TEST_URL; do
  require_http_url "${url_name}"
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
    if [[ "${credential_dir}" != /* || "${credential_dir}" == *[[:space:],]* || ! -d "${credential_dir}" ]]; then
      failures+=("${credential_name} must be an existing absolute directory without whitespace or commas")
      continue
    fi
    key_file="${credential_dir}/git/id_ed25519"
    known_hosts_file="${credential_dir}/git/known_hosts"
    canary_file="${credential_dir}/env/CONFORMANCE_CANARY_SECRET"
    [[ -f "${key_file}" ]] || failures+=("${credential_name} is missing git/id_ed25519")
    [[ -f "${known_hosts_file}" ]] || failures+=("${credential_name} is missing git/known_hosts")
    [[ -f "${canary_file}" ]] || failures+=("${credential_name} is missing env/CONFORMANCE_CANARY_SECRET")
    if [[ -f "${credential_dir}/env/CONFORMANCE_CANARY_SECRET" ]]; then
      canary="$(<"${credential_dir}/env/CONFORMANCE_CANARY_SECRET")"
      [[ ${#canary} -ge 16 && "${canary}" != *$'\n'* ]] || failures+=("${credential_name} canary must be a single line with at least 16 characters")
    fi
    model_credential=""
    if [[ -d "${credential_dir}/env" ]]; then
      model_credential="$(find "${credential_dir}/env" -mindepth 1 -maxdepth 1 -type f ! -name CONFORMANCE_CANARY_SECRET -print -quit)"
    fi
    if [[ -z "${model_credential}" ]]; then
      failures+=("${credential_name}/env contains no model credential besides CONFORMANCE_CANARY_SECRET")
    fi
    unsafe_entry="$(find "${credential_dir}" -mindepth 1 ! -type d ! -type f -print -quit)"
    if [[ -n "${unsafe_entry}" ]]; then
      failures+=("${credential_name} contains a symlink or special file")
    fi
    if [[ -d "${credential_dir}/env" ]]; then
      for environment_file in "${credential_dir}"/env/*; do
        [[ -f "${environment_file}" ]] || continue
        environment_name="$(basename "${environment_file}")"
        if [[ ! "${environment_name}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
          failures+=("${credential_name}/env contains invalid environment name ${environment_name}")
        fi
      done
    fi
    for directory in "${credential_dir}" "${credential_dir}/env" "${credential_dir}/git"; do
      if [[ -d "${directory}" && "$(mode_of "${directory}")" != "700" ]]; then
        failures+=("${directory} must have mode 700")
      fi
      if [[ -d "${directory}" && "$(owner_of "${directory}")" != "65532:65532" ]]; then
        failures+=("${directory} must be owned by 65532:65532")
      fi
    done
    while IFS= read -r credential_file; do
      [[ "$(mode_of "${credential_file}")" == "600" ]] || failures+=("${credential_file} must have mode 600")
      [[ "$(owner_of "${credential_file}")" == "65532:65532" ]] || failures+=("${credential_file} must be owned by 65532:65532")
    done < <(find "${credential_dir}" -type f -print)
    if [[ -f "${key_file}" ]] && ! ssh-keygen -y -P '' -f "${key_file}" </dev/null >/dev/null 2>&1; then
      failures+=("${credential_name} git/id_ed25519 must be a readable unencrypted private key")
    fi
    if [[ -f "${known_hosts_file}" && ! -s "${known_hosts_file}" ]]; then
      failures+=("${credential_name} git/known_hosts is empty")
    fi
  fi
done

for root_name in CONFORMANCE_WORK_ROOT CONFORMANCE_EVIDENCE_ROOT; do
  root="${!root_name:-}"
  [[ -z "${root}" || "${root}" == /* ]] || failures+=("${root_name} must be absolute")
  if [[ -n "${root}" && -e "${root}" && ( ! -d "${root}" || ! -w "${root}" ) ]]; then
    failures+=("${root_name} must be a writable directory")
  fi
done
if [[ -n "${CONFORMANCE_WORK_ROOT:-}" && -n "${CONFORMANCE_EVIDENCE_ROOT:-}" && "${CONFORMANCE_WORK_ROOT}" == "${CONFORMANCE_EVIDENCE_ROOT}" ]]; then
  failures+=("work and evidence roots must differ")
fi

network="${AGENT_EGRESS_NETWORK:-agent-public-egress}"
if command -v docker >/dev/null 2>&1; then
  docker network inspect "${network}" >/dev/null 2>&1 || failures+=("Docker network ${network} is unavailable")
fi

if ((${#failures[@]} > 0)); then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi

# Fail before model calls if an immutable image or a Runtime-specific Git credential is unusable.
for runtime in CLAUDE CODEX HERMES OPENCLAW; do
  image_name="CONFORMANCE_${runtime}_IMAGE"
  credential_name="CONFORMANCE_${runtime}_CREDENTIAL_DIR"
  image="${!image_name}"
  credential_dir="${!credential_name}"
  docker image inspect "${image}" >/dev/null 2>&1 || failures+=("${image_name} is unavailable to Docker")
  if ! GIT_SSH_COMMAND="ssh -i ${credential_dir}/git/id_ed25519 -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=${credential_dir}/git/known_hosts" \
    git ls-remote --exit-code --heads "${CONFORMANCE_REPOSITORY_URL}" "refs/heads/${CONFORMANCE_BASE_BRANCH}" >/dev/null 2>&1; then
    failures+=("${credential_name} cannot read ${CONFORMANCE_BASE_BRANCH} from CONFORMANCE_REPOSITORY_URL")
  fi
done

if ((${#failures[@]} > 0)); then
  printf '%s\n' "${failures[@]}" >&2
  exit 1
fi

echo "Production Conformance preflight passed"
