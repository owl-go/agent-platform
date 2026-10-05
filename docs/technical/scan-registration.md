# 扫码登录与注册

实现日期：2026-10-05。本地 PostgreSQL 17 和 Keycloak 26.7.1 的验证记录见 [验收证据](../evidence/agent-workspace/2026-10-05-scan-registration.md)。已从 `main_temp` 发布，见 [发布验证](../evidence/agent-workspace/2026-10-05-scan-registration-deployment.md)。真实飞书企业应用和微信公众号关注/扫码尚未验收，两种方法保持关闭。

## 产品流程

产品浏览器继续自动跳转 Keycloak。账号管理 → 注册方式提供两个独立开关；关闭默认扫码入口，管理员密码入口持续可用。开放入口同时允许首次注册与已有外部账号登录，关闭不会结束已存在的产品会话。

- 微信公众号：服务端申请 300 秒 `QR_STR_SCENE` 带参数二维码，浏览器进入公开二维码页。服务器配置的 GET 握手仅校验 Token/timestamp/nonce 的 `signature` 并回显 `echostr`，不确认身份。安全模式 `subscribe` 中的 `qrscene_` 或已关注者的 `SCAN` 事件必须通过签名、AES-CBC、App ID、原始 ID、时间和 ticket 校验。回调只确认 Attempt；绑定浏览器轮询后通过同源 POST 完成建号和登录。普通关注、不匹配的二维码或明文回调不能注册。
- 飞书：跳转官方网页授权扫码页，以随机 State 和 HttpOnly 浏览器 Cookie 绑定返回。服务器使用授权码取得 User Token，然后获取 User 信息，要求其 `tenant_key` 与 App ID/App Secret 查询到的企业一致。供应商 User Token 只在当前调用内存在。

首次使用创建无密码、无邮箱的普通 Keycloak User 和本地 User/Personal Settings 投影，并绑定固定 `aw-feishu` 或 `aw-wechat_official` 联邦身份。返回用户重用同一投影；无治理角色默认值，不按邮箱或名称合并现有账号。外部身份 Subject 是 method/App ID/原始身份的 SHA-256，更换 App ID 会产生新的身份命名空间。

## 配置与部署

先按仓库规则在 `main_temp` 完成集成和发布检查。运行服务需要三项可选的私密主机环境变量；三项全空保持现有部署行为，部分配置或非 HTTPS origin 会拒绝启动：

| 变量 | 要求 |
|---|---|
| `REGISTRATION_PUBLIC_URL` | 产品的 HTTPS origin，无路径、userinfo、query 或 fragment；与产品 Web 页面共用该 origin |
| `REGISTRATION_CLIENT_SECRET` | 至少 32 字符随机秘密，仅用于 Keycloak 与 Broker 之间的 confidential client 认证 |
| `REGISTRATION_SIGNING_KEY` | 至少 RSA 2048、PKCS8 DER 的标准 Base64；各 API 副本共用同一私密 Key |

私密环境文件中生成凭证，勿将输出加入日志或仓库。示例（在私密目录、`umask 077` 下执行）：

```bash
openssl rand -base64 48 > registration-client-secret.txt
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out registration-signing.pem
openssl pkcs8 -topk8 -nocrypt -in registration-signing.pem -outform DER -out registration-signing.der
openssl base64 -A -in registration-signing.der -out registration-signing-key.txt
```

将秘密值写入已有私密主机 env 并提供给 API/Worker，重新构建 API；部署配置示例和 Compose 已包含可选字段。公钥 `kid` 从公钥摘要派生。Key 轮换必须同步全部 API 副本；client secret 轮换后，管理员需要重新保存两种方法以同步 Keycloak。

部署管理员使用现有 `VITE_OIDC_AUTHORITY`、`KEYCLOAK_ADMIN_USER`、`KEYCLOAK_ADMIN_PASSWORD`、`KEYCLOAK_SERVICE_CLIENT_ID` 的保护环境执行：

```bash
python3 scripts/configure-registration-identity.py apply
python3 scripts/configure-registration-identity.py check
```

该幂等脚本只新增/收敛 `aw_registration_subject`、`aw_registration_provider` 为 admin-only profile 属性，取消 email 的 required 条件，保留其他资料规则和 email 格式校验，并为既有服务账号补齐 `manage-users`、`view-users`、`manage-identity-providers`。不改变 password registration、其他客户端、用户、会话或 unmanaged attribute policy，不授予 manage-realm。产品手动建号仍验证 username/display name/email。新建 Realm 和既有 Realm 都需要这一步；不能仅依赖 Realm import。

构建并发布本仓库的 `agent-workspace` Keycloak Theme，以得到中文/英文扫码入口标签。产品管理员登录后填写配置：

