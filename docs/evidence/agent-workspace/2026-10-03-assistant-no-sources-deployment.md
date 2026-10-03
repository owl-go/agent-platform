# Assistant answers without source annotations — 2026-10-03

## Change and validation

The User requested Assistant answers without document-source marks, then explicitly requested publication. The answer instruction now asks for conclusions, relevant data and explanations without appended source sections, retrieved document names or paths, revision IDs, citation numbers, footnotes or source links. It takes precedence over contradictory citation requirements in custom prompts, response styles or previous answers. Directly relevant business links and substantive answer data remain available.

The permission-checked Knowledge excerpts and source labels remain model context; retrieval authorization, classification, streaming, stored FAQ answers, history and billing were not modified. The shared instruction applies to authenticated and public model-generated answers. Historical answers are not rewritten.

The feature checkout passed the target service tests, `make test`, `make build`, Go formatting and `git diff --check` before committing `6f584e6`. The regression covers both `{knowledge}` and legacy prompts, a contradictory response style, retained Knowledge labels and the no-citation presentation rule. The integration checkout also passed:

- `go -C backend test ./internal/service/workspace/...`.
- `make test` and `make build`.
- `git diff --check`.

The earlier unrelated `main_temp` merge had completed before publication; no conflict resolution was performed by this release. Frontend and Linux/gVisor Runtime gates were not rerun because their implementation did not change. These Go unit/build checks do not constitute new real-storage or Runtime conformance evidence.

## Release

- Feature code: `6f584e6` on `codex/assistant-prompt-variables`, pushed.
- Integrated code: `75ef8fa6f00c487b050838eb2b8b3423b9e629cc` on `main_temp`, pushed before deployment.
- API/source release: `assistant-no-sources-20261003-1`.
- Source: `/opt/agent-platform/src.release-assistant-no-sources-20261003-1`.
- API image: `sha256:124a628c84b3e3b9e74c7ffaf398a521ea5fce34cbfb220b03a2ff4242323434`.
- Verified protected backup: `/opt/agent-platform/backups/pre-assistant-no-sources-20261003-1`. Business and identity dumps passed `pg_restore -l`; configuration, previous release pointers and backup manifest were verified.

Only tracked integrated source was archived and uploaded. The existing service Dockerfile built the API image; a release override activated API only through the production Compose stack with no dependency rebuild. Health and Readiness returned `ok` and `ready` before the source pointer switched. Canonical configuration hashes remained unchanged, preserving the newly enabled message-channel configuration. API, Worker and Egress Controller remained healthy; Caddy remained running.

## Live acceptance

A temporary iframe under `http://localhost:4177` loaded the existing share and asked a customer follow-up question from the User's previous scenario. The real model-backed answer completed with 471 characters, retained the relevant customer and both expected product references, and contained no scope refusal or Knowledge-not-found response. The rendered body contained no document-extension names, source section, document attribution, citation number or footnote under the tested source-marker checks. There were zero error surfaces. The production POST returned 200 in 20,215 ms; API ERROR count was zero and OIDC discovery matched the configured issuer.

Only bounded verification metrics were retained; private answer content, source filenames, visitor handles and Share Tokens are excluded from this evidence. The turn used normal model/retrieval and Credit rules. The temporary page and tab were removed without changing the User's share configuration or original conversation. This verifies the requested presentation on one real Knowledge question, not exhaustive model compliance across all possible wording. Saved FAQ content and historical answers were not rewritten.

## Scope and rollback

Web remains `/opt/agent-platform/web/releases/workflow-channels-20261003-1`. Worker remains `sha256:82550fda89d741c1ab615b0b16af924d6a8c22a72f562d348e81dad2d8d50c5e`, and Egress Controller remains `sha256:ccd474838cc671d6afd20f1dc107ac711adf0f9e3c2666bf607446ce030dbd70`. No migration, frontend, configuration, Runtime image, identity or persistent volume changed.

Rollback uses the protected previous source `/opt/agent-platform/src.release-workflow-channels-20261003-1` and API image `sha256:889698d27ca74595674a303885e7ff05c07b228da5302fcc048c130456ef8e78` through the production Compose stack. Verify health before switching the source pointer; retain the current Worker, Web, canonical configuration and volumes.
