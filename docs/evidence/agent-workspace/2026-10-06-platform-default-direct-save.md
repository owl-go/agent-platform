# 企业默认执行组合直接保存

日期：2026-10-06。用户要求移除企业默认执行组合中的“验证 Run”字段与保存前置条件。

## 行为

管理员选好可用 Runtime 和模型即可保存，无需填写或生成验证 Run。保留模型可用性、连接 API Key、不兼容组合拒绝、管理员权限与 Version CAS。保存继续原子同步继承账号，不修改个人执行 override；保存不晋升模型兼容性或供应商验证状态。历史 Run 引用可读取，后续保存清空该引用。Migration `000074` 仅放宽历史引用列的非空约束。

旧 API 请求的 `validation_run_id` 标记 deprecated 并忽略，Wire 字段编号与返回契约保持兼容；Web 不展示或发送该字段。继承提示改为管理员已配置，不再宣称已验证。

## 已执行检查

```text
make generate
make breaking
make verify-generated
WORKSPACE_TEST_POSTGRES_DSN=<disposable PostgreSQL 17> go -C backend test -race -count=1 ./internal/data/workspace/gormrepo -run TestPlatformExecutionDefault
WORKSPACE_TEST_POSTGRES_DSN=<disposable PostgreSQL 17> go -C backend test -count=1 ./internal/data/workspace/gormrepo/...
go -C backend test -race -count=1 ./internal/service/workspace -run TestPlatformExecutionDefault
make test
make build
pnpm --dir frontend exec vitest run
make web-typecheck
make web-build
git diff --check
cmp -s AGENTS.md CLAUDE.md
```

以上检查通过。前端 613 项测试通过。真实本地 PostgreSQL 完整 GORM 包通过；定向测试验证未验证供应商、无 Run 保存、不晋升兼容性、继承更新及新账号继承、使用未验证继承组合创建 Run、历史引用读回和后续清空、拒绝不兼容/缺少 API Key/不可用模型/普通账号/stale Version。Service 测试验证无 Run 请求与旧字段均可保存，并保持管理员和 Runtime 可用性边界。页面测试验证无输入框、选好模型按钮可用、请求省略字段及保存成功 Toast。

## 集成与发布

开发提交 `3df555e`；`main_temp` 集成提交 `e7feed69ceec8a706112685ee7094db5f22feac4`，Tree 与已通过上述门禁的开发提交相同。

发布版本 `execution-default-direct-save-20261006-1`。源码归档 SHA-256：`2b39ac780c31e3a15d9f4fcb696b56c56a60c8a2c5e22e27e91ad87fd49d76e0`。已从 `main_temp` 发布 API 与 Web；数据库和保护配置备份校验通过，保留回退镜像及旧 Web 版本。

- API 镜像：`sha256:126bc390bb2e2d5f0131c870c529a9e5be47ad2cfeb5a27b6294ef22a7a0ef2e`。API 与保留的 Worker 均 healthy、重启次数 0。
- Migration 已到 `000074_optional_platform_default_validation_run.sql`，生产信息 Schema 确认 `validation_run_id` 可为 NULL。
- `/api/healthz`、`/api/readyz`、Keycloak Discovery 与公开状态页返回 200；管理员 API 匿名访问拒绝。
- 在线设置页面 bundle 已无验证 Run 输入，index、SettingsPage、i18n、client 和账号管理 bundle 与本次生产构建逐字节一致。
- 微信四位数字登录码、真实 Keycloak 到公众号等待页面的跳转、Cookie 属性、错误浏览器与 Origin 拒绝条件仍通过检查。
- Keycloak 配置 check 通过；API 日志未发现 error/fatal/panic。

## 验证边界

本次未修改线上企业默认的选定组合或供应商凭据；未在生产提交配置保存或启动真实付费模型执行。真实 Runtime Production Conformance 和远端对象存储 Conformance 未执行。本地创建 Run 不等于指定镜像实际执行成功。
