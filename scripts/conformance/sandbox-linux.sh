#!/usr/bin/env bash
set -euo pipefail

image="${SANDBOX_TEST_IMAGE:-}"
network_name="${AGENT_EGRESS_NETWORK:-agent-public-egress}"
resolver_file="${AGENT_RESOLVER_CONFIG_FILE:-}"
public_url="${SANDBOX_PUBLIC_TEST_URL:-https://example.com}"
redirect_url="${SANDBOX_REDIRECT_TEST_URL:-}"
rebind_url="${SANDBOX_REBIND_TEST_URL:-}"
control_plane_url="${SANDBOX_CONTROL_PLANE_TEST_URL:-}"
test_entrypoint="${SANDBOX_TEST_ENTRYPOINT:-/usr/local/bin/runtime-entrypoint}"
container_name="agent-sandbox-conformance-$(date +%s%N)-$$"
volume_name=""
credential_root="$(mktemp -d)"

cleanup() {
  docker rm --force --volumes "${container_name}" >/dev/null 2>&1 || true
  [[ -z "${volume_name}" ]] || docker volume rm "${volume_name}" >/dev/null 2>&1 || true
  rm -rf "${credential_root}"
}
trap cleanup EXIT

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "sandbox conformance requires Linux" >&2
  exit 1
fi
if [[ -z "${image}" || "${image}" != *@sha256:* ]]; then
  echo "SANDBOX_TEST_IMAGE must be an immutable image digest with sh, curl, and id" >&2
  exit 1
fi
if [[ -z "${redirect_url}" || -z "${rebind_url}" || -z "${control_plane_url}" ]]; then
  echo "SANDBOX_REDIRECT_TEST_URL, SANDBOX_REBIND_TEST_URL, and SANDBOX_CONTROL_PLANE_TEST_URL are required" >&2
  exit 1
fi
if [[ "${resolver_file}" != /* || ! -f "${resolver_file}" ]]; then
  echo "AGENT_RESOLVER_CONFIG_FILE must be an existing absolute file" >&2
  exit 1
fi
if ! docker info --format '{{json .Runtimes}}' | grep -q '"runsc"'; then
  echo "Docker runtime runsc is unavailable" >&2
  exit 1
fi
docker network inspect "${network_name}" >/dev/null

mkdir -p "${credential_root}/selected"
printf '%s' 'fake-model-secret' >"${credential_root}/selected/model-key"
chown -R 65532:65532 "${credential_root}/selected"
chmod 0700 "${credential_root}/selected"
chmod 0600 "${credential_root}/selected/model-key"
volume_name="$(docker volume create --label agent-platform.managed=true --label agent-platform.purpose=sandbox-conformance)"

entrypoint_args=()
if [[ -n "${test_entrypoint}" ]]; then
  entrypoint_args=(--entrypoint "${test_entrypoint}")
fi

docker create \
  --name "${container_name}" \
  --runtime runsc \
  --user 65532:65532 \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --network "${network_name}" \
  --memory 536870912 \
  --pids-limit 256 \
  --cpus 2 \
  --mount "type=volume,src=${volume_name},dst=/workspace,readonly=false" \
  --mount "type=bind,src=${credential_root}/selected,dst=/run/agent-credentials,readonly=true" \
  --mount "type=bind,src=${resolver_file},dst=/etc/resolv.conf,readonly=true" \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=67108864 \
  --label agent-platform.managed=true \
  --label agent-platform.run-id=conformance \
  --label agent-platform.egress=public \
  --init \
  "${entrypoint_args[@]}" \
  "${image}" \
  sh -ceu '
    test "$(id -u)" = "65532"
    test -r /run/agent-credentials/model-key
    test "$(cat /run/agent-credentials/model-key)" = "fake-model-secret"
    test ! -S /var/run/docker.sock
    if touch /rootfs-write-probe 2>/dev/null; then exit 20; fi
    touch /workspace/workspace-write-probe
    curl --noproxy "*" -fsS --connect-timeout 5 "$1" >/dev/null
    for target in \
      http://127.0.0.1/ http://host.docker.internal/ http://169.254.169.254/ \
      http://10.0.0.1/ http://172.16.0.1/ http://172.17.0.1/ http://192.168.0.1/ \
      "$2" "$3" "$4"; do
      if curl --noproxy "*" -LfsS --connect-timeout 2 --max-time 5 "$target" >/dev/null 2>&1; then exit 21; fi
    done
    sleep 300
  ' sh "${public_url}" "${redirect_url}" "${rebind_url}" "${control_plane_url}" >/dev/null

runtime="$(docker inspect --format '{{.HostConfig.Runtime}}' "${container_name}")"
readonly_root="$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "${container_name}")"
configured_user="$(docker inspect --format '{{.Config.User}}' "${container_name}")"
if [[ "${runtime}" != "runsc" || "${readonly_root}" != "true" || "${configured_user}" != "65532:65532" ]]; then
  echo "created container does not match isolation policy" >&2
  exit 1
fi

docker start "${container_name}" >/dev/null
deadline=$((SECONDS + 30))
while [[ "$(docker inspect --format '{{.State.Status}}' "${container_name}")" == "created" ]]; do
  if (( SECONDS >= deadline )); then
    echo "sandbox did not start" >&2
    exit 1
  fi
  sleep 1
done
if [[ "$(docker inspect --format '{{.State.Running}}' "${container_name}")" != "true" ]]; then
  docker logs "${container_name}" >&2
  exit 1
fi

docker stop --time 2 "${container_name}" >/dev/null
if [[ "$(docker inspect --format '{{.State.Running}}' "${container_name}")" != "false" ]]; then
  echo "sandbox survived docker stop timeout" >&2
  exit 1
fi

echo "sandbox Linux conformance passed for ${image}"
