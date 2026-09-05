#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
deploy_host="${PLATFORM_DEPLOY_HOST:-agent-platform}"
deploy_root="${PLATFORM_DEPLOY_ROOT:-/opt/agent-platform}"
remote_env_file="${PLATFORM_ENV_FILE:-${deploy_root}/config/platform.env}"
remote_config_file="${PLATFORM_CONFIG_FILE:-${deploy_root}/config/platform.https.yaml}"
release_id="${PLATFORM_RELEASE_ID:-platform-$(date -u +%Y%m%dT%H%M%SZ)}"
release_dir="${deploy_root}/src.release-${release_id}"
backup_dir="${deploy_root}/backups/pre-${release_id}"
web_release_root="${WEB_RELEASE_ROOT:-${deploy_root}/web}"
business_database_container="${BUSINESS_DATABASE_CONTAINER:-agent-platform-postgres-1}"
identity_database_container="${IDENTITY_DATABASE_CONTAINER:-agent-platform-identity-db-1}"
api_container="${API_CONTAINER:-agent-platform-api-1}"
worker_container="${WORKER_CONTAINER:-agent-platform-worker-1}"
egress_controller_container="${EGRESS_CONTROLLER_CONTAINER:-agent-platform-egress-controller-1}"
skip_gates="${SKIP_DEPLOY_GATES:-0}"

