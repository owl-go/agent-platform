# Feishu CLI Connector repair - 2026-09-07

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Reproduced failures

The production Session failed before model execution with `listen unix ...
cli-broker.sock: bind: invalid argument`. Its Workspace socket path was 114
bytes long. A real Unix socket regression test reproduced the same failure
with a long Workspace root while the existing short-path test passed.

After the socket repair, a real Session reached the broker. The original
built-in Feishu profile exposed `auth status`, but the official
`@larksuite/cli@1.0.93` rejects interactive auth commands when credentials are
provided externally. Real Docker then reproduced a second failure: connecting
a `network=none` container to the Egress network is rejected because Docker
does not allow the private `none` network to coexist with another network. A
stopped container also has no assigned address for Egress policy installation.

The web authorization flow sent `scopes: [null]` when the server omitted its
optional scopes field. Enabling a Feishu Connector also stopped in
`waiting_for_user` instead of opening the OAuth flow.

An existing Session retained the exact Feishu Connector revision from before
the current Codex Runtime image was published. Its frozen Runtime digest list
therefore rejected the current image before model execution even though the
same Definition and bundle had since passed exact-digest Conformance. The
Session response could only tell the User to visit Connector management and
did not present an authorization action in the conversation.

## Changes

- `cbffe60` treats omitted authorization scopes as an empty list.
- `6d50e2f` opens the first Feishu OAuth flow immediately after Enable. Existing
  active authorizations remain connected; a separate expand-permissions action
  requests the reviewed capability scopes.
- `087849e` binds the broker through a private temporary short pathname while
  retaining the socket in its Stage scratch directory. Cold and Warm Runtime
  containers additionally mount the protected broker directory at
  `/run/agent-cli` and use its short client pathname.
- `5566c43` starts a credential-free `/bin/sleep 900` bootstrap on the Egress
  network, installs policy for its allocated address, then supplies credentials
  only to the reviewed `docker exec` command. The container is removed before
  policy release; restrictive policy remains when removal fails.
- `6f5c9cc` replaces the unsupported auth command with reviewed Feishu
  capabilities for chat search and user-authored message send. The policy
  allows only the declared argument prefixes, identities, scopes, host and
  timeouts. Message send is high risk and still requires command approval.
- `1425e33` refreshes the built-in Feishu profile from trusted package metadata
  during every rebuild so an older persisted profile cannot override the
  current reviewed policy.
- `2ed11dd` lets a retained Connector snapshot use a current passing
  Conformance record only when Definition ID, frozen bundle SHA-256, and current
  Runtime RepoDigest match exactly. It also presents missing Feishu User scopes
  beside the Session or Run Conversation composer, opens the OAuth flow from
  that prompt, retains its direct recovery link, and asks the User to reply
  `已授权` after Feishu confirms completion.

## Executed validation

- The long-path Unix broker test failed before the fix and passed afterward.
- Targeted `cliconnector`, `containerprocess`, and `runtimeexecutor` Go suites
  passed, including permissions, cleanup, cold and Warm mount configuration,
  bootstrap failure, execution failure, cancellation, and failed removal.
- `make test`, `make build`, `make web-typecheck`, and `make web-build` passed.
  The frontend suite passed 180 tests in 24 files, including the conversation
  authorization prompt, direct-link fallback, and completed-state instruction.
- A cross-compiled Linux test binary ran on the production Linux + gVisor host:
  all `TestUnixBroker` tests passed, including the real Runtime image's
  `agent-cli` talking to the actual broker on cold startup and two Warm
  checkouts. The command backend was a recording fake and made no external API
  call.
- `TestDockerConnectorWaitsForPolicyBeforeCommand` failed against the original
  lifecycle with Docker's real network error. Its allowed and rejected cases
  passed after the change with both the host iptables Gate and the Worker's
  Unix Egress Controller RPC. The checks cover the allocated container address,
  absence of credentials from container configuration, policy installation
  before command execution, rejection without execution, and cleanup.
- The rebuilt Codex image reported `codex-cli 0.147.0` under runsc with a
  non-root UID, read-only Rootfs, dropped capabilities, resource limits and no
  network in the Runtime smoke check.
- The production catalog returned the Feishu Connector to `available` at
  version 40. Normal Worker conformance, rather than a direct database insert,
  produced passing records for every configured Runtime digest. The new Codex
  digest passed at `2026-09-07T11:52:35.831570Z`.
- Production UI verification as user `wuyuewei` showed the Feishu Connector as
  available, enabled, authorized to the existing account, and offering the
  expand-permissions action.
- After the conversation recovery deployment, the existing failed Session
  displayed `飞书 CLI 需要飞书账号授权`, an `打开飞书授权` action, and the instruction
  to return and reply `已授权`. The action was deliberately not clicked during
  verification. The production database contained one passing record matching
  the current Feishu Definition bundle and Codex Runtime RepoDigest exactly.

## Deployment

Public origin: `https://workspace.example.com`.

Codex Runtime RepoDigest:

`127.0.0.1:5000/agent-platform/codex@sha256:9a18fa516d3044f23b3b2588aff83ec66e24c49f7f47b2c98eb5be2f81bf9097`

Active source:
`/srv/agent-workspace/src.release-conversation-auth-2ed11dd-20260907`.

Active service images:

- API: `sha256:0f6c8a12b945022d89a20864521647431b073e06d0ad9191a64a0688609eb16e`
- Worker: `sha256:d839142443440f6b48540d32bfca0113f9b45c7bb48c656b921b6309c9beceb1`
- Web: `/srv/agent-workspace/web/releases/conversation-auth-2ed11dd-20260907`

The runsc runtime now has `runtimeArgs: ["--host-uds=open"]`. Docker validated
the configuration before a HUP reload; `create` and `all` were not enabled.
Only the Stage broker directory is mounted into the model Runtime.

Previous service images, source, platform configuration, and Docker
configuration remain available for rollback. Deployment checked that no
Session or Run was executing before service replacement. API and Worker health
checks passed, their new logs contained no errors, and the public JavaScript
asset was byte-for-byte identical to the production build. Native Resume
remains disabled for the new Codex digest pending complete capability
conformance; platform message context continues to provide Session continuity.

The current built-in policy exposes:

- `im_chat_search`: `im +chat-search`, low risk, user identity,
  `im:chat:read`, and `open.feishu.cn`.
- `im_messages_send`: `im +messages-send`, high risk, user identity,
  `im:message` plus `im:message.send_as_user`, and `open.feishu.cn`.

## Remaining evidence boundary

No Feishu chat lookup or message send was performed during this repair. The
existing authorization does not yet include all newly reviewed scopes, so the
user must explicitly complete the Feishu expand-permissions OAuth flow before
an end-to-end business call can be validated. No group message was sent or
retried, no OAuth action was started during the final UI check, and no account
authorization was disconnected.

This is targeted Linux/gVisor, Egress Controller, Connector, and web validation.
It is not a complete `make sandbox-conformance` or
`make production-conformance` run. Other Runtime images were not rebuilt and
their broker clients were not exercised here.
