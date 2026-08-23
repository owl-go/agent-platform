import { createI18n } from "vue-i18n";

export type SupportedLocale = "zh-CN" | "en-US";
export const localeStorageKey = "agent-platform.locale";
export const runStates = ["queued", "provisioning", "running", "waiting_confirmation", "interrupting", "interrupted", "resuming", "completed", "failed", "cancelled"] as const;
export type RunState = typeof runStates[number];

const messages = {
  "zh-CN": {
    auth: {
      checkingCode: "身份 / 检查",
      checkingTitle: "正在恢复会话",
      checkingBody: "正在通过配置的 OIDC 提供方验证企业身份。",
      requiredCode: "身份 / 必需",
      requiredTitle: "进入 Agent Platform",
      requiredBody: "使用企业身份登录以访问受治理的 Agent 和 Run。",
      expiredBody: "会话已过期，请重新登录。",
      signIn: "使用 OIDC 登录",
      unavailableCode: "身份 / 不可用",
      unavailableTitle: "认证服务暂不可用",
      signOut: "退出 {name}",
    },
    navigation: { label: "产品区域", studio: "Agent Studio", workspace: "协作工作区", operations: "运维控制台", settings: "设置", open: "打开导航" },
    shell: { product: "AGENT PLATFORM", api: "API {status}", team: "当前 Team", language: "语言", userScope: "{organization} / {team}" },
    health: { checking: "检查中", online: "在线", offline: "离线" },
    locale: { zh: "简体中文", en: "English" },
    access: { deniedTitle: "此区域不可用", deniedBody: "当前 Team 的 Role Grant 不允许访问此区域。服务端仍会对每次请求重新授权。", noTeamTitle: "没有可访问的 Team", noTeamBody: "请联系平台管理员分配 Organization 或 Team 范围的 Role Grant。" },
    surfaces: {
      studio: { kicker: "构建 / 验证 / 发布", title: "Agent Studio", body: "为当前 Team 管理 Agent Draft 和不可变 Release。", emptyTitle: "尚无 Agent 数据", emptyBody: "真实 Agent 目录将在下一张票据中接入；这里不会展示模拟记录。" },
      workspace: { kicker: "协作 / 执行 / 审阅", title: "协作工作区", body: "在当前 Team 中管理 Coding Task、Session、Run 和 Memory。", emptyTitle: "尚无 Coding Task 数据", emptyBody: "真实协作 API 将在后续票据中接入；这里不会展示模拟会话。" },
      operations: { kicker: "观察 / 干预 / 恢复", title: "运维控制台", body: "检索并诊断当前 Team 的 Run。", emptyTitle: "尚无 Run 数据", emptyBody: "真实 Run 查询将在后续票据中接入；这里不会展示模拟运行记录。" },
    },
    workspace: {
      kicker: "协作 / 启动 / 恢复", title: "Conversation Workspace", body: "从当前 Team 的可用 Agent Release 启动 Coding Task，并从真实 Session 与首个 Run 恢复上下文。", readOnly: "只读工作区", loading: "正在读取 Coding Task 与可用发布依赖", loadingTask: "正在恢复 Coding Task 与 Session", errorBody: "请求未完成；未确认的 Coding Task 不会被创建。",
      launch: "启动 Coding Task", newTask: "委派新的编码任务", launchHint: "一次原子写入创建 Coding Task、Session、稳定 Review Branch 和首个 Run。", source: "任务输入来源", freeText: "自由文本", issueSnapshot: "Issue Snapshot", repositoryBinding: "Repository Binding", agentRelease: "Agent Release", taskTitle: "任务标题", requestText: "需求正文", issueTitle: "Issue 标题", issueBody: "Issue 正文", issueURL: "Issue 链接（可选）", launching: "正在启动…",
      tasks: "Coding Task", noTasks: "当前 Team 尚无 Coding Task。", selectTask: "选择一项 Coding Task 查看可恢复上下文", session: "Session", reviewBranch: "Review Branch", targetBranch: "目标分支", runtime: "Runtime", model: "Configured Model", firstRun: "最新 Run", runCount: "{count} Runs", immutableIssue: "不可变 Issue Snapshot",
      evidence: {
        title: "Run 执行证据", runs: "Run 列表", run: "Run {number}", attempt: "Attempt {number}", infrastructureFailure: "基础设施失败", noAttempts: "尚无 Attempt 记录", noEvents: "等待 Runtime Event", noArtifacts: "尚无 Artifact", usage: "Usage", cost: "成本", capabilities: "冻结 Capability", terminalError: "终态错误", contractError: "Run Event 合同或连接错误", artifactError: "Artifact 列表加载失败",
        categories: { plan: "计划", command: "命令", file: "文件", validation: "验证", diff: "Diff", approval: "审批", error: "错误", usage: "Usage", cost: "成本", terminal: "终态", run: "Run", runtime: "Runtime" },
        stream: { idle: "未连接", connecting: "加载历史事件", live: "实时连接", reconnecting: "正在从 Cursor 重连", complete: "终态已确认", error: "证据流错误" },
      },
      prerequisite: { runtime: "缺少 Production Runtime；请先完成 Runtime Conformance 与生产状态配置。", model: "缺少已启用的 Configured Model；请检查模型及其 Credential Profile。", binding: "缺少验证通过的 Repository Binding；请重新验证仓库依赖。", release: "当前 Repository Binding 没有可用的已发布 Agent Release。" },
      notice: { created: "Coding Task、Session、Review Branch 与首个 Run 已原子创建。" },
    },
    status: { queued: "排队中", provisioning: "准备环境", running: "运行中", waiting_confirmation: "等待确认", interrupting: "正在中断", interrupted: "已中断", resuming: "正在恢复", completed: "已完成", failed: "失败", cancelled: "已取消", lost: "执行节点丢失" },
    runtimeCatalog: {
      kicker: "平台目录 / 固定制品", title: "Runtime Image", body: "管理固定 Registry RepoDigest 及其 Runtime Adapter 声明。注册记录不可修改。",
      register: "注册 Runtime Image", readOnly: "只读目录", retry: "重试", loading: "正在读取真实 Runtime Catalog", emptyTitle: "尚未注册 Runtime Image", emptyBody: "目录为空。Organization 范围的 Platform Administrator 可以注册第一条固定 Digest。",
      errorBody: "请求未完成。服务端没有应用未经确认的更改。", requestId: "请求 ID：{id}", pagination: "Runtime Image 分页", previous: "上一页", next: "下一页",
      listLabel: "Runtime Image 列表", close: "关闭",
      registered: "已注册", runtime: "Agent Runtime", cliVersion: "CLI 版本", adapterVersion: "Adapter 版本", digest: "Registry RepoDigest", registeredAt: "注册时间",
      capabilities: "Runtime Capability 声明", capabilityCaveat: "这些值是该镜像的已注册声明；只有 Production 状态与对应 Conformance 证据才能表明已验证。",
      productionRuntime: "Production Runtime", evidenceRecorded: "已记录 Conformance 证据", noEvidence: "尚无 Conformance 证据", noEvidenceBody: "此固定 Digest 尚未关联 Conformance 证据，不能视为 Production Runtime。", evidenceKey: "Conformance 证据 Object Key", evidenceSHA256: "证据 SHA-256",
      blockedReason: "Blocked 原因", changeStatus: "生产状态", saveStatus: "保存状态", saving: "正在保存…", deprecatedFinal: "Deprecated 是不可逆终态；如需替代，请注册新的 Runtime Image。",
      registerTitle: "注册不可变 Runtime Image", immutableHint: "提交后 Runtime、版本、Digest 和 Capability 均不可编辑。不同制品必须创建新记录。", declaredCapabilities: "声明的 Capability（不等于 Conformance 验证）", cancel: "取消",
      notice: { registered: "Runtime Image 已注册。", statusChanged: "Runtime Image 状态已更新。" },
      status: { experimental: "Experimental", production: "Production Runtime", blocked: "Blocked", deprecated: "Deprecated" },
    },
    modelCatalog: {
      kicker: "平台目录 / 安全模型访问", title: "Model Catalog", body: "通过受治理的 Credential Profile 注册可用于企业代码的 Configured Model。",
      readOnly: "只读目录", policyTitle: "责任边界：", policyBody: "平台只治理 Endpoint、模型标识和安全凭证引用；不验证模型 Provider 的数据保留、训练、地区或合规政策。",
      loading: "正在读取真实 Model Catalog", errorBody: "Model Catalog 请求未完成，Secret 值不会出现在错误信息中。", credentials: "Credential Profile", models: "Configured Model",
      credentialSection: "01 / 凭证", modelSection: "02 / 模型", newCredential: "新建 / 凭证", newModel: "新建 / 模型", close: "关闭",
      addCredential: "注册 Credential Profile", addModel: "注册 Configured Model", noCredentials: "尚无模型凭证", noCredentialsBody: "先注册 Organization 范围的 model Credential Profile。", noModels: "尚无 Configured Model", noModelsBody: "启用的模型 Credential Profile 可用于注册第一个模型。",
      enabled: "已启用", disabled: "已禁用", enable: "启用", disable: "禁用", scope: "Scope", organizationScope: "Organization", teamScope: "Team", secret: "Secret 状态", secretConfigured: "已安全配置", secretMissing: "未配置", endpoint: "Endpoint", credential: "Credential Profile",
      missingCredential: "不可用的 Credential Profile", name: "名称", secretRef: "Secret Manager 引用", credentialHint: "只提交安全引用，例如 vault://team/model；不要在此粘贴 API Key 或 Secret 值。", modelID: "模型标识", chooseCredential: "选择已启用的模型凭证", register: "注册",
      notice: { credentialRegistered: "Credential Profile 已注册。", modelRegistered: "Configured Model 已注册。", credentialChanged: "Credential Profile 状态已更新，依赖模型已重新加载。", modelChanged: "Configured Model 状态已更新。" },
    },
    repositoryCatalog: {
      kicker: "平台目录 / 仓库治理", title: "Source Control 与 Repository Binding", body: "把受治理的 Git SSH 仓库、Runtime、模型、预算、质量命令和公网 Egress 组合为 Team 可用配置。", readOnly: "只读仓库目录",
      secretBoundary: "Secret 边界：", secretBoundaryBody: "界面只处理 Credential Profile ID 与安全状态；Git 私钥、known_hosts 内容和构建 Secret 不进入浏览器响应。", loading: "正在读取真实 Source Control 与 Repository Binding", errorBody: "请求未完成；安全表单内容仍保留在当前页面。", conflictBody: "已重新加载服务端权威版本；安全表单内容仍保留，请核对后重试。",
      providerSection: "01 / 源代码提供方", bindingSection: "02 / 仓库绑定", providers: "Source Control Provider", bindings: "Repository Binding", addProvider: "注册 Provider", addBinding: "创建 Repository Binding", noProviders: "尚无 Source Control Provider", noBindings: "当前 Team 尚无 Repository Binding",
      baseURL: "HTTPS Base URL", kind: "Provider 类型", provider: "Source Control Provider", newProvider: "新建 / PROVIDER", newBinding: "新建 / REPOSITORY BINDING", editBinding: "编辑 / REPOSITORY BINDING", repository: "Git SSH 仓库", repositorySSHURL: "Git SSH 地址", defaultBranch: "默认目标分支",
      sshCredential: "SSH Credential Profile ID", buildCredentials: "构建凭证引用", buildCredentialsHint: "构建 Credential Profile ID（逗号分隔）", gitAuthorName: "Git Commit 作者", gitAuthorEmail: "Git Commit 邮箱", allowedRuntimes: "允许的 Runtime Images", requiredCapabilities: "必需 Runtime Capability", defaultRuntime: "默认 Runtime Image", defaultModel: "默认 Configured Model", defaults: "默认 Runtime / 模型",
      inputBudget: "最大输入 Token", outputBudget: "最大输出 Token", costBudget: "最大模型金额", budget: "模型预算（输入 / 输出 / 金额）", instructions: "仓库指令", qualityCommand: "质量命令 {number}", qualityKind: "质量命令类型", qualityName: "质量命令名称", executable: "可执行文件", arguments: "结构化参数", argument: "参数 {number}", addArgument: "添加参数", removeArgument: "删除参数", addQualityCommand: "添加质量命令", removeQualityCommand: "删除质量命令", timeout: "超时秒数", egress: "Egress Policy", publicOnly: "仅允许公网",
      valid: "验证通过", invalid: "验证失败", unvalidated: "尚未验证", validationErrors: "Repository Binding 验证错误", validate: "重新验证", edit: "编辑", save: "保存", missingDependency: "依赖不可用",
      notice: { providerRegistered: "Source Control Provider 已注册。", providerChanged: "Source Control Provider 状态已更新。", bindingSaved: "Repository Binding 已保存，验证状态已清除。", bindingValidated: "Repository Binding 已使用当前依赖重新验证。" },
    },
    agentCatalog: {
      kicker: "Agent 生命周期 / 可验证配置", title: "Agent 与 Draft", body: "为当前 Team 创建稳定 Agent 身份，并编辑、验证可发布的 Draft。", readOnly: "只读 Agent 目录", loading: "正在读取真实 Agent Catalog", errorBody: "Agent 操作未完成，服务端没有应用未经确认的更改。", conflictBody: "已重新加载服务端权威 Version；安全表单内容仍保留，请核对后重试。",
      agents: "Agent 目录", drafts: "Agent Draft", noAgents: "当前 Team 尚无 Agent。", noDrafts: "此 Agent 尚无 Draft。", selectAgent: "选择 Agent", createAgent: "创建 Agent", createDraft: "创建 Draft", newAgent: "新建 / AGENT", newDraft: "新建 / DRAFT", editDraft: "编辑 / DRAFT", description: "描述", instructions: "Agent 指令", releaseRisk: "发布风险", nativeSubagents: "启用 Runtime 原生 Subagent（高风险）", timeout: "最长运行秒数", create: "创建", save: "保存", edit: "编辑", validate: "验证",
      repositoryBinding: "Repository Binding", runtimeImage: "Runtime Image", configuredModel: "Configured Model", cpu: "CPU 核数", memoryBytes: "内存字节数", pids: "进程数上限", tempBytes: "临时空间字节数", egress: "Egress Policy", publicEgress: "仅公网", validated: "已验证", unvalidated: "未验证", enabled: "已启用", disabled: "已禁用", risk: { low: "低风险", high: "高风险" },
      publish: "发布不可变 Release", releases: "Agent Release", releaseImmutable: "冻结配置与审批证据", noReleases: "此 Agent 尚无 Release。", capabilities: "已冻结 Capability", none: "无", deprecate: "Deprecated", block: "Block", blockRelease: "紧急 Block Agent Release", blockReason: "Block 原因", releaseStatus: { released: "Released", deprecated: "Deprecated", blocked: "Blocked" },
      releaseApproval: { title: "Release Approval", request: "申请 Release Approval", approve: "批准 Release", reject: "拒绝 Release", state: "审批状态", status: { pending: "待审批", approved: "已批准", rejected: "已拒绝" }, requestedBy: "申请人", draftVersion: "精确 Draft Version", riskReason: "高风险原因", decisionReason: "审批说明（拒绝时必填）", expired: "此审批属于旧 Draft Version，必须重新验证并审批。", notRequested: "尚未申请 Release Approval", evidence: "冻结审批证据" },
      state: { draft: "未保存验证", validating: "验证中", ready: "Ready", blocked: "Blocked" },
      notice: { agentCreated: "Agent 已创建。", draftSaved: "Agent Draft 已保存，旧 Validation Report 已清除。", validated: "Agent Draft 已使用当前依赖完成验证。", approvalRequested: "Release Approval 已绑定当前 Draft Version。", approvalDecided: "Release Approval 决定已记录。", published: "不可变 Agent Release 已发布。", deprecated: "Agent Release 已进入 Deprecated。", blocked: "Agent Release 已按原因 Block。" },
    },
    errors: { authentication: "无法完成身份验证，请重试或联系平台管理员。", offline: "服务离线", forbidden: "无权访问", validation: "请检查输入", conflict: "数据已被其他操作更新", server: "服务暂不可用" },
  },
  "en-US": {
    auth: {
      checkingCode: "IDENTITY / CHECK",
      checkingTitle: "Restoring your session",
      checkingBody: "Verifying your enterprise identity with the configured OIDC Provider.",
      requiredCode: "IDENTITY / REQUIRED",
      requiredTitle: "Enter Agent Platform",
      requiredBody: "Sign in with your enterprise identity to access governed Agents and Runs.",
      expiredBody: "Your session expired. Sign in again to continue.",
      signIn: "Sign in with OIDC",
      unavailableCode: "IDENTITY / UNAVAILABLE",
      unavailableTitle: "Authentication is unavailable",
      signOut: "Sign out {name}",
    },
    navigation: { label: "Product surfaces", studio: "Agent Studio", workspace: "Conversation Workspace", operations: "Operations Console", settings: "Settings", open: "Open navigation" },
    shell: { product: "AGENT PLATFORM", api: "API {status}", team: "Active Team", language: "Language", userScope: "{organization} / {team}" },
    health: { checking: "checking", online: "online", offline: "offline" },
    locale: { zh: "简体中文", en: "English" },
    access: { deniedTitle: "This surface is unavailable", deniedBody: "Role Grants for the active Team do not allow this surface. The server still authorizes every request.", noTeamTitle: "No accessible Team", noTeamBody: "Ask a platform administrator for an Organization- or Team-scoped Role Grant." },
    surfaces: {
      studio: { kicker: "BUILD / VALIDATE / RELEASE", title: "Agent Studio", body: "Manage Agent Drafts and immutable Releases for the active Team.", emptyTitle: "No Agent data yet", emptyBody: "The real Agent catalog is connected in the next ticket; this page does not show mock records." },
      workspace: { kicker: "COLLABORATE / EXECUTE / REVIEW", title: "Conversation Workspace", body: "Manage Coding Tasks, Sessions, Runs, and Memory for the active Team.", emptyTitle: "No Coding Task data yet", emptyBody: "Real collaboration APIs are connected in later tickets; this page does not show mock sessions." },
      operations: { kicker: "OBSERVE / INTERVENE / RECOVER", title: "Operations Console", body: "Search and diagnose Runs for the active Team.", emptyTitle: "No Run data yet", emptyBody: "The real Run search is connected in a later ticket; this page does not show mock runs." },
    },
    workspace: {
      kicker: "COLLABORATE / LAUNCH / RESTORE", title: "Conversation Workspace", body: "Launch Coding Tasks from available Agent Releases in the active Team and restore their real Session and first Run context.", readOnly: "Read-only workspace", loading: "Loading Coding Tasks and available release dependencies", loadingTask: "Restoring Coding Task and Session", errorBody: "The request did not complete; no unconfirmed Coding Task is created.",
      launch: "Launch Coding Task", newTask: "Delegate new coding work", launchHint: "One atomic write creates the Coding Task, Session, stable Review Branch, and first Run.", source: "Task input source", freeText: "Free text", issueSnapshot: "Issue Snapshot", repositoryBinding: "Repository Binding", agentRelease: "Agent Release", taskTitle: "Task title", requestText: "Request text", issueTitle: "Issue title", issueBody: "Issue body", issueURL: "Issue link (optional)", launching: "Launching…",
      tasks: "Coding Tasks", noTasks: "No Coding Tasks exist for this Team.", selectTask: "Select a Coding Task to inspect its recoverable context", session: "Session", reviewBranch: "Review Branch", targetBranch: "Target branch", runtime: "Runtime", model: "Configured Model", firstRun: "Latest Run", runCount: "{count} Runs", immutableIssue: "Immutable Issue Snapshot",
      evidence: {
        title: "Run evidence", runs: "Run history", run: "Run {number}", attempt: "Attempt {number}", infrastructureFailure: "Infrastructure failure", noAttempts: "No Attempts recorded", noEvents: "Waiting for Runtime Events", noArtifacts: "No Artifacts recorded", usage: "Usage", cost: "Cost", capabilities: "Frozen capabilities", terminalError: "Terminal error", contractError: "Run Event contract or connection error", artifactError: "Artifact list failed to load",
        categories: { plan: "Plan", command: "Command", file: "File", validation: "Validation", diff: "Diff", approval: "Approval", error: "Error", usage: "Usage", cost: "Cost", terminal: "Terminal", run: "Run", runtime: "Runtime" },
        stream: { idle: "Disconnected", connecting: "Loading event history", live: "Live stream", reconnecting: "Reconnecting from cursor", complete: "Terminal event confirmed", error: "Evidence stream error" },
      },
      prerequisite: { runtime: "No Production Runtime is available. Complete Runtime Conformance and production status first.", model: "No enabled Configured Model is available. Check the model and its Credential Profile.", binding: "No validated Repository Binding is available. Revalidate repository dependencies.", release: "This Repository Binding has no available published Agent Release." },
      notice: { created: "Coding Task, Session, Review Branch, and first Run were created atomically." },
    },
    status: { queued: "Queued", provisioning: "Provisioning", running: "Running", waiting_confirmation: "Waiting for confirmation", interrupting: "Interrupting", interrupted: "Interrupted", resuming: "Resuming", completed: "Completed", failed: "Failed", cancelled: "Cancelled", lost: "Worker lost" },
    runtimeCatalog: {
      kicker: "PLATFORM CATALOG / PINNED ARTIFACTS", title: "Runtime Images", body: "Govern pinned Registry RepoDigests and their declared Runtime Adapter behavior. Registrations are immutable.",
      register: "Register Runtime Image", readOnly: "Read-only catalog", retry: "Retry", loading: "Loading the real Runtime Catalog", emptyTitle: "No Runtime Images registered", emptyBody: "The catalog is empty. An organization-scoped Platform Administrator can register the first pinned digest.",
      errorBody: "The request did not complete. The server did not apply an unconfirmed change.", requestId: "Request ID: {id}", pagination: "Runtime Image pagination", previous: "Previous", next: "Next",
      listLabel: "Runtime Image list", close: "Close",
      registered: "Registered", runtime: "Agent Runtime", cliVersion: "CLI version", adapterVersion: "Adapter version", digest: "Registry RepoDigest", registeredAt: "Registered at",
      capabilities: "Runtime Capability declarations", capabilityCaveat: "These values are registered declarations. Only Production status backed by corresponding Conformance evidence means verified.",
      productionRuntime: "Production Runtime", evidenceRecorded: "Conformance evidence recorded", noEvidence: "No Conformance evidence", noEvidenceBody: "This pinned digest has no linked Conformance evidence and is not a Production Runtime.", evidenceKey: "Conformance evidence Object Key", evidenceSHA256: "Evidence SHA-256",
      blockedReason: "Blocked reason", changeStatus: "Production status", saveStatus: "Save status", saving: "Saving…", deprecatedFinal: "Deprecated is irreversible. Register a new Runtime Image to provide a replacement.",
      registerTitle: "Register an immutable Runtime Image", immutableHint: "Runtime, versions, digest, and capabilities cannot be edited after submission. A different artifact requires a new registration.", declaredCapabilities: "Declared capabilities (not Conformance evidence)", cancel: "Cancel",
      notice: { registered: "Runtime Image registered.", statusChanged: "Runtime Image status updated." },
      status: { experimental: "Experimental", production: "Production Runtime", blocked: "Blocked", deprecated: "Deprecated" },
    },
    modelCatalog: {
      kicker: "PLATFORM CATALOG / SAFE MODEL ACCESS", title: "Model Catalog", body: "Register Configured Models for enterprise code through governed Credential Profiles.",
      readOnly: "Read-only catalog", policyTitle: "Responsibility boundary:", policyBody: "The platform governs the Endpoint, model identifier, and safe credential reference. It does not verify a model Provider's retention, training, region, or compliance policies.",
      loading: "Loading the real Model Catalog", errorBody: "The Model Catalog request did not complete. Secret values are never included in errors.", credentials: "Credential Profiles", models: "Configured Models",
      credentialSection: "01 / CREDENTIAL", modelSection: "02 / MODEL", newCredential: "NEW / CREDENTIAL", newModel: "NEW / MODEL", close: "Close",
      addCredential: "Register Credential Profile", addModel: "Register Configured Model", noCredentials: "No model credentials", noCredentialsBody: "Register an Organization-scoped model Credential Profile first.", noModels: "No Configured Models", noModelsBody: "An enabled model Credential Profile can register the first model.",
      enabled: "Enabled", disabled: "Disabled", enable: "Enable", disable: "Disable", scope: "Scope", organizationScope: "Organization", teamScope: "Team", secret: "Secret status", secretConfigured: "Safely configured", secretMissing: "Not configured", endpoint: "Endpoint", credential: "Credential Profile",
      missingCredential: "Unavailable Credential Profile", name: "Name", secretRef: "Secret Manager reference", credentialHint: "Submit only a safe reference such as vault://team/model. Never paste an API key or secret value here.", modelID: "Model identifier", chooseCredential: "Choose an enabled model credential", register: "Register",
      notice: { credentialRegistered: "Credential Profile registered.", modelRegistered: "Configured Model registered.", credentialChanged: "Credential Profile status updated; dependent models reloaded.", modelChanged: "Configured Model status updated." },
    },
    repositoryCatalog: {
      kicker: "PLATFORM CATALOG / REPOSITORY GOVERNANCE", title: "Source Control & Repository Bindings", body: "Compose governed Git SSH repositories, Runtimes, models, budgets, quality commands, and public Egress into Team-ready configurations.", readOnly: "Read-only repository catalog",
      secretBoundary: "Secret boundary:", secretBoundaryBody: "The browser handles only Credential Profile IDs and safe status. Git private keys, known_hosts content, and build secrets never enter browser responses.", loading: "Loading real Source Control and Repository Binding data", errorBody: "The request did not complete. Safe form input remains on this page.", conflictBody: "The authoritative server Version was reloaded. Review the preserved safe form input before retrying.",
      providerSection: "01 / SOURCE CONTROL", bindingSection: "02 / REPOSITORY BINDINGS", providers: "Source Control Providers", bindings: "Repository Bindings", addProvider: "Register Provider", addBinding: "Create Repository Binding", noProviders: "No Source Control Providers", noBindings: "No Repository Bindings for this Team",
      baseURL: "HTTPS Base URL", kind: "Provider kind", provider: "Source Control Provider", newProvider: "NEW / PROVIDER", newBinding: "NEW / REPOSITORY BINDING", editBinding: "EDIT / REPOSITORY BINDING", repository: "Git SSH repository", repositorySSHURL: "Git SSH URL", defaultBranch: "Default target branch",
      sshCredential: "SSH Credential Profile ID", buildCredentials: "Build credential references", buildCredentialsHint: "Build Credential Profile IDs (comma-separated)", gitAuthorName: "Git Commit author", gitAuthorEmail: "Git Commit email", allowedRuntimes: "Allowed Runtime Images", requiredCapabilities: "Required Runtime Capabilities", defaultRuntime: "Default Runtime Image", defaultModel: "Default Configured Model", defaults: "Default Runtime / model",
      inputBudget: "Maximum input tokens", outputBudget: "Maximum output tokens", costBudget: "Maximum model cost", budget: "Model budget (input / output / cost)", instructions: "Repository instructions", qualityCommand: "Quality command {number}", qualityKind: "Quality command kind", qualityName: "Quality command name", executable: "Executable", arguments: "Structured arguments", argument: "Argument {number}", addArgument: "Add argument", removeArgument: "Remove argument", addQualityCommand: "Add quality command", removeQualityCommand: "Remove quality command", timeout: "Timeout seconds", egress: "Egress Policy", publicOnly: "public internet only",
      valid: "Validated", invalid: "Validation failed", unvalidated: "Not validated", validationErrors: "Repository Binding validation errors", validate: "Validate again", edit: "Edit", save: "Save", missingDependency: "Dependency unavailable",
      notice: { providerRegistered: "Source Control Provider registered.", providerChanged: "Source Control Provider status updated.", bindingSaved: "Repository Binding saved; validation state cleared.", bindingValidated: "Repository Binding revalidated against current dependencies." },
    },
    agentCatalog: {
      kicker: "AGENT LIFECYCLE / VALIDATED CONFIG", title: "Agents & Drafts", body: "Create stable Agent identities for the active Team, then edit and validate release-ready Drafts.", readOnly: "Read-only Agent catalog", loading: "Loading the real Agent Catalog", errorBody: "The Agent operation did not complete. The server did not apply an unconfirmed change.", conflictBody: "The authoritative server Version was reloaded. Review the preserved safe form before retrying.",
      agents: "Agent catalog", drafts: "Agent Drafts", noAgents: "No Agents exist for this Team.", noDrafts: "This Agent has no Drafts.", selectAgent: "Select an Agent", createAgent: "Create Agent", createDraft: "Create Draft", newAgent: "NEW / AGENT", newDraft: "NEW / DRAFT", editDraft: "EDIT / DRAFT", description: "Description", instructions: "Agent instructions", releaseRisk: "Release risk", nativeSubagents: "Enable Runtime-native Subagents (high risk)", timeout: "Maximum runtime seconds", create: "Create", save: "Save", edit: "Edit", validate: "Validate",
      repositoryBinding: "Repository Binding", runtimeImage: "Runtime Image", configuredModel: "Configured Model", cpu: "CPU cores", memoryBytes: "Memory bytes", pids: "Process limit", tempBytes: "Temporary storage bytes", egress: "Egress Policy", publicEgress: "public internet only", validated: "Validated", unvalidated: "Not validated", enabled: "Enabled", disabled: "Disabled", risk: { low: "Low risk", high: "High risk" },
      publish: "Publish immutable Release", releases: "Agent Releases", releaseImmutable: "Frozen configuration and approval evidence", noReleases: "This Agent has no Releases.", capabilities: "Frozen Capabilities", none: "None", deprecate: "Deprecate", block: "Block", blockRelease: "Emergency Block Agent Release", blockReason: "Block reason", releaseStatus: { released: "Released", deprecated: "Deprecated", blocked: "Blocked" },
      releaseApproval: { title: "Release Approval", request: "Request Release Approval", approve: "Approve Release", reject: "Reject Release", state: "Approval state", status: { pending: "Pending", approved: "Approved", rejected: "Rejected" }, requestedBy: "Requested by", draftVersion: "Exact Draft Version", riskReason: "High-risk reason", decisionReason: "Decision reason (required for rejection)", expired: "This Approval belongs to an older Draft Version. Revalidate and request approval again.", notRequested: "No Release Approval requested", evidence: "Frozen approval evidence" },
      state: { draft: "Unvalidated", validating: "Validating", ready: "Ready", blocked: "Blocked" },
      notice: { agentCreated: "Agent created.", draftSaved: "Agent Draft saved; the previous Validation Report was cleared.", validated: "Agent Draft validated against current dependencies.", approvalRequested: "Release Approval is bound to the current Draft Version.", approvalDecided: "Release Approval decision recorded.", published: "Immutable Agent Release published.", deprecated: "Agent Release deprecated.", blocked: "Agent Release blocked with its reason." },
    },
    errors: { authentication: "Authentication could not be completed. Try again or contact a platform administrator.", offline: "Service is offline", forbidden: "Access denied", validation: "Check the entered values", conflict: "Data changed in another operation", server: "Service is unavailable" },
  },
} as const;

