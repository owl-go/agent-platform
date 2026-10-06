# 微信公众号一次性登录码验证

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

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

## main_temp 与生产发布

功能提交 `4f3ebc0`，集成构建源 `a5168779fafa6b9575372d31fd8c182d2e60259d`；发布前均已推送。从 `main_temp` 发布 `wechat-message-code-20261006-1`，仅更新 API 和 Web。Worker、Keycloak 镜像、主题和配置保持原值；公众号 enabled=true、ready=true、version=1，未重新保存或轮换凭证。

集成分支执行 `make test`、`make build`、前端完整测试（55 文件、608 项）、`make web-typecheck`、`make web-build`、`git diff --check` 和指引文件一致性检查，全部通过。临时 PostgreSQL 17 再次实际执行含 `000069`、`000070`、`000071` 的完整迁移链及 Repository race 测试；真实临时 Keycloak 再次执行微信/飞书 OIDC 交接 race 测试通过。

持有发布锁，备份业务库 `pg_dump -Fc`、env/YAML、旧源码/Web 指针和 API/Worker 镜像 ID 到 `/srv/agent-workspace/backups/pre-wechat-message-code-20261006-1`。Dump 通过 `pg_restore -l`，全部备份通过 SHA-256 校验；切换前活跃 Run/Session Message 汇总为 0。源码归档本地与主机 SHA-256 均为 `326a4e82f508bb85c02288a4fafcd1222e665863e77df123173adb4e8351cc28`。

API healthy 后确认线上迁移账本为 `000071_registration_login_codes.sql`，源码指向 `/srv/agent-workspace/src.release-wechat-message-code-20261006-1`。身份 Profile 和窄服务角色只执行 `configure-registration-identity.py check`，通过。Web 经 `scripts/deploy-web.sh` 使用生产公开 OIDC 参数构建并原子切换到 `/srv/agent-workspace/web/releases/wechat-message-code-20261006-1`。

| 服务 | 镜像 ID | 状态 |
|---|---|---|
| API | `sha256:f143ad840e52288221615993e080e700d1e652f43c8b80ed1bca1e3641e1a3f7` | healthy、restart=0，切换后错误日志计数 0 |
| Worker | `sha256:5a8dabcac731a7b5e596d061ceff08a4a7458c3387de67863e2ed2af5fe6101a` | healthy、restart=0，未重建 |
| Keycloak | `sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6` | running、restart=0，未重建 |

公网 Health、Readiness、失败/扫码页面和 OIDC Discovery 均 200，两种 Broker JWKS 为 RS256；匿名管理员接口为 401。真实 Keycloak 的 `aw-wechat_official` 授权入口经重定向到网页后，绑定 Cookie 的 status 返回等待状态、普通关注二维码和符合格式的登录码；Cookie 为 Secure/HttpOnly/SameSite=Lax，status 为 no-store。另一浏览器读取该 Attempt 为 410，错误 Origin 完成请求为 403。检查只创建短期等待 Attempt，没有发送伪造的生产确认消息、创建生产 User 或兑换产品 Token；未记录登录码、Cookie 或授权 query。

公网 HTML、入口、账号管理、API client 与 i18n JS 与本地产物逐字节一致：

| 产物 | SHA-256 |
|---|---|
| `index.html` | `8e4e0b5cfc3a11b25991738fdadb7db070162fec68ad52173a3b720123339643` |
| `UsersPage-VR5_vaWY.js` | `42e183e2611bb732e94430bdf81d4dd82a801b37854e00e176ae9e0f0e4ab5cd` |
| `client-tdIlFTlL.js` | `a79232a102adcdd5d35a3bf1fff52121dfc1c19ee5cc8f7f951003abbd850865` |
| `i18n-CDejXTno.js` | `a066810577a8319cea554a2555edc10bb56bea1268194c075578f5ba50182cbf` |
| `index-Cvn_moe8.js` | `a8e801e057b49000d1d4497389f2ef2767356f18b2e7abda14c4e622607472e1` |

本机发布日志及公开验证记录为 `/tmp/aw-wechat-message-code-api-release.log`、`/tmp/aw-wechat-message-code-web-release.log`、`/tmp/aw-wechat-message-code-public-evidence.json`。本次证明发布与网页登录挑战可用，真实手机端发送、被动回复及首次/返回产品登录仍待验收。

## 回退

旧源码为 `scan-registration-20261005-1`，旧 Web 为 `scan-registration-save-20261005-1`。`000071` 仅添加可空 hash 列和约束，旧二进制不使用该列。保留旧镜像标签 `agent-platform-api:pre-wechat-message-code-20261006-1` 和备份；回退需在发布锁下使用源码目录中的 `compose.rollback-message-code.json` 恢复旧 API，并恢复源码/Web 指针。Web 可用 `scripts/deploy-web.sh activate scan-registration-save-20261005-1`。旧微信流程仍受带参数二维码 48001 限制，回退不会解决供应商权限问题。不要删除持久卷或用旧库 Dump 覆盖之后发生的业务数据。
