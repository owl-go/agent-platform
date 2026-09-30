---
name: notion
display_name: Notion
description: Read and manage Notion pages or query data sources through the reviewed ntn CLI capabilities.
version: 0.23.13
author: Agent Workspace
---

# Notion Connector

Use this Skill for a User's Notion pages, title search, and data source queries. It documents only the commands allowed by this Connector Revision. The `cli.json` capability policy, selected Authorization, Runtime Conformance, Egress, and one-use approval remain the execution gates.

## Authorization

Install the Connector Package and connect a Notion personal access token as provided credentials JSON: `{"token":"<personal-access-token>"}`. The User creates the token in Notion and gives it the **Notion API** capability. Leave the platform `scopes` list empty: this PAT capability is a Notion token property, not a platform OAuth scope. Store the token only through the platform's Authorization flow; never include it in a prompt, command argument, file, or package. Use one Authorization per User and workspace. The platform's generic `connector_package` driver injects `CONNECTOR_CREDENTIALS_JSON`; the package bridge passes its token to `ntn` for one command process. Disconnect through the platform. This revision does not use `ntn login` or claim Notion OAuth.

Notion PATs act with the creator's page permissions. Guests and restricted workspace members cannot create PATs. This revision does not expose Workers, unrestricted `ntn api`, file uploads, or data source schema changes.

## Read workflow

1. For a known page, use `ntn pages get <page-id> --json` when structure or truncation information matters; otherwise `ntn pages get <page-id>` returns Markdown.
2. To search titles, use `ntn api v1/search query=<title>`. Only `query`, `page_size`, and `start_cursor` inputs are allowed on this route. Search is limited to content accessible to the token.
3. For a database with multiple data sources, use `ntn datasources resolve <database-id> --json`, then `ntn datasources query <data-source-id> --json`. Use `--limit` and `--start-cursor` for pagination. The query accepts an optional JSON `--filter` according to Notion's data source schema.
4. `ntn api v1/users/me` checks the token identity. It does not list all workspace users.

## Write workflow

1. Read the target and identify the intended parent or page ID before writing.
2. Create with `ntn pages create --parent page:<id> --content <markdown> --json` (or a `data-source:<id>` parent). Use the platform's one-use approval for this high-risk capability.
3. Edit with `ntn pages edit <page-id> --content <markdown> --json` after reviewing the current Markdown. Editing can replace page content, so present the full intended change for approval. The bridge rejects `--allow-deleting-content`.
4. Trash with `ntn pages trash <page-id> --yes` only when the User explicitly requested deletion, and obtain the platform's one-use approval.

Pass arguments as a structured argv array. Do not use a shell command string. Supply page content through the required `--content` argument. A CLI `--help` check proves only that the binary starts. Report missing Authorization, Conformance, Egress, or Notion permissions distinctly.

Official command syntax: [Notion CLI command reference](https://developers.notion.com/cli/reference/commands) and [authentication](https://developers.notion.com/cli/get-started/authentication).
