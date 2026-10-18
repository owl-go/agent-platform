---
status: accepted
---

# 分离产品控制镜像与 Runtime 镜像

2026-10-18 用户要求 Docker Hub 的 `agent-platform` 只表达产品后端服务与前端入口，并通过 Docker Compose 部署；五个 Runtime Engine 改由独立的 `agent-platform-runtime` 镜像承载。本决策取代 ADR-0047 中“控制服务与 Runtime 复用同一镜像”的部分，保留一个产品控制容器内共同运行 Web、API、Worker、Keycloak、PostgreSQL 与 MinIO 的便利部署边界。

`haozhi152/agent-platform` 不安装 Claude Code、Codex、Hermes、OpenClaw 或 PI Agent。`haozhi152/agent-platform-runtime` 继续复用既有统一 Runtime Dockerfile、非 root 用户、公共 Entrypoint 与固定版本。Compose 用一次性只读、无网络服务确保 Runtime 镜像已拉取，再启动控制服务；控制服务只解析该独立镜像的本地不可变 Image ID，生产仍要求精确 Runtime RepoDigest。任务容器继续不挂载 Docker Socket、控制服务进程或完整数据卷。

两个镜像分别构建和发布 ARM64/AMD64。只有两边的两个架构版本都成功后，发布流程才更新各自的 `latest`、多架构版本 Tag 与架构别名。产品镜像和 Runtime 镜像拥有独立 Digest；Conformance 只绑定 Runtime Digest，不能用产品镜像 Digest 代替。

这种拆分减少产品更新重复传输大型 Runtime 层，也让 Docker Hub 的职责和部署文档更清楚；代价是 Compose 必须协调两个镜像，并在更新与备份说明中分别记录它们的版本。
