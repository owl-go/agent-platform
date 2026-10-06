# 飞书发送者一次性配对 — 2026-10-04

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

用户已确认飞书企业自建应用账号连接成功，随后因发送者 Open ID 为空无法保存。用户接受通过一次性配对消息自动识别发送者的方式。此改动提供识别和 owner 确认，不绕过明确 Audience 与真实收发验证。

## 实现与验证

- 功能提交：`2aa750d`；会话分支继续使用 `codex/feishu-channel-credentials`。
- 发布源：`main_temp` 的 `4d44bdc9674132baf6243abbb55380a10296bc9a`。
- 发布 ID：`feishu-sender-pairing-20261004-1`。
- 在当前已认证账号或已停用的保存账号上生成 `pair <128 位随机码>`，两分钟到期并受五分钟临时登录期限限制。账号凭证保持服务端加密与 owner/Workflow/配置绑定；保存时重新核对账号、企业与 BindingID。
- 临时飞书/Lark 连接完成认证后才显示配对消息。只接受同一应用和企业的近期私聊 user 文本；只识别第一个匹配者并消费随机码。owner 确认加入名单并保存后才持久化受众。原有发送者保留，手工 Open ID 输入继续可用。
- 配对不进入 Inbox、不发送消息、不创建 Run、不消耗 Credits。关闭/离开配置、取消、保存、期限及 API 生命周期会清理临时连接。已启用或验证中的同一应用拒绝配对；全局 16 个连接上限，每 owner 仍受四个临时登录上限约束。
- 配对查询不取得 Worker claim lock。接口必须通过 OIDC 与 owner/Workflow 检查；重复、跨作用域、请求取消、错误消息、群聊、Bot、断线、过期、重放、连接失败和身份漂移均有聚焦回归测试。

实际通过的门禁：

```bash
make generate
make verify-generated
go -C backend test ./internal/biz/workspace/application/... ./internal/data/messagechannel/... ./internal/data/workspace/gormrepo/... ./internal/service/workspace/...
go -C backend test -race ./internal/biz/workspace/application -run TestSenderPairing -count=10
make test
make build
pnpm --dir frontend test
pnpm --dir frontend test src/components/WorkflowMessageChannels.test.ts src/api/client.test.ts
make web-typecheck
make web-build
git diff --check
```

完整前端测试 52 个文件、582 项通过，目标 API/组件测试 70 项通过。`main_temp` 另通过完整 Go 测试/构建、接口生成校验、目标前端测试和类型检查，并采用线上公开 OIDC 配置完成生产构建；功能分支与集成源的 backend/frontend 内容一致。环境依赖的 PostgreSQL/远端对象存储测试 Skip 不计为集成验收。没有新 Runtime Capability，未执行完整 Linux Sandbox/Production Conformance。

## 发布检查

- 公网入口：`https://workspace.example.com`。
- 源目录：`/srv/agent-workspace/src.release-feishu-sender-pairing-20261004-1`。
- Web：`/srv/agent-workspace/web/releases/feishu-sender-pairing-20261004-1`。
- 备份：`/srv/agent-workspace/backups/pre-feishu-sender-pairing-20261004-1`。业务库与身份库 `pg_restore -l` 及全部备份 SHA-256 校验通过，配置备份保持私有。
- 候选 API `/readyz` 通过；切换前活跃 Run/Session Message 汇总为 0。API、Worker 都 healthy；近五分钟 ERROR/FATAL/PANIC 匹配均为 0。
- API 镜像 ID：`sha256:9c525691f4ea50e19c823c6176b873c57f8636684ae931a24ec321ba9e95c524`。
- Worker 镜像 ID：`sha256:cb2c462c5de5e0b0fe63b761feb48e39d36bff77cd9a5b02ef31262786db1065`。
- 配置 SHA-256 与候选/切换后的 Migration name/checksum 不变，无新 Migration；仅构建 API/Worker/Web。
- 公网 Health、Readiness、OIDC Discovery 均为 HTTP 200。配对接口未认证调用为 401。HTML、入口、Workflow 详情、API Client 与 i18n 五项文件逐字节匹配本地生产构建；产物包含自动识别与配对状态。
- HTML SHA-256：`7bbb691d78a2a1d83184458a6e4729207366aa56a61710edbf701d45bd56d41a`。
- 公网产物、配置/迁移对比、镜像和日志计数保存在 `/srv/agent-workspace/evidence/feishu-sender-pairing-20261004-1`，不包含配对码或应用凭证。

真实飞书/Lark 配对与完整收发仍待用户验收；自动化协议、假连接与组件测试不能替代真实账号消息验证。已请用户刷新、生成配对消息、私聊发送、确认发送者并保存，无需提供 App Secret。

回退使用备份记录的旧源码/Web 指针及保留的 `agent-platform-{api,worker}:pre-feishu-sender-pairing-20261004-1` 镜像。上一版源和 Web 均为 `feishu-account-errors-20261004-1`。
