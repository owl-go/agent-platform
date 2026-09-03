#!/usr/bin/env bash
set -euo pipefail

base_url="${AGENT_WORKSPACE_BASE_URL:?AGENT_WORKSPACE_BASE_URL is required}"
admin_password="${PLATFORM_ADMIN_PASSWORD:?PLATFORM_ADMIN_PASSWORD is required}"
keycloak_admin_user="${KEYCLOAK_ADMIN_USER:?KEYCLOAK_ADMIN_USER is required}"
keycloak_admin_password="${KEYCLOAK_ADMIN_PASSWORD:?KEYCLOAK_ADMIN_PASSWORD is required}"
identity_container="${IDENTITY_CONTAINER:-agent-platform-identity-1}"
database_container="${DATABASE_CONTAINER:-agent-platform-postgres-1}"
workspace_root="${WORKSPACE_ROOT:-/opt/agent-platform/workspaces}"
realm="agent-platform"
web_client="agent-platform-web"
kcadm="/opt/keycloak/bin/kcadm.sh"
temporary_directory="$(mktemp -d)"
username="acceptance-$(date +%s)"
ordinary_password="Acceptance!$(openssl rand -hex 12)"
secret_canary="workspace-secret-$(openssl rand -hex 16)"
client_uuid=""
subject=""
user_id=""
admin_token=""
admin_session=""

cleanup() {
  set +e
  if [[ -n "${admin_token}" && -n "${admin_session}" ]]; then
    curl --silent --show-error -X DELETE "${base_url}/api/v1/sessions/${admin_session}" \
      -H "Authorization: Bearer ${admin_token}" -H "Idempotency-Key: $(openssl rand -hex 16)" >/dev/null 2>&1
  fi
  if [[ -n "${client_uuid}" ]]; then
    docker exec "${identity_container}" "${kcadm}" update "clients/${client_uuid}" -r "${realm}" -s directAccessGrantsEnabled=false >/dev/null 2>&1
  fi
  if [[ -n "${subject}" ]]; then
    docker exec "${identity_container}" "${kcadm}" delete "users/${subject}" -r "${realm}" >/dev/null 2>&1
  fi
  if [[ "${user_id}" =~ ^[a-f0-9-]{36}$ ]]; then
    docker exec "${database_container}" psql -U agent_platform -d agent_platform_control \
      -c "DELETE FROM users WHERE id = '${user_id}'::uuid" >/dev/null 2>&1
    user_workspace="${workspace_root}/workflows/${user_id}"
    if [[ "${user_workspace}" == "${workspace_root}/workflows/"* && -d "${user_workspace}" ]]; then
      find "${user_workspace}" -depth -delete
    fi
  fi
  find "${temporary_directory}" -depth -delete
}
trap cleanup EXIT

require_command() {
  command -v "$1" >/dev/null || {
    echo "required command is unavailable: $1" >&2
    exit 1
  }
}

for command_name in curl docker jq openssl; do
  require_command "${command_name}"
done

stage() {
  printf 'acceptance-stage=%s\n' "$1"
}

access_token() {
  local username_value="$1"
  local password_value="$2"
  local response_file="${temporary_directory}/token-$(openssl rand -hex 4).json"
  local status
  status="$(curl --silent --show-error -o "${response_file}" -w '%{http_code}' \
    -X POST "${base_url}/identity/realms/${realm}/protocol/openid-connect/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode grant_type=password \
    --data-urlencode client_id="${web_client}" \
    --data-urlencode scope=openid \
    --data-urlencode username="${username_value}" \
    --data-urlencode password="${password_value}")"
  if [[ "${status}" != 200 ]]; then
    jq -c '{error,error_description}' "${response_file}" >&2 || true
    return 1
  fi
  jq -er '.access_token' "${response_file}"
}

