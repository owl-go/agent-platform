#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_root}"

./scripts/generate-openapi.sh
go -C backend test ./...
go -C backend build ./...
go -C backend vet ./...
./scripts/verify-slo-gates.sh
pnpm --dir frontend typecheck
pnpm --dir frontend build

while IFS= read -r -d '' script; do
    bash -n "${script}"
done < <(find scripts deploy -type f -name '*.sh' -print0)

git diff --check
