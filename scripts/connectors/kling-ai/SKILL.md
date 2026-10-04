---
name: kling-ai
display_name: 可灵 AI
description: 使用可灵生成图片或视频、控制主体动作、管理可复用主体、查询生成任务或个人空间灵感值时使用。
version: 0.1.0
author: Agent Workspace
---

# 可灵 AI

通过已选择的可灵 AI MCP Connector 调用官方工具。执行身份为当前 User 在浏览器中授权的可灵账号；在连接器设置中点击「连接」完成 OAuth。授权缺失或过期时回到该入口连接，凭证由平台保存。

## 调用顺序

1. 新会话先调用 `who_am_i`。以返回的账号、工具 schema、可用模型和参数规格为准。只采用已发现且本修订列出的工具。
2. 明确用户要生成的内容、参考素材、模型、时长和数量。需要检查权益时，使用 `who_am_i` 返回的 User ID 调用 `query_membership_and_credits`。生成会消耗该账号个人空间付费灵感值；依照用户明确指令及平台批准流程提交。
3. 按实时 schema 调用 `text_to_image`、`image_to_image`、`text_to_video`、`image_to_video` 或 `motion_control`。保存返回的 `generationId`（旧响应可能为 `generation_id`）。任务提交后不可取消；请求超时、断线或返回不明时先查已知任务，避免重复扣费。
4. 用 `query_tasks` 轮询同一个任务，逐渐拉长间隔，遵守服务端限流和任务状态。完成后从 `works[].url` 提取实际结果并展示；执行失败时说明可操作的错误原因。结果链接通常有效 24 小时。

## 工具与素材

官方指南列出 16 个工具：身份与规格发现 `who_am_i`；五个上述生成工具；动作库 `motion_library_list`；主体 `element_create`、`element_list`、`element_get`、`element_update`、`element_delete`；任务 `query_tasks`；上传 `file_upload`；权益 `query_membership_and_credits`；退出 `logout`。工具清单已对照指南核实，真实账号的业务调用验收尚未完成。

需要动作参考时先查询 `motion_library_list`，使用真实返回的动作 ID。需要复用主体时先查询 `element_list`，按 ID 调用 `element_get`；创建、更新、删除主体须有用户的明确请求并遵循平台批准。`file_upload` 只申请一次性上传票据，文件字节仍需调用方传输；仅在实际返回目标通过平台 Egress 校验时上传，否则报告当前上传受限。账号退出优先使用连接器设置中的断开操作，以同步平台授权状态。

## 失败与边界

- 只使用当前连接器 MCP 工具，密钥或 Token 不进入提示词、参数、URL、输出或文件。
- 参数错误时重新读取已发现 schema 和 `who_am_i`，不能猜测模型名、尺寸、价格或主体 ID。
- OAuth 失败时重试平台连接入口；不要改用网页 Cookie、手填 Token 或第三方服务。
- 目前 MCP 使用个人空间付费灵感值；团队权益、赠送灵感值和错峰免费模式不可在此调用。
- 5 QPS 是上游上限，轮询采用更低速率；遇到 429 时遵守 Retry-After。非会员同时视频生成任务限制以实时返回为准。
- 不把生成任务已提交、查询到任务或协议测试通过写成媒体生成成功。完成状态与结果 URL 才是业务成功依据。