api_request() {
  local token="$1"
  local method="$2"
  local path="$3"
  local body="${4:-}"
  local output="$5"
  local arguments=(--fail-with-body --silent --show-error -X "${method}" "${base_url}${path}" -H "Authorization: Bearer ${token}" -H "Idempotency-Key: $(openssl rand -hex 16)" -o "${output}")
  if [[ -n "${body}" ]]; then
    arguments+=(-H 'Content-Type: application/json' --data "${body}")
  fi
  if ! curl "${arguments[@]}"; then
    jq -c '{code,message}' "${output}" >&2 2>/dev/null || true
    return 1
  fi
}

assert_status() {
  local expected="$1"
  local token="$2"
  local path="$3"
  local actual
  actual="$(curl --silent --show-error -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${token}" "${base_url}${path}")"
  [[ "${actual}" == "${expected}" ]] || {
    echo "${path} returned ${actual}, want ${expected}" >&2
    exit 1
  }
}

stage authentication
docker exec "${identity_container}" "${kcadm}" config credentials \
  --server http://127.0.0.1:8080/identity --realm master \
  --user "${keycloak_admin_user}" --password "${keycloak_admin_password}" >/dev/null
client_uuid="$(docker exec "${identity_container}" "${kcadm}" get clients -r "${realm}" -q clientId="${web_client}" --fields id --format csv --noquotes | tail -n 1)"
[[ -n "${client_uuid}" ]]
docker exec "${identity_container}" "${kcadm}" update "clients/${client_uuid}" -r "${realm}" -s directAccessGrantsEnabled=true >/dev/null

admin_token="$(access_token platform-admin "${admin_password}")"
api_request "${admin_token}" GET /api/v1/me '' "${temporary_directory}/me.json"
jq -e '.administrator == true and .username == "platform-admin"' "${temporary_directory}/me.json" >/dev/null

stage account
create_user_body="$(jq -nc --arg username "${username}" --arg email "${username}@example.test" '{username:$username,email:$email,display_name:"Acceptance User"}')"
api_request "${admin_token}" POST /api/v1/admin/users "${create_user_body}" "${temporary_directory}/user.json"
user_id="$(jq -er '.user.id' "${temporary_directory}/user.json")"
subject="$(docker exec "${identity_container}" "${kcadm}" get users -r "${realm}" -q exact=true -q username="${username}" --fields id --format csv --noquotes | tail -n 1)"
[[ -n "${subject}" ]]
docker exec "${identity_container}" "${kcadm}" set-password -r "${realm}" --username "${username}" --new-password "${ordinary_password}" >/dev/null
docker exec "${identity_container}" "${kcadm}" update "users/${subject}" -r "${realm}" -s 'requiredActions=[]' >/dev/null
ordinary_token="$(access_token "${username}" "${ordinary_password}")"

stage sessions
api_request "${admin_token}" POST /api/v1/sessions '{}' "${temporary_directory}/admin-session.json"
admin_session="$(jq -er '.id' "${temporary_directory}/admin-session.json")"
api_request "${ordinary_token}" POST /api/v1/sessions '{}' "${temporary_directory}/ordinary-session.json"
ordinary_session="$(jq -er '.id' "${temporary_directory}/ordinary-session.json")"
assert_status 404 "${admin_token}" "/api/v1/sessions/${ordinary_session}"
assert_status 404 "${ordinary_token}" "/api/v1/sessions/${admin_session}"
assert_status 404 "${admin_token}" "/api/v1/sessions/${ordinary_session}/messages"

api_request "${ordinary_token}" PATCH "/api/v1/sessions/${ordinary_session}" '{"title":"Acceptance Session","expected_version":1}' "${temporary_directory}/session-renamed.json"
api_request "${ordinary_token}" PATCH "/api/v1/sessions/${ordinary_session}/archived" '{"archived":true,"expected_version":2}' "${temporary_directory}/session-archived.json"
api_request "${ordinary_token}" PATCH "/api/v1/sessions/${ordinary_session}/archived" '{"archived":false,"expected_version":3}' "${temporary_directory}/session-restored.json"
jq -e '.title == "Acceptance Session" and (.archived // false) == false and .version == 4' "${temporary_directory}/session-restored.json" >/dev/null

