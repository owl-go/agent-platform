#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
"${repo_root}/scripts/conformance/production-preflight.sh"

stamp="${CONFORMANCE_SUITE_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "${stamp}" =~ ^[A-Za-z0-9_.-]+$ ]] || {
  echo "CONFORMANCE_SUITE_ID contains unsupported characters" >&2
  exit 2
}
suite_root="${CONFORMANCE_EVIDENCE_ROOT}/${stamp}"
export CONFORMANCE_SUITE_ID="${stamp}"

cleanup_suite() {
  local status=$?
  local child_pids container run_id suite_id
  trap - EXIT INT TERM HUP

  child_pids="$(jobs -pr)"
  if [[ -n "${child_pids}" ]]; then
    kill -TERM ${child_pids} 2>/dev/null || true
  fi
  if command -v docker >/dev/null 2>&1; then
    while IFS= read -r container; do
      [[ -n "${container}" ]] || continue
      run_id="$(docker inspect --format '{{index .Config.Labels "agent-platform.run-id"}}' "${container}" 2>/dev/null || true)"
      suite_id="$(docker inspect --format '{{index .Config.Labels "agent-platform.conformance-suite"}}' "${container}" 2>/dev/null || true)"
      if [[ "${run_id}" == "${stamp}-"* || "${suite_id}" == "${stamp}" ]]; then
        docker rm --force --volumes "${container}" >/dev/null 2>&1 || true
      fi
    done < <(docker ps --all --quiet --filter label=agent-platform.managed=true 2>/dev/null)
  fi
  if [[ -n "${child_pids}" ]]; then
    wait ${child_pids} 2>/dev/null || true
  fi
  if ((status != 0)); then
    echo "Production Conformance failed; partial evidence retained at ${suite_root}" >&2
  fi
  exit "${status}"
}

trap cleanup_suite EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

mkdir -p "${CONFORMANCE_WORK_ROOT}"
mkdir "${suite_root}"
go -C backend build -o "${suite_root}/runtime-conformance" ./cmd/runtime-conformance
go -C backend build -o "${suite_root}/conformance-artifact" ./cmd/conformance-artifact

# Validate both storage providers before spending model budget.
go -C backend test -count=1 -v ./internal/objectstore/minio >"${suite_root}/minio.log" 2>&1
go -C backend test -count=1 -v ./internal/objectstore/aliyunoss >"${suite_root}/aliyun-oss.log" 2>&1

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

