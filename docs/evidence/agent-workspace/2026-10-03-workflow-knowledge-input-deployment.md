# 工作流知识库多选输入框部署验证 — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 发布标识

- 功能分支：`codex/workflow-basic-knowledge`，功能 Commit：`99ca5d0`。
- 集成分支：`main_temp`，实际 Web 构建源 Commit：`c65dca971f4560f1fdaf43caea7f32719e0724ed`；无冲突合并，已推送。
- Web 发布 ID：`workflow-knowledge-input-20261003-1`。
- 公网入口：`https://workspace.example.com`。
- Web 目录：`/srv/agent-workspace/web/releases/workflow-knowledge-input-20261003-1`。
- 发布前记录：`/srv/agent-workspace/backups/pre-workflow-knowledge-input-20261003-1`，保留原 Web、源目录指针和原首页 SHA-256。
- 原 Web：`/srv/agent-workspace/web/releases/workflow-channel-icons-20261003-1`。

知识库选择改为可搜索的多选输入框，选中项以可移除标签显示，没有 Ready 文档的知识库仍不可新增选择。此次仅发布 Web，保留集成分支已有的消息渠道图标与其他更新。API、Worker、数据库和 Runtime 未重新部署；后端源指针仍为 `/srv/agent-workspace/src.release-resource-query-isolation-20261003-1`。

## 已执行检查

在 `main_temp` 工作区实际通过：

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend test
make web-typecheck
git diff --check
# 使用线上 PUBLIC_HOST 对应的四项公开 VITE_OIDC 配置：
WEB_DEPLOY_HOST=agent-platform WEB_RELEASE_ID=workflow-knowledge-input-20261003-1 make web-deploy
```

完整前端测试为 52 个文件、549 项用例通过。发布脚本完成类型检查、生产构建、上传、远端文件校验和 current 原子切换。生产构建成功，仍有既有大 Chunk 提示。

公网首页、工作流详情 JavaScript/CSS、入口 JavaScript/CSS 均返回 200，SHA-256 与本地生产产物一致。首页在本地产物、远端 current 和公网响应中的 SHA-256 均为 `7e71c86ac7d5d4afb5e18c5fbad196f0a2b7475f6119475a70e6b7926b982512`。

公网 `/api/healthz` 返回 200 和 `ok`，`/api/readyz` 返回 200 和 `ready`。API、Worker、Egress Controller 容器均 healthy，Caddy 运行。

本机日志：`/tmp/agent-platform-workflow-knowledge-input-tests.log`、`/tmp/agent-platform-workflow-knowledge-input-web-deploy.log`；公网验证结果：`/tmp/agent-platform-workflow-knowledge-input-public-evidence.json`。

## 验证边界与回滚

本记录证明集成测试、构建、发布与公网产物一致性。未使用真实账号执行发布后的桌面/移动端知识库选择操作或真实消息渠道收发。本次未运行 Go、Linux Sandbox 或完整 Production Conformance 门禁，没有新增 Runtime Capability 验收结论。

需要回滚 Web 时，在集成工作区执行：

```bash
WEB_DEPLOY_HOST=agent-platform scripts/deploy-web.sh activate workflow-channel-icons-20261003-1
```
