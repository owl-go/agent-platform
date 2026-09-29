---
name: wecom
display_name: 企业微信
description: 在 Agent Workspace 中使用本修订放行的企业微信 CLI 命令查询身份、成员和文档，创建待办，查看最近机器人会话或发送消息。
version: 1.3.4
author: Agent Workspace
---

# 企业微信连接器

本修订使用 `@wecom/cli@1.3.4` 的固定 Linux 包。安装 Connector Package 后，User 在 Connector Installation 中提供自己 API 模式机器人的 `bot_id` 和 `secret`，由平台加密保存并仅在单次命令中注入。适配入口按上游签名协议为每次命令换取短期令牌，不保存令牌或 CLI 的本地凭据文件。Secret 轮换后更新该 Authorization。

包内保留与该 CLI 版本对应的[上游 Skills 索引](UPSTREAM.md)和[命令参考](references/upstream/docs/cli-reference.md)。这些文件用于确认参数及业务约束；上游的安装、`auth init` 和能力清单不改变本修订的授权方式或七项命令白名单。

## 调用顺序

1. 选择已安装的企业微信 Connector，确认平台显示当前 Authorization 有效。提供的凭证 JSON 形如 `{"bot_id":"...","secret":"..."}`；实际值只提交给平台授权界面。命令实际调用前由平台复验安装、授权和策略。
2. 先读[上游公共 Skill](references/upstream/skills/wecomcli-shared/SKILL.md)中与身份和输出有关的规则，再按下表读目标操作的 Skill 与 references。以平台的安装和授权流程代替上游文档中的本地安装和 `auth init`。
3. 只调用下表已放行的命令。需要参数的命令统一传 `--json` 和一个 JSON 对象 argv 值；按上游参考构造，并遵守本修订额外的参数限制。命令的实际服务目录由企业微信在线下发，遇到上游不可用时停止并说明错误。
4. 对写入操作提交明确的目标和内容，等待平台的一次性批准后执行。企业微信要求的成员授权或企业审批仍须完成。
5. 按返回值报告结果。内部 ID 只用于后续调用；面向用户使用名称、主题或可读链接。令牌和其他凭证不进入回复。

| 意图 | 命令前缀 | 上游用法 | 风险 |
|---|---|---|---|
| 获取授权身份 | `identity whoami` | [公共 Skill](references/upstream/skills/wecomcli-shared/SKILL.md) | 低 |
| 搜索成员 | `contact users search` | [通讯录 Skill](references/upstream/skills/wecomcli-contact/SKILL.md) | 低 |
| 搜索文档 | `doc search` | [文档管理 Skill](references/upstream/skills/wecomcli-doc-manage/SKILL.md) | 低 |
| 读取 doc 内容 | `doc contents get` | [文档 Skill](references/upstream/skills/wecomcli-doc/SKILL.md) | 低 |
| 创建待办 | `todo create` | [待办创建参考](references/upstream/skills/wecomcli-todo/references/todo-create.md) | 高 |
| 列出最近机器人会话 | `message aibot sessions list` | [消息 Skill](references/upstream/skills/wecomcli-message/SKILL.md) | 低 |
| 给已核对的会话发送消息 | `message aibot send` | [消息 Skill](references/upstream/skills/wecomcli-message/SKILL.md) | 高 |

`identity whoami` 和 `message aibot sessions list` 不带参数。其他命令只接受 `--json` 和一个 JSON 对象；适配入口阻止文件输出等额外 CLI flag。`contact users search` 最多 10 个关键词，`todo create` 一次最多 20 条。CLI 在线 schema 对可选字段拥有最终解释；报参数错误时停止并据实说明。

本修订的 `message aibot send` 只支持 Markdown 文本，传 `{"chat_id":"当次会话列表中的值","msg_type":"markdown","markdown":{"content":"消息正文"}}`，正文最长 20480 UTF-8 字节。媒体发送需要另一个经审核的修订。

发送消息时，先调用 `message aibot sessions list` 或 `identity whoami`，从当次结果取得目标 `chat_id`，再调用 `message aibot send`。候选不唯一时请 User 选择。搜索或读取文档可能受企业审批限制；报错时指出需完成的上游授权，不通过其他命令绕过。

本修订只放行上表命令。上游提供的邮件、日程、会议、微盘等能力需要另一个经审核的 Connector Revision。包解析和 CLI 启动验证不代表特定企业授权或业务 API 已通过验证。
