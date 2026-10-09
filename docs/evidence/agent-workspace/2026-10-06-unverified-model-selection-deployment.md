# 未验证模型选择修复发布验证 — 2026-10-06

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。版本、结果和证据边界保留。

## 发布版本与行为

功能分支 `codex/select-unverified-models` 的提交 `7743dc6` 已合入并推送 `main_temp`，集成提交为 `e0206162dda2430795a687b496ddc6c9deedd235`。未验证 Model Provider Connection 下的可用模型现在可以出现在企业默认模型选项中；设置默认时不再要求供应商预先 verified，仍要求配置 API Key、非不兼容组合以及成功且匹配的 Administrator validation Run。该 Run 只验证当前组合，不改变供应商的验证状态。

最终线上 API 与 Web 为并行发布的 `wechat-four-digit-20261006-1`，源码提交 `6b176f6f141aa0db4755619375f5959e808c03f0` 已包含上述集成提交。本任务没有覆盖该并行发布：独立发布在取得主机发布锁之前退出，未进行数据库或服务切换，未启用的源码、Web staging 和 API 镜像标签已清理。以下为本任务直接执行的实际版本核对。

当前源码与 Web 分别为 `/srv/agent-workspace/src.release-wechat-four-digit-20261006-1` 和 `/srv/agent-workspace/web/releases/wechat-four-digit-20261006-1`。旧 Worker 仍运行 `scan-registration-20261005-1`，没有因本次模型默认配置修复重建。

## 实际执行的门禁

在固定的 `e020616` 发布 checkout 执行并通过：

```bash
pnpm --dir frontend install --frozen-lockfile
make test
make build
pnpm --dir frontend test
make web-typecheck
make web-build
git diff --check
cmp -s AGENTS.md CLAUDE.md
```

完整前端为 55 个文件、609 项测试通过。生产构建使用目标部署的公开 OIDC 配置。功能分支此前在隔离 PostgreSQL 17 上实际运行 `go -C backend test ./internal/data/workspace/gormrepo/...` 通过，涵盖 verified/unverified 供应商、组合验证晋升、继承传播、无效 Run、缺失 API Key、不可用模型和不兼容组合。其他未配置环境的集成 Skip 不作为远端验证。

并行发布完成后，将隔离发布 checkout 固定至实际运行的 `6b176f6`，再次执行模型设置组件测试（10 项）、`make web-typecheck` 和使用同一公开 OIDC 配置的 `make web-build`，全部通过。构建仍有已有的大 Chunk 提示。

## 线上与公网核对

- 当前 API 镜像为 `sha256:6c57ded0c96dcc4758ac0aa615610f4acbcc0b26d2fce3e6a13e2ae82798cd13`；API/Worker 均 healthy，restart=0，API 启动日志 ERROR/FATAL/PANIC 匹配数为 0。
- 运行中的 API 二进制包含新的缺失 API Key 错误文本，旧的供应商未 verified 拒绝文本已不存在。
- 线上 `platform_execution_defaults.go` SHA-256 为 `4175df8f701dedb7171881dc95bb00678fa3eda7e600eb8aa7abb3dcc9f68350`，`SettingsPage.vue` 为 `032178655280351df81678d1a9e888cf21f95c24f295aaa0b21df16b642ae282`；均与已验证修复的源码相同。
- 公网首页、Health、Readiness、OIDC Discovery 均 HTTP 200。
- 公网 HTML、模型设置页面、i18n 和入口 JS 与实际运行提交 `6b176f6` 的本地生产构建逐字节一致。

| 产物 | SHA-256 |
|---|---|
| `index.html` | `3b6100c6934c339408b9e97981d770b57fe9313c64908d4316cfacf552ba21f3` |
| `SettingsPage-DIFZJQKs.js` | `8dc0c2f337d948b89e9df375b87001cc9f0485bb3c4c1dd183e6d99c0a2095be` |
| `i18n-CBT0QFck.js` | `be484acb18b4aaad01eb41a572c6a126a59f1205bfcbd4d2031d960dee411e0f` |
| `index-Bd8tljnX.js` | `548ecdfbebb3ea979e5ff2ddc78eddef33a701e214be2dac3de5ae0253531629` |

本机公网校验记录为 `/tmp/aw-unverified-models-public-evidence.json`，预检记录为 `/tmp/aw-unverified-models-preflight.log`。临时脚本及生成产物未提交。

## 证据边界

本次证明修复的测试、运行 API 和公网 Web 产物已上线，未使用生产 Administrator 账号实际选择模型或更改企业默认配置，未对 OpenAI 供应商进行真实调用。目标 Linux 的 `make production-conformance-preflight` 因缺少专用 Fixture、探测配置、Runtime 凭证及存储测试配置失败，未执行完整 Production/Sandbox Conformance；没有新增 Runtime Capability 验收声明。最终数据库已随并行发布到 `000073_registration_short_code_limits.sql`，本任务未执行该发布的数据库备份或迁移，不将其记作本任务的验证。
