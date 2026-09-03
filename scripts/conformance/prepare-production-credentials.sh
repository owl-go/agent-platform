#!/usr/bin/env bash
set -euo pipefail
set -f

credential_root="${CONFORMANCE_CREDENTIAL_ROOT:-}"
ssh_key="${CONFORMANCE_SSH_KEY_FILE:-}"
known_hosts="${CONFORMANCE_KNOWN_HOSTS_FILE:-}"
owner_uid="${CONFORMANCE_CREDENTIAL_UID:-65532}"
owner_gid="${CONFORMANCE_CREDENTIAL_GID:-65532}"

fail() {
  echo "$1" >&2
  exit 2
}

[[ "$(uname -s)" == "Linux" ]] || fail "production credential preparation requires Linux"
[[ "$(id -u)" == "0" ]] || fail "production credential preparation requires root"
[[ "${credential_root}" == /* && "${credential_root}" != *[[:space:],]* ]] || fail "CONFORMANCE_CREDENTIAL_ROOT must be an absolute path without whitespace or commas"
[[ -f "${ssh_key}" && ! -L "${ssh_key}" ]] || fail "CONFORMANCE_SSH_KEY_FILE must be a regular private key file"
[[ -s "${known_hosts}" && ! -L "${known_hosts}" ]] || fail "CONFORMANCE_KNOWN_HOSTS_FILE must be a non-empty regular file"
[[ "${owner_uid}" =~ ^[0-9]+$ && "${owner_gid}" =~ ^[0-9]+$ ]] || fail "credential UID and GID must be numeric"
ssh-keygen -y -P '' -f "${ssh_key}" </dev/null >/dev/null 2>&1 || fail "CONFORMANCE_SSH_KEY_FILE must be a readable unencrypted private key"

if [[ -e "${credential_root}" ]]; then
  fail "CONFORMANCE_CREDENTIAL_ROOT already exists; use a new path to avoid overwriting credentials"
fi

runtimes=(claude codex hermes openclaw pi)
install -d -m 0700 "${credential_root}"
for runtime in "${runtimes[@]}"; do
  upper="${runtime^^}"
  names_variable="CONFORMANCE_${upper}_MODEL_ENV_NAMES"
  environment_names="${!names_variable:-}"
  [[ -n "${environment_names}" ]] || fail "${names_variable} must list at least one model credential environment variable"

  runtime_root="${credential_root}/${runtime}"
  install -d -m 0700 "${runtime_root}"
  install -d -m 0700 "${runtime_root}/env" "${runtime_root}/git"
  install -m 0600 "${ssh_key}" "${runtime_root}/git/id_ed25519"
  install -m 0600 "${known_hosts}" "${runtime_root}/git/known_hosts"

  written=0
  for environment_name in ${environment_names}; do
    [[ "${environment_name}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || fail "${names_variable} contains invalid environment name ${environment_name}"
    environment_value="${!environment_name:-}"
    [[ -n "${environment_value}" ]] || fail "${environment_name} is empty or unset for ${runtime}"
    [[ "${environment_value}" != *$'\n'* ]] || fail "${environment_name} must be a single-line value"
    printf '%s' "${environment_value}" >"${runtime_root}/env/${environment_name}"
    chmod 0600 "${runtime_root}/env/${environment_name}"
    written=$((written + 1))
  done
  ((written > 0)) || fail "no model credentials were written for ${runtime}"

  canary="$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')"
  printf '%s' "${canary}" >"${runtime_root}/env/CONFORMANCE_CANARY_SECRET"
  chmod 0600 "${runtime_root}/env/CONFORMANCE_CANARY_SECRET"
  chown -R "${owner_uid}:${owner_gid}" "${runtime_root}"
done

chmod 0700 "${credential_root}"
chown "${owner_uid}:${owner_gid}" "${credential_root}"
echo "prepared isolated production credentials under ${credential_root}"
