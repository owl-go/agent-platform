# Notion Connector Package source

This directory builds a CLI Connector Package for `ntn@0.23.13`. The upstream npm tarball is pinned by SHA-512. A matching source ZIP contains the official Linux arm64/x64 binaries, license, and `bridge.cjs`. The platform's `cliconnector.ZIPPackageBuilder` builds the exact bundle used in the Connector Package from that source ZIP; the Worker uses the same builder for Linux + `runsc` Conformance. The bridge maps per-command provided credential JSON to `NOTION_API_TOKEN`; it does not store a token. `cli-policy.json` is the reviewed command allowlist.

Build with the **actual target Runtime registry RepoDigest**:

```sh
python3 connectors/notion/build.py \
  --runtime-image 'registry.example/agent-platform/runtime@sha256:<64 lowercase hex digits>' \
  --source-output /absolute/path/notion-cli-source-0.23.13.zip \
  --output /absolute/path/notion-connector-0.23.13.zip
go -C backend run ./cmd/connector-package-validate /absolute/path/notion-connector-0.23.13.zip
```

`--tarball /path/to/ntn-0.23.13.tgz` uses a previously downloaded npm tarball after verifying the same SHA-512. Upload the source ZIP through the Administrator CLI Definition flow and wait for its exact bundle × Runtime Digest Conformance record. The manifest's Runtime Digest is a staging input, not Conformance evidence. Stage and publish the matching Connector Package only after the Worker has recorded a passing result. Actual Notion API use requires an authorized token and Egress to `api.notion.com` (search may also require `developers.notion.com` for API schema lookup).

Reviewed upstream docs: [overview](https://developers.notion.com/cli/get-started/overview), [installation](https://developers.notion.com/cli/get-started/installation), [authentication](https://developers.notion.com/cli/get-started/authentication), [API requests](https://developers.notion.com/cli/guides/api-requests), [data sources](https://developers.notion.com/cli/guides/data-sources), [file uploads](https://developers.notion.com/cli/guides/file-uploads), [command reference](https://developers.notion.com/cli/reference/commands), and [personal access tokens](https://developers.notion.com/guides/get-started/personal-access-tokens).
