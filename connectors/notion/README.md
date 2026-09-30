# Notion Connector Package source

This directory builds Connector Package revision `0.23.15` for `ntn@0.23.13`. The upstream npm tarball is pinned by SHA-512. A matching source ZIP contains the official Linux arm64/x64 binaries, license, and `bridge.cjs`. The platform's `cliconnector.ZIPPackageBuilder` builds the exact bundle used in the Connector Package from that source ZIP; the Worker uses the same builder for Linux + `runsc` Conformance. Platform authorization runs the pinned CLI's no-browser login and stores the resulting token privately. The bridge maps that per-command credential JSON to `NOTION_API_TOKEN`; it does not store a token. `cli-policy.json` is the reviewed command allowlist. Revision `0.23.15` declares a 128-process sandbox budget: the earlier 16-process limit prevented Linux + `runsc` startup. The source ZIP carries that same resource profile into Builder Conformance. The bridge pins Notion API version `2026-03-11` so API requests do not depend on a live documentation download to select their version.

Build with the **actual target Runtime registry RepoDigest**:

```sh
python3 connectors/notion/build.py \
  --runtime-image 'registry.example/agent-platform/runtime@sha256:<64 lowercase hex digits>' \
  --source-output /absolute/path/notion-cli-source-0.23.13.zip \
  --output /absolute/path/notion-connector-0.23.15.zip
go -C backend run ./cmd/connector-package-validate /absolute/path/notion-connector-0.23.15.zip
```

`--tarball /path/to/ntn-0.23.13.tgz` uses a previously downloaded npm tarball after verifying the same SHA-512. Upload the source ZIP through the Administrator CLI Definition flow and wait for its exact bundle × Runtime Digest Conformance record. The manifest's Runtime Digest is a staging input, not Conformance evidence. Stage and publish the matching Connector Package only after the Worker has recorded a passing result. Actual Notion API use requires a completed browser login and Egress to `api.notion.com` (search may also require `developers.notion.com` for API schema lookup).

Reviewed upstream docs: [overview](https://developers.notion.com/cli/get-started/overview), [installation](https://developers.notion.com/cli/get-started/installation), [authentication](https://developers.notion.com/cli/get-started/authentication), [API requests](https://developers.notion.com/cli/guides/api-requests), [data sources](https://developers.notion.com/cli/guides/data-sources), [file uploads](https://developers.notion.com/cli/guides/file-uploads), [command reference](https://developers.notion.com/cli/reference/commands), and [personal access tokens](https://developers.notion.com/guides/get-started/personal-access-tokens).

## Production verification — 2026-09-30

Release `platform-20260930T094115Z` was deployed from `main_temp` integration commit `662f529`. Published revision `d9f2154a-8e16-450f-bcc9-61c1ebb0ad78` (`0.23.15`) has normalized package SHA-256 `660a91d5166392c7a20a6d25406fd4131bc9a5fe26ebbf9e2fe4bb289e1fe5f0` and bundle SHA-256 `5b916409e510134fe4309b855e55bc30e629aa0bb5b9471a2cf66ee0f50b634a`. Its passed Linux + `runsc` startup Conformance uses Runtime `127.0.0.1:5000/agent-platform/runtime@sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b` and the declared 128-process budget. A controlled 16-process probe reproduced the startup failure; the same probe with 128 processes reached the API.

The existing Installation was upgraded while preserving its selected active Authorization. Live calls through the normal broker, credential, policy, and sandbox boundaries succeeded for `identity` (`/v1/users/me`, ID and name present) and `search-by-title` (`query=测试 page_size:=5`, zero matches). The identity response had no top-level `person` field; this evidence does not establish the human account's email. Catalog verification found one Notion entry with icon `notion`; the temporary staging Definition was soft-deleted after checking it had no User bindings, retaining its Conformance evidence.

Verification passed: targeted Go tests, the PostgreSQL-backed managed Runtime Conformance lookup test, `make test`, `make build`, `make web-typecheck`, `make web-build`, all 390 frontend tests, package validation, and deployment gates. The task panel regression verifies that a completed Run with failed calls displays “已完成，有调用失败”. Notion write operations, a model-driven Session rerun, and the full `make production-conformance` suite were not executed for this repair.
