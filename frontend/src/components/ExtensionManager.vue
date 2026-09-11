<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useZIndex } from "element-plus";
import ToastMessage from "./ToastMessage.vue";
import { Pencil, Plus, RefreshCw, Trash2 } from "@lucide/vue";
import CatalogDetails from "./CatalogDetails.vue";
import { ApiError, platformApiKey, type CLIConnectorAuthorization, type CLIConnectorAuthorizationFlow, type CLIConnectorDefinition, type CLIConnectorDefinitionInput, type CLIConnectorEnablement, type CLIRecommendedSkill, type EnvironmentVariable, type MCPServer, type ResourceDeletionImpact, type Skill } from "../api/client";
import { authContextKey } from "../auth/session";
import ConfirmDialog from "./ConfirmDialog.vue";
import { localizedSkillDescription, parseSkillDocument } from "../skillDocument";
import ConnectorDetails from "./ConnectorDetails.vue";
import ConnectorIcon from "./ConnectorIcon.vue";

type ResourceTab = "mcp" | "skills";
type MCPDraft = { name: string; icon: string; transport: "streamable_http" | "stdio"; url: string; runner: "npx" | "uvx"; package: string; package_version: string; argumentsText: string; environment: EnvironmentVariable[]; bearerToken: string };
type CLIDraft = { name: string; icon: string; description: string; installation_type: "npm" | "upload"; npm_install: string; archive: string };

