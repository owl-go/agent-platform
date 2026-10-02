import { createI18n } from "vue-i18n";

export type SupportedLocale = "zh-CN" | "en-US";
export const localeStorageKey = "agent-workspace-locale";

const zh = {
  artifactDisclosure: { viewAllArtifacts: "查看所有产物 ({count})", viewAllChanges: "查看所有变更 ({count})" },
  product: "Agent Workspace", nav: { label: "主导航", groups: { workspace: "工作区", resources: "资源中心", system: "系统" }, home: "首页", sessions: "会话", workflows: "工作流", experts: "专家", resources: "专家/技能/连接器", "resources-experts": "专家", "resources-skills": "技能", "resources-connectors": "连接器", "resources-knowledge": "知识库", "knowledge-bases": "知识库", settings: "设置", users: "账号管理", "ai-applications": "AI 应用", "ai-applications-assistants": "智能助手", "ai-applications-image-creation": "图片创作" },
  auth: { checking: "正在连接工作空间", required: "登录后开始使用", body: "你的会话、工作流、技能和连接器均属于你的工作空间。", signIn: "登录", unavailable: "身份服务暂时不可用", signOut: "退出登录", online: "API 在线", offline: "API 离线", checkingApi: "正在检查 API" },
  common: { new: "新增", edit: "编辑", name: "名称", save: "保存", saving: "保存中…", cancel: "取消", cancelled: "已取消", delete: "删除", archive: "归档", unarchive: "取消归档", rename: "重命名", more: "更多", loading: "加载中…", empty: "这里还没有内容", close: "关闭", retry: "重试", run: "运行", running: "运行中", waitingForUser: "等待用户操作", success: "成功", failed: "失败", queued: "排队中", copied: "已复制", copy: "复制", back: "返回", comingSoon: "即将开放", readOnly: "只读", upload: "上传", download: "下载", folder: "文件夹", enabled: "已启用" },
  sessions: { title: "会话", subtitle: "与专家或默认助手持续对话", new: "新建会话", archived: "已归档", active: "当前会话", chooseExpert: "选择专家（可选）", noExpert: "不选择专家", placeholder: "输入消息，Enter 发送，Shift + Enter 换行", welcome: "从一个问题开始。首条消息发送后，专家配置会冻结到本会话。", elapsed: "用时 {value}", jumpToLatest: "回到最新消息", copyQuestion: "复制当前问题", copyAnswer: "复制当前回答", usedSkills: "本次使用的技能", thinking: "思考中", stopGeneration: "中止生成", stopping: "正在中止…", cancelled: "已中止生成", modelSelector: "选择本会话使用的模型", modelUnverified: "此模型与当前 Runtime 尚未验证", deleteTitle: "删除这个会话？", deleteDescription: "删除后，这段对话及其全部消息将从你的工作空间中永久移除。", deleteTarget: "即将删除", deleteWarning: "此操作无法撤销", deleteConfirm: "删除会话", deleting: "正在删除…", deleteAction: "删除会话 {title}", progress: { preparing: "正在准备运行环境", thinking: "正在分析问题", using_tool: "正在调用工具", working: "正在处理结果", responding: "正在组织答案" }, setupTitle: "开始前完成 3 个步骤", setupModel: "连接模型供应商并选择默认模型", setupRuntime: "选择可用 Runtime", setupStart: "开始会话或工作流" },
  workflows: { title: "工作流", subtitle: "把一个目标变成可重复、可调度、可通过 API 调用的执行单元", new: "新建工作流", name: "名称", goal: "目标", expert: "专家（可选）", runtime: "运行引擎", model: "模型", open: "打开工作流", artifacts: "产物", workspace: "工作空间", history: "运行记录", settings: "设置", trigger: "触发方式", state: "状态", duration: "耗时", started: "运行时间", manual: "手动触发", scheduled: "自动触发", api: "API 调用", noRuns: "还没有运行记录", runNow: "立即运行", input: "本次补充输入（可选）", deletedRecords: "已删除记录", basic: "基本信息", knowledgeBases: "指定知识库", knowledgeBasesHint: "可选择多个知识库；运行时会按当前版本检索并保留引用。", execution: "执行设置", schedule: "定时触发", enableSchedule: "启用定时触发", frequency: "频率", hourly: "每小时", daily: "每天", weekly: "每周", hour: "小时", minute: "分钟", weekday: "星期", timezone: "时区", apiCredential: "API 凭证", generate: "生成", regenerate: "重新生成", copySecret: "请立即复制，Secret 不会再次显示。", gitSource: "Git 来源", gitDescription: "将公共 HTTPS 或私有 SSH 仓库克隆到空工作空间。", openWorkspace: "打开工作空间", deleteTitle: "删除工作流", deleteDescription: "配置、定时任务、凭证和工作空间会被删除，运行记录保留为只读。", queuedAt: "排队时间", queuePosition: "队列位置", startedAt: "开始时间", endedAt: "结束时间", inputLabel: "输入", eventsLabel: "事件与日志", runArtifacts: "本次运行产物", noRunArtifacts: "本次运行没有产物", snapshotLabel: "工作流快照", resultLabel: "结果", expired: "已过期", clearWorkspace: "清空工作空间", directoryName: "目录名称", gitURL: "Git URL（公共 HTTPS 或私有 SSH）", branch: "分支", privateKey: "SSH 私钥（保存后不可查看）", sshConfig: "SSH config（保存到本工作流）", sshConfigHelp: "支持 Host、HostName、User、Port、IdentityFile、IdentitiesOnly 和连接保活配置。克隆时写入隔离的 ~/.ssh/config。", sshConfigPlaceholder: "Host git-server\n  HostName git.example.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n  IdentitiesOnly yes", personalDefault: "个人默认", environment: "环境变量" },
  experts: { title: "专家", subtitle: "组合专业指引、技能与连接器", new: "创建专家", name: "专家名称", description: "简介", mcp: "MCP", skills: "技能", noBindings: "未绑定技能或连接器", descriptionHint: "仅用于展示，不会注入模型指令。", tested: "已测试", testRequired: "需要测试" },
  settings: { title: "设置", subtitle: "定义你的默认个性、模型、运行引擎与扩展", personality: "个性", gentle: "温和专业", direct: "直接高效", lively: "活泼亲切", custom: "自定义", gentleInstructions: "请以温和、耐心、专业的语气回复，像一位友善且经验丰富的专家在引导我，避免命令式表达和绝对化断言。", directInstructions: "请直接、简洁并以行动为导向地回复，优先给出结论和可执行步骤，省略不必要的寒暄与铺垫。", livelyInstructions: "请以活泼、亲切且易于交流的语气回复，在保持内容清晰有用的同时，让表达更自然、更有温度。", instructions: "个性说明", model: "模型设置", providers: "模型供应商", embedding: "向量模型", embeddingHint: "用于将知识库文档和问题转换为向量，仅管理员可以修改。", embeddingModel: "向量模型名称", embeddingDimensions: "向量维度", embeddingAPIKey: "向量模型 API Key", embeddingEnabled: "启用向量检索", embeddingSaved: "向量模型配置已保存", provider: "供应商", addProvider: "添加供应商", providerSaved: "模型供应商已保存", providerSaveFailed: "保存失败，请稍后重试。", providerValidationFailed: "保存失败：请求参数无效，请检查填写内容。", protocols: "API 协议", importedModels: "个已导入模型", refreshModels: "刷新模型", manualModel: "手动添加模型", modelCatalog: "模型目录", modelType: "模型类型", runtimeModels: "各运行引擎默认模型", verified: "已验证", unverified: "未验证", failed: "验证失败", runtime: "运行引擎", extensions: "扩展", mcp: "MCP 管理", skills: "Skills 管理", cli: "第三方 CLI", language: "语言", timezone: "时区", addModel: "新增模型", endpoint: "Endpoint", secret: "API Secret（保存后不可查看）", modelId: "模型 ID", default: "默认", testPending: "测试中", tested: "已测试", testRequired: "需要测试", saved: "设置已保存", secretSet: "Secret 已配置", secretMissing: "缺少 Secret", keepSecret: "留空则保留现有 API Key", transport: "传输方式", bearerToken: "Bearer Token（可选）", optional: "可选", fixedVersion: "固定版本", arguments: "参数", onePerLine: "每行一个参数", value: "值", environment: "环境变量", source: "来源", skillHint: "Skill 根目录必须包含 SKILL.md；每次运行都会冻结精确版本。", available: "可用", unavailable: "不可用" },
  modelField: "模型",
  users: { title: "账号管理", subtitle: "创建普通账号、停用账号或重置临时密码", create: "创建账号", username: "用户名", displayName: "显示名称", email: "邮箱", enabled: "已启用", disabled: "已停用", reset: "重置密码", temporary: "临时密码仅显示一次", forceChange: "用户首次登录时必须修改密码", user: "用户", status: "状态", created: "创建时间", administrator: "管理员" },
  errors: { generic: "操作没有完成，请稍后重试。", conflict: "内容已被其他操作更新，请刷新后重试。", validation: "请检查填写内容。", copy: "复制失败，请检查浏览器剪贴板权限。" },
};

const en = {
  artifactDisclosure: { viewAllArtifacts: "View all artifacts ({count})", viewAllChanges: "View all changes ({count})" },
  product: "Agent Workspace", nav: { label: "Primary navigation", groups: { workspace: "Workspace", resources: "Resource Center", system: "System" }, home: "Home", sessions: "Sessions", workflows: "Workflows", experts: "Experts", resources: "Experts / Skills / Connectors", "resources-experts": "Experts", "resources-skills": "Skills", "resources-connectors": "Connectors", "resources-knowledge": "Knowledge Bases", "knowledge-bases": "Knowledge Bases", settings: "Settings", users: "User accounts", "ai-applications": "AI Applications", "ai-applications-assistants": "Smart Assistants", "ai-applications-image-creation": "Image Creation" },
  auth: { checking: "Connecting to your workspace", required: "Sign in to continue", body: "Your sessions, workflows, Skills, and Connectors belong to your workspace.", signIn: "Sign in", unavailable: "Identity service is unavailable", signOut: "Sign out", online: "API online", offline: "API offline", checkingApi: "Checking API" },
  common: { new: "New", edit: "Edit", name: "Name", save: "Save", saving: "Saving…", cancel: "Cancel", cancelled: "Cancelled", delete: "Delete", archive: "Archive", unarchive: "Unarchive", rename: "Rename", more: "More", loading: "Loading…", empty: "Nothing here yet", close: "Close", retry: "Retry", run: "Run", running: "Running", waitingForUser: "Waiting for user action", success: "Succeeded", failed: "Failed", queued: "Queued", copied: "Copied", copy: "Copy", back: "Back", comingSoon: "Coming soon", readOnly: "Read only", upload: "Upload", download: "Download", folder: "Folder", enabled: "Enabled" },
  sessions: { title: "Sessions", subtitle: "Keep a conversation going with an Expert or your default assistant", new: "New session", archived: "Archived", active: "Current sessions", chooseExpert: "Choose an Expert (optional)", noExpert: "No Expert", placeholder: "Message your workspace — Enter to send, Shift + Enter for a new line", welcome: "Start with a question. Expert configuration freezes after the first message.", elapsed: "{value} elapsed", jumpToLatest: "Jump to latest message", copyQuestion: "Copy this question", copyAnswer: "Copy this answer", usedSkills: "Skills used for this message", thinking: "Thinking", stopGeneration: "Stop generating", stopping: "Stopping…", cancelled: "Generation stopped", modelSelector: "Choose the model for this Session", modelUnverified: "This model and Runtime have not been verified", deleteTitle: "Delete this session?", deleteDescription: "This conversation and every message in it will be permanently removed from your workspace.", deleteTarget: "About to delete", deleteWarning: "This action cannot be undone", deleteConfirm: "Delete session", deleting: "Deleting…", deleteAction: "Delete session {title}", progress: { preparing: "Preparing the runtime", thinking: "Analyzing your request", using_tool: "Using a tool", working: "Processing the result", responding: "Composing the answer" }, setupTitle: "Complete three steps to start", setupModel: "Connect a model provider and choose a default", setupRuntime: "Select an available Runtime", setupStart: "Start a Session or Workflow" },
  workflows: { title: "Workflows", subtitle: "Turn one goal into an execution you can repeat, schedule, or call by API", new: "New workflow", name: "Name", goal: "Goal", expert: "Expert (optional)", runtime: "Runtime engine", model: "Model", open: "Open workflow", artifacts: "Artifacts", workspace: "Workspace", history: "Run history", settings: "Settings", trigger: "Trigger", state: "State", duration: "Duration", started: "Run time", manual: "Manual", scheduled: "Scheduled", api: "API", noRuns: "No runs yet", runNow: "Run now", input: "Optional input for this run", deletedRecords: "Deleted records", basic: "Basic", knowledgeBases: "Knowledge Bases", knowledgeBasesHint: "Select one or more Knowledge Bases. Runs retrieve from their current indexed generation and retain citations.", execution: "Execution", schedule: "Schedule", enableSchedule: "Enable schedule", frequency: "Frequency", hourly: "Hourly", daily: "Daily", weekly: "Weekly", hour: "Hour", minute: "Minute", weekday: "Weekday", timezone: "Timezone", apiCredential: "API credential", generate: "Generate", regenerate: "Regenerate", copySecret: "Copy now. The secret will not be shown again.", gitSource: "Git source", gitDescription: "Clone a public HTTPS or private SSH repository into an empty Workspace.", openWorkspace: "Open Workspace", deleteTitle: "Delete workflow", deleteDescription: "Configuration, schedule, credential, and Workspace are removed. Run history remains read-only.", queuedAt: "Queued", queuePosition: "Queue position", startedAt: "Started", endedAt: "Ended", inputLabel: "Input", eventsLabel: "Events and logs", runArtifacts: "Artifacts from this run", noRunArtifacts: "This run did not produce artifacts", snapshotLabel: "Workflow snapshot", resultLabel: "Result", expired: "Expired", clearWorkspace: "Clear Workspace", directoryName: "Directory name", gitURL: "Git URL (public HTTPS or private SSH)", branch: "Branch", privateKey: "Private SSH key (write-only)", sshConfig: "SSH config (saved to this Workflow)", sshConfigHelp: "Supports Host, HostName, User, Port, IdentityFile, IdentitiesOnly, and keepalive settings. It is written to an isolated ~/.ssh/config while cloning.", sshConfigPlaceholder: "Host git-server\n  HostName git.example.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519\n  IdentitiesOnly yes", personalDefault: "Personal default", environment: "Environment variable" },
  experts: { title: "Experts", subtitle: "Compose specialist guidance, Skills, and Connectors", new: "Create Expert", name: "Expert name", description: "Introduction", mcp: "MCP", skills: "Skills", noBindings: "No Skills or Connectors bound", descriptionHint: "Display only — not injected into model instructions.", tested: "Tested", testRequired: "Test required" },
  settings: { title: "Settings", subtitle: "Choose your default personality, model, Runtime, and extensions", personality: "Personality", gentle: "Gentle & professional", direct: "Direct & efficient", lively: "Lively & friendly", custom: "Custom", gentleInstructions: "Reply in a calm, patient, and professional tone, like a friendly and experienced expert guiding me. Avoid commanding language and absolute claims.", directInstructions: "Reply directly, concisely, and with an action-oriented approach. Lead with the result and executable steps, without unnecessary greetings or preamble.", livelyInstructions: "Reply in an upbeat, approachable, and conversational tone while keeping the content clear, useful, and naturally warm.", instructions: "Personality instructions", model: "Models", providers: "Model providers", embedding: "Embedding provider", embeddingHint: "Used to vectorize Knowledge Base documents and questions. Only administrators can change it.", embeddingModel: "Embedding model", embeddingDimensions: "Vector dimensions", embeddingAPIKey: "Embedding API Key", embeddingEnabled: "Enable vector retrieval", embeddingSaved: "Embedding provider saved", provider: "Provider", addProvider: "Add provider", providerSaved: "Model provider saved", providerSaveFailed: "Save failed. Try again shortly.", providerValidationFailed: "Save failed: the request values are invalid. Check the form and retry.", protocols: "API protocols", importedModels: "imported models", refreshModels: "Refresh models", manualModel: "Add model manually", modelCatalog: "Model catalog", modelType: "Model type", runtimeModels: "Default model for each Runtime", verified: "Verified", unverified: "Unverified", failed: "Verification failed", runtime: "Runtime engine", extensions: "Extensions", mcp: "MCP management", skills: "Skills management", cli: "Third-party CLI", language: "Language", timezone: "Timezone", addModel: "Add model", endpoint: "Endpoint", secret: "API Secret (write-only)", modelId: "Model ID", default: "Default", testPending: "Testing", tested: "Tested", testRequired: "Test required", saved: "Settings saved", secretSet: "Secret configured", secretMissing: "Secret missing", keepSecret: "Leave blank to keep the existing API Key", transport: "Transport", bearerToken: "Bearer token (optional)", optional: "Optional", fixedVersion: "Pinned version", arguments: "Arguments", onePerLine: "One argument per line", value: "Value", environment: "Environment variable", source: "Source", skillHint: "The Skill root must contain SKILL.md; every Run freezes the exact version.", available: "available", unavailable: "unavailable" },
  modelField: "Model",
  users: { title: "User accounts", subtitle: "Create, disable, or reset ordinary user accounts", create: "Create account", username: "Username", displayName: "Display name", email: "Email", enabled: "Enabled", disabled: "Disabled", reset: "Reset password", temporary: "Temporary password is shown once", forceChange: "The user must change it on first sign-in", user: "User", status: "Status", created: "Created", administrator: "Administrator" },
  errors: { generic: "The operation did not complete. Try again.", conflict: "This content changed elsewhere. Refresh and retry.", validation: "Check the entered values.", copy: "Copy failed. Check the browser clipboard permission." },
};

