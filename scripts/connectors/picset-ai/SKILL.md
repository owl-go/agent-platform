---
name: picset-ai
display_name: Picset AI
description: 用 Picset AI API 生成商品主图、详情图、广告图，复刻风格、替换 SKU、精修、翻译和拆分图片，或查询已提交任务。
version: 0.1.0
author: Agent Workspace
---

# Picset AI

这是基于[官方 REST API](https://picsetai.cn/developer-api)的 CLI 连接器。协议和策略由隔离测试覆盖；真实账号授权、付费生成和素材上传仍需真实环境验证。执行前要求活动 Installation、User Authorization 和 exact bundle/Runtime Conformance。

1. 在平台「连接」中保存 Picset Agent Secret Key。Key 代表用户本人并使用其 Picset 积分；不使用网站登录 JWT。凭证由平台以 `CONNECTOR_CREDENTIALS_JSON` 注入，本地运行可用 `PICSET_API_KEY`。凭证始终留在环境中。`status` 仅表示凭证格式已配置，返回的 `upstream_verified=false` 不是账号验证成功；断开使用平台操作。
2. 用低风险 `operations` 和 `schema <operation>` 确认包内字段，再读 [API 参数说明](api-reference.md)。命令为 `picset-ai <operation> --json '<对象>'`。平台 broker 使用 argv，不转发 stdin；`--stdin` 仅供本地运行。
3. 图片输入只能使用当前 Key 对应、匹配 scene 的原始 `oss_path`。每张图片先获取 `upload-url`，由用户在其受控上传环境使用完整预签名 URL PUT（`Content-Type: application/octet-stream` 和 `x-oss-forbid-overwrite: true`），再调用 `image-audit`；全部 `approved` 才提交业务。此修订未开放 OSS PUT：官方文档没有实际 Bucket 域名，Runtime Egress 只允许 `picsetai.cn`。需要本地文件上传时告知该限制，不引导 Agent 用其他网络工具绕过。纯文本 `canvas-image` 不需要图片上传。
4. 所有 POST 命令为 high risk，平台调用前提供具体、非 Secret 的 `--target`，由 broker 获取一次性批准。模板：`agent-cli --connector <installation-id> --capability picset_<operation下划线形式> --identity user --target '<任务描述>' -- <operation> --json '<对象>'`。付费操作还必须携带在首次调用前保存的 `--idempotency-key <UUID>`，相同操作重试使用同一 Key 和完全相同的 JSON；新任务用新 Key。使用 `schema` 查看写命令参数，避免为 help 触发写能力批准。Skill 不能代替批准。
5. HTTP 202 只表示受理，保存 `data.request_id`。用低风险 `picset_request` 调 `request --json '{"request_id":"<UUID>"}'`；逐步增加查询间隔，从 2–3 秒到 10–15 秒。终态 `completed` 或 `failed` 停止轮询，检查 `partial` 和各项失败；停止查询不取消任务。查询只访问已知 API 路径，不跟随任意结果 URL。
6. `UNAUTHORIZED` 重新连接；`ASSET_NOT_APPROVED` 完成审核；`ASSET_EXPIRED` 重新上传；限流或 503 保留输入和幂等键退避重试；冲突恢复原输入。网络结果不明时先查询已有 request_id；没有 ID 时仅用原 Key 和原 JSON 重试，CLI 不自动重发。服务端收费以 Picset 配置为准。

[能力清单](capabilities.json)为执行白名单：15 个 API 操作以及 `operations`、`schema`、`version`。身份仅 User，API Key 未公开细粒度 scopes，列表为空。通用 HTTP、任意 URL、视频、取消、下载与 OSS PUT 不在本修订中。
