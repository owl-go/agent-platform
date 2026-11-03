---
status: accepted
---

# 默认启用引擎与自动开放内置连接器

2026-11-03 用户明确要求当前安装及后续部署默认启用所有引擎与内置连接器。整合部署的五个 Runtime Engine 默认 available；`ENABLED_RUNTIME_ENGINES=none` 或逗号分隔名单可显式收紧。available 表示允许选择已安装的引擎，不是该引擎与任意 Provider Model 已完成真实调用或完整 Production Conformance 的证明。协议不兼容仍明确拒绝，Native Resume 等可选 Capability 继续默认关闭。本决策取代部署说明和 Production Conformance 中以未完成整套验收为由统一关闭引擎选择的要求，不改变 runsc、RepoDigest、非 root、只读 Rootfs、资源限制、挂载、Egress、Secret 或审批边界。

内置 CLI 包启动时自动完成当前 bundle SHA-256 × 配置 Runtime RepoDigest × Worker 架构的无凭据、无网络启动检查，通过后自动恢复保留自动启用意图的初始或受管升级目录条目；失败继续不可用并重试。独立生产证据不读取本地 runc 表，也不伪造旧包声明 Digest 的验证记录。API 将受管包的验证目标投影为本安装的固定 Runtime，包、Revision 和历史快照保持不变。管理员明确停用、自建包、User 安装与授权均不覆盖；默认开放不是自动获取外部账号授权，也不是业务 API 成功证明。
