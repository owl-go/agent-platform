# 服务端架构

状态：Expert、Skill 与 Connector 简化的控制面、执行快照、CLI bundle 生命周期、User Action Wait、飞书 User 授权、Worker 重启恢复和管理员聚合健康、AI Creation 图片生成控制面与 Worker 已实现；AI Applications 目录、FAQ、分享 iframe 与 Knowledge Base 文档已接入。Knowledge Retrieval 通过可选 RAGFlow Adapter 接入受信任 API/Worker：异步解析确认后提交 Ready 状态和不可变 Generation Manifest，检索在当前权限下读取冻结版本；未配置或不可用时 fail closed。历史 Embedding Provider 设置和 pgvector 召回不是活动产品路径；真实部署验收以日期化证据为准。AI Creation 真实供应商验证、Token 刷新、Bot 权限恢复和 Linux + gVisor 生产证据仍待完成

AI Creation 的详细接口、状态、数据与验证设计见 `docs/technical/image-generation.md`。

## 结构

后端是两个 Go Kratos 进程：`cmd/api` 提供认证后的控制面，`cmd/worker` 领取会话回复、工作流 Run、定时触发和 MCP 测试。AI Creation 实现后，Worker 还会领取持久化的 Image Generation Record；图片供应商调用不在 API 请求生命周期内运行。Wire 只负责显式装配；所有运行配置来自严格校验的 YAML。

当前实现包含以下限界上下文：

- Account：OIDC 身份、Administrator 管理的 Registration Method、短期 Registration Attempt、本地 User 投影、Bootstrap Administrator 与可委派 Administrator、Resource Publisher、只读 Identity Group/成员关系同步、账号治理和隐私受限的 Governance Audit Event。
- Workspace：Session、Workflow、Run Conversation、Run、Expert、Expert Team、Skill、Administrator-owned Connector Publication、User-private Connector Installation/Authorization、兼容期 CLI Definition/Enablement/Approval、平台级 Model Provider Connection 与 Provider Model，以及 Personal Settings。
- Credits：Credit Ledger、余额投影、Daily Credit Allocation、Redemption Code、Model Credit Rate、Credit Adjustment，以及模型执行的积分准入和结算。
- Product Analytics：从已确认的登录、默认执行配置、Session 首次任务与终态、Workflow 创建、第二次成功运行和执行流重连生成追加式 Product Event。执行流重连只记录流类型与恢复方式，并按匿名对象的五分钟窗口去重；所有事件都只保存匿名 User/对象 Key 和白名单粗粒度属性，采集失败不改变业务操作结果。

- AI Creation：具有独立 Endpoint 和加密 API Key 的 Image Model、单一 Prompt Optimization 设置、Image Generation Record、Reference Image 与 Generated Image 的生命周期；不引用 Workspace 的 Model Provider Connection 或 Provider Model，通过 Credits 端口完成 Image Credit Reservation 与结算，并只保存 Object Storage 的逻辑 Object Key。
- AI Applications：Smart Assistant、FAQ、Knowledge Base 和分享配置的用户私有目录与版本控制。认证用户的 Assistant Conversation 拥有独立于 Workspace Session 的持久回合和完整审计记录；API 请求内通过独立 Model Provider Adapter 执行预处理和 SSE 生成，每个模型阶段走 Credits 准入与结算。External Conversation 仍是独立的匿名分享链路；完整外部会话审计仍按产品规格分阶段实现。

Account 拥有 User、治理角色、Identity Group 投影和 Governance Audit Event，不拥有积分状态。Credits 只读取当前 Group membership 与 Department Credit Budget 完成聚合准入；Workspace 通过 Credits 的 Application 端口检查准入、冻结每个 Execution Stage 的费率并结算实际消耗，不直接更新 Credit Ledger 或余额投影。AI Creation 同样不能直接更新余额或读取供应商凭证明文；它通过窄端口解析冻结的连接版本、创建预留并提交终态结算。四个上下文可以使用同一个 PostgreSQL 实例，但 Domain 和 Application 端口不泄漏 GORM Model。

