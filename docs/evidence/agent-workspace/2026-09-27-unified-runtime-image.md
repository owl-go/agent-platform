# Unified Runtime image candidate - 2026-09-27

## Built candidate

The five fixed Runtime Engine CLIs were built into one Linux image from
`deploy/runtimes/unified/Dockerfile` on the production Worker host and pushed to
the private registry.

RepoDigest:

`127.0.0.1:5000/agent-platform/runtime@sha256:b674584d47184ac55cefce639b2ba0cae98e8fda00155dff07019883e005c48d`

The candidate was built from `main_temp` commit
`b267b5abc49eefe6ff5556c10770343095417f39` using release identifier
`platform-20260927T084332Z-unified-runtime`.

## Executed validation

- `make test` and `make build` passed.
- The frontend suite passed 288 tests in 40 files.
- `make web-typecheck` and `make web-build` passed.
- The production Linux host ran Docker 29.7.2 with `runsc` configured using
  `--host-uds=open`.
- The shared image reported Claude Code `2.1.233`, Codex `0.147.0`, Hermes
  `0.19.0`, OpenClaw `2026.7.1-2`, and PI Agent `0.84.4`.
- The standard Runtime image smoke passed against the immutable RepoDigest,
  including the fixed CLI Builder image.
- A second smoke under `runsc` verified all five versions, UID 65532, read-only
  Rootfs, writable Workspace, the common Entrypoint, and the current
  `agent-cli -- <argv>` broker protocol failing closed without a broker socket.

## Activation boundary

The candidate was pushed but not written to the production `RUNTIME_IMAGE`
setting, and API, Worker, Egress Controller, Caddy, and Web were not replaced.
The existing five Runtime images remain active.

The target host does not currently provide the Production Conformance
environment and five Runtime-specific model credential directories required by
`scripts/conformance/production-preflight.sh`. Consequently no real model call,
cancel, timeout, MCP loading, Secret redaction, Workspace write, or exact
Runtime RepoDigest plus CLI bundle Conformance was executed for this candidate.
The shared Digest must not make any Runtime Engine available until that
engine's required evidence passes. This candidate is therefore build-and-smoke
evidence, not production activation or complete Production Conformance.
