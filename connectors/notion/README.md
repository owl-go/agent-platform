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