Domain 与 Application 不依赖 GORM、HTTP、对象存储、Runtime CLI 或 YAML。`internal/data` 实现 PostgreSQL、Runtime、Keycloak 等端口；`internal/service` 只做 Proto/HTTP 映射、身份提取与公开错误转换。

账号扫码接入通过 Account Application 的供应商验证、Keycloak 联邦身份和短期 Repository 端口实现；Go Broker 只向固定 Keycloak client/回调签发一分钟身份断言，产品 Token 仍仅由 Keycloak 签发。配置和 Attempt 密文、Version CAS、一次性兑换及配置与治理审计的原子提交由 Account GORM Adapter 负责；供应商 HTTP 不进入数据库事务。Keycloak 普通账号无邮箱时允许扫码建号；密码账号仍由 Application 验证必填邮箱。部署预声明两个 admin-only 身份标记，不向产品服务授予 manage-realm。详细设计和真实验证边界见 [扫码登录与注册](scan-registration.md)。

## 默认资源初始化

API 与 Worker 在账号 Bootstrap 完成后复用 `Repository.EnsureDefaultResources`，将镜像中只读的 `connectors/`、`skills/`、`experts/` 目录定义写入现有 Expert、Skill、Connector Revision/Publication seam。目录根路径由镜像的 `AGENT_WORKSPACE_RESOURCE_ROOT` 指定；本地 Go 命令可从工作目录向上发现仓库 `resources.json` 标记。加载先冻结并校验全部包，再进入初始化事务；构建/发布脚本不作为第二来源。Migration `000072_default_resource_seeds.sql` 保存按资源稳定键与版本管理的初始化记录；它是 Adapter 层的升级账本，不引入新的 Domain 聚合。并发、失败、升级和权限边界见[默认资源分发](default-resources.md)。

## 所有权

Session、Workflow、Expert、Expert Team、Skill、MCP Connector、CLI Enablement/Authorization/Approval、Personal Settings 和 Image Generation Record 等 User-owned 资源的每个查询和写入都以认证 User ID 过滤。Knowledge Base 另有不可变的 private、group、platform 三种 scope：group 读取要求当前 active Department membership，写入还要求 Resource Publisher；Platform Resource 继续按全企业只读投影。Administrator 权限不会绕过 private 或 group membership 查询。Model Provider Connection、Provider Model、Image Model 与 CLI Connector Definition 是平台级目录，所有认证 User 可读取可用投影，只有 Administrator 可写；User 只保存引用全局资源的个人默认、Enablement、Authorization 和最近图片模型选择。管理员可以查看账号级余额、今日用量、每日额度、Department budget、兑换、人工调整，以及按 CLI Connector Definition 汇总的启用、等待操作和授权健康计数，但不能借助管理权限读取其他 User 的会话、工作流、图片提示词、Reference Image、Generated Image、Connector 凭证/内容、外部身份、授权 Scope 或逐次执行消费明细。跨 User ID 与不存在资源使用相同的 Not Found 语义。

## 事务与并发

