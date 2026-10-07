.PHONY: build test deploy web-build web-typecheck web-deploy oidc-browser-acceptance runtime-images runtime-image-smoke sandbox-conformance minio-conformance production-conformance-preflight production-conformance

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
	python3 -m unittest discover -s scripts -p 'deployment_test.py'
	python3 -m unittest discover -s scripts -p 'installation_test.py'
	bash -n scripts/deploy-platform.sh scripts/deploy-platform-infrastructure.sh scripts/install-platform.sh

resources-check:
	go -C backend run ./cmd/resourcecheck --root ..

.PHONY: install deploy-setup deploy-check deploy-test resources-check

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
