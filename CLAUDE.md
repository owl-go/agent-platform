# Claude Code 入口

@AGENTS.md

开始分析或修改代码前，读取并遵循仓库根目录的 `AGENTS.md`。它是项目结构、文档路由、架构边界、安全不变量和验证门禁的唯一权威 AI 指引。

本文件只负责 Claude Code 的引导，不复制 `AGENTS.md` 内容；两者冲突时以 `AGENTS.md` 为准。

## Agent skills

### Issue tracker

本仓库使用 `owl-go/agent-platform` 的 GitHub Issues。参见 `docs/agents/issue-tracker.md`。

### Triage labels

本仓库使用默认的五类 triage 标签。参见 `docs/agents/triage-labels.md`。

### Domain docs

本仓库使用根目录 `CONTEXT.md` 与 `docs/adr/` 的 single-context 领域文档布局。参见 `docs/agents/domain.md`。