- Session 发消息在一个事务中创建 User Message 和排队中的 Assistant Message；同一 Session 同时只有一个生成任务，不同 Session 之间不共享该执行槽位并可并行。
- Workflow 以 `workflow_id` 为并发边界维护持久 FIFO Queue；同一 Workflow 同时最多一个 Run 处于 `running` 或 `waiting_for_user`，不同 Workflow（包括同一 User 的 Workflow）可并行。首版每个 Workflow 最多五个 `queued` Run；队列位置按 `queued_at + id` 动态计算，取消 queued Run 后立即推进后继任务。
- Session 首次发送消息、Run Conversation 首个 Run 创建时从 Personal Settings 解析并冻结一个 Provider Model 与 Runtime Engine。Snapshot 的每个 Stage 共用该配置，同时独立包含可选 Expert/Team Member 身份、四段结构化 guidance、Model Provider Connection 版本、Model API Protocol、Endpoint，以及 exact Skill/Connector revisions；环境变量和共享 Workspace 配置保留在公共快照层。后续消息或 Run 复用冻结的执行配置；每轮按 owning User 与 Session / Run Conversation 隔离的不可变 Conversation Selection 合并专家默认和显式资源，在该轮 Response Snapshot / Run Snapshot 中保留实际执行计划。成功提交在同一事务内清空 retained selection 的显式 Skills，保留专家与 Connector 选择。API Key 与 Connector Secret 通过版本化凭证引用在 Worker 领取或命令启动前加载，不进入普通 Snapshot JSON。
- 仅 User 显式选择 `Plan first` 时，API 使用冻结的首个 Execution Stage 的 Provider Model，在有界请求时限内生成具体 Plan Step 标签；模型输入只含有界目标、步骤类型、已选 Stage 名称和资源名称，不含凭证或 Runtime 原始输出。这个额外调用通过 Credits 独立准入、结算并写入 Plan generation cost。生成失败或格式不合格时保留规则 Plan 骨架，并明确标识回退及已经结算的费用。模型标签不能决定 Step state；自动安全 Plan 与新建 Workflow 的验证 Plan 不产生额外模型调用。
- Worker 按 Session 或 Run Conversation、冻结 Team Member 身份（没有成员时为 Expert 或匿名 Stage）、Runtime Engine 和实际资源集合摘要维护隔离的 Warm Runtime Container 租约。动态资源选择的轮次关闭 Native Resume，始终使用平台消息与摘要续接，避免旧上下文保留已移除的指导与工具。租约不共享执行上下文、User 或资源边界；同一 Expert 的不同 Team Member 也只按顺序挂载同一轮 Workflow 临时 Workspace。执行结束立即停止并清理单次凭证，空闲 30 分钟后回收 Container 定义。
- CLI Connector bundle 在无 User 凭证的 Builder 中生成并通过 Object Storage 发布；不可变 Revision 与 Administrator Publication 只在 exact bundle/Runtime Digest Conformance 完整时进入目录。User 安装和多账号授权归属私有 Installation。公共 Wrapper 是所有 Runtime 的唯一 direct CLI 入口，负责 argv 与权限策略。`waiting_for_user`、一次性 Approval、nonce consumption 和执行前对 Publication、冻结 Revision、Installation 与 Authorization 的重校验由 Workspace Application 协调并持久化；每个 Stage 同时只有一个 active Approval。
- Run 状态与终态 Event 在同一 Repository 事务提交；Event Sequence 从 1 单调递增且只有一个终态。User Action Wait event 为非终态；拒绝或过期作为结构化 CLI 错误交回 Runtime，不绕过终态规则。
- Credits 上下文以不可变 Credit Ledger 为事实来源，并在同一事务维护 Credit Balance、每日额度剩余和今日用量投影。Daily Credit Allocation 以 `(user_id, credit_day)` 唯一，消费结算以 `(execution_id, stage_position)` 唯一；重试只能重放原结算，不能重复发放或扣减。Department Credit Budget 在文本与图片准入事务内按 Group + Credit Day 获取 Advisory Lock，汇总当前成员已结算消费与活动预留，并以所有适用部门中最小剩余值限制个人 Available Credit。
- Runtime-backed text invocation 按 Workflow 串行而非按 User 串行。Stage 开始前在 Credits 事务中锁定 User 余额并创建等于冻结 Model Credit Rate fallback 的 Execution Credit Reservation；实际输入/输出 Token 用量在终态精确结算，超出预留时沿用负余额语义。Expert Team 成员逐个 reservation/settlement，未启动成员不收费；Stage 终态、Reservation、Credit Ledger 消费记录和余额投影在一个 Repository 事务中提交。不同 Workflow 的 Stage 可并行，互不共享执行锁。
- Image Generation 是上述串行规则的受控例外：提交时在 Credits 上下文按 User 锁定余额，为完整输出数量创建归属提交 Credit Day 的 Image Credit Reservation，并分别记录来自当日额度与兑换余额的来源；同一 User 最多一个非终态图片批次，但可与一个 Runtime-backed 调用并行。后续任何准入都使用扣除未结算预留的 Available Credit。图片终态、成功输出元数据、Credit Consumption 和预留释放在一个事务提交；释放时已过期的旧 Daily Credit Allocation 不带入次日，未使用的 Redeemed Credit Balance 回到原余额。删除私有记录不退款，只留下不含执行内容的通用账本金额与时间。
- Image Generation Record 由 Worker 通过 `FOR UPDATE SKIP LOCKED` 和租约领取。未发给供应商的工作可在进程重启后恢复；已发出但结果不确定的工作进入 `outcome_unknown`，不盲目重试。User 停止后拒收迟到输出，且只对停止前已验证持久化的图片结算。
- 跨零点调用归属开始时的 Credit Day。次日额度通过首次余额读取或执行准入惰性物化，不依赖零点批处理；Personal Settings 时区变更只能从下一个 Credit Day 生效。
- 更新使用 Version 乐观锁；外部 Workflow API 创建 Run 还使用 `Idempotency-Key` 保存响应。
- Worker 使用 PostgreSQL `FOR UPDATE SKIP LOCKED` 领取任务，并在专用数据库连接上持有进程级 Advisory Lock，保证同一数据库只有一个执行 Worker 能够领取和恢复任务；连接或进程退出会自动释放该锁。每个 Worker 进程的第一次领取会在同一 PostgreSQL 事务中对账上一个进程遗留的 `generating`、`running` 和 `waiting_for_user`：已请求取消或所属资源已停用的执行直接收口为 `cancelled`，其余执行清除未完成输出后重新进入对应 Workflow Queue。对账同时释放该执行遗留的 Execution Credit Reservation，并关闭尚未消费的 Connector Approval；已经消费 Approval 的外部命令结果无法安全确认，因此对应执行 fail closed 而不盲目重放。恢复只改变非终态执行，不重开或改写终态 Session response 或 Run。

