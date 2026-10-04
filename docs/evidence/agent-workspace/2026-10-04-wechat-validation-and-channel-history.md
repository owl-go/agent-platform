# 微信消息轮询与渠道运行记录 — 2026-10-04

## 复现和修复

已保存微信账号在接收验证阶段显示连接异常，截止时间显示 `Invalid Date`。只读诊断使用当前账号密文和同一真实 WeChat Adapter，凭证只在进程内解密；仅记录响应字段名、HTTP 状态、数值错误码和消息数量，不记录账号、Token、Cursor、验证码或聊天内容。

真实 `getupdates` 响应为 HTTP 200，字段只有 `get_updates_buf`、`msgs`、`sync_buf`，消息数量为 0；不存在 `ret`/`errcode`。旧 Adapter 返回 `provider_receive_failed`。使用相同请求和账号、仅替换修复后的 Adapter，响应被正常接受，并到达 Cursor Save；诊断主动拒绝写 Cursor，未确认消费消息、未保存 Inbox、未创建 Run 或发送聊天。腾讯公开协议与客户端也允许 getupdates/sendmessage 成功时省略返回码。

回归测试在修复前复现了遗漏返回码导致正常入站和发送失败，且显式字符串或 null 返回码可能被误当成 0。修复仅对消息收发允许省略成功码；显式 ret/errcode 必须是数值 0，非零、类型错误、null 响应、HTTP 拒绝仍失败。QR 可信身份、入站账号匹配、受众控制、Secret 脱敏和整个批次提交后推进加密 Cursor 的边界保持生效。

`validation_until` 从接口返回 Protobuf Timestamp 对象，前端原来只转换 `_at` 字段。API Client 回归测试在修复前失败，修复后将验证截止时间转换为 ISO 字符串，并能显示本地时间。

## 配置与 Run History

- 已保存账号的 provider 入口及账号卡片显示“已配置”，provider 入口打开现有账号配置；渠道配置暂时移除“回复记录”入口和面板。账号授权、验证与启用仍分别依据实际状态。
- 微信等仅支持私聊的渠道使用私聊验证文案，不再提示群聊 @。
- 每条通过准入的渠道问题在 Workflow Queue 创建一个 Run；渠道对话在运行列表逐个 Turn 展示，连续追问不会合并成一行。打开某一 Turn 仍读取根 Run Conversation 的完整上下文，取消与重新运行针对选中的渠道 Run。
- Run 保留具体 provider 与当时的渠道名称，触发方式例如“微信 · 客服机器人”。Migration `000068_message_channel_run_origin.sql` 回填已有 Run 的 provider，新准入在同一事务写入该字段；渠道改名与软删除不改写历史。
- 临时 PostgreSQL 17 上实际执行完整 Migration 链和渠道集成测试，验证三条问题产生三条历史记录、连续上下文隔离、渠道改名/删除后来源仍保留。非渠道对话继续使用原有最近 Turn 汇总。
- 验证文案属于收发探测，不创建模型 Run；未通过准入的问题和重复投递也不伪造运行记录。

## 检查

目标 Adapter/Application/Repository/HTTP Service 测试、四包 race 检查、`make test`、`make build`、`make breaking`、`make web-typecheck`、`make web-build`、`git diff --check` 均实际通过。前端 52 个测试文件、568 项测试通过，包括全部 13 个 provider 的运行来源、已有响应兼容、选中渠道 Turn 的重新运行和取消。`main_temp` 集成后再次执行完整后端测试与构建、前端完整测试与构建，并通过 `make verify-generated`。

## 发布和回退

功能提交 `d791fbb`，经 `main_temp` 集成为 `acf057422853664c8bdb62ab43729e99683f9d58`。发布标识 `wechat-validation-history-20261004-1`，API/Worker 源码与 Web 均使用该发布标识。

发布前备份 `/opt/agent-platform/backups/pre-wechat-validation-history-20261004-1`：业务 pgdump 经 `pg_restore -l` 和 SHA-256 校验，保存配置、旧源码/Web 指针、旧镜像及 Migration ledger。只发布 API、Worker、Web，不重建 Runtime、CLI Builder 或其他基础服务。回退使用备份中的旧源码、旧 Web 和 API/Worker 镜像；新增 Migration 是向后兼容的追加字段，不删除历史数据。

候选 API 仅绑定回环地址，使用现有控制与公网网络，完成正常 OIDC Authorization Code + PKCE 登录。九个资源接口均 200，已有 Workflow 的渠道列表与运行记录接口均 200。正式切换前无非终态 Run，停止 Worker 后重建 API 和 Worker并分别等待 healthy。切换阶段回填候选迁移与旧 Worker 停止之间可能留下的空 provider。

正式 API 镜像 `sha256:c6ea36361c652965467728218705909e9b466e277ab6a4b5abcf4d24cef60dab`，Worker 镜像 `sha256:b641a3713a66007b8a994b1c10bd4529982fcad193eea1226dca486ff0df0db9`。公网 healthz/readyz 为 ok/ready，正式九个资源接口及渠道/运行记录接口均为 200。两个服务 healthy，启动 ERROR/FATAL/PANIC 为 0；四个消息渠道循环 started=1、fatal=0。全部既有 Migration checksum 不变，000068 的 checksum 与发布源码匹配，渠道 Run 缺失 provider 数为 0。API/Worker 配置 SHA-256 保持不变。

公网 HTML 与生产构建字节相同，SHA-256 `9fc5ff2ad2e34d70fd43032ff31669bf55c7249a6e0177f86d564866d275183a`；30 个入口资源 SHA-256 一致。源码与 Web 指针均指向新发布。候选 API、候选私有环境文件、线上诊断源码及临时 overlay 已清理。

## 验收边界

真实已保存微信账号的空消息长轮询在修复后被正常接受。发布后 owner 刷新页面、重新发起验证并从手机微信发送新验证文案，明确确认已收到机器人回复。数据库核对该账号 validation_state=passed、health=connected、error_code 为空，validation 类型的已发送回复数为 1；真实验证消息接收和原聊天回复已经完成。该账号仍保持未启用，启用由 owner 在正常产品界面控制。

正式账号当时没有渠道 Run，公网检查不伪造模型执行记录；正常问题的模型执行和运行来源以 PostgreSQL 集成及前端测试为证，尚无该正式账号的模型 Run 验收。没有新增 Runtime Digest、Linux/gVisor 或 Production Conformance 证据。
