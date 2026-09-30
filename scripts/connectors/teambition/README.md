# 钉钉项目（Teambition）CLI Connector Package

固定上游 `@tng/teambition-mcp-cli@0.3.3`，使用 npm SHA-512 与两个 Linux 资产 SHA-256 校验，无安装脚本执行。官方 Skill 2.0.2 下载包单独固定 SHA-256；保留其业务编排及两份 TQL 参考。本包的来源是 `teambition`，与钉钉 DWS `dingtalk` 独立。

上游文档：[Teambition Skill 使用指南](https://open.teambition.com/docs/documents/6a561ee8662d4dfb252d0bb8)。公有云 CLI 连接 `https://open.teambition.com/api/mcp/v2`。CLI 动态发现帮助与命令；本文档不把服务端目录的变化直接转为授权策略。

## 包与授权

包内只开放 `capabilities.json` 的 17 项策略：官方文档/Skill 明确出现的项目、任务查询，文件链接查询，任务创建/移动，只读文档检索、工具目录/Schema 及帮助。任务创建/移动是高风险操作，包括其叶子 `--help`，需平台具体 target 和一次性批准。状态更新、评论、成员管理及其他动态操作尚未进入本修订；获取相应账号的最新工具目录并审阅后再发布新修订。

上游 OAuth + PKCE 适用于本地 CLI。平台目前没有 Teambition 交互式 OAuth driver；本修订使用内置 `connector_package` 的加密托管凭证入口，只接受 JSON 的 `user_token` 字段（填写该账号真实兼容 UserToken）。单次进程桥接到官方 `TEAMBITION_MCP_TOKEN`，CLI 的临时 HOME 与缓存执行后删除。Token 不放进参数、包或 Skill。平台安装与连接由各 User 独立完成；本构建和发布流程不替 User 授权。

包内 `platform status` 仅验证凭证格式，返回 `configured` 与 `verification: credential_shape_only`，不验证 Token 有效性或业务权限。上游 `--help` 会请求动态 MCP 目录，所以 wrapper 的总览帮助是本修订离线策略列表；Conformance 还应检查实际 native `--version`。版本 0.3.4 使用 [Teambition 官网 favicon](https://www.teambition.com/favicon.ico) 的 128px PNG，SHA-256 为 `941a5a0aa5c3609ead813267836a717d369c1e48321fd69eae1d985df4674908`；包内 SVG 嵌入同一 PNG，平台 `DisplayIcon` 将该品牌图标投影为前端支持的 PNG Data URL。

## 构建与检查

在仓库根目录执行，使用当前生产 Registry RepoDigest 与真实 Node 版本：

```bash
npm pack @tng/teambition-mcp-cli@0.3.3 --pack-destination /tmp --ignore-scripts --silent
curl -fL -o /tmp/teambition-skills.zip \
  https://file.teambition.net/public/teambition-cli/skills/teambition/teambition.zip
python3 scripts/connectors/teambition/build.py \
  --npm-tgz /tmp/tng-teambition-mcp-cli-0.3.3.tgz \
  --skill-zip /tmp/teambition-skills.zip \
  --runtime-image <registry/repository@sha256:digest> \
  --runtime-version <exact-node-version> \
  --output <absolute-output-directory>/teambition-0.3.4.zip
go -C backend run ./cmd/connector-package-validate <absolute-package-path>
node --test scripts/connectors/teambition/launcher.test.cjs
python3 -m unittest discover -s scripts/connectors/teambition -p 'test_*.py'
go -C backend test ./internal/connectorpackage/... ./internal/cliconnector/...
```

构建器同时输出 `.source.zip`。它用 `cli-connector-bundle` 调用现有 `ZIPPackageBuilder`，使平台从同一 source ZIP 构建的 bundle 与外层包逐字节一致；包和 source ZIP 的产物放在不提交的输出目录。构建器输出 `conformance: not_run`，只有真实生产 Worker 执行后才能记录通过。

## 平台发布

代码先提交、推送并集成到 `main_temp`，通过适用门禁。将同一输出包、source ZIP 和发布脚本传到已授权的部署主机，在主机读取其现有平台环境配置；不要将环境配置或 Token 拉到仓库。

```bash
python3 publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package <package-path> \
  --source <source-zip-path> \
  --evidence-directory <release-evidence-directory>
```

脚本用部署管理员的 OIDC + PKCE 登录，不输出登录材料。在部署主机上传大包时可传 `--api-base <已核实的本机容器 API origin>`，避免公网回流占满普通 API 的请求期限；默认使用公网平台 origin。API 地址必须属于本次授权的平台，鉴权和服务端校验照常执行。通过现有管理员 CLI upload/publish API 让 Worker 执行 ZIP 构建和 Linux + runsc Conformance，校验返回的 exact bundle SHA-256 与当前 Runtime Digest，通过 Stage/Publish API 发布 managed Publication，确认没有用户使用后以 Delete API 软删除临时 legacy Definition，保留原有 Conformance 行与历史证据。停用仍会在管理员目录显示，不能代替清理。再次运行或仅更新包内展示资产时复用已有 exact bundle/Runtime 验证，不重新创建 Definition；失败保持平台状态，不写入伪造的通过记录或直接修改数据库。`source.zip` 与外层包不一致时在 Stage 之前拒绝。

发布证据只包含非敏感的 build、stage、publication 和 health 响应。Conformance 通过仅证明这个 bundle 在这个 Runtime 可启动；真实 Teambition 账号授权、项目查询、任务写入、业务权限和动态帮助仍需实际账号验证。其他 Runtime Digest、私有部署和交互式 OAuth 均不由本修订宣称已验证。

## 已执行的发布证据（2026-09-30）

平台 Publication 已为 `available`，修订 `262b03b4-b10c-4eb2-bef0-b552d42e2a92`、版本 `0.3.3`。生产 Worker 的 exact bundle/Runtime 验证及证据边界见 `docs/technical/connector-platform.md` 的 Teambition 段落。发布构建来自 `main_temp`；首次发布的临时 legacy Definition 当时仅 disabled，导致管理员目录出现第二张卡片；该问题由后续 0.3.4 修订的软删除流程修复。平台用户使用 managed Publication。未替任何 User 安装或授权。

实际通过：Node launcher 三项边界测试、Python 构建器三项测试、目标 Go 包测试、`make test`、`make build`、最终 ZIP 的 `connectorpackage.Parse`、生产 Worker Linux + runsc Conformance，以及 native `--version`、无凭证状态、未放行命令拒绝检查。功能分支和 `main_temp` 均已执行测试与构建；同一输入再次构建的包逐字节一致。未执行真实 Teambition 账号业务 API、其他 Runtime Digest、私有部署、完整模型 Runtime Production Conformance；未改 Web 或 Runtime 镜像，对应构建门禁不适用。
