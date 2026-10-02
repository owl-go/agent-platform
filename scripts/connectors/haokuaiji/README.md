# 用友好会计 MCP Connector Package

当前交付是官方配置导入器和 companion Skill。没有企业授权配置，因此尚未生成真实好会计 ZIP，也没有完成安装、发布、MCP 握手或业务查询。构建器要求输入真实配置，不填入猜测的服务 ID；测试 fixture 不作为交付包。

## 调研结论（2026-10-02）

| 来源 | 已确认内容 | 选择 |
| --- | --- | --- |
| [畅捷通官方产品接入](https://cclaw.cjtmsp.com/docs/features/product-mcp-config)与[配置指南](https://cclaw.cjtmsp.com/docs/getting-started/mcp) | 好会计使用 MCP 插件市场；用户选择企业、对应产品、确认权限后复制 `mcp-json`；配置使用企业授权的 API Key | 优先使用官方 HTTPS Streamable HTTP |
| [官方 ERP 接入页](https://cclaw.cjtmsp.com/erp) | 明确包含好会计；各产品的操作由产品能力、企业授权、岗位权限共同决定 | 不把全部 ERP 能力归给好会计 |
| [社区 chanjet-MCP](https://github.com/sleel-sun/chanjet-MCP) | Python 项目自称支持 HKJ 文档和业务 API；`pyproject.toml` 为 `chanjet-tcloud-mcp` 0.1.8；安装采用源码 editable 模式，OAuth Token 存储在本机文件 | 不直接采用；不是畅捷通官方 CLI，也不是已验证的托管授权适配器 |

本次查询 PyPI 的 `chanjet-mcp` 和 `chanjet-tcloud-mcp` 均返回 404；npm `haokuaiji mcp` 搜索未返回好会计专用包。这是本次查询结果，不能据此断言不存在任何 CLI。[官方已公告支持 CLI/MCP](https://www.chanjet.com/news/136271.html)，但公开材料未提供本任务可以固定安装的好会计 CLI 软件包。

官方公开配置截图展示 MCP Hub Host `mcphub.chanapp.chanjet.com` 和 `/<service-id>/mcp` 路径；示例属于好生意／好业财，不能复制其 ID 作为好会计地址。好会计实际地址、请求头和工具清单必须来自目标企业的导出与真实发现。

## 构建

在官方「设置 → MCP → MCP 插件市场」选择目标企业及好会计，完成权限确认后复制 `mcp-json`，存到仓库之外的本机 JSON 文件。API Key 不发到聊天，不提交 Git。导出必须明确标识好会计，如 `HKJ_TENANT_MCP`、`HKJ-MCP` 或「好会计」。

```bash
python3 scripts/connectors/haokuaiji/build.py \
  --config /absolute/private/path/haokuaiji-mcp.json \
  --output /tmp/haokuaiji-0.1.0.zip
```

有多个好会计服务时用 `--server` 指定导出中的完整名称。构建器在内存中读取凭证，最终 ZIP 只包含 `connector-meta.json`、`icon.svg`、`mcp.json`、`skills/haokuaiji/SKILL.md`。不复制 Authorization，也不另存凭证文件。生成相同配置的结果确定；写入输出前用当前仓库 `connectorpackage.Parse` 验证最终 ZIP。输出仅表示包验证结果，业务验收明确为未执行。

兼容性采用 fail closed：仅接受官方 Host、HTTPS、数字服务 ID 和无 query/fragment 的 URL；拒绝 stdio、SSE、URL 凭证、重复键、未知配置字段、额外路由 Header 和非 Bearer 认证。如果真实配置不同，需要先审阅官方要求并实现适配，不删除字段以勉强通过。产品名检查只是误配保护，不证明上游服务身份或账号权限。

品牌 logo 来自[官方好会计产品标识](https://cclaw.cjtmsp.com/scheme/logos/logo-haokuaiji.webp)，SVG 原样嵌入 WebP。原始资产 SHA-256 为 `f1fce9d0299e6182e7b505d5e7814ca9a3dd6df02d49f82f6c431205210fb4c6`，外层 SVG 固定哈希由构建器核对。平台当前不会自动展示上传包内的 SVG；如果后续发布，需要补齐 `DisplayIcon` 映射并检查真实目录展示。

## 授权和执行边界

当前 MCP 托管执行只会将加密授权 JSON 中的 `MCP_BEARER_TOKEN` 物化为 `Authorization: Bearer …`；manifest 的任意 `headers` 没有被投影到执行配置。本构建器因此仅接受与此契约兼容的官方导出。实际好会计 Header 尚未确认。

现有 metadata 只允许 `oauth`、`none`、`cli`。本包选择 `auth_mode: cli`，使用服务端已有的提供凭证路径，表示由用户提供官方生成的 API Key；Connector 模式仍为 MCP，没有 CLI executable，也不声称实现平台浏览器 OAuth。官方企业授权仍在畅捷通入口完成。

现有 owner-scoped Connect API 可加密保存 `credentials_json` 中的 `MCP_BEARER_TOKEN`，不申请虚构 scopes。前端仅为指定 source 提供凭证表单，尚没有好会计的未连接卡片入口；上传 ZIP 或服务端保存授权不能作为 UI 接入验收。私人安装／公共发布需要明确目标身份，再检查现有 Publication 和 Installation 后复用或升级，避免重复条目。

本修订 Skill 仅指导已发现的只读工具。MCP manifest 没有 CLI 的逐项风险批准策略，Skill 不是服务端写入拦截器；发布前必须核对真实工具目录，禁用未审阅写工具或另行完成 broker 策略。没有真实工具目录时，不声称凭证写入、结账或报税可用。

## 验证

```bash
python3 -m unittest discover -s scripts/connectors/haokuaiji -p 'test_*.py' -v
go -C backend test ./internal/connectorpackage/...
make test
make build
```

测试覆盖配置拒绝边界、凭证不进入 ZIP、固定品牌资产和真实 Go Parse；它们不证明 MCP 连通或财税功能。未运行 Linux + runsc MCP 隔离检查、真实账号调用、目录图标和未授权卡片验收。没有 CLI bundle，因此无 CLI exact bundle × Runtime Conformance 记录。

2026-10-02 已实际执行上述命令：9 项 Python 测试、目标 Go 包测试、完整 Go 测试和构建均通过。真实 Go Parse 验证的是测试目录临时生成的 synthetic fixture ZIP，验证后已删除；它不是目标企业的交付包。未修改 Web，因此没有运行 Web 门禁；未执行 Runtime 镜像、远端存储或 Production Conformance 门禁，常规单元测试不能代替这些真实环境证据。