Object.assign(zh.common, { send: "发送" });
Object.assign(zh.common, { loadMore: "加载更多" });
Object.assign(zh, { credits: { title: "积分", balance: "积分余额", consumed: "共消耗 ✧ {value}", dailyRemaining: "今日剩余", persistent: "兑换积分", todayConsumed: "今日消耗", nextReset: "下次发放", codePlaceholder: "输入兑换码", redeem: "兑换", codeUnavailable: "兑换码不可用", ledger: "积分明细", entry: { daily_allocation: "每日积分", daily_expiry: "每日积分过期", consumption: "模型消耗", redemption: "兑换", adjustment: "管理员调整" } } });
Object.assign(zh.users, { tabs: { users: "用户", rates: "模型倍率", codes: "兑换码" }, dailyLimit: "每日积分", credits: "积分设置", creditSettings: "用户积分", adjustment: "积分调整", adjustmentReason: "调整原因", rateEditor: "模型倍率设置", rateHistory: "倍率版本记录", providerType: "供应商类型", protocol: "API 协议", modelId: "模型 ID（全部留空表示平台默认）", inputMultiplier: "输入倍率", outputMultiplier: "输出倍率", fallback: "缺失 Usage 回退积分", platformDefault: "平台默认", createCodes: "生成兑换码", codeCount: "数量（1–100）", codeValue: "每个兑换积分", expiry: "过期时间（可选）", generateCodes: "生成", copyCodesNow: "兑换码仅本次显示，请立即复制", codeStatus: "兑换码状态", voidCode: "作废", codeState: { available: "可用", redeemed: "已兑换", void: "已作废", expired: "已过期" } });
Object.assign(zh.workflows, { conversation: "运行对话", followUpPlaceholder: "继续对话，Enter 发送，Shift + Enter 换行", activityDetails: "查看执行过程", command: "命令", reasoningSummary: "思考过程", runtimePrepared: "运行环境已准备", toolCompleted: "工具调用完成", updatingFiles: "正在更新文件", streamingAnswer: "正在生成回答", answerReady: "回答已生成", howToIntegrate: "如何接入", credentialUnavailable: "该凭证由旧版本生成，Secret 无法恢复，请点击“重新生成”。", apiKeyLabel: "API Key", apiSecretLabel: "API Secret", showSecret: "显示 API Secret", hideSecret: "隐藏 API Secret", integrationGuideTitle: "如何接入工作流", integrationStepToken: "1. 定义自动换取 JWT 的请求函数（Bash）", integrationStepInvoke: "2. 创建 Run（返回 Run ID）", integrationStepStream: "3. 流式获取输出（SSE）", integrationStepFullOutput: "4. 一次性获取完整输出", integrationTokenHint: "在同一 Bash 会话中先定义此函数，再执行下方命令。它会在每次调用前自动换取 JWT，无需人工续期；请安全保存 API Key/Secret。", integrationIdempotencyHint: "每个业务请求使用唯一的 Idempotency-Key；重试同一个请求时保持该值不变。", integrationRunHint: "创建接口立即返回 202 和 Run ID；将返回的 ID 保存到 RUN_ID。", integrationStreamHint: "SSE 会先回放历史事件，再持续推送实时事件，直到 Run 进入终态。", integrationFullOutputHint: "Run 完成后调用此接口一次，完整结果位于 final_text 或 final_json。" });
Object.assign(zh.sessions, { activitySummary: { runtime: { running: "正在准备运行环境", completed: "运行环境已准备" }, reasoning: { running: "正在分析问题", completed: "已分析问题" }, feishuChatSearchHelp: { running: "调用飞书连接器读取群聊搜索说明", completed: "已调用飞书连接器读取群聊搜索说明" }, feishuChatSearch: { running: "调用飞书连接器搜索群聊", completed: "已调用飞书连接器搜索群聊" }, feishuMessageSendHelp: { running: "调用飞书连接器读取消息发送说明", completed: "已调用飞书连接器读取消息发送说明" }, feishuMessageSend: { running: "调用飞书连接器发送消息", completed: "已调用飞书连接器发送消息" }, tool: { running: "执行工具操作", completed: "已执行工具操作" }, file: { running: "正在更新文件", completed: "已更新文件" }, activity: { running: "正在处理任务", completed: "已完成处理" } } });
Object.assign(zh.sessions, { executionEvidence: { title: "本次执行", tools: "{count} 项工具调用", files: "{count} 项文件变化", stages: "{count} 个执行阶段", artifacts: "{count} 个产物", sources: "{count} 项依据", noExternal: "未记录外部工具、文件变化或依据", running: "进行中", completed: "已完成", stage: "阶段 {position}", openSource: "打开来源", sourceUnavailable: "来源已更新、删除或你已无权访问", sourceKind: { file: "文件", knowledge: "知识库", connector: "连接器", artifact: "产物" }, sourceState: { requested: "已请求", succeeded: "已使用", failed: "调用失败", not_used: "未采用" }, sourceAction: { retrieved: "检索到相关内容", noMatch: "没有相关检索结果", retrievalFailed: "检索失败", retrievalUnavailable: "检索服务不可用", indexUnavailable: "可用索引不存在", notInvoked: "本次未调用" }, kind: { runtime: "环境", reasoning: "分析", tool: "工具", file: "文件", activity: "执行" } } });
Object.assign(zh.workflows, { gitCredentialSaved: "凭证已保存，无法查看。重新克隆时请重新填写。", replaceGitCredential: "重新填写", gitPrivateKeyDraft: "新 SSH 私钥（尚未保存）", gitPasswordDraft: "新密码 / Token（尚未保存）" });
Object.assign(zh.workflows, { gitErrors: {
  git_source_invalid: "Git 配置无效。请检查仓库地址与认证方式是否匹配、分支是否填写，以及 Git / SSH config 是否符合字段下方的说明。",
  git_credentials_required: "请填写本次克隆使用的密码 / Token 或 SSH 私钥。已保存的凭证不会回显。",
  git_workspace_not_empty: "工作空间已有内容，无法克隆。请先备份并清空工作空间，或新建一个空工作流后重试；现有文件未被删除。",
  git_server_unavailable: "服务端 Git / SSH 或主机校验配置不可用，请联系管理员检查。",
  git_ssh_host_untrusted: "SSH 主机校验失败。请联系管理员核实仓库主机指纹并更新受信任主机配置后重试。",
  git_ssh_key_invalid: "SSH 私钥无法读取。请检查是否完整粘贴了私钥首尾标记和换行，并使用无需交互输入口令的密钥。",
  git_authentication_failed: "Git 认证失败。请检查账号、Token 或 SSH 私钥，以及该账号对仓库的访问权限。",
  git_branch_not_found: "仓库中找不到指定分支，请检查分支名称。",
  git_repository_unavailable: "仓库不存在或不可访问，请检查仓库地址和访问权限。",
  git_connection_failed: "无法连接 Git 服务器或连接超时，请检查地址、端口和网络后重试。",
  git_repository_too_large: "克隆后的仓库超过工作空间 1 GiB 容量限制。",
  git_clone_failed: "保存并克隆失败。请检查仓库地址、分支、认证信息和工作空间是否为空，然后重试。输入内容已保留。",
} });

