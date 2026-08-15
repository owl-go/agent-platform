#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
"${repo_root}/scripts/conformance/production-preflight.sh"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
suite_root="${CONFORMANCE_EVIDENCE_ROOT}/${stamp}"
mkdir -p "${CONFORMANCE_WORK_ROOT}" "${suite_root}"
go build -o "${suite_root}/runtime-conformance" ./cmd/runtime-conformance

prepare_owned_directory() {
  local directory="$1"
  mkdir -p "${directory}"
  if [[ "$(id -u)" == "0" ]]; then
    chown 65532:65532 "${directory}"
  fi
  [[ "$(stat -c '%u:%g' "${directory}")" == "65532:65532" ]] || {
    echo "${directory} must be owned by 65532:65532" >&2
    exit 2
  }
}

wait_for_container() {
  local run_id="$1"
  local deadline=$((SECONDS + 120))
  local container=""
  while [[ -z "${container}" ]]; do
    container="$(docker ps --filter "label=agent-platform.run-id=${run_id}" --format '{{.ID}}' | head -n 1)"
    if ((SECONDS >= deadline)); then
      echo "container for ${run_id} did not start" >&2
      return 1
    fi
    [[ -n "${container}" ]] || sleep 1
  done
  printf '%s' "${container}"
}

SANDBOX_TEST_IMAGE="${CONFORMANCE_CLAUDE_IMAGE}" \
  "${repo_root}/scripts/conformance/sandbox-linux.sh" >"${suite_root}/sandbox.log" 2>&1

