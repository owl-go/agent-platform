# Knowledge Base interactive search: verification record

This record covers commit `990af45` on `codex/knowledge-search`. It is local implementation evidence, not a production deployment or real AnythingLLM conformance result.

## Automated checks completed

| Check | Result | Scope |
| --- | --- | --- |
| `make test` | Passed | Go unit and available integration tests; database-dependent tests without a DSN are skipped. |
| `make build` | Passed | Go build. |
| `make web-typecheck` | Passed | Vue/TypeScript types. |
| `make web-build` | Passed | Production frontend build. |
| `pnpm --dir frontend exec vitest run` | Passed: 39 files, 258 tests | Includes two Knowledge Base page-search tests for sourced results, unready index, and no matches. |
| `WORKSPACE_TEST_POSTGRES_DSN=<temporary PostgreSQL DSN> go -C backend test ./internal/data/workspace/gormrepo -run '^TestKnowledgeSearchUsesAuthorizedCurrentSources$' -count=1 -v` | Passed | Real temporary PostgreSQL database with the full migration chain; verifies current generation, owner/public access, and stale/failed/deleted source rejection. |
| `go -C backend test ./internal/service/workspace` | Passed | Includes ten-result cap, source extraction, and storage-error fail-closed tests. |

The disposable PostgreSQL container used for the targeted integration test was stopped after testing. No production database or provider was modified.

## Real-provider acceptance still required

1. In the intended environment, record the configured AnythingLLM image digest and embedding model; confirm the API and Worker reach the same endpoint.
2. Upload a supported document containing a unique, non-sensitive test phrase to a private Knowledge Base. Confirm the document reaches `ready` and the Base has a ready Knowledge Index Generation.
3. Search that phrase in the Base detail page. Verify a relevant excerpt appears with the correct document name and optional Category, and no more than ten excerpts are returned. Repeat with a query known to have no match and confirm an indexed no-hit state rather than an unready or error state.
4. Verify another User cannot search the private Base; then check that an authenticated User can search a public Administrator Base without being able to mutate it.
5. Replace or delete the document and repeat the search. No superseded or deleted revision should appear as a valid citation even if the provider still returns its old vector. Check provider outage separately: the UI must show an error, not a no-hit result.

Until this round trip and the remaining production conformance gates pass, the feature is implemented and locally tested but **not verified in production**.

The current AnythingLLM client also cannot query a historical provider index by Knowledge Index Generation; it queries the Base's mutable provider workspace. This does not block the detail page's current-index preview, but frozen-generation reproducibility for Workflow Runs is still unverified and must not be inferred from these checks.
