#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <pinned-upstream-git-checkout> <output-directory>" >&2
  exit 2
fi

upstream=$1
output=$2
commit=$(git -C "$upstream" rev-parse 'v1.0.62^{commit}')
test "$commit" = 70323e1486e64b1ca823fa2bd9c48b9b07f88519 || {
  echo "DingTalk v1.0.62 tag moved" >&2
  exit 1
}

build_root=$(mktemp -d)
trap 'rm -rf "$build_root"' EXIT
git -C "$upstream" archive "$commit" | tar -x -C "$build_root"
python3 - "$build_root/internal/app/flags.go" <<'PY'
import pathlib, sys
path = pathlib.Path(sys.argv[1])
source = path.read_text()
old_import = 'import (\n'
old_flag = 'StringVar(&flags.Token, "token", "", "Override the configured API token")'
if source.count(old_import) != 1 or source.count(old_flag) != 1:
    raise SystemExit("pinned DingTalk source no longer matches the reviewed token bridge")
source = source.replace(old_import, 'import (\n\t"os"\n', 1)
source = source.replace(old_flag, 'StringVar(&flags.Token, "token", os.Getenv("AGENT_PLATFORM_DWS_ACCESS_TOKEN"), "Override the configured API token")', 1)
path.write_text(source)
PY

mkdir -p "$output"
for architecture in amd64 arm64; do
  (
    cd "$build_root"
    CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" go build -trimpath \
      -ldflags='-s -w -X github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app.version=v1.0.62-agent-platform.1' \
      -o "$output/dws-linux-$architecture" ./cmd
  )
done
shasum -a 256 "$output/dws-linux-amd64" "$output/dws-linux-arm64"