## API

Workflow Message Channel 的账号、受众配对、收发验证、原聊天回复与恢复设计见 [接入设计](workflow-message-channels.md)，产品边界见 [产品规格](../product/agent-workspace-requirements.md)。当前全部 13 个目标渠道有文本 Adapter；真实账号验收与部署健康检查分别记录，后者不能证明外部 IM 闭环可用。

Workspace Application 定义 `ChannelAccount`、`ChannelWebhookReceiver`/`ChannelStreamReceiver`、`ChannelSender`，通过 `ChannelTransport` 静态注册唯一接收方式与独立发送器。`ChannelStreamHealthReceiver` 仅报告可信连接健康；飞书、钉钉与企业微信临时发送者配对复用已认证接收连接，只生成由 owner 确认的受众候选，不创建 Inbox 或 Run。供应商错误映射为固定公共分类，可保留数值错误码，不传播凭据或原始错误内容。Matrix、Signal、BlueBubbles 的精确 HTTPS Endpoint 继续由 Administrator 批准，host 运输权限不扩展 Sandbox Egress。

可选 `ChannelTypingSender`/`ChannelTypingSession` 管理瞬态输入状态；`ChannelResponseSender` 管理共享回复的创建与更新，`ChannelReactionSender` 是独立可选的表情能力。Worker 的 `TrackResponse` 经 Application 端口接收公开预览，Runtime Executor 在已提交事件之后提取固定进度及已脱敏最终成员摘要/答案，供应商调用留在 Data Adapter，失败不改变 Run 结果。更新端口同时接收原 `ChannelMessage` 与 opaque handle；Application 在当前 owner、Audience、配置与 generation 检查后解密 Inbox 的 Reply，临时使用原回调信息，不把回调能力放入普通回复状态。飞书、钉钉和企业微信的动态回复、截止时间、Markdown 与备用发送边界以接入设计为准。

