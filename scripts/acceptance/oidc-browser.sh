#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
postgres_container="agent-platform-oidc-acceptance-postgres"
keycloak_container="agent-platform-oidc-acceptance-keycloak"
playwright_session="agent-platform-oidc-acceptance-$$"
acceptance_tmp="$(mktemp -d)"
api_pid=""
web_pid=""
pwcli="${PWCLI:-playwright-cli}"

cleanup() {
  "$pwcli" --session "$playwright_session" close >/dev/null 2>&1 || true
  if [[ -n "$web_pid" ]]; then
    kill "$web_pid" >/dev/null 2>&1 || true
    wait "$web_pid" 2>/dev/null || true
  fi
  if [[ -n "$api_pid" ]]; then
    kill "$api_pid" >/dev/null 2>&1 || true
    wait "$api_pid" 2>/dev/null || true
  fi
  docker rm -f "$keycloak_container" "$postgres_container" >/dev/null 2>&1 || true
  rm -rf "$acceptance_tmp"
}
trap cleanup EXIT

if docker ps -a --format '{{.Names}}' | grep -Fxq "$postgres_container" ||
   docker ps -a --format '{{.Names}}' | grep -Fxq "$keycloak_container"; then
  echo "OIDC browser acceptance container names are already in use" >&2
  exit 1
fi
if ! command -v "$pwcli" >/dev/null 2>&1 && [[ ! -x "$pwcli" ]]; then
  echo "Set PWCLI to the Playwright CLI wrapper or install playwright-cli" >&2
  exit 1
fi

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

for _ in {1..90}; do
  if docker exec "$postgres_container" pg_isready -U agent_platform -d agent_platform_oidc >/dev/null 2>&1 &&
     curl -fsS http://127.0.0.1:18091/realms/agent-platform/.well-known/openid-configuration >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$postgres_container" pg_isready -U agent_platform -d agent_platform_oidc >/dev/null
curl -fsS http://127.0.0.1:18091/realms/agent-platform/.well-known/openid-configuration >/dev/null

(
  cd "$repository_root/backend"
  export OIDC_BROWSER_DATABASE_DSN='postgres://agent_platform:acceptance-db-password@127.0.0.1:15432/agent_platform_oidc?sslmode=disable'
  exec go run ./cmd/api -config testdata/oidc-browser/platform.yaml
) >"$acceptance_tmp/api.log" 2>&1 &
api_pid=$!

for _ in {1..60}; do
  curl -fsS http://127.0.0.1:18090/readyz >/dev/null 2>&1 && break
  sleep 1
done
if ! curl -fsS http://127.0.0.1:18090/readyz >/dev/null; then
  cat "$acceptance_tmp/api.log" >&2
  exit 1
fi

docker exec "$postgres_container" psql -v ON_ERROR_STOP=1 -U agent_platform -d agent_platform_oidc -c \
  "INSERT INTO organizations (id, slug, name) VALUES ('22222222-2222-4222-8222-222222222222', 'acme', 'Acme');
   INSERT INTO teams (id, organization_id, slug, name) VALUES
     ('55555555-5555-4555-8555-555555555555', '22222222-2222-4222-8222-222222222222', 'platform', 'Platform Team'),
     ('66666666-6666-4666-8666-666666666666', '22222222-2222-4222-8222-222222222222', 'runtime', 'Runtime Team');
   INSERT INTO users (id, organization_id, oidc_subject, email, display_name) VALUES ('33333333-3333-4333-8333-333333333333', '22222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111', 'platform-user@example.test', 'Platform User');
   INSERT INTO role_grants (id, organization_id, user_id, role) VALUES ('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222', '33333333-3333-4333-8333-333333333333', 'platform_administrator');" >/dev/null

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
  cat "$acceptance_tmp/web.log" >&2
  exit 1
fi

"$pwcli" --session "$playwright_session" open http://127.0.0.1:18092
"$pwcli" --session "$playwright_session" run-code 'async (page) => { await page.getByTestId("sign-in-button").click(); await page.getByRole("textbox", { name: "Username or email" }).fill("platform-user"); await page.getByRole("textbox", { name: "Password", exact: true }).fill("acceptance-only-password"); await page.getByRole("button", { name: "Sign In" }).click(); await page.waitForURL(/team=55555555/); const user = page.getByTestId("current-user"); const text = await user.textContent(); if (!text.includes("Platform User") || !text.includes("Acme") || !text.includes("Platform Team")) throw new Error("real User and Team bootstrap was not rendered"); await page.getByTestId("team-select").selectOption("66666666-6666-4666-8666-666666666666"); await page.waitForURL(/team=66666666/); await page.getByTestId("nav-operations").click(); await page.waitForURL(/operations/); }'
"$pwcli" --session "$playwright_session" reload
"$pwcli" --session "$playwright_session" run-code 'async (page) => { if (!(await page.getByTestId("current-user").textContent()).includes("Runtime Team")) throw new Error("OIDC, Team, or route state was not restored after reload"); await page.getByTestId("locale-select").selectOption("zh-CN"); if (await page.locator("html").getAttribute("lang") !== "zh-CN" || !(await page.locator("body").textContent()).includes("运维控制台")) throw new Error("Chinese locale was not applied"); await page.setViewportSize({ width: 390, height: 844 }); const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth); if (overflow) throw new Error("390px viewport has horizontal overflow"); const localValues = await page.evaluate(() => Object.values(localStorage)); if (localValues.some((value) => /access_token|refresh_token/i.test(value))) throw new Error("OIDC token data was written to localStorage"); await page.getByTestId("sign-out").click(); await page.getByTestId("sign-in-button").waitFor(); if (await page.locator(".shell").count()) throw new Error("protected shell remained visible after logout"); }'

echo "OIDC browser acceptance passed"
