#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
deploy_host="${WEB_DEPLOY_HOST:?WEB_DEPLOY_HOST is required, for example agent-platform}"
release_root="${WEB_RELEASE_ROOT:-/opt/agent-platform/web}"
mode="${1:-deploy}"
if [[ "$mode" == "activate" ]]; then
  release_id="${2:?usage: deploy-web.sh activate RELEASE_ID}"
elif [[ "$mode" == "deploy" ]] && [[ $# -le 1 ]]; then
  release_id="${WEB_RELEASE_ID:-$(git -C "$repo_root" rev-parse --short=12 HEAD)}"
else
  echo "usage: deploy-web.sh [deploy | activate RELEASE_ID]" >&2
  exit 1
fi
dist_dir="$repo_root/frontend/dist"

if [[ ! "$deploy_host" =~ ^[A-Za-z0-9_.@:-]+$ ]]; then
  echo "WEB_DEPLOY_HOST contains unsupported characters" >&2
  exit 1
fi
if [[ ! "$release_root" =~ ^/[A-Za-z0-9._/-]+$ ]] || [[ "$release_root" == "/" ]] || [[ "$release_root" == *"//"* ]] || [[ "$release_root" == *"/../"* ]] || [[ "$release_root" == *"/./"* ]] || [[ "$release_root" == */.. ]] || [[ "$release_root" == */. ]]; then
  echo "WEB_RELEASE_ROOT must be a specific absolute path without traversal" >&2
  exit 1
fi
if [[ ! "$release_id" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "WEB_RELEASE_ID contains unsupported characters" >&2
  exit 1
fi

activate_release() {
  ssh "$deploy_host" sh -s -- "$release_root" "$release_id" <<'REMOTE_ACTIVATE'
set -eu
release_root=$1
release_id=$2
release_dir="$release_root/releases/$release_id"
test -s "$release_dir/index.html"
test -d "$release_dir/assets"
test -n "$(find "$release_dir/assets" -type f -print -quit)"
test -z "$(find "$release_dir" -type l -print -quit)"
next_link="$release_root/.current-$release_id"
test ! -e "$next_link"
ln -s "releases/$release_id" "$next_link"
mv -Tf "$next_link" "$release_root/current"
test "$(readlink -f "$release_root/current")" = "$release_dir"
REMOTE_ACTIVATE
}

if [[ "$mode" == "activate" ]]; then
  activate_release
  printf 'Web release %s is active at %s:%s/current\n' "$release_id" "$deploy_host" "$release_root"
  exit 0
fi

for name in VITE_OIDC_AUTHORITY VITE_OIDC_CLIENT_ID VITE_OIDC_REDIRECT_URI VITE_OIDC_POST_LOGOUT_REDIRECT_URI; do
  if [[ -z "${!name:-}" ]]; then
    echo "$name is required for the production Web build" >&2
    exit 1
  fi
done

command -v pnpm >/dev/null
command -v rsync >/dev/null
command -v ssh >/dev/null

pnpm --dir "$repo_root/frontend" build

if [[ ! -s "$dist_dir/index.html" ]] || [[ ! -d "$dist_dir/assets" ]]; then
  echo "frontend/dist is missing the production entrypoint or assets" >&2
  exit 1
fi
if [[ -n "$(find "$dist_dir" -type l -print -quit)" ]]; then
  echo "frontend/dist must not contain symbolic links" >&2
  exit 1
fi

release_dir="$release_root/releases/$release_id"
ssh "$deploy_host" sh -s -- "$release_root" "$release_id" <<'REMOTE_PREPARE'
set -eu
release_root=$1
release_id=$2
release_dir="$release_root/releases/$release_id"
umask 022
install -d -m 0755 "$release_root" "$release_root/releases"
test ! -e "$release_dir" && test ! -L "$release_dir"
install -d -m 0755 "$release_dir"
REMOTE_PREPARE

rsync --archive --checksum "$dist_dir/" "$deploy_host:$release_dir/"

ssh "$deploy_host" sh -s -- "$release_root" "$release_id" <<'REMOTE_VERIFY'
set -eu
release_root=$1
release_id=$2
release_dir="$release_root/releases/$release_id"
test -s "$release_dir/index.html"
test -d "$release_dir/assets"
test -n "$(find "$release_dir/assets" -type f -print -quit)"
test -z "$(find "$release_dir" -type l -print -quit)"
find "$release_dir" -type d -exec chmod 0755 {} +
find "$release_dir" -type f -exec chmod 0644 {} +
REMOTE_VERIFY

activate_release

printf 'Web release %s is active at %s:%s/current\n' "$release_id" "$deploy_host" "$release_root"
