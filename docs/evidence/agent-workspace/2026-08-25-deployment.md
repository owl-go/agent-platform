# Agent Workspace deployment evidence - 2026-08-25

Status: deployed; API acceptance, real-browser identity acceptance, real Codex Session continuity, and real Workflow execution are complete.

## Target and rollback

- Worker: `47.237.108.63` (Linux)
- Public origin: `https://47-237-108-63.sslip.io`
- Active source: `/opt/agent-platform/src.release-agent-workspace-20260825`
- Rollback source: `/opt/agent-platform/src.release-1c261de`
- Pre-reset backup: `/opt/agent-platform/backups/20260825-023445`

The backup contains `business.pgdump`, `identity.pgdump`, and `config.tar.gz`. `sha256sum -c SHA256SUMS` passed for all three files before the reset. The legacy business database was rebuilt from the Agent Workspace schema; the additive `000002_run_queued_at.sql` migration repaired the initial zero-time Run rows and now rejects invalid queued timestamps.

## Deployed services

- API, Worker, Web, PostgreSQL, and MinIO report healthy; Caddy, Keycloak, and identity PostgreSQL are running.
- `/api/healthz`, `/api/readyz`, and OIDC discovery return HTTP 200 through the public TLS origin. An unauthenticated `/api/v1/me` returns HTTP 401.
- MinIO bucket creation is a one-shot `minio-init` dependency of API and Worker, so a clean deployment cannot accept execution before its private Artifact bucket exists.
- The public OIDC client uses Authorization Code with PKCE. The deployment acceptance script enables the password grant only for its isolated API fixture and restores it to disabled in its cleanup trap.
- The Linux public-egress service is enabled. Runtime containers use `runsc`, immutable image digests, a read-only root filesystem, non-root execution, capability removal, and `no-new-privileges`.

## Runtime images

The four Runtime engines use separate immutable images:

- Claude Code: `127.0.0.1:5000/agent-platform/claude@sha256:521a8bc2bd08345a82dfade7c554d5cbc64ed85acabd21d3ac5db4393e749748`
- Codex: `127.0.0.1:5000/agent-platform/codex@sha256:573d2a75415bdca6f79a8fc7fd1dd14c14f240ed51c2eb4eaccc418acffb6a77`
- Hermes: `127.0.0.1:5000/agent-platform/hermes@sha256:376de53a1564c4332b12e101f19c527e38db6499d193ace2e9d0598b29f6cd5b`
- OpenClaw: `127.0.0.1:5000/agent-platform/openclaw@sha256:7074894081ec42a9c3e083ea0ee671982b05779d6dc1eea0f0b51974a16612b8`

The exact Codex image runs CLI `0.147.0`. Its configured Model Profile uses model `gpt-5.6-sol` and the TLS relay `https://47-237-108-63.sslip.io/model-relay/openai`; the original plaintext model endpoint is not reachable from the Runtime network. The Model Secret remains encrypted in product storage and enters the Runtime only through the single-execution credential environment.

## Real browser evidence

The in-app browser completed OIDC login as `platform-admin` and exercised the deployed Vue application against the real API.

### Account first login

The Administrator opened Account Management, created a temporary ordinary User, and received a one-time password. The ordinary User signed in at the real Keycloak page, was forced to update the password, and then reached their private empty Sessions page. The temporary Keycloak identity and business User projection were deleted after verification; no password was retained in evidence.

### Session continuity and native Resume

- Session: `96dc718c-d36d-488c-8176-3d1e85a482ca`
- First message asked the model to remember project code `白鹭-731`; the response was `已记住`.
- The next message asked only for the project code; the response was `白鹭-731`.
- Both turns used the same Codex native Session checkpoint `01a03698-ca9e-7352-870c-17a09bfdceb4`.
- Persistent Runtime state contains only the exact-secret-redacted `sessions/` tree. Auth, config, logs, plugin cache, and temporary MCP configuration are excluded.

### Workflow, Workspace, Artifacts, and API trigger

- Workflow: `49dd2d9d-03de-42fc-8067-198a7e4cc4b5` (`真实 Codex 工作流`)
- Manual Run `9ce8bb19-...` completed successfully through Codex and produced `verification.txt` with exact content `workflow-real-success`; follow-up Run `fad17780-...` verified the corrected Workspace merge permissions while preserving that content.
- The persistent Workspace lists and previews the 22-byte file. Its merged ownership is the configured platform UID/GID with mode `0600`; API preview succeeds.
- Run `9ce8bb19-...` produced the immutable file Artifact with SHA-256 prefix `5495ec4f76eb`, and the Artifact preview shows the same content. Inline final-result Artifacts render correctly when protobuf omits the empty SHA-256/default size fields.
- The Workflow API credential was generated in the browser and revealed once. An HTTP Basic request returned HTTP 202 with Run `397fc780-8a5a-4617-9b95-6689be1982b8`; the Run History shows trigger `API 调用`, terminal state `成功`, and 14-second elapsed time.
- Run History is ordered by the repaired `queued_at` timestamp and shows manual/API trigger, state, and bounded duration without `NaN`.

## Deployment acceptance

`scripts/acceptance/agent-workspace-deployment.sh` completed with exit code 0 against the public TLS origin. Its isolated fixtures were cleaned automatically. Evidence includes:

- Administrator API and ordinary User creation, disable, enable, password reset, and disabled-token rejection
- non-enumerating owner isolation for Sessions, messages, Workflow Runs, and Artifacts
- Session rename, archive, cancel archive, explicit failure, retry, and delete
- Personal Settings, Model Profile, and optional Expert creation
- Workflow creation, hourly scheduled trigger, HTTP Basic API execution, idempotent write headers, cancellation, terminal SSE, rerun, deletion, and retained read-only history
- Workspace directory creation, binary upload/download, public HTTPS Git clone, and clear
- deterministic Runtime failure and replayed `run.started`, `run.failed`, and `run.cancelled` events
- Secret canary absence from API/Worker logs and persisted Run Event payloads

The intentional failure fixture uses an HTTPS Endpoint containing a query, which the generic Model Profile accepts but the Codex Driver rejects immediately under its stricter executable configuration contract. It therefore proves explicit incompatibility without relying on DNS timeout behavior.

## Repository gates

The following commands passed after the final changes:

```text
make test
make build
make web-typecheck
make web-build
git diff --check
bash -n scripts/acceptance/agent-workspace-deployment.sh
```

## Remaining external evidence

- Private SSH Clone is implemented with a write-only per-Workflow key and strict `known_hosts`, but no private repository/deploy key was supplied for a live clone.
- Aliyun OSS remains implemented behind the shared Object Storage provider contract, but this deployment uses MinIO and no Aliyun OSS credentials were supplied.
- Claude Code, Hermes, and OpenClaw images pass the version/sandbox smoke boundary but were not exercised with a real compatible model endpoint in this deployment. Their native Resume capability remains disabled.
