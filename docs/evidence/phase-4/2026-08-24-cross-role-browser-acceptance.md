# Phase 4 Cross-role Browser Acceptance — 2026-08-24

## Scope

- Revision under test: `8a573fb` plus the Ticket 14 acceptance hardening recorded with this evidence.
- Browser: Playwright CLI against the production Vite application, Go API, PostgreSQL, MinIO, and a pinned Keycloak OIDC test issuer.
- Product roles: Platform Administrator, Agent Builder, Agent User, and Run Operator.
- Locales and layout: `en-US`, `zh-CN`, desktop, and a 390 x 844 mobile viewport.

## Result

`scripts/acceptance/oidc-browser.sh` completed successfully without skipped browser scenarios. It verified:

- Runtime, model, Credential Profile, Source Control Provider, and Repository Binding setup through real APIs.
- Agent/Draft validation, Release Approval separation, immutable Release publication, deprecation, and emergency block.
- Coding Task launch, stable Session continuation, multiple Runs, Run Approval, SSE replay, Artifact download, Session Memory, approved Agent Memory, and explicit Task completion/cancellation.
- Run search, frozen diagnostics, Interrupt, Cancel, Kill confirmation, infrastructure-only recovery, Audit filtering, and authorization revocation.
- Cross-Team and cross-Organization denial, validation errors, optimistic-lock conflicts, idempotent replay, SSE reconnect, empty results, rate limiting, offline presentation, OIDC expiry, keyboard navigation, mobile layout, and localized document language.
- Exact planted model, Git, known-hosts, and build Secret values were absent from browser storage, URL, page text, console output, SSE, API/web logs, Audit output, and downloaded Artifact content.

## Commands

```bash
PWCLI=/Users/frank/.codex/skills/playwright/scripts/playwright_cli.sh \
  scripts/acceptance/oidc-browser.sh

cd frontend
pnpm test
pnpm exec vue-tsc --noEmit
pnpm build
```

The browser suite also runs its required PostgreSQL integration tests before opening the browser. Generated contract consistency, the full Go test suite, and Go builds are separate repository gates and must remain green for the ticket commit.

## Evidence Boundary

This result proves the browser-to-control-plane business closure. The suite uses controlled Runtime Event, Artifact, and Git-delivery fixtures for deterministic browser assertions; it does not prove a real model invocation or a real Git SSH push by all four Runtime images.

Phase 0 therefore remains `NO-GO`. Production readiness still requires the independent Linux + gVisor, pinned Runtime image, real model, Git SSH Push, MinIO, and Aliyun OSS evidence defined by the Phase 0 Production Conformance specification.
