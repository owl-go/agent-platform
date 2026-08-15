# P0-05 接入 Hermes Agent 与 OpenClaw

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

