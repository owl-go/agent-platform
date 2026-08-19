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
api_pid=""
web_pid=""
pwcli="${PWCLI:-playwright-cli}"

cleanup() {
  cleanup_status=$?
  if [[ "$cleanup_status" -ne 0 ]]; then
    [[ -f "$acceptance_tmp/api.log" ]] && sed "s/$model_secret_canary/[REDACTED]/g" "$acceptance_tmp/api.log" >&2
    [[ -f "$acceptance_tmp/web.log" ]] && sed "s/$model_secret_canary/[REDACTED]/g" "$acceptance_tmp/web.log" >&2
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
  docker rm -f "$keycloak_container" "$postgres_container" "$minio_container" >/dev/null 2>&1 || true
  rm -rf "$acceptance_tmp"
}
trap cleanup EXIT

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
    sed "s/$model_secret_canary/[REDACTED]/g" "$acceptance_tmp/browser.log" >&2
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
go -C "$repository_root/backend" test -count=1 -run '^TestProviderConformance$' ./internal/objectstore/minio >/dev/null

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

go -C "$repository_root/backend" build -o "$acceptance_tmp/api" ./cmd/api

(
  cd "$repository_root/backend"
  export OIDC_BROWSER_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable'
  export MODEL_SECRET_CANARY="$model_secret_canary"
  exec "$acceptance_tmp/api" -config testdata/oidc-browser/platform.yaml
) >"$acceptance_tmp/api.log" 2>&1 &
api_pid=$!

for _ in {1..60}; do
  curl -fsS http://127.0.0.1:18090/readyz >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS http://127.0.0.1:18090/readyz >/dev/null; then
  sed "s/$model_secret_canary/[REDACTED]/g" "$acceptance_tmp/api.log" >&2
  exit 1
fi

EXECUTION_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable' \
  go -C "$repository_root/backend" test -count=1 -run '^TestRepositorySerializesCredentialDisableAndModelEnable$' ./internal/data/modelcatalog/gormrepo >/dev/null

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "INSERT INTO organizations (id, slug, name) VALUES ('22222222-2222-4222-8222-222222222222', 'acme', 'Acme');
   INSERT INTO organizations (id, slug, name) VALUES ('77777777-7777-4777-8777-777777777777', 'other', 'Other Organization');
   INSERT INTO teams (id, organization_id, slug, name) VALUES
     ('55555555-5555-4555-8555-555555555555', '22222222-2222-4222-8222-222222222222', 'platform', 'Platform Team'),
     ('66666666-6666-4666-8666-666666666666', '22222222-2222-4222-8222-222222222222', 'runtime', 'Runtime Team');
   INSERT INTO users (id, organization_id, oidc_subject, email, display_name) VALUES ('33333333-3333-4333-8333-333333333333', '22222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111', 'platform-user@example.test', 'Platform User');
   INSERT INTO role_grants (id, organization_id, user_id, role) VALUES ('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222', '33333333-3333-4333-8333-333333333333', 'platform_administrator');
   INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES ('99999999-9999-4999-8999-999999999999', '22222222-2222-4222-8222-222222222222', '55555555-5555-4555-8555-555555555555', 'hidden-team-model-key', 'model', 'vault://acceptance/hidden-team-model');
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
  sed "s/$model_secret_canary/[REDACTED]/g" "$acceptance_tmp/web.log" >&2
  exit 1
fi

