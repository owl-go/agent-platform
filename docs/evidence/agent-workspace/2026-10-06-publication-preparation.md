# 2026-10-06：发布准备、默认资源与公开副本检查

本次完成的是可审阅的代码/文档与公众号草稿准备，没有将仓库设为 public，没有重写历史，没有发布公众号文章，也没有从功能分支部署线上环境。

## 默认资源与安装文档

当前平台资源通过只读方式导出完整归档，只白名单保存定义：8 个 Expert、9 个附加 Skill、20 个 Connector Package；三个创建 Skill 复用仓库原版，共 12 个 Skill。没有导出或分发 User Installation、Authorization、API Key、Model Provider Connection 或原部署 Conformance 行。20 个包为 9 MCP / 11 CLI，11 个 CLI 在新安装保持 disabled。

本次在一次性 Docker PostgreSQL 17（仅 loopback、无生产数据）实际执行完整 Migration 链与默认资源集成测试，覆盖全量首次安装、四路并发/重复启动、原版版本升级与稳定绑定、同版本内容变更拒绝、同名管理员资源不接管、私人资源不覆盖、私人 Connector 版本冲突不提升为平台发布、对象存储失败回滚及重试。默认资源结构和完整 ZIP/CLI Bundle 通过公共安全校验；目录与发布接口不会将完整 manifest 当成当前安装的真实 CLI Conformance。

已执行：

```text
go -C backend test ./internal/defaultresources ./internal/connectorpackage ./internal/skillstore ./internal/data/workspace/gormrepo ./internal/service/workspace ./internal/wiring/agentworkspace ./internal/wiring/workspaceworker
make test
make build
bash -n scripts/deploy-platform.sh scripts/deploy-web.sh scripts/acceptance/agent-workspace-deployment.sh
git diff --check
cmp -s AGENTS.md CLAUDE.md
```

上述 Repository/全量测试配置了 `WORKSPACE_TEST_POSTGRES_DSN`，不将未配置 PostgreSQL 的 Skip 计作数据库验证。MinIO/OSS、真实供应商授权、Linux + gVisor、Runtime Digest 黑盒验收和全新服务器安装未在本次执行；README 容量配置是估算，不是已压测最低值。

## 公开副本与历史边界

当前文件中的实际部署地址、机器路径与部署 SSH 默认值改为通用示例，保留产品/技术文档结构及历史证据的日期、结果、版本和验证边界。Runtime 内固定 `/opt/agent-platform/connectors` 协议路径、官方供应商 Endpoint、项目 GitHub 链接、localhost 和文档保留地址不当作私有部署信息删除。

对 1,102 个可达 Commit / 6,711 个历史 Blob 的只读检查发现，实际部署域名仍存在于 108 个历史 Blob、涉及 42 条文件路径；当前文件清理无法去除旧 Commit。仓库继续 private，任何公开动作均须先完成获授权的历史处理和复查。

Gitleaks v8.30.1 以 `--redact=100` 扫描可达历史，四个候选均为测试假值或文档镜像标识；当前文件扫描同时开启两层归档扫描，十个候选均为已有测试假值/镜像标识或官方飞书参考文档中的文档、Base 与分页标识。完整新资源 ZIP 及嵌套 CLI Bundle 同时检查已知私有部署值，未命中。这里只记录该扫描范围与人工判定，不能推导为对所有潜在秘密的绝对保证。含原始值的审计结果和导出过程数据只保存在仓库外，不提交。

## 公众号草稿

同一篇《我为什么做一个企业内部的 Agent 工作台》草稿保留九张真实界面图与两个操作 GIF。包含任务中部署地址的会话图与资源菜单动画换为避开敏感区域的真实局部；最终微信预览的 11 个素材全部加载，两处 GIF 实际播放，首页封面保持安全。九图及两 GIF 的八个源帧经过已知地址 OCR 和逐张视觉复核。没有声称添加并未实际添加的马赛克，也没有发布或群发。
