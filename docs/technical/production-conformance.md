# Production Conformance

状态：部署验收合同

发布前必须在目标 Linux Worker 上使用 Docker + gVisor `runsc` 验证 Agent Workspace 完整闭环，macOS 单元测试不能替代该证据。

最低验收矩阵：

1. Keycloak 登录、管理员创建/停用/启用账号、临时密码重置。
2. Session 新建、流式回复、失败重试、归档、取消归档、删除以及同 Runtime 原生 Resume/切换 Runtime 平台恢复。
3. Workflow 新建、编辑、手动运行、定时运行、API Credential 运行、幂等重放、取消和失败重跑。
4. Workspace 浏览、上传、下载、建目录、公共 HTTPS/私有 SSH Git Clone、1 GiB 限制和成功合并。
5. Run 历史、SSE 实时事件、终态、最终结果与 Artifact 下载。
6. Expert 可见结构化指导、Derived Expertise Tags、Team Member 稳定身份与角色注入、Personal Settings 单一执行配置冻结；Git/ZIP Skill 安装；HTTPS/stdio MCP Connector 隔离测试；适用 Runtime 读取对应 MCP 配置。
7. Administrator-only CLI Connector Definition、隔离 bundle 构建、完整性与 schema review、User-private enablement/authorization，以及公共 Wrapper 的 argv、identity、scope、Egress、Workspace、timeout、output 和 Secret 边界。
8. Feishu CLI 对一个 User 只创建一个应用、多个 account authorization、User/Bot 身份、最小权限与恢复链接、Token 刷新/撤销；高风险命令批准、拒绝、超时、重复请求、`waiting_for_user`、取消、重启恢复和执行前重校验。
9. Claude Code、Codex、Hermes、OpenClaw、PI Agent 各自使用固定 RepoDigest 进行真实模型调用，并验证取消、超时、输出边界和 Secret canary 不泄漏。每个 CLI Connector 可用性声明同时记录 exact bundle SHA-256 和 Runtime RepoDigest 的联合证据。
10. MinIO 完整上传/回读/签名下载；若生产选择阿里云 OSS，则额外执行真实 OSS Conformance。
11. 中文/英文、桌面/移动端主要流程与刷新恢复。
12. 数据库备份可恢复、增量 Expert/Connector Migration、API/Worker 健康检查和重启恢复。

只有实际执行且保留日志、镜像 Digest、时间与环境信息的检查才能声明通过。任何 Runtime 未通过时，其 `/api/v1/runtime-engines` 状态必须为不可用，前端不得允许新配置选择它。
