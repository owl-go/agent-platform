# Model Provider Deployment Verification — 2026-08-28

## Scope

- Host: `47.237.108.63`
- Public origin: `https://47-237-108-63.sslip.io`
- Product revision: `2403d5d` (`79b3cf3` Agent Workspace release plus model-provider and Toast feedback fixes)
- Release directory: `/opt/agent-platform/src.release-2403d5d`
- Pre-deployment backup: `/opt/agent-platform/backups/20260828-074618-pre-79b3cf3`

The backup contains PostgreSQL business and Keycloak custom-format dumps, the deployment configuration archive, the previous release target, and SHA-256 checksums. No Secret value is recorded in this evidence.

## Deployed images

- API: `sha256:be139d6f9f62bd70f2e72812425603375f07f0fe071ddbf9e510a7944ad09794`
- Worker: `sha256:01af34799bcb0afe7c56f20dc16027a89aeabe11b1d6f7d7e157cc34610656cb`
- Web: `sha256:1fcdf92f4d8ef1d3a8d7bbc76c2c68a50e3fdf56cda7bec5e0912425579cd2f2`

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
- An authenticated temporary custom connection using an absolute HTTP Endpoint returned `200`, preserved write-only API Key state, and was deleted after verification.
- A temporary Session message froze the selected Provider Model and Codex Runtime in its Response Snapshot.
- The Worker claimed the message and reached the expected failed terminal state with a random invalid canary key.
- The canary key was absent from API and Worker logs.
- Temporary Session and Model Provider Connection rows were removed, and Keycloak Direct Access Grants were restored to disabled.

During the first authenticated check, static catalog success exposed a nil-error panic in the service verification branch. The request left no product data, the regression was fixed in `68a4741`, full Go tests passed, and the check then completed successfully. API logs after the fixed container started contain no recovered panic.

The `e8da914` follow-up allows trusted HTTP model gateways and makes provider-save success, failure, and in-progress states visible in the Web editor. Its domain and component regression tests passed before deployment, and recent API/Worker logs contained no panic or error-level entry.

The `2403d5d` Web-only follow-up standardizes transient success and failure feedback on one Toast component across Sessions, Workflows, Workflow detail, Experts, Settings, and User administration. The component provides success/error semantics, accessible live regions, manual dismissal, timed dismissal, responsive placement above modal layers, and reduced-motion behavior. All 44 frontend tests, TypeScript checking, and the production build passed before deployment. After the Web-only cutover, API, Worker, and Web remained healthy and the served asset was `assets/index-D3NPOGKa.js`.

## Evidence boundary

This verification proves deployment, migration, authenticated Model Provider management, Session Response Snapshot persistence, Worker claiming, explicit Runtime failure, cleanup, and Secret-log absence. It does not claim real-provider model success or refresh behavior, and it does not replace four-Runtime production Conformance for the deployed image digests.
