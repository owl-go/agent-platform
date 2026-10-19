---
status: accepted
---

# 显式本地 CLI Connector 执行与验证

2026-10-19 用户要求恢复本地 runc 的飞书、钉钉、企业微信等 Connector，并继续使用生产 runsc。本决策取代 ADR-0046 中“本地 CLI Connector 执行保持关闭”的部分，不改变生产 Sandbox、隔离 Builder 或 Workflow Message Channel 边界。

显式 `sandbox.profile: local` 与 `runtime: runc` 在个人可信 Docker bridge 内运行短生命周期 Connector Container；仍使用非 root、只读 Rootfs、资源限制、单包只读挂载、平台 Broker、账号授权、Scope、一次性高风险审批、凭证脱敏和取消清理。不装配生产 Unix Egress Gate，也不挂载生产 Resolver。该模式没有 capability 域名的网络强制隔离保证，不适用于共享或生产主机；不能自动从生产降级。

Worker 对受管理目录的固定 CLI bundle 校验对象摘要后，在当前实际本地 Image ID、无网络、无用户凭证的容器中执行 `--help`。结果保存在独立 `local_cli_connector_conformance` 表，只证明可执行兼容性；API/Worker 均显式装配相同的本地目标镜像及构建架构。Docker Desktop 的 containerd 存储可能把多架构索引当作 Image ID，因此 Migration 000089 将 Worker 架构加入证据主键；旧未标注行不再用于验证，迁盘或换架构不能复用。生产查询继续只读取 exact bundle × Runtime RepoDigest 的原有证据，绝不读取本地表。镜像、架构或 bundle 变化必须重新验证。包内原始生产 Runtime 声明和不可变 Revision 不回写；本地 API 的验证目标投影为实际配置的镜像 ID。

通过全部当前镜像验证后，仅恢复 `default_resource_seeds` 管理、仍持有首次初始化恢复标记的 Publication。旧目录只为 `disabled/version=1` 回填标记，seed 修订升级保留，管理员明确发布或停用则清除；验证提交后重启也可继续恢复目录。已有管理员停用、自建包、安装、授权和历史快照不被覆盖。Publication 的目录启用不等于生产验证通过，生产前端和执行仍以生产证据为准。第三方账号授权和真实读写请求另行验收，不能由 `--help` 或测试 Fixture 冒充；隔离 npm Builder 继续关闭，现成无凭据包不需要在用户任务内安装软件。

本地模型容器、Warm Container、Connector Container 和兼容验证统一显式传入 `--platform linux/<Worker 架构>` 与 `--pull=never`，从不可变索引选择相同平台，不按 Docker 宿主默认架构偷偷替换，也不在任务中自动拉取缺失的平台。缺失镜像须先由部署入口准备；Warm 配置指纹包含本地架构。生产命令及其指纹不变。