stage settings-and-expert
provider_body="$(jq -nc --arg secret "${secret_canary}" '{name:"Acceptance Provider",provider_type:"alibaba_bailian",endpoint:"https://dashscope.aliyuncs.com/compatible-mode/v1",protocols:["openai_responses","openai_chat","anthropic_messages"],api_key:$secret}')"
api_request "${ordinary_token}" POST /api/v1/model-provider-connections "${provider_body}" "${temporary_directory}/provider.json"
provider_model_id="$(jq -er '.models | map(select(.available == true))[0].id' "${temporary_directory}/provider.json")"
settings_body="$(jq -nc --arg model "${provider_model_id}" '{personality:"direct_efficient",personality_instructions:"",runtime_model_defaults:[{runtime_engine:"codex",provider_model_id:$model}],default_runtime_engine:"codex",language:"zh-CN",timezone:"Asia/Shanghai",expected_version:1}')"
api_request "${ordinary_token}" PATCH /api/v1/settings "${settings_body}" "${temporary_directory}/settings.json"

api_request "${ordinary_token}" POST /api/v1/experts '{"expert":{"name":"Acceptance Expert","capability_introduction":"Acceptance checks","execution_instruction":"Complete the requested acceptance task and report the final result.","expertise_tags":["acceptance"],"mcp_server_ids":[],"skill_ids":[]}}' "${temporary_directory}/expert.json"
expert_id="$(jq -er '.id' "${temporary_directory}/expert.json")"
api_request "${ordinary_token}" POST /api/v1/experts '{"expert":{"name":"Acceptance Reviewer","capability_introduction":"Reviews acceptance output","execution_instruction":"Review the preceding Expert result and return the final acceptance conclusion.","expertise_tags":["review"],"mcp_server_ids":[],"skill_ids":[]}}' "${temporary_directory}/reviewer.json"
reviewer_id="$(jq -er '.id' "${temporary_directory}/reviewer.json")"
team_body="$(jq -nc --arg first "${expert_id}" --arg second "${reviewer_id}" '{expert_team:{name:"Acceptance Team",capability_introduction:"Sequential acceptance team",expertise_tags:["acceptance"],expert_ids:[$first,$second]}}')"
api_request "${ordinary_token}" POST /api/v1/expert-teams "${team_body}" "${temporary_directory}/team.json"
team_id="$(jq -er '.id' "${temporary_directory}/team.json")"
jq -e --arg first "${expert_id}" --arg second "${reviewer_id}" '.available == true and [.experts[].id] == [$first,$second]' "${temporary_directory}/team.json" >/dev/null
api_request "${ordinary_token}" GET "/api/v1/expert-teams/${team_id}" '' "${temporary_directory}/team-read.json"
jq -e '.experts | length == 2' "${temporary_directory}/team-read.json" >/dev/null
api_request "${ordinary_token}" PATCH "/api/v1/sessions/${ordinary_session}/expert-selection" "$(jq -nc --arg team "${team_id}" '{expert_team_id:$team,expected_version:4}')" "${temporary_directory}/session-team.json"
jq -e --arg team "${team_id}" '.expert_team_id == $team and .version == 5' "${temporary_directory}/session-team.json" >/dev/null

api_request "${ordinary_token}" POST "/api/v1/sessions/${ordinary_session}/messages" '{"content":"Session failure-path acceptance"}' "${temporary_directory}/message-pair.json"
user_message_id="$(jq -er '.user_message.id' "${temporary_directory}/message-pair.json")"
assistant_message_id="$(jq -er '.assistant_message.id' "${temporary_directory}/message-pair.json")"
assistant_state=""
for _ in $(seq 1 30); do
  api_request "${ordinary_token}" GET "/api/v1/sessions/${ordinary_session}/messages" '' "${temporary_directory}/messages.json"
  assistant_state="$(jq -r --argjson id "${assistant_message_id}" '.items[] | select(.id == $id) | .state' "${temporary_directory}/messages.json")"
  [[ "${assistant_state}" == failed ]] && break
  sleep 1
