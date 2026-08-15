# Agent Platform

Internal platform for configuring, running, and operating governed Coding Agents.

## Workspace

- `apps/web` - Vue and TypeScript product surfaces
- `cmd/api` - Go HTTP control plane
- `cmd/worker` - Go Run worker
- `internal/agentruntime` - Runtime Adapter seam
- `internal/sandbox` - Docker + gVisor Sandbox lifecycle seam
- `internal/objectstore` - Aliyun OSS and MinIO provider seam
- `deploy/runtimes` - isolated Claude, Codex, Hermes, and OpenClaw images
- `docs/product` - confirmed product requirements and MVP scope
- `docs/technical` - implementation specifications
- `docs/tickets/phase-0` - executable Phase 0 work

## Local checks

```bash
pnpm install
make test
make build
make web-typecheck
make web-build
```

gVisor validation requires a Linux Worker with `runsc`; it cannot run on the macOS development host.
