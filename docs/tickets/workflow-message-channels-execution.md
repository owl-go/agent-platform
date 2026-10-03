# Workflow Message Channel 执行计划

需求与实施跟踪：[GitHub Issue #62](https://github.com/owl-go/agent-platform/issues/62)。

状态：2026-10-03 待实施。用户本轮选择“先完善需求、接入设计和渠道计划”；本轮交付文档，不实施渠道、不上线。行为依据为 [产品规格 §5.7](../product/agent-workspace-requirements.md#57-workflow-message-channels)；协议、当前代码 seam、推荐默认值与一手来源见 [接入设计](../technical/workflow-message-channels.md)。

## 目标与范围

Workflow Settings 增加消息渠道配置，让获授权的外部参与者直接提问、连续追问，并在原聊天收到 Workflow 答复。目标名单完整覆盖 Telegram、Discord、Slack、Matrix、WhatsApp、Signal、DingTalk（钉钉）、Feishu/Lark（飞书）、WeCom（企业微信）、WeChat（微信）、QQ Bot、BlueBubbles（iMessage）、Yuanbao（元宝）。

首期公共基线为文本问题和完成后的文本答复、private allowlist、群 @、sender 隔离上下文、Run History 溯源与回复失败恢复。媒体、群共享模型历史、流式卡片、公开匿名受众、跨 Workflow 自动路由、自动审批、广播和主动定时推送不属于此基线。底层不支持的群/线程/加密房间必须明确拒绝。

## 推荐执行顺序

| 批次 | 交付 | 验收与依赖 | 当前状态 |
|---|---|---|---|
| WMC-01 | 公共领域/Proto/追加 Migration、owner 管理、Audience、版本化凭证、接入 validation、fake Adapter 与幂等 Inbox | 明确速率/backlog/正文与 tombstone 期限；真实 PostgreSQL 并发、签名拒绝、事务回滚、停用竞态；先审定设计默认值 | 待实施 |
| WMC-02 | channel 初始/追问事务、来源、队列、Credits、terminal Outbox 与回复 Worker | 同一问题只执行/结算一次；unknown 不盲重试；partial chunks/restart/期限；不破坏既有手动/API/Schedule | 依赖 WMC-01，待实施 |
| WMC-03 | 设置页第六分组、测试指引、启停、失败恢复与 Run History 来源 | 中文/英文、移动/桌面、加载/空/错误态；配置与 health 分离；重发回答与重新执行的影响清楚 | 依赖 WMC-01/02，待实施 |
| WMC-04（B1a） | Telegram Webhook Adapter，跑通第一条真实执行闭环 | 私聊、群 @、重复 Update、长答案、Credits、连接断开与 owner 停用；取得 API/Worker 到真实 IM 的证据 | 依赖 WMC-01/02/03，待实施 |
| WMC-05（B1b） | Discord、Slack、钉钉、飞书/Lark | 四个供应商各自的权限、身份、线程、ACK、回复窗口与重连；飞书/Lark region 分别验收；每个 Adapter 独立合入和出证据 | 依赖 Telegram 公共链路验收，待实施 |
| WMC-06（B2 准入核查） | 并行于 B1 的 Matrix、WhatsApp、WeCom、WeChat、QQ Bot、Yuanbao 前置核查清单 | 只做接口/账号/权限检查；微信直连资格和 context_token、元宝固定包与协议、企微原始协议、WhatsApp 现行 AI 条款、QQ 现行被动窗口；不先承诺可用 | 待核查，不实施 Adapter |
| WMC-07（B2 实施） | 前置核查通过的 Matrix、WhatsApp、WeCom、WeChat、QQ Bot、Yuanbao Adapter | 使用相同公共链路；每个渠道逐个通过真实收发验收。Matrix 加密房间为单独能力项；资格不满足的渠道保留阻塞记录 | 依赖 WMC-06 与公共链路，待实施 |
| WMC-08（B3） | Signal 与 BlueBubbles 专用 Bridge 接入 | signal-cli 账号/协议恢复、Mac/iMessage/BlueBubbles、relay 认证、离线补收、密钥隔离、固定版本；需准备额外环境 | 待实施 |
| WMC-09 | 跨渠道回归、严格配置、部署与运维 | 关闭默认值、Migration/恢复演练、限流、owner 私有投影、无凭证日志；适用门禁后经 main_temp 发布，独立生产证据 | 待实施 |

渠道前置核查可与 B1 实施同时推进。批次不附虚构工期；实际先后可按账号可用性调整，但全部 13 个渠道继续保留在目标范围。

## 关键验收场景

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
- [ ] 每个渠道至少有真实接收→Runtime→原聊天答复与连续追问证据；不支持能力实际拒绝，尚未验收的能力不展示为可用。

## 实施时的门禁

Go 先跑变更包测试；公共契约、凭证或跨模块改动再跑 `make test`、`make build`。Web 跑目标组件测试、`make web-typecheck`、`make web-build`；首次使用前 `pnpm install`。生成 Proto/Go/OpenAPI/TypeScript 后检查差异。数据库事务与恢复使用真实临时 PostgreSQL，不以 Skip 作为通过。涉及 Runtime/Sandbox 的变化继续执行适用 Conformance，不用 IM 测试替代 Linux + runsc 验收。

完成代码批次后提交并推送会话开发分支；适用检查通过后合入 `main_temp`，从 `main_temp` 发布。每个上线渠道单独更新证据与能力状态，不能把整个目标名单一次性标记完成。

## 本轮交付与缺失证据

已完成：当前实现 seam 检查、13 渠道一手资料核查、统一闭环设计、产品规格与 CONTEXT 术语同步、推荐批次及验收清单。部分供应商页面访问受限已写入设计。

未完成且未声称完成：任何 Adapter、公共渠道代码、数据库迁移、UI、真实 IM 账号验证、Runtime 执行闭环、生产部署。文档检查与提交/推送结果由本轮完成说明记录，代码测试与 Conformance 仅列为实施门禁。