Object.assign(zh.workflows, { apiTokenDescription: "API Key 和 API Secret 没有固定到期时间，重新生成或停用后失效；它们会加密保存，可在此查看和复制。JWT 有效期为 72 小时，业务系统可在每次调用前自动换取。", gitAuthentication: "认证方式", gitPublic: "公共 HTTPS", gitAccount: "账号密码", gitUsername: "Git 账号", gitPassword: "Git 密码 / Token", gitURLHelp: "支持 https:// 地址和 git{'@'}host:path 格式的 SSH 地址。", gitSaved: "Git 仓库已克隆并保存", cloneRepository: "保存并克隆仓库" });
Object.assign(zh.sessions, { addAttachment: "添加图片或文件", removeAttachment: "移除附件 {name}", attachmentLimits: "每条消息最多 10 个附件，单个附件不能超过 100 MB" });
Object.assign(zh.sessions, { connectorAuthorizationWait: "等待{provider}授权：这次操作尚未执行。请在下方输入框的授权提示中点击“打开{provider}授权”，在{provider}完成授权后返回会话，回复“已授权”或点击“重试”继续。如果没有看到授权入口，请到连接器页面检查账号授权状态。" });
Object.assign(zh.sessions.progress, { finalizing: "正在安全保存会话" });
Object.assign(zh.sessions, { executionStatus: { stale: "任务仍在运行 · 最后更新于 {time}", modelCalls: "模型调用 {count} 次", settlementPending: "完成后结算", cancelQueue: "取消排队" } });
Object.assign(en.sessions, { executionStatus: { stale: "Still running · last updated at {time}", modelCalls: "{count} model calls", settlementPending: "Settles on completion", cancelQueue: "Cancel queued run" } });
Object.assign(zh.nav, { resources: "专家/技能/连接器" });
Object.assign(zh.sessions, { resourceActionSkill: "技能创建预览", resourceActionExpert: "专家创建预览", resourceActionConnector: "连接器创建预览", resourceActionConfirm: "确认创建", resourceActionConfirmed: "已创建并安装", resourceActionCancelled: "已取消", resourceActionExpired: "操作已过期或失败" });
Object.assign(zh, { resources: { title: "技能·连接器", centerTitle: "资源中心", subtitle: "管理可复用的技能与连接器", skills: "技能", connectors: "连接器", cli: "第三方 CLI", cliDefinition: "CLI 连接器定义", continueSetup: "继续完成授权", enable: "启用", exactVersion: "固定版本", executable: "可执行命令", npmPackage: "npm 包", npmIntegrity: "npm 完整性", feishuCLI: "飞书 CLI", deleteAffected: "删除“{resource}”会同时从这些专家中解除绑定：{experts}。历史快照不会改变。", deleteUnaffected: "确认删除“{resource}”？历史快照不会改变。", capabilities: "能力策略", capabilityId: "能力标识", argvPrefix: "命令参数前缀", risk: "风险等级", lowRisk: "低风险", highRisk: "高风险", identities: "执行身份", scopes: "所需权限", egressHosts: "允许访问的域名", timeoutSeconds: "超时（秒）", architectures: "支持架构", recommendedSkills: "推荐技能", state: { draft: "草稿", building: "构建中", testing: "验证中", available: "可用", failed: "失败", disabled: "已停用" }, health: { enabled: "{count} 个用户已启用", waiting: "{count} 个等待用户操作", authorization: "{active} 个有效授权 · {attention} 个需处理" } } });
Object.assign(zh, { approvals: { title: "请确认{connector}操作", messageSendDescription: "助手准备通过{connector}向下方目标发送消息。选择“仅批准本次”后才会发送；拒绝则不会执行这次发送。", otherDescription: "助手准备通过{connector}执行下方操作。选择“仅批准本次”后才会执行；拒绝则不会执行这次操作。", operation: "拟执行操作", sendMessage: "发送消息", target: "目标标识", groupTarget: "目标群聊标识", hiddenArguments: "消息内容及命令参数不会在此处展示。如果这不是你刚才要求的操作，或你需要先核对具体内容，请拒绝并让助手说明。", expires: "请在{time}前确认，超时后本次请求将自动失效。", identity: "执行身份", user: "以用户身份", bot: "以机器人身份", reject: "拒绝", approveOnce: "仅批准本次" } });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { operationFailed: "操作失败，请重试。若持续失败，请联系管理员。", deleteCLI: "确认删除“{resource}”？所有用户将无法继续使用此连接器，专家中的绑定和账号授权会解除。历史快照和文件会保留。", publish: "发布", disable: "停用", developerConsole: "开发者后台", authorizeAccount: "授权飞书账号", authorizeNow: "打开飞书授权", authorizedAccount: "已授权：{name}", disconnectAccount: "断开账号", authorizationPending: "等待你在飞书完成授权", recommendedSkillOffer: "推荐技能：{name}", recommendedSkillWarning: "建议为当前专家选择技能“{name}”", installSkill: "安装技能", selectSkill: "选择技能" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { expandAuthorization: "扩展飞书权限" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { loginRequired: "登录已失效，请重新登录后重试。", permissionDenied: "你没有权限执行此操作，请联系管理员。", resourceMissing: "此技能或连接器已不存在，请刷新页面。", resourceChanged: "内容已被其他操作更新，请刷新后重试。", invalidInput: "请检查填写内容和安装包格式后重试。", invalidNPMInstall: "npm 包规格必须指定精确版本，例如 package@1.2.3；如果粘贴 npm install 命令，也必须包含版本。", authorizationInvalidInput: "无法发起飞书账号授权，请刷新页面后重试。若持续失败，请联系管理员。", uploadTooLarge: "上传内容过大，请缩小文件后重试。ZIP 包上限为 50 MiB。", tooManyRequests: "请求过于频繁，请稍后重试。", serviceUnavailable: "服务暂时不可用，请稍后重试。", networkFailed: "网络连接失败，请检查网络后重试。", statusUpdateFailed: "部分内容或状态暂时无法更新，请稍后刷新重试。当前内容已保留。" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { newSkill: "添加技能", createSkill: "创建技能", uploadSkill: "上传技能", systemSkillUnavailable: "平台默认技能暂不可用，请联系管理员。", newConnector: "新建连接器", chooseConnectorType: "新建连接器", chooseConnectorTypeHint: "选择连接器的接入方式，保存后统一显示在连接器目录中。", mcpConnector: "MCP 连接器", mcpConnectorHint: "连接 Streamable HTTP 服务，或运行固定版本的 stdio 包。", cliConnector: "CLI 连接器", cliConnectorHint: "配置固定版本的第三方命令行工具及其能力策略。", administratorOnly: "只有管理员可以创建 CLI 连接器。" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { installCLI: "安装第三方 CLI", editCLI: "重新安装第三方 CLI", icon: "图标", capabilityDescription: "能力描述", installationType: "安装方式", npmInstall: "npm 安装", npmPackageSpec: "npm 包规格", zipUpload: "上传 ZIP 包", exactNPMHint: "填写包含精确版本的包规格，例如 {'@'}scope/package{'@'}1.2.3；也可粘贴带 -g 的 npm install 命令，但必须包含版本。", cliZipHint: "ZIP 根目录需包含 package.json；运行元数据由安装包提供并经过平台校验。", install: "安装", noCapabilityDescription: "暂无能力描述", feishuCapability: "读取、检索并操作飞书文档、日历、消息及其他开放平台资源。" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { iconUploadHint: "支持 PNG、JPEG、WebP 或 GIF，大小不超过 384KB。" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { platformSkills: "平台技能", mySkills: "我的技能", platformConnectors: "平台连接器", myConnectors: "我的连接器" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { connected: "已连接", setupRequired: "需要设置", availableToInstall: "可安装", packageVersion: "版本 {version}", conformanceAvailable: "已通过运行验证", conformanceUnavailable: "运行验证不可用", selectedAccount: "当前账号", refreshAuthorization: "刷新授权", upgrade: "升级", uninstall: "卸载" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { connect: "连接", wecomBotId: "Bot ID", wecomSecret: "Secret", providedCredentialsInvalid: "请填写有效的企业微信 Bot 凭证。" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { modaoToken: "个人空间令牌", modaoTokenHelp: "在墨刀头像菜单的令牌设置中创建令牌", modaoCredentialsInvalid: "请填写有效的墨刀个人空间令牌，不能包含空格或换行。" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { deactivate: "取消激活" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { connectorPackageUpload: "上传连接器", connectorPackageUploadHint: "上传一个经过校验的 ZIP；安装后仍显示在同一个连接器目录中。" });
Object.assign(zh.experts, { platformExperts: "平台专家", myExperts: "我的专家" });
Object.assign(zh.experts, {
  teams: "专家团", catalog: "专家目录", createTeam: "创建专家团", editExpert: "编辑专家", editTeam: "编辑专家团",
  searchExperts: "搜索专家名称或能力介绍", searchTeams: "搜索专家团名称或能力介绍", expertiseFilter: "分类筛选", categoryFilter: "分类筛选", category: "分类", all: "全部",
  loadFailed: "加载专家目录失败", loadExpertFailed: "加载专家失败", loadTeamFailed: "加载专家团失败", incomplete: "待完善",
  teamUnavailable: "至少需要 2 位可用专家", noExperts: "没有匹配的专家", noTeams: "没有匹配的专家团", perRound: "{count} 位专家 / 每轮",
  backCatalog: "返回专家目录", backTeams: "返回专家团", capability: "能力介绍", instruction: "执行指令", expertise: "擅长领域",
  basic: "基本信息", basicHint: "能力介绍仅用于目录展示；执行指令会在专家运行时注入。", instructionHint: "这是专家唯一的角色指令，不会添加隐藏提示词。",
  expertiseHint: "最多 10 个标签，每个不超过 20 个字符。", addTag: "添加标签", tagPlaceholder: "输入后按 Enter", extensions: "扩展",
  extensionsHint: "绑定此专家可以使用的 MCP 与 Skills。", extensionFailed: "扩展操作失败", saved: "专家已保存", teamSaved: "专家团已保存",
  saveFailed: "保存失败，请检查名称、能力介绍和执行指令", teamSaveFailed: "保存失败，专家团需要 2–10 位不同的可用专家",
  operationFailed: "操作失败", saveSucceeded: "保存成功", teamInfo: "团队信息", teamInfoHint: "介绍只用于展示，团队不包含额外执行指令。",
  teamName: "专家团名称", members: "团队成员", membersHint: "每轮按从上到下的顺序执行，后一个专家会收到前面专家的最终结果。",
  chooseExpert: "选择可用专家", add: "添加", sequential: "顺序执行", moveUp: "上移 {name}", moveDown: "下移 {name}", removeMember: "移除 {name}",
  deleteExpertTitle: "删除“{name}”？", deleteExpertHint: "该专家会从可编辑的专家团中移除。已经冻结到历史会话和运行对话中的快照不会改变。",
  deleteTeamHint: "工作流和未开始的会话会回到“不使用专家”。历史快照保持不变。", confirmDelete: "确认删除", deleteExpertFailed: "删除专家失败", deleteTeamFailed: "删除专家团失败", modelUnavailable: "模型不可用", incompatible: "不兼容", confirmProfile: "我确认该模型与运行引擎用于此专家", confirmProfileRequired: "请确认专家使用的模型与运行引擎",
  icon: "图标", iconBackground: "图标背景", introduction: "简介", coreCapability: "核心能力", operatingProcedure: "工作流程", outputStandard: "输出规范", cautions: "注意事项（可选）", sage: "鼠尾草绿", sand: "暖沙色", sky: "天空蓝", coral: "珊瑚色", resourceCounts: "{skills} 个技能 · {connectors} 个连接器", memberName: "成员名称", memberLabels: "成员标签（最多 5 个）", tagGenerating: "正在生成标签", tagFailed: "标签生成失败"
});
Object.assign(en.common, { send: "Send" });
Object.assign(en.common, { loadMore: "Load more" });
Object.assign(en, { credits: { title: "Credits", balance: "Credit balance", consumed: "Used ✧ {value}", dailyRemaining: "Remaining today", persistent: "Redeemed Credits", todayConsumed: "Used today", nextReset: "Next allocation", codePlaceholder: "Enter Redemption Code", redeem: "Redeem", codeUnavailable: "Redemption Code is unavailable", ledger: "Credit ledger", entry: { daily_allocation: "Daily allocation", daily_expiry: "Daily expiry", consumption: "Model usage", redemption: "Redemption", adjustment: "Administrator adjustment" } } });
Object.assign(en.users, { tabs: { users: "Users", rates: "Model rates", codes: "Redemption codes" }, dailyLimit: "Daily Credits", credits: "Credits", creditSettings: "User Credits", adjustment: "Credit adjustment", adjustmentReason: "Adjustment reason", rateEditor: "Model Credit Rate", rateHistory: "Rate revision history", providerType: "Provider type", protocol: "API protocol", modelId: "Model ID (leave all three blank for default)", inputMultiplier: "Input multiplier", outputMultiplier: "Output multiplier", fallback: "Missing-Usage fallback", platformDefault: "Platform default", createCodes: "Generate Redemption Codes", codeCount: "Count (1–100)", codeValue: "Credits per code", expiry: "Expiry (optional)", generateCodes: "Generate", copyCodesNow: "Codes are shown once. Copy them now.", codeStatus: "Code status", voidCode: "Void", codeState: { available: "Available", redeemed: "Redeemed", void: "Voided", expired: "Expired" } });
Object.assign(en.workflows, { conversation: "Run conversation", followUpPlaceholder: "Continue the conversation — Enter to send, Shift + Enter for a new line", activityDetails: "View execution progress", command: "Command", reasoningSummary: "Reasoning summary", runtimePrepared: "Runtime prepared", toolCompleted: "Tool call completed", updatingFiles: "Updating files", streamingAnswer: "Generating the answer", answerReady: "Answer ready", howToIntegrate: "How to integrate", credentialUnavailable: "This credential was generated by an older version and its Secret cannot be recovered. Regenerate it to continue.", apiKeyLabel: "API Key", apiSecretLabel: "API Secret", showSecret: "Show API Secret", hideSecret: "Hide API Secret", integrationGuideTitle: "How to integrate this Workflow", integrationStepToken: "1. Define a request function that automatically exchanges JWTs (Bash)", integrationStepInvoke: "2. Create a Run (returns a Run ID)", integrationStepStream: "3. Stream output with SSE", integrationStepFullOutput: "4. Fetch the complete output once", integrationTokenHint: "Define this function in the same Bash session before running the commands below. It exchanges a JWT before every request, so no manual renewal is needed. Store the API Key and Secret securely.", integrationIdempotencyHint: "Use a unique Idempotency-Key for each business request and keep it unchanged when retrying that request.", integrationRunHint: "The create endpoint immediately returns 202 and a Run ID; save that ID as RUN_ID.", integrationStreamHint: "SSE replays historical events, then streams live events until the Run reaches a terminal state.", integrationFullOutputHint: "After the Run completes, call this endpoint once; the complete result is in final_text or final_json." });
Object.assign(en.sessions, { activitySummary: { runtime: { running: "Preparing the runtime", completed: "Runtime prepared" }, reasoning: { running: "Analyzing the request", completed: "Request analyzed" }, feishuChatSearchHelp: { running: "Use the Feishu Connector to read chat search instructions", completed: "Used the Feishu Connector to read chat search instructions" }, feishuChatSearch: { running: "Use the Feishu Connector to search chats", completed: "Used the Feishu Connector to search chats" }, feishuMessageSendHelp: { running: "Use the Feishu Connector to read message instructions", completed: "Used the Feishu Connector to read message instructions" }, feishuMessageSend: { running: "Use the Feishu Connector to send a message", completed: "Used the Feishu Connector to send a message" }, tool: { running: "Run a tool operation", completed: "Ran a tool operation" }, file: { running: "Updating files", completed: "Updated files" }, activity: { running: "Processing the task", completed: "Completed processing" } } });
Object.assign(en.sessions, { executionEvidence: { title: "This execution", tools: "{count} tool call | {count} tool calls", files: "{count} file change | {count} file changes", stages: "{count} execution stage | {count} execution stages", artifacts: "{count} artifact | {count} artifacts", sources: "{count} source | {count} sources", noExternal: "No external tools, file changes, or sources recorded", running: "In progress", completed: "Completed", stage: "Stage {position}", openSource: "Open source", sourceUnavailable: "The source changed, was deleted, or is no longer available to you", sourceKind: { file: "File", knowledge: "Knowledge", connector: "Connector", artifact: "Artifact" }, sourceState: { requested: "Requested", succeeded: "Used", failed: "Failed", not_used: "Not used" }, sourceAction: { retrieved: "Relevant content retrieved", noMatch: "No relevant indexed source", retrievalFailed: "Retrieval failed", retrievalUnavailable: "Retrieval unavailable", indexUnavailable: "Ready index unavailable", notInvoked: "Not invoked in this execution" }, kind: { runtime: "Runtime", reasoning: "Analysis", tool: "Tool", file: "File", activity: "Execution" } } });
Object.assign(en.workflows, { gitCredentialSaved: "Credentials are saved and cannot be viewed. Enter them again to clone again.", replaceGitCredential: "Enter new credentials", gitPrivateKeyDraft: "New SSH private key (not saved)", gitPasswordDraft: "New password / token (not saved)" });
Object.assign(en.workflows, { gitErrors: {
  git_source_invalid: "Invalid Git configuration. Check that the repository URL matches the authentication method, the branch is filled in, and Git / SSH config follows the field guidance.",
  git_credentials_required: "Enter the password / token or SSH private key for this clone attempt. Saved credentials are not displayed.",
  git_workspace_not_empty: "The Workspace already contains files. Back them up and clear the Workspace, or create an empty Workflow before retrying. Existing files have not been deleted.",
  git_server_unavailable: "Server Git / SSH or host verification configuration is unavailable. Contact an Administrator.",
  git_ssh_host_untrusted: "SSH host verification failed. Ask an Administrator to verify the repository host fingerprint and update the trusted hosts configuration.",
  git_ssh_key_invalid: "The SSH private key could not be read. Check the complete key markers and line breaks, and use a key that does not require an interactive passphrase.",
  git_authentication_failed: "Git authentication failed. Check the account, token or SSH key and the account's repository access.",
  git_branch_not_found: "The specified branch was not found. Check the branch name.",
  git_repository_unavailable: "The repository does not exist or is inaccessible. Check its address and access permissions.",
  git_connection_failed: "The Git server could not be reached or the connection timed out. Check the address, port and network, then retry.",
  git_repository_too_large: "The cloned repository exceeds the Workspace limit of 1 GiB.",
  git_clone_failed: "Save and clone failed. Check the repository URL, branch, credentials and that the Workspace is empty, then retry. Your input has been kept.",
} });

Object.assign(en.workflows, { apiTokenDescription: "The API Key and API Secret have no fixed expiration and remain valid until regenerated or revoked. They are encrypted at rest and can be viewed or copied here. JWTs expire after 72 hours; your integration can exchange a new one before every request.", gitAuthentication: "Authentication", gitPublic: "Public HTTPS", gitAccount: "Username and password", gitUsername: "Git username", gitPassword: "Git password / token", gitURLHelp: "Supports HTTPS URLs and SSH addresses in git{'@'}host:path format.", gitSaved: "Git repository cloned and saved", cloneRepository: "Save and clone repository" });
Object.assign(en.sessions, { addAttachment: "Attach images or files", removeAttachment: "Remove attachment {name}", attachmentLimits: "Up to 10 attachments per message, 100 MB each" });
Object.assign(en.sessions.progress, { finalizing: "Saving the session securely" });
Object.assign(en.nav, { resources: "Experts / Skills / Connectors" });
Object.assign(en.sessions, { resourceActionSkill: "Skill creation preview", resourceActionExpert: "Expert creation preview", resourceActionConnector: "Connector creation preview", resourceActionConfirm: "Confirm creation", resourceActionConfirmed: "Created and installed", resourceActionCancelled: "Cancelled", resourceActionExpired: "Action expired or failed" });
Object.assign(zh.sessions, { failureTitle: "本次响应失败", failureLocation: "失败位置", failureResponse: "Agent 响应", failureReason: "原因", failureCompleted: "已完成的部分", failurePartialKept: "失败前生成的内容已保留", failureNoResult: "没有可保留的结果", retryStep: "重试这一步" });
Object.assign(en.sessions, { failureTitle: "This response failed", failureLocation: "Failure point", failureResponse: "Agent response", failureReason: "Reason", failureCompleted: "Completed work", failurePartialKept: "Content produced before the failure was retained", failureNoResult: "No reusable result was produced", retryStep: "Retry this step" });
Object.assign(zh.sessions, { search: "搜索会话", searchPlaceholder: "搜索会话" });
Object.assign(en.sessions, { search: "Search sessions", searchPlaceholder: "Search sessions" });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { createConnectorInConversation: "在会话中创建", createConnectorInConversationHint: "使用创建连接器技能，生成并安装 MCP 连接器。" });
Object.assign(en, { resources: { title: "Skills & Connectors", centerTitle: "Resource Center", subtitle: "Manage reusable Skills and Connectors", skills: "Skills", connectors: "Connectors", cli: "Third-party CLI", cliDefinition: "CLI Connector Definition", continueSetup: "Continue authorization", enable: "Enable", exactVersion: "Exact version", executable: "Executable", npmPackage: "npm package", npmIntegrity: "npm integrity", feishuCLI: "Feishu CLI", deleteAffected: "Deleting “{resource}” also detaches it from these Experts: {experts}. Historical snapshots stay unchanged.", deleteUnaffected: "Delete “{resource}”? Historical snapshots stay unchanged.", capabilities: "Capability policies", capabilityId: "Capability ID", argvPrefix: "Argument prefix", risk: "Risk", lowRisk: "Low risk", highRisk: "High risk", identities: "Execution identities", scopes: "Required scopes", egressHosts: "Allowed Egress hosts", timeoutSeconds: "Timeout (seconds)", architectures: "Supported architectures", recommendedSkills: "Recommended Skills", state: { draft: "Draft", building: "Building", testing: "Testing", available: "Available", failed: "Failed", disabled: "Disabled" }, health: { enabled: "Enabled by {count} users", waiting: "{count} waiting for user action", authorization: "{active} active authorizations · {attention} need attention" } } });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { createConnectorInConversation: "Create in conversation", createConnectorInConversationHint: "Use the Create Connector Skill to install an MCP Connector." });
Object.assign(en, { approvals: { title: "Confirm {connector} action", messageSendDescription: "The assistant is about to send a message to the target below through {connector}. Approving permits this send once; rejecting stops it.", otherDescription: "The assistant is about to perform the action below through {connector}. Approving permits this action once; rejecting stops it.", operation: "Requested action", sendMessage: "Send a message", target: "Target ID", groupTarget: "Target group ID", hiddenArguments: "Message content and command arguments are hidden here. If you did not request this action or need to review the exact content, reject it and ask the assistant to explain first.", expires: "Confirm before {time}; this request expires automatically afterward.", identity: "Execution identity", user: "Send as user", bot: "Send as bot", reject: "Reject", approveOnce: "Approve once" } });
Object.assign(en.sessions, { connectorAuthorizationWait: "Waiting for {provider} authorization: this action has not run. Open the {provider} authorization prompt in the composer below, complete authorization, then return and reply “Authorized” or select Retry. If no authorization action appears, check the account authorization in Connectors." });
Object.assign((zh as unknown as { approvals: Record<string, string> }).approvals, { impact: "影响范围", irreversible: "不可逆点", irreversibleHint: "外部系统中的更改可能无法由 Agent 自动撤销", basis: "执行依据" });
Object.assign((en as unknown as { approvals: Record<string, string> }).approvals, { impact: "Impact", irreversible: "Irreversible point", irreversibleHint: "Changes in the external system may not be automatically reversible", basis: "Basis" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { operationFailed: "The operation failed. Please try again or contact your administrator if it continues.", deleteCLI: "Delete “{resource}”? This removes access for all users, detaches it from Experts, and disconnects account authorizations. Historical snapshots and files are retained.", publish: "Publish", disable: "Disable", developerConsole: "Developer console", authorizeAccount: "Authorize Feishu account", authorizeNow: "Open Feishu authorization", authorizedAccount: "Authorized: {name}", disconnectAccount: "Disconnect account", authorizationPending: "Waiting for authorization in Feishu", recommendedSkillOffer: "Recommended Skill: {name}", recommendedSkillWarning: "Recommended for this Expert: {name}", installSkill: "Install Skill", selectSkill: "Select Skill" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { expandAuthorization: "Grant required Feishu permissions" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { loginRequired: "Your session has expired. Sign in again and retry.", permissionDenied: "You do not have permission to perform this action. Contact your administrator.", resourceMissing: "This Skill or Connector no longer exists. Refresh the page.", resourceChanged: "This content changed elsewhere. Refresh and retry.", invalidInput: "Check the entered values and package format, then retry.", invalidNPMInstall: "The npm package spec must include an exact version, such as package@1.2.3. Pasted npm install commands must include the version too.", authorizationInvalidInput: "Could not start Feishu account authorization. Refresh the page and retry, or contact your administrator if it continues.", uploadTooLarge: "The upload is too large. Reduce the file size and retry. ZIP files must be at most 50 MiB.", tooManyRequests: "Too many requests. Wait a moment and retry.", serviceUnavailable: "The service is temporarily unavailable. Please try again later.", networkFailed: "The network request failed. Check your connection and retry.", statusUpdateFailed: "Some content or status could not be updated. Refresh and retry later. The current content is retained." });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { newSkill: "Add Skill", createSkill: "Create Skill", uploadSkill: "Upload Skill", systemSkillUnavailable: "The platform default Skill is unavailable. Contact an Administrator.", newConnector: "New Connector", chooseConnectorType: "New Connector", chooseConnectorTypeHint: "Choose how this Connector is integrated. It will appear in the same Connector catalog after saving.", mcpConnector: "MCP Connector", mcpConnectorHint: "Connect to a Streamable HTTP service or run an exact-version stdio package.", cliConnector: "CLI Connector", cliConnectorHint: "Configure an exact-version third-party command and its capability policies.", administratorOnly: "Only an Administrator can create CLI Connectors." });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { installCLI: "Install third-party CLI", editCLI: "Reinstall third-party CLI", icon: "Icon", capabilityDescription: "Capability description", installationType: "Installation method", npmInstall: "Install from npm", npmPackageSpec: "npm package spec", zipUpload: "Upload ZIP package", exactNPMHint: "Enter an exact package spec, for example {'@'}scope/package{'@'}1.2.3. You may also paste an npm install command with -g, but it must include the version.", cliZipHint: "The ZIP root must contain package.json; runtime metadata is supplied by the package and validated by the platform.", install: "Install", noCapabilityDescription: "No capability description", feishuCapability: "Read, search, and operate on Feishu documents, calendars, messages, and other Open Platform resources." });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { iconUploadHint: "PNG, JPEG, WebP, or GIF up to 384KB." });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { platformSkills: "Platform Skills", mySkills: "My Skills", platformConnectors: "Platform Connectors", myConnectors: "My Connectors" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { connected: "Connected", setupRequired: "Setup required", availableToInstall: "Available to install", packageVersion: "Version {version}", conformanceAvailable: "Runtime verified", conformanceUnavailable: "Runtime verification unavailable", selectedAccount: "Selected account", refreshAuthorization: "Refresh authorization", upgrade: "Upgrade", uninstall: "Uninstall" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { connect: "Connect", wecomBotId: "Bot ID", wecomSecret: "Secret", providedCredentialsInvalid: "Enter valid WeCom Bot credentials." });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { modaoToken: "Personal-space token", modaoTokenHelp: "Create a token in Modao's avatar menu under Token Settings", modaoCredentialsInvalid: "Enter a valid Modao personal-space token without spaces or line breaks." });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { deactivate: "Deactivate" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { connectorPackageUpload: "Upload Connector", connectorPackageUploadHint: "Upload a validated ZIP. The installation appears in this same Connector catalog." });
Object.assign(en.experts, { platformExperts: "Platform Experts", myExperts: "My Experts" });
Object.assign(en.experts, {
  teams: "Expert Teams", catalog: "Expert catalog", createTeam: "Create Expert Team", editExpert: "Edit Expert", editTeam: "Edit Expert Team",
  searchExperts: "Search Experts by name or capability", searchTeams: "Search teams by name or capability", expertiseFilter: "Filter by category", categoryFilter: "Filter by category", category: "Category", all: "All",
  loadFailed: "Could not load the Expert catalog", loadExpertFailed: "Could not load the Expert", loadTeamFailed: "Could not load the Expert Team", incomplete: "Incomplete",
  teamUnavailable: "At least two available Experts required", noExperts: "No matching Experts", noTeams: "No matching Expert Teams", perRound: "{count} Experts / round",
  backCatalog: "Back to Expert catalog", backTeams: "Back to Expert Teams", capability: "Capability introduction", instruction: "Execution instruction", expertise: "Expertise",
  basic: "Basic information", basicHint: "The capability introduction is display-only; the execution instruction is injected while this Expert runs.", instructionHint: "This is the Expert's only role instruction. No hidden prompt is added.",
  expertiseHint: "Up to 10 tags, 20 characters each.", addTag: "Add tag", tagPlaceholder: "Type and press Enter", extensions: "Extensions",
  extensionsHint: "Bind the MCP servers and Skills this Expert may use.", extensionFailed: "Extension operation failed", saved: "Expert saved", teamSaved: "Expert Team saved",
  saveFailed: "Save failed. Check the name, capability introduction, and execution instruction.", teamSaveFailed: "Save failed. A team requires 2–10 distinct available Experts.",
  operationFailed: "Operation failed", saveSucceeded: "Saved", teamInfo: "Team information", teamInfoHint: "The introduction is display-only. A team has no additional execution instruction.",
  teamName: "Team name", members: "Team members", membersHint: "Experts run from top to bottom. Each Expert receives the preceding final results.",
  chooseExpert: "Choose an available Expert", add: "Add", sequential: "Sequential", moveUp: "Move {name} up", moveDown: "Move {name} down", removeMember: "Remove {name}",
  deleteExpertTitle: "Delete “{name}”?", deleteExpertHint: "The Expert is removed from editable teams. Frozen historical Session and Run snapshots remain unchanged.",
  modelUnavailable: "Model unavailable", incompatible: "Incompatible", confirmProfile: "I confirm this model and Runtime Engine for the Expert", confirmProfileRequired: "Confirm the Expert's model and Runtime Engine",
  deleteTeamHint: "Workflows and unstarted Sessions fall back to no Expert. Historical snapshots remain unchanged.", confirmDelete: "Confirm delete", deleteExpertFailed: "Could not delete the Expert", deleteTeamFailed: "Could not delete the Expert Team",
  icon: "Icon", iconBackground: "Icon background", introduction: "Introduction", coreCapability: "Core capability", operatingProcedure: "Operating procedure", outputStandard: "Output standard", cautions: "Cautions (optional)", sage: "Sage", sand: "Sand", sky: "Sky", coral: "Coral", resourceCounts: "{skills} Skills · {connectors} Connectors", memberName: "Member name", memberLabels: "Member labels (up to 5)", tagGenerating: "Generating tags", tagFailed: "Tag generation failed"
});
Object.assign(zh.settings, { subtitle: "定义你的默认个性、模型与 Runtime" });
Object.assign(en.settings, { subtitle: "Choose your default personality, model, and Runtime" });
Object.assign(zh.experts, { extensions: "技能与连接器", extensionsHint: "绑定此专家可以使用的技能与连接器。", extensionFailed: "技能或连接器操作失败" });
Object.assign(en.experts, { extensions: "Skills & Connectors", extensionsHint: "Bind the Skills and Connectors this Expert may use.", extensionFailed: "Skill or Connector operation failed" });
Object.assign(zh.experts, { selectSkillsHint: "选择要提供给此专家的技能。", selectConnectorsHint: "选择已就绪的 MCP 或 CLI 连接器。", selectedCount: "已选 {count}" });
Object.assign(en.experts, { selectSkillsHint: "Choose the Skills available to this Expert.", selectConnectorsHint: "Choose ready MCP or CLI Connectors.", selectedCount: "{count} selected" });
Object.assign(zh.experts, { iconTeam: "团队", iconSparkles: "闪光", iconCompass: "指南针" });
Object.assign(en.experts, { iconTeam: "Team", iconSparkles: "Sparkles", iconCompass: "Compass" });
Object.assign(zh.experts, { deleteExpertHint: "被专家团引用的专家不允许删除；请先移除对应成员。历史快照不会改变。" });
Object.assign(en.experts, { deleteExpertHint: "An Expert referenced by an Expert Team cannot be deleted; remove those members first. Historical snapshots stay unchanged." });

Object.assign((zh as unknown as { resources: Record<string, string> }).resources, {
  importSkill: "导入技能", updateSkill: "更新技能", gitAddress: "Git 地址", gitBranchOptional: "分支（可选）", defaultBranchHint: "留空使用仓库默认分支",
  chooseSkillArchive: "选择包含 SKILL.md 的 ZIP 包", skillArchiveReady: "ZIP 包已选择",
  skillDetail: "技能详情", backSkills: "返回技能", skillLoadFailed: "无法加载技能详情", noSkillDescription: "该技能未提供当前语言的描述。",
  documentVersion: "版本", previewDocument: "预览技能文档", rawDocument: "查看 SKILL.md 源文件"
});
Object.assign((en as unknown as { resources: Record<string, string> }).resources, {
  importSkill: "Import Skill", updateSkill: "Update Skill", gitAddress: "Git URL", gitBranchOptional: "Branch (optional)", defaultBranchHint: "Leave blank to use the repository default branch",
  chooseSkillArchive: "Choose a ZIP package containing SKILL.md", skillArchiveReady: "ZIP package selected",
  skillDetail: "Skill details", backSkills: "Back to Skills", skillLoadFailed: "Could not load the Skill details", noSkillDescription: "This Skill has no description for the current language.",
  documentVersion: "Version", previewDocument: "Preview Skill document", rawDocument: "View SKILL.md source"
});
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { connectorType: "连接器类型", connectionAddress: "连接地址", connectedApplication: "已连接应用", mcpDetailDescription: "通过标准 MCP 协议为会话、工作流和专家提供外部能力。" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { connectorType: "Connector type", connectionAddress: "Connection", connectedApplication: "Connected application", mcpDetailDescription: "Provides external capabilities to Sessions, Workflows, and Experts through the standard MCP protocol." });
Object.assign((zh as unknown as { resources: Record<string, string> }).resources, {
  githubVerifyCode: "请在 GitHub 授权页面输入验证码：{code}。",
  notionVerifyCode: "请核对验证码：{code}。",
  connectorAuthorizeNow: "打开授权页面", connectorAuthorizationPending: "等待在对应应用中完成授权", connectorAuthorizationInvalidInput: "无法发起账号授权，请刷新页面后重试。",
  dingtalkCLIAccessDisabled: "钉钉账号已确认授权，但当前企业或账号尚未获得 CLI 使用权限。请联系钉钉企业管理员检查开放范围，处理后再点击“继续完成授权”。",
  dingtalkCLIEnterpriseDenied: "钉钉账号已确认授权，但未通过企业的 CLI 安全认证。请联系钉钉企业管理员开放 CLI 使用权限；处理后重新授权。",
  dingtalkCLIUserDenied: "钉钉账号已确认授权，但当前账号不在企业允许使用 CLI 的人员范围内。请联系钉钉企业管理员调整开放范围；处理后重新授权。",
  dingtalkCLIChannelRequired: "钉钉账号已确认授权，但企业启用了 CLI 渠道管控，当前连接器没有可用的授权渠道。请联系平台管理员核对渠道配置和企业开放范围；处理后重新授权。",
  dingtalkCLIAuthExpired: "钉钉账号已确认授权，但钉钉未接受本次 CLI 权限凭证。请重新授权；若仍失败，请联系钉钉企业管理员检查账号权限。",
  dingtalkIdentityMismatch: "钉钉返回的账号与本次授权企业不一致。请确认当前登录的企业账号，然后重新授权。",
  dingtalkAuthorizationFailed: "钉钉已确认授权，但连接器未能完成账号连接。请点击“继续完成授权”重新授权；若仍失败，请联系管理员检查服务日志。"
});
Object.assign((en as unknown as { resources: Record<string, string> }).resources, {
  githubVerifyCode: "Enter this code on the GitHub authorization page: {code}.",
  notionVerifyCode: "Confirm the verification code: {code}.",
  connectorAuthorizeNow: "Open authorization page", connectorAuthorizationPending: "Waiting for authorization in the connected app", connectorAuthorizationInvalidInput: "Could not start account authorization. Refresh and retry.",
  dingtalkCLIAccessDisabled: "DingTalk authorization was approved, but this organization or account does not have CLI access. Ask your DingTalk administrator to check access, then continue authorization.",
  dingtalkCLIEnterpriseDenied: "DingTalk approved the account sign-in but denied enterprise CLI security access. Ask your DingTalk administrator to enable CLI access, then authorize again.",
  dingtalkCLIUserDenied: "DingTalk approved the account sign-in, but this account is outside the enterprise CLI access scope. Ask your DingTalk administrator to include it, then authorize again.",
  dingtalkCLIChannelRequired: "DingTalk approved the account sign-in, but this enterprise restricts CLI channels and this Connector has no approved channel. Ask your platform administrator to check channel configuration and enterprise access, then authorize again.",
  dingtalkCLIAuthExpired: "DingTalk approved the account sign-in but rejected the CLI credential. Authorize again; if this continues, ask your DingTalk administrator to check account access.",
  dingtalkIdentityMismatch: "DingTalk returned an account from a different organization. Confirm the signed-in organization and authorize again.",
  dingtalkAuthorizationFailed: "DingTalk authorization was approved, but the account could not be connected. Continue authorization to retry; if it still fails, ask an administrator to check the service logs."
});

Object.assign(zh, { composer: {
  add: "添加到对话", skills: "技能", connectors: "连接器", send: "发送", search: "搜索", empty: "没有匹配的技能",
  placeholder: "输入消息，/ 选择技能，{'@'} 引用对话文件", removeToken: "移除 {name}", useSkill: "去使用", summon: "召唤",
  manageSkills: "管理技能", manageConnectors: "管理连接器", moreExperts: "召唤更多专家", localFiles: "从本地添加文件", localSkill: "本地上传的技能",
  parentFolder: "返回上一级文件夹", conversationFiles: "对话文件", fileUnavailable: "文件已过期或不可用", resourceUnavailable: "资源不可用", version: "版本 {version}",
  connectorUnavailable: "连接器当前不可用", connectorInactive: "打开右侧开关即可激活", connectorCredentialsRequired: "先在连接器管理页填写凭证", connectorActivation: "激活{name}", connectorSelected: "已用于当前对话", connectorNotSelected: "未用于当前对话", selectionFailed: "资源选择未保存，请重试或到管理页检查可用状态。", selectionRecovered: "此前保存的连接器选择已失效，请重新选择。", filesFailed: "无法读取对话文件，请重试。",
  sendFailed: "发送未完成，草稿已保留，请检查资源或文件后重试。", reselectFiles: "这些文件尚未上传，请重新选择：{names}",
  activationRequired: "{name} 需要完成应用激活", activationHint: "请在飞书页面完成应用创建；返回后系统会继续账号授权。",
  providerFeishu: "飞书", providerDingtalk: "钉钉", authorizeNow: "打开{provider}授权",
  authorizationRequired: "{name} 需要{provider}账号授权", authorizationHint: "点击打开{provider}授权；完成后回到会话并回复“已授权”即可继续。",
  activationAuthorizationHint: "请在{provider}页面完成授权；完成后即可在当前会话中使用。", activationAuthorizationCompleted: "连接器已激活并完成授权", activationAuthorizationReady: "现在可以在当前会话中使用此连接器。",
  authorizationCompleted: "{provider}授权已完成", authorizationContinue: "请在会话中回复“已授权”，助手会继续刚才的操作。",
} });
Object.assign(en, { composer: {
  add: "Add to conversation", skills: "Skills", connectors: "Connectors", send: "Send", search: "Search", empty: "No matching Skills",
  placeholder: "Write a message, / to select Skills, {'@'} to reference conversation files", removeToken: "Remove {name}", useSkill: "Use Skill", summon: "Summon",
  manageSkills: "Manage Skills", manageConnectors: "Manage Connectors", moreExperts: "Discover more Experts", localFiles: "Add local files", localSkill: "Uploaded Skill",
  parentFolder: "Parent folder", conversationFiles: "Conversation files", fileUnavailable: "File expired or unavailable", resourceUnavailable: "Resource unavailable", version: "Version {version}",
  connectorUnavailable: "This Connector is unavailable", connectorInactive: "Turn on the switch to activate", connectorCredentialsRequired: "Enter credentials in Connector management first", connectorActivation: "Activate {name}", connectorSelected: "Used in this conversation", connectorNotSelected: "Not used in this conversation", selectionFailed: "Selection was not saved. Retry or check the resource in its management page.", selectionRecovered: "A previously saved Connector selection is no longer available. Please select it again.", filesFailed: "Could not load conversation files. Please retry.",
  sendFailed: "The message was not sent. Your draft is preserved; check its resources and files, then retry.", reselectFiles: "These files were not uploaded. Select them again: {names}",
  activationRequired: "{name} needs application activation", activationHint: "Complete application creation in Feishu. Account authorization continues when you return.",
  providerFeishu: "Feishu", providerDingtalk: "DingTalk", authorizeNow: "Open {provider} authorization",
  authorizationRequired: "{name} needs {provider} account authorization", authorizationHint: "Open {provider} authorization, then return and reply “Authorized” to continue.",
  activationAuthorizationHint: "Complete authorization in {provider}. The Connector will then be ready in this conversation.", activationAuthorizationCompleted: "Connector activated and authorized", activationAuthorizationReady: "This Connector is ready to use in the current conversation.",
  authorizationCompleted: "{provider} authorization completed", authorizationContinue: "Reply “Authorized” in this conversation and the assistant will continue the previous operation.",
} });
Object.assign(zh.sessions, { welcome: "从一个问题开始，随时添加专家、技能或连接器。" });
Object.assign(en.sessions, { welcome: "Start with a question and add an Expert, Skill, or Connector whenever you need one." });
Object.assign(zh, { attachments: { preview: "预览图片 {name}", download: "下载附件 {name}", previous: "上一张图片", next: "下一张图片", position: "第 {current} 张，共 {total} 张", navigationHint: "使用方向键切换图片，Esc 关闭" } });
Object.assign(en, { attachments: { preview: "Preview image {name}", download: "Download attachment {name}", previous: "Previous image", next: "Next image", position: "{current} of {total}", navigationHint: "Use the arrow keys to move between images and Esc to close" } });
Object.assign(zh, { imageGeneration: { title: "图片创作", subtitle: "从提示词或参考图创作私人图片", settings: "创作设置", results: "生成结果", textToImage: "文生图", imageToImage: "图生图", model: "图片模型", prompt: "提示词", promptPlaceholder: "描述画面主体、构图、风格、光线和细节…", size: "尺寸", count: "数量", quality: "质量", format: "输出格式", background: "背景", opaque: "不透明", transparent: "透明", estimate: "最多预留 {value} 积分", generate: "创作图片", stop: "停止创作", empty: "输入提示词并开始创作，结果会显示在这里。", noModel: "管理员尚未配置可用的图片模型。", configure: "配置图片模型", history: "历史记录", pending: "等待 Worker", running: "正在创作", succeeded: "创作完成", partially_succeeded: "部分完成", failed: "创作失败", cancelled: "已停止", outcome_unknown: "供应商结果未知", download: "下载", adminTitle: "图片模型设置", addModel: "新增图片模型", verify: "测试", enable: "启用", disable: "停用", connection: "模型供应商连接", providerModelId: "模型 ID", capabilities: "能力选项（逗号分隔）", rate: "每张图片积分", created: "图片模型已创建，请测试后启用。", requestFailed: "操作未完成，请检查配置后重试。" } });
Object.assign(en, { imageGeneration: { title: "Image Creation", subtitle: "Create private images from a prompt or visual references", settings: "Creation settings", results: "Results", textToImage: "Text to image", imageToImage: "Image to image", model: "Image Model", prompt: "Prompt", promptPlaceholder: "Describe the subject, composition, style, lighting, and details…", size: "Size", count: "Quantity", quality: "Quality", format: "Output format", background: "Background", opaque: "Opaque", transparent: "Transparent", estimate: "Reserves up to {value} Credits", generate: "Create images", stop: "Stop creation", empty: "Enter a prompt and create. Results appear here.", noModel: "No available Image Model has been configured.", configure: "Configure Image Models", history: "History", pending: "Waiting for Worker", running: "Creating", succeeded: "Complete", partially_succeeded: "Partially complete", failed: "Failed", cancelled: "Stopped", outcome_unknown: "Provider outcome unknown", download: "Download", adminTitle: "Image Model settings", addModel: "Add Image Model", verify: "Test", enable: "Enable", disable: "Disable", connection: "Model Provider Connection", providerModelId: "Model ID", capabilities: "Capability options (comma separated)", rate: "Credits per image", created: "Image Model created. Test it before enabling.", requestFailed: "The operation did not complete. Check the configuration and retry." } });
Object.assign(zh, { aiApplications: { title: "AI 应用", subtitle: "管理智能助手和图片创作", comingSoon: "即将支持", create: "创建", name: "名称", goal: "服务目标", answerScope: "回答范围", answerScopePlaceholder: "例如：仅回答产品使用、售后政策和账户操作相关问题", rules: "回答规则", responseStyle: "回答风格", state: "启用状态", startConversation: "开始对话", assistantNamePlaceholder: "例如：售后答疑助手", loadFailed: "加载 AI 应用失败", saveFailed: "保存 AI 应用失败", deleteFailed: "删除 AI 应用失败", shared: "已开启分享", private: "仅自己可用", tabs: { assistants: { label: "智能助手", english: "Smart Assistants" }, imageCreation: { label: "图片创作", english: "Image Creation" } }, faq: { title: "常见问题", question: "问题", answer: "答案（Markdown）", add: "添加问题", enable: "启用", disable: "停用" }, share: { title: "分享与嵌入", enabled: "开启分享", width: "iframe 宽度", height: "iframe 高度", allowedOrigins: "允许嵌入来源", allowedOriginsPlaceholder: "每行一个完整来源，例如 https://example.com；留空表示不限制", dailyLimit: "每日调用上限（0 表示不限）", freeText: "允许自由提问", regenerate: "生成新的分享 Token" }, knowledge: { title: "知识库", subtitle: "维护助手可检索的文档内容", hint: "选择知识库后，助手会在未命中预设问题时检索其中的文档。", selected: "已绑定知识库", selectPlaceholder: "选择知识库", manage: "管理知识库", create: "创建知识库", description: "描述", list: "知识库列表", empty: "还没有知识库。", noDescription: "暂无描述", selectTitle: "选择一个知识库", selectHint: "请选择知识库查看文档", documentsEmpty: "还没有文档。", documentName: "文档名称", documentContent: "文档内容", addDocument: "添加文档" }, assistants: { title: "智能助手", subtitle: "面向具体场景的可复用 AI 应用", empty: "还没有智能助手。", defaultIntro: "已创建，可继续配置回答范围、FAQ 和知识库。" } } });
Object.assign(en, { aiApplications: { title: "AI Applications", subtitle: "Manage Smart Assistants and Image Creation", comingSoon: "Coming soon", create: "Create", name: "Name", goal: "Service goal", answerScope: "Answer scope", answerScopePlaceholder: "e.g. Only answer product usage, after-sales policy, and account operation questions", rules: "Response rules", responseStyle: "Response style", state: "Enabled state", startConversation: "Start conversation", assistantNamePlaceholder: "e.g. Product support assistant", loadFailed: "Unable to load AI applications", saveFailed: "Unable to save AI application", deleteFailed: "Unable to delete AI application", shared: "Shared", private: "Private", tabs: { assistants: { label: "Smart Assistants", english: "Smart Assistants" }, imageCreation: { label: "Image Creation", english: "Image Creation" } }, faq: { title: "Frequently Asked Questions", question: "Question", answer: "Answer (Markdown)", add: "Add FAQ", enable: "Enable", disable: "Disable" }, share: { title: "Sharing and embed", enabled: "Enable sharing", width: "iframe width", height: "iframe height", freeText: "Allow free-text questions", regenerate: "Generate new share Token" }, knowledge: { title: "Knowledge bases", subtitle: "Maintain documents that assistants can retrieve", hint: "After selecting a knowledge base, the assistant retrieves its documents when no FAQ matches.", selected: "Bound knowledge bases", selectPlaceholder: "Select knowledge bases", manage: "Manage knowledge bases", create: "Create knowledge base", description: "Description", list: "Knowledge bases", empty: "No knowledge bases yet.", noDescription: "No description", selectTitle: "Select a knowledge base", selectHint: "Select a knowledge base to view documents", documentsEmpty: "No documents yet.", documentName: "Document name", documentContent: "Document content", addDocument: "Add document" }, assistants: { title: "Smart Assistants", subtitle: "Reusable AI applications for focused scenarios", empty: "No Smart Assistants yet.", defaultIntro: "Created. Continue configuring FAQs, scope, and knowledge." } } });
Object.assign((zh as unknown as { aiApplications: Record<string, unknown> }).aiApplications, { search: "搜索智能助手", searchAction: "搜索", scenario: "场景类型", allScenarios: "全部场景", enable: "启用", disable: "停用", confirmDelete: "确认删除“{name}”？历史对话仍会保留快照。", states: { draft: "草稿", enabled: "已启用", disabled: "已停用" }, scenarios: { "customer-consultation": "客服咨询", "pre-sales-advisor": "售前顾问", "after-sales-support": "售后支持", "product-guide": "商品导购", "enterprise-knowledge": "企业知识助手", recruitment: "招聘助手", training: "培训助手", custom: "自定义" } });
Object.assign((en as unknown as { aiApplications: Record<string, unknown> }).aiApplications, { search: "Search Smart Assistants", searchAction: "Search", scenario: "Scenario", allScenarios: "All scenarios", enable: "Enable", disable: "Disable", confirmDelete: "Delete “{name}”? Historical conversations keep their snapshots.", states: { draft: "Draft", enabled: "Enabled", disabled: "Disabled" }, scenarios: { "customer-consultation": "Customer consultation", "pre-sales-advisor": "Pre-sales advisor", "after-sales-support": "After-sales support", "product-guide": "Product guide", "enterprise-knowledge": "Enterprise knowledge", recruitment: "Recruitment", training: "Training", custom: "Custom" } });
Object.assign((zh as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { optimize: "优化提示词", optimizationModel: "文本优化模型" });
Object.assign((en as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { optimize: "Optimize prompt", optimizationModel: "Prompt optimization model" });
Object.assign((zh as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { useAsReference: "用作参考图", noEditModel: "没有支持图生图的可用模型", endpoint: "API 地址", apiKey: "API Key", keepApiKey: "留空则保留当前独立 API Key", apiKeyRequired: "必须填写独立的 API Key", secretSet: "密钥已配置", secretMissing: "缺少密钥", verifyFailed: "图片模型测试失败，请检查图片 API 地址、API Key 和模型 ID。", optimizationPrompt: "提示词优化 Prompt" });
Object.assign((en as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { useAsReference: "Use as reference", noEditModel: "No available model supports image editing", endpoint: "API endpoint", apiKey: "API Key", keepApiKey: "Leave blank to keep the current independent API Key", apiKeyRequired: "A separate API Key is required", secretSet: "Key configured", secretMissing: "Key missing", verifyFailed: "Image Model test failed. Check the Image API endpoint, API Key, and model ID.", optimizationPrompt: "Prompt optimization instruction" });
Object.assign((zh as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { regenerateEstimate: "将按当前模型和费率重新生成整批图片，最多预留 {value} 积分。继续吗？" });
Object.assign((en as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { regenerateEstimate: "Regenerate the full batch with the current model and rate, reserving up to {value} Credits?" });
Object.assign((zh as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { customSize: "自定义尺寸", customSizePlaceholder: "例如 1280x720", customSizeHelp: "请输入宽x高，总像素不超过 6400 万", customSizeInvalid: "尺寸格式无效或超过 6400 万像素" });
Object.assign((en as unknown as { imageGeneration: Record<string,string> }).imageGeneration, { customSize: "Custom size", customSizePlaceholder: "For example, 1280x720", customSizeHelp: "Enter widthxheight, up to 64 million pixels", customSizeInvalid: "Invalid size or more than 64 million pixels" });
Object.assign(zh, { knowledgeBases: { eyebrow: "可复用知识", title: "知识库", subtitle: "整理文档并为工作流提供可追溯的检索上下文", new: "新建知识库", edit: "编辑知识库", catalog: "知识库目录", catalogHint: "选择一个知识库，查看其中的分类和文档", private: "私有", public: "公开", empty: "还没有知识库", select: "选择一个知识库开始管理", open: "查看", backToCatalog: "返回知识库目录", noDescription: "暂无描述", description: "说明", visibility: "可见范围", displayMode: "展示方式", cardView: "卡片视图", listView: "列表视图", upload: "上传文件", categories: "文档分类", categoriesHint: "选择分类查看其中的文档", viewAll: "查看全部", allDocuments: "全部文档", documentCount: "份文档", categoryPlaceholder: "输入分类名称并回车", addCategory: "添加分类", unclassified: "未分类", document: "文档", state: "索引状态", source: "来源", updated: "更新时间", preview: "预览", previewUnsupported: "此文件格式暂不支持在线预览，请下载后查看。", previewFailed: "文档预览失败，请稍后重试。", noDocuments: "还没有文档", urlPlaceholder: "输入公开网页 URL", importURL: "导入网页", accepted: "来源已接收，正在后台处理", retry: "重试", retryAccepted: "已重新加入处理队列", retryFailed: "重试失败，请稍后再试。", uploadFailed: "上传未完成，请检查文件格式和大小。", importFailed: "网页导入失败，请确认 URL 可公开访问且格式受支持。", downloadFailed: "文档下载失败或当前无权访问。", deleteBase: "确认删除此知识库", deleteDocument: "确认删除此文档", deleteAccepted: "已删除", deleteFailed: "删除失败，请稍后再试。", loadFailed: "知识库加载失败，请重试。", saveFailed: "保存未完成，请重试。" } });
Object.assign(en, { knowledgeBases: { eyebrow: "Reusable knowledge", title: "Knowledge Bases", subtitle: "Organize source material and ground Workflows with traceable retrieval context", new: "New Knowledge Base", edit: "Edit Knowledge Base", catalog: "Knowledge Base catalog", catalogHint: "Choose a Knowledge Base to browse its categories and documents", private: "Private", public: "Public", empty: "No Knowledge Bases yet", select: "Select a Knowledge Base to manage it", open: "Open", backToCatalog: "Back to Knowledge Bases", noDescription: "No description", description: "Description", visibility: "Visibility", displayMode: "Display mode", cardView: "Card view", listView: "List view", upload: "Upload file", categories: "Document categories", categoriesHint: "Choose a category to browse its documents", viewAll: "View all", allDocuments: "All documents", documentCount: "documents", categoryPlaceholder: "Enter a Category name and press Enter", addCategory: "Add Category", unclassified: "Unclassified", document: "Document", state: "Index state", source: "Source", updated: "Updated", preview: "Preview", previewUnsupported: "This file type cannot be previewed online. Download it to view the file.", previewFailed: "Document preview failed. Try again shortly.", noDocuments: "No documents yet", urlPlaceholder: "Enter a public web URL", importURL: "Import URL", accepted: "Source accepted and processing in the background", retry: "Retry", retryAccepted: "Queued for processing again", retryFailed: "Retry failed. Try again later.", uploadFailed: "Upload failed. Check the file type and size.", importFailed: "URL import failed. Confirm it is publicly reachable and supported.", downloadFailed: "Document download failed or is no longer permitted.", deleteBase: "Confirm deleting this Knowledge Base", deleteDocument: "Confirm deleting this document", deleteAccepted: "Deleted", deleteFailed: "Delete failed. Try again later.", loadFailed: "Could not load Knowledge Bases. Try again.", saveFailed: "The operation did not complete. Try again." } });
Object.assign((zh as unknown as { knowledgeBases: Record<string, string> }).knowledgeBases, { regenerate: "重新生成", blocked: "已阻止" });
Object.assign((en as unknown as { knowledgeBases: Record<string, string> }).knowledgeBases, { regenerate: "Regenerate", blocked: "Blocked" });

Object.assign((zh as unknown as { knowledgeBases: Record<string, string> }).knowledgeBases, { search: "检索", searchPlaceholder: "输入要从文档中查找的信息", searchHint: "从当前已索引的文档中检索，最多返回 10 条相关内容。", searchResults: "检索结果", searchSource: "引用来源", searchEmpty: "没有找到相关内容。", searchNotReady: "文档尚未完成索引，请稍后重试。", searchFailed: "检索失败，请稍后重试。" });
Object.assign((en as unknown as { knowledgeBases: Record<string, string> }).knowledgeBases, { search: "Search", searchPlaceholder: "Enter information to find in documents", searchHint: "Search indexed documents in this Knowledge Base; up to 10 matching excerpts.", searchResults: "Search results", searchSource: "Source", searchEmpty: "No matching content found.", searchNotReady: "Documents are not indexed yet. Try again later.", searchFailed: "Search failed. Try again later." });
Object.assign((zh as unknown as { knowledgeBases: Record<string, string> }).knowledgeBases, { regenerate: "重新生成", blocked: "待处理" });
Object.assign((en as unknown as { knowledgeBases: Record<string, string> }).knowledgeBases, { regenerate: "Regenerate", blocked: "Blocked" });
Object.assign((zh as unknown as { aiApplications: Record<string, unknown> }).aiApplications, { basic: "基本信息", icon: "助手图标", welcome: "欢迎语", welcomePlaceholder: "用户第一次打开智能助手时显示的欢迎语", startFailed: "创建对话失败，请稍后重试。", validationFailed: "保存失败，请检查名称、图标和提示词配置。", versionConflict: "保存失败，数据已更新，请刷新后重试。", conflict: "操作失败，该智能助手仍被其他配置引用。", createdAt: "创建时间", actions: "操作", detail: "详情", faq: { empty: "还没有常见问题。", enable: "启用", disable: "停用" } });
Object.assign((en as unknown as { aiApplications: Record<string, unknown> }).aiApplications, { basic: "Basic information", icon: "Assistant icon", welcome: "Welcome message", welcomePlaceholder: "Shown when a user opens this Smart Assistant for the first time", startFailed: "Unable to create the conversation. Try again shortly.", validationFailed: "Save failed. Check the name, icon, and prompt configuration.", versionConflict: "Save failed because the data changed. Refresh and try again.", conflict: "The operation failed because this Smart Assistant is still referenced.", createdAt: "Created at", actions: "Actions", detail: "Details", faq: { empty: "No FAQs yet.", enable: "Enable", disable: "Disable" } });
Object.assign((zh as unknown as { aiApplications: Record<string, unknown> }).aiApplications, { search: "搜索智能助手", searchAction: "搜索", scenario: "场景类型", allScenarios: "全部场景", enable: "启用", disable: "停用", confirmDelete: "确认删除“{name}”？历史对话仍会保留快照。", states: { draft: "草稿", enabled: "已启用", disabled: "已停用" }, scenarios: { "customer-consultation": "客服咨询", "pre-sales-advisor": "售前顾问", "after-sales-support": "售后支持", "product-guide": "商品导购", "enterprise-knowledge": "企业知识助手", recruitment: "招聘助手", training: "培训助手", custom: "自定义" }, copy: "复制" });
Object.assign((en as unknown as { aiApplications: Record<string, unknown> }).aiApplications, { search: "Search Smart Assistants", searchAction: "Search", scenario: "Scenario", allScenarios: "All scenarios", enable: "Enable", disable: "Disable", confirmDelete: "Delete “{name}”? Historical conversations keep their snapshots.", states: { draft: "Draft", enabled: "Enabled", disabled: "Disabled" }, scenarios: { "customer-consultation": "Customer consultation", "pre-sales-advisor": "Pre-sales advisor", "after-sales-support": "After-sales support", "product-guide": "Product guide", "enterprise-knowledge": "Enterprise knowledge", recruitment: "Recruitment", training: "Training", custom: "Custom" }, copy: "Copy" });

// Keep nested AI application translations intact after the legacy compatibility merges above.
Object.assign((zh as unknown as { aiApplications: Record<string, unknown> }).aiApplications, {
  chat: { loadFailed: "加载对话失败，请重试。", sendFailed: "发送失败，请重试。", stopFailed: "打断失败，请重试。", newFailed: "新建对话失败，请重试。", modelUnavailable: "请为智能助手选择支持 openai_responses 的可用模型。", history: "历史对话", new: "清空并新建对话", defaultWelcome: "你好，有什么可以帮你？", thinking: "思考中...", cancelled: "已停止", failed: "回答失败，请重试。", failureAuthentication: "模型服务凭证已失效，请联系管理员更新 API Key 后重试。", failureRateLimited: "模型服务请求过于频繁，请稍后重试。", failureUnavailable: "模型服务暂时不可用，请稍后重试。", failureConfiguration: "模型服务配置不可用，请联系管理员检查模型设置。", failureInvalidResponse: "模型服务返回了无法识别的结果，请重试。", placeholder: "输入问题，Enter 发送", stop: "停止输出", send: "发送" },
  model: "对话模型",
  modelPlaceholder: "选择支持 openai_responses 的模型",
  modelUnavailable: "请选择已配置密钥、可用且支持 openai_responses 的模型。",
  description: "助手简介",
  prompt: "提示词",
  preprocessPrompt: "预处理提示词",
  startRequiresIcon: "请先上传助手图标，再开始对话。",
  startRequiresEnabled: "请先启用智能助手，再开始对话。",
  shareAction: "分享",
  iconInvalid: "请选择 2MB 以内的 PNG、JPG、WebP 或 GIF 图片。",
  iconUploadFailed: "助手图标上传失败，请重试。",
  iconLoadFailed: "助手图标加载失败，请重新上传。",
  validationFailed: "保存失败，请检查名称、图标和提示词配置。",
  faq: { title: "常见问题", question: "问题", answer: "答案", add: "添加问题", edit: "编辑问题", import: "导入", export: "导出", importFailed: "导入失败，请确认文件包含“问题”和“答案”两列。", empty: "还没有常见问题。", enable: "启用", disable: "停用" },
});
Object.assign((en as unknown as { aiApplications: Record<string, unknown> }).aiApplications, {
  chat: { loadFailed: "Could not load the conversation.", sendFailed: "Could not send the question.", stopFailed: "Could not stop the response.", newFailed: "Could not start a new conversation.", modelUnavailable: "Choose an available openai_responses model for this Smart Assistant.", history: "Conversation history", new: "Clear and start again", defaultWelcome: "Hello, how can I help?", thinking: "Thinking...", cancelled: "Stopped", failed: "Response failed. Try again.", failureAuthentication: "The model service credential has expired. Ask an administrator to update the API key, then try again.", failureRateLimited: "The model service is receiving too many requests. Try again later.", failureUnavailable: "The model service is temporarily unavailable. Try again later.", failureConfiguration: "The model service configuration is unavailable. Ask an administrator to check the model settings.", failureInvalidResponse: "The model service returned an unrecognized result. Try again.", placeholder: "Ask a question, Enter to send", stop: "Stop", send: "Send" },
  model: "Conversation model",
  modelPlaceholder: "Select an openai_responses model",
  modelUnavailable: "Choose an available model with an API key and openai_responses support.",
  description: "Assistant description",
  prompt: "Assistant prompt",
  preprocessPrompt: "Pre-process prompt",
  startRequiresIcon: "Upload an assistant icon before starting the conversation.",
  startRequiresEnabled: "Enable the Smart Assistant before starting a conversation.",
  shareAction: "Share",
  iconInvalid: "Choose a PNG, JPG, WebP, or GIF image no larger than 2MB.",
  iconUploadFailed: "Unable to upload the assistant icon. Try again.",
  iconLoadFailed: "Unable to load the assistant icon. Upload it again.",
  validationFailed: "Save failed. Check the name, icon, and prompt configuration.",
  faq: { title: "Frequently Asked Questions", question: "Question", answer: "Answer", add: "Add FAQ", edit: "Edit FAQ", import: "Import", export: "Export", importFailed: "Import failed. The file must contain Question and Answer columns.", empty: "No FAQs yet.", enable: "Enable", disable: "Disable" },
});
Object.assign((zh as unknown as { aiApplications: { assistants: Record<string, unknown> } }).aiApplications.assistants, { defaultIntro: "已创建，可继续配置简介、提示词和知识库。" });
Object.assign((en as unknown as { aiApplications: { assistants: Record<string, unknown> } }).aiApplications.assistants, { defaultIntro: "Created. Continue configuring the description, prompts, and knowledge bases." });

Object.assign((zh as unknown as { approvals: Record<string, unknown> }).approvals, {
  createTask: "创建任务",
  taskCreateDescription: "助手准备通过{connector}创建任务。请核对负责人；选择“仅批准本次”后才会创建，拒绝则不会创建。",
  approvedNotice: "已批准本次操作，助手正在继续执行。请在会话中查看最终结果。",
  rejectedNotice: "已拒绝本次操作，助手正在继续执行。",
  staleNotice: "这项请求已处理或过期，已刷新审批状态。请查看会话中的最新结果。",
  failedNotice: "提交失败，请检查网络后重试；本次操作尚未获得批准。",
});
Object.assign((en as unknown as { approvals: Record<string, unknown> }).approvals, {
  createTask: "Create task",
  taskCreateDescription: "The assistant is about to create a task through {connector}. Check the assignee; approving once creates it, and rejecting prevents it.",
  approvedNotice: "Approved this action. The assistant is continuing; check the conversation for the final result.",
  rejectedNotice: "Rejected this action. The assistant is continuing.",
  staleNotice: "This request was already handled or expired. The approval status has been refreshed; check the conversation.",
  failedNotice: "Submission failed. Check your connection and retry; this action has not been approved.",
});

export function resolveInitialLocale(stored: string | null, browserLanguage: string): SupportedLocale {
  if (stored === "zh-CN" || stored === "en-US") return stored;
  return browserLanguage.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}

Object.assign((zh as unknown as { composer: Record<string, unknown> }).composer, { planFirst: "先制定计划", planFirstHint: "额外调用一次模型并按计费规则扣除 Credits；先展示具体计划，确认后再执行" });
Object.assign((en as unknown as { composer: Record<string, unknown> }).composer, { planFirst: "Plan first", planFirstHint: "Uses one extra metered model call; review the detailed plan before execution" });
Object.assign(zh.sessions, { executionPlan: {
  title: "执行计划", resources: "将使用", sideEffectsTitle: "可能产生的变更", estimate: "预计消耗", generationCost: "制定计划", start: "按计划开始", edit: "修改要求", direct: "直接回答，不执行外部操作", automaticStart: "自动开始",
  estimateValue: "{calls} 次模型调用 · 最多约 {credits} Credits", generationCostValue: "平台规则生成 · {credits} Credits", modelGenerationCostValue: "模型生成 · {credits} Credits", modelFailedHint: "详细计划生成失败，当前显示规则计划；若模型已响应，仍按实际用量计费。可修改要求后重试。",
  states: { pending: "等待确认", approved: "已确认", executing: "执行中", completed: "已完成", failed: "未完成", cancelled: "已取消", skipped: "已跳过" },
  stepStates: { pending: "待执行", running: "进行中", completed: "已完成", skipped: "已跳过", failed: "失败" },
  reasons: { user_requested: "你要求先制定计划", multiple_stages: "包含多个执行阶段", multiple_external_sources: "将读取多个外部资源", external_side_effect: "可能修改外部系统", workflow_execution: "这是工作流执行", workspace_change: "可能修改工作区文件" },
  sideEffects: { external_connector_operation: "可能调用连接器修改外部系统", workspace_files_may_change: "可能修改工作区文件" },
} });
Object.assign(zh.workflows, { editPlanTitle: "修改要求", editPlanHint: "先编辑新要求；提交后才会取消当前计划并创建新任务。", editPlanRetryHint: "当前计划已取消，但新任务未创建。要求仍保留在这里，请重试提交。", backToPlan: "返回计划" });
Object.assign(en.sessions, { executionPlan: {
  title: "Execution plan", resources: "Resources", sideEffectsTitle: "Possible changes", estimate: "Estimate", generationCost: "Plan generation", start: "Start plan", edit: "Edit requirements", direct: "Answer directly without external actions", automaticStart: "Starting automatically",
  estimateValue: "{calls} model calls · up to about {credits} Credits", generationCostValue: "Platform rules · {credits} Credits", modelGenerationCostValue: "Model-generated · {credits} Credits", modelFailedHint: "Detailed plan generation failed. A rule-based safety plan is shown instead; edit the request to retry.",
  states: { pending: "Awaiting confirmation", approved: "Approved", executing: "Running", completed: "Completed", failed: "Incomplete", cancelled: "Cancelled", skipped: "Skipped" },
  stepStates: { pending: "Pending", running: "Running", completed: "Completed", skipped: "Skipped", failed: "Failed" },
  reasons: { user_requested: "You requested a plan first", multiple_stages: "Multiple execution stages", multiple_external_sources: "Uses multiple external resources", external_side_effect: "May change an external system", workflow_execution: "Workflow execution", workspace_change: "May change workspace files" },
  sideEffects: { external_connector_operation: "May use a Connector to change an external system", workspace_files_may_change: "May change workspace files" },
} });
Object.assign(en.workflows, { editPlanTitle: "Edit requirements", editPlanHint: "Edit the request first. Submitting cancels the current plan and creates a new task.", editPlanRetryHint: "The plan was cancelled, but the new task was not created. Your request is still here; retry sending.", backToPlan: "Back to plan" });
Object.assign(zh, { taskWorkspace: {
  completedWithFailures: "已完成，有调用失败", eyebrow: "当前任务", title: "任务面板", close: "关闭任务面板", open: "查看任务详情", reopen: "打开任务面板", plan: "计划", evidence: "证据", activities: "执行活动", modelCalls: "模型阶段", files: "文件", inputFile: "输入文件", outputFile: "输出产物", result: "结果摘要", state: "状态", elapsed: "耗时", credits: "消耗积分",
  stepKinds: { review_input: "核对输入", retrieve_knowledge: "检索知识", execute_stage: "执行任务", deliver_result: "交付结果" },
} });
Object.assign(en, { taskWorkspace: {
  completedWithFailures: "Completed with failed calls", eyebrow: "Current task", title: "Task panel", close: "Close task panel", open: "View task details", reopen: "Open task panel", plan: "Plan", evidence: "Evidence", activities: "Execution activities", modelCalls: "Model stages", files: "Files", inputFile: "Input file", outputFile: "Output artifact", result: "Result summary", state: "State", elapsed: "Elapsed", credits: "Credits used",
  stepKinds: { review_input: "Review input", retrieve_knowledge: "Retrieve knowledge", execute_stage: "Execute task", deliver_result: "Deliver result" },
} });
Object.assign(zh.sessions, { workflowSave: {
  action: "保存为工作流", open: "打开已保存工作流", eyebrow: "从成功结果创建", title: "保存为可重复运行的工作流", description: "名称、目标和本次实际使用的专家、技能与连接器会被带入。会话与工作流后续各自保留独立历史。", carriedConfiguration: "带入的执行配置", noExtraResources: "本次未使用额外技能或连接器。", filesTitle: "处理本次文件", filesDescription: "每个输入文件和输出产物都必须明确选择去向；不会静默复制。", unavailable: "不可用，只能排除", toWorkspace: "复制到工作区", exclude: "不带入", validationNotice: "创建后会立即排队执行首个验证运行，无需确认规则计划；连接器高风险操作仍需单独批准。工作流、来源关系和验证运行会一起成功或一起失败。", confirm: "创建并验证", loadFailed: "无法读取这个成功结果，请刷新后重试。", createFailed: "工作流未创建；请检查名称、目标和文件后重试。",
} });
Object.assign(en.sessions, { workflowSave: {
  action: "Save as Workflow", open: "Open saved Workflow", eyebrow: "Create from a successful result", title: "Save as a repeatable Workflow", description: "The name, goal, and the exact Experts, Skills, and Connectors used in this response are carried over. The Session and Workflow keep independent histories afterward.", carriedConfiguration: "Execution configuration", noExtraResources: "No additional Skills or Connectors were used.", filesTitle: "Handle files from this result", filesDescription: "Choose an explicit destination for every input file and artifact. Nothing is copied silently.", unavailable: "Unavailable; must be excluded", toWorkspace: "Copy to Workspace", exclude: "Do not carry over", validationNotice: "Creation immediately queues the first validation Run without requiring rule-Plan confirmation. High-risk Connector commands still require separate approval. The Workflow, source link, and validation Run either all succeed or all fail.", confirm: "Create and validate", loadFailed: "This successful result could not be loaded. Refresh and retry.", createFailed: "No Workflow was created. Check the name, goal, and files, then retry.",
} });
Object.assign(zh.workflows, { session_conversion: "由会话创建", fromSession: "来自会话" });
Object.assign(en.workflows, { session_conversion: "Created from Session", fromSession: "From Session" });
Object.assign(zh.workflows, { createHint: "先用名称和目标跑通一次，再配置专家、知识库、定时、API 或 Git。", createAndValidate: "创建并验证", validationRunFailed: "工作流已创建，但验证运行未能启动；请检查个人执行设置后重试。", validatedTitle: "这个工作流已经跑通", validatedHint: "现在再按使用方式配置定时触发、API 接入或 Git 来源。", configureNext: "继续配置" });
Object.assign(en.workflows, { createHint: "Start with a name and goal. Configure Experts, Knowledge Bases, schedules, API, or Git after the first run works.", createAndValidate: "Create and validate", validationRunFailed: "The Workflow was created, but its validation Run could not start. Check Personal Settings and retry.", validatedTitle: "This Workflow has completed a run", validatedHint: "Now configure a schedule, API integration, or Git source when the use case needs it.", configureNext: "Configure next" });
Object.assign(zh.workflows, {
  searchPlaceholder: "搜索名称或目标", filterLabel: "筛选工作流", filterAll: "全部", filterAttention: "需要处理", filterScheduled: "已计划", filterNeverRun: "尚未运行",
  sortLabel: "工作流排序", sortAttention: "需要处理优先", sortUpdated: "最近更新", sortFrequent: "运行最多", noMatching: "没有匹配的工作流",
  success30d: "30 天成功率", runs30d: "30 天运行", lastRun: "最近运行", nextRun: "下次计划", noSchedule: "未设置", notRun: "尚未运行",
  continueAction: "继续处理", recoverAction: "查看并恢复", viewRun: "查看运行", validateAction: "开始验证",
  runState: { queued: "排队中", running: "运行中", waiting_for_user: "等待处理", succeeded: "运行成功", failed: "运行失败", cancelled: "已取消" },
  deleteImpact: "删除“{name}”后：{schedule}；{api}；工作空间会被清空，历史运行记录保留为只读。",
  deleteScheduleActive: "正在运行的定时计划会停止", deleteScheduleInactive: "当前没有启用的定时计划", deleteApiActive: "现有 API 凭证会立即失效", deleteApiInactive: "当前没有 API 凭证",
});
Object.assign(en.workflows, {
  searchPlaceholder: "Search by name or goal", filterLabel: "Filter Workflows", filterAll: "All", filterAttention: "Needs attention", filterScheduled: "Scheduled", filterNeverRun: "Never run",
  sortLabel: "Sort Workflows", sortAttention: "Attention first", sortUpdated: "Recently updated", sortFrequent: "Most run", noMatching: "No matching Workflows",
  success30d: "30-day success", runs30d: "30-day runs", lastRun: "Last run", nextRun: "Next schedule", noSchedule: "Not scheduled", notRun: "Never run",
  continueAction: "Continue", recoverAction: "View and recover", viewRun: "View run", validateAction: "Start validation",
  runState: { queued: "Queued", running: "Running", waiting_for_user: "Needs input", succeeded: "Succeeded", failed: "Failed", cancelled: "Cancelled" },
  deleteImpact: "Deleting “{name}”: {schedule}; {api}; the Workspace is cleared and Run history remains read-only.",
  deleteScheduleActive: "the active schedule stops", deleteScheduleInactive: "no schedule is active", deleteApiActive: "the API credential is revoked immediately", deleteApiInactive: "no API credential is configured",
});
Object.assign(zh.workflows, {
  overview: "概览", configureSchedule: "设置定时运行", configureApi: "接入业务系统", configureGit: "连接代码仓库", validateHint: "完成首次验证后才能自动运行",
  runSuccessCount: "成功 {success}/{total} 次", configureScheduleHint: "验证成功后再设置自动运行", recoveryRequired: "最近一次运行失败", actionRequired: "有操作等待处理",
  planWaitingHint: "运行计划或授权仍在等待你的确认。", runFailedHint: "打开运行记录查看原因并选择恢复方式。", recentRuns: "最近运行", viewAllRuns: "查看全部", noRunsOverview: "先运行一次，确认目标和执行计划是否正确。",
  resourceSettings: "执行资源", resourceSummary: "{knowledge} 个知识库 · {environment} 个环境变量", configured: "已配置", notConfigured: "未配置", scheduleEnabledSummary: "{frequency} · {timezone}",
  previewSchedule: "预览未来三次", schedulePreviewHint: "预览会按当前填写的时区计算，不会保存配置。", schedulePreviewFailed: "无法计算计划时间，请检查频率、时间和时区。",
  sunday: "周日", monday: "周一", tuesday: "周二", wednesday: "周三", thursday: "周四", friday: "周五", saturday: "周六",
  rotateCredentialTitle: "重新生成 API 凭证？", rotateCredentialImpact: "旧的 API Key 和 API Secret 会立即失效。请先确认调用方可以同步替换凭证。",
  revokeCredential: "停用凭证", revokeCredentialTitle: "停用 API 凭证？", revokeCredentialImpact: "所有使用当前凭证的调用会立即失败。工作流本身和历史运行不会被删除。", credentialRevoked: "API 凭证已停用。",
  artifactsEmpty: "还没有结果文件", artifactsEmptyHint: "运行明确要求生成文件的任务后，结果会出现在这里。", workspaceEmpty: "工作空间为空", workspaceEmptyHint: "连接 Git 来源或让运行写入持久文件后，内容会出现在这里。", rerun: "重新运行",
});
Object.assign(en.workflows, {
  overview: "Overview", configureSchedule: "Set schedule", configureApi: "Connect a system", configureGit: "Connect repository", validateHint: "Complete validation before automating this Workflow",
  runSuccessCount: "{success}/{total} succeeded", configureScheduleHint: "Validate once before scheduling", recoveryRequired: "The latest run failed", actionRequired: "Action required",
  planWaitingHint: "A plan or authorization still needs your decision.", runFailedHint: "Open the Run to see the cause and recovery action.", recentRuns: "Recent runs", viewAllRuns: "View all", noRunsOverview: "Run once to validate the goal and execution plan.",
  resourceSettings: "Execution resources", resourceSummary: "{knowledge} Knowledge Bases · {environment} environment variables", configured: "Configured", notConfigured: "Not configured", scheduleEnabledSummary: "{frequency} · {timezone}",
  previewSchedule: "Preview next three", schedulePreviewHint: "The preview uses the entered timezone and does not save the configuration.", schedulePreviewFailed: "Could not calculate schedule times. Check the frequency, time, and timezone.",
  sunday: "Sunday", monday: "Monday", tuesday: "Tuesday", wednesday: "Wednesday", thursday: "Thursday", friday: "Friday", saturday: "Saturday",
  rotateCredentialTitle: "Regenerate API credential?", rotateCredentialImpact: "The existing API Key and API Secret stop working immediately. Confirm that every caller can replace the credential.",
  revokeCredential: "Revoke credential", revokeCredentialTitle: "Revoke API credential?", revokeCredentialImpact: "Every caller using this credential fails immediately. The Workflow and Run history are not deleted.", credentialRevoked: "The API credential was revoked.",
  artifactsEmpty: "No result files yet", artifactsEmptyHint: "Files explicitly produced by a Run appear here.", workspaceEmpty: "The Workspace is empty", workspaceEmptyHint: "Connect a Git source or let a Run write persistent files to populate it.", rerun: "Rerun",
});
Object.assign(zh.settings, { usePlatformDefault: "继承企业默认执行组合", platformDefaultHint: "管理员已验证 Runtime 与模型；无需单独配置即可开始。", platformDefaultUnavailable: "企业默认尚未配置。取消继承后可使用个人执行设置。", platformDefault: "企业默认执行组合", platformDefaultAdminHint: "必须绑定一个已成功且执行快照完全匹配的验证 Run；该证据会把 unverified 组合晋升为 verified，不兼容组合不可选。", validationRun: "验证 Run", validationRunHint: "填写由当前 Runtime 与模型完成的成功 Run ID。", setPlatformDefault: "设为企业默认", platformDefaultSaved: "企业默认已更新，所有仍在继承的账号已同步。" });
Object.assign(en.settings, { usePlatformDefault: "Inherit the enterprise execution default", platformDefaultHint: "An Administrator verified this Runtime and model; no personal execution setup is required.", platformDefaultUnavailable: "No enterprise default is available. Turn inheritance off to configure a personal execution profile.", platformDefault: "Enterprise execution default", platformDefaultAdminHint: "A successful validation Run with an exactly matching execution snapshot is required. That evidence promotes an unverified pair to verified; incompatible pairs cannot be selected.", validationRun: "Validation Run", validationRunHint: "Enter the ID of a successful Run completed by this Runtime and model.", setPlatformDefault: "Set enterprise default", platformDefaultSaved: "The enterprise default was updated and propagated to every account still inheriting it." });
Object.assign((zh as unknown as { credits: Record<string, unknown> }).credits, { available: "可用额度", reserved: "执行中预留", warning: "今日额度使用已达到 {percent}% 提醒线", exhausted: "今日额度已用尽", contactAdministrator: "需要更多额度时请联系企业管理员；新的执行会在可用额度不足时被阻止。", stageSummary: "按执行阶段查看", stage: "阶段 {position}", measuredCharge: "按实际用量结算", estimatedCharge: "缺少用量时按冻结估算结算", consumedBeforeStop: "停止前已产生的消耗", notCharged: "本次未记录模型消耗；重试会作为新的执行单独计费。" });
Object.assign((en as unknown as { credits: Record<string, unknown> }).credits, { available: "Available Credits", reserved: "Reserved for active work", warning: "Today's usage reached the {percent}% warning threshold", exhausted: "Today's Credits are exhausted", contactAdministrator: "Contact your Administrator if you need more Credits. New executions are blocked when Available Credits are insufficient.", stageSummary: "View execution stages", stage: "Stage {position}", measuredCharge: "Settled from measured usage", estimatedCharge: "Settled from the frozen fallback when usage was unavailable", consumedBeforeStop: "Usage incurred before the execution stopped", notCharged: "No model usage was recorded for this execution. A retry is charged as a separate execution." });
Object.assign(zh.users, { creditPolicy: "企业额度策略", creditPolicyHint: "默认额度只作用于新建额度账户；单用户额度保持独立。兑换码默认关闭。", defaultDailyLimit: "新账号每日默认额度", warningThreshold: "用量提醒阈值", enableCodes: "启用兑换码渠道" });
Object.assign(en.users, { creditPolicy: "Enterprise Credit policy", creditPolicyHint: "The default applies only to new Credit accounts. Per-user allocations remain independent. Redemption Codes are off by default.", defaultDailyLimit: "Default daily Credits for new accounts", warningThreshold: "Usage warning threshold", enableCodes: "Enable Redemption Code channel" });
Object.assign(zh, { home: {
  title: "回到要处理的事", subtitle: "只汇总任务元数据；消息正文、结果、参数和错误详情仍留在原任务中。", refresh: "刷新", loadFailed: "首页暂时无法加载", empty: "还没有任务或工作流", actions: "待你处理", actionsHint: "审批、计划确认和失败恢复都回到原任务处理。", noActions: "没有待处理事项", recent: "最近任务", recentHint: "继续最近更新的会话或工作流运行。", noRecent: "还没有最近任务", commonWorkflows: "常用工作流", commonWorkflowsHint: "按实际运行次数和最近使用时间排序。", noWorkflows: "还没有可用工作流", runCount: "运行 {count} 次", actionKind: { approval: "等待审批", plan: "等待确认计划", failed: "需要恢复" }, taskKind: { session: "会话", run: "工作流运行" }, taskState: { idle: "暂无执行", queued: "排队中", running: "运行中", waiting_for_user: "等待你的操作", succeeded: "已成功", completed: "已完成", failed: "失败", cancelled: "已取消" },
} });
Object.assign(en, { home: {
  title: "Pick up what needs attention", subtitle: "Only task metadata is aggregated. Messages, results, arguments, and error details stay in the original task.", refresh: "Refresh", loadFailed: "Home is temporarily unavailable", empty: "No tasks or Workflows yet", actions: "Needs your attention", actionsHint: "Open the original task to approve, confirm a plan, or recover a failure.", noActions: "Nothing needs your attention", recent: "Recent tasks", recentHint: "Continue a recently updated Session or Workflow Run.", noRecent: "No recent tasks", commonWorkflows: "Common Workflows", commonWorkflowsHint: "Ranked by actual Run count and recent use.", noWorkflows: "No Workflows yet", runCount: "{count} Runs", actionKind: { approval: "Approval required", plan: "Plan confirmation required", failed: "Recovery required" }, taskKind: { session: "Session", run: "Workflow Run" }, taskState: { idle: "No active execution", queued: "Queued", running: "Running", waiting_for_user: "Waiting for you", succeeded: "Succeeded", completed: "Completed", failed: "Failed", cancelled: "Cancelled" },
} });
Object.assign(zh.nav, { resources: "资源库" });
Object.assign(en.nav, { resources: "Resource Library" });
Object.assign((zh as unknown as { resources: Record<string, unknown> }).resources, {
  taskSearch: "按任务搜索当前类型的名称、用途或能力",
  availableOnly: "可直接使用",
  allStates: "全部状态",
  availableOnlyHint: "默认不推荐未测试、失败、停用或不兼容的资源；切换到全部状态后可处理这些资源。",
  allStatesHint: "正在显示草稿、验证中、失败、停用和不兼容资源；这些状态不代表可以执行。",
  platformPublished: "平台发布",
  userPublished: "个人创建",
  allAuthenticated: "企业内可用",
  ownerOnly: "仅我可见",
  allCanInstall: "企业内可安装",
  personalInstallation: "我的安装",
  allCanEnable: "企业内可启用",
  verifiedAvailable: "已验证可用",
  available: "可用",
  unavailable: "不可用",
  unverified: "未验证",
  verificationPending: "验证中",
  runtimeVerified: "运行环境已验证",
  connectionTested: "连接测试通过",
  packageValidated: "安装包校验通过",
  isolatedRuntime: "仅在隔离 Runtime 内执行",
  runtimeDigestCount: "{count} 个 Runtime 镜像证据",
  noRuntimeEvidence: "没有当前 Runtime 镜像证据",
  noMatchingResources: "没有匹配当前筛选条件的资源",
  platformKnowledge: "平台知识库",
  myKnowledge: "我的知识库",
  allReadOwnerEdit: "企业内可读 · 我可编辑",
  allReadOnly: "企业内只读",
  accessible: "当前有权访问",
  retrievalReady: "可检索",
  indexNotReady: "索引未就绪",
  emptyKnowledge: "空知识库",
  knowledgeDocumentEvidence: "{ready}/{total} 份文档可检索",
  lastIndexed: "最近就绪 {date}",
});
Object.assign((en as unknown as { resources: Record<string, unknown> }).resources, {
  taskSearch: "Search this resource type by task, purpose, or capability",
  availableOnly: "Ready to use",
  allStates: "All states",
  availableOnlyHint: "Untested, failed, disabled, or incompatible resources are not recommended by default. Switch to All states to manage them.",
  allStatesHint: "Draft, testing, failed, disabled, and incompatible resources are visible. These states do not mean the resource can execute.",
  platformPublished: "Platform published",
  userPublished: "Personally created",
  allAuthenticated: "Available across the enterprise",
  ownerOnly: "Visible only to me",
  allCanInstall: "Installable across the enterprise",
  personalInstallation: "My installation",
  allCanEnable: "Enableable across the enterprise",
  verifiedAvailable: "Verified and available",
  available: "Available",
  unavailable: "Unavailable",
  unverified: "Unverified",
  verificationPending: "Verification pending",
  runtimeVerified: "Runtime verified",
  connectionTested: "Connection test passed",
  packageValidated: "Package validation passed",
  isolatedRuntime: "Runs only in an isolated Runtime",
  runtimeDigestCount: "Evidence for {count} Runtime images",
  noRuntimeEvidence: "No current Runtime image evidence",
  noMatchingResources: "No resources match the current filters",
  platformKnowledge: "Platform Knowledge Bases",
  myKnowledge: "My Knowledge Bases",
  allReadOwnerEdit: "Enterprise-readable · editable by me",
  allReadOnly: "Enterprise read-only",
  accessible: "Accessible with current permissions",
  retrievalReady: "Retrieval ready",
  indexNotReady: "Index not ready",
  emptyKnowledge: "Empty Knowledge Base",
  knowledgeDocumentEvidence: "{ready}/{total} documents retrieval-ready",
  lastIndexed: "Last ready {date}",
});

Object.assign(zh.workflows, { readyKnowledgeDocuments: "{count} 份文档可检索", knowledgeBaseNotReady: "暂无可检索文档" });
Object.assign(en.workflows, { readyKnowledgeDocuments: "{count} retrieval-ready documents", knowledgeBaseNotReady: "No retrieval-ready documents" });

Object.assign(zh.common, { confirm: "确认" });
Object.assign(en.common, { confirm: "Confirm" });
Object.assign((zh as unknown as { credits: Record<string, string> }).credits, { groupBudget: "部门预算：{name}", groupBudgetRemaining: "今日部门剩余 ✧ {remaining} / 上限 ✧ {limit}；可用积分取个人与部门剩余中的较小值。" });
Object.assign((en as unknown as { credits: Record<string, string> }).credits, { groupBudget: "Department budget: {name}", groupBudgetRemaining: "Department remaining today: ✧ {remaining} of ✧ {limit}. Available Credits use the lower of the personal and Department balance." });
Object.assign(zh.users, {
  tabs: { users: "用户", groups: "部门与群组", audit: "治理审计", rates: "模型倍率", codes: "兑换码" },
  result: "操作结果", roles: "角色", bootstrapAdministrator: "初始管理员", bootstrapLocked: "初始管理员不可降级", resourcePublisher: "资源发布者", departments: "所属部门",
  reason: "变更原因", disableUser: "停用账号", enableUser: "启用账号", syncGroups: "从身份源同步", groupSyncFailed: "身份群组同步失败；本地权限未变更。",
  identityReadOnly: "群组与成员关系来自身份源，只能同步；平台不会反向修改企业目录。", group: "群组", groupPath: "身份源路径", groupType: "用途", department: "部门", identityGroup: "普通群组", memberCount: "成员数",
  groupDailyBudget: "部门每日预算", unlimited: "不限制", budget: "预算", limitBudget: "设置上限", lastSynced: "最近同步", transfer: "移交部门资源", transferFrom: "离职/停用成员", transferTo: "接收发布者",
  transferBoundary: "只移交该部门的知识库归属；不会读取、转移或暴露个人会话、文件和私有知识库。", transferCompleted: "已移交 {count} 个部门知识库", groupBudgetBound: "受 {name} 部门预算约束",
  auditPrivacy: "审计记录只保留治理动作、目标标识、原因和计数，不记录私有内容。", occurredAt: "发生时间", action: "动作", target: "目标", metrics: "计数"
});
Object.assign(en.users, {
  tabs: { users: "Users", groups: "Departments & groups", audit: "Governance audit", rates: "Model rates", codes: "Redemption codes" },
  result: "Result", roles: "Roles", bootstrapAdministrator: "Bootstrap administrator", bootstrapLocked: "The Bootstrap Administrator cannot be demoted", resourcePublisher: "Resource Publisher", departments: "Departments",
  reason: "Reason", disableUser: "Disable account", enableUser: "Enable account", syncGroups: "Sync identity source", groupSyncFailed: "Identity group sync failed. Local permissions were not changed.",
  identityReadOnly: "Groups and memberships are read-only from the identity source. The platform never writes back to the enterprise directory.", group: "Group", groupPath: "Identity path", groupType: "Purpose", department: "Department", identityGroup: "Identity group", memberCount: "Members",
  groupDailyBudget: "Department daily budget", unlimited: "Unlimited", budget: "Budget", limitBudget: "Set limit", lastSynced: "Last synced", transfer: "Transfer department resources", transferFrom: "Disabled member", transferTo: "Receiving publisher",
  transferBoundary: "Only ownership of this Department's Knowledge Bases is transferred. Private Sessions, files, and private Knowledge Bases are never read, exposed, or transferred.", transferCompleted: "Transferred {count} Department Knowledge Bases", groupBudgetBound: "Limited by {name} Department budget",
  auditPrivacy: "Audit records contain only governance actions, target identifiers, reasons, and counts—not private content.", occurredAt: "Occurred", action: "Action", target: "Target", metrics: "Metrics"
});
Object.assign((zh as unknown as { knowledgeBases: Record<string, unknown> }).knowledgeBases, {
  scope: "归属范围", privateScope: "仅自己", departmentScope: "部门", platformScope: "全企业", department: "部门", departmentKnowledge: "部门知识库",
  scopeImmutable: "创建后归属范围不可修改，避免绕过权限与审计边界。", scopeHint: { private: "只有你可以查看和维护。", group: "部门成员可读；该部门的资源发布者可维护。", platform: "全企业可读；仅创建它的管理员可维护。" },
  departmentPublished: "{name} 发布", departmentReadPublisherEdit: "部门内可读 · 发布者可编辑", departmentReadOnly: "部门内只读"
});
Object.assign((en as unknown as { knowledgeBases: Record<string, unknown> }).knowledgeBases, {
  scope: "Ownership scope", privateScope: "Only me", departmentScope: "Department", platformScope: "Enterprise", department: "Department", departmentKnowledge: "Department Knowledge Bases",
  scopeImmutable: "Ownership scope cannot change after creation, preserving permission and audit boundaries.", scopeHint: { private: "Only you can view and maintain it.", group: "Department members can read it; Resource Publishers in the Department can maintain it.", platform: "Everyone can read it; only the Administrator who created it can maintain it." },
  departmentPublished: "Published by {name}", departmentReadPublisherEdit: "Department-readable · Publisher-editable", departmentReadOnly: "Department read-only"
});

const zhAIApplications = (zh as unknown as { aiApplications: { share: Record<string, unknown> } & Record<string, unknown> }).aiApplications;
const enAIApplications = (en as unknown as { aiApplications: { share: Record<string, unknown> } & Record<string, unknown> }).aiApplications;
Object.assign(zhAIApplications.share, {
  allowedOriginsPlaceholder: "每行一个明确的 HTTPS 来源，例如 https://support.example.com",
  dailyLimit: "每日自由提问上限",
  controlsRequired: "开启分享前必须填写允许来源、正数调用上限，并确认数据处理与所有者付费影响。",
  impactTitle: "匿名访客会消耗助手所有者的额度",
  impactDescription: "访客问题会经过安全检查并可能发送给模型与知识检索服务；管理端只展示调用、错误和额度聚合值，不展示访客问题正文。",
  acknowledge: "我已确认匿名访问、数据处理和所有者付费影响",
  visitorPreview: "以访客身份预览",
  previewNotice: "这是不调用模型、不创建对话、不计入用量的外观预览。",
  tokenOnce: "这是新 Token 的唯一展示机会；复制后妥善保存。旧 Token 已立即失效。",
  aggregateStats: "最近 {days} 天聚合数据",
  statConversations: "匿名会话 {count}", statCalls: "自由提问 {count}", statErrors: "失败或取消 {count}", statCredits: "消耗积分 {count}"
});
Object.assign(enAIApplications.share, {
  allowedOrigins: "Allowed embed origins", allowedOriginsPlaceholder: "One explicit HTTPS origin per line, e.g. https://support.example.com",
  dailyLimit: "Daily free-text limit",
  controlsRequired: "Sharing requires an allowed origin, a positive daily cap, and acknowledgement of data processing and owner-paid usage.",
  impactTitle: "Anonymous visitors consume the Assistant owner's Credits",
  impactDescription: "Visitor questions pass safety checks and may be sent to model and knowledge retrieval services. Management sees only aggregate calls, errors, and Credits—not visitor question content.",
  acknowledge: "I acknowledge anonymous access, data processing, and owner-paid usage",
  visitorPreview: "Preview as visitor", previewNotice: "This appearance preview makes no model call, creates no conversation, and does not affect usage.",
  tokenOnce: "This is the only display of the new Token. Copy it now; the old Token is already invalid.",
  aggregateStats: "Aggregate data for the last {days} days",
  statConversations: "Anonymous conversations {count}", statCalls: "Free-text calls {count}", statErrors: "Failed or cancelled {count}", statCredits: "Credits consumed {count}"
});
Object.assign(zhAIApplications, { publication: {
  title: "发布检查", runCheck: "运行发布检查", ready: "可以发布", blocked: "暂不可发布", passed: "通过", failed: "阻断", checkFailed: "发布检查失败，请稍后重试。",
  controlledVisitors: "受控访客", authenticatedOwner: "已认证所有者", boundKnowledge: "绑定 {count} 个知识库", validatedAt: "最近验证 {time}", notValidated: "尚无当前版本的发布验证",
  checks: { configuration_complete: "名称、图标和可见配置完整", model_available: "所选模型与凭证可用", faq_safe: "已启用 FAQ 通过安全检查", knowledge_ready: "绑定知识库均有 Ready 文档", resources_available: "引用的 Expert、Team 或数字人可用", credit_policy_ready: "所有者有可用额度策略", share_controls: "分享来源、每日上限和数据确认完整" }
} });
Object.assign(enAIApplications, { publication: {
  title: "Publication check", runCheck: "Run publication check", ready: "Ready to publish", blocked: "Publication blocked", passed: "Passed", failed: "Blocked", checkFailed: "The publication check failed. Try again.",
  controlledVisitors: "Controlled visitors", authenticatedOwner: "Authenticated owner", boundKnowledge: "{count} Knowledge Bases", validatedAt: "Last validated {time}", notValidated: "No validation for the current revision",
  checks: { configuration_complete: "Name, icon, and visible configuration are complete", model_available: "Selected model and credential are available", faq_safe: "Enabled FAQs passed safety checks", knowledge_ready: "Every bound Knowledge Base has Ready content", resources_available: "Referenced Expert or Team is available", credit_policy_ready: "Owner has a usable Credit policy", share_controls: "Origins, daily cap, and data acknowledgement are explicit" }
} });

export function createAppI18n(storage: Pick<Storage, "getItem"> = localStorage, browserLanguage = navigator.language) {
  const locale = resolveInitialLocale(storage.getItem(localeStorageKey), browserLanguage);
  document.documentElement.lang = locale;
  return createI18n({ legacy: false, locale, fallbackLocale: "en-US", messages: { "zh-CN": zh, "en-US": en } });
}

export function formatDuration(milliseconds: number, locale: SupportedLocale): string {
	const seconds = Number.isFinite(milliseconds) ? Math.max(0, Math.round(milliseconds / 1000)) : 0;
  if (seconds < 60) return new Intl.NumberFormat(locale, { style: "unit", unit: "second", unitDisplay: "short" }).format(seconds);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainder = seconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`
    : `${minutes}:${String(remainder).padStart(2, "0")}`;
}

Object.assign((zh as unknown as { resources: Record<string, string> }).resources, { connectorExamples: "试试这样用", connectorDraftHint: "点击指引，在新会话中选中此连接器并填入输入框，确认后再发送。", connectorStarter: "使用{name}，帮我完成……", connectConnector: "连接" });
Object.assign((en as unknown as { resources: Record<string, string> }).resources, { connectorExamples: "Try these prompts", connectorDraftHint: "Choose a prompt to select this connector in a new conversation and fill the composer. Send when ready.", connectorStarter: "Use {name} to help me with…", connectConnector: "Connect" });

Object.assign((zh as unknown as { resources: Record<string, unknown> }).resources, { connectorMarket: "市场", installed: "已安装", disconnectConnector: "断开", connectorCategory: { collaboration: "沟通协作", documents: "知识文档", projects: "项目管理", design: "设计创作", other: "其他" } });
Object.assign((en as unknown as { resources: Record<string, unknown> }).resources, { connectorMarket: "Market", installed: "Installed", disconnectConnector: "Disconnect", connectorCategory: { collaboration: "Communication & collaboration", documents: "Knowledge & documents", projects: "Project management", design: "Design & creation", other: "Other" } });