| 方法 | 管理员填写 | 供应商后台前置条件 |
|---|---|---|
| 飞书 | App ID、App Secret | 企业自建网页应用；发布 `contact:user.base:readonly` 和 `tenant:tenant:readonly` 权限，授权范围包括目标员工；登记表单显示的 callback URL 为重定向 URL |
| 微信公众号 | App ID、App Secret、公众号原始 ID（`gh_...`）、回调 Token、43 字符 EncodingAESKey | 账号具有带参数二维码及 stable token API 权限；按公众平台要求配置服务器 IP 白名单；callback URL 作为服务器 URL，启用安全模式；关注和扫码事件交给该接口 |

微信原始 ID 与 App ID 是两个不同字段，不能互换；加密事件的尾部 App ID 和解密消息的 ToUserName 都必须匹配。已有公众号业务若使用同一回调入口，需要先评估消息路由，不能用其他渠道凭证替代这里的配置。开放方法的保存前验证应用凭证；凭证或权限验证失败不改动已存配置。

## 接口与状态

普通 JSON API 的契约位于 `backend/api/workspace/v1/workspace.proto`：

- `GET /api/v1/admin/registration-methods`：Administrator 专用，返回可用性和两种方法的配置投影，秘密只返回 configured 标记。
- `PUT /api/v1/admin/registration-methods/{provider}`：Administrator、expected_version 和 reason 必需；JSON 请求须声明 `Content-Type: application/json`，Web 使用统一 JSON 请求构造器。浏览器字符串请求体默认的 `text/plain` 会在 HTTP 解码阶段返回 400，尚未进入凭证验证或写库。配置与 Governance Audit Event 同事务提交；供应商和 Keycloak HTTP 在事务外运行。

配置先提交 `ready=false`，再同步唯一平台 IdP，最后按版本标记 Ready。Keycloak 故障或并发变更保留 pending 版本，Broker 拒绝开始/确认/兑换；管理员刷新后重新保存。关闭配置立即在 Broker 生效，即使 Keycloak 页面暂时保留旧入口也不能认证。空密钥只在 App ID 不变时保留旧值。

公开协议入口仅允许已知 provider 下的 `authorize`、`callback`、`status`、`complete`、`token` 和 `jwks` 的指定 HTTP Method；不绕过管理员 API 或普通产品认证。`/register/wechat` 与 `/register/failed` 是公开 Web 状态页，不初始化产品会话。

Attempt 的 State、Nonce、浏览器 hash、ticket hash、身份和 code hash 存入加密 payload。Cookie 使用 Secure/HttpOnly/SameSite=Lax；确认还要求同源 Origin 和自定义请求头。Attempt 5 分钟有效，code 一分钟有效；改变配置、关闭方法、过期或 CAS 失配均拒绝。创建新 Attempt 时删除过期记录，最多保留 10,000 个未清理记录；API 每副本最多开始 120 个 Attempt/分钟，供应商调用使用有限超时。

Broker 只接受固定 client ID、client secret、该 provider 的精确 Keycloak 回调，以及 S256 PKCE。其 ID Token 使用 RS256/JWKS、固定 issuer/audience、原请求 nonce 和一分钟 expiry；opaque access_token 无产品或供应商权限，不支持 refresh/token exchange。产品认证仍只验证 Keycloak issuer。不得在日志/证据中记录原始 callback query、Token、二维码 ticket、Cookie 或配置秘密。

## 验证边界

本地 fake Gateway 覆盖真实供应商协议形状和拒绝条件；Keycloak 集成以 fake 上游身份跑实际联邦身份、PKCE、RS256/JWKS、无邮箱建号和最终产品 OIDC 签发，不能记作真实飞书扫码。

真实验收需分别完成：公众号新关注者与已有关注者、回调服务器验证/安全模式、拒绝伪造事件、飞书应用真实授权及外企业拒绝、后台关闭入口和已禁用用户拒绝、管理员密码入口、API 副本共享 Key 和版本一致性。未经这些验收，不记录为线上可用。

官方协议依据：[飞书 User Token](https://open.feishu.cn/document/authentication-management/access-token/get-user-access-token)、[飞书公司信息](https://open.feishu.cn/document/server-docs/tenant-v2/tenant/query)、[微信公众号带参数二维码](https://developers.weixin.qq.com/doc/offiaccount/Account_Management/Generating_a_Parametric_QR_Code.html)、[微信公众号接入](https://developers.weixin.qq.com/doc/offiaccount/Basic_Information/Access_Overview.html)、[EasyWeChat 接入握手实现](https://github.com/w7corp/easywechat/blob/6.x/src/OfficialAccount/Server.php)、[微信公众号消息加解密](https://developers.weixin.qq.com/doc/offiaccount/Message_Management/Message_encryption_and_decryption_instructions.html)。
