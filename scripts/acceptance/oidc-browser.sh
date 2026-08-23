#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
postgres_container="agent-platform-oidc-acceptance-postgres"
keycloak_container="agent-platform-oidc-acceptance-keycloak"
minio_container="agent-platform-oidc-acceptance-minio"
playwright_session="agent-platform-oidc-acceptance-$$"
acceptance_tmp="$(mktemp -d)"
model_secret_canary="model-secret-$(openssl rand -hex 24)"
model_secret_canary_left="${model_secret_canary:0:25}"
model_secret_canary_right="${model_secret_canary:25}"
git_private_key_canary="git-private-key-$(openssl rand -hex 24)"
known_hosts_canary="known-hosts-$(openssl rand -hex 24)"
build_secret_canary="build-secret-$(openssl rand -hex 24)"
git_private_key_canary_left="${git_private_key_canary:0:26}"
git_private_key_canary_right="${git_private_key_canary:26}"
known_hosts_canary_left="${known_hosts_canary:0:24}"
known_hosts_canary_right="${known_hosts_canary:24}"
build_secret_canary_left="${build_secret_canary:0:23}"
build_secret_canary_right="${build_secret_canary:23}"
secret_canaries=("$model_secret_canary" "$git_private_key_canary" "$known_hosts_canary" "$build_secret_canary")
api_pid=""
web_pid=""
runtime_approval_browser_pid=""
pwcli="${PWCLI:-playwright-cli}"

cleanup() {
  cleanup_status=$?
  if [[ "$cleanup_status" -ne 0 ]]; then
    [[ -f "$acceptance_tmp/api.log" ]] && tail -n 120 "$acceptance_tmp/api.log" | redact_canaries >&2
    [[ -f "$acceptance_tmp/web.log" ]] && tail -n 40 "$acceptance_tmp/web.log" | redact_canaries >&2
  fi
  "$pwcli" --session "$playwright_session" close >/dev/null 2>&1 || true
  if [[ -n "$web_pid" ]]; then
    kill "$web_pid" >/dev/null 2>&1 || true
    wait "$web_pid" 2>/dev/null || true
  fi
  if [[ -n "$api_pid" ]]; then
    kill "$api_pid" >/dev/null 2>&1 || true
    wait "$api_pid" 2>/dev/null || true
  fi
  if [[ -n "$runtime_approval_browser_pid" ]]; then
    kill "$runtime_approval_browser_pid" >/dev/null 2>&1 || true
    wait "$runtime_approval_browser_pid" 2>/dev/null || true
  fi
  docker rm -f "$keycloak_container" "$postgres_container" "$minio_container" >/dev/null 2>&1 || true
  rm -rf "$acceptance_tmp"
}
trap cleanup EXIT

redact_canaries() {
  sed -e "s/$model_secret_canary/[REDACTED]/g" -e "s/$git_private_key_canary/[REDACTED]/g" -e "s/$known_hosts_canary/[REDACTED]/g" -e "s/$build_secret_canary/[REDACTED]/g" "$@"
}

if docker ps -a --format '{{.Names}}' | grep -Fxq "$postgres_container" ||
   docker ps -a --format '{{.Names}}' | grep -Fxq "$keycloak_container" ||
   docker ps -a --format '{{.Names}}' | grep -Fxq "$minio_container"; then
  echo "OIDC browser acceptance container names are already in use" >&2
  exit 1
fi
if ! command -v "$pwcli" >/dev/null 2>&1 && [[ ! -x "$pwcli" ]]; then
  echo "Set PWCLI to the Playwright CLI wrapper or install playwright-cli" >&2
  exit 1
fi

browser() {
  if ! "$pwcli" "$@" >>"$acceptance_tmp/browser.log" 2>&1; then
    tail -n 120 "$acceptance_tmp/browser.log" | redact_canaries >&2
    return 1
  fi
}

docker run -d --name "$postgres_container" \
  -e POSTGRES_DB=agent_platform_oidc \
  -e POSTGRES_USER=agent_platform \
  -e POSTGRES_PASSWORD=acceptance-db-password \
  -p 127.0.0.1:15432:5432 \
  postgres@sha256:ef257d85f76e48da1c64832459b59fcaba1a4dac97bf5d7450c77753542eee94 >/dev/null

docker run -d --name "$keycloak_container" \
  -e KC_BOOTSTRAP_ADMIN_USERNAME=admin \
  -e KC_BOOTSTRAP_ADMIN_PASSWORD=acceptance-admin-password \
  -p 127.0.0.1:18091:8080 \
  -v "$repository_root/backend/testdata/oidc-browser/keycloak-realm.json:/opt/keycloak/data/import/agent-platform-realm.json:ro" \
  quay.io/keycloak/keycloak@sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6 \
  start-dev --import-realm >/dev/null

docker run -d --name "$minio_container" \
  -e MINIO_ROOT_USER=acceptance \
  -e MINIO_ROOT_PASSWORD=acceptance-only-secret \
  -p 127.0.0.1:19000:9000 \
  minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e \
  server /data >/dev/null

for _ in {1..90}; do
  if docker exec "$postgres_container" pg_isready -U agent_platform -d agent_platform_oidc >/dev/null 2>&1 &&
     curl -fsS http://127.0.0.1:18091/realms/agent-platform/.well-known/openid-configuration >/dev/null 2>&1 &&
     curl -fsS http://127.0.0.1:19000/minio/health/ready >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$postgres_container" pg_isready -U agent_platform -d agent_platform_oidc >/dev/null
curl -fsS http://127.0.0.1:18091/realms/agent-platform/.well-known/openid-configuration >/dev/null
curl -fsS http://127.0.0.1:19000/minio/health/ready >/dev/null

MINIO_ENDPOINT=127.0.0.1:19000 \
MINIO_ACCESS_KEY=acceptance \
MINIO_SECRET_KEY=acceptance-only-secret \
MINIO_BUCKET=acceptance \
MINIO_SECURE=false \
MINIO_CREATE_BUCKET=true \
go -C "$repository_root/backend" test -count=1 -run '^TestProviderConformance$' ./internal/objectstore/minio

evidence_source="$acceptance_tmp/evidence"
mkdir -p "$evidence_source"
evidence_digest="registry.example/codex@sha256:$(printf 'c%.0s' {1..64})"
snapshot_sha="$(printf 'a%.0s' {1..64})"
jq -n \
  --arg runtime claude --arg version 1.0.0 --arg image "$evidence_digest" --arg snapshot_sha "$snapshot_sha" \
  'def report: {runtime: {name: $runtime, version: $version, capabilities: {streaming: false, structured_final: false, native_resume: false, subagents: false, usage: false}}, image: $image};
   {runtime: $runtime, image: $image, review_branch: "conformance/acceptance/codex",
    scenarios: {forced_kill: (report + {error_code: "execution_failed", error: "container terminated"}), recovery: (report + {result: {exit_code: 0}}), interrupt: (report + {error_code: "interrupted"}), cancel: (report + {error_code: "interrupted"}), timeout: (report + {error_code: "timed_out"})},
    snapshots: {minio: {action: "restored", provider: "minio", key: "phase-0/acceptance/workspace.tar", size: 1024, sha256: $snapshot_sha}, aliyun_oss: {action: "restored", provider: "aliyun_oss", key: "phase-0/acceptance/workspace.tar", size: 1024, sha256: $snapshot_sha}}}' \
  >"$evidence_source/scenario-summary.json"
MINIO_ENDPOINT=127.0.0.1:19000 \
MINIO_ACCESS_KEY=acceptance \
MINIO_SECRET_KEY=acceptance-only-secret \
MINIO_BUCKET=acceptance \
MINIO_SECURE=false \
go -C "$repository_root/backend" run ./cmd/conformance-artifact \
  --action upload --provider minio --source "$evidence_source" \
  --key phase-0/acceptance/codex/evidence.tar --report "$acceptance_tmp/evidence-upload.json"
artifact_size="$(jq -r '.size' "$acceptance_tmp/evidence-upload.json")"
artifact_sha256="$(jq -r '.sha256' "$acceptance_tmp/evidence-upload.json")"
run_event_payload="$(RUN_EVENT_SECRET_CANARY="$model_secret_canary" go -C "$repository_root/backend" run ./testdata/oidc-browser/redact-event)"
if [[ "$run_event_payload" == *"$model_secret_canary"* ]] || [[ "$run_event_payload" != *"[REDACTED]"* ]]; then
  echo "Run Event acceptance fixture was not redacted" >&2
  exit 1
fi
run_event_payload_base64="$(printf '%s' "$run_event_payload" | base64 | tr -d '\n')"

go -C "$repository_root/backend" build -o "$acceptance_tmp/api" ./cmd/api

(
  cd "$repository_root/backend"
  export OIDC_BROWSER_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable'
  export MODEL_SECRET_CANARY="$model_secret_canary"
  export GIT_PRIVATE_KEY_CANARY="$git_private_key_canary"
  export GIT_KNOWN_HOSTS_CANARY="$known_hosts_canary"
  export BUILD_SECRET_CANARY="$build_secret_canary"
  exec "$acceptance_tmp/api" -config testdata/oidc-browser/platform.yaml
) >"$acceptance_tmp/api.log" 2>&1 &
api_pid=$!

for _ in {1..60}; do
  curl -fsS http://127.0.0.1:18090/readyz >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS http://127.0.0.1:18090/readyz >/dev/null; then
  redact_canaries "$acceptance_tmp/api.log" >&2
  exit 1
fi

EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
  go -C "$repository_root/backend" test -count=1 -run '^TestRepositorySerializesCredentialDisableAndModelEnable$' ./internal/data/modelcatalog/gormrepo
EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
  go -C "$repository_root/backend" test -count=1 -run '^TestAgentLifecyclePersistsValidatedLowAndHighRiskReleases$' ./internal/data/agentlifecycle/gormrepo
EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
  go -C "$repository_root/backend" test -count=1 -run '^(TestCollaborationLaunchContinueAndMemoryAreTransactional|TestCodingTaskLaunchSerializesBuildCredentialDisable)$' ./internal/data/collaboration/gormrepo
EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
  go -C "$repository_root/backend" test -count=1 -run '^TestGORMRepositoryRunLifecycle$' ./internal/data/execution/gormrepo

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "INSERT INTO organizations (id, slug, name) VALUES ('22222222-2222-4222-8222-222222222222', 'acme', 'Acme');
   INSERT INTO organizations (id, slug, name) VALUES ('77777777-7777-4777-8777-777777777777', 'other', 'Other Organization');
   INSERT INTO teams (id, organization_id, slug, name) VALUES
     ('55555555-5555-4555-8555-555555555555', '22222222-2222-4222-8222-222222222222', 'platform', 'Platform Team'),
     ('66666666-6666-4666-8666-666666666666', '22222222-2222-4222-8222-222222222222', 'runtime', 'Runtime Team');
   INSERT INTO users (id, organization_id, oidc_subject, email, display_name) VALUES
     ('33333333-3333-4333-8333-333333333333', '22222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111', 'platform-user@example.test', 'Platform User'),
     ('34343434-3434-4434-8434-343434343434', '22222222-2222-4222-8222-222222222222', '12121212-1212-4212-8212-121212121212', 'release-reviewer@example.test', 'Release Reviewer');
   INSERT INTO role_grants (id, organization_id, team_id, user_id, role) VALUES
     ('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222', NULL, '33333333-3333-4333-8333-333333333333', 'platform_administrator'),
     ('45454545-4545-4545-8545-454545454545', '22222222-2222-4222-8222-222222222222', '66666666-6666-4666-8666-666666666666', '34343434-3434-4434-8434-343434343434', 'agent_builder');
   INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES ('99999999-9999-4999-8999-999999999999', '22222222-2222-4222-8222-222222222222', '55555555-5555-4555-8555-555555555555', 'hidden-team-model-key', 'model', 'vault://acceptance/hidden-team-model');
   INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', '22222222-2222-4222-8222-222222222222', '66666666-6666-4666-8666-666666666666', 'acceptance-git-ssh', 'git_ssh', 'env://GIT_PRIVATE_KEY_CANARY');
   INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', '22222222-2222-4222-8222-222222222222', '66666666-6666-4666-8666-666666666666', 'acceptance-build', 'build', 'env://BUILD_SECRET_CANARY');
   INSERT INTO runtime_images (id, organization_id, runtime, cli_version, adapter_version, image_digest) VALUES ('88888888-8888-4888-8888-888888888888', '77777777-7777-4777-8777-777777777777', 'claude', 'private', 'private', 'registry.example/private@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff');" >/dev/null

(
  cd "$repository_root/frontend"
  export VITE_OIDC_AUTHORITY='http://127.0.0.1:18091/realms/agent-platform'
  export VITE_OIDC_CLIENT_ID='agent-platform-web'
  export VITE_OIDC_REDIRECT_URI='http://127.0.0.1:18092/auth/callback'
  export VITE_OIDC_POST_LOGOUT_REDIRECT_URI='http://127.0.0.1:18092'
  export VITE_API_PROXY_TARGET='http://127.0.0.1:18090'
  exec pnpm dev --host 127.0.0.1 --port 18092
) >"$acceptance_tmp/web.log" 2>&1 &
web_pid=$!

for _ in {1..60}; do
  curl -fsS http://127.0.0.1:18092 >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS http://127.0.0.1:18092 >/dev/null; then
  redact_canaries "$acceptance_tmp/web.log" >&2
  exit 1
fi

browser --session "$playwright_session" open http://127.0.0.1:18092
browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("platform-user"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=55555555/); const user = page.getByTestId("current-user"); const text = await user.textContent(); if (!text.includes("Platform User") || !text.includes("Acme") || !text.includes("Platform Team")) throw new Error("real User and Team bootstrap was not rendered"); await page.getByTestId("team-select").selectOption("66666666-6666-4666-8666-666666666666"); await page.waitForURL(/team=66666666/); await page.getByTestId("nav-operations").click(); await page.waitForURL(/operations/); }'
browser --session "$playwright_session" run-code 'async (page) => { const digest = "registry.example/codex@sha256:" + "c".repeat(64); await page.getByTestId("nav-studio").click(); await page.waitForURL(/studio/); await page.getByTestId("register-runtime").click(); await page.getByTestId("runtime-cli-version").fill("1.0.0"); await page.getByTestId("runtime-adapter-version").fill("2026.08"); await page.getByTestId("runtime-digest").fill(digest); await page.getByTestId("submit-runtime").click(); await page.getByTestId("runtime-detail").waitFor(); if (!(await page.getByTestId("runtime-detail").textContent()).includes(digest)) throw new Error("registered Runtime Image was not rendered from the API"); await page.getByTestId("runtime-status").selectOption("production"); await page.getByTestId("conformance-evidence-key").fill("phase-0/acceptance/codex/evidence.tar"); await page.getByTestId("save-runtime-status").click(); await page.getByTestId("runtime-notice").waitFor(); const runtimeButton = page.locator(".catalog-list > button").first(); const runtimeID = (await runtimeButton.getAttribute("data-testid")).replace("runtime-", ""); const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); if (!accessToken) throw new Error("OIDC access token was not available in session storage"); const invalidEvidence = await page.evaluate(async ({ accessToken, runtimeID }) => fetch(`/api/v1/runtime-images/${runtimeID}/status`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-invalid-evidence", "If-Match": "2" }, body: JSON.stringify({ status: "production", conformance_evidence_key: "arbitrary/missing.tar" }) }).then(async (response) => ({ status: response.status, body: await response.json() })), { accessToken, runtimeID }); if (invalidEvidence.status !== 422 || invalidEvidence.body.error !== "invalid_conformance_evidence") throw new Error("unverified evidence key certified Production"); const crossOrganization = await page.evaluate(async (accessToken) => fetch("/api/v1/runtime-images/88888888-8888-4888-8888-888888888888", { headers: { Authorization: `Bearer ${accessToken}` } }).then((response) => response.status), accessToken); if (crossOrganization !== 404 || (await page.locator("body").textContent()).includes("registry.example/private")) throw new Error("cross-Organization Runtime Image was visible"); const replayDigest = "registry.example/hermes@sha256:" + "d".repeat(64); const replayBody = JSON.stringify({ runtime: "hermes", cli_version: "2.0.0", adapter_version: "2026.08", image_digest: replayDigest, capabilities: { streaming: true } }); const headers = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-replay-intent" }; const first = await page.evaluate(async ({ headers, replayBody }) => { const response = await fetch("/api/v1/runtime-images", { method: "POST", headers, body: replayBody }); return { status: response.status, body: await response.json() }; }, { headers, replayBody }); const replay = await page.evaluate(async ({ headers, replayBody }) => { const response = await fetch("/api/v1/runtime-images", { method: "POST", headers, body: replayBody }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { headers, replayBody }); if (first.status !== 201 || replay.status !== 201 || replay.replayed !== "true" || first.body.id !== replay.body.id) throw new Error("idempotent Runtime Image registration was not replayed"); const conflict = await page.evaluate(async ({ accessToken, runtimeID }) => fetch(`/api/v1/runtime-images/${runtimeID}/status`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-stale-intent", "If-Match": "1" }, body: JSON.stringify({ status: "blocked", blocked_reason: "stale update" }) }).then(async (response) => ({ status: response.status, body: await response.json() })), { accessToken, runtimeID }); if (conflict.status !== 412 || conflict.body.error !== "version_conflict") throw new Error("stale Runtime Image Version was not rejected"); await page.reload(); await page.getByTestId("runtime-detail").waitFor(); if (!(await page.locator("body").textContent()).includes("Production Runtime") || !(await page.locator("body").textContent()).includes(digest)) throw new Error("Runtime Catalog did not persist across refresh"); }'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE runtime_images SET capabilities = jsonb_set(capabilities, '{streaming}', 'true') WHERE organization_id = '22222222-2222-4222-8222-222222222222';" >/dev/null

