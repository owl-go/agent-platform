---
name: ai-hive
display_name: AI-Hive
description: 使用 AI-Hive 查询账号余额及模型，进行文本对话，预检并生成图片或视频，上传用户指定媒体或查询生成任务。
version: 0.3.0
author: Agent Workspace
---

# AI-Hive

本包使用官方 `@infimind-next/ai-hive-mcp@0.3.0` stdio 服务。已通过 MCP 握手和 8 个工具发现，核对了上游参数定义；真实账号授权、上传及付费生成需要目标环境验证。执行前确认当前 User 的 Installation 和 Authorization 有效。API Key 来自蜂巢 AI 网页的「MCP 接入」，由平台加密保存并仅通过 `AI_HIVE_MCP_KEY` 注入本次进程。

## 选型与生成

1. 需要核对账号或余额时调用 `get_user_info`。用 `list_models` 按 `TEXT`、`IMAGE` 或 `VIDEO` 选型；快速选型使用 `detail: "lite"`，需要完整价格时使用 `full`，保留 `publicModelId`。
2. 用 `get_model` 获取该模型及所选 `routingMode` 的契约。路由只能取 `COST_FIRST`、`SPEED_FIRST`、`SUCCESS_FIRST`。所有模型参数、媒体要求、枚举和默认值均来自返回的 schema。
3. 图片或视频先调用 `generate_image` 或 `generate_video`，明确传 `dryRun: true`。读取 `resolvedParams`、`pricingSnapshot` 和 `estimatedReservationMicros`；以人民币元展示估价（1 元 = 1,000,000 micros）。预检成功且用户确认这笔费用后才进入正式生成。
4. 正式生成复用已确认的模型、路由和业务参数，`params` 使用预检的 `resolvedParams`，显式传入预检的 `pricingSnapshot` 与稳定唯一 UUID `clientRequestId`，去掉 `dryRun`。用户修改输入或价格变化时重新预检，并让用户确认新的估价。
5. 收到 `taskId` 后用 `get_generation_task` 查询状态及结果，直到服务端终态或用户取消等待。查询时复用任务 ID，避免再次创建任务。仅依据服务端结果报告成功、失败或仍在处理中。

## 文本与媒体

`chat_text` 接收所选 `publicModelId`、`routingMode`、非空 `messages`、稳定 `clientRequestId` 和匹配的 `pricingSnapshot`，可传 `thinkingEnabled`。消息角色为 `system`、`user`、`assistant`；媒体放在消息的 `mediaIds`，使用当前契约而不是已废弃的 `attachmentIds`。文本调用会消费账号余额；确认用户授权该次调用后执行。文本工具没有图片／视频的 `dryRun` 预检能力。

用户明确指定要发送的本地媒体时，可用 `upload_media_from_path` 获取 `mediaId`；图片放进 `imageMediaIds`，视频放进 `videoMediaIds`，音频放进 `audioMediaIds`，首末帧按模型契约填写 `firstFrameMediaId`、`lastFrameMediaId`。路径必须在本次 Workspace 可访问范围内。上传依赖服务端返回的签名 PUT 地址，该地址的域名需要目标 Egress 策略核验；当前修订没有上传域名的真实环境证据，缺少核验时先停止上传。下载或展示结果仅使用服务端实际返回的媒体地址。

## 重试与失败

- 传输中断、超时或 `REQUEST_IN_PROGRESS`：复用原 `clientRequestId` 和相同业务参数；已有 `taskId` 时优先查询。
- 参数、余额、媒体或已缓存的模型调用失败修正后：使用新的 `clientRequestId`。
- 价格过期：保持业务参数和原 `clientRequestId`，重新预检，确认新费用并使用最新 `pricingSnapshot`。
- `REQUEST_OUTCOME_UNKNOWN`：停止自动重试，记录请求／任务 ID，请用户联系服务管理员核查。
- 授权缺失或失效：让当前 User 重新连接；诊断、会话输出及上传文件均不包含密钥。
- 服务端不支持 `get_model` 或 `dryRun`：停止依赖该能力的生成流程并报告版本不匹配。Skill 的费用确认要求不会替代平台授权、权限或一次性批准。
