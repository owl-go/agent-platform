# Embedded public Assistant visitor continuity — 2026-10-03

## Diagnosis

The User's localhost preview loaded the Assistant and answered a greeting, then showed a failed answer for the next question. Production request metadata showed the greeting POST returning 200, followed by two continuation POSTs returning 404. A fresh HTTPS public metadata response set the visitor Cookie with HttpOnly and `SameSite=Lax`, without Secure or Partitioned. Share Tokens, Cookie values, request bodies, private questions and Knowledge content were not included in the diagnostic output.

HTTPS terminates at Caddy, which forwards HTTP to the API. The previous policy checked only `request.TLS`, so it downgraded the Cookie behind the edge. Without an accepted cross-site Cookie, a continuation acquires a new visitor hash and correctly fails the existing visitor-bound conversation lookup. This was a visitor continuity failure, rather than a Knowledge retrieval or model failure.

A handler-level regression simulated this HTTPS edge plus embedded-browser Cookie acceptance. Before the fix, its greeting succeeded and continuation failed with `status=404 secure=false sameSite=2 partitioned=false`. After the fix, the same conversation accepted the second turn and returned its exact enabled FAQ without model calls.

## Implementation and tests

The public visitor helper now uses direct TLS or a valid HTTPS origin from the existing platform authentication redirect configuration to select Secure Cookie policy. It does not trust client-supplied Host, Forwarded or X-Forwarded-Proto to establish HTTPS. HTTPS visitor Cookies use HttpOnly, Secure, SameSite=None and Partitioned, remain host-only with path `/` and the existing one-day lifetime, and preserve their opaque visitor value on continuation. HTTP development retains Lax without Secure or Partitioned.

Partitioned storage separates a Cookie by the embedding top-level site, following the [official CHIPS design](https://privacysandbox.google.com/cookies/chips). Cookies remain inaccessible to client JavaScript. Assistant, visitor and Share Token revision checks, origin restrictions, safety, rate limits, owner Credits and publication requirements remain enforced. Missing or different visitor Cookies still cannot resume another conversation.

Both feature and integration checkouts passed:

- `gofmt` and `git diff --check` for the changed Go files.
- `go -C backend test ./internal/service/workspace/...`.
- `make test` and `make build`.

Tests cover the exact two-turn proxy failure, direct HTTPS, HTTP development, missing/invalid/credential-containing configured origins, forged forwarding headers, Cookie validity and visitor hash continuity. Existing another-visitor and stale-Token-revision rejection tests remained passing. There were no frontend changes, so frontend gates were not rerun. PostgreSQL/real storage integration and Linux/gVisor Runtime gates were not run for this HTTP Cookie fix; passing unit/build gates do not imply those acceptance results.

## Deployment and production verification

- Feature code: `eb09c8a` on `codex/assistant-prompt-variables`, pushed.
- Integrated code: `615035d16235ec2486fa681fcf996878925f126a` on `main_temp`, pushed before deployment.
- API/source release: `assistant-visitor-cookie-20261003-1`.
- Source: `/opt/agent-platform/src.release-assistant-visitor-cookie-20261003-1`.
- API image: `sha256:971c21956555118bba5e4cc53057fd6d8cbae2df79dd1a00e998ac16f8ddba1a`.
- Verified protected backup: `/opt/agent-platform/backups/pre-assistant-visitor-cookie-20261003-1`. Business and identity dumps passed `pg_restore -l`; configuration, prior release pointers and the backup manifest were verified.

Only tracked integrated source was archived and uploaded. The API image was built with the existing service Dockerfile. The production Compose stack activated only API with no dependency rebuild; the immutable source pointer switched after health passed. Canonical environment and YAML hashes stayed unchanged. The full platform script was not invoked because it rebuilds unrelated execution images and performs unrelated retired-volume cleanup.

A real browser loaded the current User-provided share in a temporary iframe under `http://localhost:4177`. In one fresh visitor conversation, `你好呀` and then `您好` each received the configured welcome response. The DOM contained two completed answer bodies and zero error surfaces; the production requests returned 200 in 68 ms and 28 ms. These were the platform's no-model greeting path, with no Knowledge query or owner Credit consumption. A screenshot was inspected. No User-owned share configuration or existing visitor conversation was modified. The temporary fixture/tab were removed and the User's original preview/code retained.

A fresh HTTPS metadata response confirmed HttpOnly=true, Secure=true, SameSite=None and Partitioned=true. Public Health and Readiness returned `ok` and `ready`; OIDC discovery matched the configured issuer. API startup ERROR count was zero. API, Worker and Egress Controller remained healthy; Caddy remained running.

## Scope and rollback

Web remains `assistant-scroll-20261003-1`; Worker remains `sha256:864b3058c31e8dfe5b438017a78187f91084ad53f9c796ab303965a41916bea6`. Runtime/CLI Builder, identity, Egress, retrieval configuration, canonical config and persistent volumes were retained. No migration was added. Real model-backed answering and private Knowledge retrieval were not exercised by this production acceptance check. Existing in-memory conversations created with the missing/old visitor identity require one fresh conversation after the fix.

Rollback uses the protected previous source/image pointers: `/opt/agent-platform/src.release-assistant-embed-types-20261003-1` and API image `sha256:73f33c016e1c633851feba773f8cf7f207aac9b66809586c194d36a96646b90e`, through the production Compose stack, then restores the source pointer after health passes. Retain the current Web, configuration and volumes.
