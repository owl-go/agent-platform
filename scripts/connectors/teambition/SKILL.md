---
name: teambition
display_name: 钉钉项目
description: 查询钉钉项目（Teambition）的项目和任务、创建或移动任务、发表或读取任务评论、查询文件链接时使用。通过本修订命令策略和官方参考执行。
version: 0.3.7
author: Agent Workspace
---

# 钉钉项目 CLI

1. 确认选中的 Connector Installation 已启用、Authorization 有效，且 exact bundle SHA-256 与 Runtime RepoDigest 已通过 Conformance。授权缺失时使用平台连接入口；点击平台「连接」打开 Teambition 官方浏览器 OAuth + PKCE 授权；无需手工申请或填写 UserToken。授权结果由平台加密保存，短期 access token 通过单次进程的受控本地传输适配器发送到固定 MCP 服务；refresh token 留在平台。凭证不进入 argv 或 Workspace。
2. 读取本修订 [capabilities.json](capabilities.json) 与[官方业务 Skill](reference/SKILL.md)。官方参考支持的操作多于本修订；它不能增加命令白名单。命令策略已按官方文档和 Skill 审阅，启动和参数边界测试不等于真实账号业务 API 验证。
3. 通过 `agent-cli --connector <id> --capability <capability-id> --identity user -- <argv>` 调用。先查精确叶子 `--help`；路径不清楚时用已允许的分组帮助或只读文档工具。`tools list --refresh --json` 和 `tools describe <tool-name> --json` 能发现当前服务端能力，发现新命令后也要核对本修订策略。未放行的命令需管理员发布新修订，停止该步骤。
4. 用只读命令解析项目、任务和当前身份，绑定真实 ID；多结果消歧后再写。可用业务命令为 `user me`、`project query`、`project task-query`、`project file-detail-query`、`task query`、`task activity`、`task create`、`task move`、`task comment`。文件链接查询按官方参考使用 `--need-sign`。企业权限、动态参数和 TQL 以当前帮助及服务端结果为准。
5. `task create`、`task move` 与 `task comment` 每次调用都使用具体、不含 Secret 的 `--target`，置于 `agent-cli` 分隔符之前，并遵守平台一次性批准。它们的 `--help` 也属于相同高风险 capability；可先用只读 `tools describe`／官方文档核对参数。写后读取对象验证；超时或结果不明时先核查，不自动重发。

## 任务评论

评论操作针对真实任务 ID，不能使用项目 ID。先用 `task query` 绑定用户指定任务；同一会话刚创建且已确认成功的任务可复用其真实 ID，不按名称重新选择其他任务。`task comment -t <taskId> -c <content> --json` 对应官方 `teambition.task.comment`，需 `task:write` 和平台一次性批准，target 中明确任务及待发表的内容。仅在用户明确要求发表且确认目标与原文时调用。保留用户原文，不自行改变相对日期或添加通知对象。官方 Schema 仅将 taskId 列入 required，但文档明确 content 必需，不能发布空评论。

Markdown、mention、fileTokens 仅在用户要求且参数来源已确认时使用；@提及须有真实成员 ID 和正文中的对应显示名称，最多 30 个。当前包未放行成员查询／上传命令，缺少 ID 或已上传文件 Token 时先说明缺失，不凭空构造。发表成功后用 `task activity -t <taskId> --json` 读取评论并按返回 ID 验证。任务评论非幂等，`UNKNOWN_OUTCOME`／超时先查动态，不自动重发。项目级评论未在本修订审阅到对应官方命令，不把任务评论当作项目级评论。

只读文档工具仅允许 `tools call teambition.docs.search` 和 `tools call teambition.docs.get`，使用 `--arguments-json` 或 `--arguments-file` 传结构化参数。业务操作使用公开命令。禁止使用原始 Tool 调用、安装、Skill 更新、登录或配置命令扩展本包执行范围。

CLI 的 `auth status` 检查本地 OAuth 配置，不代表平台 Authorization。本包只读生命周期状态表示凭证格式已配置，不宣称账号权限或 Token 有效。平台内需通过实际业务查询核验。认证过期时使用平台刷新授权，无法刷新时重新打开浏览器连接；普通 403 先核对目标权限，不用更换身份或扩大范围重试。

本修订只连接公有云 `open.teambition.com`。CLI 缓存保存在单次进程临时 HOME 并在结束时清理；禁止 `--config`、端点覆盖及 `--debug`。展示前保留安全的错误摘要和请求 ID。
