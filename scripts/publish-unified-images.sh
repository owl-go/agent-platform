#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -n "$(git status --porcelain)" ]]; then
  echo 'Refusing publication from a dirty checkout.' >&2
  exit 1
fi
git fetch origin main_temp
revision=$(git rev-parse HEAD)
if [[ "$revision" != "$(git rev-parse origin/main_temp)" ]]; then
  echo 'Integrate into main_temp and check out that exact commit before publishing.' >&2
  exit 1
fi
image_repository=${IMAGE_REPOSITORY:-haozhi152/agent-platform}
runtime_image_repository=${RUNTIME_IMAGE_REPOSITORY:-haozhi152/agent-platform-runtime}
release_tag=${RELEASE_TAG:-$(git show -s --format=%cs HEAD)-${revision:0:12}}
if [[ ! "$image_repository" =~ ^[a-z0-9._/-]+$ || ! "$runtime_image_repository" =~ ^[a-z0-9._/-]+$ || ! "$release_tag" =~ ^[a-zA-Z0-9_.-]+$ ]]; then
  echo 'Invalid image repository or release tag.' >&2
  exit 1
fi
for architecture in arm64 amd64; do
  IMAGE="$image_repository" TAG="$release_tag-$architecture" SOURCE_REVISION="$revision" \
    docker buildx bake -f deploy/all-in-one/docker-bake.hcl --progress=plain \
    --set "*.platform=linux/$architecture" --push app
  RUNTIME_IMAGE="$runtime_image_repository" RUNTIME_TAG="$release_tag-$architecture" \
    docker buildx bake -f deploy/all-in-one/docker-bake.hcl --progress=plain \
    --set "*.platform=linux/$architecture" --push runtime
done
# Do not switch either latest alias until both architectures of both images
# have been pushed successfully.
docker buildx imagetools create -t "$image_repository:$release_tag" -t "$image_repository:latest" \
  "$image_repository:$release_tag-arm64" "$image_repository:$release_tag-amd64"
docker buildx imagetools create -t "$runtime_image_repository:$release_tag" -t "$runtime_image_repository:latest" \
  "$runtime_image_repository:$release_tag-arm64" "$runtime_image_repository:$release_tag-amd64"
for architecture in arm64 amd64; do
  docker buildx imagetools create -t "$image_repository:$architecture" "$image_repository:$release_tag-$architecture"
  docker buildx imagetools create -t "$runtime_image_repository:$architecture" "$runtime_image_repository:$release_tag-$architecture"
done
docker buildx imagetools inspect "$image_repository:latest"
docker buildx imagetools inspect "$runtime_image_repository:latest"