browser --session "$playwright_session" run-code 'async (page) => {
  const modelSecret = "'"$model_secret_canary_left"'" + "'"$model_secret_canary_right"'";
  const consoleMessages = [];
  const recordConsole = (message) => consoleMessages.push(message.text());
  page.on("console", recordConsole);
  if ((await page.locator("body").textContent()).includes("hidden-team-model-key")) throw new Error("Team-scoped Credential Profile metadata crossed the Model Catalog boundary");
  await page.getByTestId("register-credential").click();
  await page.getByTestId("credential-name").fill("acceptance-model-key");
  await page.getByTestId("credential-secret-ref").fill("env://MODEL_SECRET_CANARY");
  await page.getByTestId("submit-credential").click();
  await page.getByTestId("model-notice").waitFor();
  if ((await page.locator("body").textContent()).includes("env://MODEL_SECRET_CANARY")) throw new Error("Secret reference was echoed by the Model Catalog");
  await page.getByTestId("register-model").click();
  await page.getByTestId("model-name").fill("acceptance-primary");
  await page.getByTestId("model-id").fill("acceptance-model-v1");
  await page.getByTestId("model-endpoint").fill("https://models.example.test/v1");
  await page.getByTestId("model-credential").selectOption({ label: "acceptance-model-key" });
  await page.getByTestId("submit-model").click();
  await page.getByTestId("model-notice").waitFor();
  const credentialRecord = page.locator(".catalog-record[data-testid^=credential-]").filter({ hasText: "acceptance-model-key" });
  const modelRecord = page.locator(".catalog-record[data-testid^=model-]").filter({ hasText: "acceptance-primary" });
  const credentialID = (await credentialRecord.getAttribute("data-testid")).replace("credential-", "");
  const modelID = (await modelRecord.getAttribute("data-testid")).replace("model-", "");
  await page.getByTestId(`toggle-credential-${credentialID}`).click();
  await page.getByTestId("model-notice").waitFor();
  if (!(await page.getByTestId(`model-${modelID}`).textContent()).includes("Disabled") && !(await page.getByTestId(`model-${modelID}`).textContent()).includes("已禁用")) throw new Error("disabling a Credential Profile did not disable its Configured Model");
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const rejected = await page.evaluate(async ({ accessToken, modelID }) => { const response = await fetch(`/api/v1/configured-models/${modelID}/status`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "disabled-credential-model", "If-Match": "2" }, body: JSON.stringify({ enabled: true }) }); return { status: response.status, body: await response.json() }; }, { accessToken, modelID });
  if (rejected.status !== 422 || rejected.body.error !== "invalid_catalog_resource") throw new Error("Configured Model was enabled with a disabled Credential Profile");
  await page.getByTestId(`toggle-credential-${credentialID}`).click();
  await page.getByTestId("model-notice").waitFor();
  if (!(await page.getByTestId(`model-${modelID}`).textContent()).includes("Disabled") && !(await page.getByTestId(`model-${modelID}`).textContent()).includes("已禁用")) throw new Error("re-enabling a Credential Profile automatically enabled its Configured Model");
  await page.getByTestId(`toggle-model-${modelID}`).click();
  await page.getByTestId("model-notice").waitFor();
  if (!(await page.getByTestId(`model-${modelID}`).textContent()).includes("Enabled") && !(await page.getByTestId(`model-${modelID}`).textContent()).includes("已启用")) throw new Error("Configured Model could not be explicitly re-enabled");
  const replayBody = JSON.stringify({ name: "acceptance-replay-key", kind: "model", secret_ref: "vault://acceptance/replay" });
  const headers = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "credential-register-replay" };
  const register = async () => page.evaluate(async ({ headers, replayBody }) => { const response = await fetch("/api/v1/credential-profiles", { method: "POST", headers, body: replayBody }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { headers, replayBody });
  const first = await register(); const replay = await register();
  if (first.status !== 201 || replay.status !== 201 || replay.replayed !== "true" || first.body.id !== replay.body.id || "secret_ref" in replay.body) throw new Error("Credential Profile registration was not safely replayed");
  const browserValues = await page.evaluate(async () => ({ url: location.href, page: document.body.textContent ?? "", local: Object.values(localStorage), session: Object.values(sessionStorage) }));
  const cookieValues = (await page.context().cookies()).flatMap((cookie) => [cookie.name, cookie.value]);
  if ([browserValues.url, browserValues.page, ...browserValues.local, ...browserValues.session, ...cookieValues, ...consoleMessages].some((value) => value.includes(modelSecret))) throw new Error("model Secret canary reached a browser surface");
  page.off("console", recordConsole);
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const secrets = [
    "'"$git_private_key_canary_left"'" + "'"$git_private_key_canary_right"'",
    "'"$known_hosts_canary_left"'" + "'"$known_hosts_canary_right"'",
    "'"$build_secret_canary_left"'" + "'"$build_secret_canary_right"'",
  ];
  const consoleMessages = [];
  const recordConsole = (message) => consoleMessages.push(message.text());
  page.on("console", recordConsole);
  await page.getByTestId("register-provider").click();
  await page.getByTestId("provider-name").fill("acceptance-github");
  await page.getByTestId("submit-provider").click();
  await page.getByTestId("repository-notice").waitFor();
  await page.getByTestId("register-binding").click();
  await page.getByTestId("binding-name").fill("acceptance-repository");
  await page.getByTestId("binding-repository-url").fill("git@github.com:acme/agent-platform.git");
  await page.getByTestId("binding-ssh-credential").fill("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa");
  await page.getByTestId("binding-build-credentials").fill("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb");
  await page.getByTestId("binding-author-email").fill("agent@example.test");
  const runtimeInputs = page.locator(".runtime-checks input[type=checkbox]");
  if (await runtimeInputs.count() < 2) throw new Error("Runtime Image choices were not refreshed before Binding creation");
  for (let index = 0; index < await runtimeInputs.count(); index += 1) await runtimeInputs.nth(index).click();
  await page.getByTestId("binding-capability-streaming").click();
  const modelOption = page.getByTestId("binding-default-model").locator("option").filter({ hasText: "acceptance-primary" });
  await page.getByTestId("binding-default-model").selectOption(await modelOption.getAttribute("value"));
  await page.getByTestId("binding-quality-executable").fill("go");
  await page.getByRole("button", { name: /Add argument|添加参数/ }).click();
  await page.getByTestId("binding-quality-argument-0-0").fill("test");
  await page.getByRole("button", { name: /Add argument|添加参数/ }).click();
  await page.getByTestId("binding-quality-argument-0-1").fill("./...");
  const bindingWrite = page.waitForResponse((response) => response.url().includes("/api/v1/repository-bindings") && response.request().method() === "POST");
  await page.getByTestId("submit-binding").click();
  const bindingResponse = await bindingWrite;
  if (bindingResponse.status() !== 201) throw new Error(`Repository Binding registration failed safely: ${JSON.stringify(await bindingResponse.json())}`);
  await page.getByTestId("repository-notice").waitFor();
  const bindingRecord = page.locator(".binding-record").filter({ hasText: "acceptance-repository" });
  const bindingID = (await bindingRecord.getAttribute("data-testid")).replace("binding-", "");
  await page.getByTestId(`validate-binding-${bindingID}`).click();
  await page.getByTestId("repository-notice").waitFor();
  if (!(await bindingRecord.textContent()).includes("allowed_runtime_image_ids")) throw new Error("non-Production Runtime did not produce a localized Binding validation error");
  await page.getByTestId(`edit-binding-${bindingID}`).click();
  await page.locator(".runtime-checks label").filter({ hasText: "experimental" }).locator("input").click();
  await page.getByTestId("submit-binding").click();
  await page.getByTestId("repository-notice").waitFor();
  await page.getByTestId(`validate-binding-${bindingID}`).click();
  await page.getByTestId("repository-notice").waitFor();
  if (!(await bindingRecord.textContent()).includes("Validated") && !(await bindingRecord.textContent()).includes("验证通过")) throw new Error("valid Repository Binding dependencies did not pass validation");
  const providerRecord = page.locator(".catalog-record[data-testid^=provider-]").filter({ hasText: "acceptance-github" });
  const providerID = (await providerRecord.getAttribute("data-testid")).replace("provider-", "");
  await page.getByTestId(`toggle-provider-${providerID}`).click();
  await page.getByTestId("repository-notice").waitFor();
  await page.getByTestId(`validate-binding-${bindingID}`).click();
  await page.getByTestId("repository-notice").waitFor();
  if (!(await bindingRecord.textContent()).includes("source_control_provider_id")) throw new Error("disabled Provider did not invalidate its Repository Binding");
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const conflict = await page.evaluate(async ({ accessToken, bindingID }) => {
    const current = await fetch(`/api/v1/repository-bindings/${bindingID}?team_id=66666666-6666-4666-8666-666666666666`, { headers: { Authorization: `Bearer ${accessToken}` } }).then((response) => response.json());
    const body = { team_id: current.team_id, source_control_provider_id: current.source_control_provider_id, name: current.name, repository_ssh_url: current.repository_ssh_url, default_branch: current.default_branch, ssh_credential_profile_id: current.ssh_credential_profile_id, build_credential_profile_ids: current.build_credential_profile_ids, git_author_name: current.git_author_name, git_author_email: current.git_author_email, allowed_runtime_image_ids: current.allowed_runtime_image_ids, default_runtime_image_id: current.default_runtime_image_id, default_model_id: current.default_model_id, model_budget: current.model_budget, instructions: current.instructions, quality_commands: current.quality_commands, egress_policy: current.egress_policy };
    const response = await fetch(`/api/v1/repository-bindings/${bindingID}`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "binding-stale-version", "If-Match": "1" }, body: JSON.stringify({ binding: body }) });
    return { status: response.status, body: await response.json() };
  }, { accessToken, bindingID });
  if (conflict.status !== 412 || conflict.body.error !== "version_conflict") throw new Error("stale Repository Binding Version was not rejected");
  const replayBody = JSON.stringify({ name: "acceptance-gitlab", kind: "gitlab_self_managed", base_url: "https://gitlab.example.test" });
  const replayHeaders = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "provider-register-replay" };
  const register = async () => page.evaluate(async ({ replayBody, replayHeaders }) => { const response = await fetch("/api/v1/source-control-providers", { method: "POST", headers: replayHeaders, body: replayBody }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { replayBody, replayHeaders });
  const first = await register(); const replay = await register();
  if (first.status !== 201 || replay.status !== 201 || replay.replayed !== "true" || first.body.id !== replay.body.id) throw new Error("Source Control Provider registration was not replayed");
  const browserValues = await page.evaluate(() => ({ url: location.href, page: document.body.textContent ?? "", local: Object.values(localStorage), session: Object.values(sessionStorage) }));
  const cookieValues = (await page.context().cookies()).flatMap((cookie) => [cookie.name, cookie.value]);
  if (secrets.some((secret) => [browserValues.url, browserValues.page, ...browserValues.local, ...browserValues.session, ...cookieValues, ...consoleMessages].some((value) => value.includes(secret)))) throw new Error("repository Secret canary reached a browser surface");
  page.off("console", recordConsole);
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const teamID = "66666666-6666-4666-8666-666666666666";
  const providerRecord = page.locator(".catalog-record[data-testid^=provider-]").filter({ hasText: "acceptance-github" });
  const providerID = (await providerRecord.getAttribute("data-testid")).replace("provider-", "");
  await page.getByTestId(`toggle-provider-${providerID}`).click();
  await page.getByTestId("repository-notice").waitFor();
  const bindingRecord = page.locator(".binding-record").filter({ hasText: "acceptance-repository" });
  const bindingID = (await bindingRecord.getAttribute("data-testid")).replace("binding-", "");
  await page.getByTestId(`validate-binding-${bindingID}`).click();
  await page.getByTestId("repository-notice").waitFor();
  if (!(await bindingRecord.textContent()).match(/Validated|验证通过/)) throw new Error("Repository Binding was not ready for Agent Draft validation");

  await page.getByTestId("create-agent").click();
  await page.getByTestId("agent-name").fill("acceptance-coding-agent");
  await page.getByTestId("agent-description").fill("Browser acceptance Agent");
  await page.getByTestId("submit-agent").click();
  await page.getByTestId("agent-notice").waitFor();
  const agentButton = page.locator(".agent-list > button").filter({ hasText: "acceptance-coding-agent" });
  const agentID = (await agentButton.getAttribute("data-testid")).replace("agent-", "");
  await page.getByTestId("create-draft").click();
  await page.getByTestId("draft-instructions").fill("Implement, test, and push the requested change.");
  await page.getByTestId("submit-draft").click();
  await page.getByTestId("agent-notice").waitFor();
  const draftCard = page.locator(".draft-card").first();
  const draftID = (await draftCard.getAttribute("data-testid")).replace("draft-", "");

  await page.getByTestId(`edit-draft-${draftID}`).click();
  await page.getByTestId("draft-input-budget").fill("100001");
  await page.getByTestId("submit-draft").click();
  await page.getByTestId("agent-notice").waitFor();
  await page.getByTestId(`validate-draft-${draftID}`).click();
  await page.getByTestId("agent-notice").waitFor();
  if (!(await page.getByTestId(`draft-${draftID}`).textContent()).includes("model_budget")) throw new Error("invalid Draft budget did not produce a field-level Validation Report");

  await page.getByTestId(`edit-draft-${draftID}`).click();
  await page.getByTestId("draft-input-budget").fill("100000");
  await page.getByTestId("submit-draft").click();
  await page.getByTestId("agent-notice").waitFor();
  const editedText = await page.getByTestId(`draft-${draftID}`).textContent();
  if (editedText.includes("model_budget") || !editedText.match(/Unvalidated|未保存验证/)) throw new Error("Draft edit retained a stale Validation Report");

  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const current = await page.evaluate(async ({ accessToken, agentID, draftID, teamID }) => fetch(`/api/v1/agents/${agentID}/drafts/${draftID}?team_id=${teamID}`, { headers: { Authorization: `Bearer ${accessToken}` } }).then((response) => response.json()), { accessToken, agentID, draftID, teamID });
  const validationHeaders = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "agent-draft-validation-replay", "If-Match": `"${current.version}"` };
  const validate = async () => page.evaluate(async ({ accessToken, agentID, draftID, teamID, validationHeaders }) => { const response = await fetch(`/api/v1/agents/${agentID}/drafts/${draftID}/validation`, { method: "POST", headers: validationHeaders, body: JSON.stringify({ team_id: teamID }) }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { accessToken, agentID, draftID, teamID, validationHeaders });
  const firstValidation = await validate();
  const replayedValidation = await validate();
  if (firstValidation.status !== 200 || firstValidation.body.state !== "ready" || replayedValidation.status !== 200 || replayedValidation.replayed !== "true" || replayedValidation.body.version !== firstValidation.body.version) throw new Error("Agent Draft validation was not idempotently replayed");

  const staleUpdate = await page.evaluate(async ({ accessToken, agentID, draftID, teamID, current }) => { const response = await fetch(`/api/v1/agents/${agentID}/drafts/${draftID}`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "agent-draft-stale-update", "If-Match": "1" }, body: JSON.stringify({ team_id: teamID, configuration: current.configuration, release_risk: current.release_risk }) }); return { status: response.status, body: await response.json() }; }, { accessToken, agentID, draftID, teamID, current });
  if (staleUpdate.status !== 412 || staleUpdate.body.error !== "version_conflict") throw new Error("stale Agent Draft Version was not rejected");
  const crossTeam = await page.evaluate(async ({ accessToken, agentID }) => fetch(`/api/v1/agents/${agentID}?team_id=55555555-5555-4555-8555-555555555555`, { headers: { Authorization: `Bearer ${accessToken}` } }).then((response) => response.status), { accessToken, agentID });
  if (crossTeam !== 404) throw new Error("cross-Team Agent lookup disclosed the resource");
  await page.reload();
  await page.getByTestId(`draft-${draftID}`).waitFor();
  const persisted = await page.getByTestId(`draft-${draftID}`).textContent();
  if (!persisted.match(/Ready/) || !(await page.locator("body").textContent()).includes("acceptance-coding-agent")) throw new Error("Agent or ready Draft did not persist across refresh");
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const teamID = "66666666-6666-4666-8666-666666666666";
  const agentID = (await page.locator(".agent-list > button").filter({ hasText: "acceptance-coding-agent" }).getAttribute("data-testid")).replace("agent-", "");
  const lowDraftID = (await page.locator(".draft-card").first().getAttribute("data-testid")).replace("draft-", "");
  const publishRequest = page.waitForRequest((request) => request.url().includes(`/api/v1/agents/${agentID}/drafts/${lowDraftID}/release`) && request.method() === "POST");
  const publishResponse = page.waitForResponse((response) => response.url().includes(`/api/v1/agents/${agentID}/drafts/${lowDraftID}/release`) && response.request().method() === "POST");
  await page.getByTestId(`publish-draft-${lowDraftID}`).click();
  const request = await publishRequest; const response = await publishResponse;
  if (response.status() !== 201) throw new Error(`low-risk Agent Release failed: ${JSON.stringify(await response.json())}`);
  const lowReleaseID = (await page.locator(".release-card").first().getAttribute("data-testid")).replace("release-", "");
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const replay = await page.evaluate(async ({ accessToken, agentID, lowDraftID, teamID, key }) => { const result = await fetch(`/api/v1/agents/${agentID}/drafts/${lowDraftID}/release`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ team_id: teamID }) }); return { status: result.status, replayed: result.headers.get("Idempotency-Replayed"), body: await result.json() }; }, { accessToken, agentID, lowDraftID, teamID, key: request.headers()["idempotency-key"] });
  if (replay.status !== 201 || replay.replayed !== "true" || replay.body.id !== lowReleaseID) throw new Error("Agent Release publish was not idempotently replayed");

  const createHighDraft = async (instructions) => {
    await page.getByTestId("create-draft").click();
    await page.getByTestId("draft-instructions").fill(instructions);
    await page.getByTestId("draft-risk").selectOption("high");
    const created = page.waitForResponse((result) => result.url().includes(`/api/v1/agents/${agentID}/drafts`) && result.request().method() === "POST");
    await page.getByTestId("submit-draft").click();
    const createdResponse = await created;
    if (createdResponse.status() !== 201) throw new Error("high-risk Draft creation failed");
    const draftID = (await createdResponse.json()).id;
    const validated = page.waitForResponse((result) => result.url().includes(`/drafts/${draftID}/validation`) && result.request().method() === "POST");
    await page.getByTestId(`validate-draft-${draftID}`).click();
    if ((await validated).status() !== 200) throw new Error("high-risk Draft validation failed");
    await page.getByTestId(`request-release-approval-${draftID}`).click();
    await page.getByTestId("approval-risk-reason").fill(`High-risk capability: ${instructions}`);
    const requested = page.waitForResponse((result) => result.url().includes(`/drafts/${draftID}/approval`) && result.request().method() === "POST");
    await page.getByTestId("submit-release-approval").click();
    if ((await requested).status() !== 201) throw new Error("Release Approval request failed");
    return draftID;
  };
  const highA = await createHighDraft("High risk A");
  const highB = await createHighDraft("High risk B");
  const currentApproval = await page.evaluate(async ({ accessToken, agentID, highA, teamID }) => fetch(`/api/v1/agents/${agentID}/drafts/${highA}/approval?team_id=${teamID}`, { headers: { Authorization: `Bearer ${accessToken}` } }).then((result) => result.json()), { accessToken, agentID, highA, teamID });
  const selfDecision = await page.evaluate(async ({ accessToken, agentID, highA, teamID, currentApproval }) => { const result = await fetch(`/api/v1/agents/${agentID}/drafts/${highA}/approval`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "release-self-approval", "If-Match": `"${currentApproval.version}"` }, body: JSON.stringify({ team_id: teamID, approved: true, reason: "self" }) }); return { status: result.status, body: await result.json() }; }, { accessToken, agentID, highA, teamID, currentApproval });
  if (selfDecision.status !== 422 || selfDecision.body.error !== "invalid_agent_lifecycle_resource") throw new Error("Release Approval requester approved their own request");
  await page.evaluate(({ agentID, lowReleaseID, highA, highB }) => { sessionStorage.setItem("acceptance-agent-id", agentID); sessionStorage.setItem("acceptance-low-release-id", lowReleaseID); sessionStorage.setItem("acceptance-high-a", highA); sessionStorage.setItem("acceptance-high-b", highB); }, { agentID, lowReleaseID, highA, highB });
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const teamID = "66666666-6666-4666-8666-666666666666";
  await page.getByTestId("nav-workspace").click(); await page.waitForURL(/workspace/); await page.getByTestId("binding-select").waitFor();
  await page.getByTestId("task-title").fill("Acceptance free-text task");
  await page.getByTestId("request-text").fill("Implement the free-text acceptance change and run tests.");
  const requestPromise = page.waitForRequest((request) => request.url().endsWith("/api/v1/coding-tasks") && request.method() === "POST");
  const responsePromise = page.waitForResponse((response) => response.url().endsWith("/api/v1/coding-tasks") && response.request().method() === "POST");
  await page.getByTestId("create-task").click();
  const request = await requestPromise; const response = await responsePromise; const launch = await response.json();
  if (response.status() !== 201 || !launch.task?.id || !launch.session?.id || !launch.run_id || !launch.session.review_branch) throw new Error("free-text Coding Task launch did not return a complete atomic context");
  await page.waitForURL(new RegExp(`task=${launch.task.id}`)); await page.locator(".workspace-facts").filter({ hasText: launch.session.review_branch }).waitFor();
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const replay = await page.evaluate(async ({ accessToken, key, body }) => { const result = await fetch("/api/v1/coding-tasks", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key }, body }); return { status: result.status, replayed: result.headers.get("Idempotency-Replayed"), body: await result.json() }; }, { accessToken, key: request.headers()["idempotency-key"], body: request.postData() });
  if (replay.status !== 201 || replay.replayed !== "true" || replay.body.task.id !== launch.task.id || replay.body.session.id !== launch.session.id || replay.body.run_id !== launch.run_id) throw new Error("Coding Task launch was not idempotently replayed");
  const conflict = await page.evaluate(async ({ accessToken, key, body }) => { const changed = JSON.parse(body); changed.title = "Changed intent"; const result = await fetch("/api/v1/coding-tasks", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify(changed) }); return result.status; }, { accessToken, key: request.headers()["idempotency-key"], body: request.postData() });
  if (conflict !== 409) throw new Error("same Coding Task Idempotency Key accepted a different request");
  const crossTeam = await page.evaluate(async ({ accessToken, taskID }) => fetch(`/api/v1/coding-tasks/${taskID}?team_id=55555555-5555-4555-8555-555555555555`, { headers: { Authorization: `Bearer ${accessToken}` } }).then((result) => result.status), { accessToken, taskID: launch.task.id });
  if (crossTeam !== 404) throw new Error("cross-Team Coding Task query disclosed the resource");
  await page.reload(); await page.locator(".workspace-facts").filter({ hasText: launch.session.review_branch }).waitFor();
  if (!(await page.locator(".workspace-detail").textContent()).includes(launch.run_id)) throw new Error("Coding Task direct URL did not restore its Session and first Run");

  await page.locator(".source-switch button").filter({ hasText: "Issue Snapshot" }).click();
  await page.getByTestId("issue-title").fill("Acceptance issue snapshot"); await page.getByTestId("issue-body").fill("Implement the snapshotted issue body."); await page.getByTestId("issue-url").fill("https://git.example.test/issues/42");
  const issueResponse = page.waitForResponse((result) => result.url().endsWith("/api/v1/coding-tasks") && result.request().method() === "POST");
  await page.getByTestId("create-task").click(); const issueLaunch = await issueResponse; const issueBody = await issueLaunch.json();
  if (issueLaunch.status() !== 201 || issueBody.task.issue_snapshot?.title !== "Acceptance issue snapshot" || issueBody.task.issue_snapshot?.url !== "https://git.example.test/issues/42") throw new Error("immutable Issue Snapshot launch failed");
  await page.evaluate(({ firstTask, firstSession, firstBranch, issueTask }) => { sessionStorage.setItem("acceptance-task-id", firstTask); sessionStorage.setItem("acceptance-task-session-id", firstSession); sessionStorage.setItem("acceptance-task-branch", firstBranch); sessionStorage.setItem("acceptance-issue-task-id", issueTask); }, { firstTask: launch.task.id, firstSession: launch.session.id, firstBranch: launch.session.review_branch, issueTask: issueBody.task.id });
}'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "INSERT INTO run_attempts (id, run_id, attempt_number, worker_id, state, infrastructure_failure, started_at, ended_at)
     SELECT 'abababab-abab-4bab-8bab-ababababab01', runs.id, 1, 'acceptance-worker', 'completed', false, now() - interval '2 minutes', now() - interval '1 minute'
     FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id
     WHERE coding_tasks.title = 'Acceptance free-text task' ORDER BY runs.created_at LIMIT 1;
   INSERT INTO run_events (run_id, sequence, event_type, payload, created_at)
     SELECT runs.id, event.sequence, event.event_type, event.payload, now() - interval '90 seconds' + event.sequence * interval '1 second'
     FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id
     CROSS JOIN (VALUES
       (2::bigint, 'plan.updated', '{\"steps\":[\"inspect\",\"change\",\"verify\"]}'::jsonb),
       (3::bigint, 'command.requested', '{\"executable\":\"go\",\"arguments\":[\"test\",\"./...\"]}'::jsonb),
       (4::bigint, 'file.changed', '{\"path\":\"backend/parser.go\"}'::jsonb),
       (5::bigint, 'validation.completed', '{\"name\":\"go test\",\"passed\":true}'::jsonb),
       (6::bigint, 'diff.generated', convert_from(decode('$run_event_payload_base64', 'base64'), 'UTF8')::jsonb),
       (7::bigint, 'approval.requested', '{\"required\":false}'::jsonb),
       (8::bigint, 'usage.updated', '{\"input_tokens\":120,\"output_tokens\":48}'::jsonb),
       (9::bigint, 'cost.updated', '{\"amount\":\"0.0042\",\"currency\":\"USD\"}'::jsonb),
       (10::bigint, 'runtime.completed', '{\"result\":\"review branch pushed\"}'::jsonb),
       (11::bigint, 'run.completed', '{\"result\":\"success\"}'::jsonb)
     ) AS event(sequence, event_type, payload)
     WHERE coding_tasks.title = 'Acceptance free-text task';
   UPDATE runs SET state = 'completed', attempt_count = 1, usage = '{\"input_tokens\":120,\"output_tokens\":48}', cost_amount = 0.0042, started_at = now() - interval '2 minutes', ended_at = now(), updated_at = now(), version = version + 1
     WHERE id = (SELECT runs.id FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id WHERE coding_tasks.title = 'Acceptance free-text task' ORDER BY runs.created_at LIMIT 1);
   INSERT INTO artifacts (id, run_id, kind, object_key, size_bytes, sha256, content_type, metadata, expires_at)
     SELECT 'abababab-abab-4bab-8bab-ababababab02', runs.id, 'diff', 'phase-0/acceptance/codex/evidence.tar', $artifact_size, '$artifact_sha256', 'application/x-tar', '{\"attempt\":\"1\",\"secret\":\"[REDACTED]\"}', now() + interval '1 day'
     FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id
     WHERE coding_tasks.title = 'Acceptance free-text task' ORDER BY runs.created_at LIMIT 1;" >/dev/null

