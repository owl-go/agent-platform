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
