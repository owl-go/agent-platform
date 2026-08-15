# P0-04 接入 Claude Code 与 Codex CLI

状态：实现完成，等待镜像构建与真实模型 Conformance

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

## 当前证据

- Claude Code `2.1.233` 与 Codex CLI `0.147.0` 分别拥有独立 Dockerfile、非 root EntryPoint 和固定版本测试。
- 两个 Driver 通过共享 `cliadapter.Adapter` 使用 Process Harness，并通过同一 Runtime Contract Test。
- Claude 解析 stream-json、Session 与 Usage；Codex 解析 JSONL、Thread、命令、文件变化与 Usage；新会话和 Resume 参数均有单元测试。
- Capability 默认全部关闭，只能由特定镜像 Digest 的 Production Conformance 显式打开。
- 当前 Docker daemon 未运行，环境中也没有模型 API Key，因此尚无镜像 Digest、真实代码修改、Interrupt、重建或 Review Branch Push 证据；完成前本 Ticket 不标记“已完成”。
