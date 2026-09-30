# Notion Connector Package source

This directory builds a local CLI Connector Package for `ntn@0.23.13`. The upstream npm tarball is pinned by SHA-512 and the resulting bundle retains only the official Linux arm64/x64 binaries and license. The bundled `bridge.cjs` maps the platform's per-command provided credential JSON to `NOTION_API_TOKEN`; it does not store a token. `cli-policy.json` is the reviewed command allowlist.

Build with the **actual target Runtime registry RepoDigest**:

```sh
python3 connectors/notion/build.py \
  --runtime-image 'registry.example/agent-platform/runtime@sha256:<64 lowercase hex digits>' \
  --output /absolute/path/notion-connector-0.23.13.zip
go -C backend run ./cmd/connector-package-validate /absolute/path/notion-connector-0.23.13.zip
```

`--tarball /path/to/ntn-0.23.13.tgz` uses a previously downloaded npm tarball after verifying the same SHA-512. The manifest's Runtime Digest is a staging input, not Conformance evidence. Platform CLI availability still requires Linux + `runsc` Conformance of the exact bundle SHA-256 against that exact RepoDigest, and actual Notion API use requires an authorized token and Egress to `api.notion.com` (search may also require `developers.notion.com` for API schema lookup).

Reviewed upstream docs: [overview](https://developers.notion.com/cli/get-started/overview), [installation](https://developers.notion.com/cli/get-started/installation), [authentication](https://developers.notion.com/cli/get-started/authentication), [API requests](https://developers.notion.com/cli/guides/api-requests), [data sources](https://developers.notion.com/cli/guides/data-sources), [file uploads](https://developers.notion.com/cli/guides/file-uploads), [command reference](https://developers.notion.com/cli/reference/commands), and [personal access tokens](https://developers.notion.com/guides/get-started/personal-access-tokens).
