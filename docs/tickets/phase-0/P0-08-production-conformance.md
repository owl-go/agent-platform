# P0-08 运行 Production Conformance Suite

状态：本地门禁完成，Production Conformance 被外部环境阻断

## 目标

在真实 Linux + gVisor 环境对四种 Runtime 和两种对象存储实现执行可重复验收。

## 工作

- 建立包含测试、长命令、失败命令和已知 Secret 的示例仓库。
- 自动运行 Runtime 修改、测试、取消、强杀、重建和 Push 场景。
- 自动运行网络逃逸和凭证泄漏场景。
- 分别用阿里云 OSS 与 MinIO 保存 Artifact 和 Snapshot。

## 验收

- 四种 Runtime 均满足 Production Runtime 最低门禁。
- 每个已声明 Capability 都有通过的场景证据。
- 两种对象存储的产品行为一致。
- 报告包含镜像 Digest、CLI 版本、耗时和失败证据链接。

## 依赖

P0-04、P0-05、P0-06、P0-07。

## 当前证据

本地镜像和 MinIO 证据见 `docs/evidence/phase-0/2026-08-15-local-conformance.md`。Linux + gVisor、真实模型、Git SSH Push、Registry RepoDigest 和阿里云 OSS 尚未执行，因此本 Ticket 未完成。

Runtime Adapter 与 gVisor Container 的执行 seam、证据文件和统一 Fixture 已实现，见 `docs/technical/production-conformance.md`。这减少了外部环境中的人工步骤，但不替代真实执行结果。
