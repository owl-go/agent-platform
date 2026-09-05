#!/usr/bin/env bash
set -euo pipefail

registry="${RUNTIME_IMAGE_REGISTRY:-agent-platform}"
metadata_directory="${RUNTIME_BUILD_METADATA_DIR:-build/runtime-images}"
mkdir -p "${metadata_directory}"

runtimes=(claude codex hermes openclaw pi)
versions=(2.1.233 0.147.0 0.19.0 2026.7.1-2 0.84.4)

for index in "${!runtimes[@]}"; do
  runtime="${runtimes[$index]}"
  version="${versions[$index]}"
  tag="${registry}/${runtime}:${version}"
  docker build \
    --pull \
    --tag "${tag}" \
    --file "deploy/runtimes/${runtime}/Dockerfile" \
    .
  image_id="$(docker image inspect --format '{{.Id}}' "${tag}")"
  printf '%s\n' "${tag} ${image_id}" >"${metadata_directory}/${runtime}.txt"
done

builder_tag="${registry}/cli-builder:1.0.0"
docker build \
  --pull \
  --tag "${builder_tag}" \
  --file deploy/runtimes/cli-builder/Dockerfile \
  .
builder_image_id="$(docker image inspect --format '{{.Id}}' "${builder_tag}")"
printf '%s\n' "${builder_tag} ${builder_image_id}" >"${metadata_directory}/cli-builder.txt"

echo "local image IDs written to ${metadata_directory}; production releases must use registry repo digests"