browser --session "$playwright_session" open http://127.0.0.1:18092
browser --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("platform-user"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=55555555/); const user = page.getByTestId("current-user"); const text = await user.textContent(); if (!text.includes("Platform User") || !text.includes("Acme") || !text.includes("Platform Team")) throw new Error("real User and Team bootstrap was not rendered"); await page.getByTestId("team-select").selectOption("66666666-6666-4666-8666-666666666666"); await page.waitForURL(/team=66666666/); await page.getByTestId("nav-operations").click(); await page.waitForURL(/operations/); }'
browser --session "$playwright_session" run-code 'async (page) => { const digest = "registry.example/codex@sha256:" + "c".repeat(64); await page.getByTestId("nav-studio").click(); await page.waitForURL(/studio/); await page.getByTestId("register-runtime").click(); await page.getByTestId("runtime-cli-version").fill("1.0.0"); await page.getByTestId("runtime-adapter-version").fill("2026.08"); await page.getByTestId("runtime-digest").fill(digest); await page.getByTestId("submit-runtime").click(); await page.getByTestId("runtime-detail").waitFor(); if (!(await page.getByTestId("runtime-detail").textContent()).includes(digest)) throw new Error("registered Runtime Image was not rendered from the API"); await page.getByTestId("runtime-status").selectOption("production"); await page.getByTestId("conformance-evidence-key").fill("phase-0/acceptance/codex/evidence.tar"); await page.getByTestId("save-runtime-status").click(); await page.getByTestId("runtime-notice").waitFor(); const runtimeButton = page.locator(".catalog-list > button").first(); const runtimeID = (await runtimeButton.getAttribute("data-testid")).replace("runtime-", ""); const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); if (!accessToken) throw new Error("OIDC access token was not available in session storage"); const invalidEvidence = await page.evaluate(async ({ accessToken, runtimeID }) => fetch(`/api/v1/runtime-images/${runtimeID}/status`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-invalid-evidence", "If-Match": "2" }, body: JSON.stringify({ status: "production", conformance_evidence_key: "arbitrary/missing.tar" }) }).then(async (response) => ({ status: response.status, body: await response.json() })), { accessToken, runtimeID }); if (invalidEvidence.status !== 422 || invalidEvidence.body.error !== "invalid_conformance_evidence") throw new Error("unverified evidence key certified Production"); const crossOrganization = await page.evaluate(async (accessToken) => fetch("/api/v1/runtime-images/88888888-8888-4888-8888-888888888888", { headers: { Authorization: `Bearer ${accessToken}` } }).then((response) => response.status), accessToken); if (crossOrganization !== 404 || (await page.locator("body").textContent()).includes("registry.example/private")) throw new Error("cross-Organization Runtime Image was visible"); const replayDigest = "registry.example/hermes@sha256:" + "d".repeat(64); const replayBody = JSON.stringify({ runtime: "hermes", cli_version: "2.0.0", adapter_version: "2026.08", image_digest: replayDigest, capabilities: { streaming: true } }); const headers = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-replay-intent" }; const first = await page.evaluate(async ({ headers, replayBody }) => { const response = await fetch("/api/v1/runtime-images", { method: "POST", headers, body: replayBody }); return { status: response.status, body: await response.json() }; }, { headers, replayBody }); const replay = await page.evaluate(async ({ headers, replayBody }) => { const response = await fetch("/api/v1/runtime-images", { method: "POST", headers, body: replayBody }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { headers, replayBody }); if (first.status !== 201 || replay.status !== 201 || replay.replayed !== "true" || first.body.id !== replay.body.id) throw new Error("idempotent Runtime Image registration was not replayed"); const conflict = await page.evaluate(async ({ accessToken, runtimeID }) => fetch(`/api/v1/runtime-images/${runtimeID}/status`, { method: "PATCH", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-stale-intent", "If-Match": "1" }, body: JSON.stringify({ status: "blocked", blocked_reason: "stale update" }) }).then(async (response) => ({ status: response.status, body: await response.json() })), { accessToken, runtimeID }); if (conflict.status !== 412 || conflict.body.error !== "version_conflict") throw new Error("stale Runtime Image Version was not rejected"); await page.reload(); await page.getByTestId("runtime-detail").waitFor(); if (!(await page.locator("body").textContent()).includes("Production Runtime") || !(await page.locator("body").textContent()).includes(digest)) throw new Error("Runtime Catalog did not persist across refresh"); }'

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
  const runtimeButton = page.locator(".catalog-list > button").first();
  const runtimeID = (await runtimeButton.getAttribute("data-testid")).replace("runtime-", "");
  const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; });
  const body = JSON.stringify({ status: "blocked", blocked_reason: "acceptance replay" });
  const headers = { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-status-replay", "If-Match": "2" };
  const write = async () => page.evaluate(async ({ runtimeID, headers, body }) => { const response = await fetch(`/api/v1/runtime-images/${runtimeID}/status`, { method: "PATCH", headers, body }); return { status: response.status, replayed: response.headers.get("Idempotency-Replayed"), body: await response.json() }; }, { runtimeID, headers, body });
  const first = await write();
  const replay = await write();
  const evidenceKey = "phase-0/acceptance/codex/evidence.tar";
  if (first.status !== 200 || replay.status !== 200 || replay.replayed !== "true" || first.body.version !== 3 || replay.body.version !== 3 || replay.body.conformance_evidence_key !== evidenceKey || !replay.body.conformance_evidence_sha256) throw new Error("Runtime status write was not replayed from its persisted response");
  await page.reload();
  await page.getByTestId("runtime-detail").waitFor();
  if (!(await page.getByTestId("runtime-detail").textContent()).includes(evidenceKey)) throw new Error("retained evidence was presented as missing after leaving Production");
}'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE role_grants SET team_id = '55555555-5555-4555-8555-555555555555' WHERE id = '44444444-4444-4444-8444-444444444444';" >/dev/null
browser --session "$playwright_session" run-code 'async (page) => { const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const response = await page.evaluate(async (accessToken) => fetch("/api/v1/runtime-images", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "browser-denied-intent" }, body: JSON.stringify({ runtime: "openclaw", cli_version: "1", adapter_version: "1", image_digest: "registry.example/openclaw@sha256:" + "e".repeat(64) }) }).then(async (result) => ({ status: result.status, body: await result.json() })), accessToken); if (response.status !== 403 || response.body.error !== "catalog_write_access_denied") throw new Error("Team-scoped Platform Administrator modified the Organization Runtime Catalog"); }'
browser --session "$playwright_session" run-code 'async (page) => { const accessToken = await page.evaluate(() => { for (const value of Object.values(sessionStorage)) { try { const parsed = JSON.parse(value); if (typeof parsed.access_token === "string") return parsed.access_token; } catch {} } return ""; }); const response = await page.evaluate(async (accessToken) => fetch("/api/v1/credential-profiles", { method: "POST", headers: { Authorization: `Bearer ${accessToken}`, "Content-Type": "application/json", "Idempotency-Key": "model-catalog-denied" }, body: JSON.stringify({ name: "denied-model-key", kind: "model", secret_ref: "vault://denied/model" }) }).then(async (result) => ({ status: result.status, body: await result.json() })), accessToken); if (response.status !== 403 || response.body.error !== "catalog_write_access_denied") throw new Error("Team-scoped Platform Administrator modified the Model Catalog"); }'

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "UPDATE role_grants SET team_id = NULL WHERE id = '44444444-4444-4444-8444-444444444444';" >/dev/null
browser --session "$playwright_session" reload
browser --session "$playwright_session" run-code 'async (page) => { if (!(await page.getByTestId("current-user").textContent()).includes("Runtime Team")) throw new Error("OIDC, Team, or route state was not restored after reload"); await page.getByTestId("locale-select").selectOption("zh-CN"); if (await page.locator("html").getAttribute("lang") !== "zh-CN" || !(await page.locator("body").textContent()).includes("运维控制台")) throw new Error("Chinese locale was not applied"); await page.setViewportSize({ width: 390, height: 844 }); const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth); if (overflow) throw new Error("390px viewport has horizontal overflow"); const localValues = await page.evaluate(() => Object.values(localStorage)); if (localValues.some((value) => /access_token|refresh_token/i.test(value))) throw new Error("OIDC token data was written to localStorage"); await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); if (await page.locator(".shell").count()) throw new Error("protected shell remained visible after logout"); }'

if rg --fixed-strings --quiet -- "$model_secret_canary" "$acceptance_tmp/api.log" "$acceptance_tmp/web.log" "$acceptance_tmp/browser.log"; then
  echo "model Secret canary appeared in application or test output" >&2
  exit 1
fi

echo "OIDC browser acceptance passed"
