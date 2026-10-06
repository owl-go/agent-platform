# 北大法宝与明白律师连接器 — 2026-10-02

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 发布结果

两个独立 `1.0.0` 官方远程 MCP Connector Package 已发布至[连接器市场](https://workspace.example.com/resources?tab=connectors)，按 source 归入“法务合规”。构建源与操作说明位于 `scripts/connectors/legal/README.md`。

| 服务 | Source | Revision | 验收 Installation | 授权状态 |
|---|---|---|---|---|
| 北大法宝 | `pkulaw` | `3ecdbcc3-4021-415a-b7fc-6286d2044ac0` | `d47433df-3c49-435e-a751-a16cd281b08f` | 未授权，等待用户填写 Token |
| 明白律师 | `mindbye` | `9c1bf1d7-50ed-4044-ab93-b3cfcc269246` | `bf702b1c-3021-4870-b8e5-4040993e38b7` | 无需凭证，已可选择 |

两条 Publication 均为 `available`，验收 Installation 均为 `active`。发布管理员自己的 User catalog、Administrator 正式 Publication 条目与 Installation 目录分别各只有一个对应 source。北大法宝的 MCP DTO 兼容字段 `authentication_driver` 回显 `oauth`；服务端按 provided-credentials 模式处理，不能把该字段误读为一个浏览器 OAuth driver。发布器初次已完成法宝暂存、发布、安装，但用 CLI driver 名称检查 MCP DTO 导致验收中止；修正检查后复用原 Revision，没有重复创建法宝条目，并完成明白律师发布。

## 包、代码与部署

- 功能提交 `00518ab`（`codex/legal-connectors`），通过 `main_temp` 集成提交 `fb9d7a0` 发布。发布器兼容检查修正提交 `20508d9` 通过 `main_temp` 集成提交 `1fe27cc` 发布；该修正只改变发布脚本、测试与说明，未改变 ZIP 或已部署服务行为。
- 完整部署 release：`legal-connectors-20261002-1.0.0`。源目录 `/srv/agent-workspace/src.release-legal-connectors-20261002-1.0.0`，Web `/srv/agent-workspace/web/releases/legal-connectors-20261002-1.0.0`，发布前备份 `/srv/agent-workspace/backups/pre-legal-connectors-20261002-1.0.0`。
- 北大法宝 normalized Package SHA-256：`66118421d6d83f98a13e6479113592222dd782fc7970ee177bd971c73db41669`。
- 明白律师 normalized Package SHA-256：`e1f7bbd076ad91403898dfb5b929eae6f29dadc971fd9197ee0d588a9a75cba0`。
- 本地 ZIP 和收据位于功能工作区忽略目录 `outputs/legal/packages`；不含凭证的发布响应与测试、部署日志位于 `outputs/legal/evidence`。远端永久包归档位于 `/srv/agent-workspace/connector-releases/legal-1.0.0`，结果位于 `/srv/agent-workspace/evidence/legal-1.0.0`。修正后的发布工具从已推送的 `main_temp` 快照复制到包归档的 `tools`，没有改写不可变源 release。
- 每个 ZIP 均包含 metadata、内嵌官方品牌 PNG 的 SVG、恰好一个 `mcp.json` 与一个 companion `SKILL.md`。没有 CLI bundle、临时 CLI Definition 或共享外部账号授权需要清理。User 安装与授权保持独立；其他 User 的安装不会被批量升级。

## 已执行验证

- 两个最终 ZIP 由当前仓库 `go -C backend run ./cmd/connector-package-validate` 实际调用 `connectorpackage.Parse`；平台 Stage 再次验证并返回相同 normalized SHA-256。
- 2 项 Python 构建/发布生命周期测试通过：确定性 ZIP、模式和端点、凭证变量、当前工具名、复用 Revision 与幂等安装。
- `go -C backend test ./internal/connectorpackage/... ./internal/service/workspace/... ./internal/data/workspace/runtimeexecutor/...` 通过。新增法宝凭证边界用例拒绝错误字段、非字符串、空值、额外字段、超长、Bearer 前缀与 CRLF；品牌 PNG 通过前端支持的 icon 校验并与捆绑资产字节一致。
- 功能分支 `make test`、`make build`、`make web-typecheck`、`make web-build` 通过。121 项 ExtensionManager 组件测试通过，覆盖法宝遮蔽 Token 表单、控制台入口、提交与升级顺序、非法输入拒绝、失败重试与取消清空，以及两者在市场和已安装视图的分类。发布器修正后的 121 项组件测试与 2 项 Python 测试再次通过。
- `make deploy` 从 `main_temp` 集成提交执行完整 Go 测试/构建、前端 46 文件 / 483 项测试、类型检查和生产构建。数据库、Identity 数据库和配置备份校验、镜像构建/部署检查、API/Worker/Egress/Caddy 健康、HTTPS/OIDC 与 Web release 检查完成。
- 本地与目标 Linux Worker 宿主的真实网络请求均确认：法宝无凭证请求为 401；明白律师 MCP initialize 为 200，server 为 `contract-review-server` 1.0.0，工具发现包含 `contract_review_prepare_upload`、`contract_review_submit`、`contract_review_result`。
- 明白律师真实上传申请成功，使用合成文件名和大小，返回 HTTPS `contract-review-mcp.oss-cn-hangzhou.aliyuncs.com` 上传目标；该域名在 manifest Egress 清单中。未执行文件 PUT、合同提交或结果查询，签名 URL 和临时 fileId 未进入证据或版本库。
- Publication 和 Installation 的图标均投影官方 PNG Data URL，本地图标像素已查看。线上分类与 Token 入口的行为由组件测试、实际发布的源/Web release 与 API 目录结果支持。

## 验证边界

没有真实北大法宝 Token，未验证授权后的工具发现、法规查询、权限和服务积分。没有提交合同，未验证明白律师完整异步审查、正式报告与附件下载。MCP catalog 的 `conformance_available` 只表示现有包策略可供目录使用，不代表目标 Runtime Digest 已通过这些服务的生产模型调用。

未运行这两个服务的完整 Linux + gVisor + 真实模型 Production Conformance、完整 Sandbox Conformance 或真实远端存储 Conformance；测试的 Skip 不计为相应环境通过。Browser 接口读取和创建标签超时，native Chrome 控制返回用户同时改变应用的状态；本次没有取得线上浏览器的分类、品牌图片加载和 Token 表单截图验收，也没有向浏览器提交任何法宝凭证。
