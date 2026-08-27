# Agent Workspace

Personal AI workspace for private Sessions, reusable Workflows, Experts, Extensions, and managed Runtime execution.

## Workspace

- `frontend` - standalone Vue and TypeScript product surfaces
- `backend` - standalone Go Kratos control plane and Worker module
- `backend/cmd/api` - Wire-built Kratos HTTP control plane
- `backend/cmd/worker` - Wire-built Kratos Worker App
- `backend/internal/biz` - bounded-context models, use cases, and workflow ports
- `backend/internal/data` - GORM and external-system adapters
- `backend/internal/service` - generated API adapters and authenticated Run Event SSE
- `backend/internal/server` - Kratos HTTP and Worker lifecycle servers
- `backend/api` - authoritative Protobuf contracts and generated Go transports
- `deploy/runtimes` - isolated Claude, Codex, Hermes, and OpenClaw images
- `docs/product` - Agent Workspace product requirements
- `docs/technical` - implementation specifications
- `docs/adr` - active architectural decisions

## Local checks

```bash
cd frontend && pnpm install
cd ..
make generate
make test
make build
make web-typecheck
make web-build
```

gVisor validation requires a Linux Worker with `runsc`; it cannot run on the macOS development host.
