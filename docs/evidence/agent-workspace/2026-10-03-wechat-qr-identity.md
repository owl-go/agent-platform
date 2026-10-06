# 微信扫码确认后无法完成账号接入 — 2026-10-03

> 公开副本中的部署地址与机器路径已通用化；示例值不代表验收目标。原有日期、结果、版本和证据边界保留。

## 复现

用户在手机微信完成扫码确认，网页的授权轮询仍持续返回 HTTP 422 `invalid_input`。未扫码的公网授权申请与连续四次等待轮询均为 200，排除了原有 30 秒请求期限问题。

使用线上相同 WeChat Adapter 建立临时诊断，凭证只在进程内保留。用户扫描诊断二维码后，实际腾讯接口返回：`get_qrcode_status` HTTP 200、`ret=0,status=confirmed`，Bot Token、Bot ID、User ID 均存在；随后的 `getconfig` HTTP 200、`ret=-4`，Adapter Identify 返回 `provider_identity_failed`。诊断未保存账号配置、Token 或完整响应，临时线上源码已删除。身份已经由可信 QR 确认，但平台额外要求会话配置接口成功，导致误判授权失败。

## 修复与检查

- WeChat QR 的成功响应携带服务端 `_platform_wechat_qr_account_id`，只进入临时授权和渠道凭证密文。QR Account Identify 校验完整凭证、匹配账号身份与可信 baseurl；不再调用会话配置接口。旧有无证明凭证继续使用原校验。
- 原始账号接入和原始渠道保存均拒绝客户端提供 `_platform_` 字段；只有已校验 scope 的服务端登录密文或保留的账号密文能携带证明。收发 Configure 和编辑保留凭证沿用同一身份，避免授权之后再次被错误接口拦截。
- `TestWeChatConfirmedQRDoesNotRequireConversationConfig` 在修复前失败：`official confirmation was rejected: resource is invalid: provider_identity_failed`；修复后通过。测试覆盖 Application → 真实 Adapter 的 QR 确认、加密保存、收发 Configure、保留凭证编辑，以及客户端伪造证明和身份替换拒绝。
- 覆盖失败的 QR 业务返回码、无证明旧凭证、恶意 API host 和调用者取消。保存保持 disabled/unverified，必须完成真实测试消息与原聊天回复后才能启用。
- 目标 Application/Adapter/HTTP Service 测试及三包 race 检查通过，`make test`、`make build`、`git diff --check` 通过。`main_temp` 集成后再次通过完整后端测试与构建。没有修改前端、Proto、Migration、Runtime 或 Sandbox。

## 发布

功能提交 `2d9c1ca` 经 `main_temp` 集成为 `a3d629f78dd7a2915f19ad3d7652c7898a2a6545`，发布标识 `wechat-qr-identity-20261003-1`。

候选 API 通过正常 OIDC Authorization Code + PKCE 登录验证：实际微信二维码申请为 200/waiting，连续两次轮询均为 200/waiting（约 20 秒），取消为 200；伪造登录/保存的服务端证明字段均为 422。候选只绑定回环地址，使用与正式 API 相同的网络、配置和环境；第一次候选未连接 edge 网络导致 OIDC DNS 失败，连接正确网络后检查通过，失败候选未上线。

发布前备份位于服务器 `/srv/agent-workspace/backups/pre-wechat-qr-identity-20261003-1`，业务 pgdump 经 `pg_restore -l` 和 SHA-256 清单校验。切换时无非终态 Run，停止 Worker 后重建 API/Worker 并分别等待 healthy，再原子切换源码指针。API 镜像 `sha256:6c2cf48d7a05535c01785f1e867e9c2264f503261ff1d945490464b1cc121393`，Worker 镜像 `sha256:ee46d30d0b185519f4697a37515d26b327b23063dd1b09948cee8224a209d0ee`。

公网 healthz/readyz 为 ok/ready；九个资源接口均为 200。公网微信二维码申请、两次等待轮询和取消均为 200，伪造登录/保存服务端证明均被 422 拒绝。两服务 healthy，启动 ERROR/FATAL/PANIC 为 0，四个消息渠道循环均 started=1、fatal=0；Migration ledger、配置 SHA-256 与 Web 发布指针保持一致。临时候选 API 和私有环境文件已清理；脱敏候选/公网/切换/后检查日志保存在服务器 `/srv/agent-workspace/evidence/wechat-qr-identity-20261003-1`。

## 验收边界

实际扫码诊断复现了“官方 confirmed 成功 → getconfig ret=-4 → 平台误判失败”。修复后的确认、保存、保留凭证编辑和收发 Configure 由使用同一真实 Adapter 的回归测试覆盖；新版本的真实扫码及完整 IM 收发仍需用户在原页面验证。旧 API 的临时授权随重启销毁，之前失败的确认未保存，需要重新生成二维码。没有新增 Runtime Digest 或 Linux/gVisor/Production Conformance 证据。
