<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useZIndex } from "element-plus";
import ToastMessage from "./ToastMessage.vue";
import { Check, ChevronDown, Pencil, Plus, Trash2 } from "@lucide/vue";
import CatalogDetails from "./CatalogDetails.vue";
import { ApiError, platformApiKey, type CLIConnectorAuthorization, type CLIConnectorAuthorizationFlow, type CLIConnectorDefinition, type CLIConnectorDefinitionInput, type CLIConnectorEnablement, type CLIRecommendedSkill, type ConnectorAuthorization, type ConnectorAuthorizationFlow, type ConnectorInstallation, type ConnectorPublication, type ConnectorSetup, type EnvironmentVariable, type MCPServer, type ResourceDeletionImpact, type Skill } from "../api/client";
import { authContextKey } from "../auth/session";
import ConfirmDialog from "./ConfirmDialog.vue";
import { localizedSkillDescription, parseSkillDocument } from "../skillDocument";
import ConnectorDetails from "./ConnectorDetails.vue";
import ConnectorIcon from "./ConnectorIcon.vue";
import ProfileIcon from "./ProfileIcon.vue";
import IconPicker from "./IconPicker.vue";
import ResourceTrustMeta from "./ResourceTrustMeta.vue";

type ResourceTab = "mcp" | "skills";
type ConnectorCatalogEntry = { publication?: ConnectorPublication; installation?: ConnectorInstallation };
type MCPDraft = { name: string; icon: string; transport: "streamable_http" | "stdio"; url: string; runner: "npx" | "uvx"; package: string; package_version: string; argumentsText: string; environment: EnvironmentVariable[]; bearerToken: string };
type CLIDraft = { name: string; icon: string; description: string; installation_type: "npm" | "upload"; npm_install: string; archive: string };

const props = withDefaults(defineProps<{ selectable?: boolean; initialTab?: ResourceTab; mineOnly?: boolean; showTabs?: boolean; catalogQuery?: string; availableOnly?: boolean; mcpServerIds?: string[]; skillIds?: string[]; cliConnectorDefinitionIds?: string[] }>(), {
  selectable: false,
  initialTab: "mcp",
  mineOnly: false,
  showTabs: true,
  catalogQuery: "",
  availableOnly: false,
  mcpServerIds: () => [],
  skillIds: () => [],
  cliConnectorDefinitionIds: () => [],
});
const emit = defineEmits<{
  "update:mcpServerIds": [value: string[]];
  "update:skillIds": [value: string[]];
  "update:cliConnectorDefinitionIds": [value: string[]];
  resources: [value: { mcp: MCPServer[]; skills: Skill[] }];
  tabChange: [value: ResourceTab];
  error: [];
}>();
const api = inject(platformApiKey)!;
const router = useRouter();
const detailSkill = ref<Skill>();
const skillDocuments = ref<Record<string, string>>({});
function useSkill(item: Skill) { void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), skill_id: item.id } }); }
const createSkillPrompt = "请帮我创建一个可以实现「……」的技能";
function systemSkill(systemKey: string, fallback: string) { return skills.value.find((item) => item.system_key === systemKey) ?? skills.value.find((item) => item.platform && item.name === fallback); }
function createSkillSession() {
  const skill = systemSkill("system.create_skill", "Create Skill");
  if (!skill) { reportError(undefined, "systemSkillUnavailable"); return; }
  void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), skill_id: skill.id, draft: createSkillPrompt } });
}
function createConnectorSession() {
  const skill = systemSkill("system.create_connector", "Create Connector");
  if (!skill) { reportError(undefined, "systemSkillUnavailable"); return; }
  showConnectorKind.value = false;
  void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), skill_id: skill.id, draft: "请帮我创建一个连接器，用于……" } });
}
function handleSkillAction(command: "create" | "upload") { if (command === "create") createSkillSession(); else openNewSkill(); }
const auth = inject(authContextKey, undefined);
const { locale, t } = useI18n();
const { nextZIndex } = useZIndex();
const operationError = ref<{ message: string; zIndex: number }>();
const statusErrors = ref<string[]>([]);
function reportError(cause?: unknown, validationKey = "invalidInput") {
  const keys = { unauthenticated: "loginRequired", forbidden: "permissionDenied", not_found: "resourceMissing", conflict: "resourceChanged", validation: validationKey, rate_limited: "tooManyRequests", unavailable: "serviceUnavailable", unknown: "operationFailed" } as const;
  const authorizationErrors: Record<string, string> = { tianyancha_region_blocked: "tianyanchaRegionBlocked", xiaoe_oauth_callback_blocked: "xiaoeOAuthCallbackBlocked", dingtalk_cli_access_disabled: "dingtalkCLIAccessDisabled", dingtalk_cli_enterprise_denied: "dingtalkCLIEnterpriseDenied", dingtalk_cli_user_denied: "dingtalkCLIUserDenied", dingtalk_cli_channel_required: "dingtalkCLIChannelRequired", dingtalk_cli_auth_expired: "dingtalkCLIAuthExpired", dingtalk_identity_mismatch: "dingtalkIdentityMismatch", dingtalk_authorization_failed: "dingtalkAuthorizationFailed" };
  const key = cause instanceof ApiError ? authorizationErrors[cause.code] ?? (cause.status === 413 ? "uploadTooLarge" : keys[cause.kind]) : cause instanceof TypeError ? "networkFailed" : "operationFailed";
  operationError.value = { message: t(`resources.${key}`), zIndex: nextZIndex() };
  emit("error");
}
function setStatusError(source: string, failed: boolean) {
  statusErrors.value = statusErrors.value.filter((item) => item !== source);
  if (failed) statusErrors.value.push(source);
}
const canManageCLI = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const activeTab = ref<ResourceTab>(props.initialTab);
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const createdSkillIDs = ref(new Set<string>());
const cliDefinitions = ref<CLIConnectorDefinition[]>([]);
const connectorPublications = ref<ConnectorPublication[]>([]);
const connectorInstallations = ref<ConnectorInstallation[]>([]);
const connectorAuthorizations = ref<Record<string, ConnectorAuthorization[]>>({});
const connectorBusy = ref<string[]>([]);
const connectorSetups = ref<Record<string, ConnectorSetup>>({});
const connectorAuthorizationFlows = ref<Record<string, ConnectorAuthorizationFlow>>({});
const providedConnection = ref<{ installation: ConnectorInstallation; botID: string; secret: string }>();
const providedConnectionBusy = ref(false);
const connectorFlowWindows = new Map<string, Window | null>();
const reportedAuthorizationFlowErrors = new Set<string>();
const cliEnablements = ref<CLIConnectorEnablement[]>([]);
const cliAuthorizations = ref<Record<string, CLIConnectorAuthorization[]>>({});
const cliAuthorizationFlow = ref<CLIConnectorAuthorizationFlow>();
const editingCLI = ref<CLIConnectorDefinition>();
const deletingCLI = ref<CLIConnectorDefinition>();
const cliDeleteBusy = ref(false);
const cliSaveBusy = ref(false);
const cliEnableBusy = ref<string[]>([]);
const cliAuthorizationBusy = ref<string[]>([]);
const cliSetupWindows = new Map<string, Window | null>();
let cliCompletionBusy = false;
let publishedCompletionBusy = false;
let disposed = false;
const showCLI = ref(false);
const showConnectorKind = ref(false);
const connectorPackageInput = ref<HTMLInputElement>();
const cliForm = ref<CLIDraft>(emptyCLIDraft());
const editingMCP = ref<MCPServer>();
const showMCP = ref(false);
const detailMCP = ref<MCPServer>();
const detailCLI = ref<CLIConnectorDefinition>();
const detailPackageSource = ref<string>();
const launchingConnector = ref(false);
const connectorView = ref<"market" | "installed">(props.mineOnly ? "installed" : "market");
const detailPackage = computed(() => connectorCatalogItems.value.find((entry) => detailPackageSource.value
  ? (entry.publication?.source || entry.installation?.source) === detailPackageSource.value
  : Boolean(entry.installation && entry.installation.id === (detailCLI.value?.id || detailMCP.value?.id))));
