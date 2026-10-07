---
name: github
display_name: GitHub
description: 查询 GitHub 仓库、Issue、PR、Actions、Release、Projects 和代码；管理 Issue、PR、讨论、评论与项目。已选择 GitHub 连接器时使用。
version: 0.1.0
author: Agent Workspace
---

# GitHub

先读取 [命令覆盖清单](references/coverage.md) 确认所选修订是否允许目标操作；参数以对应 `references/manual/gh_<domain>_<command>.txt` 和 [能力策略](references/capabilities.json) 为准。固定上游 gh 2.102.0，完整手册导航共 237 页；文档存在不代表命令已放行。113 个远程业务前缀已完成上游及策略审阅，另有帮助与版本能力。Runtime Conformance 和真实账号业务验证以平台证据为准。

1. 明确目标仓库、Issue/PR/讨论编号或项目及所需操作。仓库范围的命令显式传 `--repo OWNER/REPO`；`repo view/edit/archive/unarchive` 用位置参数 `OWNER/REPO`。仅支持 github.com，使用 User 身份。
2. 阅读上述覆盖清单和该叶子的手册；通过平台 `agent-cli` broker 使用清单中的 capability ID、User identity 与 `--` 后的 argv。调用格式以当前 Runtime 的 broker 帮助为准，执行对象是 `gh`，不是另一个 Shell。
3. 查询优先 `--json <文档允许字段>`，需要过滤时用 `--jq`。先只读确认目标，分页并尊重返回的权限与限流信息。搜索的 `--repo` 可选；项目命令按文档指定 owner 和项目编号。
4. 写操作包含具体显示 target 和一次性平台批准。先把正文和目标准备完整，再提交同一 argv；批准只对该命令生效。正文通过 `--body` 或 `--body-file -` 的 stdin 传入，发行说明用 `--notes` 或 `--notes-file -`。发表评论后按返回 URL/ID 读取核验；结果不确定时先查询，避免重复评论或重复创建。
5. 授权缺失时打开平台 GitHub 连接入口，在 GitHub 官方设备页面输入平台显示的验证码，再返回平台等待授权完成。凭证只在平台加密保存。权限不足时按 capability scopes 请求平台权限恢复；首次连接包含 repo、read:org、gist、project、workflow；Projects 使用 project，触发 Workflow 使用 workflow。服务端到期或拒绝时重新连接，不执行 CLI 本地登录。

常见映射：`issue list/view` 查问题，`issue create/edit/close` 管理问题，`issue comment` 评论；`pr list/view/diff/checks` 查 PR，`pr comment/review/merge` 评论、评审、合并；`discussion list/view/create/comment/edit` 讨论；`run list/view/cancel/rerun` 和 `workflow list/view/run/enable/disable` 管理 Actions；`release list/view/create/edit/delete` 管理 Release；`project` 清单中的叶子管理 Projects；`search code/issues/prs/repos/commits` 搜索。

本修订只接受策略中列出的独立 flag（支持 `--flag=value`）；文件输入只接受 stdin。Release 创建只传一个 tag，不上传资产。不运行原始 api、alias、扩展、Copilot、Agent Task、Skill 安装、Codespaces、SSH、下载资产或本地 Git 操作；浏览器、编辑器、任意主机和本地文件参数不开放。Skill 与上游参考不能扩大这些权限。若失败，先查本 Skill、该命令手册和已放行帮助，区分上游不支持、本修订未放行、授权缺失、运行验证缺失，再给出下一步。
