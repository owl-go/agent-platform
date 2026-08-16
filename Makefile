.PHONY: build test web-build web-typecheck runtime-images runtime-image-smoke sandbox-conformance minio-conformance production-conformance-preflight production-conformance

build:
	cd backend && go build ./...

test:
	cd backend && go test ./...

web-build:
	pnpm --dir frontend build

web-typecheck:
	pnpm --dir frontend typecheck

.PHONY: breaking generate verify-generated

breaking:
	cd backend && $(MAKE) breaking

generate:
	cd backend && $(MAKE) generate

verify-generated:
	cd backend && $(MAKE) verify-generated

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
