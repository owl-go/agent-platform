# OpenBoost Connector Package

`openboost` 0.1.0 是仓库维护的 CLI 桥接器，通过 OpenBoost 官方统一 HTTPS Streamable HTTP MCP 提供跨境数据查询。执行身份是 User，认证是官方支持的 `secret-key` 请求头，由平台的 `connector_package` driver 加密保存并逐命令物化。包不包含密钥，也不写本地凭证文件。

## 上游核实与选择

- [官方 MCP 配置](https://open.microdata-inc.com/mcp-list)提供分服务 Streamable HTTP/SSE 及 Secret Key 配置；官方页面公开 JavaScript 的配置生成器确认 `secret-key` 请求头。
- [官方 CLI](https://open.microdata-inc.com/cli)及 [PyPI openboost-cli 2.0.2](https://pypi.org/project/openboost-cli/2.0.2/)提供 Python 3.10+ 命令行客户端。该版本 wheel SHA-256 为 `fabe8be5eb8c37a8a4cc2b7b11c0ab4ab05a1811dbff3050ed4b414d9b07cba8`；其文档包含环境变量、配置文件与通用调用，并引用统一 `openboost-all-mcp/sse`。本包使用这个统一服务的 HTTPS Streamable HTTP 入口，不打包或冒充该 PyPI CLI。
- `https://mcp.microdata-inc.com/mcp-servers/openboost-all-mcp` 在 2026-10-02 无凭证真实 `initialize` 返回协议 `2025-03-26`、服务 `proboost-tiktok-amazon-patent-mcp` 1.0.0；`notifications/initialized` 返回 202；`tools/list` 返回 101 个带参数 Schema 的工具。实际目录也包含社媒和账号能力，已保存为 `tools.json`。
- 无凭证 `account_info` 和 `account_links` 返回 HTTP 200、MCP `isError:true`，内容明确为缺少 Authorization 或 secret-key。握手、公开工具发现不代表业务授权。
- 当前平台 MCP 投影未传递 `headers`，执行端也未应用 `disabled_tools`。本修订沿用 CLI broker，用固定命令和端点实现官方协议，保留逐工具能力审核。100 个发现的工具允许查询，创建支付订单 `account_order_create` 被排除；新增工具不会自动进入白名单。

## 构建与执行

```bash
python3 scripts/connectors/openboost/build.py \
  --runtime-image '<configured-repository>@sha256:<64-lowercase-hex>' \
  --output /tmp/openboost-0.1.0.zip
go -C backend run ./cmd/connector-package-validate /tmp/openboost-0.1.0.zip
node scripts/connectors/openboost/openboost.mjs --help
node --test scripts/connectors/openboost/openboost.test.mjs
python3 -m unittest discover -s scripts/connectors/openboost -p 'test_*.py' -v
```

构建固定 Node 24.15.0、代码 SHA-256 与品牌 SVG SHA-256，使用平台实际 `cli-connector-bundle` 构建器产生确定性 source ZIP、bundle 和最终包。103 项 capabilities = 100 个工具 + tools/schema/version。业务超时 180 秒，容纳上游最长约 120 秒的社媒采集；每次查询可能扣减套餐额度，不自动重试。

本地通过 `OPENBOOST_SECRET_KEY` 环境变量调用；托管执行只读取 `CONNECTOR_CREDENTIALS_JSON` 中的 `openboost_secret_key`。平台凭证存在时不回退到本机密钥。先 `tools`，再 `schema call <tool>`，最后 `call <tool> --json '<对象>'`；托管必须使用 [companion Skill](SKILL.md) 的 `agent-cli invoke`。`auth`/`status` 通过真实 `account_info` 校验权益；`unauth` 提示断开平台授权，不声称撤销上游密钥。

品牌 SVG 来自官方结构化数据的 Organization Logo：[官方品牌资产](https://picbed.microdata-inc.com/nas/20230711763b909dbbd24b6bae1996699bddc6a2.svg)。包、后端 source 展示映射与前端静态 SVG 保持一致。

## 平台发布

先经 `main_temp` 集成适用测试与平台部署，再在授权部署主机执行：

```bash
python3 scripts/connectors/openboost/publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package /tmp/openboost-0.1.0.zip \
  --source /tmp/openboost-0.1.0.source.zip \
  --evidence-directory /tmp/openboost-publication-evidence
```

发布器检查已有修订；缺少 exact bundle × Runtime 证据时，用临时 Definition 让 Worker 执行隔离构建和真实 Conformance，再暂存、发布正式 Revision，通过管理员 API 软删除无使用关系的临时 Definition。重跑复用符合条件的证据与修订。

安装后的连接详情提供单个密码类型 Secret Key 字段与官方链接，先升级旧修订再保存；成功与取消清空输入，失败保留以供重试。Publication 不共享任何 User 的授权，也不自动安装给所有 User。

协议 fixture 覆盖固定端点/Header、初始化/通知、session 与 stateless 两种响应、发现、100 个调用路由、JSON/SSE、UTF-8 分片、支付创建拒绝、参数与输出限额、重定向、超时及凭证反射脱敏。模拟工具调用不作为真实账号业务验证。发布证据与仍未验证的账号能力另记于 `docs/evidence/agent-workspace`；完整模型 Production Conformance 与远端存储门禁不属于本次改动。
