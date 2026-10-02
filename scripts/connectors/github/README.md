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
  --config /opt/agent-platform/config/platform.env \
  --package <package-path> --source <source-zip-path> \
  --evidence-directory <evidence-directory>
```

正式发布前检查 Administrator 和 User catalog 各只有一个正式 GitHub 条目，图标实际显示，已安装未授权时可打开 GitHub 设备页面并显示验证码。平台发布不替任何 User 安装或授权；Conformance、模拟协议、真实登录入口和真实账号 API 必须分别报告。
