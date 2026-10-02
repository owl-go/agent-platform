# 法务合规连接器

两个独立 `1.0.0` MCP Connector Package：`pkulaw`（北大法宝）与 `mindbye`（明白律师）。它们使用厂商公开的 HTTPS Streamable HTTP 服务，不包含自制 CLI 或自托管中转。

## 上游与范围

- [北大法宝官方接入指南](https://mcp.pkulaw.com/docs?doc=mcp-integration)与[官方 CLI npm 文档](https://www.npmjs.com/package/@pkulaw/mcp-cli)提供 MCP 与 `@pkulaw/mcp-cli`。本修订选择远程法规语义服务 `https://apim-gateway.pkulaw.com/mcp-law-search-service`；文档列出 `search_article`、`get_article`。其他法规关键词、案例、引用校验等能力使用不同端点。平台单个 MCP manifest 只有一个服务，因此此修订不宣称覆盖全部法宝服务。
- 北大法宝使用控制台 Token，无需另造 OAuth 协议。包的兼容元数据 `auth_mode: oauth` 在现有平台被映射为 `connector_package` provided-credentials 流程；表单只收 Token 值，以 `MCP_BEARER_TOKEN` 保存。Runtime 现有适配器生成 `Authorization: Bearer ...`；manifest 的变量引用不包含字面 Secret。平台加密保存，单次执行物化并精确脱敏。未带 Token 的真实请求返回 401；真实 Token 的权限、积分、查询仍需验证。
- [明白律师官方接入页面](https://www.mindbye.com/mcp)提供公开端点 `https://mcp-server.mindbye.com/mcp-servers/contract-review-prod` 和[合同审查技能下载](https://mindbye.com/skill/contract-risk-review.zip)。本修订无需凭证。实际工具发现为 `contract_review_prepare_upload`、`contract_review_submit`、`contract_review_result`；上游下载技能中的旧 `submit_contract_review` 名称不能用于现网调用。包内 Skill 按现网 Schema 适配审查参数，并要求保存 taskId、使用同一任务轮询、只交付服务真实返回的报告与附件。
- 真实临时上传申请返回 `contract-review-mcp.oss-cn-hangzhou.aliyuncs.com`。该域名与 MCP 服务域名加入 Egress 清单。测试只使用合成文件名和大小，未上传文件或提交合同；签名 URL 和文件 ID 不进入日志或版本库。

官方网页 favicon 转为原尺寸 PNG：北大法宝 `https://mcp.pkulaw.com/gaoshaolin.ico`（32×32），明白律师 `https://www.mindbye.com/favicon.ico`（64×64）。包内 SVG 内嵌 PNG；平台展示使用同一品牌资产。产品市场和已安装目录均按 source 归入“法务合规”。

## 构建、验证与发布

```bash
python3 -m unittest discover -s scripts/connectors/legal -p 'test_*.py'
python3 scripts/connectors/legal/build.py --output-directory outputs/legal/packages
python3 scripts/connectors/legal/smoke.py
go -C backend test ./internal/connectorpackage/... ./internal/service/workspace/... ./internal/data/workspace/runtimeexecutor/...
pnpm --dir frontend test src/components/ExtensionManager.test.ts
```

构建器对最终 ZIP 实际执行 `go run ./cmd/connector-package-validate`，由当前 `connectorpackage.Parse` 生成规范化 SHA-256，保存在对应 `.receipt.json`。包同时包含 metadata、icon、一个 mcp manifest 和一个 companion Skill。

功能提交先合入 `main_temp`，完成适用门禁后从该分支部署品牌投影、分类和 Token 表单。部署完成后，在已授权的部署机使用既有平台环境文件发布：

```bash
python3 scripts/connectors/legal/publish.py \
  --config /opt/agent-platform/config/platform.env \
  --package-directory /opt/agent-platform/connector-releases/legal-1.0.0 \
  --evidence-directory /opt/agent-platform/evidence/legal-1.0.0
```

发布器校验 ZIP 与审阅源文件及收据一致，按 normalized SHA-256 复用 Revision，用乐观版本更新 Publication，并幂等安装到执行管理员的 User 视图；已有旧安装仅升级该验收安装。它检查 User 目录、Administrator 正式条目和 Installation 各恰有一个 source，验证品牌投影和认证模式。其他 User 自行安装，已有 User 安装不会被批量升级。无 CLI 临时 Definition 需要清理。

包解析、MCP 工具发现、安装可用状态分别属于不同证据。当前未验证北大法宝真实账号查询、明白律师完整合同审查与附件下载，亦未对这两个服务执行完整 Linux + gVisor + 真实模型 Production Conformance。
