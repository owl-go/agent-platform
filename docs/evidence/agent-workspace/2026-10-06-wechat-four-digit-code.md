# 微信四位数字登录码验证

日期：2026-10-06。用户明确保留网页生成码、用户向公众号发送码、网页自动完成登录的流程，仅将码缩短为四位数字。

## 实现与本地验证

- 随机码范围 `0000`–`9999`，字符串保留前导零；拒绝非四位 ASCII 数字。
- 五分钟有效、PostgreSQL 唯一码预留、至多 32 次碰撞分配重试；OIDC handoff 不提前释放预留码。
- 微信 Attempt 容量 100；已认证发送者五分钟 5 次新确认、全局一分钟 30 次，跨 API 副本原子计数。已确认的同身份重试幂等；限流及存储故障不绑定身份。
- 浏览器 Cookie、同源完成请求、S256 PKCE、加密消息和 CAS 身份绑定继续保留；旧长码页面提示过期。

实际执行并通过：

```text
pnpm install --frozen-lockfile
WORKSPACE_TEST_POSTGRES_DSN=<disposable PostgreSQL 17> go -C backend test -race -count=1 ./internal/biz/account/... ./internal/data/account/... ./internal/service/workspace/...
WORKSPACE_TEST_KEYCLOAK_URL=<disposable Keycloak 26.7.1> go -C backend test -race -count=1 -p 1 ./internal/service/workspace -run TestRegistrationKeycloakOIDCHandoffIntegration
go -C backend test -race -count=1 ./internal/service/workspace -run 'TestWeChatEncrypted|TestRegistration'
make test
make build
pnpm --dir frontend exec vitest run
make web-typecheck
make web-build
git diff --check
cmp -s AGENTS.md CLAUDE.md
```

开发分支前端 611 项测试通过；`main_temp` 集成最新设置修复后前端 612 项测试通过。真实本地 PostgreSQL 验证包含同一发送者并发准入、跨 Repository 共享额度、全局额度、过期解锁、码冲突、handoff 后预留与容量拒绝。Keycloak 集成使用 fake 上游身份验证实际联邦登录、PKCE 和 OIDC 签发，不能替代真实微信手机发码。

## 发布

已从 `main_temp` 发布 API 与 Web。集成源码：`6b176f6f141aa0db4755619375f5959e808c03f0`。发布版本：`wechat-four-digit-20261006-1`。归档 SHA-256：`48bf6c7598d71e1ee52022ff12b459103c31f935f013ff6fc658d34981fdc499`。


- API 镜像：`sha256:6c57ded0c96dcc4758ac0aa615610f4acbcc0b26d2fce3e6a13e2ae82798cd13`，健康且重启次数 0；Worker 原镜像保持健康。
- Migration 已到 `000073_registration_short_code_limits.sql`；此次最新基线同时应用 `000072_default_resource_seeds.sql`，启动记录 40 个默认资源种子。
- 数据库备份、保护环境/配置备份与归档校验通过，保留回退镜像和 Web 版本；公众号配置仍开启且 ready，版本 1。
- `scripts/configure-registration-identity.py check` 通过。真实 Keycloak 授权跳转公众号等待页，绑定浏览器的 status 返回字符串四位数字及普通关注二维码，未输出实际登录码。
- `/api/healthz`、`/api/readyz`、公开状态页与 Keycloak Discovery 返回 200；管理员 API 拒绝匿名请求，两个 Broker JWKS 为 RS256。
- Cookie 保持 Secure/HttpOnly/SameSite=Lax，其他浏览器 status 返回 410、错误 Origin 完成请求返回 403。
- 在线 index、主 bundle、i18n、client、账号管理 bundle 与本次生产构建逐字节一致；API 日志未发现 error/fatal/panic。

## 尚未验证

真实微信手机发送短码、首次/返回账号完整登录与真实飞书企业扫码仍需独立验收；本次未执行 Linux Runtime Production Conformance 或远端对象存储 Conformance。未记录登录码、Cookie、Callback Query、Token 或供应商凭据。
