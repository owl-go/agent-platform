---
status: accepted
---

# 每种 Agent Runtime 使用独立镜像

Claude Code CLI、Hermes Agent、OpenClaw 和 Codex CLI 分别使用独立、不可变的镜像，由平台通过 Runtime Adapter 选择。相比共享工具箱镜像，这会增加构建和镜像管理工作，但能隔离依赖、凭证、许可证、漏洞、升级和回滚，并保证一个 Run 只包含所选 Runtime。

## 影响

每个 Agent Release 固定 Runtime 名称、CLI 版本、Adapter 版本和镜像 Digest。每个 Runtime 镜像独立通过相同的 Production Conformance Suite，可被独立封禁；只要仍有可恢复 Run 引用，原镜像就必须保持可用。