Migration `000066` 保存配置/Inbox/Conversation/Delivery，`000067` 扩展 provider 与加密接收游标，`000068` 冻结 Run 的渠道来源，`000069` 添加回复状态与 revision fence。回复句柄、创建意图和有界公开摘要可持久化，暂定答案只在内存；不确定创建遵守显式恢复规则。Inbox 准入复用 Workflow Queue 与 Credits，终态 Event/Credits/Outbox 同事务提交。Worker 持有进程级 Advisory Lock，连接、回执读取、进度和发送分别沿有界循环运行；渠道凭据与回复能力只用于 host 运输及执行脱敏，不进入 Runtime env/Snapshot。接入设计包含日期化发布证据；初次平台启用记录见 [部署记录](../evidence/agent-workspace/2026-10-03-workflow-message-channels-deployment.md)，完整 Production Conformance 与真实账号能力仍按各项证据单独判定。

`backend/api/workspace/v1/workspace.proto` 是普通 JSON API 的权威契约。用户认证使用 Bearer OIDC Token。Workflow API Key/API Secret 只允许通过 HTTP Basic 调用该 Workflow 的 Token Exchange；凭证通过拥有者专用的 Workflow API Credential 读取接口返回，API Secret 在存储中加密。Token Exchange 返回的 72 小时 JWT 通过 Bearer Header 启动和查看该 Workflow 的 Run，不代表 User 身份，也不能访问其他产品 API。

Credits 契约允许 User 读取自己的 Credit Balance、Available Credit、预留汇总和 Credit Ledger，并允许 Administrator 管理企业默认额度、提醒阈值、账号每日额度、Model Credit Rate 修订、Image Credit Rate 修订和带原因的 Credit Adjustment。Redemption Code 是默认关闭的可选渠道；关闭时普通用户和管理员 API 都 fail closed。余额不足统一映射为 `insufficient_credits` 和 HTTP `429 Too Many Requests`；准入按所有余额桶减去活动预留后的净 Available Credit 判定，返回当前 Available Credit 与下一次每日额度时间，不返回其他 User 或内部费率数据。

治理 API 只对当前 Administrator 开放：角色与账号状态更新、Identity Group 完整同步、Department Credit Budget、Department Knowledge Base custody 移交和 Governance Audit Event 列表。所有 mutation 要求 1–500 字符 reason 和适用资源的 Version CAS。Keycloak Adapter 只使用 Admin API 的 GET 读取 Group、层级和成员；Access Token 请求除外，不执行 Group/member mutation。Audit response 只返回 action、actor/target identifier、reason、time 与有界数值指标。

工作流历史中的每一行是一个 Run Conversation。`GET /api/v1/workflows/{workflow_id}/runs/{run_id}/turns` 按顺序读取所有 Run；`POST` 同一路径提交追问并排队一个新 Run，即使同一 Conversation 已有 queued/running turn 也不返回冲突。创建请求立即返回 `202` 和稳定 Run ID；GET/SSE 返回权威状态与动态 queue position，API 继续使用 `Idempotency-Key`。队列超过五个 queued Run 时手动/API 返回 `429 queue_full` 且不创建 Run，定时触发记录失败历史 Run。已经终态的 Run 永不重开，因而事件顺序、终态和 Artifact 审计边界保持不变。

AI Creation 使用普通 Proto/HTTP API 管理 Image Model、Prompt Optimization 设置、临时 Reference Image、Image Generation Record、历史和下载。当前记录另有手写 SSE Handler，只发布有界产品进度；断线后客户端先读取权威记录再续接。首期不向 Workflow Credential 或外部调用方开放图片生成接口。

