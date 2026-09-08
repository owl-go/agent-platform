# Feishu Connector Runtime recovery deployment verification - 2026-09-09

## Release

- Public origin: `https://47-237-108-63.sslip.io`
- Source commit: `fea491d43e169c12e7edd9a14eefe1fb2ceba5ce` (`codex/fix-feishu-cli-install`)
- Active release: `connector-auth-final-fea491d-20260909`
- Source: `/opt/agent-platform/src.release-connector-auth-final-fea491d-20260909`
- Web: `/opt/agent-platform/web/releases/connector-auth-final-fea491d-20260909`
- Backup: `/opt/agent-platform/backups/pre-connector-auth-final-fea491d-20260909`

The guarded `scripts/deploy-platform.sh` deployment completed with every gate enabled. The active container image IDs are:

- API: `sha256:527ff8de6ef74d6ac7636cf42d08580af65d5c290dc893f4d699b3f3101f63e5`
- Worker: `sha256:0ad655d772bfd6abbcb3fcf84b2bac60a4348e6c113cdaec1233a2ff4fd1907c`
- Egress Controller: `sha256:d1578cc6e3f276189fbb584404a35e67d2c326e16b7bbfc02b19aefb7f4f5c8b`

Public Health returned HTTP `200` with `{"status":"ok"}` after cutover. The release script verified the API, Worker, Egress Controller, Web release, OIDC endpoints, HTTPS routing, migration ledger, backups, and post-replacement logs.

## Recovered failures

The production reproduction exercised the original Session and resolved five independent failures without adding Feishu capability checks to the Runtime or conversation UI:

1. Commit `01b4c35` gives a long host-side CLI Broker socket a private short Unix-socket alias and mounts it at the fixed Runtime path `/run/agent-cli/cli-broker.sock`. This removes the Linux `AF_UNIX` `bind: invalid argument` failure for long Workspace paths.
2. Commit `639718b` verifies a retained conversation snapshot against the current Connector Definition ID, frozen Bundle SHA-256, and current Runtime RepoDigest. A stale snapshot no longer causes a false `CLI Connector is not verified` rejection after Runtime image replacement.
3. Commit `6188b7e` adds migration `000034_session_message_waiting_for_user.sql`, allowing the persisted `waiting_for_user` progress stage required by conversation Connector actions.
4. Commit `cd1b6fe` permits Runtime progress events while a Session message or Run is waiting, while preserving the visible `waiting_for_user` stage. A later `command.requested` event no longer cancels the pending action through an event-delivery conflict.
5. Commit `8cd4964` clears the generated primary key before reading an upserted authorization attempt and reuses an unexpired attempt when it already covers the requested Permissions. First click, repeated click, and popup retry therefore use a stable generic Connector authorization flow instead of returning `500` or issuing another provider challenge.

## Runtime and Connector evidence

All five fixed Runtime images passed image smoke tests. Linux + `runsc` cold and warm integration tests also passed with the original long Workspace path shape.

- Claude: `sha256:f87fd965b5cbc5e6f02df91407b78a3edf0c0b594409cbdd57f521ce6f53d07f`
- Codex: `sha256:939f4ae2a98bb1578443b11523bf0d8c1cf3a6714bbf5548c99b21bc781f93db`
- Hermes: `sha256:21e8ddaf3c4b5f55fe6d394947175c5b256748d22865cd91dbb436492b1823f1`
- OpenClaw: `sha256:d58349b09ff43840d3b81199a5113e6affafd3c0f66c4c47e57d9a0e5b1fc4ed`
- PI: `sha256:9c9245dd9e7d06576e88a0eace0e7eef9f84a258ab1af9939d00e0e40442e985`

The production `飞书 CLI` Definition `37b92007-4645-4e50-9682-54adf369ab23` is `available` at version `50`, Manifest version `1`, with Bundle SHA-256 `a1b99b398fe4134a5d3f38828941548304e184c9778d52b33cbdb6e5b1a6276a`. Production contains passing exact-Bundle Conformance rows for all five current Runtime RepoDigests. Five older passing rows remain as historical evidence and are not used for current execution.

## Verification

The final source passed:

- `make test`
- `make build`
- 171 Web tests across 24 files
- `make web-typecheck`
- `make web-build`
- `git diff --check`

On the production Linux host, the focused PostgreSQL integration tests `TestConnectorActionPausesAndResumesSessionMessage` and `TestBeginCLIConnectorAuthorizationUpdatesExistingAttempt` ran against disposable databases and passed. The latter executes the create-then-upsert path that reproduced the repeated-click failure.

A transient Keycloak impersonation session verified the exact ordinary User `wuyuewei` through `/api/v1/me`, then retried the existing Feishu request. The resulting assistant message entered `waiting_for_user`, with an `authorization_required` action for `chat.search`. The first action start returned HTTP `200`; its one-time platform URL returned HTTP `303` to `accounts.feishu.cn`. A second start of the same pending action returned HTTP `200` in 8 ms and its renewed one-time platform URL again returned HTTP `303` to `accounts.feishu.cn`, demonstrating reuse of the existing provider attempt. The first start took 248 ms because it created that attempt.

The Connector audit for this validation contains only `operation.requested` and `operation.waiting` with reason `authorization_required`. The provider consent was not completed, no protected Connector command ran, and no Feishu message was sent.

## Evidence boundary

This verification proves the production long-socket recovery, retained-snapshot Conformance lookup, persisted conversation wait state, progress-event compatibility, authorization-link creation, one-time redirect, and repeated-click behavior. It does not claim completed Feishu OAuth consent or successful message delivery because those actions require the owning User's interactive provider confirmation and would send an external message.
