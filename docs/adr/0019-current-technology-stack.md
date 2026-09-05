---
status: accepted
---

# Agent Workspace 现行技术栈

Agent Workspace 前端使用 Vue 3、TypeScript 与 Vite；服务端使用 Go、Kratos、Wire、GORM、PostgreSQL 和严格 YAML 配置；身份由 Keycloak OIDC 提供；对象存储支持 MinIO 与阿里云 OSS；Claude Code、Codex、Hermes、OpenClaw、PI Agent 分别使用独立不可变镜像并在 Docker + gVisor 中执行。

`backend/api/workspace/v1/workspace.proto` 是普通业务 HTTP API 的权威契约，并生成 Kratos HTTP、OpenAPI 和前端类型。Run Event SSE 与文件下载是显式的自定义流式 HTTP 端点。业务按 Account、Workspace 与 Credits 三个限界上下文组织，依赖方向保持 Domain/Application 到 Data Adapter；GORM Model、HTTP DTO 与 YAML Config 不进入 Domain。Credits 上下文由 ADR-0024 引入并取代本 ADR 早期仅列出 Account 与 Workspace 的上下文清单。

该决策取代早期企业 Coding Agent 控制面的分上下文、跨角色审批、Agent Release、Repository Binding、Coding Task、Operations Console 和 Review Branch 设计。历史 ADR 已删除，不能再作为当前实现依据。
