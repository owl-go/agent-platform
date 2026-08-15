#!/usr/bin/env bash
set -euo pipefail

image="${MINIO_TEST_IMAGE:-minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e}"
container="agent-platform-minio-conformance-$$"
access_key="agentconformance"
secret_key="agentconformance-secret"

cleanup() {
  docker rm --force "${container}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run --detach --rm \
  --name "${container}" \
  --publish 127.0.0.1::9000 \
  --env "MINIO_ROOT_USER=${access_key}" \
  --env "MINIO_ROOT_PASSWORD=${secret_key}" \
  "${image}" server /data >/dev/null

port="$(docker inspect --format '{{(index (index .NetworkSettings.Ports "9000/tcp") 0).HostPort}}' "${container}")"
for _ in {1..60}; do
  if curl --fail --silent "http://127.0.0.1:${port}/minio/health/ready" >/dev/null; then
    break
  fi
  sleep 0.25
done
curl --fail --silent "http://127.0.0.1:${port}/minio/health/ready" >/dev/null

MINIO_ENDPOINT="127.0.0.1:${port}" \
MINIO_ACCESS_KEY="${access_key}" \
MINIO_SECRET_KEY="${secret_key}" \
MINIO_BUCKET="agent-platform-conformance" \
MINIO_CREATE_BUCKET=true \
go test -count=1 -v ./internal/objectstore/minio
