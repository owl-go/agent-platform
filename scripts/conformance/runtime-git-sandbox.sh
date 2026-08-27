#!/usr/bin/env bash
set -euo pipefail

operation="${1:-}"
image="${2:-}"
workspace="${3:-}"
credentials="${4:-}"
repository="${5:-}"
branch="${6:-}"
network="${AGENT_EGRESS_NETWORK:-agent-public-egress}"
resolver_file="${AGENT_RESOLVER_CONFIG_FILE:-}"
suite_id="${CONFORMANCE_SUITE_ID:-standalone-$$}"
run_id="${suite_id}-git-${operation}-$$"

fail() {
  echo "$1" >&2
  exit 2
}

[[ "$(uname -s)" == "Linux" ]] || fail "Git sandbox requires Linux"
[[ "${operation}" == "clone" || "${operation}" == "push" || "${operation}" == "test" ]] || fail "operation must be clone, push, or test"
[[ "${image}" =~ ^[^[:space:]@]+@sha256:[a-f0-9]{64}$ ]] || fail "runtime image must be an immutable repo digest"
[[ "${workspace}" == /* && -d "${workspace}" && "${workspace}" != *","* ]] || fail "workspace must be an existing absolute path without commas"
[[ "${credentials}" == /* && -d "${credentials}" && "${credentials}" != *","* ]] || fail "credential directory must be an existing absolute path without commas"
[[ -f "${credentials}/git/id_ed25519" && -f "${credentials}/git/known_hosts" ]] || fail "Git SSH key and pinned known_hosts are required"
[[ -n "${repository}" && -n "${branch}" ]] || fail "repository and branch are required"
[[ "${resolver_file}" == /* && -f "${resolver_file}" && "${resolver_file}" != *","* ]] || fail "AGENT_RESOLVER_CONFIG_FILE must be an existing absolute file without commas"
[[ "${suite_id}" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "CONFORMANCE_SUITE_ID contains unsupported characters"
docker info --format '{{json .Runtimes}}' | rg -q '"runsc"' || fail "Docker runtime runsc is unavailable"
docker network inspect "${network}" >/dev/null

common=(
  run --rm
  --runtime runsc
  --user 65532:65532
  --read-only
  --cap-drop ALL
  --security-opt no-new-privileges
  --network "${network}"
  --memory 1073741824
  --pids-limit 256
  --cpus 2
  --mount "type=bind,src=${workspace},dst=/workspace,readonly=false"
  --mount "type=bind,src=${credentials},dst=/run/agent-credentials,readonly=true"
  --mount "type=bind,src=${resolver_file},dst=/etc/resolv.conf,readonly=true"
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=134217728
  --workdir /workspace
  --init
  --label agent-platform.managed=true
  --label "agent-platform.run-id=${run_id}"
  --label "agent-platform.conformance-suite=${suite_id}"
  --entrypoint /usr/local/bin/runtime-entrypoint
)

case "${operation}" in
  clone)
    if find "${workspace}" -mindepth 1 -maxdepth 1 | read -r; then
      fail "clone workspace must be empty"
    fi
    docker "${common[@]}" "${image}" sh -ceu '
      git clone --single-branch --branch "$2" "$1" /workspace
    ' sh "${repository}" "${branch}"
    ;;
  test)
    docker "${common[@]}" "${image}" sh -ceu '
      test -d .git
      ./scripts/test.sh
    ' sh "${repository}" "${branch}"
    ;;
  push)
    author_name="${CONFORMANCE_GIT_AUTHOR_NAME:-Agent Platform Conformance}"
    author_email="${CONFORMANCE_GIT_AUTHOR_EMAIL:-agent-platform-conformance@example.invalid}"
    docker "${common[@]}" "${image}" sh -ceu '
      test -d .git
      git config user.name "$3"
      git config user.email "$4"
      git add --all
      git diff --cached --quiet && { echo "no staged conformance change" >&2; exit 3; }
      git commit -m "chore: runtime production conformance"
      git push origin "HEAD:refs/heads/$2"
    ' sh "${repository}" "${branch}" "${author_name}" "${author_email}"
    ;;
esac
