.PHONY: build test web-build web-typecheck sandbox-conformance

build:
	go build ./...

test:
	go test ./...

web-build:
	pnpm web:build

web-typecheck:
	pnpm web:typecheck

sandbox-conformance:
	scripts/conformance/sandbox-linux.sh
