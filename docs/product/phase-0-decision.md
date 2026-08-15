# Phase 0 决策报告

日期：2026-08-15  
决策：**NO-GO — 暂不进入 Phase 1**

## 决策依据

Phase 0 的代码实现和本地门禁已经形成，但 P0-08 要求的 Linux + gVisor、四种真实模型执行、Git SSH Push 和阿里云 OSS 证据尚未产生。Phase 1 不得依赖“本地 Docker 可运行”等同于“生产隔离和 Runtime 可用”的假设。

本地证据见 `docs/evidence/phase-0/2026-08-15-local-conformance.md`。

## Runtime 状态

| Runtime | 状态 | 已证实 | 未证实 |
|---|---|---|---|
| Claude Code CLI | Experimental | 固定版本镜像、非 root、Adapter 契约、输出解析 | 真实模型任务、gVisor、Push、恢复、Usage |
| Codex CLI | Experimental | 固定版本镜像、非 root、Adapter 契约、JSONL 解析 | 真实模型任务、gVisor、Push、恢复、Usage |
| Hermes Agent | Experimental | 固定版本镜像、非 root、一次性安全模式、Usage 文件解析 | 真实模型任务、gVisor、Push、平台重建 |
| OpenClaw | Experimental | 固定版本镜像、非 root、原生 Agent 模式、禁止嵌套 Claude/Codex | 真实模型任务、gVisor、Push、平台重建 |

四个 Runtime 的可选 Capability 当前全部关闭。只有指定 Registry RepoDigest 通过对应黑盒场景后，才能逐项启用 `streaming`、`native_resume`、`structured_final`、`subagents` 或 `usage`。

## 已接受限制

- MVP 使用一个 Run 内的 Runtime 原生 Subagent，不建设平台级多 Agent、Child Run 或委派图。
- 平台恢复依赖 Session Memory 与 Workspace Snapshot；原生 Runtime Resume 只是可选优化。
- OpenClaw 只使用原生 Agent 模式，不在 Run 内再次代理 Claude Code 或 Codex CLI。
- Git 只使用配置的仓库地址、SSH 私钥、固定 `known_hosts`、Commit 和 Push；不集成 GitHub/GitLab API，也不自动创建 PR/MR。
- 模型由管理员配置后均可用于仓库；MVP 不验证模型供应商合规，也不建设 Model Gateway。
- 对象存储部署时选择阿里云 OSS 或 MinIO，上层只依赖统一 Provider 契约。

## 阻断项与 Owner

| 阻断项 | Owner | 解除条件 |
|---|---|---|
| Linux + `runsc` Worker 与公网 Egress Policy | Platform Engineering | `scripts/conformance/sandbox-linux.sh` 对 Registry Digest 通过，并保存网络拒绝证据 |
| 四种 Runtime 的真实模型任务 | Runtime Integration | 每个 Runtime 完成修改、测试、取消、强杀重建、脱敏和 Push 场景 |
| 临时 GitHub.com/自建 GitLab 仓库与 SSH 凭证 | Repository Administrator | Clone/Commit/Push Review Branch 通过，目标分支未被直接修改 |
| 阿里云 OSS 测试环境 | Storage Operations | OSS Provider 完整 Conformance、私有 Bucket 与 URL 到期通过 |
| Registry RepoDigest 与证据归档 | Release Engineering | 四镜像 Push 后以 RepoDigest 重跑并归档报告、耗时和失败 Artifact |

## Go 条件

只有以下条件全部满足，才能把本报告改为 `GO` 并开始 Phase 1：

1. P0-08 的四个 Runtime、gVisor、Git、凭证与 OSS/MinIO 场景全部通过；
2. 每个 Runtime 绑定不可变 Registry RepoDigest，并有独立状态和 Capability 证据；
3. 所有失败均已有修复、明确范围调整或书面接受，不留未归属阻断项；
4. Platform Administrator、Agent Builder、Agent User 和 Run Operator 的 Phase 0 关键路径没有被限制破坏。

在此之前，P0-01 至 P0-07 的实现可继续维护和复测，但不能被视为 Production Runtime 承诺。
