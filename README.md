# Agent Workspace

个人 AI 工作台：在私有对话中提问，把重复任务保存为工作流，复用 Expert、Skill 和 Connector。管理员集中配置账号、模型与 Credits。

## 部署

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
