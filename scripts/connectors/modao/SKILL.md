---
name: modao
display_name: 墨刀 CLI
description: 用墨刀 AI 生成 HTML 原型、React 应用和 PRD，查询账号权益及任务结果，或将 HTML 导入墨刀个人空间。
version: 0.1.0
author: Agent Workspace
---

# 墨刀 CLI

本包是 Agent Workspace 的命令行桥接器，连接[墨刀官方 MCP](https://modao.cc/hc/articles/470)。本修订已验证协议与命令策略；真实账号操作尚未验证。托管执行前确认 Installation、所选 User Authorization 和该组合的 Conformance 均有效。

1. 先用低风险 `tools` 查看当前授权账号的工具列表，再用 `schema <业务命令>` 读取参数 Schema。仅使用下表的命令；服务端新增工具不会自动放行。托管调用用 `--json '<对象>'` 传入 Schema 要求的 JSON 对象；`--stdin` 适用于本地直接运行，当前平台 broker 不转发 stdin。不要猜 prompt、HTML 或任务字段名。
2. 执行 `account status` 检查账号、个人空间和权益。结果归属于该凭证绑定的个人空间。CLI 使用平台单次进程注入的 `CONNECTOR_CREDENTIALS_JSON`；本地运行使用 `MODAO_TOKEN` 环境变量。授权通过平台加密 API 管理，不把凭证写入参数或文件。`status` 会读取真实账号；缺少凭证时返回未连接。`unauth` 仅提示断开平台授权，不撤销上游令牌。
3. 按用户请求选择生成类型。生成与导入须经平台一次性批准；每次高风险 broker 调用在分隔符前提供具体、非 Secret 的 `--target`，包括同一 capability 的 help 调用。命令模板为 `agent-cli --connector <installation-id> --capability <下表 ID> --identity user --target '<产物名称或任务>' -- <业务命令> --json '<参数>'`。低风险命令可省略 target。Skill 和本地 CLI 都不能代替 broker 批准。
4. 生成返回 running 时保留真实 task_id，稍后用 `task result` 单次查询。生成工具可能等待约 100 秒；不要重新创建相同任务。未知写入结果先查任务和墨刀个人空间，确认结果后再决定是否重试。
5. `proto import` 仅用于 HTML 内容或已完成的 HTML 任务。React、Vue 和任意外部 URL 不能直接导入。写后按返回的任务 ID、预览或原型链接核对结果。

| 业务命令 | capability ID | 官方 MCP 工具 | 风险 |
|---|---|---|---|
| `account status` | `modao_account_status` | `get_account_status` | low |
| `task result` | `modao_task_result` | `get_task_result` | low |
| `generate auto` | `modao_generate_auto` | `generate` | high |
| `generate html` | `modao_generate_html` | `generate_html` | high |
| `generate react` | `modao_generate_react` | `generate_react` | high |
| `generate prd` | `modao_generate_prd` | `generate_prd` | high |
| `proto import` | `modao_proto_import` | `import_to_proto` | high |

`modao_tools`、`modao_schema` 和 `modao_version` 是低风险诊断能力；`schema` 仅接受上表业务命令。所有能力只允许 User 身份，网络仅访问 `modao.cc`。官方未公布细粒度 scopes，本修订不填写虚构 scopes。

结果为 `{ok,data,request_id,warnings}` 或 `{ok:false,error,request_id}`。`authorization_required` 走平台连接流程；`upstream_unsupported` 表示当前已授权目录缺少该工具；未审查命令返回 `invalid_request`；`tool_error`、传输失败和超时先核对任务。工具目录和协议 fixture 不能证明真实业务成功。评论、项目管理、其他墨刀产品及通用 raw MCP 调用不在本修订中。