runtimes=(claude codex hermes openclaw pi)
for runtime in "${runtimes[@]}"; do
  upper="${runtime^^}"
  image_name="CONFORMANCE_${upper}_IMAGE"
  model_name="CONFORMANCE_${upper}_MODEL"
  credential_name="CONFORMANCE_${upper}_CREDENTIAL_DIR"
  image="${!image_name}"
  model="${!model_name}"
  credential_dir="${!credential_name}"
  canary="$(<"${credential_dir}/env/CONFORMANCE_CANARY_SECRET")"
  if [[ ${#canary} -lt 16 ]]; then
    echo "${credential_name} canary must contain at least 16 characters" >&2
    exit 2
  fi
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
    --resolver-config "${AGENT_RESOLVER_CONFIG_FILE}" \
    --timeout 15m \
    --instruction "Complete the task in CONFORMANCE.md. After scripts/test.sh passes, run scripts/long-command.sh as the final action and wait for it." &
  forced_pid=$!
  set -e
  forced_container="$(wait_for_container "${forced_run_id}")"
  deadline=$((SECONDS + 600))
  until "${workspace}/scripts/test.sh" >/dev/null 2>&1 && [[ -f "${workspace}/.conformance-long-started" ]]; do
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
  if rg --fixed-strings --quiet -- "${canary}" "${forced_evidence}" "${workspace}"; then
    echo "credential canary persisted before ${runtime} snapshot" >&2
    exit 1
  fi

  snapshot_key="phase-0/${stamp}/${runtime}/workspace.tar"
  "${suite_root}/conformance-artifact" --action upload --provider minio \
    --source "${workspace}" --key "${snapshot_key}" --report "${forced_evidence}/minio-upload.json"
  "${suite_root}/conformance-artifact" --action upload --provider aliyun_oss \
    --source "${workspace}" --key "${snapshot_key}" --report "${forced_evidence}/aliyun-oss-upload.json"

  recovery_workspace="${CONFORMANCE_WORK_ROOT}/${stamp}-${runtime}-recovery"
  oss_restore_workspace="${CONFORMANCE_WORK_ROOT}/${stamp}-${runtime}-oss-restore"
  prepare_owned_directory "${recovery_workspace}"
  prepare_owned_directory "${oss_restore_workspace}"
  "${suite_root}/conformance-artifact" --action restore --provider minio \
    --source "${recovery_workspace}" --key "${snapshot_key}" --report "${forced_evidence}/minio-restore.json"
  "${suite_root}/conformance-artifact" --action restore --provider aliyun_oss \
    --source "${oss_restore_workspace}" --key "${snapshot_key}" --report "${forced_evidence}/aliyun-oss-restore.json"
  if [[ "$(id -u)" == "0" ]]; then
    chown -R 65532:65532 "${recovery_workspace}" "${oss_restore_workspace}"
  fi
  "${oss_restore_workspace}/scripts/test.sh" >/dev/null
  minio_snapshot_sha="$(jq -r '.sha256' "${forced_evidence}/minio-restore.json")"
  oss_snapshot_sha="$(jq -r '.sha256' "${forced_evidence}/aliyun-oss-restore.json")"
  [[ "${minio_snapshot_sha}" == "${oss_snapshot_sha}" ]] || {
    echo "MinIO and Aliyun OSS restored different ${runtime} snapshots" >&2
    exit 1
  }
  workspace="${recovery_workspace}"

  "${suite_root}/runtime-conformance" \
    --runtime "${runtime}" \
    --image "${image}" \
    --model "${model}" \
    --workspace "${workspace}" \
    --credentials "${credential_dir}" \
    --output "${evidence}" \
    --run-id "${stamp}-${runtime}" \
    --network "${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    --resolver-config "${AGENT_RESOLVER_CONFIG_FILE}" \
    --instruction "$(<"${repo_root}/testdata/production-conformance/task.txt")"

  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" test \
    "${image}" "${workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${review_branch}"
  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" push \
    "${image}" "${workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${review_branch}"

  if rg --fixed-strings --quiet -- "${canary}" "${evidence}" "${workspace}"; then
    echo "credential canary persisted for ${runtime}" >&2
    exit 1
  fi

  for control in interrupt cancel; do
    control_workspace="${CONFORMANCE_WORK_ROOT}/${stamp}-${runtime}-${control}"
    control_evidence="${evidence_root}/${control}"
    control_run_id="${stamp}-${runtime}-${control}"
    control_signal="INT"
    [[ "${control}" == "cancel" ]] && control_signal="TERM"
    prepare_owned_directory "${control_workspace}"
    mkdir -p "${control_evidence}"
    chown 65532:65532 "${control_evidence}" 2>/dev/null || true
    AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
      "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" clone \
      "${image}" "${control_workspace}" "${credential_dir}" \
      "${CONFORMANCE_REPOSITORY_URL}" "${CONFORMANCE_BASE_BRANCH}"

    set +e
    "${suite_root}/runtime-conformance" \
      --runtime "${runtime}" --image "${image}" --model "${model}" \
      --workspace "${control_workspace}" --credentials "${credential_dir}" \
      --output "${control_evidence}" --run-id "${control_run_id}" \
      --network "${AGENT_EGRESS_NETWORK:-agent-public-egress}" --timeout 15m \
      --resolver-config "${AGENT_RESOLVER_CONFIG_FILE}" \
      --instruction "Run ./scripts/long-command.sh now and wait for it to finish. Do not modify files." &
    control_pid=$!
    set -e
    wait_for_container "${control_run_id}" >/dev/null
    deadline=$((SECONDS + 300))
    until [[ -f "${control_workspace}/.conformance-long-started" ]]; do
      if ! kill -0 "${control_pid}" 2>/dev/null; then
        echo "${runtime} exited before starting the ${control} command" >&2
        wait "${control_pid}" || true
        exit 1
      fi
      if ((SECONDS >= deadline)); then
        echo "${runtime} did not start the ${control} command" >&2
        kill -TERM "${control_pid}" 2>/dev/null || true
        wait "${control_pid}" || true
        exit 1
      fi
      sleep 2
    done
    kill -"${control_signal}" "${control_pid}"
    if wait "${control_pid}"; then
      echo "${runtime} ${control} scenario unexpectedly succeeded" >&2
      exit 1
    fi
    jq -e '.error_code == "interrupted"' "${control_evidence}/report.json" >/dev/null
    if docker ps --all --filter "label=agent-platform.run-id=${control_run_id}" --format '{{.ID}}' | rg -q .; then
      echo "${runtime} container survived ${control} cleanup" >&2
      exit 1
    fi
  done

  timeout_workspace="${CONFORMANCE_WORK_ROOT}/${stamp}-${runtime}-timeout"
  timeout_evidence="${evidence_root}/timeout"
  timeout_run_id="${stamp}-${runtime}-timeout"
  prepare_owned_directory "${timeout_workspace}"
  mkdir -p "${timeout_evidence}"
  chown 65532:65532 "${timeout_evidence}" 2>/dev/null || true
  AGENT_EGRESS_NETWORK="${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    "${repo_root}/scripts/conformance/runtime-git-sandbox.sh" clone \
    "${image}" "${timeout_workspace}" "${credential_dir}" \
    "${CONFORMANCE_REPOSITORY_URL}" "${CONFORMANCE_BASE_BRANCH}"
  if "${suite_root}/runtime-conformance" \
    --runtime "${runtime}" --image "${image}" --model "${model}" \
    --workspace "${timeout_workspace}" --credentials "${credential_dir}" \
    --output "${timeout_evidence}" --run-id "${timeout_run_id}" \
    --network "${AGENT_EGRESS_NETWORK:-agent-public-egress}" \
    --resolver-config "${AGENT_RESOLVER_CONFIG_FILE}" \
    --timeout "${CONFORMANCE_TIMEOUT_DURATION:-5m}" \
    --instruction "Run ./scripts/long-command.sh now and wait for it to finish. Do not modify files."; then
    echo "${runtime} timeout scenario unexpectedly succeeded" >&2
    exit 1
  fi
  jq -e '.error_code == "timed_out"' "${timeout_evidence}/report.json" >/dev/null
  [[ -f "${timeout_workspace}/.conformance-long-started" ]] || {
    echo "${runtime} timed out before starting the fixture long command" >&2
    exit 1
  }
  if docker ps --all --filter "label=agent-platform.run-id=${timeout_run_id}" --format '{{.ID}}' | rg -q .; then
    echo "${runtime} container survived timeout cleanup" >&2
    exit 1
  fi

  if rg --fixed-strings --quiet -- "${canary}" "${evidence_root}"; then
    echo "credential canary persisted in ${runtime} evidence" >&2
    exit 1
  fi
  jq -n \
    --arg runtime "${runtime}" --arg image "${image}" --arg review_branch "${review_branch}" \
    --slurpfile forced "${forced_evidence}/report.json" \
    --slurpfile recovery "${evidence}/report.json" \
    --slurpfile interrupt "${evidence_root}/interrupt/report.json" \
    --slurpfile cancel "${evidence_root}/cancel/report.json" \
    --slurpfile timeout "${evidence_root}/timeout/report.json" \
    --slurpfile minio_snapshot "${forced_evidence}/minio-restore.json" \
    --slurpfile oss_snapshot "${forced_evidence}/aliyun-oss-restore.json" \
    '{runtime: $runtime, image: $image, review_branch: $review_branch,
      scenarios: {forced_kill: $forced[0], recovery: $recovery[0], interrupt: $interrupt[0], cancel: $cancel[0], timeout: $timeout[0]},
      snapshots: {minio: $minio_snapshot[0], aliyun_oss: $oss_snapshot[0]}}' \
    >"${evidence_root}/scenario-summary.json"

  artifact_report_root="${suite_root}/artifact-reports/${runtime}"
  mkdir -p "${artifact_report_root}"
  evidence_key="phase-0/${stamp}/${runtime}/evidence.tar"
  "${suite_root}/conformance-artifact" --action upload --provider minio \
    --source "${evidence_root}" --key "${evidence_key}" --report "${artifact_report_root}/minio-evidence.json"
  "${suite_root}/conformance-artifact" --action upload --provider aliyun_oss \
    --source "${evidence_root}" --key "${evidence_key}" --report "${artifact_report_root}/aliyun-oss-evidence.json"
done

jq -s '{generated_at: now | todate, decision: "GO", runtimes: .}' \
  "${suite_root}"/{claude,codex,hermes,openclaw,pi}/scenario-summary.json >"${suite_root}/summary.json"

echo "Production Conformance passed; evidence: ${suite_root}"
