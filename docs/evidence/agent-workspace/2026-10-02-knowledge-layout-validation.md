# Knowledge Base detail layout validation — 2026-10-02

The Knowledge Base detail header now groups the return action, name, scope, description, and management actions into a compact header. Opening a Base hides the Resource Library catalog search until the User returns. Retrieval has one input row without an additional padded card. Category navigation, upload, URL import, and document actions retain the existing permission and ingestion contracts.

## Checks executed

- Feature branch targeted Vitest: `pnpm --dir frontend test src/pages/KnowledgeBasesPage.test.ts src/pages/ResourceCenterPage.test.ts` — 8 tests passed, including category/unclassified filtering, regeneration/retry menu commands, and catalog/detail navigation.
- Integrated `main_temp`: `make web-typecheck`, `pnpm --dir frontend test` (493 tests across 46 files), and `make web-build` — passed.
- `git diff --check` — passed. The integration conflict retained both Connector catalog fixtures and the new Knowledge Base detail fixtures.
- Playwright Chromium visual checks used temporary, isolated frontend fixtures at 1440×900, 1024×768, and 390×844, with Chinese and English labels. Desktop/tablet header height was approximately 55px; document rows measured 45px. The mobile page width remained 390px with table scrolling confined to its region. Opening a document menu and resizing to mobile also preserved the page width after disabling Popper tethering.
- Preview fixtures and the temporary Vite process were removed. Local screenshots remain under ignored `output/playwright/knowledge-layout-*.png`; no sample records were added to production.

## Release verified

Feature commit: `51bfed8`. Integrated and pushed `main_temp` commit: `fa11684`. The frontend was built with the existing production OIDC configuration and published using `scripts/deploy-web.sh` from the `main_temp` worktree.

- Active web release: `/opt/agent-platform/web/releases/knowledge-layout-20261002-1`.
- Previous web release remains available: `ragflow-knowledge-20261002-4`.
- Public `/resources?tab=knowledge`, the main JS/CSS, and Resource Center JS/CSS all returned HTTP 200 and matched local production build bytes.
- Public entrypoint SHA-256: `b3d83549d4528abef3820defe95598a299da4e6a6ecdb1aadfd1f347339661da`.
- Public `/api/healthz` returned HTTP 200.

This is frontend layout validation, with isolated visual fixtures and public deployment asset verification. An authenticated production browser session was not rerun. Backend services, RAGFlow configuration, credentials, and document data were not changed; RAGFlow ingestion/retrieval, Go gates, Runtime image smoke, and Linux/gVisor Production Conformance were not rerun for this frontend-only change.
