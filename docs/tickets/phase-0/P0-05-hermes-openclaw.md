# P0-05 接入 Hermes Agent 与 OpenClaw

状态：实现完成，等待镜像构建与真实模型 Conformance

## 目标

交付 Hermes Agent 和 OpenClaw 原生 Agent 的独立镜像与 Adapter，并明确其能力差异。

## 工作

- 固定 Hermes Agent 与 OpenClaw 版本。
- Hermes 使用非交互 one-shot 路径，验证其自动批准行为只能发生在 Sandbox 内。
- OpenClaw 只使用原生 Agent，不允许再次路由 Claude Code 或 Codex。
- 对无法稳定获得的流式、Usage 和原生 Resume 保持 Capability 关闭。

## 验收

- 两个 Adapter 均能完成示例代码修改、测试和 Push。
- Hermes 自动批准不能突破 Sandbox 与 Egress Policy。
- OpenClaw 不产生双重 Runtime Session 或嵌套 Runtime 路由。
- 能力矩阵与实际 Conformance 结果一致。

## 依赖

P0-02、P0-03、P0-06 的 Credential 接口。

## 当前证据

- Hermes Agent `0.19.0` 与 OpenClaw `2026.7.1-2` 分别拥有独立 Dockerfile、非 root EntryPoint 和固定版本测试。
- Hermes 固定 one-shot + safe-mode，自动批准只在外层 Sandbox 使用；最终文本与独立 Usage 文件均有解析测试，未验证 Resume 会被拒绝。
- OpenClaw 固定 `agent --local` 原生路径，并拒绝 Claude/Codex CLI backend Model ID；不启动 Gateway、不执行渠道交付，未验证 Resume/Streaming/Usage 保持关闭。
- 两个 Driver 与 Claude/Codex 通过同一 Runtime Contract Test，Capability 默认全部关闭。
- 当前 Docker daemon 未运行，环境中也没有模型 API Key，因此尚无真实修改、测试、Push、Sandbox 越权或双重 Session 的运行证据；完成前本 Ticket 不标记“已完成”。
