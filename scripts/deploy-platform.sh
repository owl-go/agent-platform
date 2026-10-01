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
caddy_container="${CADDY_CONTAINER:-agent-platform-caddy-1}"
retired_retrieval_container="agent-platform-anythingllm-1"
retired_retrieval_volume="agent-platform_anythingllm-data"
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
[[ "$caddy_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "CADDY_CONTAINER contains unsupported characters"
[[ "$retired_retrieval_container" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "retired retrieval container name contains unsupported characters"
[[ "$retired_retrieval_volume" =~ ^[A-Za-z0-9_.-]+$ ]] || fail "retired retrieval volume name contains unsupported characters"
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
for command_name in curl df docker python3 rsync tar; do
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

stage "Build unified Runtime, CLI Builder, and service images"
ssh "$deploy_host" bash -s -- "$release_dir" "$release_id" "$remote_env_file" "$remote_config_file" <<'REMOTE_BUILD'
set -euo pipefail
release_dir=$1
release_id=$2
env_file=$3
config_file=$4
cd "$release_dir"
test -s backend/go.mod
test -s frontend/package.json
candidate_env_file="${env_file}.candidate-${release_id}"
candidate_config_file="${config_file}.candidate-${release_id}"
test ! -e "$candidate_env_file" && test ! -L "$candidate_env_file"
test ! -e "$candidate_config_file" && test ! -L "$candidate_config_file"
cp -p "$env_file" "$candidate_env_file"
cp -p "$config_file" "$candidate_config_file"
trap 'rm -f -- "$candidate_env_file" "$candidate_config_file" "${candidate_env_file}.next" "${candidate_config_file}.next"' EXIT
set -a
. "$env_file"
set +a

runtime_reference=${RUNTIME_IMAGE:-}
if [[ -n "$runtime_reference" ]]; then
    runtime_repository=${runtime_reference%@sha256:*}
else
    legacy_runtime_reference=${CODEX_RUNTIME_IMAGE:-}
    test -n "$legacy_runtime_reference"
    legacy_runtime_repository=${legacy_runtime_reference%@sha256:*}
    test -n "$legacy_runtime_repository"
    test "$legacy_runtime_repository" != "$legacy_runtime_reference"
    runtime_repository=${legacy_runtime_repository%/*}/runtime
fi
[[ "$runtime_repository" =~ ^[^[:space:]@]+$ ]]
runtime_tag="$runtime_repository:$release_id"
# Connector revisions pin the Runtime RepoDigest. Keep rebuilds of unchanged
# Runtime inputs stable instead of publishing a new provenance attestation.
docker build --pull --provenance=false --tag "$runtime_tag" --file deploy/runtimes/unified/Dockerfile .
docker push "$runtime_tag"
runtime_digest=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$runtime_tag" | awk -v prefix="$runtime_repository@" 'index($0, prefix) == 1 { print; exit }')
[[ "$runtime_digest" =~ ^[^[:space:]@]+@sha256:[a-f0-9]{64}$ ]]

builder_reference=$(python3 - "$config_file" <<'PY'
import pathlib, sys
lines = pathlib.Path(sys.argv[1]).read_text().splitlines()
inside = False
for line in lines:
    stripped = line.strip()
    if stripped == "cli_builder:":
        inside = True
        continue
    if inside and stripped.startswith("image_digest:"):
        print(stripped.split(":", 1)[1].strip().strip('"'))
        break
else:
    raise SystemExit("CLI Builder image_digest was not found")
PY
)
builder_repository=${builder_reference%@sha256:*}
test -n "$builder_repository"
test "$builder_repository" != "$builder_reference"
builder_tag="$builder_repository:$release_id"
docker build --pull --tag "$builder_tag" --file deploy/runtimes/cli-builder/Dockerfile .
docker push "$builder_tag"
builder_digest=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$builder_tag" | awk -v prefix="$builder_repository@" 'index($0, prefix) == 1 { print; exit }')
test -n "$builder_digest"
RUNTIME_IMAGE_REF="$runtime_digest" CLI_BUILDER_IMAGE_REF="$builder_digest" scripts/conformance/runtime-image-smoke.sh

python3 - "$candidate_env_file" "$runtime_digest" <<'PY'
import os, pathlib, re, sys
path = pathlib.Path(sys.argv[1])
digest = sys.argv[2]
if not re.fullmatch(r"[^\s@]+@sha256:[a-f0-9]{64}", digest):
    raise SystemExit("Runtime RepoDigest is invalid")
lines = path.read_text().splitlines(keepends=True)
lines = [line for line in lines if not line.startswith("ANYTHINGLLM_")]
replacement = f"RUNTIME_IMAGE={digest}\n"
updated = 0
for index, line in enumerate(lines):
    if line.startswith("RUNTIME_IMAGE="):
        lines[index] = replacement
        updated += 1
if updated > 1:
    raise SystemExit("RUNTIME_IMAGE is defined more than once")
if updated == 0:
    if lines and not lines[-1].endswith("\n"):
        lines[-1] += "\n"
    lines.append(replacement)
temporary = path.with_name(path.name + ".next")
temporary.write_text("".join(lines))
original = path.stat()
os.chown(temporary, original.st_uid, original.st_gid)
os.chmod(temporary, original.st_mode)
os.replace(temporary, path)
PY

python3 - "$candidate_config_file" "$builder_digest" <<'PY'
import os, pathlib, re, sys
path = pathlib.Path(sys.argv[1])
digest = sys.argv[2]
lines = path.read_text().splitlines(keepends=True)
inside = False
updated = 0
for index, line in enumerate(lines):
    stripped = line.strip()
    if stripped == "cli_builder:":
        inside = True
        continue
    if inside and stripped.startswith("image_digest:"):
        indent = line[:len(line) - len(line.lstrip())]
        lines[index] = f'{indent}image_digest: "{digest}"\n'
        updated += 1
        break
if updated != 1:
    raise SystemExit("CLI Builder image_digest could not be updated")
text = "".join(lines)
legacy_pattern = re.compile(r"\$\{(?:CLAUDE|CODEX|HERMES|OPENCLAW|PI)_RUNTIME_IMAGE\}")
legacy_count = len(legacy_pattern.findall(text))
if legacy_count not in (0, 5):
    raise SystemExit(f"expected zero or five legacy Runtime image references, found {legacy_count}")
text = legacy_pattern.sub("${RUNTIME_IMAGE}", text)
filtered = []
inside_retired_retrieval = False
for line in text.splitlines(keepends=True):
    if not line[:1].isspace() and line.strip() == "anythingllm:":
        inside_retired_retrieval = True
        continue
    if inside_retired_retrieval and line.strip() and not line[:1].isspace():
        inside_retired_retrieval = False
    if not inside_retired_retrieval:
        filtered.append(line)
text = "".join(filtered)
temporary = path.with_name(path.name + ".next")
temporary.write_text(text)
original = path.stat()
os.chown(temporary, original.st_uid, original.st_gid)
os.chmod(temporary, original.st_mode)
os.replace(temporary, path)
PY
set -a
. "$candidate_env_file"
set +a
compose_args=(--env-file "$candidate_env_file" -f deploy/platform/compose.yaml -f deploy/platform/compose.execution.yaml -f deploy/platform/compose.https.yaml)
PLATFORM_CONFIG_FILE="$candidate_config_file" docker compose "${compose_args[@]}" config --quiet
PLATFORM_CONFIG_FILE="$candidate_config_file" docker compose "${compose_args[@]}" build api worker egress-controller
echo 'Reverify installed CLI Connector bundles for the candidate Runtime'
PLATFORM_CONFIG_FILE="$candidate_config_file" docker compose "${compose_args[@]}" run --rm --no-deps worker -config /etc/agent-platform/platform.yaml -reverify-cli-connectors
mv -f -- "$candidate_env_file" "$env_file"
mv -f -- "$candidate_config_file" "$config_file"
trap - EXIT
REMOTE_BUILD

stage "Activate source, migrate, and replace services"
if ! ssh "$deploy_host" bash -s -- "$deploy_root" "$release_dir" "$release_id" "$remote_env_file" "$remote_config_file" "$latest_migration" "$business_database_container" "$api_container" "$worker_container" "$egress_controller_container" "$caddy_container" "$retired_retrieval_container" "$retired_retrieval_volume" <<'REMOTE_CUTOVER'
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
caddy_container=${11}
retired_retrieval_container=${12}
retired_retrieval_volume=${13}

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
# API and Worker run as UID 65532 and must be able to traverse every existing
# Workspace directory before the new API is started.
set -a
. "$env_file"
set +a
workspace_root=${WORKSPACE_ROOT:?WORKSPACE_ROOT is required}
[[ "$workspace_root" =~ ^/[A-Za-z0-9._/-]+$ && "$workspace_root" != "/" && "$workspace_root" != *".."* && "$workspace_root" != *"//"* ]] || {
  echo "WORKSPACE_ROOT is not a safe absolute path" >&2
  exit 1
}
test ! -L "$workspace_root"
install -d -m 0700 "$workspace_root"
find "$workspace_root" -xdev -exec chown -h 65532:65532 {} +
chmod 0700 "$workspace_root"
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
retired_retrieval_volumes=()
if docker inspect "$retired_retrieval_container" >/dev/null 2>&1; then
  while IFS= read -r volume_name; do
    [[ -n "$volume_name" ]] && retired_retrieval_volumes+=("$volume_name")
  done < <(docker inspect --format '{{range .Mounts}}{{if eq .Type "volume"}}{{println .Name}}{{end}}{{end}}' "$retired_retrieval_container")
  docker rm --force "$retired_retrieval_container" >/dev/null
fi
retired_retrieval_volumes+=("$retired_retrieval_volume")
for volume_name in "${retired_retrieval_volumes[@]}"; do
  [[ "$volume_name" =~ ^[A-Za-z0-9_.-]+$ ]] || exit 1
  docker volume inspect "$volume_name" >/dev/null 2>&1 || continue
  docker volume rm "$volume_name" >/dev/null
done
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate api
wait_healthy "$api_container"

migration_count=$(docker exec "$business_database_container" sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' sh "SELECT count(*) FROM schema_migrations WHERE name = '$latest_migration'")
test "$migration_count" = 1

PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate egress-controller
wait_healthy "$egress_controller_container"
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate worker
wait_healthy "$worker_container"
PLATFORM_CONFIG_FILE="$config_file" docker compose "${compose_args[@]}" up -d --no-deps --force-recreate caddy
test "$(docker inspect --format '{{.State.Status}}' "$caddy_container")" = running
docker exec "$caddy_container" caddy validate --config /etc/caddy/Caddyfile >/dev/null
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
ssh "$deploy_host" bash -s -- "$deploy_root" "$release_dir" "$web_release_root" "$release_id" "$public_origin" "$api_container" "$worker_container" "$egress_controller_container" "$caddy_container" "$retired_retrieval_container" "$retired_retrieval_volume" <<'REMOTE_VERIFY'
set -euo pipefail
deploy_root=$1
release_dir=$2
web_release_root=$3
release_id=$4
public_origin=$5
api_container=$6
worker_container=$7
egress_controller_container=$8
caddy_container=$9
retired_retrieval_container=${10}
retired_retrieval_volume=${11}
test "$(readlink -f "$deploy_root/src")" = "$release_dir"
test "$(readlink -f "$web_release_root/current")" = "$web_release_root/releases/$release_id"
test "$(docker inspect --format '{{.State.Health.Status}}' "$api_container")" = healthy
test "$(docker inspect --format '{{.State.Health.Status}}' "$worker_container")" = healthy
test "$(docker inspect --format '{{.State.Health.Status}}' "$egress_controller_container")" = healthy
test "$(docker inspect --format '{{.State.Status}}' "$caddy_container")" = running
! docker inspect "$retired_retrieval_container" >/dev/null 2>&1
! docker volume inspect "$retired_retrieval_volume" >/dev/null 2>&1
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$api_container")" = "$release_dir/deploy/platform"
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$worker_container")" = "$release_dir/deploy/platform"
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$egress_controller_container")" = "$release_dir/deploy/platform"
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$caddy_container")" = "$release_dir/deploy/platform"
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
printf 'caddy_image=%s\n' "$(docker inspect --format '{{.Image}}' "$caddy_container")"
REMOTE_VERIFY

printf '\nDeployment complete\n'
printf '  release: %s\n' "$release_id"
printf '  source:  %s:%s\n' "$deploy_host" "$release_dir"
printf '  web:     %s:%s/releases/%s\n' "$deploy_host" "$web_release_root" "$release_id"
printf '  backup:  %s:%s\n' "$deploy_host" "$backup_dir"
printf '  public:  %s\n' "$public_origin"
