# Connector catalog layout — local validation

Date: 2026-10-01 (Asia/Shanghai).

Implemented category groups, Market/Installed views, compact icon/name/description cards, installation plus/check markers, and centered responsive Connector details with bottom connection/disconnection/uninstall actions. Guidance continues to populate an unsent Session draft.

Validation performed:

- `pnpm --dir frontend test`: 46 files, 413 tests passed.
- `make web-typecheck`: passed.
- `make web-build`: passed; existing large-chunk warning remains.
- `git diff --check`: passed.
- Playwright CLI with the actual Vue components and controlled local API fixtures: desktop 1280×960, mobile 390×844, two-column/single-column cards, centered modal, visible bottom actions, scrollable body, and Escape close.
- On mobile, page width was 390px, modal bounds were x=12px and width=366px, with no horizontal overflow.

Regression coverage includes Notion/Teambition browser authorization, DingTalk device authorization, WeCom/Modao credentials, Feishu setup and scope recovery, account disconnect retaining installation, versioned uninstall, exact installed-revision guidance, unsent Session routing, and returning to Market from the old personal Connector URL.

These local checks preceded deployment; the later release is recorded in [the deployment evidence](2026-10-01-connector-catalog-layout-deployment.md). Browser checks used local fixtures and are not evidence of real external-account authorization or production Runtime Conformance. No backend or Runtime behavior was changed.
