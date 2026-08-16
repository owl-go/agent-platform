---
status: accepted
---

# 后端整体迁移到 Go Kratos

Agent Platform 将前端与后端分别放入顶层 `frontend` 和 `backend`，并把后端整体迁移到 Go 1.25 与 Go Kratos v3 的标准工程形态和运行模型。后端以全局 `biz`、`data`、`service`、`server`、`conf` 分层为第一目录轴，各层内部继续按限界上下文分包；`cmd/api` 与 `cmd/worker` 保持两个独立 Kratos App，一次性 Conformance 命令仍为普通 CLI。

迁移保留 GORM、不可变 SQL Migration 和现有业务不变量，不因官方模板使用 Ent 而替换持久化技术。Kratos Config 只负责配置 Source、Load 和 Scan，项目继续使用严格 YAML 解码、单次环境变量展开和显式 API/Worker 校验；未知字段、多个 YAML Document、缺失环境变量和非法安全配置必须拒绝启动，且不启用配置热更新或 Environment Source 隐式覆盖。仓库根目录继续拥有跨端的文档、部署、Fixture、脚本与统一门禁；前端成为独立 PNPM 工程，后端成为独立 Go Module。第一阶段临时使用 `module agent-platform/backend`，正式企业 Git Host/Organization 路径必须在迁移完成前确定并机械替换 Module Path 与 Proto `go_package` 前缀。

Kratos App 通过 Wire 注入统一结构化 Logger、Recovery、Request ID、Tracing、Metrics、Access Log 和 Authentication。开发环境允许 Tracing/Metric Provider 为 No-op；生产是否允许 No-op 在 Exporter 选型后另行决定。日志和 Metric Label 必须遵守 Secret 脱敏与低基数约束。

本决策取代 ADR-0003 中“以限界上下文作为第一目录轴，并在每个上下文内设置 Domain/Application/Infrastructure 目录”的物理分层，但不取消限界上下文对聚合、数据和业务不变量的所有权。
