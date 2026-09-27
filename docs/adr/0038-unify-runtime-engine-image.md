---
status: accepted
---

# 五个 Runtime Engine 共用一个不可变镜像

五个 Runtime Engine 使用同一组隔离、Entrypoint、Git 和 Connector 客户端依赖。分别构建、推送与配置五个镜像增加部署维护和版本漂移风险，因此将五种固定版本 CLI 安装在一个不可变镜像中。部署只配置一个 `RUNTIME_IMAGE` RepoDigest；每个 Runtime Engine 仍单独声明版本、Capability、Native Resume 和 `available`。

Hermes 需要 Python 3.13，OpenClaw 使用 Node 24。统一镜像以固定 Digest 的 Python 3.13 基础镜像为运行层，从固定 Digest 的 Node 24 镜像复制 Node/npm 工具链。每次执行由平台通过公共 Entrypoint 显式传入冻结的 Runtime Engine 命令。Warm Container 身份和配置指纹继续包含 Runtime Engine 与镜像 Digest；共享镜像不允许不同引擎复用同一个 Warm Container。

单一镜像减少镜像数量，但更新任一 CLI 都会产生新的共享 Digest，并要求五个引擎重新取得该 Digest 的 Conformance 证据。每个引擎的可用性独立判定；CLI Connector bundle 与新 Digest 的组合也需重新验证。缺少证据时按现有 fail-closed 规则保持不可用，不从旧 Digest 继承声明。此决策取代 ADR-0019 中五个 Runtime Engine 分别使用独立镜像的部分。