`GET/POST /api/v1/conversation-selection` 读取或解析 owning User 的不可变选择修订。客户端仅持有 opaque ID 与展示元数据，不提交执行配置或读取 Secret。`GET /api/v1/conversation-files` 汇总当前对话附件与 Artifact，并为 Workflow 提供标明来源的 Workspace 文件；发送接口接收 `selection_id` 和 `file_references`。引用在提交前经 scope、路径、大小和 SHA-256 校验后复制为普通不可变附件；失败提交清理新副本，成功提交使用既有附件物化与只读挂载路径。`GET /api/v1/skills/{skill_id}/document` 在拥有者校验后读取已安装包中的 `SKILL.md`，前端安全渲染。

两个流式端点有意使用手写 Handler：

- `GET /api/v1/workflows/{workflow_id}/runs/{run_id}/events`：SSE 历史回放、实时事件与 Heartbeat。
- `GET /api/v1/workflows/{workflow_id}/runs/{run_id}`：Run 终态后读取一次完整的 `final_text` 或 `final_json`；执行中的读取仍返回权威状态与队列位置。
- `GET /api/v1/sessions/{session_id}/messages/{message_id}/events`：按 Owner 隔离持续推送 Assistant Message 快照。快照只暴露受限的产品进度阶段与已脱敏答案，不暴露 Runtime 原始事件、命令内容或模型私有推理；完成、失败或取消后关闭连接。
- `GET /api/v1/workflows/{workflow_id}/workspace/download?path=...`：认证后流式下载 Workspace 文件。
- Workspace HTTP API 只提供目录查看、文本预览和文件下载；Git Clone 由 `/api/v1/workflows/{workflow_id}/git-source` 设置入口完成。

保存失败使用现有 Kratos Error 的 `reason`、公开 `message` 与 `X-Request-ID`：前端保留 `invalid_input` / `invalid_request_body` 的公开校验说明，其他供应商原始说明不进入通用提示。公共错误格式器被各保存入口复用，优先展示明确业务原因，并提供已知故障类别和恢复动作；未知原因如实标记，保留请求编号。生产 PostgreSQL Dialector 的错误转换同时保留 GORM 标准分类与原始 cause（包括 SQLSTATE 和约束名）；集成测试使用 API/Worker 的实际数据库连接工厂验证该行为。Model Provider Connection 的名称唯一约束在 GORM Adapter 中转换为领域名称冲突，新增与改名返回 HTTP 409 / `model_provider_name_conflict`；版本 CAS 失败继续返回 HTTP 412 / `version_conflict`。未识别的数据库错误保留内部 cause，不向 API 回传 SQL、约束内容或凭证。

## Secret

Model Provider API Key、Workflow Secret 环境变量、MCP Secret、CLI App ID/App Secret/Token、Git HTTPS 密码/Token 和 Git SSH 私钥使用服务端数据密钥加密。读取 API 只返回 `configured` 或外部身份元数据，不返回明文，Administrator 也无权读取 User Connector Secret。Workflow SSH config 不是 Secret，但只接受无命令执行能力的连接字段；Git Clone 时与私钥一起物化到隔离的临时 HOME，私钥文件名匹配受限的 `IdentityFile`，并继续使用管理员固定的 `known_hosts`。执行时其他 Secret 物化为单次任务的 0600 文件，经公共 Entrypoint 或 Wrapper 导入；Runtime 输出、Event、结果和 Artifact 在持久化前使用精确值脱敏。

Derived Expertise Tag 后台任务与 Session、Run 共用执行阶段的版本化 Model Provider 凭证加载逻辑：Worker 领取任务时按 Connection ID 和 Version 读取密文及凭证归属，再交给 Runtime Executor 解密，不将凭证写入普通 Snapshot。凭证不可用时将标签任务标为失败并保留旧标签，不向 Runtime 提交缺失凭证的任务。

