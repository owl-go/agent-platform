.PHONY: build test deploy web-build web-typecheck web-deploy oidc-browser-acceptance runtime-images runtime-image-smoke sandbox-conformance minio-conformance production-conformance-preflight production-conformance local-up local-down local-status local-credentials local-test

build: resources-check
	cd backend && go build ./...

test: resources-check
	cd backend && go test ./...

deploy:
	scripts/deploy-platform.sh

install:
	scripts/install-platform.sh

deploy-setup:
	scripts/deploy-platform.sh --configure

deploy-check:
	scripts/deploy-platform.sh --check

deploy-test:
	python3 -m unittest discover -s deploy/all-in-one -p '*_test.py'
	python3 -m unittest discover -s scripts -p 'unified_publish_test.py'
	python3 -m unittest discover -s scripts -p 'deployment_test.py'
	python3 -m unittest discover -s scripts -p 'installation_test.py'
	node --test scripts/local-platform.test.mjs
	bash -n scripts/deploy-platform.sh scripts/deploy-platform-infrastructure.sh scripts/install-platform.sh

resources-check:
	go -C backend run ./cmd/resourcecheck --root ..

.PHONY: install deploy-setup deploy-check deploy-test resources-check
.PHONY: image-build image-publish

image-build:
	docker buildx bake -f deploy/all-in-one/docker-bake.hcl --load app runtime

image-publish:
	bash scripts/publish-unified-images.sh

web-build:
	pnpm --dir frontend build

web-typecheck:
	pnpm --dir frontend typecheck

web-deploy:
	scripts/deploy-web.sh

oidc-browser-acceptance:
	scripts/acceptance/oidc-browser.sh

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

local-up:
	node scripts/local-platform.mjs start

local-down:
	node scripts/local-platform.mjs stop

local-status:
	node scripts/local-platform.mjs status

local-credentials:
	node scripts/local-platform.mjs credentials

local-test:
	node --test scripts/local-platform.test.mjs
