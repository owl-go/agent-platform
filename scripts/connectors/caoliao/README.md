# 草料二维码 Connector Package

本包接入[草料公开 MCP](https://cli.im/open-api/agent/public-mcp/quick-start.html)，修订 `caoliao@1.0.0`，Streamable HTTP 地址为 `https://mcp.objqr.com/mcp`。采用游客身份与 `auth_mode: none`，Egress 只含该 MCP 域名。包内包含元信息、品牌图标、一个 MCP manifest 和配套 Skill。

支持[官方五个只读工具](https://cli.im/open-api/agent/public-mcp/tools.html)：实体详情、记录、操作清单、表单字段、绑定查询。本修订不生成二维码、不提交记录或修改数据。填写入口使用服务返回的 `humanUrl`；缺失字段、过渡标识和绑定历史的解释见包内 Skill。

[公开查询范围](https://cli.im/open-api/agent/public-mcp/data-scope.html)由游客可见内容决定，结果可能缓存 120 秒。官方尚未开放公开 MCP JWT 获取流程；本包没有凭证表单，开放平台 API Key 不适用于这个服务。

品牌图标来自[草料官网引用的 PNG](https://static.clewm.net/cli/images/cli_logo_new.png)，原始 SHA-256 为 `bf9f2469b5cfb64da88c8b94d1f7a3ad43b3012b052ab186245a33e69ef17147`。SVG 的方形 viewport 显示原图左侧品牌符号，图片内嵌，无外部加载。`backend/internal/connectorpackage/icons/caoliao.svg` 是包和平台目录共用的资产源。

## 构建与验证

从仓库根目录执行：

```bash
python3 scripts/connectors/caoliao/build.py --output outputs/connectors/caoliao-1.0.0.zip
go -C backend run ./cmd/connector-package-validate "$(pwd)/outputs/connectors/caoliao-1.0.0.zip"
go -C backend test ./internal/connectorpackage/...
python3 scripts/connectors/caoliao/smoke.py
make test
make build
```

构建器离线读取受版本控制的文件，固定 ZIP 时间戳和权限。最终包必须通过 `connectorpackage.Parse`；发布使用其规范化 SHA-256，不能替换成原始 ZIP SHA-256。Smoke 使用官方公开演示码，检查握手、工具发现、只读契约及五次调用，只输出检查摘要。工具集变化即失败，需重新审阅；本包的 MCP manifest 不冻结远端工具集，Skill 不能扩展权限策略。

## 发布

先提交、推送并集成到 `main_temp`，执行适用门禁，再从集成源码构建包。目录品牌投影需要对应 API 版本。部署主机使用现有受保护配置进行管理员 OIDC + PKCE 登录，不复制凭证到本地仓库。

```bash
python3 scripts/connectors/caoliao/publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package /tmp/caoliao-1.0.0.zip \
  --normalized-sha256 '<connectorpackage.Parse 输出的 SHA-256>' \
  --evidence-directory /tmp/caoliao-publication-evidence
```

发布器先检查已有修订，幂等暂存／发布，并为登录的管理员账号安装或升级作验收。核对 User 与 Administrator 目录各只有一个正式条目、包摘要匹配、品牌投影以及无凭证 Installation 可用状态。其他 User 自行安装。重跑应复用同一 Revision 和 Installation，完成后清理临时远端包与脚本。

## 当前证据

2026-10-02 本地与部署主机均执行了官方服务握手、五工具发现和公开演示码的五次只读调用，均成功，服务报告 `im.cli/qrcode@0.1.0`、协议 `2025-06-18`。最终 ZIP Parse、品牌资产目标测试及功能分支／集成后的 `make test`、`make build` 已通过；两次构建的 ZIP 逐字节一致。

## 线上发布证据（2026-10-02）

从 `main_temp` 集成提交 `42d587ac447bbd9a5dcd9b46cddaf0e0a310b6f8` 构建并发布到 [Agent Workspace](https://47-237-108-63.sslip.io/resources?tab=connectors)：

- Publication 为 `available`，版本 `1.0.0`。
- Revision 为 `65be998a-a7ae-4767-aa70-06425225271c`，规范化包 SHA-256 为 `019e3d30ab19bca4fa3983de1214b965e9bd2ed11249123df539ad613ed1d0f1`。
- 管理员验收 Installation 为 `0151368f-b103-4361-aaeb-9c3807ae6d70`，状态 `active`、`authorized: true`；`auth_mode: none` 的该状态表示无须凭证，不代表取得了第三方账号授权。
- 重跑发布器复用了同一 Revision 与 Installation。User／Administrator 正式目录和该账号 Installation 各只有一个草料条目，没有创建 legacy staging Definition。

品牌投影通过 API 镜像 `agent-platform-api:caoliao-42d587a` 发布。容器健康检查及公网 `/api/healthz`、`/api/readyz` 均通过；未修改数据库 Migration、Worker、Web 或 Runtime 镜像。回滚 API 镜像保留为 `agent-platform-api:before-caoliao-42d587a`。

Playwright 实际查看线上 User 目录：只有一个“草料二维码”卡片，SVG 品牌图标加载成功；详情显示“已连接”、版本 `1.0.0` 和“安装包校验通过”，无凭证输入。卡片截图保存在忽略目录 `output/playwright/caoliao`，非敏感 API 证据在 `outputs/caoliao/evidence`。临时远端 ZIP、源码 tar、构建日志和证据已清理。

未运行目标模型 Runtime 的真实 Session／Run、生产 Sandbox Egress 验收或完整 Production Conformance，也未验证非公开码／账号数据。部署主机 MCP smoke 不替代 Runtime 容器与 Egress 证据。未改 Web 和 Runtime 镜像，相应构建门禁不适用；全量 Go 命令通过不代表依赖外部环境的集成测试均执行。
