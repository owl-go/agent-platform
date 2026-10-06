# OpenBoost 连接器发布证据 — 2026-10-02

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 发布结果

- 功能提交：`52db842`，分支 `codex/openboost-connector`；集成提交：`f290f5b51f1b8de08389594aceb5a0b80abd68da`，已推送 `main_temp`。
- 正式部署：`openboost-20261002`，使用完整 `make deploy`，未跳过门禁。源目录 `/srv/agent-workspace/src.release-openboost-20261002`；Web 目录 `/srv/agent-workspace/web/releases/openboost-20261002`；备份 `/srv/agent-workspace/backups/pre-openboost-20261002`。
- [平台目录](https://workspace.example.com/resources?tab=connectors)中 `openboost` 0.1.0 为 `available`，CLI 模式、User 身份、`connector_package` driver。
- Revision：`75afb574-5872-4d5a-af39-9a9c6df23a18`。
- Package SHA-256（实际 Parse 规范化）：`9d73b98a675df28e2b3050567e8f732dce81d3a94ec3205f560b6d89021d83af`。
- Bundle SHA-256：`7e0a61f9aee4625ff93e32820def36339be4f0b40bf8ed03063ba1c1a2abb3d9`。
- Runtime：`127.0.0.1:5000/agent-platform/runtime@sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b`；部署复用相同 Digest。
- 发布验收安装：`47a2cd49-eb86-4f89-91b1-13de50765d85`，所有者是发布操作使用的 Platform Administrator；Revision 匹配，状态 active、未授权。未给其他 User 创建安装，也未写入任何测试 Secret Key。

## 上游与能力证据

[官方 MCP 配置](https://open.microdata-inc.com/mcp-list)及公开页面配置生成器确认 HTTPS Streamable HTTP 和 `secret-key` Header。另找到[官方 CLI 页面](https://open.microdata-inc.com/cli)与 [openboost-cli 2.0.2](https://pypi.org/project/openboost-cli/2.0.2/)。本修订是仓库维护的 MCP CLI 桥接器，沿用现有平台的逐命令策略与凭证 seam，不将它称为 PyPI 官方 CLI。

真实访问统一端点 `https://mcp.microdata-inc.com/mcp-servers/openboost-all-mcp`：initialize 返回协议 `2025-03-26`、服务 `proboost-tiktok-amazon-patent-mcp` 1.0.0；初始化通知返回 202；公开 tools/list 返回 101 个带 Schema 的工具。无凭证的 account_info 与 account_links 调用均返回 HTTP 200、MCP isError=true，错误明确为缺少 Authorization 或 secret-key。未使用密钥或触发购买。

工具快照保存在 `scripts/connectors/openboost/tools.json`。本修订静态允许其中 100 个工具并附带 tools/schema/version 三项诊断能力；排除创建支付订单 account_order_create。覆盖 Amazon 36 项，TikTok 的 tt 30 项与 tk 1 项，专利 17 项，Instagram 6 项、YouTube 3 项、Reddit 2 项，以及剩余账号查询 5 项。公开发现不自动扩展能力，也不证明具体账号拥有相应服务权益。

## 已执行验证

- 本地 19 项 Node 协议测试、5 项 Python 构建测试、4 项共享发布生命周期测试通过；Python 构建测试和交付命令均调用当前仓库的实际 `connectorpackage.Parse` 校验最终 ZIP。
- `go -C backend test ./internal/connectorpackage/... ./internal/cliconnector/...`、完整 `make test` 和 `make build` 通过。
- 功能分支相关前端 76 项测试通过；合并 main_temp 后完整前端 46 文件 / 447 用例通过，`make web-typecheck`、`make web-build` 和 `git diff --check` 通过。正式部署再次执行完整门禁。
- Linux + runsc 目标 Runtime 在无外网、非 root 1000:1000、只读 Rootfs、1 CPU / 512 MiB / 64 进程限制下通过全部 19 项协议测试。Node 测试使用 `--test-isolation=none`，避免该镜像禁止的嵌套 Node 子进程；未声明新增 Runtime Capability。
- Worker 从同一 source ZIP 实际隔离构建，得到相同 bundle SHA-256，对该 bundle × Runtime RepoDigest 运行平台 Conformance 并记录通过证据。暂存与发布返回相同 Revision 和 `conformance_available:true`。
- 临时 Definition `0bdaeeb1-5199-4df1-8d22-34e52b29137c` 无 User 使用关系，发布器通过管理员 DELETE API 软删除。Administrator 与 User catalog 各一个正式 OpenBoost 条目；本次临时构建条目为零。发布重跑复用同一 Revision，没有新建 Definition。
- 发布后的 API healthz 为 ok、readyz 为 ready；正式部署的 API、Worker、Egress Controller、Caddy 健康检查、运行镜像 smoke、备份校验及 Web 发布检查通过。

## 浏览器与连接验收

使用 Playwright CLI 实际打开线上目录，在发布管理员自己的 User 视图安装 OpenBoost，查看详情并点击连接。卡片图标为 `/assets/openboost-BYKA0Gsa.svg`，实际 naturalWidth 80，与包内和前端的官方品牌 SVG 一致；Installation 与 Revision icon 均为 openboost。

连接表单恰有一个 `name=openboost_secret_key`、type=password 的输入和官方配置链接。取消后重新打开仍为空。桌面 1200px 页面无横向溢出；移动端 390×844 的 documentWidth 为 390，表单左右边界为 20 / 370。截图已人工查看。提交、失败重试、取消清空以及旧修订升级由真实组件 fixture 覆盖；线上没有提交 Secret Key。

浏览器使用短期平台登录状态进行验收；该状态到期时发生了既有 OIDC iframe 的 X-Frame-Options 拒绝并退出登录，不作为 OpenBoost 授权或刷新验收。实际 OpenBoost 认证选择官方密钥模式，本修订没有提供或宣称浏览器 OAuth。

## 验证边界与产物

真实 OpenBoost 账号授权、业务数据、套餐权益与额度扣减未验证。工具发现及 fixture 路由验证不等于 100 个业务 API 已在真实账号成功。未运行完整模型 Production Conformance、完整 Sandbox Conformance 或真实远端存储 Conformance；本次未改变其契约，缺少环境的集成 Skip 不计为实际远端通过。

本地包与非敏感发布 API 证据位于被忽略的 `outputs/openboost`，浏览器截图位于集成工作区的 `output/playwright/openboost`。临时凭证状态与远端验证源码/构建包在验收后清理；永久部署与发布证据保留，未删除任何已有资源或历史 Conformance。
