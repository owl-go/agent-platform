# AI 协作规范同步记录（2026-10-04）

本次执行 `$ai-project-conventions sync`，只更新规范与文档，不修改产品实现或部署服务。

## 比较范围

- 用户未指定 Git 基线；开始时工作区干净，复用本会话分支 `codex/feishu-channel-credentials`。
- 使用本地保存的 `origin/main`（`257ed164164ed276734a2bc99dc656f7202db9ad`）至同步前 HEAD（`eed1cea95a4fa517bc3884ace22740946a0f1b0f`）的三点差异，共 48 个变化路径。该基线未在本轮重新拉取，不能代表此刻远端最新状态。
- 读取相关差异、Application 端口、Worker/Runtime Executor 调用、供应商 Adapter、GORM Repository、Migration 与相邻测试，并核对领域、产品、技术设计和适用 ADR。补充检查根目录入口及现有构建命令。
- 这是消息渠道相关变化的文档同步，不是全仓库质量评分或完整历史审计，不能证明更早历史和其他模块没有漂移。

## 候选影响与结论

使用技能的 `collect_project_evidence.py --since origin/main --format json` 收集线索。其七类提示覆盖上述 48 个路径，分类可重叠；Go 测试和公共端口还需人工补充识别。

| 候选影响 | 证据与结论 | 处置 |
|---|---|---|
| 领域语言（1 个路径） | `message_channel.go` 的账号绑定使用认证所得 Tenant ID；[领域模型](../../../CONTEXT.md)已说明钉钉 Corp ID 的自动获取与绑定边界。 | `confirmed`：既有说明匹配，无需新增术语。 |
| 依赖与工具链（2 个路径） | `backend/go.mod` / `go.sum` 新增 Goldmark；[接入设计](../../technical/workflow-message-channels.md)已有 Markdown 解析与平台降级边界。构建入口未改变。 | `confirmed`：沿用清单作为版本权威；工具链和新增 ADR 为 `not-applicable`。 |
| 实现与接口（39 个路径） | 涵盖配对、动态回复、事件预览、供应商协议、回调恢复及前端接入。旧架构说明仅列 Typing 接口，未覆盖 Response/Reaction、回复 revision 与加密 Reply 恢复。 | `confirmed`：更新[服务架构](../../technical/service-architecture.md)与[Runtime 契约说明](../../technical/runtime-adapter.md)；产品交互及平台协议已由既有规格和接入设计覆盖，不重复复制。 |
| 数据与迁移（1 个路径） | `000069_message_channel_responses.sql` 和 GORM Repository 保存回复状态及 revision fence；回调能力仍来自 Inbox 的加密 Reply，暂定答案只在内存。 | `confirmed`：补齐服务架构中的 Migration 000066–000069 职责与恢复边界；无需修改不可变 Migration。 |
| 安全与权限（1 个路径） | `message_channel_secrets.go` 及预览 Sink 将回复能力纳入精确值脱敏；配对复用认证连接、由 owner 确认受众，不直接创建 Run。 | `confirmed`：架构说明补齐 host 运输与公开预览边界；既有安全意图未改变，无新增安全例外。 |
| 测试与质量（1 个启发式路径及相邻 Go 测试） | 前端测试、响应/配对测试、事件预览测试与数据库集成测试对应当前行为；数据库测试在未配置 `WORKSPACE_TEST_POSTGRES_DSN` 时会 Skip。 | `confirmed`：入口补充数据库 Skip 说明及文档检查方式。已有测试结果保留原日期与范围，本轮不追认代码或生产验收。 |
| 既有文档（3 个路径） | [产品规格](../../product/agent-workspace-requirements.md)、领域模型、接入设计已经记录此次渠道行为；历史执行记录明确区分目标、实现、部署和真实账号验收。 | `confirmed`：保留其权威内容与历史证据；新增入口路由，不改写历史测试数字。 |
| 入口中的现状漂移 | `AGENTS.md` 仍称四个独立 Runtime 镜像，且 Contract 起点缺少 `backend/` 前缀；[ADR 0038](../../adr/0038-unify-runtime-engine-image.md)及现有目录已经采用统一镜像与 CLI Builder。 | `confirmed`：修正代码地图与路径，避免固定数量缓存；`CLAUDE.md` 同步为逐字节镜像。 |
| CI/CD、生产操作与已批准决策 | 本比较范围没有流水线或部署配置变化；统一镜像决策已经记录，消息渠道行为已有规格依据。 | `not-applicable`：不新增 ADR、不执行部署，保留 `main_temp` 集成及保护 `main` 的规则。 |

## 本轮验证与未验证范围

- 入口镜像：`cmp -s AGENTS.md CLAUDE.md` 通过。
- 补丁检查：`git diff --check` 通过。
- 只读 Python 检查通过：11 份文档的 15 个相对 Markdown 文件链接、入口中的 82 处仓库路径引用均存在，收集器候选集合覆盖上述 48 个变化路径。此检查不验证远端 URL 或 Markdown 锚点。
- 入口列出的九个 `make` 目标均由根目录 `Makefile` 定义；这属于命令存在性检查，不表示运行了这些目标。
- 本轮仅文档变更，未重跑 Go/Web 测试与构建，也未运行 Linux + gVisor Production Conformance。已有结果以原日期化证据为准。
- `needs-confirmation`：外部 IM 客户端的真实账号、Markdown 显示和流式闭环验收，仍按[接入设计](../../technical/workflow-message-channels.md)的各项证据单独判定；部署健康不能代替这些验收。本轮没有向真实账号发送测试消息。
