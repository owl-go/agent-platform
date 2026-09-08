# Feishu authorization and command approval verification - 2026-09-08

## Reported behavior and causes

The User completed Feishu OAuth, but subsequent Session turns still reported
`authorization_unavailable` and did not show another authorization link. The
active authorization was valid and already contained the required chat search
and message send scopes, so omitting a new OAuth link was correct. Runtime had
used the CLI-facing identity value `me`, while the credential boundary accepts
the canonical `user` and `bot` identities. Commit `4c72693` normalizes `me` to
`user`, rejects other unknown identities before credential resolution, and
instructs the Runtime with literal supported identity values.

After that correction, chat search succeeded but a high-risk send returned
`user_action_unavailable`. The command approval transaction changed the Session
message state to `waiting_for_user` and also wrote the same value to
`progress_stage`. The database intentionally restricts progress stages to the
Runtime activity stages, so its check constraint rejected the update and rolled
back both the Session transition and pending approval record. Commit `c303dfe`
keeps the Session state as `waiting_for_user` while retaining `using_tool` as its
progress stage.

## Regression and deployment validation

`TestSessionCommandApprovalEntersUserActionWait` exercises the real PostgreSQL
repository seam. Before the fix it failed deterministically with SQLSTATE
`23514` from `session_messages_progress_stage_valid`. After the fix it verifies
that the approval becomes listable, the Session enters `waiting_for_user`, and
cancelling the wait restores `generating` with the tool progress stage.

The following checks passed:

- The targeted PostgreSQL regression test with `WORKSPACE_TEST_POSTGRES_DSN`.
- `make test` across all Go packages.
- `make build` across all Go commands.
- The guarded deployment gates: all Go tests and builds, 184 frontend tests
  across 25 files, frontend type checking, and the production Web build.
- Verified business and identity PostgreSQL backups and all deployment health,
  release identity, migration ledger, and error-log checks.

Running every PostgreSQL integration test in the repository also reached the
unrelated existing `TestBeginCLIConnectorAuthorizationReusesPendingAttempt`
fixture, which still inserts the removed
`cli_connector_definitions.owner_user_id` column and fails with SQLSTATE `42703`.
This does not affect the focused command approval regression or the standard
deployment gate, but the full opt-in integration suite is not claimed as
passing.

The active deployment is:

- Source: `/opt/agent-platform/src.release-feishu-approval-c303dfe-20260908`
- Web: `/opt/agent-platform/web/releases/feishu-approval-c303dfe-20260908`
- Backup: `/opt/agent-platform/backups/pre-feishu-approval-c303dfe-20260908`
- API image: `sha256:9bba772b07e04706f5f3fd60156cf085383f9b4d3fc6e555e046e6ece23280cb`
- Worker image: `sha256:66346963a454c3a1033ada2e822c3314e993cceb587bdca3ff71bd113b67e024`
- Egress Controller image:
  `sha256:672ccc057586b698a4ad9c0c3bf0d5819ab09a8a7f2a23db7c0626ed73545b53`

## Production browser verification

An authenticated production Session retried the original request after the
deployment. The Runtime used `--identity user`, found the Feishu group, and
created persisted `im_messages_send` approvals. The API returned the pending
items to the global approval inbox. User decisions reached the decision endpoint
with HTTP 200, each approved command was consumed once, and a duplicate decision
received HTTP 412. The final Session response reported that the requested text
was sent successfully to the requested Feishu group.

No OAuth link appeared during this retry because the existing active User
authorization already covered the required scopes. This is the expected
on-demand behavior; a direct OAuth link appears only when an attempted User
capability lacks its required scopes.

The Runtime requested one approval for a `--help` invocation before requesting
the separate approval for the actual send. Both were correctly treated as
distinct one-use high-risk commands, but the extra help invocation is a remaining
interaction inefficiency rather than part of the authorization or approval
persistence failure fixed here.
