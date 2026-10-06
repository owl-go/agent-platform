# Unified Runtime image deployment - 2026-09-27

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Built candidate

The five fixed Runtime Engine CLIs were built into one Linux image from
`deploy/runtimes/unified/Dockerfile` on the production Worker host and pushed to
the private registry.

The pre-activation build-and-smoke candidate used RepoDigest:

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

## Production activation

Release `platform-20260927T090000Z-unified-runtime` rebuilt the same image
contents with a new BuildKit attestation manifest and activated this final
RepoDigest:

`127.0.0.1:5000/agent-platform/runtime@sha256:b4ad4abd0356ae01129f2e75233c41b4c7833bab44dbd9895e9b3ed4b2a3df37`

The active source is
`/srv/agent-workspace/src.release-platform-20260927T090000Z-unified-runtime`.
API, Worker, Egress Controller, Caddy, the then-configured external retrieval service, the public Web origin, and
OIDC discovery passed post-cutover health checks. The Administrator's original
default Runtime Engine and model mappings were restored after validation.

The first cutover exposed that the external production YAML still referenced
the five legacy Runtime image variables while the new Compose file passed only
`RUNTIME_IMAGE`. The deployment stopped with Worker intentionally down and the
backup plus previous source intact. The YAML was migrated atomically, API was
recovered, and the remaining cutover completed. The deployment script now
migrates exactly zero or five legacy references and rejects partial drift.

## Real model calls

Five isolated Administrator Sessions exercised the normal OIDC, settings,
Session, Worker, credential materialization, `runsc`, and Runtime paths against
the active shared Digest:

- Claude Code with Qwen Plus completed in 12.756 seconds and returned
  `OK-CLAUDE`.
- Codex with gpt-6-astra completed in 21.229 seconds and returned `OK-CODEX`.
- Hermes with GPT 5.6 Sol completed in 30.498 seconds and returned `OK-HERMES`.
- OpenClaw ran for 337.558 seconds and returned `LLM request failed` without
  producing the requested content.
- PI Agent failed in 11.632 seconds with `Unexpected end of JSON input`.

Claude, Codex, and Hermes remained available. OpenClaw and PI were immediately
set unavailable in the production YAML and API/Worker were recreated healthy;
the public Runtime catalog confirmed those two entries omitted availability
while the PI failure was investigated.

## PI Agent stream recovery

The PI `0.84.4` failure was reproduced deterministically with a loopback SSE
endpoint that ended an OpenAI-compatible event before its JSON payload was
complete. The first patch covered PI's installed `openai` dependency and was
released as `platform-20260927T093600Z-pi-sse` with RepoDigest:

`127.0.0.1:5000/agent-platform/runtime@sha256:b9e5c90aa7ae756893e3cf4da0e36b032eafa8d824ffe4e3000406ff84110521`

Two production calls passed, but the third reproduced `Unexpected end of JSON
input`. PI was disabled again immediately. Inspection of the actual CLI import
path then showed that the published PI executable also embeds a separate copy
of the OpenAI stream parser in
`dist/bundle/chunks/chunk-NUHFSC37.js`; dependency-only smoke coverage had not
exercised this executable path.

The final patch covers both the installed dependency files and the embedded PI
bundle. Empty SSE metadata frames are ignored. A `SyntaxError` whose exact
message is `Unexpected end of JSON input` is normalized to PI's existing
bounded stream-retry path; unrelated JSON errors remain failures. The image
build also fails if the fixed-version source no longer contains the expected
patch sites.

The Runtime image smoke now verifies both parser copies and starts the real PI
CLI against a loopback endpoint. Its first stream is deliberately truncated;
the second succeeds, and the smoke requires PI to emit its retry event and the
expected final output.

Release `platform-20260927T101000Z-pi-stream-retry`, built from `main_temp`
commit `7c768a6`, activated the corrected immutable RepoDigest:

`127.0.0.1:5000/agent-platform/runtime@sha256:7c3cbcd7d6277e4fa806b305ada2d616b41f581bac18532e561859177b5349a7`

The production Digest-level smoke passed. Six consecutive real PI calls in one
Administrator Session then returned `PI-LIVE-1` through `PI-LIVE-6` in 9, 9,
8, 8, 7, and 8 seconds respectively, crossing the earlier third-call failure
point. PI is now available in the production Runtime catalog. The
Administrator's default Runtime Engine was restored to Codex after validation,
and the validation Session was archived. OpenClaw remains unavailable.

This activation does not claim the complete `make production-conformance`
matrix. The target host still lacks its dedicated Conformance environment and
credential directories, and cancellation, timeout, MCP loading, Secret
redaction, Workspace write, Connector bundle combinations, and recovery cases
remain unverified for the shared Digest. Availability therefore remains
fail-closed for OpenClaw and for any Capability not backed by the required
Digest-specific evidence.