function closeConnectorDetails() { detailMCP.value = undefined; detailCLI.value = undefined; detailPackageSource.value = undefined; }
function showPackageDetails(entry: ConnectorCatalogEntry) { closeConnectorDetails(); detailPackageSource.value = entry.publication?.source || entry.installation?.source; }
async function connectFromDetails() {
  const entry = detailPackage.value;
  if (entry?.installation) {
    if (["notion", "teambition", "xiaoe", "github", "camscanner", "kling-ai", "linear", "pixso", "tianyancha"].includes(entry.installation.source)) await startBrowserConnection(entry.installation, entry.publication);
    else if (entry.installation.state === "disabled" && entry.publication) await installPublication(entry.publication);
    else if (["wecom", "modao", "picset-ai", "ai-hive", "openboost"].includes(entry.installation.source)) { const installation = entry.installation; closeConnectorDetails(); openProvidedConnection(installation); }
    else if (entry.installation.authentication_driver === "feishu" || entry.installation.authentication_driver === "dingtalk") await setupPublishedConnector(entry.installation, entry.publication);
  } else if (entry?.publication) await installPublication(entry.publication);
  else if (detailCLI.value) {
    const item = detailCLI.value;
    if (enablementFor(item.id)?.state === "waiting_for_user") resumeCLISetup(item);
    else if (enablementFor(item.id)?.state === "enabled") await authorizeCLIAccount(item);
    else await enableCLI(item);
  }
  else if (detailMCP.value) await testMCP(detailMCP.value);
}
const detailBusy = computed(() => launchingConnector.value || Boolean(detailPackage.value && connectorOperationBusy(detailPackage.value.publication?.source || detailPackage.value.installation?.source || "")) || Boolean(detailCLI.value && (cliEnableBusy.value.includes(detailCLI.value.id) || cliAuthorizationBusy.value.includes(detailCLI.value.id))));
const detailCanConnect = computed(() => Boolean(detailPackage.value?.publication && (!detailPackage.value.installation || detailPackage.value.installation.state === "disabled") || detailPackage.value?.installation && ["feishu", "dingtalk", "notion", "teambition", "xiaoe", "github", "camscanner", "kling-ai", "linear", "pixso", "tianyancha", "wecom", "modao", "picset-ai", "ai-hive", "openboost"].includes(detailPackage.value.installation.source) || detailCLI.value || detailMCP.value && ((!detailMCP.value.platform && !detailMCP.value.managed_installation) || canManageCLI.value)));
async function disconnectFromDetails() {
  const installation = detailPackage.value?.installation;
  if (installation) await runConnectorOperation(installation.source, async () => {
    for (const authorization of connectorAuthorizations.value[installation.id] ?? []) {
      if (authorization.state === "active" || authorization.state === "expired") await api.disconnectPublishedConnectorAuthorization(authorization.id);
    }
  });
  else if (detailCLI.value) {
    launchingConnector.value = true;
    try { for (const authorization of authorizationsFor(detailCLI.value.id)) if (authorization.state === "active") await disconnectCLIAccount(authorization); }
    finally { launchingConnector.value = false; }
  }
}
async function uninstallFromDetails() {
  const installation = detailPackage.value?.installation;
  if (installation) await uninstallInstallation(installation);
  else if (detailCLI.value) await deactivateCLI(detailCLI.value);
}
async function useConnectorPrompt(prompt: string) {
  if (launchingConnector.value) return;
  launchingConnector.value = true;
  try {
    const entry = detailPackage.value;
    let installation = entry?.installation;
    if ((!installation || installation.state === "disabled") && entry?.publication) {
      await installPublication(entry.publication);
      installation = connectorInstallations.value.find((item) => item.source === entry.publication!.source);
      if (!installation) return;
    }
    const kind = installation?.mode || entry?.publication?.revision.mode || (detailCLI.value ? "cli" : "mcp");
    const id = installation?.id || detailCLI.value?.id || detailMCP.value?.id;
    if (!id) return;
    closeConnectorDetails();
    await router.push({ path: "/sessions", query: { new: crypto.randomUUID(), connector_kind: kind, connector_id: id, draft: prompt } });
  } finally { launchingConnector.value = false; }
}
const mcpForm = ref<MCPDraft>(emptyMCPDraft());
const editingSkill = ref<Skill>();
const showSkill = ref(false);
const skillForm = ref({ source: "git" as "git" | "upload", git_url: "", git_ref: "", archive: "", icon: "sparkles" });
const pendingDelete = ref<({ kind: "mcp"; item: MCPServer } | { kind: "skill"; item: Skill }) & { impact: ResourceDeletionImpact }>();
const deleteBusy = ref(false);
let poll: number | undefined;
let lastCLICompletionPoll = 0;
let browserReturnRefreshUntil = 0;
const connectorCatalogItems = computed(() => {
  const bySource = new Map<string, ConnectorCatalogEntry>(connectorPublications.value.map((publication) => [publication.source, { publication, installation: connectorInstallations.value.find((item) => item.source === publication.source) }]));
  for (const installation of connectorInstallations.value) if (!bySource.has(installation.source)) bySource.set(installation.source, { publication: undefined, installation });
  return [...bySource.values()];
});
const queryNeedle = computed(() => props.catalogQuery.trim().toLocaleLowerCase());
function matchesCatalog(...values: Array<string | undefined>) { return !queryNeedle.value || values.join(" ").toLocaleLowerCase().includes(queryNeedle.value); }
const visibleMCP = computed(() => mcp.value.filter((item) => (!props.availableOnly || item.tested) && matchesCatalog(item.name, item.url, item.package, item.transport)));
const visibleCLI = computed(() => cliDefinitions.value.filter((item) => (!props.availableOnly || (item.state === "available" && (item.conformance_runtime_digests?.length ?? 0) > 0)) && matchesCatalog(item.name, item.description, item.npm_package, ...(item.capabilities ?? []).map((capability) => capability.id))));
const visibleConnectorCatalogItems = computed(() => connectorCatalogItems.value.filter((entry) => {
  const available = entry.publication?.state === "available" && entry.publication.revision.conformance_available;
  return (!props.availableOnly || available) && matchesCatalog(entry.publication?.revision.name, entry.publication?.revision.description, entry.installation?.name, entry.installation?.description, entry.publication?.source, entry.installation?.source);
}));
// Categories describe existing connector sources; unknown and private connectors stay discoverable.
function connectorCategory(source: string) {
  if (source === "tianyancha") return "industry";
  if (["xiaoe", "openboost"].includes(source)) return "marketing";
  if (["caoliao", "camscanner"].includes(source)) return "productivity";
  if (["feishu", "dingtalk", "wecom", "@larksuite/cli"].includes(source)) return "collaboration";
  if (source === "notion") return "documents";
  if (["teambition", "linear", "github"].includes(source)) return "projects";
  if (["modao", "picset-ai", "kling-ai", "pixso", "ai-hive"].includes(source)) return "design";
  return "other";
}
const installedOnly = computed(() => props.selectable && props.mineOnly || connectorView.value === "installed");
const connectorSections = computed(() => ["collaboration", "documents", "projects", "design", "marketing", "productivity", "industry", "other"].map(key => ({
  key, title: t(`resources.connectorCategory.${key}`),
  mcp: visibleMCP.value.filter(item => !item.managed_installation && connectorCategory("") === key && (!props.mineOnly || !props.selectable || !item.platform)),
  cli: visibleCLI.value.filter(item => !item.managed_installation && connectorCategory(item.npm_package) === key && (!installedOnly.value || cliInstalled(item))),
  packages: visibleConnectorCatalogItems.value.filter(entry => connectorCategory(entry.publication?.source || entry.installation?.source || "") === key && (!installedOnly.value || Boolean(entry.installation))),
})).filter(section => section.mcp.length || section.cli.length || section.packages.length));
const skillSections = computed(() => {
  const visible = skills.value.filter((item) => matchesCatalog(skillDisplayName(item), skillDescription(item), item.git_url, item.source));
  return (props.mineOnly
    ? [{ key: "mine", title: t("resources.mySkills"), items: visible.filter((item) => !item.platform) }]
    : [
        { key: "platform", title: t("resources.platformSkills"), items: visible.filter((item) => item.platform) },
        { key: "mine", title: t("resources.mySkills"), items: visible.filter((item) => !item.platform) },
      ]).filter((section) => section.items.length);
});

onMounted(() => {
  void completeBrowserReturn();
  poll = window.setInterval(() => {
    if (Date.now() < browserReturnRefreshUntil) void refresh();
    if (mcp.value.some((item) => item.test_pending)) void refreshMCP();
    if (cliDefinitions.value.some((item) => item.state === "building" || item.state === "testing")) void refreshCLI();
    if (cliEnablements.value.some((item) => item.state === "waiting_for_user") && Date.now() - lastCLICompletionPoll >= 5000) {
      lastCLICompletionPoll = Date.now();
      void completePendingCLIEnablements();
    }
    if (cliAuthorizationFlow.value?.state === "waiting_for_user" && Date.now() - lastCLICompletionPoll >= 5000) void completeCLIAccountAuthorization();
    if ((Object.values(connectorSetups.value).some((item) => item.state === "waiting_for_user") || Object.values(connectorAuthorizationFlows.value).some((item) => item.state === "waiting_for_user")) && Date.now() - lastCLICompletionPoll >= 5000) void completePublishedConnectorFlows();
  }, 1500);
});
onBeforeUnmount(() => {
  disposed = true;
  if (poll !== undefined) window.clearInterval(poll);
  for (const popup of cliSetupWindows.values()) closeBlankCLIWindow(popup);
  for (const popup of connectorFlowWindows.values()) closeBlankCLIWindow(popup);
  cliSetupWindows.clear();
});

async function completeBrowserReturn() {
  await refresh();
  const flowID = router?.currentRoute.value.query.xiaoe_auth ?? router?.currentRoute.value.query.connector_auth ?? router?.currentRoute.value.query.linear_auth ?? router?.currentRoute.value.query.teambition_auth;
  if (typeof flowID !== "string" || !/^[0-9a-f-]{36}$/i.test(flowID)) return;
  try { await api.completeConnectorAuthorizationFlow(flowID); }
  catch (cause) { if (!(cause instanceof ApiError && cause.kind === "not_found")) reportError(cause); }
  // Another tab may already be exchanging this single-use code. Read the saved
  // installation briefly so both the returning browser and its opener converge.
  browserReturnRefreshUntil = Date.now() + 15000;
  await refresh();
  const query = { ...router.currentRoute.value.query };
  delete query.teambition_auth;
  delete query.xiaoe_auth;
  delete query.connector_auth;
  delete query.linear_auth;
  await router.replace({ query });
}

function emptyMCPDraft(): MCPDraft { return { name: "", icon: "terminal", transport: "streamable_http", url: "", runner: "npx", package: "", package_version: "", argumentsText: "", environment: [], bearerToken: "" }; }
function emptyCLIDraft(): CLIDraft { return { name: "", icon: "terminal", description: "", installation_type: "npm", npm_install: "", archive: "" }; }
function notifyResources() { emit("resources", { mcp: mcp.value, skills: skills.value }); }
function upsertSkill(item: Skill) {
  if (!item.platform) createdSkillIDs.value = new Set(createdSkillIDs.value).add(item.id);
  skills.value = [item, ...skills.value.filter((entry) => entry.id !== item.id)];
  notifyResources();
}
function selectTab(value: ResourceTab) { activeTab.value = value; emit("tabChange", value); }
async function refresh() {
  try {
    [mcp.value, skills.value, cliDefinitions.value, cliEnablements.value, connectorPublications.value, connectorInstallations.value] = await Promise.all([api.listMCPServers(), api.listSkills(), api.listCLIConnectorDefinitions?.() ?? Promise.resolve([]), api.listCLIConnectorEnablements?.() ?? Promise.resolve([]), api.listConnectorPublications?.() ?? Promise.resolve([]), api.listConnectorInstallations?.() ?? Promise.resolve([])]);
    await Promise.all([refreshCLIAuthorizations(), refreshConnectorAuthorizations(), refreshSkillDocuments()]);
    notifyResources();
  } catch (cause) { reportError(cause); }
}

