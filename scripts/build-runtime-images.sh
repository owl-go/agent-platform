#!/usr/bin/env bash
set -euo pipefail

registry="${RUNTIME_IMAGE_REGISTRY:-agent-platform}"
metadata_directory="${RUNTIME_BUILD_METADATA_DIR:-build/runtime-images}"
mkdir -p "${metadata_directory}"

runtime_tag="${registry}/runtime:${RUNTIME_IMAGE_TAG_OVERRIDE:-1.0.0}"
docker build \
  --pull \
  --tag "${runtime_tag}" \
  --file deploy/runtimes/unified/Dockerfile \
  .
runtime_image_id="$(docker image inspect --format '{{.Id}}' "${runtime_tag}")"
printf '%s\n' "${runtime_tag} ${runtime_image_id}" >"${metadata_directory}/runtime.txt"

builder_tag="${registry}/cli-builder:1.0.0"
docker build \
  --pull \
  --tag "${builder_tag}" \
  --file deploy/runtimes/cli-builder/Dockerfile \
  .
builder_image_id="$(docker image inspect --format '{{.Id}}' "${builder_tag}")"
printf '%s\n' "${builder_tag} ${builder_image_id}" >"${metadata_directory}/cli-builder.txt"

echo "local image IDs written to ${metadata_directory}; production releases must use registry repo digests"
