# Phase 0 Tickets

目标：用真实 PoC 证明 Runtime Adapter、Docker + gVisor、安全凭证、Workspace 恢复和对象存储 Provider 可行，再进入平台功能开发。

## 依赖图

```text
P0-01 Runtime contract
  +-> P0-02 Adapter harness
  +-> P0-03 Sandbox runner
  +-> P0-06 Credentials and redaction

P0-02 + P0-03 + P0-06
  +-> P0-04 Claude Code and Codex
  +-> P0-05 Hermes and OpenClaw

P0-07 Object storage providers

P0-04 + P0-05 + P0-07
  -> P0-08 Production conformance
      -> P0-09 Phase 0 decision report
```

按编号只表示稳定引用，不表示所有 Ticket 串行执行。没有依赖边的 Ticket 可以并行。
