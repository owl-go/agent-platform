---
status: accepted
---

# 使用 Wire 进行编译期依赖注入

Agent Platform 后端使用 Wire 组织 Kratos Provider Set 并生成编译期依赖装配代码，以保持依赖图显式且贴近官方 Kratos Layout。Wire 只装配 API 与 Worker 两个 Kratos App；各限界上下文分别暴露 Provider Set，再由进程级 Injector 选择性聚合，禁止以一个全局 Provider Set 隐藏不必要的跨上下文依赖。

虽然 Google Wire 已归档，项目仍接受这一维护风险并固定 `github.com/google/wire/cmd/wire@v0.6.0`。生成的 `wire_gen.go` 提交到 Git，CI 重新生成并拒绝未同步的 Diff；普通构建和生产镜像不现场执行 Wire。若未来 Go 版本导致 Wire 无法生成，必须通过新的 ADR 决定替换工具或维护 Fork。
