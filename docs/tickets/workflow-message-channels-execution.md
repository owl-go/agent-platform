# Workflow Message Channel 执行计划

需求与实施跟踪：[GitHub Issue #62](https://github.com/owl-go/agent-platform/issues/62)。

状态：2026-10-03 全部 13 个渠道的文本 Adapter 和配置项已实现，待真实应用账号和生产验收。用户先实施五个渠道，随后要求其他八个一起实现。行为依据为 [产品规格 §5.7](../product/agent-workspace-requirements.md#57-workflow-message-channels)；协议、当前代码 seam、推荐默认值与一手来源见 [接入设计](../technical/workflow-message-channels.md)。

## 目标与范围

Workflow Settings 增加消息渠道配置，让获授权的外部参与者直接提问、连续追问，并在原聊天收到 Workflow 答复。目标名单完整覆盖 Telegram、Discord、Slack、Matrix、WhatsApp、Signal、DingTalk（钉钉）、Feishu/Lark（飞书）、WeCom（企业微信）、WeChat（微信）、QQ Bot、BlueBubbles（iMessage）、Yuanbao（元宝）。

首期公共基线为文本问题和完成后的文本答复、private allowlist、群 @、sender 隔离上下文、Run History 溯源与回复失败恢复。媒体、群共享模型历史、流式卡片、公开匿名受众、跨 Workflow 自动路由、自动审批、广播和主动定时推送不属于此基线。底层不支持的群/线程/加密房间必须明确拒绝。

## 批次进度

| 批次 | 交付 | 验收与依赖 | 当前状态 |
|---|---|---|---|
| WMC-01 | 公共领域/Proto/追加 Migration、owner 管理、Audience、版本化凭证、接入 validation、fake Adapter 与幂等 Inbox | 明确速率/backlog/正文与 tombstone 期限；真实 PostgreSQL 并发、签名拒绝、事务回滚、停用竞态；默认配置与边界测试 | 已实现，本地 PostgreSQL/协议测试通过，真实账号验证待补 |
| WMC-02 | channel 初始/追问事务、来源、队列、Credits、terminal Outbox 与回复 Worker | 同一问题只执行/结算一次；unknown 不盲重试；partial chunks/restart/期限；不破坏既有手动/API/Schedule | 已实现，事务回滚、队列、Credits、unknown 恢复通过 |
| WMC-03 | 设置页第六分组、测试指引、启停、失败恢复与 Run History 来源 | 中文/英文、移动/桌面、加载/空/错误态；配置与 health 分离；重发回答与重新执行的影响清楚 | 已实现，前端 417 项测试、类型检查、构建通过；真实浏览器账号验收待补 |
| WMC-04（B1a） | Telegram Webhook Adapter，跑通第一条真实执行闭环 | 私聊、群 @、重复 Update、长答案、Credits、连接断开与 owner 停用；取得 API/Worker 到真实 IM 的证据 | Adapter 和公共闭环已实现；真实 Bot 收发及 Runtime 证据待补 |
| WMC-05（B1b） | Discord、Slack、钉钉、飞书/Lark | 四个供应商各自的权限、身份、线程、ACK、回复窗口与重连；飞书/Lark region 分别验收；每个 Adapter 独立验收并出证据 | 四个 Adapter 与 UI 已实现，recording 发送/接收归一化测试通过；各真实应用账号验收待补 |
| WMC-06（B2 准入核查） | 并行于 B1 的 Matrix、WhatsApp、WeCom、WeChat、QQ Bot、Yuanbao 前置核查清单 | 只做接口/账号/权限检查；微信直连资格和 context_token、元宝固定包与协议、企微原始协议、WhatsApp 现行 AI 条款、QQ 现行被动窗口；不先承诺可用 | 官方协议核查并落地实现；实际账号资格与 WhatsApp 部署政策仍待确认 |
| WMC-07（B2 实施） | 前置核查通过的 Matrix、WhatsApp、WeCom、WeChat、QQ Bot、Yuanbao Adapter | 使用相同公共链路；每个渠道逐个通过真实收发验收。Matrix 加密房间为单独能力项；资格不满足的渠道保留阻塞记录 | 六个 Adapter 与配置项已实现，真实账号端到端验收待补 |
| WMC-08（B3） | Signal 与 BlueBubbles 专用 Bridge 接入 | signal-cli 账号/协议恢复、Mac/iMessage/BlueBubbles、relay 认证、离线补收、密钥隔离、固定版本；需准备额外环境 | 两个 Bridge Client 与配置项已实现，真实 Bridge 环境验收待补 |
| WMC-09 | 跨渠道回归、严格配置、部署与运维 | 关闭默认值、Migration/恢复演练、限流、owner 私有投影、无凭证日志；适用门禁后经 main_temp 发布，独立生产证据 | 严格关闭默认配置与本地回归已实现；部署与生产证据待补 |

渠道前置核查可与 B1 实施同时推进。批次不附虚构工期；实际先后可按账号可用性调整，但全部 13 个渠道继续保留在目标范围。

## 尚待真实环境验收的关键场景

- [ ] owner 配置、真实固定回复 validation、启用；其他 User 与 Administrator 无权看凭证/私有聊天。
- [ ] 同一私聊两次问题产生一个根 Run Conversation、两个有序 Run，答复有上下文且来自冻结的原始 goal。
- [ ] 不同 sender、群、线程、tenant、渠道、owner 不混用历史；外部消息不能改变资源选择或审批。
- [ ] 第六个 queued 请求以相同 rejection 幂等返回；Credits 不足不执行模型，不把额度明细发给外部。
- [ ] Inbox 与 Run 关联、终态/Event/Credits 与 Outbox 均能验证事务回滚；重复事件、并发消费者与 lease 恢复不产生多 Run/多结算。
- [ ] Run 成功但外部发送失败仍显示执行成功与回复失败；重发只发送既有答案。unknown 发送需确认或供应商幂等证明。
- [ ] 分块答复、原聊天/thread、群提问者标记、消息排序、Retry-After、短期回复能力和过期状态正确。
- [ ] API/Worker/Bridge 重启恢复、游标提交、初次历史 sync 过滤、自身/Bot 回声过滤及对话 reset 无重复执行。
- [ ] 停用、撤销受众、轮换凭证/换号、Workflow 删除、owner disable 立即阻止新领取，并对在途发送记录其无法撤回的边界。
- [ ] Connector User Action Wait 仅 owner 在认证浏览器处理，IM“同意”无效；超时与取消不泄露操作细节。
- [ ] 共享 Workspace 影响明确，私有 Artifact 下载地址不自动外发，允许域/Bridge 网络与精确脱敏拒绝绕过。
- [ ] 每个渠道至少有真实接收→Runtime→原聊天答复与连续追问证据；不支持能力实际拒绝，尚未验收的账号不允许跳过验证启用。

## 实施时的门禁

Go 先跑变更包测试；公共契约、凭证或跨模块改动再跑 `make test`、`make build`。Web 跑目标组件测试、`make web-typecheck`、`make web-build`；首次使用前 `pnpm install`。生成 Proto/Go/OpenAPI/TypeScript 后检查差异。数据库事务与恢复使用真实临时 PostgreSQL，不以 Skip 作为通过。涉及 Runtime/Sandbox 的变化继续执行适用 Conformance，不用 IM 测试替代 Linux + runsc 验收。

完成代码批次后提交并推送会话开发分支；适用检查通过后合入 `main_temp`，从 `main_temp` 发布。每个上线渠道单独更新证据与能力状态，不能把整个目标名单一次性标记完成。

## 本轮实现与本地证据

开发分支 `codex/workflow-message-channels`，需求设计提交 `118085a`。本轮新增 Domain/Application port、五个 Data Adapter、Proto/Go/OpenAPI/TypeScript 生成契约、Migration 000066、owner 配置/验证/启停/reset、回调认证、持久化 Inbox/Conversation/Delivery、Worker 运输循环及设置页第六分组。所有接入默认关闭，配置和各平台设置步骤见 [设计 §9](../technical/workflow-message-channels.md#9-首期部署配置与当前限制)。

本地真实 PostgreSQL 使用临时 `postgres:17-alpine` 容器，每项集成测试建立独立数据库并执行完整 Migration 链；设置 `WORKSPACE_TEST_POSTGRES_DSN` 后运行，十个渠道集成用例没有因缺少数据库而 Skip。覆盖验证不执行模型、十二个并发重复事件只创建一个 Run、sender/owner 隔离、冻结 goal 与 reset、五槽队列和非正 Credits 拒绝、终态/Outbox/结算事务回滚、发送分块和 unknown 确认重发、过期 lease fencing、发送冷却、backlog、凭证轮换、账号停用和 retention。既有 Worker 使用 recording Executor 运行两个连续问题和独立 sender，验证前轮保存答复进入下一轮上下文。

独立协议测试覆盖 Telegram Secret/Unicode mention/编辑与 Bot 忽略，Slack raw-body HMAC/时间窗口/tenant/thread，Discord/钉钉/飞书身份归一化与 tenant/Bot 拒绝，五种发送目标和安全错误、钉钉 Stream 成功 ACK 以 Inbox 提交为前提及网关限制、短期回复地址/过期拒绝。Runtime Executor 使用 recording Adapter 验证渠道 Secret 过滤结果、进度和 Workspace 文件，但不进入 Runtime Request 或凭证挂载；未调用真实模型。

发送冷却、重试与租约使用数据库时钟，避免 API/Worker 与数据库宿主机时钟差使新任务短暂不可领取。Channel 控制锁使用 `NO KEY UPDATE`，与终态 Outbox 的 FK key-share 兼容；真实 PostgreSQL 测试覆盖该锁序边界。已生成契约通过 Buf lint 和 Wire，Buf breaking 与 verify-generated 检查通过。

已实际执行的本地检查：

```bash
pnpm --dir frontend install
make generate
make breaking
make verify-generated
go -C backend test ./internal/biz/workspace/... ./internal/data/messagechannel/... ./internal/platformconfig/... ./internal/service/workspace/...
# 下列数据库命令均设置了实际 WORKSPACE_TEST_POSTGRES_DSN：
go -C backend test ./internal/data/workspace/gormrepo -run TestChannel -count=3
go -C backend test -race ./internal/data/workspace/gormrepo -run TestChannel -count=1
make test
make build
pnpm --dir frontend test
make web-typecheck
make web-build
git diff --check
```

前端 47 个文件、417 项测试通过，覆盖中英文、配置为空/全局关闭、启用授权说明、unknown 重发确认和六个 Settings 分组。Adapter、配置与渠道 Secret Runtime 路径另以 `go test -race` 聚焦执行通过；未匹配测试的 Application 包不记为 race 场景通过。

未取得：五个供应商真实账号/权限、真实接收→模型 Runtime→IM 答复及连续追问、真实 Gateway 掉线/重启补收、飞书与 Lark 分别验收、真实浏览器桌面/移动账号交互、Linux + runsc Sandbox/Production Conformance、新 Digest Runtime 镜像验收、远端 MinIO/OSS 集成以及生产部署。它们没有被本地 mock、Skip 或构建成功替代。本轮没有变更 Runtime CLI/Image/Sandbox 配置，不声称具有新的 Runtime Capability 证据。

剩余八个渠道维持原目标与前置资格核查，不在本轮 UI 或服务器枚举中伪装为可用。恢复可靠性从 Inbox 提交开始；钉钉 fire-forgot 与 Discord 进程内 resume 的边界已单独记录。未知发送不自动重放，owner 可确认重发或等待发送期限收口。

## 收发 Interface 拆分与证据

2026-10-03 根据用户补充要求，将原先包含账号识别、Webhook 和发送的单一 `ChannelAdapter` 替换为 `ChannelAccount`、`ChannelWebhookReceiver`/`ChannelStreamReceiver`、`ChannelSender`，通过 `ChannelTransport` 分别注入。五个供应商按实际接收方式注册，长连接渠道删除不支持的 Callback 方法；公共 Callback、连接 Supervisor 与 Delivery 循环分别使用接收或发送 Interface。配置仍要求完整角色与唯一接收方式，公开 HTTP 路由仍限定当前批准的两个供应商。

新增 Application 契约测试使用互相独立的 receiver/sender fake，验证接收器可独立注入、Inbox 失败不成功 ACK、发送不触发接收或 Run 准入、稳定 Delivery key/原线程/受保护 Reply 保持、缺失发送器安全失败、注册缺失/歧义拒绝、长连接移除时取消。Supervisor 现在传播接收器提供的单条消息 context/deadline，过期 context 的入库失败也通过 sink 返回。

本次实际通过：

```bash
go -C backend test ./internal/biz/workspace/application ./internal/data/messagechannel ./internal/service/workspace ./internal/wiring/...
go -C backend test -race ./internal/biz/workspace/application ./internal/data/messagechannel ./internal/service/workspace ./internal/wiring/workspaceworker
# 以下两条设置了临时 postgres:17-alpine 实例的 WORKSPACE_TEST_POSTGRES_DSN：
go -C backend test -race ./internal/data/workspace/gormrepo -run TestChannel -count=1
make test
make build
git diff --check
```

十个渠道 PostgreSQL 集成用例实际执行通过。此次未修改 Proto、Migration、前端或 Runtime 镜像；之前的生成契约与前端证据见上一节。本次本地检查仍不能替代五个 IM 的真实账号联调或 Linux Production Conformance，也未进行部署。

## 全部渠道扩展与本地证据（2026-10-03）

用户追加要求其他渠道一起实现。本次新增 Matrix、WhatsApp、Signal、WeCom、WeChat iLink、QQ Bot、BlueBubbles、Yuanbao 共八个文本收发 Adapter，保持独立 Receiver/Sender port 和现有 Inbox→Run→Outbox 链路。配置界面提供全部 13 个渠道的字段与中英文部署说明；每个 Workflow 的配置上限调整为 16，容纳全部目标渠道。未安装第三方 Bridge 或申请真实 IM 账号。

Migration 000067 扩展 provider 约束并追加加密接收游标；API/Worker 装配同一游标与 Administrator HTTPS Endpoint 批准列表。配置轮换、渠道删除和 Workflow 删除清理游标。新增 Secret/短期 context_token 使用公共脱敏集合；私聊限定、Matrix 明文拒绝、QQ 被动回复额度和各供应商身份边界由渠道实现负责。

本地协议测试覆盖新增渠道的原始回调签名/挑战、tenant/账号匹配、稳定回复目标与 key、Matrix 初次历史过滤/limited gap/发送前加密检查、Signal SSE 重放与提交后游标、iLink cursor/context_token、BlueBubbles 插入游标/同时间戳跨页与账号变化拒绝、WeCom 长连接握手/回执关联/取消、Yuanbao 官方 Protobuf 字段/签名/原群引用/Inbox 成功后 ACK，以及批准 Endpoint 的私网/metadata/redirect/path 拒绝。加密游标测试使用实际 AES-GCM 校验 AAD，新增凭证与回复能力覆盖入站和最终外发脱敏。

实际使用临时 `postgres:17-alpine` 执行完整 Migration 链。全部 13 个 `TestChannel...` 集成用例通过，包括原有十个用例、游标版本栅栏/停用/轮换/两种删除、十三种 provider 约束和同一 Workflow 全部渠道的容量边界；没有因数据库缺失而 Skip。前端完整 47 个文件、418 项测试通过。

已实际执行的检查（数据库命令均设置真实 `WORKSPACE_TEST_POSTGRES_DSN`）：

```bash
go -C backend test ./internal/biz/workspace/application ./internal/data/messagechannel ./internal/service/workspace ./internal/platformconfig
go -C backend test -race ./internal/data/messagechannel ./internal/biz/workspace/application ./internal/platformconfig ./internal/service/workspace
go -C backend test -race ./internal/data/workspace/gormrepo -run TestChannel -count=1
make test
make build
pnpm --dir frontend test -- WorkflowMessageChannels.test.ts
make web-typecheck
make web-build
git diff --check
```

本轮不改变 Proto 或 Runtime 镜像，没有重新生成契约或执行 Linux + runsc 的 Sandbox/Production Conformance。全部 IM 的真实账号收发→Runtime→原聊天连续问答、WhatsApp 账号政策资格、Signal/macOS Bridge 部署与重启恢复、各平台生产验收仍缺证据；没有上线部署。具体配置和拒绝能力见 [设计 §10](../technical/workflow-message-channels.md#10-新增八个渠道的实现边界)。
