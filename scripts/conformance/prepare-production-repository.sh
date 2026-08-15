#!/usr/bin/env bash
set -euo pipefail

repository="${CONFORMANCE_REPOSITORY_URL:-}"
branch="${CONFORMANCE_BASE_BRANCH:-phase0-fixture}"
key="${CONFORMANCE_SSH_KEY_FILE:-}"
known_hosts="${CONFORMANCE_KNOWN_HOSTS_FILE:-}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
staging="$(mktemp -d)"

cleanup() {
  rm -rf "${staging}"
}
trap cleanup EXIT

[[ -n "${repository}" ]] || { echo "CONFORMANCE_REPOSITORY_URL is required" >&2; exit 2; }
[[ -f "${key}" && -f "${known_hosts}" ]] || { echo "SSH key and known_hosts files are required" >&2; exit 2; }
[[ "${key}${known_hosts}" != *"'"* && "${key}${known_hosts}" != *$'\n'* ]] || { echo "SSH paths contain unsupported characters" >&2; exit 2; }

export GIT_SSH_COMMAND="ssh -i '${key}' -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile='${known_hosts}'"
if git ls-remote --exit-code --heads "${repository}" "refs/heads/${branch}" >/dev/null 2>&1; then
  echo "remote fixture branch ${branch} already exists; refusing to overwrite it" >&2
  exit 2
fi

cp -R "${repo_root}/testdata/production-conformance/." "${staging}/"
git -C "${staging}" init --initial-branch "${branch}"
git -C "${staging}" config user.name "Agent Platform Conformance"
git -C "${staging}" config user.email "agent-platform-conformance@example.invalid"
git -C "${staging}" remote add origin "${repository}"
git -C "${staging}" add --all
git -C "${staging}" commit -m "test: add production conformance fixture"
git -C "${staging}" push --set-upstream origin "${branch}"

echo "fixture pushed to ${branch}"
