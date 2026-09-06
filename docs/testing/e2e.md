# Agent Workspace 端到端测试

测试以真实浏览器登录 Keycloak，再连接当前 Vue 页面、Go API、PostgreSQL 和 MinIO。测试环境由脚本单独创建，不使用线上账号、生产数据库或其他项目的容器。用例编号、步骤和预期结果在 `scripts/e2e/cases.mjs`；执行断言在 `frontend/e2e/workspace.spec.ts`。

## 执行

前提：Docker 已启动，Go 可使用仓库要求的工具链，Node.js 和 pnpm 可用。首次需要下载依赖及 Chromium：

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend exec playwright install chromium
pnpm --dir frontend run typecheck:e2e
pnpm --dir frontend run test:e2e
node scripts/e2e/report.mjs
```

也可以从仓库根目录运行 `node scripts/e2e/run-local.mjs --grep E2E-043` 定位单条失败；汇总脚本仅接受完整用例运行的报告。测试脚本保留真实失败，失败退出码为 1，不把已知缺陷改成通过。

脚本使用本机回环端口 14173、19080、19081、19090、15439。数据库和对象存储容器是此次运行独立创建的；退出时删除其容器及匿名卷。Keycloak 使用固定 Digest；可通过 `E2E_KEYCLOAK_IMAGE` 指定兼容镜像。脚本使用临时密码和随机服务凭据，通过私有临时目录和进程环境传递，不写进测试报告。失败时私有诊断目录的位置会打印到终端，其中配置和日志不能发布。

报告位于被 Git 忽略的 `output/playwright/`：`results.json` 为 Playwright 原始结果；`summary.json` 为只含用例、状态和耗时的汇总；`html/index.html` 为可视报告。默认关闭 Trace、视频及自动截图，避免意外记录登录凭据；移动端用例只截取已登录的测试页面。

## 范围

44 条记录包含 11 条浏览器 E2E、29 条 API 集成 E2E，以及 4 个明确跳过的环境门禁。API 用例使用真实 OIDC 登录得到的 Token，不模拟产品 API。账号、Session、Workflow、Expert、Expert Team、Skill、MCP、选择草稿、附件、积分、权限、页面及健康接口均有执行记录。浏览器用例覆盖新建 Session、创建 Expert、创建 Workflow、MCP 新增、团队重新编辑及移动布局。

MCP 管理用例只保存和删除配置，不访问 `example.test`。本地所有 Runtime 均显式 unavailable，不启动 Worker，不冒充模型执行成功。真实模型、SSE 终态、定时执行、CLI 授权及审批恢复、Linux + gVisor 隔离，需要独立的 Linux 验收环境。四条跳过记录是环境门禁清单，不是已经实现的完整 Runtime 自动化脚本。

原始 Excel 中 395 条手工用例比这次自动化范围更广。关联编号仅代表对应步骤的证据，不表示相关手工用例全部通过。真实 Runtime 验收应遵循 `docs/technical/production-conformance.md`，先执行 Preflight。
