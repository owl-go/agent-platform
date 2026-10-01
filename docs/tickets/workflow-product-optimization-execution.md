# Workflow 产品闭环优化执行计划

本计划把 Workflow 从“可运行的配置对象”改造成“可管理、可恢复、可重复执行的任务”。范围只覆盖现有产品契约，不新增模板市场、可视化编排或新的触发器。

## 成功标准

- 用户从 Workflow 列表启动或重新运行后，直接进入刚创建的 Run Conversation。
- 列表与详情概览能回答：最近结果、近 30 天成功情况、下次计划时间、是否需要处理。
- 新 Workflow 默认进入概览，不再落在空的产物页。
- 高级配置按任务分组、按需展开，Schedule 保存前展示未来三次执行时间。
- 重新生成 API Credential 和删除 Workflow 前明确展示影响。
- 失败、等待确认和运行中状态提供单一、可执行的主操作。

## 执行批次

| 批次 | 范围 | 验收证据 |
|---|---|---|
| WFO-01 | Workflow 列表运营摘要、搜索、状态筛选和排序 | Repository/Service 测试；列表组件测试 |
| WFO-02 | 默认概览、列表运行与重新运行直达 Run Conversation | Workflow 详情与路由组件测试 |
| WFO-03 | 设置页分组摘要、按需展开、精确配置入口 | Workflow 设置组件测试 |
| WFO-04 | Schedule 三次预览、Credential 轮换确认、删除影响说明、失败恢复动作 | 交互组件测试与文案断言 |
| WFO-05 | 契约生成、前后端完整门禁和执行记录回填 | `make test`、`make build`、`make web-typecheck`、`make web-build` |

## 数据口径

- `run_count_30d`：最近 30 天内有活动的 Run Conversation 数。
- `succeeded_run_count_30d`：上述 Run Conversation 中，最新 Turn 为 `succeeded` 的数量。
- `last_run_state`：该 Workflow 最近一个 Run Conversation 最新 Turn 的状态。
- `needs_attention`：最近状态为 `waiting_for_user` 或 `failed`。
- `next_scheduled_at`：启用 Schedule 时由服务端计算的下一次触发时间。

## 不在本轮范围

- Workflow 模板和模板市场。
- DAG 或节点式可视化编排。
- 新增触发器类型。
- 团队共享与跨 User 内容访问。

这些能力必须等“新 Workflow 30 天内至少成功执行两次”的真实数据达到产品门槛后再决定。

## 完成记录

- WFO-01：已完成服务端运营摘要投影，以及列表搜索、关注状态筛选、排序和上下文主操作。
- WFO-02：已完成默认概览、最近运行摘要，以及启动/重新运行后直达对应 Run Conversation。
- WFO-03：已完成设置页五类折叠分组，并支持从概览精确展开 Schedule、API 和 Git 配置。
- WFO-04：已完成未来三次 Schedule 预览、连续三次计划运行失败自动暂停、Credential 轮换/撤销保护和删除影响说明。
- WFO-05：契约已重新生成；目标包测试、前端目标组件测试、后端完整测试与构建、前端完整测试与生产构建均已通过。

已执行的验证命令：

```text
go test ./internal/biz/workspace/domain ./internal/data/workspace/gormrepo ./internal/service/workspace
pnpm vitest run src/pages/WorkflowsPage.test.ts src/pages/WorkflowDetailPage.test.ts
make test
make build
pnpm test
pnpm build
```

已从 `main_temp` 的 `8d89c96` 部署到线上，发布与健康检查证据见 `docs/evidence/agent-workspace/2026-09-29-workflow-product-loop-deployment.md`。真实企业账号下的浏览器操作、Workflow 再次运行和 Schedule 触发仍需单独验收，不能由本地自动化或未认证路由检查替代。
