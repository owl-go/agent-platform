# Agent Workspace

个人 AI 工作台：在私有对话中提问，把重复任务保存为工作流，复用 Expert、Skill 和 Connector。管理员集中配置账号、模型与 Credits。

## 本地运行

本地开发实例需要 Docker Desktop、Node.js 24+、pnpm 11、Go 1.25+（较旧 Go 可通过 `GOTOOLCHAIN=auto` 自动取得项目工具链）、Python 3.9+ 和 Make。启动 Docker Desktop 后运行：

```bash
make local-up
make local-credentials
```

浏览器打开 `http://127.0.0.1:14173`，使用第二条命令显示的本地管理员账号登录。PostgreSQL 和 Keycloak 数据会保留在本机 Docker volumes 中；固定版本构建的开源 MinIO、随机凭证、配置、日志、对象数据和 Workspace 保存在 `~/.local/share/agent-workspace`，不会写入仓库。常用管理命令：

```bash
make local-status
make local-down
```

`local-down` 只停止服务，保留本地数据。macOS Docker Desktop 默认没有生产要求的 `runsc` Runtime，因此本地入口会关闭全部 Runtime Engine：账号、资源和管理界面可在本地使用，但 Session / Workflow 的模型执行不可用。不要把这套本地开发实例当成 Production Conformance 或线上部署证据。

## 服务器部署

首次安装，在项目目录运行：

```bash
make install
```

填写服务器、域名和管理员邮箱，脚本自动安装依赖、生成配置并启动服务。见[首次安装](deploy/platform/installation.md)。

已有服务器安装，首次只需保存服务器地址和安装目录：

```bash
make deploy-setup
```

以后更新只运行：

```bash
make deploy
```

自动读取服务器配置、执行检查、备份、更新和验证。前端只构建一次；没有应用改动就不重启。日常更新保留 Runtime 镜像与基础设施配置。

发布使用已集成的 `origin/main_temp`，不会上传本地未提交文件。首次接管已有安装会重新构建 API / Worker，以确认运行版本一致。

查看[部署说明](deploy/platform/README.md)。

## 资源目录

连接器放在 [`connectors/`](connectors/README.md)，Skill 放在 [`skills/`](skills/README.md)，专家放在 [`experts/`](experts/README.md)。首次安装与更新部署都会自动加载这些目录；不需要逐个上传。已有条目复用稳定标识，账号授权仍由使用者配置。

已同步线上 20 个连接器、8 个专家和 9 个业务 Skill；另外 3 个内置创建 Skill 保留在后端代码中。完整文件、专家配置和绑定的[线上核对记录](docs/evidence/agent-workspace/2026-10-06-online-resource-sync.md)包含资源清单与摘要。

修改后可运行 `make resources-check` 检查目录内容；提交并集成到 `main_temp` 后，继续使用 `make install` 或 `make deploy`。

## 开发

需要 Go 1.25、Node.js 24、pnpm 11、Python 3.9+ 和 Make。

```bash
pnpm --dir frontend install --frozen-lockfile
make test
make web-typecheck
make web-build
```

前端开发：复制 `frontend/.env.example` 为 `frontend/.env.local`，填写 OIDC 配置，运行 `pnpm --dir frontend dev`。

## 进一步了解

- [产品规格](docs/product/agent-workspace-requirements.md)与[AI Applications](docs/product/ai-applications-requirements.md)
- [领域术语](CONTEXT.md)与[服务架构](docs/technical/service-architecture.md)
- [运维参考](deploy/platform/operations.md)：恢复、基础设施升级和可选功能配置
- [生产验收](docs/technical/production-conformance.md)：Runtime 与执行环境的验证边界

## 交流

扫码加入「Ai Agent 实验室」微信群：

<img src="docs/assets/ai-agent-lab-wechat-group.jpg" alt="Ai Agent 实验室微信群二维码" width="300">

二维码有效至 2026 年 10 月 13 日。
