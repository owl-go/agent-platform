import { createI18n } from "vue-i18n";

export type SupportedLocale = "zh-CN" | "en-US";
export const localeStorageKey = "agent-platform.locale";

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
    status: { queued: "排队中", running: "运行中", waiting_confirmation: "等待确认", completed: "已完成", failed: "失败", cancelled: "已取消" },
    errors: { offline: "服务离线", forbidden: "无权访问", validation: "请检查输入", conflict: "数据已被其他操作更新", server: "服务暂不可用" },
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
    status: { queued: "Queued", running: "Running", waiting_confirmation: "Waiting for confirmation", completed: "Completed", failed: "Failed", cancelled: "Cancelled" },
    errors: { offline: "Service is offline", forbidden: "Access denied", validation: "Check the entered values", conflict: "Data changed in another operation", server: "Service is unavailable" },
  },
} as const;

export function resolveInitialLocale(stored: string | null, browserLanguage: string): SupportedLocale {
  if (stored === "zh-CN" || stored === "en-US") return stored;
  return browserLanguage.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
}

export function createAppI18n(storage: Pick<Storage, "getItem"> = localStorage, browserLanguage = navigator.language) {
  const locale = resolveInitialLocale(storage.getItem(localeStorageKey), browserLanguage);
  document.documentElement.lang = locale;
  return createI18n({ legacy: false, locale, fallbackLocale: "en-US", messages });
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

export function formatModelCost(value: number, locale: SupportedLocale): string {
  return new Intl.NumberFormat(locale, { style: "currency", currency: "USD" }).format(value);
}