browser --session "$playwright_session" run-code 'async (page) => {
  const expected = await page.evaluate(() => ({ task: sessionStorage.getItem("acceptance-task-id") }));
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const targetRunID = await page.evaluate(async ({ accessToken, task }) => { const response = await fetch(`/api/v1/runs?team_id=66666666-6666-4666-8666-666666666666&task_id=${task}&limit=50`, { headers: { Authorization: `Bearer ${accessToken}` } }); const body = await response.json(); return body.items?.[0]?.id ?? ""; }, { accessToken, task: expected.task });
  if (!targetRunID) throw new Error("target Run for reconnect acceptance was not found");
  const streamPattern = new RegExp(`/api/v1/runs/${targetRunID}/events$`);
  let streamCalls = 0; let reconnectCursor = "";
  await page.route(streamPattern, async (route, request) => {
    streamCalls += 1;
    if (streamCalls === 1) {
      const upstream = await route.fetch(); const body = await upstream.text();
      const replayPrefix = body.split("\n\n").filter(Boolean).slice(0, 3).join("\n\n") + "\n\n";
      await route.fulfill({ status: 200, headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" }, body: replayPrefix }); return;
    }
    reconnectCursor = request.headers()["last-event-id"] ?? ""; await route.continue();
  });
  await page.goto(`http://127.0.0.1:18092/workspace?team=66666666-6666-4666-8666-666666666666&task=${expected.task}`);
  const evidence = page.getByTestId("run-evidence"); await evidence.waitFor(); await evidence.locator(".stream-complete").waitFor();
  if (streamCalls !== 2 || reconnectCursor !== "3") throw new Error(`Run Event reconnect did not resume from cursor 3: calls=${streamCalls} cursor=${reconnectCursor}`);
  if (await evidence.locator(".event-timeline li:not(.empty-evidence)").count() !== 11) throw new Error("Run Event reconnect produced a gap or duplicate");
  if (!(await evidence.textContent()).includes("Attempt 1") || !(await evidence.textContent()).includes("acceptance-worker")) throw new Error("Run Attempt history was not rendered as part of the same Run");
  if (!(await evidence.textContent()).includes("[display truncated at 16384 characters]")) throw new Error("oversized Run Event did not show its explicit preview boundary");
  await page.unroute(streamPattern); await page.reload(); await evidence.locator(".stream-complete").waitFor();
  if (await evidence.locator(".event-timeline li:not(.empty-evidence)").count() !== 11) throw new Error("refresh did not restore the complete Run Event timeline");
  const pageText = await page.locator("body").textContent();
  if (pageText.includes("phase-0/acceptance/codex/evidence.tar") || pageText.includes("acceptance-only-secret")) throw new Error("Artifact Object Key or Provider Credential reached the browser");
  await page.evaluate(() => { window.open = (url) => { sessionStorage.setItem("acceptance-artifact-download", String(url)); return null; }; });
  await evidence.locator(".artifact-grid button").filter({ hasText: "diff" }).click();
  await page.waitForFunction(() => Boolean(sessionStorage.getItem("acceptance-artifact-download")));
  const signedURL = await page.evaluate(() => sessionStorage.getItem("acceptance-artifact-download"));
  if (!signedURL || !signedURL.includes("X-Amz-Signature") || !signedURL.includes("X-Amz-Expires=300")) throw new Error("Artifact download was not a five-minute signed result");
  const downloaded = await page.request.get(signedURL);
  if (!downloaded.ok() || (await downloaded.body()).length === 0) throw new Error("authorized Artifact download did not return the stored object");
  await page.evaluate(() => sessionStorage.removeItem("acceptance-artifact-download"));
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const expected = await page.evaluate(() => ({ task: sessionStorage.getItem("acceptance-task-id"), session: sessionStorage.getItem("acceptance-task-session-id"), branch: sessionStorage.getItem("acceptance-task-branch") }));
  await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); await page.getByTestId("sign-in-button").click();
  const username = page.getByRole("textbox", { name: "Username or email" });
  if (await username.waitFor({ timeout: 3000 }).then(() => true).catch(() => false)) {
    await username.fill("platform-user"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click();
  }
  await page.waitForURL(/team=55555555/);
  await page.goto(`http://127.0.0.1:18092/workspace?team=66666666-6666-4666-8666-666666666666&task=${expected.task}`);
  const facts = page.locator(".workspace-facts"); await facts.filter({ hasText: expected.branch }).waitFor();
  if (!(await facts.textContent()).includes(expected.session)) throw new Error("re-login did not restore the same Coding Task Session");
}'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE repository_bindings SET validation_report = jsonb_build_object('valid', false, 'errors', jsonb_build_object('quality_commands', 'acceptance invalidation'), 'checked_at', now()), validated_at = now() WHERE name = 'acceptance-repository';" >/dev/null
browser --session "$playwright_session" reload
browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("launch-prerequisite").waitFor(); if (!(await page.getByTestId("launch-prerequisite").textContent()).match(/Repository Binding|仓库/)) throw new Error("missing Repository Binding prerequisite was not shown accurately"); if (await page.getByTestId("create-task").isEnabled()) throw new Error("Coding Task launch remained enabled with an invalid Repository Binding"); }'
docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE repository_bindings SET validation_report = jsonb_build_object('valid', true, 'errors', jsonb_build_object(), 'checked_at', now()), validated_at = now() WHERE name = 'acceptance-repository';" >/dev/null

browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("release-reviewer"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=66666666/); await page.getByTestId("nav-studio").click(); await page.waitForURL(/studio/); }'
browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => ({ highA: sessionStorage.getItem("acceptance-high-a"), highB: sessionStorage.getItem("acceptance-high-b"), lowReleaseID: sessionStorage.getItem("acceptance-low-release-id") }));
  for (const draftID of [values.highA, values.highB]) {
    const decision = page.waitForResponse((response) => response.url().includes(`/drafts/${draftID}/approval`) && response.request().method() === "PATCH");
    await page.getByTestId(`approve-release-${draftID}`).click();
    await page.getByTestId("submit-release-decision").click();
    if ((await decision).status() !== 200) throw new Error("second Builder could not approve Release Approval");
  }
  if (await page.locator("[data-testid^=block-release-]").count()) throw new Error("Team Agent Builder was shown Organization-only Block controls");
  const deprecated = page.waitForResponse((response) => response.url().includes(`/releases/${values.lowReleaseID}/deprecation`) && response.request().method() === "POST");
  await page.getByTestId(`deprecate-release-${values.lowReleaseID}`).click();
  if ((await deprecated).status() !== 200) throw new Error("Agent Builder could not deprecate a Release");
}'

browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("platform-user"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=55555555/); await page.getByTestId("team-select").selectOption("66666666-6666-4666-8666-666666666666"); await page.getByTestId("nav-studio").click(); await page.waitForURL(/studio/); }'
browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => ({ agentID: sessionStorage.getItem("acceptance-agent-id"), highA: sessionStorage.getItem("acceptance-high-a"), highB: sessionStorage.getItem("acceptance-high-b") }));
  const publishRequest = page.waitForRequest((request) => request.url().includes(`/drafts/${values.highA}/release`) && request.method() === "POST");
  const published = page.waitForResponse((response) => response.url().includes(`/drafts/${values.highA}/release`) && response.request().method() === "POST");
  await page.getByTestId(`publish-draft-${values.highA}`).click();
  const request = await publishRequest; const response = await published;
  if (response.status() !== 201) throw new Error(`approved high-risk publish failed: ${JSON.stringify(await response.json())}`);
  const releaseBody = await response.json();
  const releaseCard = page.getByTestId(`release-${releaseBody.id}`);
  const releaseText = await releaseCard.textContent();
  if (!releaseText.includes("registry.example/codex@sha256") || !releaseText.includes("acceptance-primary") || !releaseText.includes("acceptance-repository") || !releaseText.includes("High-risk capability: High risk A")) throw new Error("immutable Release snapshots or approval evidence were not rendered");
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const replay = await page.evaluate(async ({ accessToken, values, key }) => { const result = await fetch(`/api/v1/agents/${values.agentID}/drafts/${values.highA}/release`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ team_id: "66666666-6666-4666-8666-666666666666" }) }); return { status: result.status, replayed: result.headers.get("Idempotency-Replayed"), body: await result.json() }; }, { accessToken, values, key: request.headers()["idempotency-key"] });
  if (replay.status !== 201 || replay.replayed !== "true" || replay.body.id !== releaseBody.id) throw new Error("high-risk Agent Release publish was not replayed");
  await page.getByTestId(`edit-draft-${values.highB}`).click();
  await page.getByTestId("draft-instructions").fill("High risk B edited after approval");
  const edited = page.waitForResponse((result) => result.url().includes(`/drafts/${values.highB}`) && result.request().method() === "PATCH");
  await page.getByTestId("submit-draft").click(); if ((await edited).status() !== 200) throw new Error("approved Draft edit failed");
  const validated = page.waitForResponse((result) => result.url().includes(`/drafts/${values.highB}/validation`) && result.request().method() === "POST");
  await page.getByTestId(`validate-draft-${values.highB}`).click(); if ((await validated).status() !== 200) throw new Error("edited high-risk Draft did not revalidate");
  await page.getByTestId(`request-release-approval-${values.highB}`).waitFor();
  await page.getByTestId(`publish-draft-${values.highB}`).waitFor({ state: "detached" });
  await page.evaluate(({ releaseID }) => sessionStorage.setItem("acceptance-high-release-id", releaseID), { releaseID: releaseBody.id });
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => { let accessToken = ""; for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") accessToken = parsed.access_token; } catch {} } return { accessToken, releaseID: sessionStorage.getItem("acceptance-high-release-id") }; });
  const create = async (title, key) => page.evaluate(async ({ values, title, key }) => { const response = await fetch("/api/v1/coding-tasks", { method: "POST", headers: { Authorization: `Bearer ${values.accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ team_id: "66666666-6666-4666-8666-666666666666", agent_release_id: values.releaseID, title, request_text: `${title} request` }) }); return { status: response.status, body: await response.json() }; }, { values, title, key });
  const approval = await create("Run Approval acceptance", "run-approval-task"); const control = await create("Run Control acceptance", "run-control-task"); const concurrent = await create("Run Concurrent Control acceptance", "run-concurrent-control-task"); const operator = await create("Run Operator denial acceptance", "run-operator-task"); const abort = await create("Runtime Approval Abort acceptance", "runtime-approval-abort-task"); const browserRuntime = await create("Runtime Approval Browser acceptance", "runtime-approval-browser-task");
  for (const result of [approval, control, concurrent, operator, abort, browserRuntime]) if (result.status !== 201) throw new Error(`Run governance fixture launch failed: ${JSON.stringify(result.body)}`);
  await page.evaluate(({ approval, control, concurrent, operator, abort, browserRuntime }) => { sessionStorage.setItem("acceptance-run-approval-task", approval.body.task.id); sessionStorage.setItem("acceptance-run-approval", approval.body.run_id); sessionStorage.setItem("acceptance-run-control-task", control.body.task.id); sessionStorage.setItem("acceptance-run-control", control.body.run_id); sessionStorage.setItem("acceptance-run-concurrent-control", concurrent.body.run_id); sessionStorage.setItem("acceptance-run-operator-task", operator.body.task.id); sessionStorage.setItem("acceptance-run-operator", operator.body.run_id); sessionStorage.setItem("acceptance-runtime-approval-browser-task", browserRuntime.body.task.id); sessionStorage.setItem("acceptance-runtime-approval-browser-run", browserRuntime.body.run_id); }, { approval, control, concurrent, operator, abort, browserRuntime });
}'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE runs SET state = 'running', started_at = now(), updated_at = now() WHERE id IN (
     SELECT runs.id FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id
     WHERE coding_tasks.title IN ('Run Approval acceptance', 'Run Control acceptance', 'Run Concurrent Control acceptance', 'Run Operator denial acceptance', 'Runtime Approval Abort acceptance'));
   UPDATE runs SET created_at = '2000-01-01T00:00:00Z' WHERE id = (
     SELECT runs.id FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id
     WHERE coding_tasks.title = 'Runtime Approval Browser acceptance')" >/dev/null

runtime_approval_abort_run_id="$(docker exec "$postgres_container" psql -At -U agent_platform -d agent_platform_oidc -c "SELECT runs.id FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id WHERE coding_tasks.title = 'Runtime Approval Abort acceptance'")"
EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
RUNTIME_APPROVAL_RUN_ID="$runtime_approval_abort_run_id" \
  go -C "$repository_root/backend" test -count=1 -run '^TestRuntimeApprovalGateUsesAtomicGovernanceAndClosesAbandonedApproval$' ./internal/data/controlplane/runtimeapproval

runtime_approval_browser_run_id="$(docker exec "$postgres_container" psql -At -U agent_platform -d agent_platform_oidc -c "SELECT runs.id FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id WHERE coding_tasks.title = 'Runtime Approval Browser acceptance'")"
runtime_approval_ready="$acceptance_tmp/runtime-approval-ready"
runtime_approval_marker="$acceptance_tmp/runtime-approval-continued"
(
  EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
  RUNTIME_APPROVAL_BROWSER_RUN_ID="$runtime_approval_browser_run_id" \
  RUNTIME_APPROVAL_READY_FILE="$runtime_approval_ready" \
  RUNTIME_APPROVAL_MARKER_FILE="$runtime_approval_marker" \
    go -C "$repository_root/backend" test -count=1 -run '^TestRuntimeApprovalBrowserClosedLoop$' ./internal/data/controlplane/runtimeapproval
) >"$acceptance_tmp/runtime-approval.log" 2>&1 &
runtime_approval_browser_pid=$!
for _ in {1..100}; do
  [[ -f "$runtime_approval_ready" ]] && break
  if ! kill -0 "$runtime_approval_browser_pid" >/dev/null 2>&1; then
    redact_canaries "$acceptance_tmp/runtime-approval.log" >&2
    exit 1
  fi
  sleep 0.1
done
[[ -f "$runtime_approval_ready" ]] || { redact_canaries "$acceptance_tmp/runtime-approval.log" >&2; exit 1; }

browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => ({ taskID: sessionStorage.getItem("acceptance-runtime-approval-browser-task"), runID: sessionStorage.getItem("acceptance-runtime-approval-browser-run") }));
  await page.goto(`http://127.0.0.1:18092/workspace?team=66666666-6666-4666-8666-666666666666&task=${values.taskID}`, { waitUntil: "commit" });
  const approval = page.getByTestId("run-approvals").filter({ hasText: "Browser protected write" }); await approval.waitFor();
  const approveButton = approval.locator("[data-testid^=approve-run-]"); await approveButton.waitFor(); const response = page.waitForResponse((item) => item.url().includes("/decision") && item.request().method() === "POST"); await approveButton.click(); if ((await response).status() !== 200) throw new Error("browser did not approve the paused Runtime");
  await page.waitForFunction((runID) => document.body.textContent.includes(runID), values.runID);
}'
if ! wait "$runtime_approval_browser_pid"; then
  redact_canaries "$acceptance_tmp/runtime-approval.log" >&2
  exit 1
fi
runtime_approval_browser_pid=""

browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => { let accessToken = ""; for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") accessToken = parsed.access_token; } catch {} } return { accessToken, taskID: sessionStorage.getItem("acceptance-run-approval-task"), runID: sessionStorage.getItem("acceptance-run-approval"), operatorRunID: sessionStorage.getItem("acceptance-run-operator") }; });
  const requestApproval = async (runID, key, version, risk) => page.evaluate(async ({ values, runID, key, version, risk }) => { const response = await fetch(`/api/v1/runs/${runID}/approvals`, { method: "POST", headers: { Authorization: `Bearer ${values.accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key, "If-Match": `"${version}"` }, body: JSON.stringify({ kind: "high_risk_change", request: { risk_reason: risk, command: "git push review branch" } }) }); return { status: response.status, body: await response.json() }; }, { values, runID, key, version, risk });
  const first = await requestApproval(values.runID, "run-approval-request-1", 1, "Acceptance protected write");
  if (first.status !== 201 || first.body.requested_by !== "33333333-3333-4333-8333-333333333333") throw new Error(`Run Approval request failed: ${JSON.stringify(first)}`);
  await page.goto(`http://127.0.0.1:18092/workspace?team=66666666-6666-4666-8666-666666666666&task=${values.taskID}`); await page.getByTestId("run-approvals").filter({ hasText: "Acceptance protected write" }).waitFor();
  const decisionRequest = page.waitForRequest((request) => request.url().includes(`/approvals/${first.body.id}/decision`) && request.method() === "POST"); const decisionResponse = page.waitForResponse((response) => response.url().includes(`/approvals/${first.body.id}/decision`) && response.request().method() === "POST");
  await page.getByTestId(`approve-run-${first.body.id}`).click(); const sent = await decisionRequest; if ((await decisionResponse).status() !== 200) throw new Error("Run Approval UI approve failed");
  const replay = await page.evaluate(async ({ values, first, key }) => { const response = await fetch(`/api/v1/approvals/${first.body.id}/decision`, { method: "POST", headers: { Authorization: `Bearer ${values.accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key, "If-Match": `"${first.body.version}"` }, body: JSON.stringify({ approved: true, reason: "" }) }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { values, first, key: sent.headers()["idempotency-key"] });
  if (replay.status !== 200 || replay.replayed !== "true" || replay.body.id !== first.body.id) throw new Error("Run Approval decision replay was not stable");
  const second = await requestApproval(values.runID, "run-approval-request-2", 3, "Concurrent decision"); if (second.status !== 201) throw new Error(`second Run Approval request failed: ${JSON.stringify(second)}`);
  const decide = (key) => page.evaluate(async ({ values, second, key }) => { const response = await fetch(`/api/v1/approvals/${second.body.id}/decision`, { method: "POST", headers: { Authorization: `Bearer ${values.accessToken}`, "Content-Type": "application/json", "Idempotency-Key": key, "If-Match": `"${second.body.version}"` }, body: JSON.stringify({ approved: true, reason: "concurrent acceptance" }) }); return response.status; }, { values, second, key });
  const concurrent = await Promise.all([decide("run-approval-concurrent-a"), decide("run-approval-concurrent-b")]); if (concurrent.sort().join(",") !== "200,412") throw new Error(`concurrent Run Approval decisions = ${concurrent}`);
  const third = await requestApproval(values.runID, "run-approval-request-3", 5, "Reject destructive operation"); if (third.status !== 201) throw new Error(`third Run Approval request failed: ${JSON.stringify(third)}`);
  await page.reload(); await page.getByTestId(`reject-run-${third.body.id}`).waitFor(); const rejection = page.waitForResponse((response) => response.url().includes(`/approvals/${third.body.id}/decision`) && response.request().method() === "POST");
  await page.locator(".approval-decision textarea").fill("Destructive operation denied"); await page.getByTestId(`reject-run-${third.body.id}`).click(); if ((await rejection).status() !== 200) throw new Error("Run Approval UI rejection failed");
  await page.getByTestId("run-controls").filter({ hasText: "Version 7" }).waitFor(); if (!(await page.locator(".workspace-facts").textContent()).includes("failed")) throw new Error("rejected Run did not enter failed state");
  const operatorApproval = await requestApproval(values.operatorRunID, "run-operator-approval-request", 1, "Operator must not decide"); if (operatorApproval.status !== 201) throw new Error("operator denial Approval fixture failed"); await page.evaluate((approvalID) => sessionStorage.setItem("acceptance-run-operator-approval", approvalID), operatorApproval.body.id);
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => ({ taskID: sessionStorage.getItem("acceptance-run-control-task"), runID: sessionStorage.getItem("acceptance-run-control"), concurrentRunID: sessionStorage.getItem("acceptance-run-concurrent-control") }));
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const controlConcurrent = (key) => page.evaluate(async ({ accessToken, values, key }) => fetch(`/api/v1/runs/${values.concurrentRunID}/interrupt`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Idempotency-Key": key, "If-Match": "\"1\"" } }).then((response) => response.status), { accessToken, values, key });
  const concurrent = await Promise.all([controlConcurrent("run-control-concurrent-a"), controlConcurrent("run-control-concurrent-b")]); if (concurrent.sort().join(",") !== "200,412") throw new Error(`concurrent Run controls = ${concurrent}`);
  await page.goto(`http://127.0.0.1:18092/workspace?team=66666666-6666-4666-8666-666666666666&task=${values.taskID}`);
  const request = page.waitForRequest((item) => item.url().endsWith(`/runs/${values.runID}/interrupt`) && item.method() === "POST"); const response = page.waitForResponse((item) => item.url().endsWith(`/runs/${values.runID}/interrupt`) && item.request().method() === "POST"); await page.getByTestId("interrupt-run").click(); const sent = await request; if ((await response).status() !== 200) throw new Error("Run interrupt failed");
  const replay = await page.evaluate(async ({ accessToken, values, key }) => { const headers = { Authorization: `Bearer ${accessToken}`, "Idempotency-Key": key, "If-Match": "\"1\"" }; const response = await fetch(`/api/v1/runs/${values.runID}/interrupt`, { method: "POST", headers }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed") }; }, { accessToken, values, key: sent.headers()["idempotency-key"] }); if (replay.status !== 200 || replay.replayed !== "true") throw new Error("Run interrupt replay failed");
  const stale = await page.evaluate(async ({ accessToken, values }) => fetch(`/api/v1/runs/${values.runID}/interrupt`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Idempotency-Key": "run-interrupt-stale", "If-Match": "\"1\"" } }).then((result) => result.status), { accessToken, values }); if (stale !== 412) throw new Error(`stale Run control returned ${stale}`);
}'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE runs SET state = 'interrupted', updated_at = now(), version = version + 1 WHERE id = (
     SELECT runs.id FROM runs JOIN sessions ON sessions.id = runs.session_id JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id WHERE coding_tasks.title = 'Run Control acceptance')" >/dev/null

browser --session "$playwright_session" run-code 'async (page) => {
  await page.reload(); await page.getByTestId("resume-run").waitFor(); const resumed = page.waitForResponse((response) => response.url().endsWith("/resume") && response.request().method() === "POST"); await page.getByTestId("resume-run").click(); if ((await resumed).status() !== 200) throw new Error("interrupted Run did not resume");
  page.once("dialog", (dialog) => dialog.accept()); const cancelled = page.waitForResponse((response) => response.url().endsWith("/cancel") && response.request().method() === "POST"); await page.getByTestId("cancel-run").click(); if ((await cancelled).status() !== 200) throw new Error("resuming Run did not cancel"); await page.reload(); await page.getByTestId("run-controls").filter({ hasText: "Version 5" }).waitFor(); if (await page.getByTestId("cancel-run").count()) throw new Error("terminal Run still showed Cancel");
}'

browser --session "$playwright_session" run-code 'async (page) => {
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const releaseIDs = await page.evaluate(() => ({ agent: sessionStorage.getItem("acceptance-agent-id"), release: sessionStorage.getItem("acceptance-high-release-id") }));
  const runtime = await page.evaluate(async ({ accessToken, releaseIDs }) => { const releaseResponse = await fetch(`/api/v1/agents/${releaseIDs.agent}/releases/${releaseIDs.release}?team_id=66666666-6666-4666-8666-666666666666`, { headers: { Authorization: `Bearer ${accessToken}` } }); const release = await releaseResponse.json(); const response = await fetch(`/api/v1/runtime-images/${release.runtime_image_id}`, { headers: { Authorization: `Bearer ${accessToken}` } }); return response.json(); }, { accessToken, releaseIDs });
  const body = JSON.stringify({ status: "blocked", blocked_reason: "acceptance replay" });
  const headers = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-status-replay", "If-Match": `"${runtime.version}"` };
  const write = async () => page.evaluate(async ({ runtime, headers, body }) => { const response = await fetch(`/api/v1/runtime-images/${runtime.id}/status`, { method: "PATCH", headers, body }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { runtime, headers, body });
  const first = await write(); const replay = await write();
  if (first.status !== 200 || replay.status !== 200 || replay.replayed !== "true" || replay.body.id !== runtime.id || !replay.body.conformance_evidence_sha256) throw new Error("Runtime status write was not replayed from its persisted response");
  await page.goto("http://127.0.0.1:18092/workspace?team=66666666-6666-4666-8666-666666666666"); await page.getByTestId("launch-prerequisite").waitFor();
  if (!(await page.getByTestId("launch-prerequisite").textContent()).includes("Production Runtime")) throw new Error("blocked Runtime did not produce the exact Coding Task prerequisite");
}'

browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("release-reviewer"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=66666666/); await page.getByTestId("nav-studio").click(); await page.waitForURL(/studio/); }'
browser --session "$playwright_session" run-code 'async (page) => {
  const values = await page.evaluate(() => ({ agentID: sessionStorage.getItem("acceptance-agent-id"), releaseID: sessionStorage.getItem("acceptance-high-release-id") }));
  if (await page.getByTestId(`block-release-${values.releaseID}`).count()) throw new Error("Team Agent Builder was shown Block action");
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const denied = await page.evaluate(async ({ accessToken, values }) => { const result = await fetch(`/api/v1/agents/${values.agentID}/releases/${values.releaseID}/block`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "release-block-denied", "If-Match": "1" }, body: JSON.stringify({ team_id: "66666666-6666-4666-8666-666666666666", reason: "unauthorized" }) }); return { status: result.status, body: await result.json() }; }, { accessToken, values });
  if (denied.status !== 403 || denied.body.error !== "agent_build_access_denied") throw new Error("Team Agent Builder blocked an Agent Release");
}'

browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("platform-user"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=55555555/); await page.getByTestId("team-select").selectOption("66666666-6666-4666-8666-666666666666"); await page.getByTestId("nav-studio").click(); await page.waitForURL(/studio/); }'
browser --session "$playwright_session" run-code 'async (page) => {
  const releaseID = await page.evaluate(() => sessionStorage.getItem("acceptance-high-release-id"));
  await page.getByTestId(`block-release-${releaseID}`).click(); await page.getByTestId("block-release-reason").fill("Emergency acceptance policy response");
  const blocked = page.waitForResponse((response) => response.url().includes(`/releases/${releaseID}/block`) && response.request().method() === "POST");
  await page.getByTestId("submit-block-release").click(); if ((await blocked).status() !== 200) throw new Error("Organization Platform Administrator could not Block Release");
  await page.getByTestId(`release-${releaseID}`).filter({ hasText: "Emergency acceptance policy response" }).waitFor();
}'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE role_grants SET team_id = '55555555-5555-4555-8555-555555555555' WHERE id = '44444444-4444-4444-8444-444444444444';" >/dev/null
browser --session "$playwright_session" run-code 'async (page) => { const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const response = await page.evaluate(async (accessToken) => fetch("/api/v1/runtime-images", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-denied-intent" }, body: JSON.stringify({ runtime: "openclaw", cli_version: "1", adapter_version: "1", image_digest: "registry.example/openclaw@sha256:" + "e".repeat(64) }) }).then(async (result) => ({ status: result.status, body: await result.json() })), accessToken); if (response.status !== 403 || response.body.error !== "catalog_write_access_denied") throw new Error("Team-scoped Platform Administrator modified the Organization Runtime Catalog"); }'
browser --session "$playwright_session" run-code 'async (page) => { const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const response = await page.evaluate(async (accessToken) => fetch("/api/v1/credential-profiles", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "model-catalog-denied" }, body: JSON.stringify({ name: "denied-model-key", kind: "model", secret_ref: "vault://denied/model" }) }).then(async (result) => ({ status: result.status, body: await result.json() })), accessToken); if (response.status !== 403 || response.body.error !== "catalog_write_access_denied") throw new Error("Team-scoped Platform Administrator modified the Model Catalog"); }'
browser --session "$playwright_session" run-code 'async (page) => { const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const response = await page.evaluate(async (accessToken) => fetch("/api/v1/repository-bindings", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "binding-denied" }, body: JSON.stringify({ binding: { team_id: "66666666-6666-4666-8666-666666666666", name: "denied" } }) }).then(async (result) => ({ status: result.status, body: await result.json() })), accessToken); if (response.status !== 403 || response.body.error !== "catalog_write_access_denied") throw new Error("Team-scoped Platform Administrator modified a Repository Binding"); }'
browser --session "$playwright_session" run-code 'async (page) => { const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const response = await page.evaluate(async (accessToken) => fetch("/api/v1/agents", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "agent-build-denied" }, body: JSON.stringify({ team_id: "66666666-6666-4666-8666-666666666666", name: "denied-agent" }) }).then(async (result) => ({ status: result.status, body: await result.json() })), accessToken); if (response.status !== 403 || response.body.error !== "agent_build_access_denied") throw new Error("out-of-Team administrator created an Agent"); }'
browser --session "$playwright_session" run-code 'async (page) => { const values = await page.evaluate(() => { let accessToken = ""; for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") accessToken = parsed.access_token; } catch {} } return { accessToken, releaseID: sessionStorage.getItem("acceptance-low-release-id") }; }); const response = await page.evaluate(async ({ accessToken, releaseID }) => fetch("/api/v1/coding-tasks", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "coding-task-denied" }, body: JSON.stringify({ team_id: "66666666-6666-4666-8666-666666666666", agent_release_id: releaseID, title: "Denied", request_text: "Must not launch" }) }).then(async (result) => ({ status: result.status, body: await result.json() })), values); if (response.status !== 403 || response.body.error !== "collaboration_access_denied") throw new Error("user without Team permission created a Coding Task"); }'
browser --session "$playwright_session" run-code 'async (page) => { const values = await page.evaluate(() => { let accessToken = ""; for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") accessToken = parsed.access_token; } catch {} } return { accessToken, runID: sessionStorage.getItem("acceptance-run-control") }; }); const denied = await page.evaluate(async ({ accessToken, runID }) => fetch(`/api/v1/runs/${runID}/cancel`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Idempotency-Key": "revoked-run-control", "If-Match": "\"5\"" } }).then(async (response) => ({ status: response.status, body: await response.json() })), values); if (denied.status !== 403 || denied.body.error !== "run_control_denied") throw new Error("revoked Team grant still authorized Run control"); }'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE role_grants SET role = 'run_operator' WHERE id = '45454545-4545-4545-8545-454545454545';" >/dev/null
browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("release-reviewer"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=66666666/); const approvalID = await page.evaluate(() => sessionStorage.getItem("acceptance-run-operator-approval")); const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const denied = await page.evaluate(async ({ accessToken, approvalID }) => fetch(`/api/v1/approvals/${approvalID}/decision`, { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "operator-decision-denied", "If-Match": "\"1\"" }, body: JSON.stringify({ approved: true, reason: "operator" }) }).then(async (response) => ({ status: response.status, body: await response.json() })), { accessToken, approvalID }); if (denied.status !== 403 || denied.body.error !== "run_control_denied") throw new Error("Run Operator decided a Run Approval"); }'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE role_grants SET team_id = NULL WHERE id = '44444444-4444-4444-8444-444444444444';" >/dev/null
browser --session "$playwright_session" reload
browser --session "$playwright_session" run-code 'async (page) => { if (!(await page.getByTestId("current-user").textContent()).includes("Runtime Team")) throw new Error("OIDC, Team, or route state was not restored after reload"); await page.getByTestId("locale-select").selectOption("zh-CN"); if (await page.locator("html").getAttribute("lang") !== "zh-CN" || !(await page.locator("body").textContent()).includes("运维控制台")) throw new Error("Chinese locale was not applied"); await page.setViewportSize({ width: 390, height: 844 }); const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth); if (overflow) throw new Error("390px viewport has horizontal overflow"); const localValues = await page.evaluate(() => Object.values(localStorage)); if (localValues.some((value) => /access_token|refresh_token/i.test(value))) throw new Error("OIDC token data was written to localStorage"); await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); if (await page.locator(".shell").count()) throw new Error("protected shell remained visible after logout"); }'

for secret_canary in "${secret_canaries[@]}"; do
  if rg --fixed-strings --quiet -- "$secret_canary" "$acceptance_tmp/api.log" "$acceptance_tmp/web.log" "$acceptance_tmp/browser.log"; then
    echo "Secret canary appeared in application or test output" >&2
    exit 1
  fi
done

echo "OIDC browser acceptance passed"
