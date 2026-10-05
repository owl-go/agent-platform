# 扫码注册发布验证 — 2026-10-05

## 发布标识与范围

- 功能分支：`codex/scan-registration`；实现提交 `e41fc2c`，迁移顺序修正 `2ec6730`。
- 通过 `main_temp` 集成，发布构建源：`d3d3d99345b58560260abb41ea176a47237b94b9`，发布前已推送。
- 发布 ID：`scan-registration-20261005-1`。
- 公网：[Agent Workspace](https://47-237-108-63.sslip.io)。
- 源码：`/opt/agent-platform/src.release-scan-registration-20261005-1`。
- Web：`/opt/agent-platform/web/releases/scan-registration-20261005-1`。
- Keycloak Theme：`/opt/agent-platform/identity-themes/releases/scan-registration-20261005-1`。
- 备份：`/opt/agent-platform/backups/pre-scan-registration-20261005-1`。

发布 API、Worker、Web 和登录主题；Keycloak 保留原镜像，仅重建容器以加载新的只读主题目录。Runtime、CLI Builder、Egress Controller、Caddy、PostgreSQL、MinIO 的镜像和运行配置没有更新。两种 Registration Method 尚未开放，需产品 Administrator 填写真实应用凭证后启用。

集成前线上已有 `000069_message_channel_responses.sql`，因此本次未部署过的注册 Migration 顺延为 `000070_scan_registration.sql`，保留此前 Migration 字节。

## 实际门禁与部署检查

集成分支执行 `pnpm --dir frontend install --frozen-lockfile`、`make test`、`make build`、`pnpm --dir frontend test`、`make web-typecheck`、`make web-build`、`git diff --check` 和 `cmp -s AGENTS.md CLAUDE.md`，全部通过。前端为 55 个文件、602 项测试。生产 Web 使用远端公开 OIDC 配置重新构建。构建仍有既有 embed chunk 大于 500 kB 提示。

真实隔离 PostgreSQL 17 执行 Registration Repository 的 `-race` 集成测试通过，完整 Migration 链包括集成分支的消息渠道迁移和 `000070`。此前 Keycloak/供应商 fake 边界见 [本地验收记录](2026-10-05-scan-registration.md)。其余环境缺失的集成 Skip 不计作远端验证。

远端备份包含业务库及身份库 `pg_dump -Fc`、原 env/YAML、原源码/Web/Theme 指针、旧 API/Worker 镜像 ID、原 User Profile 和服务账号角色映射。两个 Dump 均通过 `pg_restore -l`，所有备份通过 SHA-256 校验；备份目录为私密目录，旧服务镜像保留 `pre-scan-registration-20261005-1` 标签。

Broker client secret 和 RSA 2048 PKCS8 签名 Key 在主机上生成，只写入已有保护 env；YAML 添加可选 Broker 引用并保留原权限/属主。执行 `configure-registration-identity.py apply` 与 `check`，声明 admin-only 身份 marker、使 email 可选，并补齐窄服务角色。重建 Keycloak 后再次 `check` 通过。未配置任何供应商 App Secret，也未修改现有 User、客户端登录策略或产品密码。

服务构建前与切换前检查非终态 Run、活跃 Session Message 汇总均为 0。持有主机发布锁，构建 API/Worker 后停止旧 Worker，启动 API 应用 Migration，确认账本最新为 `000070_scan_registration.sql`，再启动 Worker。两者 healthy 后更新源码指针，加载新 Theme，待 OIDC 恢复后完成 Web 原子切换。

| 服务 | 镜像 ID | 验证 |
|---|---|---|
| API | `sha256:b648c68675bd6b601c3eab747a495bb31ec0077e8587f1526a849ddc90005e67` | healthy、restart=0、ERROR/FATAL/PANIC=0 |
| Worker | `sha256:5a8dabcac731a7b5e596d061ceff08a4a7458c3387de67863e2ed2af5fe6101a` | healthy、restart=0、ERROR/FATAL/PANIC=0 |
| Keycloak | `sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6` | running、restart=0、Discovery=200 |

## 公网与浏览器

- 首页、Health、Readiness、OIDC Discovery、公开失败/扫码页面均 HTTP 200；HTTP → HTTPS 为 308。
- 两种 Broker JWKS 均返回 RS256 公钥；匿名管理员配置接口和未知 provider 均 401。关闭的方法拒绝有效形状的 authorize 请求，数据库 enabled 方法数为 0。
- 实际匿名浏览器从首页自动进入中文 Keycloak 登录表单，原生 username/password 输入框各 1 个，`lang=zh-Hans`。
- 公开二维码页使用无效 Attempt 检查真实过期状态及返回登录入口，不触发 OIDC 跳转；公开失败页保持原路径。390 px 手机无横向溢出，最终 console error/warning 均 0。桌面密码登录表单与手机恢复页面截图已检查。
- 公网 HTML、入口 JS、账号管理页 JS、i18n JS 与本地生产构建逐字节相同；Theme JS 与已校验 Manifest 的上传文件逐字节相同。

| 产物 | SHA-256 |
|---|---|
| `index.html` | `b0e35e9f9c1aa5899813424b040d2120da0e06c4dad11e2eec7900326dd43e30` |
| `UsersPage-Btil_vw8.js` | `8cd8f7bc010ea1bfb1d1e591fab91dbf639403e53a0cd5627257e2b390d097fc` |
| `i18n-SqepmtvC.js` | `c9a6b4f1e04c25431e5691eb6c228930debf04526ca4b723615c3475c2dfe097` |
| `index-B6fZtVXa.js` | `8b3f7d13b24920d8fbe2ef00b3a412322c3b74bd296c3e55840da9fcba219842` |
| `registration-methods.js` | `d47f56a40f08bab00f8b7ecbaf7117116ae42f7390041f912f6947d3f194a55a` |

本机发布日志与公开产物记录为 `/tmp/aw-scan-registration-release.log`、`/tmp/aw-scan-registration-web-release.log`、`/tmp/aw-scan-registration-public-evidence.json`。临时发布脚本和忽略目录中的截图不提交；记录不包含登录 Cookie、授权 query、Token 或配置秘密。

## 验证边界和回退

本次证明发布、迁移、身份属性/角色准备、静态产物一致性、匿名密码登录入口和恢复页面可用。未用生产用户密码登录管理员后台，未进行真实飞书企业扫码、微信公众号新关注/已关注 SCAN、服务器 URL 握手或供应商应用权限验收；开放前仍需完成 [供应商配置和验收](../../technical/scan-registration.md)。

在目标 Linux 执行 `make production-conformance-preflight`，因缺少专用 Git Fixture、Work/Evidence Root、Sandbox 探测配置、五个 Runtime 模型凭证和专用对象存储配置而失败；没有执行完整 Production/Sandbox Conformance，也未新增或开启 Runtime Capability。

前一源码和 Web 为 `wecom-streaming-replies-20261004-1`；前一 Theme 为 `login-theme-da5b8c5f86fc`。回退先关闭 Registration Methods，再评估迁移后的 schema 兼容性，使用备份中的 API/Worker 镜像、env/YAML 和源码指针。恢复 Theme 指针需重新创建身份容器；恢复原 User Profile/服务角色需使用保护备份和部署管理员权限，不撤销其他并发授权。Web 可使用 `scripts/deploy-web.sh activate wecom-streaming-replies-20261004-1`。不得删除持久卷，不能盲目用旧二进制覆盖迁移后的数据库。

## 保存请求 HTTP 400 修复发布

同日管理员报告保存失败。线上微信配置 PUT 在约 2 ms 内返回 400，配置表没有写入记录。Web 的 `updateRegistrationMethod` 直接发送字符串 body，遗漏 JSON Content-Type，浏览器因此使用 `text/plain;charset=UTF-8`。真实生成的 Kratos HTTP Router 测试确认该类型返回 400 且未进入保存用例；相同 JSON body 使用 `application/json` 则返回 200 并正确绑定 provider 和字段。这解释了本次保存失败，尚不能证明真实供应商凭证有效。

新增微信和飞书请求回归测试，修复前两项均以实际 Request 的 `text/plain;charset=UTF-8` 失败；改用既有统一 JSON 请求构造器后通过。功能分支提交 `72e10c6`，从 `main_temp` 的 `004cffc66d45a93d3579c14439efe1b0cc710740` 发布。目标 API/管理员组件测试 44 项通过，功能分支完整前端 595 项、集成分支完整前端 604 项通过；两处分支均执行 Web typecheck、生产构建、`go -C backend test ./internal/service/workspace/...`、`git diff --check` 和指引文件一致性检查。生产构建仍有既有 embed chunk 大小提示。

持有主机发布锁，使用 `scripts/deploy-web.sh` 发布 `scan-registration-save-20261005-1`；仅切换 Web，API/Worker 镜像、数据库及身份配置未更新。旧 Web 保留，切换前指针和入口文件备份于 `/opt/agent-platform/backups/pre-scan-registration-save-20261005-1`。当前 `/opt/agent-platform/web/current` 指向新 release，API/Worker 保持 healthy。

公网 HTML、API client、账号管理页及入口 JS 与生产构建逐字节相同；Health、Readiness、公开失败页和 OIDC Discovery 均 200，匿名管理员 API 为 401。产物 SHA-256：

| 产物 | SHA-256 |
|---|---|
| `index.html` | `32e145d6b55f595a3aeb60dab2060974064ccc5fe705dd210343acff297b768e` |
| `client-tdIlFTlL.js` | `a79232a102adcdd5d35a3bf1fff52121dfc1c19ee5cc8f7f951003abbd850865` |
| `UsersPage-ITNWFshb.js` | `1efb27706532d2b008173913a4751e3c65a9263c6d1dd02f79c684acb7f1e95c` |
| `index-EUb5SjPG.js` | `10c69e2f28d6dc965c9cd0b7a754ac4568c7178242fa4ce016dd13ea0a99bfb9` |

发布日志与公网校验记录为 `/tmp/aw-registration-save-web-release.log`、`/tmp/aw-registration-save-public-evidence.json`。未使用生产管理员凭证提交真实注册配置，供应商校验、真实扫码和微信服务器握手仍待验收；本次没有执行完整 Runtime/数据库集成门禁。Web 可单独回退到 `scan-registration-20261005-1`，不涉及数据库回退。
