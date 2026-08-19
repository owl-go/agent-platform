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
    status: { queued: "排队中", provisioning: "准备环境", running: "运行中", waiting_confirmation: "等待确认", interrupting: "正在中断", interrupted: "已中断", resuming: "正在恢复", completed: "已完成", failed: "失败", cancelled: "已取消" },
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
    status: { queued: "Queued", provisioning: "Provisioning", running: "Running", waiting_confirmation: "Waiting for confirmation", interrupting: "Interrupting", interrupted: "Interrupted", resuming: "Resuming", completed: "Completed", failed: "Failed", cancelled: "Cancelled" },
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
