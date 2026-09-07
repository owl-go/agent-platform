# Feishu account authorization Web fix - 2026-09-07

## Failure and correction

The authenticated Connectors page reproduced an HTTP 400 response from
`POST /api/v1/connectors/cli/enablements/{id}/authorizations`. The browser payload
contained `{"identity":"user","scopes":[null]}` and the response was
`{"error":"invalid_request_body"}`. Empty capability Scope lists omitted by
Protobuf JSON became `undefined` in the frontend and then `null` in request JSON.

Source commit `cbffe60` treats omitted capability collections, identities, and
Scope lists as empty during User Scope collection. Existing User scopes remain
deduplicated and Bot-only scopes are excluded. Authorization validation failures
now use specific Chinese and English guidance instead of package-format advice.

## Validation

- The new component-to-HTTP regression test failed before the fix with the same
  `scopes: [null]` payload observed in the browser, then passed after the fix.
- `pnpm --dir frontend install --frozen-lockfile` completed.
- Targeted ExtensionManager tests passed; the final full `pnpm --dir frontend test`
  run passed 168 tests across 24 files, including both localized error messages.
- `make web-typecheck`, `make web-build`, and `git diff --check` passed.

## Deployment and browser verification

- Public origin: `https://47-237-108-63.sslip.io`.
- `scripts/deploy-web.sh` rebuilt with the existing public OIDC configuration and
  activated release `feishu-authorization-cbffe60-20260907`.
- Previous Web release `platform-20260907-main-469da9b` remains available for rollback.
- Public HTML serves `/assets/index-Do6lx2ZK.js`; the remote `current` symlink
  resolves to the new release.
- After reloading the original browser page and retrying account authorization,
  the POST returned HTTP 200 at `2026-09-07T10:08:43Z` (request ID
  `6c5a9eac-187c-487f-b0f4-0548e1a3ac4f`). The page displayed the Feishu authorization
  link and waiting-for-user message without the previous error.

This verifies authorization initiation. The user must complete the external
account authorization; token issuance and Connector commands were not verified.
This Web-only change did not run backend, Runtime, Linux sandbox, or remote
storage conformance gates. No backend service, database, or Feishu application
credential was changed by the deployment.

## Automatic opening follow-up

Source commit `6d50e2f` opens a tab during the Enable click, navigates it to Feishu
registration, and starts account authorization automatically when registration
completes. A retained application enters account authorization directly. Manual
authorization also opens its URL without requiring a second platform click.
Continuation links remain available if the browser blocks or detaches the tab.

The full frontend suite passed 178 tests across 24 files. After correcting a test
fixture type, the targeted component suite passed all 43 tests and both
`make web-typecheck` and `make web-build` passed. Coverage includes tab creation
before asynchronous requests, tab reuse, retained applications, blocked and closed
tabs, continuation links, slow polling, duplicate clicks, failure cleanup, and
Connectors without authentication.

`scripts/deploy-web.sh` activated `feishu-auto-authorization-6d50e2f-20260907` with
the existing public OIDC configuration. Public HTML serves
`/assets/index-DFpGgliw.js`. The preceding `feishu-authorization-cbffe60-20260907`
release remains available. An independent authenticated browser page loaded the
Connector catalog and preserved the existing enabled Connector and active account
authorization. Opening that catalog did not initiate another authorization.

The automatic first-registration sequence is covered by component tests; it was
not replayed with a new production account. Existing account authorization was
not disconnected for testing. No external consent or Connector command was
performed during this follow-up verification.
