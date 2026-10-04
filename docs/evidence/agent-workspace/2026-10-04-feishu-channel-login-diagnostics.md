# 飞书消息渠道账号接入错误诊断 — 2026-10-04

用户报告连接失败。线上对应 `POST /api/v1/workflows/{id}/channel-logins` 返回 422，原日志和界面只有通用错误，不能判断失败步骤。用户确认企业自建应用已启用机器人、已开通并发布获取企业信息权限；没有证据可直接归因于缺权限。

失败提交的凭证不会保存，截图中的应用也没有可复用的已保存应用凭证，未能对该账号建立无人值守的真实复现。临时只读诊断程序已删除，未输出密钥。自动化验证覆盖错误分类与提示链路；发布后用户刷新并重试，明确回复「已经ok了」，确认账号连接成功。该人工确认不提供此前失败步骤与供应商错误码，原始失败原因仍未确定。

## 修改与回归

- 功能提交：`38607ff`；分支继续使用 `codex/feishu-channel-credentials`。
- 发布源：`main_temp` 的 `bf4609e570ade655ae596bead05409d679cfc354`，包含已上线的 QQ Gateway 恢复改动。
- 发布 ID：`feishu-account-errors-20261004-1`。
- 区分应用凭证拒绝、应用认证失败、机器人信息查询失败、机器人未启用、企业信息权限拒绝及企业身份查询失败。只返回固定分类和数字型供应商错误码/HTTP 状态，不返回供应商原始消息或凭证。
- 中英文界面显示针对步骤的建议和已校验的数值错误码；未知错误保留通用提示。配置字段仍只有 App ID 和 App Secret，接入说明补充企业信息权限及版本发布要求。
- Token 与机器人响应的成功码改为严格数值解析，缺失/null/string 成功码不被接受。企业身份和消息归属校验保持 fail closed。

修复前执行 `go -C backend test ./internal/data/messagechannel -run TestFeishuAccountFailureReportsSafeStep -count=1`：多个步骤均返回 `provider_identity_failed`，回归测试失败。修复前的组件用例也在中英文界面重现了通用提示丢失权限操作建议。修复后实际通过：

```bash
go -C backend test ./internal/data/messagechannel/... ./internal/service/workspace/... ./internal/biz/workspace/application/...
make test
make build
pnpm --dir frontend test
pnpm --dir frontend test src/api/client.test.ts src/components/WorkflowMessageChannels.test.ts
make web-typecheck
make web-build
git diff --check
```

完整前端测试 52 个文件、576 项通过，目标 API/组件测试 64 项通过。最后的类型检查及构建通过。集成源另通过完整 Go 测试/构建、目标前端测试、类型检查和采用线上公开 OIDC 配置的生产构建；集成源与功能分支的 frontend 内容完全一致。

## 发布检查

- 公网入口：`https://47-237-108-63.sslip.io`。
- 源目录：`/opt/agent-platform/src.release-feishu-account-errors-20261004-1`。
- Web：`/opt/agent-platform/web/releases/feishu-account-errors-20261004-1`。
- 备份：`/opt/agent-platform/backups/pre-feishu-account-errors-20261004-1`，业务库、身份库 `pg_restore -l` 及全部备份 SHA-256 均通过。
- 候选 API 私有 `/readyz` 通过；切换前活跃 Run/Session Message 汇总为 0。API、Worker 都 healthy，近五分钟 ERROR/FATAL/PANIC 匹配数均为 0。
- API 镜像 ID：`sha256:5e5192d846e865abedf027299751f7a8c7ac4ccafd91eb642f669db17f85fdd3`。
- Worker 镜像 ID：`sha256:3b82351aec3729183ea60d283ba11ba92a9b9d396ae7b95f7548f7b4b65d40d3`。
- 配置 SHA-256 与候选/切换后的 Migration name/checksum 不变，无新 Migration；只构建 API/Worker/Web。
- 公网 Health、Readiness、OIDC Discovery 为 HTTP 200。HTML 与入口、API Client、工作流详情、i18n 五项产物逐字节匹配本地生产构建；工作流详情含具体错误分类，i18n 含权限提示及飞书错误码标签，没有 Tenant Key 输入标签。
- HTML SHA-256：`21722e2711f84c16a2cdddd0ec2fa2d94d4b9057b8d50f6843817728a6c7aa3e`。公网核对详情保存在 `/opt/agent-platform/evidence/feishu-account-errors-20261004-1/public-evidence.json`。

用户在新版本重试已人工确认真实账号连接成功；Agent 未使用该账号凭证执行无人值守连接测试，也未执行真实消息收发验收。没有新增 Runtime Capability；未执行完整 Linux Sandbox/Production Conformance，专用环境缺口见此前飞书凭证简化发布记录。PostgreSQL/远端对象存储的环境依赖测试 Skip 不计为集成验收。

回退使用备份记录的旧源码/Web 指针和保留的 `agent-platform-{api,worker}:pre-feishu-account-errors-20261004-1` 镜像；上一版 Web 为 `qq-gateway-20261004-2`。
