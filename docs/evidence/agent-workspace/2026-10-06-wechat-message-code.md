# 微信公众号一次性登录码验证

日期：2026-10-06。用户批准将带参数二维码改为普通关注二维码加一次性登录码；行为依据见 [ADR 0044](../../adr/0044-wechat-registration-with-message-code.md) 和 [扫码注册规格](../../technical/scan-registration.md)。

## 本地验证

- 账号 Application、供应商 Adapter 和 HTTP Broker 的 race 测试通过：覆盖五分钟过期、配置版本变化、关闭入口、错误浏览器、伪造签名、不同发送者并发竞争、同一发送者并发重试、关注/SCAN 不确认身份，以及安全模式文本回调和加密被动回复。
- PostgreSQL 17 临时数据库实际执行 Migration 和 Repository race 测试：登录码仅以 SHA-256 索引和加密 payload 保存，唯一约束、查询、过期清理及单次兑换通过；没有将环境缺失的 Skip 记为数据库验证。
- 与生产相同版本的临时 Keycloak 26.7.1（Digest `sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6`）实际执行飞书和微信公众号 OIDC 交接 race 测试。上游身份是 fixture；微信公众号 fixture 通过加密文本回调确认，然后经 Cookie、Origin、PKCE、Broker RS256/JWKS 和 Keycloak 最终签发完成普通 User 投影。这不是实际微信手机或飞书企业应用验收。
- `make test`、`make build`、`make web-typecheck`、`make web-build` 通过。功能分支前端完整测试 55 个文件、599 项通过；生产构建仍有既有的大于 500 kB chunk 提示。
- 浏览器在 390 × 844 视口检查中英文页面，普通公众号二维码实际加载，登录码与复制操作可见，无横向溢出；截图使用 fixture 登录码，不含生产登录码或凭证。
- `git diff --check` 和 `cmp -s AGENTS.md CLAUDE.md` 通过。

## 验收边界

本改动不改变 Runtime、Sandbox、对象存储或 Capability，未运行对应 Production Conformance。真实公众号的安全模式配置与 URL 握手已有上一轮证据；新流程仍需真实新关注者/已有关注者发送自己的登录码，验证被动回复、首次注册和返回登录。真实飞书企业应用验收仍未完成。

发布到 `main_temp` 和生产后的结果另行补充，不能将本地 fixture 的通过视为生产手机验收。
