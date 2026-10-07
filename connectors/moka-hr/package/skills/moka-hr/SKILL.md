---
name: moka-hr
display_name: Moka HR 招聘
description: 查询 Moka ATS 的职位、候选人、申请状态、招聘流程、部门、Offer 字段、人才库和面试。仅在选择此连接器且企业级 API Key 已连接时使用。
version: 0.1.0
author: Agent Workspace
---

# Moka ATS 查询

1. 通过已选择的 Connector Installation 使用 `agent-cli`，读取 `tools` 或 `schema <command>` 确认参数。调用形式：`agent-cli --connector <Installation ID> --capability <capability ID> --identity user -- <command> --json '<JSON object>'`。托管 broker 不转发 stdin；直接本地 CLI 可用 `--stdin`。
2. 使用 [查询表](operations.md) 中的 command 与 capability。参数使用 API 的 camelCase 名称，空参数可省略 `--json`。职位查询依赖连接时保存的 `moka_org_id`；`candidateId` 和 `applicationId` 各有含义，先从查询结果取相应 ID。
3. 依据结构化结果回答。分页只返回单页和上游 `next`，继续查询时原样传递 `next`，保持原筛选条件；不要把单页条数当总数。`ASC` 响应可能始终包含 `next`，先按用户所需条数及时间范围停止，避免无限翻页。
4. `authorization_required` 导向连接设置；`configuration_required` 补充组织 ID；`permission_denied` 联系 Moka CSM 核对企业接口权限；`invalid_request` 先读 schema；`upstream_error` 保留失败，不将 HTTP 500 当空结果。查询可在稍后重试，收窄超大结果。

此版本包含 14 项只读业务操作，已按官方文档验证路由与模拟请求；真实企业数据调用仍需账号验证。`tools` 和 `schema` 为包内审阅清单，不是运行时 MCP 工具发现。创建职位、上传简历、安排面试、填写反馈、推进阶段、发 Offer、写备注和 Moka People 不在本修订范围。

API Key 是企业级身份，平台 `user` 表示授权记录的所有者，不等同于 Moka 员工身份或个人数据权限。只使用当前 Installation 的授权，在已获准的企业数据范围内查询；不接受模型提供的员工邮箱作为授权身份。空 scopes 表示 Basic Auth 没有 OAuth scope 模型。手机号和身份证按字段默认掩码；邮箱、姓名、简历及附件链接仍可能是个人信息，仅输出回答所需内容。包不自动下载附件。

密钥由平台加密存储，经 `CONNECTOR_CREDENTIALS_JSON` 注入单次进程。使用企业 API Key 与组织 ID 的连接方案，没有浏览器 OAuth；`auth`/`status` 用只读部门 API 验证连接，`unauth` 仅提示断开平台授权。撤销上游 Key 由 Moka CSM 完成。此 Skill 不授予权限、不扩展 manifest 白名单。
