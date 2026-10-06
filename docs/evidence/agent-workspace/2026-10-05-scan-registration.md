# 扫码登录与注册本地验收记录

日期：2026-10-05。开发分支：`codex/scan-registration`，基线：`origin/main` 的 `257ed16`。此记录只证明本地实现与下列验证结果；未发布到线上，未使用真实飞书企业应用或微信公众号完成扫码。

## 已执行

| 检查 | 结果与边界 |
|---|---|
| `pnpm --dir frontend install --frozen-lockfile` | 成功 |
| `make generate` | Proto、Wire、OpenAPI 和前端类型生成成功 |
| Account、Gateway、Workspace HTTP、配置与 Wiring 的目标包测试 | 通过；供应商请求使用 fake HTTP，不访问真实应用 |
| `go -C backend test -race ./internal/biz/account/... ./internal/data/account/registration/... ./internal/service/workspace/...` | 通过；覆盖版本/关闭、浏览器绑定、单次确认/兑换、关闭入口、跨企业拒绝、外部账号碰撞与禁用、微信公众号 URL 握手和加密事件分离 |
| PostgreSQL Registration Repository 的 `-race` 集成测试 | 通过；真实 PostgreSQL 17，完整 Migration 链，多个空邮箱 User、配置/Attempt 密文、审计与配置事务、CAS、并发单次消费、过期清理 |
| Keycloak 注册身份集成测试 | 通过；真实 Keycloak 26.7.1，admin-only marker、无邮箱账号、幂等联邦身份链接、禁用拒绝 |
| Keycloak OIDC handoff 集成测试 | 通过；fake 飞书上游身份，真实 Keycloak，两个 PKCE 环节、Broker RS256/JWKS、最终产品 ID Token 签发/验证和本地 User 投影 |
| `make test`、`make build` | 通过；未配置环境变量的其他外部集成测试会 Skip，不能算远端验证 |
| `pnpm --dir frontend test` | 55 个文件、593 个测试通过；公开注册页入口回归证明不启动产品 Router/OIDC |
| `make web-typecheck`、`make web-build` | 通过；构建仍有既有 embed chunk 大于 500 kB 提示 |
| `python3 scripts/configure-registration-identity_test.py` | 1 个测试通过；profile 收敛幂等、保留相邻属性/规则、不添加 manage-realm |
| Playwright CLI 实际浏览器检查 | 管理员配置页桌面/手机、公开二维码页桌面/手机、失败页桌面通过；手机宽 390 px 无横向溢出，公开页面停留在原路径；最终页面 console 无 error |
| 文档相对链接、`git diff --check`、`cmp -s AGENTS.md CLAUDE.md` | 通过 |

测试 PostgreSQL、Keycloak 均为本次新建的可删除 Docker 容器。Keycloak 使用本地缓存镜像 Digest：`quay.io/keycloak/keycloak@sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6`。Keycloak fixture 服务账号只具有 manage-users/view-users/manage-identity-providers；HTTP loopback 和手动传递 Secure Cookie 仅用于隔离集成环境，产品 Broker 配置要求 HTTPS。

重复集成测试需要先准备隔离 fixture，然后执行（敏感 DSN 不记录）：

```bash
go -C backend test -race ./internal/data/account/gormrepo -run Registration -v
go -C backend test -p 1 ./internal/data/account/keycloak ./internal/service/workspace \
  -run 'TestKeycloakRegistrationProvisioningIntegration|TestRegistrationKeycloakOIDCHandoffIntegration' -v
```

分别提供 `WORKSPACE_TEST_POSTGRES_DSN` 和 `WORKSPACE_TEST_KEYCLOAK_URL`。Keycloak fixture 约定见测试代码，不能指向生产 Realm。`-p 1` 避免两个包并发修改同一 fixture IdP。

浏览器中的二维码图片明确标记为 `QR layout fixture`，配置数据也是 fixture。截图只用于本地布局检查，位于忽略的 `output/playwright`，不作为真实扫码证据。临时预览入口不提交。

## 尚未验证

- 真实公众号的新关注、已有关注者 SCAN、服务器 URL 握手、IP 白名单和安全模式回调。
- 真实飞书企业自建应用发布、权限范围、扫码授权、外企业拒绝。
- 线上 HTTPS 网关、Keycloak Theme 展示、API 多副本 Key 与配置版本一致性、管理员密码入口及真实账号禁用。
- 本次没有执行 `main_temp` 集成或线上发布；上线必须遵循仓库发布规则并完成以上验收。
- Runtime 镜像、Linux + runsc Sandbox/Production Conformance 未执行；本次未修改这些模块。对象存储真实环境门禁不属于本次验收。

配置与真实验收步骤见 [扫码注册技术说明](../../technical/scan-registration.md)。
