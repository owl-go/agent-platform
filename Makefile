.PHONY: build test web-build web-typecheck

build:
	go build ./...

test:
	go test ./...

web-build:
	pnpm web:build

web-typecheck:
	pnpm web:typecheck