usage() {
  cat <<'USAGE'
Usage: scripts/deploy-platform.sh

Build, back up, migrate, and deploy the complete single-Worker platform.

Optional environment:
  PLATFORM_DEPLOY_HOST       SSH destination (default: agent-platform)
  PLATFORM_DEPLOY_ROOT       Remote installation root (default: /opt/agent-platform)
  PLATFORM_ENV_FILE          Remote Compose env file
  PLATFORM_CONFIG_FILE       Remote API/Worker YAML configuration
  PLATFORM_RELEASE_ID        Immutable source and Web release identifier
  WEB_RELEASE_ROOT           Remote Web release root
  SKIP_DEPLOY_GATES=1        Skip local test/build gates for an emergency release
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if [[ $# -ne 0 ]]; then
  usage >&2
  exit 1
fi

fail() {
  echo "deployment failed: $*" >&2
  exit 1
}

stage() {
  printf '\n==> %s\n' "$1"
}

require_command() {
  command -v "$1" >/dev/null || fail "required command is unavailable: $1"
}

validate_remote_path() {
  local value="$1"
  local name="$2"
  if [[ ! "$value" =~ ^/[A-Za-z0-9._/-]+$ ]] || [[ "$value" == "/" ]] || [[ "$value" == *"//"* ]] || [[ "$value" == *"/../"* ]] || [[ "$value" == *"/./"* ]] || [[ "$value" == */.. ]] || [[ "$value" == */. ]]; then
    fail "$name must be a specific absolute path without traversal"
  fi
}

for command_name in git make pnpm rsync ssh; do
  require_command "$command_name"
done
[[ "$deploy_host" =~ ^[A-Za-z0-9_.@:-]+$ ]] || fail "PLATFORM_DEPLOY_HOST contains unsupported characters"
[[ "$release_id" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || fail "PLATFORM_RELEASE_ID contains unsupported characters"
[[ "$business_database_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "BUSINESS_DATABASE_CONTAINER contains unsupported characters"
[[ "$identity_database_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "IDENTITY_DATABASE_CONTAINER contains unsupported characters"
[[ "$api_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "API_CONTAINER contains unsupported characters"
[[ "$worker_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "WORKER_CONTAINER contains unsupported characters"
[[ "$egress_controller_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "EGRESS_CONTROLLER_CONTAINER contains unsupported characters"
[[ "$skip_gates" == "0" || "$skip_gates" == "1" ]] || fail "SKIP_DEPLOY_GATES must be 0 or 1"
validate_remote_path "$deploy_root" PLATFORM_DEPLOY_ROOT
validate_remote_path "$remote_env_file" PLATFORM_ENV_FILE
validate_remote_path "$remote_config_file" PLATFORM_CONFIG_FILE
validate_remote_path "$release_dir" release_dir
validate_remote_path "$backup_dir" backup_dir
validate_remote_path "$web_release_root" WEB_RELEASE_ROOT

unexpected_env="$(find "$repo_root" \
  \( -path "$repo_root/.git" -o -path "$repo_root/.scratch" -o -path '*/node_modules' -o -path '*/dist' -o -path '*/coverage' \) -prune -o \
  -type f \( -name '.env' -o -name '.env.*' ! -name '.env.example' \) -print -quit)"
[[ -z "$unexpected_env" ]] || fail "refusing to upload local environment file: $unexpected_env"
unexpected_link="$(find "$repo_root" \
  \( -path "$repo_root/.git" -o -path "$repo_root/.scratch" -o -path '*/node_modules' -o -path '*/dist' -o -path '*/coverage' \) -prune -o \
  -type l -print -quit)"
[[ -z "$unexpected_link" ]] || fail "refusing to upload source symlink: $unexpected_link"

latest_migration="$(find "$repo_root/backend/internal/infrastructure/gormdb/migrations" -maxdepth 1 -type f -name '*.sql' -exec basename {} \; | sort | tail -n 1)"
[[ -n "$latest_migration" && "$latest_migration" =~ ^[A-Za-z0-9._-]+$ ]] || fail "latest database migration could not be resolved"

stage "Validate local release"
if [[ "$skip_gates" == "0" ]]; then
  make -C "$repo_root" test
  make -C "$repo_root" build
  pnpm --dir "$repo_root/frontend" test
  make -C "$repo_root" web-typecheck
else
  echo "warning: local test/build gates were skipped" >&2
fi
git -C "$repo_root" diff --check

stage "Read public Web configuration"
read_public_config() {
  ssh "$deploy_host" bash -s -- "$remote_env_file" 2>/dev/null <<'REMOTE_PUBLIC_CONFIG'
set -euo pipefail
env_file=$1
test -r "$env_file"
set -a
. "$env_file"
set +a
for name in PUBLIC_HOST VITE_OIDC_AUTHORITY VITE_OIDC_CLIENT_ID VITE_OIDC_REDIRECT_URI VITE_OIDC_POST_LOGOUT_REDIRECT_URI; do
  eval "value=\${$name:-}"
  test -n "$value"
  case "$value" in *$'\n'*|*$'\r'*) exit 1 ;; esac
  printf '%s\n' "$value"
done
REMOTE_PUBLIC_CONFIG
}
if ! public_config="$(read_public_config)"; then
  fail "could not read public OIDC values from the remote env file"
fi
public_values=()
while IFS= read -r value; do
  public_values[${#public_values[@]}]="$value"
done <<< "$public_config"
[[ ${#public_values[@]} -eq 5 ]] || fail "remote public Web configuration is incomplete"
public_host="${public_values[0]}"
oidc_authority="${public_values[1]}"
oidc_client_id="${public_values[2]}"
oidc_redirect_uri="${public_values[3]}"
oidc_post_logout_redirect_uri="${public_values[4]}"
[[ "$public_host" =~ ^[A-Za-z0-9.-]+(:[0-9]+)?$ ]] || fail "remote PUBLIC_HOST is invalid"
[[ "$oidc_authority" == https://* ]] || fail "remote VITE_OIDC_AUTHORITY must use HTTPS"
[[ "$oidc_client_id" =~ ^[A-Za-z0-9._-]+$ ]] || fail "remote VITE_OIDC_CLIENT_ID is invalid"
[[ "$oidc_redirect_uri" == https://* ]] || fail "remote VITE_OIDC_REDIRECT_URI must use HTTPS"
[[ "$oidc_post_logout_redirect_uri" == https://* ]] || fail "remote VITE_OIDC_POST_LOGOUT_REDIRECT_URI must use HTTPS"
public_origin="https://${public_host}"

stage "Build production Web assets"
VITE_OIDC_AUTHORITY="$oidc_authority" \
VITE_OIDC_CLIENT_ID="$oidc_client_id" \
VITE_OIDC_REDIRECT_URI="$oidc_redirect_uri" \
VITE_OIDC_POST_LOGOUT_REDIRECT_URI="$oidc_post_logout_redirect_uri" \
pnpm --dir "$repo_root/frontend" build

stage "Check remote deployment"
ssh "$deploy_host" bash -s -- "$deploy_root" "$release_dir" "$backup_dir" "$remote_env_file" "$remote_config_file" <<'REMOTE_PREFLIGHT'
set -euo pipefail
deploy_root=$1
release_dir=$2
backup_dir=$3
env_file=$4
config_file=$5
for command_name in curl df docker rsync tar; do
  command -v "$command_name" >/dev/null
done
docker compose version >/dev/null
test -d "$deploy_root"
test -L "$deploy_root/src"
test -r "$env_file"
test -r "$config_file"
test ! -e "$release_dir" && test ! -L "$release_dir"
test ! -e "$backup_dir" && test ! -L "$backup_dir"
available_kb=$(df -Pk "$deploy_root" | awk 'NR == 2 {print $4}')
test "${available_kb:-0}" -ge 1048576
REMOTE_PREFLIGHT

stage "Create and verify remote backup"
ssh "$deploy_host" bash -s -- "$deploy_root" "$backup_dir" "$business_database_container" "$identity_database_container" <<'REMOTE_BACKUP'
set -euo pipefail
umask 077
deploy_root=$1
backup_dir=$2
business_database_container=$3
identity_database_container=$4
install -d -m 0700 "$backup_dir"
docker exec "$business_database_container" sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup_dir/business.pgdump"
docker exec "$identity_database_container" sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup_dir/identity.pgdump"
tar -czf "$backup_dir/config.tar.gz" -C "$deploy_root" config
readlink -f "$deploy_root/src" > "$backup_dir/previous-source.txt"
readlink -f "$deploy_root/web/current" > "$backup_dir/previous-web.txt"
docker exec -i "$business_database_container" pg_restore -l < "$backup_dir/business.pgdump" >/dev/null
docker exec -i "$identity_database_container" pg_restore -l < "$backup_dir/identity.pgdump" >/dev/null
(
  cd "$backup_dir"
  sha256sum business.pgdump identity.pgdump config.tar.gz previous-source.txt previous-web.txt > SHA256SUMS
  sha256sum -c SHA256SUMS
)
chmod -R go-rwx "$backup_dir"
REMOTE_BACKUP

stage "Upload immutable source release"
ssh "$deploy_host" "install -d -m 0755 '$release_dir'"
rsync --archive --checksum \
  --exclude='.git/' \
  --exclude='.scratch/' \
  --exclude='node_modules/' \
  --exclude='dist/' \
  --exclude='coverage/' \
  "$repo_root/" "$deploy_host:$release_dir/"

stage "Build service images"
ssh "$deploy_host" bash -s -- "$release_dir" "$remote_env_file" "$remote_config_file" <<'REMOTE_BUILD'
set -euo pipefail
release_dir=$1
env_file=$2
config_file=$3
cd "$release_dir"
test -s backend/go.mod
test -s frontend/package.json
compose_args=(--env-file "$env_file" -f deploy/platform/compose.yaml -f deploy/platform/compose.execution.yaml -f deploy/platform/compose.https.yaml)
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" config --quiet
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" build api worker egress-controller
REMOTE_BUILD

stage "Activate source, migrate, and replace services"
if ! ssh "$deploy_host" bash -s -- "$deploy_root" "$release_dir" "$release_id" "$remote_env_file" "$remote_config_file" "$latest_migration" "$business_database_container" "$api_container" "$worker_container" "$egress_controller_container" <<'REMOTE_CUTOVER'
set -euo pipefail
deploy_root=$1
release_dir=$2
release_id=$3
env_file=$4
config_file=$5
latest_migration=$6
business_database_container=$7
api_container=$8
worker_container=$9
egress_controller_container=${10}

wait_healthy() {
  container=$1
  for attempt in $(seq 1 60); do
    health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)
    if [[ "$health" == healthy ]]; then
      return 0
    fi
    if [[ "$health" == exited || "$health" == unhealthy ]]; then
      docker logs --tail 80 "$container" >&2
      return 1
    fi
    sleep 2
  done
  docker logs --tail 80 "$container" >&2 || true
  return 1
}

current_source=$(readlink -f "$deploy_root/src")
test -d "$current_source"
test -d "$release_dir"
next_previous="$deploy_root/.src.previous-${release_id}"
next_source="$deploy_root/.src-${release_id}"
test ! -e "$next_previous"
test ! -e "$next_source"
ln -s "$current_source" "$next_previous"
mv -Tf "$next_previous" "$deploy_root/src.previous"
ln -s "$release_dir" "$next_source"
mv -Tf "$next_source" "$deploy_root/src"

cd "$release_dir"
compose_args=(--env-file "$env_file" -f deploy/platform/compose.yaml -f deploy/platform/compose.execution.yaml -f deploy/platform/compose.https.yaml)
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" stop worker
# Warm containers are disposable definitions tied to the previous release's
# mount/config schema. Purge only explicitly managed warm caches at cutover.
while IFS= read -r warm_container; do
  [[ -z "$warm_container" ]] && continue
  [[ "$warm_container" =~ ^agent-runtime-warm-[a-f0-9]{32}$ ]] || exit 1
  docker rm --force "$warm_container" >/dev/null
done < <(docker ps --all \
  --filter "label=agent-platform.managed=true" \
  --filter "label=agent-platform.warm=true" \
  --format '{{.Names}}')
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate api
wait_healthy "$api_container"

migration_count=$(docker exec "$business_database_container" sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' sh "SELECT count(*) FROM schema_migrations WHERE name = '$latest_migration'")
test "$migration_count" = 1

PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate egress-controller
wait_healthy "$egress_controller_container"
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate worker
wait_healthy "$worker_container"
REMOTE_CUTOVER
then
  echo "cutover stopped; Worker may be intentionally stopped to protect the migrated database" >&2
  echo "backup: $backup_dir" >&2
  echo "previous source: $deploy_root/src.previous" >&2
  exit 1
fi

stage "Deploy Web release"
WEB_DEPLOY_HOST="$deploy_host" \
WEB_RELEASE_ROOT="$web_release_root" \
WEB_RELEASE_ID="$release_id" \
VITE_OIDC_AUTHORITY="$oidc_authority" \
VITE_OIDC_CLIENT_ID="$oidc_client_id" \
VITE_OIDC_REDIRECT_URI="$oidc_redirect_uri" \
VITE_OIDC_POST_LOGOUT_REDIRECT_URI="$oidc_post_logout_redirect_uri" \
"$repo_root/scripts/deploy-web.sh"

stage "Verify deployed release"
ssh "$deploy_host" bash -s -- "$deploy_root" "$release_dir" "$web_release_root" "$release_id" "$public_origin" "$api_container" "$worker_container" "$egress_controller_container" <<'REMOTE_VERIFY'
set -euo pipefail
deploy_root=$1
release_dir=$2
web_release_root=$3
release_id=$4
public_origin=$5
api_container=$6
worker_container=$7
egress_controller_container=$8
test "$(readlink -f "$deploy_root/src")" = "$release_dir"
test "$(readlink -f "$web_release_root/current")" = "$web_release_root/releases/$release_id"
test "$(docker inspect --format '{{.State.Health.Status}}' "$api_container")" = healthy
test "$(docker inspect --format '{{.State.Health.Status}}' "$worker_container")" = healthy
test "$(docker inspect --format '{{.State.Health.Status}}' "$egress_controller_container")" = healthy
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$api_container")" = "$release_dir/deploy/platform"
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$worker_container")" = "$release_dir/deploy/platform"
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$egress_controller_container")" = "$release_dir/deploy/platform"
curl --fail --silent --show-error "$public_origin/" >/dev/null
test "$(curl --fail --silent --show-error -o /dev/null -w '%{http_code}' "$public_origin/api/healthz")" = 200
test "$(curl --fail --silent --show-error -o /dev/null -w '%{http_code}' "$public_origin/api/readyz")" = 200
test "$(curl --fail --silent --show-error -o /dev/null -w '%{http_code}' "$public_origin/identity/realms/agent-platform/.well-known/openid-configuration")" = 200
test "$(curl --silent --show-error --head -o /dev/null -w '%{http_code}' "${public_origin/https:/http:}/")" = 308
api_errors=$(docker logs "$api_container" 2>&1 | grep -Eic 'panic|fatal|level=error|"level":"error"' || true)
worker_errors=$(docker logs "$worker_container" 2>&1 | grep -Eic 'panic|fatal|level=error|"level":"error"' || true)
egress_controller_errors=$(docker logs "$egress_controller_container" 2>&1 | grep -Eic 'panic|fatal|level=error|"level":"error"' || true)
test "$api_errors" = 0
test "$worker_errors" = 0
test "$egress_controller_errors" = 0
printf 'api_image=%s\n' "$(docker inspect --format '{{.Image}}' "$api_container")"
printf 'worker_image=%s\n' "$(docker inspect --format '{{.Image}}' "$worker_container")"
printf 'egress_controller_image=%s\n' "$(docker inspect --format '{{.Image}}' "$egress_controller_container")"
REMOTE_VERIFY

printf '\nDeployment complete\n'
printf '  release: %s\n' "$release_id"
printf '  source:  %s:%s\n' "$deploy_host" "$release_dir"
printf '  web:     %s:%s/releases/%s\n' "$deploy_host" "$web_release_root" "$release_id"
printf '  backup:  %s:%s\n' "$deploy_host" "$backup_dir"
printf '  public:  %s\n' "$public_origin"
