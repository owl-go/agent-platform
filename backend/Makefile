.PHONY: breaking build test generate verify-generated wire

BUF := go run github.com/bufbuild/buf/cmd/buf@v1.72.0
BUF_BREAKING_REF ?= $(shell git -C .. show-ref --verify --quiet refs/remotes/origin/main && echo refs/remotes/origin/main || echo main)
BUF_BREAKING_AGAINST ?= ../.git\#ref=$(BUF_BREAKING_REF),subdir=backend
GENERATED_PATHS := api gen/openapi cmd/api/wire_gen.go cmd/worker/wire_gen.go ../frontend/src/api/generated.d.ts

breaking:
	@if git -C .. cat-file -e '$(BUF_BREAKING_REF):backend/buf.yaml' 2>/dev/null; then \
		$(BUF) breaking --against '$(BUF_BREAKING_AGAINST)'; \
	else \
		echo "main has no Protobuf baseline yet; buf lint establishes the initial contract"; \
		$(BUF) lint; \
	fi

build:
	go build ./...

test:
	go test ./...

generate:
	$(BUF) lint
	$(BUF) generate
	$(MAKE) wire
	cd ../frontend && pnpm exec swagger2openapi --patch --outfile ../backend/gen/openapi/agent-platform.openapi.json ../backend/gen/openapi/agent-platform.swagger.json
	cd ../frontend && node ../scripts/normalize-openapi.mjs ../backend/gen/openapi/agent-platform.openapi.json
	cd ../frontend && pnpm exec openapi-typescript ../backend/gen/openapi/agent-platform.openapi.json --output src/api/generated.d.ts

verify-generated: generate
	git diff --exit-code -- $(GENERATED_PATHS)
	@test -z "$$(git ls-files --others --exclude-standard -- $(GENERATED_PATHS))" || { \
		echo "untracked generated files:"; \
		git ls-files --others --exclude-standard -- $(GENERATED_PATHS); \
		exit 1; \
	}

wire:
	go run github.com/google/wire/cmd/wire@v0.6.0 ./cmd/api ./cmd/worker