export function resolveInitialLocale(stored: string | null, browserLanguage: string): SupportedLocale {
  if (stored === "zh-CN" || stored === "en-US") return stored;
  return browserLanguage.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}

export function createAppI18n(storage: Pick<Storage, "getItem"> = localStorage, browserLanguage = navigator.language) {
  const locale = resolveInitialLocale(storage.getItem(localeStorageKey), browserLanguage);
  document.documentElement.lang = locale;
  return createI18n({
    legacy: false, locale, fallbackLocale: "en-US", messages,
    datetimeFormats: {
      "zh-CN": { long: { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" } },
      "en-US": { long: { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" } },
    },
  });
}

export function formatDuration(milliseconds: number, locale: SupportedLocale): string {
  const seconds = Math.max(0, Math.round(milliseconds / 1000));
  if (seconds < 60) return new Intl.NumberFormat(locale, { style: "unit", unit: "second", unitDisplay: "short" }).format(seconds);
  const minutes = Math.round(seconds / 60);
  return new Intl.NumberFormat(locale, { style: "unit", unit: "minute", unitDisplay: "short" }).format(minutes);
}

export function formatTokenUsage(value: number, locale: SupportedLocale): string {
  return new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

export function formatCount(value: number, locale: SupportedLocale): string {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(value);
}

export function formatModelCost(value: number, locale: SupportedLocale): string {
  return new Intl.NumberFormat(locale, { style: "currency", currency: "USD" }).format(value);
}

export function runStateLabel(state: RunState, locale: SupportedLocale): string {
  return messages[locale].status[state];
}