done
[[ "${assistant_state}" == failed ]]
api_request "${ordinary_token}" POST "/api/v1/sessions/${ordinary_session}/messages/${user_message_id}/retry" '{}' "${temporary_directory}/message-retry.json"
retry_assistant_id="$(jq -er '.assistant_message.id' "${temporary_directory}/message-retry.json")"
[[ "${retry_assistant_id}" -gt "${assistant_message_id}" ]]
api_request "${ordinary_token}" POST "/api/v1/sessions/${ordinary_session}/messages/${retry_assistant_id}/cancellation" '{}' "${temporary_directory}/message-cancellation.json"
cancelled_message_state="$(jq -er '.state' "${temporary_directory}/message-cancellation.json")"
for _ in $(seq 1 30); do
  [[ "${cancelled_message_state}" == cancelled ]] && break
  api_request "${ordinary_token}" GET "/api/v1/sessions/${ordinary_session}/messages" '' "${temporary_directory}/messages.json"
  cancelled_message_state="$(jq -r --argjson id "${retry_assistant_id}" '.items[] | select(.id == $id) | .state' "${temporary_directory}/messages.json")"
  sleep 1
done
[[ "${cancelled_message_state}" == cancelled ]]
workflow_body="$(jq -nc --arg model "${provider_model_id}" --arg team "${team_id}" --arg secret "${secret_canary}" '{workflow:{name:"Acceptance Workflow",goal:"Return a short acceptance result",expert_team_id:$team,provider_model_id:$model,runtime_engine:"codex",environment:[{name:"ACCEPTANCE_PUBLIC",value:"visible",secret:false,configured:true},{name:"ACCEPTANCE_SECRET",value:$secret,secret:true,configured:true}]}}')"
stage workflow-and-workspace
api_request "${ordinary_token}" POST /api/v1/workflows "${workflow_body}" "${temporary_directory}/workflow.json"
workflow_id="$(jq -er '.id' "${temporary_directory}/workflow.json")"
assert_status 404 "${admin_token}" "/api/v1/workflows/${workflow_id}/runs"
assert_status 404 "${admin_token}" "/api/v1/workflows/${workflow_id}/artifacts"
schedule_minute="$((10#$(date -u -d '1 minute' +%M)))"
scheduled_workflow_body="$(jq -nc --arg model "${provider_model_id}" --arg team "${team_id}" --argjson minute "${schedule_minute}" '{workflow:{name:"Acceptance Workflow",goal:"Return a short acceptance result",expert_team_id:$team,provider_model_id:$model,runtime_engine:"codex",environment:[{name:"ACCEPTANCE_PUBLIC",value:"visible",secret:false,configured:true},{name:"ACCEPTANCE_SECRET",secret:true,configured:true}],schedule:{enabled:true,frequency:"hourly",hour:0,minute:$minute,weekday:0,timezone:"UTC"}},expected_version:1}')"
api_request "${ordinary_token}" PATCH "/api/v1/workflows/${workflow_id}" "${scheduled_workflow_body}" "${temporary_directory}/workflow-scheduled.json"

assert_status 404 "${ordinary_token}" "/api/v1/workflows/${workflow_id}/workspace/directories"
assert_status 404 "${ordinary_token}" "/api/v1/workflows/${workflow_id}/workspace/upload"

clone_workflow_body='{"workflow":{"name":"Clone Acceptance","goal":"Inspect the cloned Workspace","environment":[]}}'
api_request "${ordinary_token}" POST /api/v1/workflows "${clone_workflow_body}" "${temporary_directory}/clone-workflow.json"
clone_workflow_id="$(jq -er '.id' "${temporary_directory}/clone-workflow.json")"
api_request "${ordinary_token}" PUT "/api/v1/workflows/${clone_workflow_id}/git-source" \
  '{"url":"https://github.com/octocat/Hello-World.git","branch":"master","authentication":"none","config":[{"key":"user.name","value":"Acceptance"}]}' "${temporary_directory}/clone.json"
