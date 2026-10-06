---
name: dingtalk
display_name: 钉钉 CLI
description: 使用已审核的 DingTalk Workspace CLI 命令操作钉钉业务资源。
version: 1.0.64
author: DingTalk-Real-AI / Agent Workspace
---

# 钉钉 CLI

本包固定 DingTalk Workspace CLI v1.0.62。先确认 Connector Installation 已启用、账号授权有效、当前 Runtime Digest 已通过该 bundle 的 Conformance。未满足任一条件时停止，不尝试在会话中重新安装 CLI 或绕过授权。

按产品读取同包的[钉钉官方 MultiSkill 入口](reference/dingtalk-shared/SKILL.md)及对应产品 Skill。官方技能是使用说明；最终可调用命令以本修订 [capabilities.json](capabilities.json) 的能力白名单为准。先查目标命令的精确 `dws <path> --help`；参数或风险不明确时查 `dws schema --cli-path "<path>" --compact --format json`。不要猜命令或参数，尤其不要沿用其他版本的指南。

所有业务命令使用当前用户身份和 `--format json`。查找接收人、群、文档、待办等目标时先读取真实 ID；零命中或多候选时请用户消歧。写入前核对组织、账号、对象和内容。高风险 capability 的每一次 `agent-cli` 调用（包括 `--help` 和 `--dry-run`）都必须在命令分隔符 `--` 之前提供非空、可展示且不含 Secret 的具体 `--target`，格式为 `agent-cli --connector <id> --capability <capability-id> --identity user --target "<具体目标>" -- <DWS 子命令和参数>`；例如建群目标可写群名，不得写“当前操作”等泛化值。低风险 capability 不需要虚构 target。

高风险命令遵守平台一次性批准。平台批准卡就是该次命令的显式用户确认；上游 DWS 写命令需要 `--yes` 时，应将它放进同一份待平台批准的 argv，平台会先阻断，批准后才启动 CLI，不要先执行一遍无 `--yes` 的命令。若返回 `user_action_unavailable`，先检查高风险调用是否遗漏 `--target` 并用完整参数重试；不得把 `user_action_unavailable` 解释为钉钉确认功能缺失。写后读取结果或对象验证，超时或结果不明时先对账，不盲目重发。

平台会在每条业务命令执行前检查 Installation 授权，并在缺失或过期时由会话输入区提供钉钉授权入口。不要调用 `dws auth status` 或 `dws profile list` 判断平台授权：它们只检查 CLI 本地 Profile，会把平台注入的短期令牌误报为未登录。平台在单次隔离进程环境中提供短期 Access Token，Refresh Token 保留在平台加密存储中；不得把凭证写入命令参数、Skill 或工作区文件。

官方资料：[DWS CLI](https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli)、[用户指南](https://open.dingtalk.com/document/development/dingtalk-cli-performing-tasks-within)、[应用管理指南](https://open.dingtalk.com/document/development/dev-cli-app-management-guide)。