AI Creation Worker 按冻结的 Image Model revision 读取该模型自己的 Endpoint 和加密 API Key，并在调用后清理明文。Prompt Optimization 从自己的单一设置读取 Endpoint 和加密 API Key；两者都不回退到 Model Provider Connection。`ImageProvider` Adapter 只接收结构化生成或编辑参数和流式图片输入，返回结构化图片结果与安全错误，不管理 Repository、Credits、Object Storage 或权限。Adapter 默认使用 OpenAI Images；当 Endpoint 精确指向阿里云百炼同步 multimodal-generation 路径时自动使用百炼原生消息协议，不向管理员暴露额外的供应商或协议设置。管理员的真实图片模型验证复用同一 Adapter，并以应用四分钟、反向代理五分钟的专用边界容纳供应商生成时延。Prompt Optimization 通过独立直接调用 Adapter 使用 OpenAI-compatible Chat Completions，且不收取图片生成积分。普通日志和审计不保存提示词、图片、Base64、原始供应商响应、Object Key、签名 URL 或明文 API Key。

## 数据库

CLI 安装草稿允许认证 Driver 暂未解析；追加式 Migration `000029_cli_connector_draft_authentication.sql` 仅在非 `available` 状态允许空值，Builder 完成后必须写入受支持的 Driver。重复验证同一 Bundle/Runtime 时更新原 Conformance 结果，目录只投影当前 Bundle 的证据。`000030_cli_connector_deletion.sql` 引入受限的软删除及仅针对未删除 Definition 的唯一索引。管理员删除在一个事务中停用 Definition 和 Enablement、清理临时授权与账号 Token、解除所有 Expert 的可变绑定；历史 Snapshot、Artifact 与 User 的 Feishu Application 保留。重新安装后启用可复用原 Application，但已断开的账号需要重新授权。

当前产品以全新基线 Migration `000001_agent_workspace.sql` 建库，后续修正只通过不可变的追加式 Migration 演进；`000005_model_provider_connections.sql` 将早期 Model Profile 数据清空并替换为 Model Provider Connection、Provider Model 与版本化凭证结构，后续 Migration 删除模型类型字段，`000014_global_model_catalog.sql` 再把已有连接与模型目录提升为全局可读资源并保留原凭证加密作用域。Provider Model 优先来自供应商 `/models`，失败或不支持时使用平台维护的厂商默认列表，Administrator 也可显式补充。从旧企业控制面切换前必须备份并重建业务数据库；不支持把旧 Organization/Team/Agent Release 数据猜测性映射为新 User 私有数据。

发布前的一次性 CLI Connector Runtime 复验使用 `gormdb.OpenWithoutMigrations` 连接现有 Schema，不提前触发新 Release 的 Migration；正常 API/Worker 启动仍使用 `gormdb.Open`。复验只在现有 Conformance 表中记录已在候选 Runtime 上真实通过的不可变 bundle 组合，失败不进入服务切换。

Credits 通过新的追加式 Migration 引入，不修改既有 Migration。Migration 为现有 User 建立上线当日的 600 Credit Allocation，兑换余额从零开始；只有在目标环境实际运行 Migration 后才能报告为已执行。

AI Creation 通过新的追加式 Migration 引入 Image Model revisions、独立 Prompt Optimization 设置、Image Credit Rate revisions、Image Credit Reservations、Image Generation Records、Reference Images、Generated Images 和每 User 最近选择；后续追加式 Migration 将 Image Model 和 Prompt Optimization 从 Model Provider Connection 解耦，旧凭证不复制，需由 Administrator 重新填写。固定图片价格变更通过新的 Image Model revision 将所有当前模型费率设为每张 50 Credits，并将已配置百炼原生 Endpoint 的输出选项归一为其支持的自动质量、PNG 和不透明背景；既有 Image Generation Record 的冻结快照保持不变。对象内容保留在私有 Object Storage，数据库只保存经过校验的逻辑 Object Key、SHA-256、大小、格式和像素尺寸。

