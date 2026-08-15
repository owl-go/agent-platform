.PHONY: build test web-build web-typecheck runtime-images runtime-image-smoke sandbox-conformance minio-conformance production-conformance-preflight production-conformance

build:
	go build ./...

test:
	go test ./...

web-build:
	pnpm web:build

web-typecheck:
	pnpm web:typecheck

runtime-images:
	scripts/build-runtime-images.sh

runtime-image-smoke:
	scripts/conformance/runtime-image-smoke.sh

sandbox-conformance:
	scripts/conformance/sandbox-linux.sh

minio-conformance:
	scripts/conformance/minio-local.sh

production-conformance-preflight:
	scripts/conformance/production-preflight.sh

production-conformance:
	scripts/conformance/production.sh