async function refreshConnectorAuthorizations() {
  const entries = await Promise.all(connectorInstallations.value.map(async (item) => [item.id, await api.listConnectorAuthorizations(item.id)] as const));
  connectorAuthorizations.value = Object.fromEntries(entries);
}
function connectorOperationBusy(source: string) { return connectorBusy.value.includes(source); }
function connectorNeedsScopeRecovery(item: ConnectorInstallation, publication?: ConnectorPublication) {
  const required = publication?.revision.required_scopes ?? [];
  if (!required.length) return false;
  const selected = (connectorAuthorizations.value[item.id] ?? []).find((authorization) => authorization.selected && authorization.state === "active");
  return !selected || required.some((scope) => !selected.scopes.includes(scope));
}
async function runConnectorOperation(source: string, action: () => Promise<unknown>) {
  if (connectorOperationBusy(source)) return;
  connectorBusy.value = [...connectorBusy.value, source];
  try { await action(); await refresh(); }
  catch (cause) { reportError(cause); }
  finally { connectorBusy.value = connectorBusy.value.filter((item) => item !== source); }
}
function installPublication(item: ConnectorPublication) { return runConnectorOperation(item.source, () => api.installPublishedConnector(item.source)); }
function upgradeInstallation(item: ConnectorInstallation) { return runConnectorOperation(item.source, () => api.upgradeConnectorInstallation(item.id, item.version)); }
function uninstallInstallation(item: ConnectorInstallation) { return runConnectorOperation(item.source, () => api.uninstallConnector(item.id, item.version)); }
function selectAuthorization(item: ConnectorInstallation, authorization: ConnectorAuthorization) { return runConnectorOperation(item.source, () => api.selectConnectorAuthorization(item.id, authorization.id, item.version)); }
function refreshAuthorization(item: ConnectorInstallation, authorization: ConnectorAuthorization) { return runConnectorOperation(item.source, () => api.refreshConnectorAuthorization(item.id, authorization.id, authorization.version)); }
function disconnectAuthorization(item: ConnectorInstallation, authorization: ConnectorAuthorization) { return runConnectorOperation(item.source, () => api.disconnectPublishedConnectorAuthorization(authorization.id)); }
function openProvidedConnection(installation: ConnectorInstallation) {
  providedConnection.value = { installation, botID: "", secret: "" };
}
function closeProvidedConnection() {
  if (providedConnectionBusy.value) return;
  providedConnection.value = undefined;
}
async function saveProvidedConnection() {
  const form = providedConnection.value;
  if (!form || providedConnectionBusy.value) return;
  const modao = form.installation.source === "modao";
  const openboost = form.installation.source === "openboost";
  const picset = form.installation.source === "picset-ai";
  const aiHive = form.installation.source === "ai-hive";
  const invalidKey = openboost ? "openboostCredentialsInvalid" : aiHive ? "aiHiveCredentialsInvalid" : picset ? "picsetCredentialsInvalid" : modao ? "modaoCredentialsInvalid" : "providedCredentialsInvalid";
  const botID = form.botID.trim();
  if (aiHive ? !form.secret || form.secret.length > 4096 || /[\s\x00-\x1f\x7f]/u.test(form.secret) : picset ? !/^sk_live_[A-Za-z0-9_-]+$/.test(form.secret) || form.secret.length > 4096 : (modao || openboost) ? !form.secret || form.secret.length > 32768 || /[\s\x00-\x1f\x7f]/u.test(form.secret) : !botID || !form.secret) {
    reportError(new ApiError("validation", 422, "invalid_input"), invalidKey);
    return;
  }
  providedConnectionBusy.value = true;
  try {
    const credentials = openboost ? { openboost_secret_key: form.secret } : aiHive ? { MCP_BEARER_TOKEN: form.secret } : picset ? { picset_api_key: form.secret } : modao ? { modao_token: form.secret } : { bot_id: botID, secret: form.secret };
    let installation = form.installation;
    if ((modao || picset || aiHive || openboost) && installation.upgrade_available) {
      installation = await api.upgradeConnectorInstallation(installation.id, installation.version);
      form.installation = installation;
    }
    await api.connectConnector(installation.id, "user", [], JSON.stringify(credentials));
    providedConnection.value = undefined;
    await refresh();
  } catch (cause) { reportError(cause, invalidKey); }
  finally { providedConnectionBusy.value = false; }
}
async function startBrowserConnection(installation: ConnectorInstallation, publication?: ConnectorPublication) {
  if (connectorOperationBusy(installation.source) || installation.state === "disabled" && !publication) return;
  const popup = window.open("about:blank", "_blank");
  connectorFlowWindows.set(installation.id, popup);
  connectorBusy.value = [...connectorBusy.value, installation.source];
  try {
    let active = installation;
    if (installation.state === "disabled") active = await api.installPublishedConnector(publication!.source);
    else if (installation.upgrade_available && publication) active = await api.upgradeConnectorInstallation(installation.id, installation.version);
    if (active !== installation) await refresh();
    if (!active.authorized || connectorNeedsScopeRecovery(active, publication)) await beginPublishedConnectorAuthorization(active, publication, popup);
    else { closeBlankCLIWindow(popup); connectorFlowWindows.delete(installation.id); }
  } catch (cause) { closeBlankCLIWindow(popup); connectorFlowWindows.delete(installation.id); reportError(cause, "connectorAuthorizationInvalidInput"); }
  finally { connectorBusy.value = connectorBusy.value.filter((source) => source !== installation.source); }
}
function connectorVerificationCode(actionURL?: string) {
  if (!actionURL) return "";
  try { return new URL(actionURL).searchParams.get("verificationCode") ?? new URL(actionURL).searchParams.get("user_code") ?? ""; }
  catch { return ""; }
}
async function setupPublishedConnector(item: ConnectorInstallation, publication?: ConnectorPublication) {
  const popup = window.open("about:blank", "_blank");
  connectorFlowWindows.set(item.id, popup);
  connectorBusy.value = [...connectorBusy.value, item.source];
  try {
    if (item.authentication_driver === "dingtalk") {
      await beginPublishedConnectorAuthorization(item, publication, popup);
      return;
    }
    const setup = await api.beginConnectorSetup(item.id);
    connectorSetups.value = { ...connectorSetups.value, [item.id]: setup };
    if (setup.state === "waiting_for_user" && setup.action_url) popup?.location.replace(setup.action_url);
    else await beginPublishedConnectorAuthorization(item, publication, popup);
  } catch (cause) { closeBlankCLIWindow(popup); reportError(cause, "connectorAuthorizationInvalidInput"); }
  finally { connectorBusy.value = connectorBusy.value.filter((source) => source !== item.source); }
}
async function beginPublishedConnectorAuthorization(item: ConnectorInstallation, publication?: ConnectorPublication, popup?: Window | null) {
  const scopes = publication?.revision.required_scopes ?? [];
  const flow = await api.beginConnectorAuthorizationFlow(item.id, "user", scopes);
  connectorAuthorizationFlows.value = { ...connectorAuthorizationFlows.value, [item.id]: flow };
  if (flow.state === "waiting_for_user" && flow.action_url) popup?.location.replace(flow.action_url);
}
async function completePublishedConnectorFlows() {
  if (publishedCompletionBusy) return;
  publishedCompletionBusy = true;
  lastCLICompletionPoll = Date.now();
  try {
    for (const [installationID, setup] of Object.entries(connectorSetups.value)) {
      if (setup.state !== "waiting_for_user") continue;
      try {
        const completed = await api.completeConnectorSetup(setup.id);
        connectorSetups.value = { ...connectorSetups.value, [installationID]: completed };
        if (completed.state === "completed") {
          const installation = connectorInstallations.value.find((item) => item.id === installationID);
          if (installation) await beginPublishedConnectorAuthorization(installation, connectorPublications.value.find((item) => item.source === installation.source), connectorFlowWindows.get(installationID));
        }
      } catch (cause) { reportError(cause); }
    }
    for (const [installationID, flow] of Object.entries(connectorAuthorizationFlows.value)) {
      if (flow.state !== "waiting_for_user") continue;
      try {
        const completed = await api.completeConnectorAuthorizationFlow(flow.id);
        reportedAuthorizationFlowErrors.delete(flow.id);
        connectorAuthorizationFlows.value = { ...connectorAuthorizationFlows.value, [installationID]: completed };
        if (completed.state !== "waiting_for_user") { closeBlankCLIWindow(connectorFlowWindows.get(installationID) ?? null); connectorFlowWindows.delete(installationID); await refresh(); }
      } catch (cause) {
        if (["teambition", "xiaoe", "kling-ai", "linear", "pixso", "tianyancha"].includes(connectorInstallations.value.find(item => item.id === installationID)?.source ?? "") && cause instanceof ApiError && cause.kind === "not_found") {
          await refresh();
          if (connectorInstallations.value.find(item => item.id === installationID)?.authorized) {
            connectorAuthorizationFlows.value = { ...connectorAuthorizationFlows.value, [installationID]: { ...flow, state: "completed" } };
          } else if (flow.expires_at && Date.parse(flow.expires_at) <= Date.now()) {
            connectorAuthorizationFlows.value = { ...connectorAuthorizationFlows.value, [installationID]: { ...flow, state: "invalid" } };
          }
          continue;
        }
        if (cause instanceof ApiError && ["dingtalk_cli_access_disabled", "dingtalk_cli_enterprise_denied", "dingtalk_cli_user_denied", "dingtalk_cli_channel_required", "dingtalk_cli_auth_expired", "dingtalk_identity_mismatch", "dingtalk_authorization_failed"].includes(cause.code)) {
          const remaining = { ...connectorAuthorizationFlows.value };
          delete remaining[installationID];
          connectorAuthorizationFlows.value = remaining;
          closeBlankCLIWindow(connectorFlowWindows.get(installationID) ?? null);
          connectorFlowWindows.delete(installationID);
        }
        if (!reportedAuthorizationFlowErrors.has(flow.id)) { reportedAuthorizationFlowErrors.add(flow.id); reportError(cause); }
      }
    }
  } finally { publishedCompletionBusy = false; }
}
async function refreshSkillDocuments() {
  let failed = false;
  const entries = await Promise.all(skills.value.map(async (item) => {
    try { return [item.id, (await api.getSkillDocument(item.id)).content] as const; }
    catch { failed = true; return [item.id, skillDocuments.value[item.id] ?? ""] as const; }
  }));
  skillDocuments.value = Object.fromEntries(entries);
  setStatusError("documents", failed);
}
function skillDescription(item: Skill) {
  const content = skillDocuments.value[item.id] ?? "";
  const metadata = parseSkillDocument(content);
  const description = localizedSkillDescription(metadata, locale.value);
  if (description) return description;
  const body = metadata.body;
  const paragraph = body.split(/\r?\n\s*\r?\n/).map((value) => value.replace(/^#+\s*/gm, "").replace(/[`*_>[\]()-]/g, "").trim()).find(Boolean);
  return paragraph || (item.source === "git" ? item.git_url : t("composer.localSkill")) || t("composer.localSkill");
}
function skillDisplayName(item: Skill) { return parseSkillDocument(skillDocuments.value[item.id] ?? "").displayName || item.name; }
function openSkillDetails(item: Skill) {
  if (props.selectable) detailSkill.value = item;
  else void router.push({ name: "skill-detail", params: { skillId: item.id }, state: { skillReturnTo: router.currentRoute.value.fullPath } });
}
async function enableCLI(item: CLIConnectorDefinition) {
  operationError.value = undefined;
  if (item.managed_installation || cliEnableBusy.value.includes(item.id)) return;
  cliEnableBusy.value.push(item.id);
  const popup = item.authentication_driver === "feishu" ? openCLIWindow() : null;
  if (item.authentication_driver === "feishu") cliSetupWindows.set(item.id, popup);
  try {
    const value = await api.enableCLIConnector(item.id);
    if (disposed) { closeBlankCLIWindow(popup); return; }
    cliEnablements.value = [...cliEnablements.value.filter((entry) => entry.definition_id !== item.id), value];
    if (value.state === "waiting_for_user") navigateCLIWindow(popup, value.action_url);
    else await continueCLISetup(item, value);
  }
  catch (cause) { cliSetupWindows.delete(item.id); closeBlankCLIWindow(popup); reportError(cause); }
  finally { cliEnableBusy.value = cliEnableBusy.value.filter((id) => id !== item.id); }
}

async function deactivateCLI(item: CLIConnectorDefinition) {
  operationError.value = undefined;
  const enablement = enablementFor(item.id);
  if (!enablement || cliEnableBusy.value.includes(item.id)) return;
  cliEnableBusy.value.push(item.id);
  try {
    const value = await api.disableCLIConnector(item.id, enablement.version);
    cliEnablements.value = [...cliEnablements.value.filter((entry) => entry.definition_id !== item.id), value];
    emit("update:cliConnectorDefinitionIds", props.cliConnectorDefinitionIds.filter((id) => id !== item.id));
  } catch (cause) { reportError(cause); }
  finally { cliEnableBusy.value = cliEnableBusy.value.filter((id) => id !== item.id); }
}

function openCLIWindow(): Window | null {
  // Reserve the tab during the click so async API responses do not trigger popup blocking.
  try {
    const popup = window.open("about:blank", "_blank");
    if (popup) popup.opener = null;
    return popup;
  } catch { return null; }
}
function navigateCLIWindow(popup: Window | null, url?: string) {
  if (!url || disposed) { closeBlankCLIWindow(popup); return; }
  try {
    if (popup && !popup.closed) popup.location.href = url;
  } catch { /* The visible link remains available if the browser detached the tab. */ }
}
function closeBlankCLIWindow(popup: Window | null) {
  try { if (popup && !popup.closed && popup.location.href === "about:blank") popup.close(); } catch { /* Keep external pages open. */ }
}
function resumeCLISetup(item: CLIConnectorDefinition, event?: MouseEvent) {
  const popup = openCLIWindow();
  if (popup) event?.preventDefault();
  cliSetupWindows.set(item.id, popup);
  navigateCLIWindow(popup, enablementFor(item.id)?.action_url);
}
async function continueCLISetup(item: CLIConnectorDefinition, enablement: CLIConnectorEnablement) {
  if (!cliSetupWindows.has(item.id) || enablement.state === "waiting_for_user") return;
  const popup = cliSetupWindows.get(item.id) ?? null;
  cliSetupWindows.delete(item.id);
  if (enablement.state === "enabled") await beginCLIAccountAuthorization(item, popup, userScopes(item));
  else closeBlankCLIWindow(popup);
}
async function removeCLI() {
  operationError.value = undefined;
  const item = deletingCLI.value;
  if (!item || cliDeleteBusy.value) return;
  cliDeleteBusy.value = true;
  try {
    await api.deleteCLIConnectorDefinition(item.id, item.version);
    cliDefinitions.value = cliDefinitions.value.filter((entry) => entry.id !== item.id);
    cliEnablements.value = cliEnablements.value.filter((entry) => entry.definition_id !== item.id);
    emit("update:cliConnectorDefinitionIds", props.cliConnectorDefinitionIds.filter((id) => id !== item.id));
    deletingCLI.value = undefined;
    if (detailCLI.value?.id === item.id) closeConnectorDetails();
  } catch (cause) { reportError(cause); }
  finally { cliDeleteBusy.value = false; }
}
async function completePendingCLIEnablements() {
  if (cliCompletionBusy) return;
  cliCompletionBusy = true;
  const pending = cliEnablements.value.filter((item) => item.state === "waiting_for_user");
  try {
    const completed = await Promise.all(pending.map((item) => api.completeCLIConnectorEnablement(item.id)));
    if (disposed) return;
    const replacements = new Map(completed.map((item) => [item.id, item]));
    cliEnablements.value = cliEnablements.value.map((item) => replacements.get(item.id) ?? item);
    setStatusError("enablements", false);
    for (const enablement of completed) {
      const definition = cliDefinitions.value.find((item) => item.id === enablement.definition_id);
      if (definition) await continueCLISetup(definition, enablement);
    }
  } catch { setStatusError("enablements", true); }
  finally { cliCompletionBusy = false; }
}
function enablementFor(id: string) { return cliEnablements.value.find((item) => item.definition_id === id); }
function cliInstalled(item: CLIConnectorDefinition) { return ["enabled", "waiting_for_user"].includes(enablementFor(item.id)?.state ?? ""); }
function authorizationsFor(definitionID: string) {
  const enablement = enablementFor(definitionID);
  return enablement ? cliAuthorizations.value[enablement.id] ?? [] : [];
}
function userScopes(item: CLIConnectorDefinition) {
  // Protobuf JSON omits empty repeated fields, including scopes on no-scope capabilities.
  return [...new Set((item.capabilities ?? []).filter((capability) => capability.identities?.includes("user")).flatMap((capability) => capability.scopes ?? []))];
}
function hasActiveCLIAuthorization(item: CLIConnectorDefinition) {
  return authorizationsFor(item.id).some((authorization) => authorization.state === "active");
}
function needsCLIReauthorization(item: CLIConnectorDefinition) {
  const required = userScopes(item);
  return required.length > 0 && !authorizationsFor(item.id).some((authorization) => authorization.state === "active" && required.every((scope) => (authorization.scopes ?? []).includes(scope)));
}
function cliNeedsActivation(item: CLIConnectorDefinition) {
  if (enablementFor(item.id)?.state !== "enabled") return true;
  return item.authentication_driver === "feishu" && (!hasActiveCLIAuthorization(item) || needsCLIReauthorization(item));
}
async function refreshCLIAuthorizations() {
  const enabled = cliEnablements.value.filter((item) => item.state === "enabled");
  const entries = await Promise.all(enabled.map(async (item) => [item.id, await (api.listCLIConnectorAuthorizations?.(item.id) ?? Promise.resolve([]))] as const));
  cliAuthorizations.value = Object.fromEntries(entries);
}
async function authorizeCLIAccount(item: CLIConnectorDefinition) {
  if (cliAuthorizationBusy.value.includes(item.id) || !enablementFor(item.id)) return;
  await beginCLIAccountAuthorization(item, openCLIWindow(), userScopes(item));
}
async function beginCLIAccountAuthorization(item: CLIConnectorDefinition, popup: Window | null, scopes: string[] = []) {
  operationError.value = undefined;
  const enablement = enablementFor(item.id);
  if (!enablement || disposed || cliAuthorizationBusy.value.includes(item.id)) { closeBlankCLIWindow(popup); return; }
  cliAuthorizationBusy.value.push(item.id);
  try {
    const flow = await api.beginCLIConnectorAuthorization(enablement.id, "user", scopes);
    if (disposed) { closeBlankCLIWindow(popup); return; }
    cliAuthorizationFlow.value = flow;
    navigateCLIWindow(popup, flow.action_url);
  } catch (cause) { closeBlankCLIWindow(popup); reportError(cause, "authorizationInvalidInput"); }
  finally { cliAuthorizationBusy.value = cliAuthorizationBusy.value.filter((id) => id !== item.id); }
}
async function completeCLIAccountAuthorization() {
  const flow = cliAuthorizationFlow.value;
  if (!flow || flow.state !== "waiting_for_user" || cliCompletionBusy) return;
  cliCompletionBusy = true;
  lastCLICompletionPoll = Date.now();
  try {
    const completed = await api.completeCLIConnectorAuthorization(flow.id);
    cliAuthorizationFlow.value = completed;
    if (completed.authorization) await refreshCLIAuthorizations();
    setStatusError("authorization", false);
  } catch { setStatusError("authorization", true); }
  finally { cliCompletionBusy = false; }
}
async function disconnectCLIAccount(item: CLIConnectorAuthorization) {
  operationError.value = undefined;
  try { await api.disconnectCLIConnectorAuthorization(item.id, item.version); await refreshCLIAuthorizations(); } catch (cause) { reportError(cause); }
}
function toggleCLI(item: CLIConnectorDefinition, checked: boolean) {
  if (item.managed_installation ? !item.managed_authorized : enablementFor(item.id)?.state !== "enabled") return;
  emit("update:cliConnectorDefinitionIds", checked ? [...new Set([...props.cliConnectorDefinitionIds, item.id])] : props.cliConnectorDefinitionIds.filter((id) => id !== item.id));
}
function openNewConnector() { showConnectorKind.value = true; }
function chooseConnectorKind(kind: "mcp" | "cli" | "package") {
  showConnectorKind.value = false;
  if (kind === "mcp") openNewMCP();
  else if (kind === "cli" && canManageCLI.value) openNewCLI();
  else if (kind === "package") connectorPackageInput.value?.click();
}
async function uploadConnectorPackage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  try { await api.uploadConnectorPackage(file); await refresh(); }
  catch (cause) { reportError(cause); }
  finally { input.value = ""; }
}
function openNewCLI() { editingCLI.value = undefined; cliForm.value = emptyCLIDraft(); showCLI.value = true; }
function openCLI(item: CLIConnectorDefinition) {
  editingCLI.value = item;
  cliForm.value = {
    name: item.name,
    icon: item.icon || "terminal",
    description: cliDescription(item),
    installation_type: item.installation_type || "npm",
    npm_install: item.installation_type === "upload" ? "" : `${item.npm_package}@${item.npm_version}`,
    archive: "",
  };
  showCLI.value = true;
}
function installedRecommendedSkill(recommendation: CLIRecommendedSkill) { return skills.value.find((skill) => skill.source === "git" && skill.git_url === recommendation.git_url && skill.git_ref === recommendation.git_ref); }
function cliDescription(item: CLIConnectorDefinition) { return item.description || (item.npm_package === "@larksuite/cli" ? t("resources.feishuCapability") : t("resources.noCapabilityDescription")); }
function selectedRecommendedSkill(recommendation: CLIRecommendedSkill) { const installed = installedRecommendedSkill(recommendation); return Boolean(installed && props.skillIds.includes(installed.id)); }
async function acceptRecommendedSkill(recommendation: CLIRecommendedSkill) {
  try {
    let skill = installedRecommendedSkill(recommendation);
    if (!skill) skill = await api.createGitSkill({ git_url: recommendation.git_url, git_ref: recommendation.git_ref });
    if (props.selectable) emit("update:skillIds", [...new Set([...props.skillIds, skill.id])]);
    await refresh();
  } catch (cause) { reportError(cause); }
}
function parseNPMInstall(value: string) {
  const match = value.trim().replace(/^npm\s+(?:install|i)(?:\s+-g)?\s+/, "").match(/^((?:@[a-z0-9][a-z0-9._-]*\/)?[a-z0-9][a-z0-9._-]*)@(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)$/);
  return match ? { npm_package: match[1]!, npm_version: match[2]! } : undefined;
}
async function selectCLIArchive(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  cliForm.value.archive = "";
  if (!file || file.size > 50 * 1024 * 1024) { if (file) reportError(new ApiError("validation", 413, "file_too_large")); return; }
  try { cliForm.value.archive = await fileToBase64(file); } catch (cause) { reportError(cause); }
}
async function saveCLI() {
  operationError.value = undefined;
  if (cliSaveBusy.value) return;
  cliSaveBusy.value = true;
  try {
    const npm = cliForm.value.installation_type === "npm" ? parseNPMInstall(cliForm.value.npm_install) : { npm_package: "", npm_version: "" };
    if (!npm) { reportError(new ApiError("validation", 422, "invalid_npm_install"), "invalidNPMInstall"); return; }
    const input: CLIConnectorDefinitionInput = { name: cliForm.value.name, icon: cliForm.value.icon, description: cliForm.value.description, installation_type: cliForm.value.installation_type, ...npm, archive: cliForm.value.installation_type === "upload" ? cliForm.value.archive : undefined };
    const saved = editingCLI.value ? await api.updateCLIConnectorDefinition(editingCLI.value.id, input, editingCLI.value.version) : await api.createCLIConnectorDefinition(input);
    editingCLI.value = saved;
    await api.publishCLIConnectorDefinition(saved.id, saved.version);
    showCLI.value = false;
    cliDefinitions.value = await api.listCLIConnectorDefinitions();
  } catch (cause) { reportError(cause); }
  finally { cliSaveBusy.value = false; }
}
async function disableCLI(item: CLIConnectorDefinition) {
  operationError.value = undefined;
  try { await api.disableCLIConnectorDefinition(item.id, item.version); cliDefinitions.value = await api.listCLIConnectorDefinitions(); cliEnablements.value = await api.listCLIConnectorEnablements(); }
  catch (cause) { reportError(cause); }
}
async function refreshCLI() { try { [cliDefinitions.value, cliEnablements.value] = await Promise.all([api.listCLIConnectorDefinitions(), api.listCLIConnectorEnablements()]); setStatusError("cli", false); } catch { setStatusError("cli", true); } }
async function refreshMCP() { try { mcp.value = await api.listMCPServers(); notifyResources(); setStatusError("mcp", false); } catch { setStatusError("mcp", true); } }
function openNewMCP() { editingMCP.value = undefined; mcpForm.value = emptyMCPDraft(); showMCP.value = true; }
function openMCP(item: MCPServer) {
  editingMCP.value = item;
  mcpForm.value = { name: item.name, icon: item.icon || "terminal", transport: item.transport, url: item.url ?? "", runner: item.runner ?? "npx", package: item.package ?? "", package_version: item.package_version ?? "", argumentsText: item.arguments.join("\n"), environment: item.environment.filter((entry) => entry.name !== "MCP_BEARER_TOKEN").map((entry) => ({ ...entry, value: "" })), bearerToken: "" };
  showMCP.value = true;
}
function showMCPDetails(item: MCPServer) { closeConnectorDetails(); detailMCP.value = item; }
function showCLIDetails(item: CLIConnectorDefinition) { closeConnectorDetails(); detailCLI.value = item; }
function editMCPFromDetails(item: MCPServer) { detailMCP.value = undefined; openMCP(item); }
function editCLIFromDetails(item: CLIConnectorDefinition) { detailCLI.value = undefined; openCLI(item); }
function addMCPEnvironment() { mcpForm.value.environment.push({ name: "", value: "", secret: false, configured: false }); }
function removeMCPEnvironment(index: number) { mcpForm.value.environment.splice(index, 1); }
function mcpPayload(): Record<string, unknown> {
  const argumentsList = mcpForm.value.argumentsText.split("\n").map((value) => value.trim()).filter(Boolean);
  const environment = mcpForm.value.environment.map((entry) => ({ ...entry, value: entry.value || undefined }));
  if (mcpForm.value.transport === "streamable_http") {
    const existingBearer = editingMCP.value?.environment.find((entry) => entry.name === "MCP_BEARER_TOKEN");
    if (mcpForm.value.bearerToken || existingBearer?.configured) environment.push({ name: "MCP_BEARER_TOKEN", value: mcpForm.value.bearerToken || undefined, secret: true, configured: Boolean(existingBearer?.configured) });
    return { name: mcpForm.value.name, icon: mcpForm.value.icon, transport: "streamable_http", url: mcpForm.value.url, arguments: [], environment };
  }
  return { name: mcpForm.value.name, icon: mcpForm.value.icon, transport: "stdio", runner: mcpForm.value.runner, package: mcpForm.value.package, package_version: mcpForm.value.package_version, arguments: argumentsList, environment };
}
async function saveMCP() {
  operationError.value = undefined;
  try {
    if (editingMCP.value) await api.updateMCPServer(editingMCP.value.id, mcpPayload(), editingMCP.value.version);
    else await api.createMCPServer(mcpPayload());
    showMCP.value = false;
    await refresh();
  } catch (cause) { reportError(cause); }
}
async function removeMCP(item: MCPServer) {
  await api.deleteMCPServer(item.id, pendingDelete.value?.impact.confirmation_token ?? "");
  emit("update:mcpServerIds", props.mcpServerIds.filter((id) => id !== item.id));
  await refresh();
}
async function testMCP(item: MCPServer) {
  try { const updated = await api.testMCPServer(item.id); mcp.value = mcp.value.map((entry) => entry.id === updated.id ? updated : entry); notifyResources(); } catch (cause) { reportError(cause); }
}
function toggleMCP(item: MCPServer, checked: boolean) {
  if (!item.tested) return;
  emit("update:mcpServerIds", checked ? [...new Set([...props.mcpServerIds, item.id])] : props.mcpServerIds.filter((id) => id !== item.id));
}
function openNewSkill() { editingSkill.value = undefined; skillForm.value = { source: "git", git_url: "", git_ref: "", archive: "", icon: "sparkles" }; showSkill.value = true; }
function openSkill(item: Skill) { editingSkill.value = item; skillForm.value = { source: item.source, git_url: item.git_url ?? "", git_ref: "", archive: "", icon: item.icon || "sparkles" }; showSkill.value = true; }
async function selectSkillArchive(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  skillForm.value.archive = "";
  if (!file || file.size > 50 * 1024 * 1024) { if (file) reportError(new ApiError("validation", 413, "file_too_large")); return; }
  try { skillForm.value.archive = await fileToBase64(file); } catch (cause) { reportError(cause); }
}
async function saveSkill() {
  operationError.value = undefined;
  try {
    let saved: Skill;
    if (editingSkill.value) {
      const input = editingSkill.value.source === "git" ? { git_ref: skillForm.value.git_ref.trim() || undefined, icon: skillForm.value.icon } : { archive: skillForm.value.archive, icon: skillForm.value.icon };
      saved = await api.updateSkill(editingSkill.value.id, input, editingSkill.value.version);
    } else if (skillForm.value.source === "git") saved = await api.createGitSkill({ git_url: skillForm.value.git_url, git_ref: skillForm.value.git_ref.trim() || undefined, icon: skillForm.value.icon });
    else saved = await api.createUploadSkill({ archive: skillForm.value.archive, icon: skillForm.value.icon });
    if (props.selectable && !editingSkill.value) emit("update:skillIds", [...new Set([...props.skillIds, saved.id])]);
    showSkill.value = false;
    await refresh();
    // Keep a successful creation visible even if the catalog read is briefly stale.
    upsertSkill(saved);
  } catch (cause) { reportError(cause); }
}
async function removeSkill(item: Skill) {
  await api.deleteSkill(item.id, pendingDelete.value?.impact.confirmation_token ?? "");
  emit("update:skillIds", props.skillIds.filter((id) => id !== item.id));
  await refresh();
}
function toggleSkill(item: Skill, checked: boolean) { emit("update:skillIds", checked ? [...new Set([...props.skillIds, item.id])] : props.skillIds.filter((id) => id !== item.id)); }
async function confirmRemove() {
  operationError.value = undefined;
  if (!pendingDelete.value || deleteBusy.value) return;
  const target = pendingDelete.value;
  deleteBusy.value = true;
  try {
    if (target.kind === "mcp") await removeMCP(target.item);
    else await removeSkill(target.item);
    pendingDelete.value = undefined;
    if (target.kind === "mcp" && detailMCP.value?.id === target.item.id) closeConnectorDetails();
  } catch (cause) { reportError(cause); }
  finally { deleteBusy.value = false; }
}
async function requestDelete(target: { kind: "mcp"; item: MCPServer } | { kind: "skill"; item: Skill }) {
  try {
    const impact = target.kind === "mcp" ? await api.getMCPConnectorDeletionImpact(target.item.id) : await api.getSkillDeletionImpact(target.item.id);
    pendingDelete.value = { ...target, impact: { ...impact, affected_experts: impact.affected_experts ?? [] } };
  } catch (cause) { reportError(cause); }
}
async function fileToBase64(file: File): Promise<string> {
  const bytes = new Uint8Array(await file.arrayBuffer()); let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  return btoa(binary);
}
</script>

<template>
  <div class="extension-manager" :class="{ selectable }">
    <el-alert v-if="statusErrors.length" :title="t('resources.statusUpdateFailed')" type="warning" :closable="false" data-testid="resource-status-error" />
    <nav v-if="showTabs" class="subtabs resource-tabs" :aria-label="t('resources.title')"><div class="resource-tabs-items"><el-button text :class="{ active: activeTab === 'skills' }" @click="selectTab('skills')">{{ t("resources.skills") }}</el-button><el-button text :class="{ active: activeTab === 'mcp' }" @click="selectTab('mcp')">{{ t("resources.connectors") }}</el-button></div><div class="resource-tabs-actions"><slot name="tab-actions" /></div></nav>
    <div v-if="activeTab === 'mcp'" class="extension-catalog-section">
      <div class="resource-toolbar extension-catalog-toolbar connector-catalog-toolbar">
        <nav v-if="!mineOnly || !selectable" class="subtabs connector-view-tabs" :aria-label="t('resources.connectors')"><el-button text :class="{ active: connectorView === 'market' }" :aria-pressed="connectorView === 'market'" @click="connectorView = 'market'">{{ t('resources.connectorMarket') }}</el-button><el-button text :class="{ active: connectorView === 'installed' }" :aria-pressed="connectorView === 'installed'" @click="connectorView = 'installed'">{{ t('resources.installed') }}</el-button></nav><slot name="catalog-actions" /><el-button type="primary" class="compact-action" @click="openNewConnector"><Plus />{{ t('resources.newConnector') }}</el-button></div>
      <div class="catalog-groups">
      <section v-for="section in connectorSections" :key="section.key" class="catalog-group">
      <h2 class="catalog-group-title">{{ section.title }}</h2>
      <div class="resource-list extension-catalog-grid connector-catalog-grid">
        <article v-for="entry in section.packages" :key="`package:${entry.publication?.source || entry.installation?.source}`" class="el-card catalog-activatable connector-catalog-card connector-summary-card published-connector-card" role="button" tabindex="0" :aria-label="entry.installation?.name || entry.publication?.revision.name" @click="showPackageDetails(entry)" @keydown.enter.self="showPackageDetails(entry)" @keydown.space.self.prevent="showPackageDetails(entry)">
          <ConnectorIcon class="connector-card-icon" :icon="entry.installation?.icon || entry.publication?.revision.icon || 'plug'" :size="48" />
          <div class="connector-summary-copy"><strong>{{ entry.installation?.name || entry.publication?.revision.name }}</strong><p>{{ entry.installation?.description || entry.publication?.revision.description }}</p></div>
          <Check v-if="entry.installation" class="connector-installed-mark" :size="22" :aria-label="t('resources.installed')" role="img" />
          <el-button v-else-if="entry.publication" class="connector-install-button" :aria-label="t('resources.install')" :loading="connectorOperationBusy(entry.publication.source)" :disabled="entry.publication.state !== 'available' || !entry.publication.revision.conformance_available" @click.stop="installPublication(entry.publication)"><Plus :size="20" /></el-button>
        </article>
        <article v-for="item in section.mcp" :key="`mcp:${item.id}`" class="el-card catalog-activatable connector-catalog-card connector-summary-card" role="button" tabindex="0" :aria-label="item.name" @click="showMCPDetails(item)" @keydown.enter.self="showMCPDetails(item)" @keydown.space.self.prevent="showMCPDetails(item)">
          <ConnectorIcon class="connector-card-icon" :icon="item.icon" :size="48" />
          <div class="connector-summary-copy"><strong>{{ item.name }}</strong><p>{{ t('resources.mcpDetailDescription') }}</p></div>
          <label v-if="selectable" class="extension-choice" @click.stop><el-checkbox :aria-label="item.name" :model-value="mcpServerIds.includes(item.id)" :disabled="!item.tested" @change="toggleMCP(item, Boolean($event))" /></label>
          <Check v-else class="connector-installed-mark" :size="22" :aria-label="t('resources.installed')" role="img" />
        </article>
        <article v-for="item in section.cli" :key="`cli:${item.id}`" class="el-card catalog-activatable connector-catalog-card connector-summary-card" role="button" tabindex="0" :aria-label="item.name" @click="showCLIDetails(item)" @keydown.enter.self="showCLIDetails(item)" @keydown.space.self.prevent="showCLIDetails(item)">
          <ConnectorIcon class="connector-card-icon" :icon="item.icon || 'terminal'" :size="48" />
          <div class="connector-summary-copy"><strong>{{ item.name }}</strong><p>{{ cliDescription(item) }}</p></div>
          <label v-if="selectable && cliInstalled(item)" class="extension-choice" @click.stop><el-checkbox :aria-label="item.name" :model-value="cliConnectorDefinitionIds.includes(item.id)" :disabled="enablementFor(item.id)?.state !== 'enabled'" @change="toggleCLI(item, Boolean($event))" /></label>
          <Check v-else-if="cliInstalled(item)" class="connector-installed-mark" :size="22" :aria-label="t('resources.installed')" role="img" />
          <el-button v-else class="connector-install-button" :aria-label="t('resources.enable')" :disabled="item.state !== 'available'" :loading="cliEnableBusy.includes(item.id)" @click.stop="enableCLI(item)"><Plus :size="20" /></el-button>
        </article>
        <div v-if="!section.packages.length && !section.mcp.length && !section.cli.length" class="empty-inline extension-empty"><span>◇</span><p>{{ t('common.empty') }}</p></div>
      </div>
      </section>
      <el-empty v-if="!connectorSections.length" class="catalog-empty" :description="t('resources.noMatchingResources')" />
      </div>
    </div>
    <div v-if="activeTab === 'skills'" class="extension-catalog-section">
      <div class="resource-toolbar extension-catalog-toolbar skill-add-actions"><slot name="catalog-actions" /><el-button type="primary" class="compact-action" @click="openNewSkill"><Plus />{{ t('resources.newSkill') }}</el-button><el-dropdown trigger="click" @command="handleSkillAction"><el-button type="primary" class="skill-add-menu-button"><ChevronDown :size="15" /></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="create">{{ t('resources.createSkill') }}</el-dropdown-item><el-dropdown-item command="upload">{{ t('resources.uploadSkill') }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown></div>
      <div class="catalog-groups">
      <section v-for="section in skillSections" :key="section.key" class="catalog-group">
      <h2 class="catalog-group-title">{{ section.title }}</h2>
      <div class="resource-list extension-catalog-grid skill-catalog-grid">
        <article v-for="item in section.items" :key="item.id" class="el-card catalog-activatable extension-catalog-card skill-catalog-card" role="button" tabindex="0" :aria-label="skillDisplayName(item)" @click="openSkillDetails(item)" @keydown.enter.self="openSkillDetails(item)" @keydown.space.self.prevent="openSkillDetails(item)">
          <div class="extension-card-copy skill-card-copy">
            <div class="skill-card-header">
              <div class="skill-card-heading"><ProfileIcon :icon="item.icon || 'sparkles'" /><strong>{{ skillDisplayName(item) }}</strong></div>
              <div class="extension-card-actions" @click.stop>
                <label v-if="selectable" class="extension-choice"><el-checkbox :model-value="skillIds.includes(item.id)" @change="toggleSkill(item, Boolean($event))" /></label>
                <el-button class="catalog-launch" circle type="primary" :aria-label="t('composer.useSkill')" :title="t('composer.useSkill')" @click="useSkill(item)"><Plus /></el-button>
                <el-button v-if="(!item.platform || canManageCLI) && !item.immutable" circle :aria-label="t('common.edit')" :title="t('common.edit')" @click="openSkill(item)"><Pencil /></el-button>
                <el-button v-if="(!item.platform || canManageCLI) && !item.immutable" circle type="danger" plain :aria-label="t('common.delete')" :title="t('common.delete')" @click="requestDelete({ kind: 'skill', item })"><Trash2 /></el-button>
              </div>
            </div>
            <p>{{ skillDescription(item) }}</p>
            <ResourceTrustMeta :source="item.platform ? t('resources.platformPublished') : t('resources.userPublished')" :permission="item.platform ? t('resources.allAuthenticated') : t('resources.ownerOnly')" :status="t('resources.packageValidated')" status-tone="success" />
          </div>
        </article>
        <div v-if="!section.items.length" class="empty-inline extension-empty"><span>◇</span><p>{{ t('common.empty') }}</p></div>
      </div>
      </section>
      <el-empty v-if="!skillSections.length" class="catalog-empty" :description="t('resources.noMatchingResources')" />
      </div>
    </div>
  </div>
  <ConnectorDetails :mcp="detailMCP" :cli="detailCLI" :installation="detailPackage?.installation" :publication="detailPackage?.publication" :can-connect="detailCanConnect" :can-uninstall="Boolean(detailPackage?.installation || detailCLI && cliInstalled(detailCLI))" :can-disconnect="Boolean(detailPackage?.installation && detailPackage.installation.authorized || detailCLI && hasActiveCLIAuthorization(detailCLI))" :connected-state="detailCLI ? enablementFor(detailCLI.id)?.state === 'enabled' && (detailCLI.authentication_driver === 'none' || hasActiveCLIAuthorization(detailCLI) && !cliNeedsActivation(detailCLI)) : undefined" :busy="detailBusy" :enablement="detailCLI ? enablementFor(detailCLI.id) : undefined" :can-edit="Boolean(detailMCP && ((!detailMCP.platform && !detailMCP.managed_installation) || canManageCLI) || detailCLI && canManageCLI && !detailCLI.managed_installation && detailCLI.mutable)" @close="closeConnectorDetails" @use="useConnectorPrompt" @connect="connectFromDetails" @disconnect="disconnectFromDetails" @uninstall="uninstallFromDetails" @edit-mcp="editMCPFromDetails" @edit-cli="editCLIFromDetails">
    <template #details>
      <div v-if="detailPackage" class="connector-supplementary-details"><template v-for="entry in [detailPackage]" :key="entry.installation?.id || entry.publication?.source">
            <ResourceTrustMeta :source="t('resources.platformPublished')" :permission="entry.installation ? t('resources.personalInstallation') : t('resources.allCanInstall')" :status="entry.publication?.revision.conformance_available ? t(entry.publication.revision.mode === 'mcp' ? 'resources.packageValidated' : 'resources.runtimeVerified') : t('resources.unverified')" :status-tone="entry.publication?.revision.conformance_available ? 'success' : 'warning'" :detail="entry.publication?.revision.runtime_digests?.length ? t('resources.runtimeDigestCount', { count: entry.publication.revision.runtime_digests.length }) : ''" />
            <small>{{ t('resources.packageVersion', { version: entry.publication?.revision.package_version || entry.installation?.package_version }) }} · {{ entry.publication?.revision.conformance_available ? t(entry.publication.revision.mode === 'mcp' ? 'resources.packageValidated' : 'resources.conformanceAvailable') : t('resources.conformanceUnavailable') }}</small>
            <small v-if="entry.installation && connectorSetups[entry.installation.id]?.provider_name">{{ connectorSetups[entry.installation.id].provider_name }}<template v-if="connectorSetups[entry.installation.id].developer_console_url"> · <a @click.stop :href="connectorSetups[entry.installation.id].developer_console_url" target="_blank" rel="noreferrer">{{ t('resources.developerConsole') }}</a></template></small>
            <small v-if="entry.installation && connectorAuthorizationFlows[entry.installation.id]?.state === 'waiting_for_user'">{{ t('resources.connectorAuthorizationPending') }} <span v-if="['notion', 'github'].includes(entry.installation.source) && connectorVerificationCode(connectorAuthorizationFlows[entry.installation.id].action_url)">{{ t(entry.installation.source === 'github' ? 'resources.githubVerifyCode' : 'resources.notionVerifyCode', { code: connectorVerificationCode(connectorAuthorizationFlows[entry.installation.id].action_url) }) }}</span> <a @click.stop v-if="connectorAuthorizationFlows[entry.installation.id].action_url" :href="connectorAuthorizationFlows[entry.installation.id].action_url" target="_blank" rel="noopener noreferrer">{{ t('resources.connectorAuthorizeNow') }}</a></small>
            <div v-if="entry.installation && connectorAuthorizations[entry.installation.id]?.length" class="connector-account-actions" @click.stop>
              <span v-for="authorization in connectorAuthorizations[entry.installation.id].filter((item) => item.state === 'active' || item.state === 'expired')" :key="authorization.id"><el-button text :type="authorization.selected ? 'primary' : 'default'" :disabled="authorization.state !== 'active'" @click="selectAuthorization(entry.installation!, authorization)">{{ authorization.external_display_name || authorization.external_identity_id || authorization.identity_ref }}{{ authorization.selected ? ` · ${t('resources.selectedAccount')}` : '' }}</el-button><el-button v-if="authorization.state === 'expired'" text type="primary" @click="refreshAuthorization(entry.installation!, authorization)">{{ t('resources.refreshAuthorization') }}</el-button><el-button text type="danger" @click="disconnectAuthorization(entry.installation!, authorization)">{{ t('resources.disconnectAccount') }}</el-button></span>
            </div>

      <el-button v-if="entry.installation?.upgrade_available" :disabled="detailBusy" @click="upgradeInstallation(entry.installation)">{{ t('resources.upgrade') }}</el-button>
      <el-button v-if="entry.installation?.authorized && connectorNeedsScopeRecovery(entry.installation, entry.publication)" :disabled="detailBusy" @click="connectFromDetails">{{ t('resources.expandAuthorization') }}</el-button>
      </template></div>
      <div v-if="detailCLI" class="connector-supplementary-details"><template v-for="item in [detailCLI]" :key="item.id">
            <ResourceTrustMeta :source="t('resources.platformPublished')" :permission="t('resources.allCanEnable')" :status="item.state === 'available' ? t('resources.runtimeVerified') : t(`resources.state.${item.state}`)" :status-tone="item.state === 'available' ? 'success' : item.state === 'failed' ? 'danger' : 'warning'" :detail="item.conformance_runtime_digests?.length ? t('resources.runtimeDigestCount', { count: item.conformance_runtime_digests.length }) : t('resources.noRuntimeEvidence')" />
            <small>{{ item.managed_installation ? `package · ${item.npm_package}@${item.npm_version}` : item.installation_type === 'upload' ? t('resources.zipUpload') : `npm · ${item.npm_package}@${item.npm_version}` }}</small>
            <small v-if="enablementFor(item.id)?.provider_name">{{ enablementFor(item.id)?.provider_name }}</small>
            <section v-if="item.recommended_skills?.length" class="recommended-skill-offers" @click.stop><span v-for="skill in item.recommended_skills" :key="`${skill.git_url}#${skill.git_ref}`" :class="{ warning: selectable && cliConnectorDefinitionIds.includes(item.id) && !selectedRecommendedSkill(skill) }"><small>{{ selectable && cliConnectorDefinitionIds.includes(item.id) && !selectedRecommendedSkill(skill) ? t('resources.recommendedSkillWarning', { name: skill.name }) : t('resources.recommendedSkillOffer', { name: skill.name }) }}</small><el-button v-if="!selectedRecommendedSkill(skill)" size="small" @click="acceptRecommendedSkill(skill)">{{ installedRecommendedSkill(skill) ? t('resources.selectSkill') : t('resources.installSkill') }}</el-button></span></section>
            <div v-if="!item.managed_installation" class="connector-account-actions" @click.stop>
              <a v-if="enablementFor(item.id)?.state === 'waiting_for_user'" :href="enablementFor(item.id)?.action_url" target="_blank" rel="noreferrer" @click="resumeCLISetup(item, $event)">{{ t('resources.continueSetup') }}</a>
              <template v-else-if="enablementFor(item.id)?.state === 'enabled'">
                <a v-if="enablementFor(item.id)?.developer_console_url" :href="enablementFor(item.id)?.developer_console_url" target="_blank" rel="noreferrer">{{ t('resources.developerConsole') }}</a>
                <template v-for="authorization in authorizationsFor(item.id)" :key="authorization.id"><span v-if="authorization.state === 'active'">{{ t('resources.authorizedAccount', { name: authorization.external_display_name }) }}</span><el-button v-if="authorization.state === 'active'" text type="danger" @click="disconnectCLIAccount(authorization)">{{ t('resources.disconnectAccount') }}</el-button></template>
                <template v-if="cliAuthorizationFlow?.enablement_id === enablementFor(item.id)?.id && cliAuthorizationFlow?.state === 'waiting_for_user'"><a :href="cliAuthorizationFlow?.action_url" target="_blank" rel="noreferrer">{{ t('resources.authorizeNow') }}</a><small>{{ t('resources.authorizationPending') }}</small></template>
                <el-button v-else-if="item.authentication_driver !== 'none' && (!hasActiveCLIAuthorization(item) || needsCLIReauthorization(item))" :loading="cliAuthorizationBusy.includes(item.id)" @click="authorizeCLIAccount(item)">{{ t(hasActiveCLIAuthorization(item) ? 'resources.expandAuthorization' : 'resources.authorizeAccount') }}</el-button>
              </template>
            </div>
              <div class="extension-card-actions" @click.stop>
                <el-button v-if="canManageCLI && !item.managed_installation && item.state === 'available'" type="danger" plain @click="disableCLI(item)">{{ t('resources.disable') }}</el-button>
                <el-button v-if="canManageCLI && !item.managed_installation" circle type="danger" plain :aria-label="t('common.delete')" :title="t('common.delete')" @click="deletingCLI = item"><Trash2 /></el-button>
              </div>
      </template></div>
      <template v-if="detailMCP"><ResourceTrustMeta :source="detailMCP.platform ? t('resources.platformPublished') : t('resources.userPublished')" :permission="detailMCP.platform ? t('resources.allAuthenticated') : t('resources.ownerOnly')" :status="detailMCP.tested ? t('resources.connectionTested') : t('resources.unverified')" :status-tone="detailMCP.tested ? 'success' : 'warning'" /><el-button v-if="(!detailMCP.platform && !detailMCP.managed_installation) || canManageCLI" type="danger" plain :aria-label="t('common.delete')" @click="requestDelete({ kind: 'mcp', item: detailMCP })">{{ t('common.delete') }}</el-button></template>
    </template>
  </ConnectorDetails>
  <CatalogDetails :skill="detailSkill" @close="detailSkill = undefined" @edit-skill="openSkill" />
  <Teleport to="body">
    <ToastMessage v-if="operationError" :key="operationError.zIndex" kind="error" :title="t('experts.operationFailed')" :message="operationError.message" :close-label="t('common.close')" :duration="0" :z-index="operationError.zIndex" @dismiss="operationError = undefined" />
    <div v-if="providedConnection" class="modal-layer" @click.self="closeProvidedConnection">
      <form class="modal-card provided-connector-form el-card" role="dialog" aria-modal="true" aria-labelledby="provided-connection-title" @keydown.esc.stop.prevent="closeProvidedConnection" @submit.prevent="saveProvidedConnection">
        <h2 id="provided-connection-title">{{ t('resources.connect') }} {{ providedConnection.installation.name }}</h2>
        <template v-if="providedConnection.installation.source === 'ai-hive'">
          <label>{{ t('resources.aiHiveApiKey') }}<input v-model="providedConnection.secret" name="ai_hive_api_key" type="password" autocomplete="new-password" maxlength="4096" required></label>
          <a href="https://ai-hive.iclip.cn/" target="_blank" rel="noopener noreferrer">{{ t('resources.aiHiveApiKeyHelp') }}</a>
        </template>
        <template v-else-if="providedConnection.installation.source === 'picset-ai'">
          <label>{{ t('resources.picsetApiKey') }}<input v-model="providedConnection.secret" name="picset_api_key" type="password" autocomplete="new-password" maxlength="4096" required></label>
          <a href="https://picsetai.cn/developer-api" target="_blank" rel="noopener noreferrer">{{ t('resources.picsetApiKeyHelp') }}</a>
        </template>
        <template v-else-if="providedConnection.installation.source === 'modao'">
          <label>{{ t('resources.modaoToken') }}<input v-model="providedConnection.secret" name="modao_token" type="password" autocomplete="new-password" maxlength="32768" required></label>
          <a href="https://modao.cc/feature/ai-mcp.html" target="_blank" rel="noopener noreferrer">{{ t('resources.modaoTokenHelp') }}</a>
        </template>
        <template v-else-if="providedConnection.installation.source === 'openboost'">
          <label>Secret Key<input v-model="providedConnection.secret" name="openboost_secret_key" type="password" autocomplete="new-password" maxlength="32768" required></label>
          <a href="https://open.microdata-inc.com/mcp-list" target="_blank" rel="noopener noreferrer">{{ t('resources.openboostTokenHelp') }}</a>
        </template>
        <template v-else>
          <label>{{ t('resources.wecomBotId') }}<input v-model="providedConnection.botID" name="bot_id" autocomplete="off" maxlength="512" required></label>
          <label>{{ t('resources.wecomSecret') }}<input v-model="providedConnection.secret" name="secret" type="password" autocomplete="new-password" maxlength="4096" required></label>
        </template>
        <div class="modal-actions"><el-button :disabled="providedConnectionBusy" @click="closeProvidedConnection">{{ t('common.cancel') }}</el-button><el-button native-type="submit" type="primary" :loading="providedConnectionBusy">{{ t('resources.connect') }}</el-button></div>
      </form>
    </div>
    <div v-if="showConnectorKind" class="modal-layer" @click.self="showConnectorKind = false"><section class="modal-card connector-kind-dialog el-card"><h2>{{ t('resources.chooseConnectorType') }}</h2><p class="muted">{{ t('resources.chooseConnectorTypeHint') }}</p><div class="connector-kind-options"><button type="button" data-testid="connector-kind-conversation" @click="createConnectorSession"><strong>{{ t('resources.createConnectorInConversation') }}</strong><span>{{ t('resources.createConnectorInConversationHint') }}</span></button><button type="button" data-testid="connector-kind-mcp" @click="chooseConnectorKind('mcp')"><strong>{{ t('resources.mcpConnector') }}</strong><span>{{ t('resources.mcpConnectorHint') }}</span></button><button type="button" data-testid="connector-kind-package" @click="chooseConnectorKind('package')"><strong>{{ t('resources.connectorPackageUpload') }}</strong><span>{{ t('resources.connectorPackageUploadHint') }}</span></button><button type="button" data-testid="connector-kind-cli" :disabled="!canManageCLI" @click="chooseConnectorKind('cli')"><strong>{{ t('resources.cliConnector') }}</strong><span>{{ canManageCLI ? t('resources.cliConnectorHint') : t('resources.administratorOnly') }}</span></button></div><div class="modal-actions"><el-button @click="showConnectorKind = false">{{ t('common.cancel') }}</el-button></div></section></div>
    <input ref="connectorPackageInput" type="file" accept=".zip,application/zip" hidden @change="uploadConnectorPackage">
    <div v-if="showMCP" class="modal-layer" @click.self="showMCP = false"><form class="modal-card el-card" @submit.prevent="saveMCP"><h2>{{ editingMCP ? t("common.edit") : t("common.new") }} MCP</h2><div class="form-field"><span>{{ t("resources.icon") }}</span><IconPicker v-model="mcpForm.icon" fallback="terminal" /></div><label>{{ t("common.name") }}<input v-model="mcpForm.name" required></label><label>{{ t("settings.transport") }}<select v-model="mcpForm.transport"><option value="streamable_http">Streamable HTTP</option><option value="stdio">stdio</option></select></label><template v-if="mcpForm.transport === 'streamable_http'"><label>URL<input v-model="mcpForm.url" type="url" required></label><label>{{ t("settings.bearerToken") }}<input v-model="mcpForm.bearerToken" type="password" :placeholder="editingMCP ? t('settings.keepSecret') : t('settings.optional')"></label></template><template v-else><label>Runner<select v-model="mcpForm.runner"><option value="npx">npx</option><option value="uvx">uvx</option></select></label><label>Package<input v-model="mcpForm.package" required></label><label>{{ t("settings.fixedVersion") }}<input v-model="mcpForm.package_version" required placeholder="1.2.3"></label><label>{{ t("settings.arguments") }}<textarea v-model="mcpForm.argumentsText" rows="4" :placeholder="t('settings.onePerLine')"></textarea></label></template><div><div v-for="(variable, index) in mcpForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><el-button text type="danger" @click="removeMCPEnvironment(index)">×</el-button></div><el-button @click="addMCPEnvironment">＋ {{ t("settings.environment") }}</el-button></div><div class="modal-actions"><el-button @click="showMCP = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ t("common.save") }}</el-button></div></form></div>
    <div v-if="showSkill" class="modal-layer" @click.self="showSkill = false"><form class="modal-card skill-import-card el-card" @submit.prevent="saveSkill"><h2>{{ editingSkill ? t('resources.updateSkill') : t('resources.importSkill') }}</h2><p v-if="editingSkill" class="skill-import-name">{{ editingSkill.name }}</p><div class="form-field"><span>{{ t('resources.icon') }}</span><IconPicker v-model="skillForm.icon" fallback="sparkles" /></div><label>{{ t("settings.source") }}<select v-model="skillForm.source" :disabled="Boolean(editingSkill)"><option value="git">{{ t('resources.gitAddress') }}</option><option value="upload">{{ t('resources.zipUpload') }}</option></select></label><template v-if="skillForm.source === 'git'"><label>{{ t('resources.gitAddress') }}<input v-model="skillForm.git_url" type="url" :disabled="Boolean(editingSkill)" required placeholder="https://github.com/owner/skill.git"></label><label>{{ t('resources.gitBranchOptional') }}<input v-model="skillForm.git_ref" :placeholder="t('resources.defaultBranchHint')"></label></template><label v-else class="skill-upload-field"><span>{{ t('resources.zipUpload') }}</span><input type="file" accept=".zip,application/zip" :required="Boolean(editingSkill) || !skillForm.archive" @change="selectSkillArchive"><small>{{ skillForm.archive ? t('resources.skillArchiveReady') : t('resources.chooseSkillArchive') }}</small></label><div class="modal-actions"><el-button @click="showSkill = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ editingSkill ? t('common.save') : t('resources.importSkill') }}</el-button></div></form></div>
    <div v-if="showCLI" class="modal-layer" @click.self="showCLI = false"><form class="modal-card cli-install-card el-card" @submit.prevent="saveCLI"><h2>{{ editingCLI ? t('resources.editCLI') : t('resources.installCLI') }}</h2><div class="form-field"><span>{{ t('resources.icon') }}</span><IconPicker v-model="cliForm.icon" fallback="terminal" /></div><label>{{ t('common.name') }}<input v-model="cliForm.name" maxlength="100" required></label><label>{{ t('resources.capabilityDescription') }}<textarea v-model="cliForm.description" rows="4" maxlength="2000" required></textarea></label><label>{{ t('resources.installationType') }}<select v-model="cliForm.installation_type"><option value="npm">{{ t('resources.npmInstall') }}</option><option value="upload">{{ t('resources.zipUpload') }}</option></select></label><label v-if="cliForm.installation_type === 'npm'">{{ t('resources.npmPackageSpec') }}<input v-model="cliForm.npm_install" required placeholder="@scope/package@1.2.3"><small>{{ t('resources.exactNPMHint') }}</small></label><label v-else>{{ t('resources.zipUpload') }}<input type="file" accept=".zip,application/zip" required @change="selectCLIArchive"><small>{{ t('resources.cliZipHint') }}</small></label><div class="modal-actions"><el-button @click="showCLI = false">{{ t('common.cancel') }}</el-button><el-button native-type="submit" type="primary" :loading="cliSaveBusy">{{ t('resources.install') }}</el-button></div></form></div>
  </Teleport>
  <ConfirmDialog :open="Boolean(pendingDelete)" :title="t('common.delete')" :message="pendingDelete ? pendingDelete.impact.affected_experts.length ? t('resources.deleteAffected', { resource: pendingDelete.item.name, experts: pendingDelete.impact.affected_experts.map((expert) => expert.name).join('、') }) : t('resources.deleteUnaffected', { resource: pendingDelete.item.name }) : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" :busy="deleteBusy" danger @cancel="pendingDelete = undefined" @confirm="confirmRemove" />
  <ConfirmDialog :open="Boolean(deletingCLI)" :title="t('common.delete')" :message="deletingCLI ? t('resources.deleteCLI', { resource: deletingCLI.name }) : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" :busy="cliDeleteBusy" danger @cancel="deletingCLI = undefined" @confirm="removeCLI" />
</template>