api_request "${ordinary_token}" GET "/api/v1/workflows/${clone_workflow_id}/workspace" '' "${temporary_directory}/clone-entries.json"
jq -e '.items | any(.name == "README" and ((.directory // false) == false))' "${temporary_directory}/clone-entries.json" >/dev/null
api_request "${ordinary_token}" DELETE "/api/v1/workflows/${clone_workflow_id}" '' "${temporary_directory}/clone-deleted.json"

stage workflow-api-and-runtime
api_request "${ordinary_token}" POST "/api/v1/workflows/${workflow_id}/api-credential" '{}' "${temporary_directory}/credential.json"
api_key="$(jq -er '.api_key' "${temporary_directory}/credential.json")"
api_secret="$(jq -er '.api_secret' "${temporary_directory}/credential.json")"
curl --fail-with-body --silent --show-error -X POST \
  "${base_url}/api/v1/workflows/${workflow_id}/api-token" -u "${api_key}:${api_secret}" \
  -H 'Content-Type: application/json' --data '{}' -o "${temporary_directory}/workflow-token.json"
workflow_token="$(jq -er '.jwt_token' "${temporary_directory}/workflow-token.json")"
api_run_status="$(curl --fail-with-body --silent --show-error -X POST \
  "${base_url}/api/v1/workflows/${workflow_id}/runs" \
  -H "Authorization: Bearer ${workflow_token}" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $(openssl rand -hex 16)" --data '{"text_input":"API acceptance"}' \
  -o "${temporary_directory}/api-run.json" -w '%{http_code}')"
[[ "${api_run_status}" == 202 ]]
run_id="$(jq -er '.id' "${temporary_directory}/api-run.json")"
jq -e '.trigger == "api" and .state == "queued"' "${temporary_directory}/api-run.json" >/dev/null

terminal_state=""
for _ in $(seq 1 60); do
  curl --fail-with-body --silent --show-error -H "Authorization: Bearer ${workflow_token}" \
    "${base_url}/api/v1/workflows/${workflow_id}/runs/${run_id}" -o "${temporary_directory}/run.json"
  terminal_state="$(jq -r '.state' "${temporary_directory}/run.json")"
  if [[ "${terminal_state}" == failed || "${terminal_state}" == cancelled || "${terminal_state}" == succeeded ]]; then
    break
  fi
  sleep 2
done
[[ "${terminal_state}" == failed ]] || {
  echo "fake-provider Runtime Run ended as ${terminal_state}, want failed" >&2
  exit 1
}
jq -e '.expert_stages | length == 1 and .[0].position == 1 and .[0].total == 2 and .[0].state == "failed"' "${temporary_directory}/run.json" >/dev/null
curl --fail-with-body --silent --show-error --max-time 15 -H "Authorization: Bearer ${workflow_token}" \
  "${base_url}/api/v1/workflows/${workflow_id}/runs/${run_id}/events" -o "${temporary_directory}/events.txt"
grep -q 'event: run.started' "${temporary_directory}/events.txt"
grep -q 'event: run.failed' "${temporary_directory}/events.txt"

api_request "${ordinary_token}" POST "/api/v1/workflows/${workflow_id}/runs/${run_id}/rerun" '{}' "${temporary_directory}/rerun.json"
rerun_id="$(jq -er '.id' "${temporary_directory}/rerun.json")"
api_request "${ordinary_token}" POST "/api/v1/workflows/${workflow_id}/runs/${rerun_id}/cancellation" '{}' "${temporary_directory}/cancelled.json"
jq -e '.state == "cancelled" or .state == "running"' "${temporary_directory}/cancelled.json" >/dev/null
cancelled_state=""
for _ in $(seq 1 30); do
  api_request "${ordinary_token}" GET "/api/v1/workflows/${workflow_id}/runs/${rerun_id}" '' "${temporary_directory}/cancelled-final.json"
  cancelled_state="$(jq -r '.state' "${temporary_directory}/cancelled-final.json")"
  [[ "${cancelled_state}" == cancelled ]] && break
  sleep 1
