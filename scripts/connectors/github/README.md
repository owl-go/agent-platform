# GitHub CLI Connector Package

固定官方 [cli/cli v2.102.0](https://github.com/cli/cli/releases/tag/v2.102.0)，Commit `fc4b137cdef0a6bd28fd461b7cf9c84a5812a8cd`。两个 Linux 发布资产的 SHA-256 固定在 build.py；MIT License 随 bundle 保存。品牌图标采用 Primer Octicons 的 mark-github-16（MIT）。不执行下载包的安装脚本。

2026-10-02 逐页读取 [官方手册](https://cli.github.com/manual/) 的全部 237 个站内导航页面，包括命令组、叶子、环境变量、格式化、退出码和完整 reference。正文及原 HTML 哈希保存在 `manual-v2.102.0.tar.gz`；官方网页内容是版本读取时的快照，命令和允许参数另与固定 2.102.0 原版 CLI `--help` 核对。覆盖表列出每一页面对应的 capability、User OAuth scopes、风险或未开放原因。该包开放 113 个远程业务前缀及两项帮助/版本能力；所有写前缀为 high，Broker 负责一次性命令批准。Scope、参数和命令均由静态 policy 决定，上游文档更新不会自动扩大权限。

浏览器授权使用固定 CLI 的 GitHub CLI OAuth App 公共 Client ID 和 GitHub 官方 Device Grant。平台展示一次性 user_code、链接官方 `/login/device`，加密保存 device_code 并绑定 owning User/Installation/Flow。该协议不使用 PKCE 或 localhost 回调。轮询间隔通过 encrypted state 的 owner-scoped compare-and-swap 保存，遵守 interval 和 slow_down；拒绝、到期或已交换后身份查询失败结束流程，成功通过 `/user` 解析不可变账号 ID 后一次性消费 Flow。无用户 Token 表单。凭证加密保存，原版 gh 仅在本次独立子进程收到 GH_TOKEN；HOME 和配置为清理的临时目录，不写系统钥匙串。默认非到期 grant；若上游返回到期凭证，到期后须重新连接，当前修订不申请 offline_access 或释放 refresh material。

查询/写入只面向 github.com。Wrapper 清空继承的认证、主机、代理、Git 配置、编辑器和插件环境；参数白名单拒绝浏览器、任意主机、Shell、插件、本地 Git、原始 api，以及会读取本地文件的参数。正文/发行说明的文件选项仅接受 stdin `-`。Release 创建不上传资产；Workflow 输入使用 raw-field 或 JSON stdin。首次连接请求已放行功能的 repo、read:org、gist、project、workflow；Projects 使用 project，触发 Workflow 使用 workflow。GitHub 仓库、组织和账号权限仍由服务端判断。

## 构建与检查

下载两个官方 `gh_2.102.0_linux_<arch>.tar.gz` 后执行：

```bash
python3 scripts/connectors/github/build.py \
  --assets-directory <absolute-assets-directory> \
  --runtime-image <registry/repository@sha256:digest> \
  --runtime-version <exact-node-version> \
  --output <absolute-output-directory>/github-0.1.0.zip
go -C backend run ./cmd/connector-package-validate <absolute-package-path>
node --test scripts/connectors/github/launcher.test.cjs
python3 -m unittest discover -s scripts/connectors/github -p 'test_*.py'
go -C backend test ./internal/githubcli/... ./internal/connectorpackage/... ./internal/service/workspace/... ./internal/cliconnector/...
```

同一脚本输出 .source.zip，用现有 ZIPPackageBuilder 构建不可变 bundle，平台 source upload 的产物应逐字节一致。构建器只标注 conformance: not_run。平台发布须从 main_temp 完成适用门禁和代码部署，再通过 publish.py 用管理员加密配置执行 source 上传、Worker Linux + runsc Conformance、Stage/Publish、临时 Definition 软删除和目录核验。再次运行复用 exact bundle × Runtime 的证据与已有 Revision，不撤销用户使用。

```bash
python3 scripts/connectors/github/publish.py \
  --config /srv/agent-workspace/config/platform.env \
  --package <package-path> --source <source-zip-path> \
  --evidence-directory <evidence-directory>
```

正式发布前检查 Administrator 和 User catalog 各只有一个正式 GitHub 条目，图标实际显示，已安装未授权时可打开 GitHub 设备页面并显示验证码。平台发布不替任何 User 安装或授权；Conformance、模拟协议、真实登录入口和真实账号 API 必须分别报告。

## 2026-10-02 发布证据

集成 `main_temp` 的 `6fffda0` 已部署为 `github-6fffda0`，正式 Revision `66cf9265-5d8c-48bb-9a0c-d8b5679b7448`（0.1.0）为 available。最终 ZIP 经当前 Parse 校验；规范化 package SHA-256 为 `ae22bb4fcf2b27b4bb18c4e3e545ec9bdd57247438cdd6187884609560da24b4`，bundle 为 `b98cf1c75a45b73c9c3b3254bee28da1cd54524586ccd981e1c4646bea13b073`。生产 Worker 对此 bundle × Runtime `sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b` 记录 Linux + runsc Conformance。两次发布返回同一 Revision；Administrator 与 User catalog 各一个正式条目，临时构建 Definition 为零。

同一隔离 Runtime 的原版 Linux amd64 CLI 版本为 2.102.0，全部 113 个业务前缀的 `--help` 可启动；wrapper 的未授权 status、未放行 api 拒绝和未授权业务拒绝均通过。完整 Go 测试／构建、集成分支 440 个前端测试、typecheck／生产构建、三个 Node 边界测试、三个包完整性测试、四个共享发布器测试和镜像 smoke 已执行通过。source 构建 bundle 与最终包逐字节一致。

线上 Playwright 验证 Administrator 作为 User 的 Installation `31279857-8743-4e07-a2bb-dda5a99a3d4b`：品牌图标实际显示、详情标记运行环境已验证、未授权连接入口显示一次性验证码并打开官方 GitHub 登录页。该 Installation 当前 active、未授权；没有保存测试 Token。会话选择器完整路径、真实 GitHub 账号授权／API、Linux arm64 执行和完整模型 Runtime Production Conformance 未验证。非敏感发布、目录和 Runtime 记录及截图保存在忽略目录 `outputs/github/evidence`；输出 ZIP 为 `outputs/github/github-0.1.0.zip`。
