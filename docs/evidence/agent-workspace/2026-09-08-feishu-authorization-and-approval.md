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

## Conversation placement follow-up

Commit `fa6ec50` moves the approval for the currently open Session into the
Conversation Composer, immediately above the editor in the same recovery area
used by Feishu authorization. The page-level inbox continues to show approvals
for other Sessions and Runs, so embedding the current item does not hide
background User Action Waits or render one approval twice. Entering the embedded
wait also requests an immediate inbox refresh instead of waiting for the empty
inbox's next 30-second poll.

The component regression first failed because the current approval remained in
the page-level component. It now verifies that the matching Session approval is
teleported into the composer target, a background Run remains global, and the
placement refreshes immediately. The Sessions page test verifies registration
only for its active `waiting_for_user` message and cleanup on unmount. All 186
frontend tests, `make web-typecheck`, `make web-build`, and `git diff --check`
passed.

Web release `feishu-approval-composer-fa6ec50-20260908` is active at
`/opt/agent-platform/web/releases/feishu-approval-composer-fa6ec50-20260908`.
Both the remote release and public origin serve `assets/index-x2RZBYxv.js`. An
authenticated production browser loaded the new release. A safety-only request
explicitly asked to inspect help without sending; the Runtime omitted the
required high-risk target, so Broker rejected it before creating an approval and
the response confirmed that no message was sent. Production placement therefore
relies on the focused DOM regression rather than claiming another live send-card
capture.

## Session execution summary follow-up

Commits `71f36dd` and `9a722b9` replace the flat Session activity timeline with
two disclosure levels. Opening `View execution progress` first shows concise
summary rows. Runtime preparation and matching command requested/completed pairs
are grouped, and known Feishu capabilities are described as chat search,
instruction lookup, or message send operations. Opening one summary row then
reveals the persisted redacted command records. The summary wording says that a
Connector was invoked rather than claiming the external operation succeeded,
because `command.completed` records process completion and does not carry its
exit status into the Session activity projection.

The redundant `Expand details` text was removed from each summary row; the native
disclosure arrow remains. A single execution stage whose `final_text` exactly
matches the final Agent message is also omitted, so the same response is not
rendered twice. Different single-stage results, failures, and multi-stage Expert
Team results remain available.

All 189 frontend tests, `make web-typecheck`, `make web-build`, and
`git diff --check` passed. Web release
`feishu-activity-summary-9a722b9-20260908` is active at
`/opt/agent-platform/web/releases/feishu-activity-summary-9a722b9-20260908`.
Both the remote release and public origin serve `assets/index-CypoTZhI.js`.

An authenticated production browser reloaded the existing successful Feishu
Session without invoking a new Runtime or external action. Expanding its activity
history showed four collapsed summaries: Runtime prepared, Feishu chat search,
message-send instruction lookup, and Feishu message send. No raw command was
present at that level. Expanding only the message-send summary revealed the
matching requested and completed command records. The repeated `1/1` stage card
was absent where its text matched the final Agent response.
