# Knowledge Base detail layout validation — 2026-10-02

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

The Knowledge Base detail header now groups the return action, name, scope, description, and management actions into a compact header. Opening a Base hides the Resource Library catalog search until the User returns. Retrieval has one input row without an additional padded card. Category navigation, upload, URL import, and document actions retain the existing permission and ingestion contracts.

## Checks executed

- Feature branch targeted Vitest: `pnpm --dir frontend test src/pages/KnowledgeBasesPage.test.ts src/pages/ResourceCenterPage.test.ts` — 8 tests passed, including category/unclassified filtering, regeneration/retry menu commands, and catalog/detail navigation.
- Integrated `main_temp`: `make web-typecheck`, `pnpm --dir frontend test` (493 tests across 46 files), and `make web-build` — passed.
- `git diff --check` — passed. The integration conflict retained both Connector catalog fixtures and the new Knowledge Base detail fixtures.
- Playwright Chromium visual checks used temporary, isolated frontend fixtures at 1440×900, 1024×768, and 390×844, with Chinese and English labels. Desktop/tablet header height was approximately 55px; document rows measured 45px. The mobile page width remained 390px with table scrolling confined to its region. Opening a document menu and resizing to mobile also preserved the page width after disabling Popper tethering.
- Preview fixtures and the temporary Vite process were removed. Local screenshots remain under ignored `output/playwright/knowledge-layout-*.png`; no sample records were added to production.

## Release verified

Feature commit: `51bfed8`. Integrated and pushed `main_temp` commit: `fa11684`. The frontend was built with the existing production OIDC configuration and published using `scripts/deploy-web.sh` from the `main_temp` worktree.

- Active web release: `/srv/agent-workspace/web/releases/knowledge-layout-20261002-1`.
- Previous web release remains available: `ragflow-knowledge-20261002-4`.
- Public `/resources?tab=knowledge`, the main JS/CSS, and Resource Center JS/CSS all returned HTTP 200 and matched local production build bytes.
- Public entrypoint SHA-256: `b3d83549d4528abef3820defe95598a299da4e6a6ecdb1aadfd1f347339661da`.
- Public `/api/healthz` returned HTTP 200.

This is frontend layout validation, with isolated visual fixtures and public deployment asset verification. An authenticated production browser session was not rerun. Backend services, RAGFlow configuration, credentials, and document data were not changed; RAGFlow ingestion/retrieval, Go gates, Runtime image smoke, and Linux/gVisor Production Conformance were not rerun for this frontend-only change.

## Follow-up: retrieval dialog

The User requested a single retrieval button that opens a dialog. Feature commit `96a1960` moves the query and all retrieval feedback/results into that dialog; `main_temp` commit `038856d` integrates it. The detail header keeps the button available to readers while edit/delete remain restricted to maintainers. Closing and reopening preserves the current query; changing the Knowledge Base resets it.

- Targeted Vitest (8 tests) passed, covering dialog queries/citations, unready/no-match/failure responses, reopening, and reader access. `main_temp` full frontend Vitest, `make web-typecheck`, `make web-build`, and `git diff --check` passed.
- Isolated Playwright Chromium fixtures verified the desktop and 390×844 mobile dialog, input autofocus, Enter submission, ten-result scrolling, and Escape dismissal with focus returned to the retrieval button. Mobile document width remained 390px; the result list scrolled inside the dialog. Temporary fixtures/processes were removed, and screenshots are retained only under ignored `output/playwright/knowledge-search-dialog-*.png`.
- Published from `main_temp` with the existing production OIDC configuration using `scripts/deploy-web.sh`. Active web release is now `/srv/agent-workspace/web/releases/knowledge-layout-20261002-2`; release `knowledge-layout-20261002-1` remains available.
- The public resource route, main JS/CSS, and Resource Center JS/CSS returned HTTP 200 and matched all five local production files. Entrypoint SHA-256: `454ce25082f1db566caa377f8b9f3b19a6c3e1f8b8e28f133893400638367bfc`. Public `/api/healthz` returned HTTP 200.

The same frontend-only validation limits above apply to this follow-up.

## Follow-up: remove the Resource Library catalog search

The User requested removal of the generic task search input. Feature commit `fdc6187`, integrated as `main_temp` commit `5467452`, removes that input, its spacing, its query binding, and the now-unused Knowledge Base detail notification. Old `q` parameters no longer silently filter resource listings. Product and UI specifications were updated to reflect the requested behavior.

Targeted Vitest (8 tests), integrated full frontend Vitest (493 tests across 46 files), `make web-typecheck`, `make web-build`, and `git diff --check` passed. Integration retained the existing resource loading/error tests. Published from `main_temp` with the existing production OIDC configuration via `scripts/deploy-web.sh`; active web release is `/srv/agent-workspace/web/releases/resource-catalog-20261002-1`. The public resource route and four main/resource JS/CSS files matched local production bytes, the compiled resource route no longer contains the removed toolbar, and `/api/healthz` returned HTTP 200. Entrypoint SHA-256: `ecb8af97bda4e14a96df7f2abab76772f566eb789d1060292df1357fbca38b0d`.

This follow-up used component regression tests and public asset verification, without repeating authenticated production browser, RAGFlow, backend, or Runtime conformance checks.

## Follow-up: simplify document upload controls

The User requested removal of the Category creation, upload Category selector, and webpage import controls. Feature commit `2185854`, integrated as `main_temp` commit `c70e7d4`, removes these controls and places the maintainer-only upload button beside the document heading. Uploads use the existing unclassified default; existing Category navigation remains available.

Targeted Vitest (9 tests), integrated full frontend Vitest (494 tests across 46 files), `make web-typecheck`, `make web-build`, and `git diff --check` passed. The upload regression test submits a file without a Category and verifies that the accepted document appears; the reader test verifies that the upload button is unavailable. Published from `main_temp` with the existing production OIDC configuration via `scripts/deploy-web.sh`; active web release is `/srv/agent-workspace/web/releases/knowledge-controls-20261002-1`. The public resource route and four main/resource JS/CSS files returned HTTP 200 and matched local production bytes. Public `/api/healthz` returned HTTP 200. Entrypoint SHA-256: `6d9f38624c9f210a7d93d98489c9b65fb63f4ca9b87d018a4dcfe65c79e9fb0b`.

This follow-up used component regression tests and public asset verification, without repeating authenticated production browser, RAGFlow, backend, or Runtime conformance checks.