const props = withDefaults(defineProps<{ selectable?: boolean; initialTab?: ResourceTab; mcpServerIds?: string[]; skillIds?: string[]; cliConnectorDefinitionIds?: string[] }>(), {
  selectable: false,
  initialTab: "mcp",
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
const auth = inject(authContextKey, undefined);
const { locale, t } = useI18n();
const { nextZIndex } = useZIndex();
const operationError = ref<{ message: string; zIndex: number }>();
const statusErrors = ref<string[]>([]);
function reportError(cause?: unknown, validationKey = "invalidInput") {
  const keys = { unauthenticated: "loginRequired", forbidden: "permissionDenied", not_found: "resourceMissing", conflict: "resourceChanged", validation: validationKey, rate_limited: "tooManyRequests", unavailable: "serviceUnavailable", unknown: "operationFailed" } as const;
  const key = cause instanceof ApiError ? cause.status === 413 ? "uploadTooLarge" : keys[cause.kind] : cause instanceof TypeError ? "networkFailed" : "operationFailed";
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
const cliDefinitions = ref<CLIConnectorDefinition[]>([]);
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
let disposed = false;
const showCLI = ref(false);
const showConnectorKind = ref(false);
const cliForm = ref<CLIDraft>(emptyCLIDraft());
const editingMCP = ref<MCPServer>();
const showMCP = ref(false);
const detailMCP = ref<MCPServer>();
const detailCLI = ref<CLIConnectorDefinition>();
const mcpForm = ref<MCPDraft>(emptyMCPDraft());
const editingSkill = ref<Skill>();
const showSkill = ref(false);
const skillForm = ref({ source: "git" as "git" | "upload", git_url: "", git_ref: "", archive: "" });
const pendingDelete = ref<({ kind: "mcp"; item: MCPServer } | { kind: "skill"; item: Skill }) & { impact: ResourceDeletionImpact }>();
const deleteBusy = ref(false);
let poll: number | undefined;
let lastCLICompletionPoll = 0;
const connectorSections = computed(() => [
  { key: "platform", title: t("resources.platformConnectors"), mcp: mcp.value.filter((item) => item.platform), cli: cliDefinitions.value },
  { key: "mine", title: t("resources.myConnectors"), mcp: mcp.value.filter((item) => !item.platform), cli: [] as CLIConnectorDefinition[] },
]);
const skillSections = computed(() => [
  { key: "platform", title: t("resources.platformSkills"), items: skills.value.filter((item) => item.platform) },
  { key: "mine", title: t("resources.mySkills"), items: skills.value.filter((item) => !item.platform) },
]);

onMounted(() => {
  void refresh();
  poll = window.setInterval(() => {
    if (mcp.value.some((item) => item.test_pending)) void refreshMCP();
    if (cliDefinitions.value.some((item) => item.state === "building" || item.state === "testing")) void refreshCLI();
    if (cliEnablements.value.some((item) => item.state === "waiting_for_user") && Date.now() - lastCLICompletionPoll >= 5000) {
      lastCLICompletionPoll = Date.now();
      void completePendingCLIEnablements();
    }
    if (cliAuthorizationFlow.value?.state === "waiting_for_user" && Date.now() - lastCLICompletionPoll >= 5000) void completeCLIAccountAuthorization();
  }, 1500);
});
onBeforeUnmount(() => {
  disposed = true;
  if (poll !== undefined) window.clearInterval(poll);
  for (const popup of cliSetupWindows.values()) closeBlankCLIWindow(popup);
  cliSetupWindows.clear();
});

function emptyMCPDraft(): MCPDraft { return { name: "", icon: "terminal", transport: "streamable_http", url: "", runner: "npx", package: "", package_version: "", argumentsText: "", environment: [], bearerToken: "" }; }
function emptyCLIDraft(): CLIDraft { return { name: "", icon: "terminal", description: "", installation_type: "npm", npm_install: "", archive: "" }; }
function notifyResources() { emit("resources", { mcp: mcp.value, skills: skills.value }); }
function selectTab(value: ResourceTab) { activeTab.value = value; emit("tabChange", value); }
async function refresh() {
  try {
    [mcp.value, skills.value, cliDefinitions.value, cliEnablements.value] = await Promise.all([api.listMCPServers(), api.listSkills(), api.listCLIConnectorDefinitions?.() ?? Promise.resolve([]), api.listCLIConnectorEnablements?.() ?? Promise.resolve([])]);
    await Promise.all([refreshCLIAuthorizations(), refreshSkillDocuments()]);
    notifyResources();
  } catch (cause) { reportError(cause); }
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
  if (canManageCLI.value || cliEnableBusy.value.includes(item.id)) return;
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
function resumeCLISetup(item: CLIConnectorDefinition, event: MouseEvent) {
  const popup = openCLIWindow();
  if (popup) event.preventDefault();
  cliSetupWindows.set(item.id, popup);
  navigateCLIWindow(popup, enablementFor(item.id)?.action_url);
}
async function continueCLISetup(item: CLIConnectorDefinition, enablement: CLIConnectorEnablement) {
  if (!cliSetupWindows.has(item.id) || enablement.state === "waiting_for_user") return;
  const popup = cliSetupWindows.get(item.id) ?? null;
  cliSetupWindows.delete(item.id);
  if (enablement.state === "enabled") await beginCLIAccountAuthorization(item, popup);
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
  if (enablementFor(item.id)?.state !== "enabled") return;
  emit("update:cliConnectorDefinitionIds", checked ? [...new Set([...props.cliConnectorDefinitionIds, item.id])] : props.cliConnectorDefinitionIds.filter((id) => id !== item.id));
}
function openNewConnector() { showConnectorKind.value = true; }
function chooseConnectorKind(kind: "mcp" | "cli") {
  showConnectorKind.value = false;
  if (kind === "mcp") openNewMCP();
  else if (canManageCLI.value) openNewCLI();
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
async function selectConnectorIcon(event: Event, target: "mcp" | "cli") {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  if (!/^image\/(png|jpeg|webp|gif)$/.test(file.type) || file.size > 384 * 1024) {
    reportError(new ApiError("validation", 422, "invalid_icon"));
    return;
  }
  try {
    const icon = `data:${file.type};base64,${await fileToBase64(file)}`;
    if (target === "mcp") mcpForm.value.icon = icon;
    else cliForm.value.icon = icon;
  } catch (cause) { reportError(cause); }
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
function showMCPDetails(item: MCPServer) { detailCLI.value = undefined; detailMCP.value = item; }
function showCLIDetails(item: CLIConnectorDefinition) { detailMCP.value = undefined; detailCLI.value = item; }
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
function openNewSkill() { editingSkill.value = undefined; skillForm.value = { source: "git", git_url: "", git_ref: "", archive: "" }; showSkill.value = true; }
function openSkill(item: Skill) { editingSkill.value = item; skillForm.value = { source: item.source, git_url: item.git_url ?? "", git_ref: "", archive: "" }; showSkill.value = true; }
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
      const input = editingSkill.value.source === "git" ? { git_ref: skillForm.value.git_ref.trim() || undefined } : { archive: skillForm.value.archive };
      saved = await api.updateSkill(editingSkill.value.id, input, editingSkill.value.version);
    } else if (skillForm.value.source === "git") saved = await api.createGitSkill({ git_url: skillForm.value.git_url, git_ref: skillForm.value.git_ref.trim() || undefined });
    else saved = await api.createUploadSkill({ archive: skillForm.value.archive });
    if (props.selectable && !editingSkill.value) emit("update:skillIds", [...new Set([...props.skillIds, saved.id])]);
    showSkill.value = false;
    await refresh();
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
    <nav class="subtabs" :aria-label="t('resources.title')"><el-button text :class="{ active: activeTab === 'skills' }" @click="selectTab('skills')">{{ t("resources.skills") }}</el-button><el-button text :class="{ active: activeTab === 'mcp' }" @click="selectTab('mcp')">{{ t("resources.connectors") }}</el-button></nav>
    <div v-if="activeTab === 'mcp'" class="extension-catalog-section">
      <div class="resource-toolbar"><el-button type="primary" class="compact-action" @click="openNewConnector"><Plus />{{ t('resources.newConnector') }}</el-button></div>
      <div class="catalog-groups">
      <section v-for="section in connectorSections" :key="section.key" class="catalog-group">
      <h2 class="catalog-group-title">{{ section.title }}</h2>
      <div class="resource-list extension-catalog-grid connector-catalog-grid">
        <article v-for="item in section.mcp" :key="`mcp:${item.id}`" class="el-card catalog-activatable extension-catalog-card connector-catalog-card" role="button" tabindex="0" :aria-label="item.name" @click="showMCPDetails(item)" @keydown.enter.self="showMCPDetails(item)" @keydown.space.self.prevent="showMCPDetails(item)">
          <ConnectorIcon class="connector-card-icon" :icon="item.icon" :size="42" />
          <div class="extension-card-copy">
            <div class="extension-card-title"><strong>{{ item.name }}</strong><el-tag :type="item.tested ? 'success' : 'warning'" size="small">{{ item.test_pending ? t("settings.testPending") : item.tested ? t("settings.tested") : t("settings.testRequired") }}</el-tag></div>
            <p>{{ item.url || `${item.runner} ${item.package}@${item.package_version}` }}<template v-if="item.test_error"> · {{ item.test_error }}</template></p>
          </div>
          <div class="extension-card-actions" @click.stop>
            <label v-if="selectable" class="extension-choice" :title="item.tested ? '' : t('experts.testRequired')"><el-checkbox :model-value="mcpServerIds.includes(item.id)" :disabled="!item.tested" @change="toggleMCP(item, Boolean($event))" /></label>
            <el-button v-if="!item.platform || canManageCLI" circle :aria-label="t('common.retry')" :title="t('common.retry')" :loading="item.test_pending" @click="testMCP(item)"><RefreshCw /></el-button>
            <el-button v-if="!item.platform || canManageCLI" circle :aria-label="t('common.edit')" :title="t('common.edit')" @click="openMCP(item)"><Pencil /></el-button>
            <el-button v-if="!item.platform || canManageCLI" circle type="danger" plain :aria-label="t('common.delete')" :title="t('common.delete')" @click="requestDelete({ kind: 'mcp', item })"><Trash2 /></el-button>
          </div>
        </article>
        <article v-for="item in section.cli" :key="`cli:${item.id}`" class="el-card catalog-activatable extension-catalog-card connector-catalog-card" role="button" tabindex="0" :aria-label="item.name" @click="showCLIDetails(item)" @keydown.enter.self="showCLIDetails(item)" @keydown.space.self.prevent="showCLIDetails(item)">
          <ConnectorIcon class="connector-card-icon" :icon="item.icon || 'terminal'" :size="42" />
          <div class="extension-card-copy">
            <div class="extension-card-title"><strong>{{ item.name }}</strong><el-tag size="small">{{ t(`resources.state.${item.state}`) }}</el-tag><el-tag v-if="enablementFor(item.id)?.state === 'enabled'" type="success" size="small">{{ t('common.enabled') }}</el-tag></div>
            <p>{{ cliDescription(item) }}<template v-if="item.failure_reason"> · {{ item.failure_reason }}</template></p>
            <small>{{ item.installation_type === 'upload' ? t('resources.zipUpload') : `npm · ${item.npm_package}@${item.npm_version}` }}</small>
            <small v-if="enablementFor(item.id)?.provider_name">{{ enablementFor(item.id)?.provider_name }}</small>
            <section v-if="item.recommended_skills?.length" class="recommended-skill-offers" @click.stop><span v-for="skill in item.recommended_skills" :key="`${skill.git_url}#${skill.git_ref}`" :class="{ warning: selectable && cliConnectorDefinitionIds.includes(item.id) && !selectedRecommendedSkill(skill) }"><small>{{ selectable && cliConnectorDefinitionIds.includes(item.id) && !selectedRecommendedSkill(skill) ? t('resources.recommendedSkillWarning', { name: skill.name }) : t('resources.recommendedSkillOffer', { name: skill.name }) }}</small><el-button v-if="!selectedRecommendedSkill(skill)" size="small" @click="acceptRecommendedSkill(skill)">{{ installedRecommendedSkill(skill) ? t('resources.selectSkill') : t('resources.installSkill') }}</el-button></span></section>
            <div v-if="!canManageCLI" class="connector-account-actions" @click.stop>
              <a v-if="enablementFor(item.id)?.state === 'waiting_for_user'" :href="enablementFor(item.id)?.action_url" target="_blank" rel="noreferrer" @click="resumeCLISetup(item, $event)">{{ t('resources.continueSetup') }}</a>
              <template v-else-if="enablementFor(item.id)?.state === 'enabled'">
                <a v-if="enablementFor(item.id)?.developer_console_url" :href="enablementFor(item.id)?.developer_console_url" target="_blank" rel="noreferrer">{{ t('resources.developerConsole') }}</a>
                <template v-for="authorization in authorizationsFor(item.id)" :key="authorization.id"><span v-if="authorization.state === 'active'">{{ t('resources.authorizedAccount', { name: authorization.external_display_name }) }}</span><el-button v-if="authorization.state === 'active'" text type="danger" @click="disconnectCLIAccount(authorization)">{{ t('resources.disconnectAccount') }}</el-button></template>
                <template v-if="cliAuthorizationFlow?.enablement_id === enablementFor(item.id)?.id && cliAuthorizationFlow?.state === 'waiting_for_user'"><a :href="cliAuthorizationFlow?.action_url" target="_blank" rel="noreferrer">{{ t('resources.authorizeNow') }}</a><small>{{ t('resources.authorizationPending') }}</small></template>
                <el-button v-else-if="!hasActiveCLIAuthorization(item) || needsCLIReauthorization(item)" :loading="cliAuthorizationBusy.includes(item.id)" @click="authorizeCLIAccount(item)">{{ t(hasActiveCLIAuthorization(item) ? 'resources.expandAuthorization' : 'resources.authorizeAccount') }}</el-button>
              </template>
              <el-button v-else-if="item.state === 'available'" :loading="cliEnableBusy.includes(item.id)" @click="enableCLI(item)">{{ t('resources.enable') }}</el-button>
            </div>
          </div>
          <div class="extension-card-actions" @click.stop>
            <label v-if="selectable" class="extension-choice"><el-checkbox :model-value="cliConnectorDefinitionIds.includes(item.id)" :disabled="enablementFor(item.id)?.state !== 'enabled'" @change="toggleCLI(item, Boolean($event))" /></label>
            <el-button v-if="canManageCLI && item.mutable" circle :aria-label="t('common.edit')" :title="t('common.edit')" @click="openCLI(item)"><Pencil /></el-button>
            <el-button v-if="canManageCLI && item.state === 'available'" type="danger" plain @click="disableCLI(item)">{{ t('resources.disable') }}</el-button>
            <el-button v-if="canManageCLI" circle type="danger" plain :aria-label="t('common.delete')" :title="t('common.delete')" @click="deletingCLI = item"><Trash2 /></el-button>
          </div>
        </article>
        <div v-if="!section.mcp.length && !section.cli.length" class="empty-inline extension-empty"><span>◇</span><p>{{ t('common.empty') }}</p></div>
      </div>
      </section>
      </div>
    </div>
    <div v-if="activeTab === 'skills'" class="extension-catalog-section">
      <div class="resource-toolbar"><el-button type="primary" class="compact-action" @click="openNewSkill"><Plus />{{ t('resources.newSkill') }}</el-button></div>
      <div class="catalog-groups">
      <section v-for="section in skillSections" :key="section.key" class="catalog-group">
      <h2 class="catalog-group-title">{{ section.title }}</h2>
      <div class="resource-list extension-catalog-grid skill-catalog-grid">
        <article v-for="item in section.items" :key="item.id" class="el-card catalog-activatable extension-catalog-card skill-catalog-card" role="button" tabindex="0" :aria-label="skillDisplayName(item)" @click="openSkillDetails(item)" @keydown.enter.self="openSkillDetails(item)" @keydown.space.self.prevent="openSkillDetails(item)">
          <span class="extension-card-mark skill-mark">{{ skillDisplayName(item).slice(0, 1).toUpperCase() }}</span>
          <div class="extension-card-copy skill-card-copy"><strong>{{ skillDisplayName(item) }}</strong><p>{{ skillDescription(item) }}</p><small>{{ item.source === 'git' ? item.git_url : t('composer.localSkill') }} · {{ t('composer.version', { version: item.version }) }}</small></div>
          <div class="extension-card-actions">
            <label v-if="selectable" class="extension-choice" @click.stop><el-checkbox :model-value="skillIds.includes(item.id)" @change="toggleSkill(item, Boolean($event))" /></label>
            <el-button class="catalog-launch" circle type="primary" :aria-label="t('composer.useSkill')" :title="t('composer.useSkill')" @click.stop="useSkill(item)"><Plus /></el-button>
            <el-button v-if="!item.platform || canManageCLI" circle :aria-label="t('common.edit')" :title="t('common.edit')" @click.stop="openSkill(item)"><Pencil /></el-button>
            <el-button v-if="!item.platform || canManageCLI" circle type="danger" plain :aria-label="t('common.delete')" :title="t('common.delete')" @click.stop="requestDelete({ kind: 'skill', item })"><Trash2 /></el-button>
          </div>
        </article>
        <div v-if="!section.items.length" class="empty-inline extension-empty"><span>◇</span><p>{{ t('common.empty') }}</p></div>
      </div>
      </section>
      </div>
    </div>
  </div>
  <ConnectorDetails :mcp="detailMCP" :cli="detailCLI" :enablement="detailCLI ? enablementFor(detailCLI.id) : undefined" :can-edit="Boolean(detailMCP && (!detailMCP.platform || canManageCLI) || detailCLI && canManageCLI && detailCLI.mutable)" @close="detailMCP = undefined; detailCLI = undefined" @edit-mcp="editMCPFromDetails" @edit-cli="editCLIFromDetails" />
  <CatalogDetails :skill="detailSkill" @close="detailSkill = undefined" @edit-skill="openSkill" />
  <Teleport to="body">
    <ToastMessage v-if="operationError" :key="operationError.zIndex" kind="error" :title="t('experts.operationFailed')" :message="operationError.message" :close-label="t('common.close')" :duration="0" :z-index="operationError.zIndex" @dismiss="operationError = undefined" />
    <div v-if="showConnectorKind" class="modal-layer" @click.self="showConnectorKind = false"><section class="modal-card connector-kind-dialog el-card"><h2>{{ t('resources.chooseConnectorType') }}</h2><p class="muted">{{ t('resources.chooseConnectorTypeHint') }}</p><div class="connector-kind-options"><button type="button" data-testid="connector-kind-mcp" @click="chooseConnectorKind('mcp')"><strong>{{ t('resources.mcpConnector') }}</strong><span>{{ t('resources.mcpConnectorHint') }}</span></button><button type="button" data-testid="connector-kind-cli" :disabled="!canManageCLI" @click="chooseConnectorKind('cli')"><strong>{{ t('resources.cliConnector') }}</strong><span>{{ canManageCLI ? t('resources.cliConnectorHint') : t('resources.administratorOnly') }}</span></button></div><div class="modal-actions"><el-button @click="showConnectorKind = false">{{ t('common.cancel') }}</el-button></div></section></div>
    <div v-if="showMCP" class="modal-layer" @click.self="showMCP = false"><form class="modal-card el-card" @submit.prevent="saveMCP"><h2>{{ editingMCP ? t("common.edit") : t("common.new") }} MCP</h2><label for="mcp-icon-upload">{{ t("resources.icon") }}<span class="cli-icon-field"><ConnectorIcon :icon="mcpForm.icon" /></span><small>{{ t('resources.iconUploadHint') }}</small></label><label>{{ t("common.name") }}<input v-model="mcpForm.name" required></label><label>{{ t("settings.transport") }}<select v-model="mcpForm.transport"><option value="streamable_http">Streamable HTTP</option><option value="stdio">stdio</option></select></label><template v-if="mcpForm.transport === 'streamable_http'"><label>URL<input v-model="mcpForm.url" type="url" required></label><label>{{ t("settings.bearerToken") }}<input v-model="mcpForm.bearerToken" type="password" :placeholder="editingMCP ? t('settings.keepSecret') : t('settings.optional')"></label></template><template v-else><label>Runner<select v-model="mcpForm.runner"><option value="npx">npx</option><option value="uvx">uvx</option></select></label><label>Package<input v-model="mcpForm.package" required></label><label>{{ t("settings.fixedVersion") }}<input v-model="mcpForm.package_version" required placeholder="1.2.3"></label><label>{{ t("settings.arguments") }}<textarea v-model="mcpForm.argumentsText" rows="4" :placeholder="t('settings.onePerLine')"></textarea></label></template><div><div v-for="(variable, index) in mcpForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><el-button text type="danger" @click="removeMCPEnvironment(index)">×</el-button></div><el-button @click="addMCPEnvironment">＋ {{ t("settings.environment") }}</el-button></div><div class="modal-actions"><el-button @click="showMCP = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ t("common.save") }}</el-button></div></form><input id="mcp-icon-upload" class="connector-icon-upload" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="selectConnectorIcon($event, 'mcp')"></div>
    <div v-if="showSkill" class="modal-layer" @click.self="showSkill = false"><form class="modal-card skill-import-card el-card" @submit.prevent="saveSkill"><h2>{{ editingSkill ? t('resources.updateSkill') : t('resources.importSkill') }}</h2><p v-if="editingSkill" class="skill-import-name">{{ editingSkill.name }}</p><label>{{ t("settings.source") }}<select v-model="skillForm.source" :disabled="Boolean(editingSkill)"><option value="git">{{ t('resources.gitAddress') }}</option><option value="upload">{{ t('resources.zipUpload') }}</option></select></label><template v-if="skillForm.source === 'git'"><label>{{ t('resources.gitAddress') }}<input v-model="skillForm.git_url" type="url" :disabled="Boolean(editingSkill)" required placeholder="https://github.com/owner/skill.git"></label><label>{{ t('resources.gitBranchOptional') }}<input v-model="skillForm.git_ref" :placeholder="t('resources.defaultBranchHint')"></label></template><label v-else class="skill-upload-field"><span>{{ t('resources.zipUpload') }}</span><input type="file" accept=".zip,application/zip" :required="Boolean(editingSkill) || !skillForm.archive" @change="selectSkillArchive"><small>{{ skillForm.archive ? t('resources.skillArchiveReady') : t('resources.chooseSkillArchive') }}</small></label><p class="muted">{{ t('resources.skillDisplayNameHint') }}</p><p class="muted">{{ t("settings.skillHint") }}</p><div class="modal-actions"><el-button @click="showSkill = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ editingSkill ? t('common.save') : t('resources.importSkill') }}</el-button></div></form></div>
    <div v-if="showCLI" class="modal-layer" @click.self="showCLI = false"><form class="modal-card cli-install-card el-card" @submit.prevent="saveCLI"><h2>{{ editingCLI ? t('resources.editCLI') : t('resources.installCLI') }}</h2><label for="cli-icon-upload">{{ t('resources.icon') }}<span class="cli-icon-field"><ConnectorIcon :icon="cliForm.icon" /><select v-model="cliForm.icon"><option value="terminal">⌘ Terminal</option><option value="code">‹› Code</option><option value="sparkles">✦ Sparkles</option><option value="compass">⌖ Compass</option></select></span><small>{{ t('resources.iconUploadHint') }}</small></label><label>{{ t('common.name') }}<input v-model="cliForm.name" maxlength="100" required></label><label>{{ t('resources.capabilityDescription') }}<textarea v-model="cliForm.description" rows="4" maxlength="2000" required></textarea></label><label>{{ t('resources.installationType') }}<select v-model="cliForm.installation_type"><option value="npm">{{ t('resources.npmInstall') }}</option><option value="upload">{{ t('resources.zipUpload') }}</option></select></label><label v-if="cliForm.installation_type === 'npm'">{{ t('resources.npmPackageSpec') }}<input v-model="cliForm.npm_install" required placeholder="@scope/package@1.2.3"><small>{{ t('resources.exactNPMHint') }}</small></label><label v-else>{{ t('resources.zipUpload') }}<input type="file" accept=".zip,application/zip" required @change="selectCLIArchive"><small>{{ t('resources.cliZipHint') }}</small></label><div class="modal-actions"><el-button @click="showCLI = false">{{ t('common.cancel') }}</el-button><el-button native-type="submit" type="primary" :loading="cliSaveBusy">{{ t('resources.install') }}</el-button></div></form><input id="cli-icon-upload" class="connector-icon-upload" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="selectConnectorIcon($event, 'cli')"></div>
  </Teleport>
  <ConfirmDialog :open="Boolean(pendingDelete)" :title="t('common.delete')" :message="pendingDelete ? pendingDelete.impact.affected_experts.length ? t('resources.deleteAffected', { resource: pendingDelete.item.name, experts: pendingDelete.impact.affected_experts.map((expert) => expert.name).join('、') }) : t('resources.deleteUnaffected', { resource: pendingDelete.item.name }) : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" :busy="deleteBusy" danger @cancel="pendingDelete = undefined" @confirm="confirmRemove" />
  <ConfirmDialog :open="Boolean(deletingCLI)" :title="t('common.delete')" :message="deletingCLI ? t('resources.deleteCLI', { resource: deletingCLI.name }) : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" :busy="cliDeleteBusy" danger @cancel="deletingCLI = undefined" @confirm="removeCLI" />
</template>