Expert、Team Member 与 Connector 简化继续使用追加式 Migration：旧 Capability Introduction 和 Execution Instruction 分别进入 Introduction 与 Operating Procedure，新必填 guidance 留空并令该 Expert 不完整；旧 Expert model/runtime/tag columns 只保留兼容读取；旧团队顺序生成稳定 Team Member ID。P1 以 Revision、Publication、Installation、Authorization 和 Approval 分表表达平台目录与 User-private 状态，且数据库唯一性约束保证每个 User 仅有一个飞书应用、每个 Installation 下同一外部账号只有一个 Authorization。安装飞书 Connector 后，API 通过官方设备流生成创建链接，只持久化加密设备码；前端以固定间隔调用完成接口，服务端取得 App ID/App Secret 后加密写入 Provider Application 并销毁临时设备码。账号 Access Token 与 Refresh Token 绑定 Installation 和外部账号身份加密，刷新采用 Authorization version CAS，断开时清除全部凭证密文。已有飞书应用和有效账号 Token 在真实 `feishu` Publication 可用时由发布触发器增量投影，旧表和历史 Snapshot JSON 不回写。

Conversation Selection 使用追加式 Migration `000027_conversation_selections.sql`，按 owner 与 Session / 根 Run 约束修订；删除所属对话时数据库级联删除修订。本地 PostgreSQL 17 临时数据库已验证完整迁移链与会话/工作流选择事务，生产迁移及 Linux + runsc 证据须单独取得。

Product Analytics 使用追加式 Migration `000057_product_analytics.sql`。事件名、对象类型和属性在写入前经过代码白名单，User ID 与 Session/Workflow ID 只以带命名空间的 SHA-256 匿名 Key 保存；唯一 Dedup Key 保证重复请求、轮询和 Worker 重试不重复计算漏斗。该表不保存提示词、回答、文件名、文件内容、外部账号、Secret、Object Key 或签名 URL。第二次成功运行由终态 Run 持久化后读取权威成功次数产生，不由前端点击推测。

企业治理使用追加式 Migration `000064_enterprise_governance.sql`：移除单一 Administrator 索引，增加唯一 Bootstrap Administrator、Resource Publisher、Identity Group/Membership、Governance Audit Event，以及 Knowledge Base scope/group 外键。Migration 将既有最早 Administrator 标为 Bootstrap Administrator，把既有 public Platform Knowledge Base 回填为 platform scope，并把旧的 Administrator-private Platform Knowledge Base 收敛为 owner-private scope；不会猜测 Department 或成员关系。完整 Migration 链与治理边界已在一次性 PostgreSQL 17 验证，生产 Migration 和真实 Keycloak 同步仍需单独证据。

Smart Assistant 受控发布使用追加式 Migration `000065_smart_assistant_controlled_publication.sql`：为 Assistant 保存当前版本的 Publication Validation，并将旧版不满足严格 Origin、正数每日上限和数据处理确认的 Share Configuration 全部撤销。Application 层只把当前版本的 passing validation 视为可服务状态；任何配置更新先使旧验证失效，开启和更新路径通过同一 Publication Check 检查配置、模型、FAQ、安全可检索知识、引用资源、Credits 和分享控制。Token 轮换只更换访问 secret，并把刚验证的配置结果绑定到新版本。当前分享默认允许自由提问，不提供自由提问开关或每日次数上限；历史配置字段仅兼容为 true/0，不参与发布校验或请求准入，已启用的分享无需重存或轮换 Token。Migration 000065 的历史撤销结果不自动恢复。Repository 的三十天发布统计只聚合 visitor conversation/turn 状态和 Credit Ledger，不读取或返回对话内容、FAQ、检索片段或访客身份。

## Connector Authorization 续期

Application 的 `ConnectorAuthorizationRenewal` 使用窄 Repository、Cipher 和供应商 Refresh port；Worker 装配当前飞书 OAuth Adapter，并通过独立分钟循环和渠道 Inbox 准入前检查调用。HTTP 不进入 GORM 事务，凭证明文只在 host 调用期间存在。Repository 负责 selected grant/owner/Installation/Publication 校验、会话 advisory lock、版本化保存及 Audit 原子性；API 手动刷新共享该锁，并在供应商调用前拒绝过期的 expected_version。没有自动扫码、账号切换或审批权限扩大。
