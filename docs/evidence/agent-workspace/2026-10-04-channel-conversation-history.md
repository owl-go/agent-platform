# 渠道连续消息共用一个会话入口 — 2026-10-04

## 复现与原因

owner 反馈同一微信用户的消息在 Workflow Run History 显示多个入口。只读生产核查发现当时八个 Run 已有同一个 conversation_id、连续的 turn_number、一个入站 sender 和 chat；会话及上下文没有拆分。原因是列表汇总对 message_channel 触发绕过了 Run Conversation 汇总，将每个执行 Turn 直接显示为一行。

修复前实际执行 `go -C backend test ./internal/data/workspace/gormrepo -run TestSummarizeChannelRunConversationsKeepsOneEntryPerParticipant -count=1`，同一参与者两轮、另一参与者一轮的测试失败：got 3, want 2。前端回归也在修复前失败：汇总行的重新运行和取消错误地使用已终态的根 Run，而不是最新/活动 Turn。正常 owner OIDC Authorization Code + PKCE 登录的公网复现随后返回九条渠道历史行，期望一条时失败；只输出状态与数量，不保存账号、Token 或聊天内容。

## 最小修复与验证

列表统一以根 Run 为稳定身份汇总 Run Conversation，使用最新一轮的状态、时间、turn_number 和队列位置。具体 provider 与渠道名称保持根 Run 的冻结来源；入站身份隔离、会话映射、Workspace、Snapshot、Credits、发送与每轮不可变记录没有改变。列表取消通过 turns 接口定位活动 Run，重新运行使用最新 Run；打开会话按原有顺序展示全部问答。不同参与者仍独立，显式重置仍产生新会话。

目标汇总测试、整个 WorkflowDetailPage 测试通过。临时 PostgreSQL 17 实际执行完整 Migration 链及 `TestChannel|TestSummarize`：两位参与者的三轮执行汇总成两个入口，三个 Turn 完整保留，第二轮使用之前的答案，另一位参与者不继承其问题，渠道改名/删除后来源仍保留。`make test`、`make build`、`make web-typecheck`、`make web-build`、`git diff --check` 实际通过；前端 52 个文件、568 项测试通过。`main_temp` 合并后重复完整后端测试/构建和前端测试/typecheck/构建，均通过。临时 PostgreSQL 容器已移除。

## 发布与真实历史验收

功能提交 `34497d0`，通过 `main_temp` 集成为 `3bc9f02d01736cd86c8618d6e5c45147d3bcd559` 并推送，发布标识 `channel-conversation-20261004-1`。只更新 API 和 Web；Worker 未重建或停止，镜像保持 `sha256:b641a3713a66007b8a994b1c10bd4529982fcad193eea1226dca486ff0df0db9`。API 镜像为 `sha256:916fd12d691dc881669a08518cad636c8f58c6363ba84bca869f08ff00ae7d18`，两个服务均 healthy，API 启动 ERROR/FATAL/PANIC 行数为零。

发布前在 `/opt/agent-platform/backups/pre-channel-conversation-20261004-1` 保存业务 pgdump、配置、旧源码/Web 指针、API/Worker 镜像与 Migration ledger；pgdump 通过 `pg_restore -l`，pgdump/config 通过 SHA-256 校验。配置 SHA-256 与全部 Migration name/checksum 在切换后保持一致，没有新 Migration 或历史数据合并/删除。回退可使用保存的旧 API 镜像和源码/Web 指针。

候选 API 仅监听回环端口，使用现有 owner 的正常 OIDC 登录核对真实数据；随后同一核查通过公网正式接口完成：history_rows=1、conversation_roots=1、retained_turns=9、latest_turn=9、provider=wechat、enabled=true、health=connected。全部九轮仍可通过 turns 接口按同一 conversation_id 读取。生产 HTML SHA-256 为 `62799bb3e81f305fa77c180386678936177b9e4143aab935920150469dc654ce`，与生产构建相同；30 个入口资源逐个 SHA-256 一致，公网 healthz/readyz 均正常。

脱敏日志位于 `/opt/agent-platform/evidence/channel-conversation-20261004-1` 的 `candidate-history.log`、`public-history.log`、`public-web.json`。候选容器、私有临时环境文件和本地/服务器临时登录探测已清理，探测自己的登录会话已注销。没有向微信注入测试消息或额外调用模型；这是既有真实问答历史的展示验收。Runtime、Sandbox 和渠道收发实现未改变，本轮没有新增 Runtime Digest、Linux/gVisor 或 Production Conformance 证据。此前逐轮显示的历史验收是旧版本行为，由本次会话汇总行为取代。
