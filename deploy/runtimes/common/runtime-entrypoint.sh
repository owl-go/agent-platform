#!/bin/sh
set -eu

umask 077
mkdir -p "${HOME}"
if [ -n "${CODEX_HOME:-}" ]; then
  mkdir -p "${CODEX_HOME}"
fi

credential_root="/run/agent-credentials"
if [ -d "${credential_root}/env" ]; then
  for credential_file in "${credential_root}"/env/*; do
    [ -f "${credential_file}" ] || continue
    variable_name="$(basename "${credential_file}")"
    case "${variable_name}" in
      ''|*[!A-Za-z0-9_]*|[0-9]*)
        echo "invalid credential environment name: ${variable_name}" >&2
        exit 2
        ;;
    esac
    variable_value="$(cat "${credential_file}")"
    export "${variable_name}=${variable_value}"
  done
fi

if [ -f "${credential_root}/git/id_ed25519" ]; then
  if [ ! -f "${credential_root}/git/known_hosts" ]; then
    echo "git SSH key requires a pinned known_hosts file" >&2
    exit 2
  fi
  export GIT_SSH_COMMAND="ssh -i ${credential_root}/git/id_ed25519 -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=${credential_root}/git/known_hosts"
fi

exec "$@"
