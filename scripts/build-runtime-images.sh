#!/usr/bin/env bash
set -euo pipefail

registry="${RUNTIME_IMAGE_REGISTRY:-agent-platform}"
metadata_directory="${RUNTIME_BUILD_METADATA_DIR:-build/runtime-images}"
mkdir -p "${metadata_directory}"

runtimes=(claude codex hermes openclaw)
versions=(2.1.233 0.147.0 0.19.0 2026.7.1-2)

for index in "${!runtimes[@]}"; do
  runtime="${runtimes[$index]}"
  version="${versions[$index]}"
  tag="${registry}/${runtime}:${version}"
  docker build \
    --pull \
    --tag "${tag}" \
    --file "deploy/runtimes/${runtime}/Dockerfile" \
    deploy/runtimes
  image_id="$(docker image inspect --format '{{.Id}}' "${tag}")"
  printf '%s\n' "${tag} ${image_id}" >"${metadata_directory}/${runtime}.txt"
done

echo "local image IDs written to ${metadata_directory}; production releases must use registry repo digests"
