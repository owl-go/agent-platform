---
name: mindbye-contract-review
display_name: 明白律师合同风险审查
description: 审查合同文本或文件，查询明白律师异步合同审查任务，交付服务返回的风险报告和附件。
version: 1.0.0
author: Agent Workspace
---

# 明白律师合同风险审查

1. 确认用户指定合同来源、审查立场（甲方、乙方或具体其他主体）和谈判尺度（强势、均势或弱势）。缺项时先收集，立场和尺度分别记录。提交时将合同文本或文件传至明白律师，仅使用用户为此任务提供的材料。
2. 读取当前 MCP 工具列表和 inputSchema。2026-10-02 现网发现 `contract_review_prepare_upload`、`contract_review_submit`、`contract_review_result`；下载的上游技能曾使用 `submit_contract_review`，调用以现网工具为准。
3. 文本审查向 `contract_review_submit` 传 `contractText`；文件审查优先用可访问的原件引用，或通过 `contract_review_prepare_upload` 获取临时上传信息。按返回的 uploadMethod、uploadHeaders 将原件二进制上传至 uploadUrl，成功后传 sourceFile.fileId。保留 Word 原件供服务生成批注，文件模式的 contractText 为空。上传地址不是文件下载地址。
4. 按 Schema 设置 reviewPosition，并在 userQuestion 明确写入立场和尺度。当前 Schema 不为 Agent Workspace 提供 agentPlatform 枚举，因此省略该字段。仅发送 Schema 支持且实际取得的参数。
5. 保存提交返回的 taskId，以同一 taskId 调用 `contract_review_result`，includeArtifacts=true。按上游建议等待 50、50、30、30、30、30、15、15、15、15、15、15 秒，最多查询 12 次；仍处理中则保留 taskId 供稍后继续。网络超时保留任务，不重复提交。
6. 仅将成功结果中的正式报告和实际附件作为服务交付。报告保留原文与风险结论；下载附件成功后交付文件，附件缺失或下载失败分别说明。失败或未知状态如实报告，不生成替代的“正式报告”。重要合同由律师结合完整材料复核。

当前公开端点无需账号凭证。若服务以后要求授权或返回 401/403，暂停调用并说明连接需要更新。原件上传只使用服务返回的 HTTPS 地址与请求头；执行环境不支持二进制上传时说明限制，等待可访问的原件或用户明确提供文本。

官方接入与技能下载：https://www.mindbye.com/mcp
