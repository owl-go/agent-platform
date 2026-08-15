# P0-04 接入 Claude Code 与 Codex CLI

## 目标

交付两个独立 Runtime 镜像和 Adapter，并证明成熟无头接口可映射到平台 Contract。

## 工作

- 固定 CLI 版本和镜像 Digest。
- 实现非交互启动、结构化事件、Usage 和原生 Resume 映射。
- 配置模型凭证、工作目录和 Runtime 状态目录。
- 验证修改、测试、Interrupt、重建和 Review Branch Push。

## 验收

- 两个 Adapter 分别通过共同 Contract Test。
- Capability 只声明实际验证的能力。
- Runtime 不能在执行期间升级自身。
- 凭证不会进入日志、Diff、Artifact 或 Snapshot。

## 依赖

P0-02、P0-03、P0-06 的 Credential 接口。

