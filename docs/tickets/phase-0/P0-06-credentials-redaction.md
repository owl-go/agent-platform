# P0-06 实现临时凭证与 Secret Redaction

## 目标

向 Runtime 提供模型 Key、Git SSH Key 和构建凭证，同时证明它们不会进入持久数据。

## 工作

- 定义 EnvironmentRef 与临时凭证目录。
- 以最小文件权限或环境变量注入所选 Runtime 凭证。
- 实现 stdout/stderr、Event、Diff、Artifact 和 Snapshot 脱敏。
- Run 结束和 Container 销毁时清理凭证。

## 验收

- 测试 Secret 出现在命令、错误、文件和二进制输出中的场景。
- 所有持久结果只包含脱敏值。
- 非选中 Runtime、其他 Run 和宿主进程无法读取凭证。
- 清理操作幂等。

## 依赖

P0-01；与 P0-02、P0-03 并行协作。

