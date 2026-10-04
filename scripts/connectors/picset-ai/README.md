# Picset AI 连接器

依据 [Picset AI 开发者 API](https://picsetai.cn/developer-api)制作的 Agent Workspace REST CLI 桥接器，版本 0.1.0。官方页面未提供可直接安装的 CLI 或 MCP 服务配置；本包使用仓库自有 Node 源码，构建器固定源码、策略和品牌图标 SHA-256，不安装第三方依赖。

`operations.json` 是 15 个官方 API 操作的路由和字段白名单；生成包额外允许 schema、operations、version 三个只读诊断命令。API Key 表示 User，官方未公布 OAuth scopes。本包通过平台 `connector_package` driver 注入 `{"picset_api_key":"…"}`，前端在安装后提供密码字段和官方密钥管理链接。全部 POST 声明 high risk，付费生成还强制使用预先保存的幂等键。

此修订不提供 OSS PUT 或结果下载；官方文档的 Bucket 域名是占位符，不能据此开放通配 Egress。upload-url、image-audit 和图片业务接口仍可用于已经在受控外部环境完成上传的素材；纯文本 canvas-image 可直接使用。没有真实账号时，不宣称 Key 有效、图片生成成功或素材上传已验证。

```bash
node --test scripts/connectors/picset-ai/picset-ai.test.mjs
python3 -m unittest discover -s scripts/connectors/picset-ai
python3 scripts/connectors/picset-ai/build.py --runtime-image <registry-repodigest> --output /tmp/picset-ai-0.1.0.zip
go -C backend run ./cmd/connector-package-validate /tmp/picset-ai-0.1.0.zip
```

构建同时生成 `.source.zip`。提交与推送后合并到 main_temp，执行适用门禁并从 main_temp 发布前端连接入口与品牌图标。随后在授权部署主机使用 `publish.py --config <platform.env> --package <ZIP> --source <source ZIP> --evidence-directory <directory>`；它通过既有 Worker 完成受限 Builder 和 exact bundle × Runtime 的 Linux + runsc Conformance，通过管理员 API 暂存、发布并软删除临时 Definition。未通过时保留失败证据且不宣称发布成功；重跑复用已有修订和验证证据。

包校验、--help Conformance、无凭证 UI 与真实业务验收是不同证据。`status` 只检查密钥格式；真实授权验证应在所有者允许范围内查询已知请求，不创建测试付费任务。

品牌图标来自官方站点 favicon.png，2026-10-01 获取；包内 SVG 嵌入该 PNG，服务端目录投影使用同一 PNG。发布证据记录在 docs/evidence/agent-workspace 中。
