# Feishu authorization on-demand deployment verification - 2026-09-07

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## Behavior and correction

Source commit `18ab7e3` changes Conversation authorization recovery to start only
after a Runtime `command.requested` event identifies a Feishu CLI capability using
the User identity. Selecting or opening the Connector does not request account
authorization. The recovery request uses only the scopes reviewed for the
attempted capability.

Repeated authorization initiation previously returned HTTP 500 when a pending
attempt already existed. The database upsert updated the existing row, but the
following GORM query retained the newly generated primary key and could not read
the updated row. The repository now reads the upserted attempt into a fresh
record. An integration test covers two consecutive authorization starts and
checks that the second response reuses the pending attempt with its updated URL
and scopes.

## Validation

- `go -C backend test ./internal/data/workspace/gormrepo` passed, including the
  repeated pending-authorization regression.
- `make test` passed all Go packages during the deployment gate.
- `make build` passed.
- `pnpm --dir frontend test` passed 184 tests across 25 files. Coverage includes
  shell-wrapped Session commands, Run events, User versus Bot identity, no prompt
  on Connector selection, capability-specific scopes, and the direct recovery
  link.
- `make web-typecheck`, the production Web build, and `git diff --check` passed.

## Deployment and browser verification

The official `scripts/deploy-platform.sh` deployment completed with verified
business and identity database backups. Active release paths are:

- Source: `/srv/agent-workspace/src.release-feishu-auth-on-demand-18ab7e3-20260907`
- Web: `/srv/agent-workspace/web/releases/feishu-auth-on-demand-18ab7e3-20260907`
- Backup: `/srv/agent-workspace/backups/pre-feishu-auth-on-demand-18ab7e3-20260907`

An authenticated production Chrome session loaded a conversation with the
Feishu CLI already selected. The authorization recovery card was absent before
an operation required missing scopes. A historical `im_chat_search` operation
also omitted the card because the current active authorization already contains
its `im:chat:read` scope.

The same browser then started an authorization extension while a pending attempt
already existed. The begin endpoint returned HTTP 200, the platform displayed a
direct continuation link, and Chrome opened an `accounts.feishu.cn` tab titled
`飞书授权`. API readiness remained HTTP 200 and the new API and Worker logs
contained no error-level entries. The external consent was not completed and no
Feishu message was sent, so token issuance and an authorized send command are
not claimed by this verification.