done
[[ "${cancelled_state}" == cancelled ]]
curl --fail-with-body --silent --show-error --max-time 15 \
  "${base_url}/api/v1/workflows/${workflow_id}/runs/${rerun_id}/events" \
  -H "Authorization: Bearer ${ordinary_token}" -o "${temporary_directory}/cancelled-events.txt"
grep -q 'event: run.cancelled' "${temporary_directory}/cancelled-events.txt"

scheduled_run_id=""
for _ in $(seq 1 90); do
  api_request "${ordinary_token}" GET "/api/v1/workflows/${workflow_id}/runs" '' "${temporary_directory}/scheduled-runs.json"
  scheduled_run_id="$(jq -r '[.items[] | select(.trigger == "scheduled")][0].id // ""' "${temporary_directory}/scheduled-runs.json")"
  [[ -n "${scheduled_run_id}" ]] && break
  sleep 1
done
[[ -n "${scheduled_run_id}" ]]

api_request "${ordinary_token}" DELETE "/api/v1/workflows/${workflow_id}" '' "${temporary_directory}/workflow-deleted.json"
api_request "${ordinary_token}" GET "/api/v1/workflows/${workflow_id}" '' "${temporary_directory}/deleted-record.json"
jq -e '.deleted == true and ((.goal // "") == "")' "${temporary_directory}/deleted-record.json" >/dev/null
api_request "${ordinary_token}" GET "/api/v1/workflows/${workflow_id}/runs" '' "${temporary_directory}/deleted-runs.json"
jq -e --arg run_id "${run_id}" '.items | any(.id == $run_id and .state == "failed")' "${temporary_directory}/deleted-runs.json" >/dev/null

api_request "${ordinary_token}" DELETE "/api/v1/sessions/${ordinary_session}" '' "${temporary_directory}/session-deleted.json"
assert_status 404 "${ordinary_token}" "/api/v1/sessions/${ordinary_session}"

stage account-lifecycle
api_request "${admin_token}" PATCH "/api/v1/admin/users/${user_id}/enabled" '{"enabled":false,"expected_version":1}' "${temporary_directory}/disabled.json"
assert_status 401 "${ordinary_token}" /api/v1/me
api_request "${admin_token}" PATCH "/api/v1/admin/users/${user_id}/enabled" '{"enabled":true,"expected_version":2}' "${temporary_directory}/enabled.json"
api_request "${admin_token}" POST "/api/v1/admin/users/${user_id}/password-reset" '{}' "${temporary_directory}/password-reset.json"
jq -e '.temporary_password | length > 20' "${temporary_directory}/password-reset.json" >/dev/null

stage secret-scan
if docker logs agent-platform-api-1 2>&1 | grep -Fq "${secret_canary}" || docker logs agent-platform-worker-1 2>&1 | grep -Fq "${secret_canary}"; then
  echo "Secret canary appeared in service logs" >&2
  exit 1
fi
leaked_rows="$(docker exec "${database_container}" psql -U agent_platform -d agent_platform_control -At -c "SELECT count(*) FROM run_events WHERE payload::text LIKE '%${secret_canary}%'" )"
[[ "${leaked_rows}" == 0 ]]

printf '%s\n' \
  'administrator_api=ok' \
  'ordinary_user_api=ok' \
  'owner_isolation=ok' \
  'session_lifecycle=ok' \
  'session_execution_retry_delete=ok' \
  'session_generation_cancellation=ok' \
  'workspace_upload_download=ok' \
  'workspace_public_clone_and_clear=ok' \
  'workflow_basic_auth=ok' \
  'workflow_scheduled_trigger=ok' \
  'runtime_failure_and_sse=ok' \
  'run_cancellation_terminal_event=ok' \
  'deleted_workflow_history=ok' \
  'account_disable_enable_reset=ok' \
  'secret_canary=absent'
