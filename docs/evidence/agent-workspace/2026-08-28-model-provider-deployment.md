# Model Provider Deployment Verification — 2026-08-28

## Scope

- Host: `47.237.108.63`
- Public origin: `https://47-237-108-63.sslip.io`
- Product revision: `68a4741` (`79b3cf3` Agent Workspace release plus the maintained-catalog panic fix)
- Release directory: `/opt/agent-platform/src.release-68a4741`
- Pre-deployment backup: `/opt/agent-platform/backups/20260828-074618-pre-79b3cf3`

The backup contains PostgreSQL business and Keycloak custom-format dumps, the deployment configuration archive, the previous release target, and SHA-256 checksums. No Secret value is recorded in this evidence.

## Deployed images

- API: `sha256:c19aa2c61bff190ab173dc2de30bee691013b7277347eba869d2dddeb3bfeb02`
- Worker: `sha256:63e844a060d04f54921918fcf23f1c2005b7afd049ef2b4f07c22720fc5d8350`
- Web: `sha256:a293008481e8d2e16e6754ce5bd37fbbf680bc0cc1de0c84fd058d21ca162d13`

## Migration

`000005_model_provider_connections.sql` applied successfully with checksum:

```text
66c46b73ab6b881e4b64f5d46da05bfdb55bfe4227be340647a470ddf90eae0f
```

The previously applied `000001`–`000004` checksums matched the release before cutover.

## Verification

- API `/healthz`: `200`, `{"status":"ok"}`
- API `/readyz`: `200`, `{"status":"ready"}`
- Keycloak discovery issuer matches the public same-origin HTTPS URL.
- HTTP redirects to HTTPS with `308`; HTTPS returns HSTS and `nosniff` headers.
- The served Web bundle contains the new `model-provider-connections` client route.
- API, Worker, Web, PostgreSQL, MinIO, Identity DB, Keycloak, and Caddy are running; services with health checks report healthy.
- Authenticated administrator projection succeeded.
- Model Provider presets include OpenAI, Anthropic, and Alibaba Bailian.
- A temporary Alibaba Bailian connection imported the maintained catalog, remained explicitly `unverified`, and returned only write-only API Key state.
- A temporary Session message froze the selected Provider Model and Codex Runtime in its Response Snapshot.
- The Worker claimed the message and reached the expected failed terminal state with a random invalid canary key.
- The canary key was absent from API and Worker logs.
- Temporary Session and Model Provider Connection rows were removed, and Keycloak Direct Access Grants were restored to disabled.

During the first authenticated check, static catalog success exposed a nil-error panic in the service verification branch. The request left no product data, the regression was fixed in `68a4741`, full Go tests passed, and the check then completed successfully. API logs after the fixed container started contain no recovered panic.

## Evidence boundary

This verification proves deployment, migration, authenticated Model Provider management, Session Response Snapshot persistence, Worker claiming, explicit Runtime failure, cleanup, and Secret-log absence. It does not claim real-provider model success or refresh behavior, and it does not replace four-Runtime production Conformance for the deployed image digests.