runtimes=(claude codex hermes openclaw)
for runtime in "${runtimes[@]}"; do
  upper="${runtime^^}"
  image_name="CONFORMANCE_${upper}_IMAGE"
  model_name="CONFORMANCE_${upper}_MODEL"
  credential_name="CONFORMANCE_${upper}_CREDENTIAL_DIR"
  image="${!image_name}"
  model="${!model_name}"
  credential_dir="${!credential_name}"
  workspace="${CONFORMANCE_WORK_ROOT}/${stamp}-${runtime}"
  evidence_root="${suite_root}/${runtime}"
  forced_evidence="${evidence_root}/forced-kill"
  evidence="${evidence_root}/recovery"
  review_branch="conformance/${stamp}/${runtime}"

  prepare_owned_directory "${workspace}"
  prepare_owned_directory "${evidence_root}"
  mkdir -p "${forced_evidence}" "${evidence}"
  chown 65532:65532 "${forced_evidence}" "${evidence}" 2>/dev/null || true

  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" clone \
    "${image}" "${workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${CONFORMANCE_BASE_BRANCH}"

  forced_run_id="${stamp}-${runtime}-forced"
  set +e
  "${suite_root}/runtime-conformance" \
    --runtime "${runtime}" \
    --image "${image}" \
    --model "${model}" \
    --workspace "${workspace}" \
    --credentials "${credential_dir}" \
    --output "${forced_evidence}" \
    --run-id "${forced_run_id}" \
    --network "${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    --timeout 15m \
    --instruction "Complete the task in CONFORMANCE.md. After scripts/test.sh passes, run scripts/long-command.sh as the final action and wait for it." &
  forced_pid=$!
  set -e
  forced_container="$(wait_for_container "${forced_run_id}")"
  deadline=$((SECONDS + 600))
  until "${workspace}/scripts/test.sh" >/dev/null 2>&1; do
    if ! kill -0 "${forced_pid}" 2>/dev/null; then
      echo "${runtime} exited before producing the forced-recovery snapshot" >&2
      wait "${forced_pid}" || true
      exit 1
    fi
    if ((SECONDS >= deadline)); then
      echo "${runtime} did not fix the fixture before forced termination" >&2
      kill -TERM "${forced_pid}" 2>/dev/null || true
      wait "${forced_pid}" || true
      exit 1
    fi
    sleep 2
  done
  docker kill "${forced_container}" >/dev/null
  if wait "${forced_pid}"; then
    echo "${runtime} conformance unexpectedly succeeded after docker kill" >&2
    exit 1
  fi
  jq -e '.error != null' "${forced_evidence}/report.json" >/dev/null

  "${suite_root}/runtime-conformance" \
    --runtime "${runtime}" \
    --image "${image}" \
    --model "${model}" \
    --workspace "${workspace}" \
    --credentials "${credential_dir}" \
    --output "${evidence}" \
    --run-id "${stamp}-${runtime}" \
    --network "${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    --instruction "$(<"${repo_root}/testdata/production-conformance/task.txt")"

  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" test \
    "${image}" "${workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${review_branch}"
  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" push \
    "${image}" "${workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${review_branch}"

  canary="$(<"${credential_dir}/env/CONFORMANCE_CANARY_SECRET")"
  if [[ ${#canary} -lt 16 ]]; then
    echo "${credential_name} canary must contain at least 16 characters" >&2
    exit 2
  fi
  if rg --fixed-strings --quiet -- "${canary}" "${evidence}" "${workspace}"; then
    echo "credential canary persisted for ${runtime}" >&2
    exit 1
  fi

  interrupt_workspace="${CONFORMANCE_WORK_ROOT}/${stamp}-${runtime}-interrupt"
  interrupt_evidence="${evidence_root}/interrupt"
  interrupt_run_id="${stamp}-${runtime}-interrupt"
  prepare_owned_directory "${interrupt_workspace}"
  mkdir -p "${interrupt_evidence}"
  chown 65532:65532 "${interrupt_evidence}" 2>/dev/null || true
  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" clone \
    "${image}" "${interrupt_workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${CONFORMANCE_BASE_BRANCH}"

  set +e
  "${suite_root}/runtime-conformance" \
    --runtime "${runtime}" --image "${image}" --model "${model}" \
    --workspace "${interrupt_workspace}" --credentials "${credential_dir}" \
    --output "${interrupt_evidence}" --run-id "${interrupt_run_id}" \
    --network "${AGENT_EGRESS_NETWORK:-agent-public-egress}" --timeout 15m \
    --instruction "Run ./scripts/long-command.sh now and wait for it to finish. Do not modify files." &
  interrupt_pid=$!
  set -e
  interrupt_container="$(wait_for_container "${interrupt_run_id}")"
  deadline=$((SECONDS + 300))
  until docker logs "${interrupt_container}" 2>&1 | rg -q 'long command started'; do
    if ! kill -0 "${interrupt_pid}" 2>/dev/null; then
      echo "${runtime} exited before starting the interrupt command" >&2
      wait "${interrupt_pid}" || true
      exit 1
    fi
    if ((SECONDS >= deadline)); then
      echo "${runtime} did not start the interrupt command" >&2
      kill -TERM "${interrupt_pid}" 2>/dev/null || true
      wait "${interrupt_pid}" || true
      exit 1
    fi
    sleep 2
  done
  kill -INT "${interrupt_pid}"
  if wait "${interrupt_pid}"; then
    echo "${runtime} interrupt scenario unexpectedly succeeded" >&2
    exit 1
  fi
  jq -e '.error_code == "interrupted"' "${interrupt_evidence}/report.json" >/dev/null
  if docker ps --all --filter "label=agent-platform.run-id=${interrupt_run_id}" --format '{{.ID}}' | rg -q .; then
    echo "${runtime} container survived interrupt cleanup" >&2
    exit 1
  fi
done

"${repo_root}/scripts/conformance/minio-local.sh" >"${suite_root}/minio.log" 2>&1
go test -count=1 -v ./internal/objectstore/aliyunoss >"${suite_root}/aliyun-oss.log" 2>&1

jq -s '{generated_at: now | todate, decision: "PENDING_SNAPSHOT_AND_TIMEOUT", runtimes: .}' \
  "${suite_root}"/{claude,codex,hermes,openclaw}/recovery/report.json >"${suite_root}/summary.json"

echo "Production Conformance passed; evidence: ${suite_root}"
