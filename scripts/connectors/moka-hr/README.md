# Moka HR 招聘连接器

`moka-hr` 0.1.0 是本仓库维护的 Node CLI，通过 Moka 官方 ATS API 提供 14 项只读业务查询。不是 Moka 官方 CLI，不覆盖 Moka People 或任何业务写入。包版本、CLI 源码 SHA-256、品牌资产和目标 Node 24.15.0 Runtime 都固定；没有 npm 运行依赖、安装脚本或本地凭证文件。

## 调研与身份

- [Moka 官方开放平台](https://open.mokahr.com/)区分 ATS 与 People；本包使用[ATS API 文档](https://www.mokahr.com/docs/api/index.html)以及其中链接的[旧版 eHR API 文档](https://www.mokahr.com/docs/api/view/v2.html)。鉴权选择官方支持的 Basic Auth：企业 API Key 为用户名、空密码，全部使用 HTTPS。官方也提供企业 clientId/clientSecret 换取 accessToken，本修订不实现该方案。
- 找到真实[社区 Moka MCP](https://github.com/mingyangsun-sketch/moka-mcpserver/tree/1377d66163cefbd59691a1fd89d9fa96dbf6503b)。调研时版本为 0.1.0，固定 commit `1377d66163cefbd59691a1fd89d9fa96dbf6503b`，MIT License，提供 13 项只读工具；PyPI 的 `moka-mcp-server` 查询返回 404，没有公开固定版本 registry 包。其文档推荐 Git 来源 uvx 或自托管 HTTP。平台当前 stdio assembler 只生成 registry `package==version`；没有现成可信 HTTPS 部署地址。因此没有生成一个虚构的 registry MCP 包，而是在既有 CLI bundle seam 中实现同类官方 API 查询，并补上面试列表。代码独立实现，不复制其运行代码或权限近似模型。
- [openings-mcp](https://github.com/amikai/openings-mcp)的 Moka provider 用于公开招聘官网搜索，不能满足内部候选人与面试需求；MokaKit 不是 Moka HR 招聘系统。
- 企业 Key 可能读取全企业数据，不能当员工授权或推导用户个人权限。本包只向当前所有者的单次进程注入 Key，没有跨 User 共享、代入员工邮箱、任意 URL、raw API 或自定义 Header 能力。没有编造 OAuth scopes；manifest 的身份 `user` 是平台所有者身份。

完整操作、API 路由和 capability 映射见 [查询表](operations.md)，使用顺序见 [伴随 Skill](SKILL.md)。eHR `stage=all` 只指 Offer 与待入职阶段；职位列表来自招聘官网，不代表全部内部职位。分页单页返回原始 `next`，不自动访问上游 URL。手机号与身份证按字段掩码，其他个人信息不会自动匿名化。HTTP 500 保留错误，不从错误推断“没有候选人”。

## 构建

```bash
python3 scripts/connectors/moka-hr/build.py \
  --runtime-image '<configured-repository>@sha256:<64-lowercase-hex>' \
  --output outputs/moka-hr/moka-hr-0.1.0.zip
go -C backend run ./cmd/connector-package-validate ../outputs/moka-hr/moka-hr-0.1.0.zip
node scripts/connectors/moka-hr/moka.mjs --help
```

外层 ZIP 含 metadata、官方品牌 SVG、一个 CLI manifest、不可变 bundle 和 Skill 及其查询表。构建器同时输出 source ZIP；本地与平台 Worker 通过相同 `ZIPPackageBuilder` 得到相同 bundle。源码与 icon 哈希不匹配、缺少 Registry RepoDigest 或 Node 版本不匹配时停止。品牌 PNG 是 [Moka 官网 favicon](https://www.mokahr.com/favicon.ico)的原始 PNG 字节；SVG 与后端 `DisplayIcon` 使用同一资产，前端现有 `ConnectorIcon` 支持其 PNG data URI。

本地将 Key 和组织 ID 放在 `MOKA_API_KEY`、`MOKA_ORG_ID` 环境变量中，再调用 `moka jobs list --json '{"mode":"social","limit":20}'`。平台使用 `connector_package` driver，连接表单保存 `{"moka_api_key": "…", "moka_org_id": "…"}` 到所有者范围内的加密存储；单次调用只读取 `CONNECTOR_CREDENTIALS_JSON`，存在该变量时不会回退到另一个本地 Key。密钥不接收 argv 输入。表单成功或取消后清空两字段，失败保留供重试；需升级时先升级再保存。`auth`/`status` 使用只读部门 API 验证企业接口访问，连接表单保存本身不能证明真实上游授权成功。`unauth` 不撤销上游 Key，撤销需联系 Moka CSM。

## 发布与验收

平台代码须先集成 `main_temp`、执行适用门禁并部署。随后在授权部署主机执行：

```bash
python3 scripts/connectors/moka-hr/publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package /tmp/moka-hr-0.1.0.zip \
  --source /tmp/moka-hr-0.1.0.source.zip \
  --evidence-directory /tmp/moka-hr-publication-evidence
```

发布器先查看已有修订与 exact Conformance。缺少证据时由平台 Worker 构建临时 Definition，真实运行 `bundle SHA-256 × Runtime RepoDigest` Conformance；确认一致后暂存和发布，并软删除没有用户使用的临时 Definition。重复执行复用已验证修订。发布不等于所有用户已经安装或授权。完成需检查 Administrator 与 User 目录各只有一个正式条目、品牌图标显示、安装后可打开无凭证连接表单，以及临时构建卡片消失。

```bash
node --test scripts/connectors/moka-hr/moka.test.mjs
python3 -m unittest discover -s scripts/connectors/moka-hr -p 'test_*.py' -v
python3 -m unittest discover -s scripts/connectors/teambition -p test_publish.py -v
go -C backend test ./internal/connectorpackage/... ./internal/cliconnector/...
pnpm --dir frontend test src/components/ExtensionManager.test.ts src/components/ConversationComposer.test.ts src/components/ConnectorIcon.test.ts
make test
make build
make web-typecheck
make web-build
```

协议 fixture 严格校验真实 Basic Auth 格式、各路由、方法、body 与 query；覆盖未审阅命令和参数拒绝、身份/路径绕过、日期范围、单页游标、密钥与 base64 反射脱敏、业务错误、HTTP 错误、输出限额及断流。它证明文档协议与本包行为，不能代替真实企业账号验证。当前没有 Moka 企业凭证，真实职位、候选人、面试与企业 API 模块权限均未验证；包不包含任何测试或真实凭证。完整模型 Runtime Production Conformance、存储 Conformance 与业务写入不属于本次验证范围。
