<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type EnvironmentVariable, type MCPServer, type ModelProviderConnection, type ModelProviderPreset, type PersonalSettings, type ProviderModel, type RuntimeEngine, type RuntimeEngineStatus, type Skill } from "../api/client";

type Section = "personal" | "models" | "extensions";
type Extension = "mcp" | "skills" | "cli";
type MCPDraft = { name: string; transport: "streamable_http" | "stdio"; url: string; runner: "npx" | "uvx"; package: string; package_version: string; argumentsText: string; environment: EnvironmentVariable[]; bearerToken: string };

const api = inject(platformApiKey)!;
const { t, locale } = useI18n();
const settings = ref<PersonalSettings>();
const connections = ref<ModelProviderConnection[]>([]);
const presets = ref<ModelProviderPreset[]>([]);
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const runtimes = ref<RuntimeEngineStatus[]>([]);
const section = ref<Section>("personal");
const extension = ref<Extension>("mcp");
const error = ref("");
const notice = ref("");
const editingConnection = ref<ModelProviderConnection>();
const showConnection = ref(false);
const connectionForm = ref({ name: "", provider_type: "openai", endpoint: "", protocols: [] as string[], api_key: "" });
const manualConnection = ref<ModelProviderConnection>();
const manualModel = ref({ model_id: "", display_name: "", model_type: "unknown" as ProviderModel["model_type"] });
const editingMCP = ref<MCPServer>();
const showMCP = ref(false);
const mcpForm = ref<MCPDraft>(emptyMCPDraft());
const editingSkill = ref<Skill>();
const showSkill = ref(false);
const skillForm = ref({ name: "", source: "git" as "git" | "upload", git_url: "", git_ref: "main", archive: "" });
let poll: number | undefined;

onMounted(() => {
  void refresh();
  poll = window.setInterval(() => { if (mcp.value.some((item) => item.test_pending)) void refreshMCP(); }, 1500);
});
onBeforeUnmount(() => { if (poll !== undefined) window.clearInterval(poll); });

function emptyMCPDraft(): MCPDraft { return { name: "", transport: "streamable_http", url: "", runner: "npx", package: "", package_version: "", argumentsText: "", environment: [], bearerToken: "" }; }
function clearFeedback() { error.value = ""; notice.value = ""; }
function showError(kind: "generic" | "validation" | "conflict" = "generic") { error.value = t(`errors.${kind}`); }
async function refresh() {
  clearFeedback();
  try { [settings.value, connections.value, presets.value, mcp.value, skills.value, runtimes.value] = await Promise.all([api.getSettings(), api.listModelProviderConnections(), api.listModelProviderPresets(), api.listMCPServers(), api.listSkills(), api.listRuntimeEngines()]); } catch { showError(); }
}
async function refreshMCP() { try { mcp.value = await api.listMCPServers(); } catch { /* Preserve the last usable projection while polling. */ } }
async function saveSettings() {
  if (!settings.value) return;
  clearFeedback();
  try {
    settings.value = await api.updateSettings(settings.value);
    locale.value = settings.value.language;
    document.documentElement.lang = settings.value.language;
    localStorage.setItem("agent-workspace-locale", settings.value.language);
    notice.value = t("settings.saved");
  } catch { showError("conflict"); }
}

const selectableModels = computed(() => connections.value.flatMap((connection) => connection.models.filter((model) => model.available && ["agent", "text", "unknown"].includes(model.model_type)).map((model) => ({ ...model, connection }))));
function choosePreset(providerType: string) { const preset = presets.value.find((item) => item.provider_type === providerType); if (!preset || editingConnection.value) return; connectionForm.value.endpoint = preset.official_endpoint; connectionForm.value.protocols = [...preset.protocols]; if (!connectionForm.value.name) connectionForm.value.name = preset.display_name; }
function openNewConnection() { editingConnection.value = undefined; const preset = presets.value[0]; connectionForm.value = { name: preset?.display_name ?? "", provider_type: preset?.provider_type ?? "openai", endpoint: preset?.official_endpoint ?? "", protocols: [...(preset?.protocols ?? [])], api_key: "" }; showConnection.value = true; }
function openConnection(item: ModelProviderConnection) { editingConnection.value = item; connectionForm.value = { name: item.name, provider_type: item.provider_type, endpoint: item.endpoint, protocols: [...item.protocols], api_key: "" }; showConnection.value = true; }
async function saveConnection() {
  clearFeedback();
  try {
    if (editingConnection.value) await api.updateModelProviderConnection(editingConnection.value.id, connectionForm.value, editingConnection.value.version);
    else await api.createModelProviderConnection(connectionForm.value);
    showConnection.value = false; await refresh();
  } catch { showError("validation"); }
}
async function removeConnection(item: ModelProviderConnection) { if (!confirm(`${t("common.delete")} ${item.name}?`)) return; try { await api.deleteModelProviderConnection(item.id); await refresh(); } catch { showError("conflict"); } }
async function refreshModels(item: ModelProviderConnection) { try { await api.refreshProviderModels(item.id); await refresh(); } catch { showError(); } }
function openManualModel(item: ModelProviderConnection) { manualConnection.value = item; manualModel.value = { model_id: "", display_name: "", model_type: "unknown" }; }
async function saveManualModel() { if (!manualConnection.value) return; try { await api.createProviderModel(manualConnection.value.id, manualModel.value); manualConnection.value = undefined; await refresh(); } catch { showError("validation"); } }
function runtimeDefault(runtime: RuntimeEngine) { return settings.value?.runtime_model_defaults.find((item) => item.runtime_engine === runtime)?.provider_model_id ?? ""; }
function setRuntimeDefault(runtime: RuntimeEngine, modelID: string) { if (!settings.value) return; settings.value.runtime_model_defaults = settings.value.runtime_model_defaults.filter((item) => item.runtime_engine !== runtime); if (modelID) settings.value.runtime_model_defaults.push({ runtime_engine: runtime, provider_model_id: modelID }); }
function setRuntimeDefaultFromEvent(runtime: RuntimeEngine, event: Event) { setRuntimeDefault(runtime, (event.target as HTMLSelectElement).value); }

function openNewMCP() { editingMCP.value = undefined; mcpForm.value = emptyMCPDraft(); showMCP.value = true; }
function openMCP(item: MCPServer) {
  editingMCP.value = item;
  mcpForm.value = { name: item.name, transport: item.transport, url: item.url ?? "", runner: item.runner ?? "npx", package: item.package ?? "", package_version: item.package_version ?? "", argumentsText: item.arguments.join("\n"), environment: item.environment.filter((entry) => entry.name !== "MCP_BEARER_TOKEN").map((entry) => ({ ...entry, value: "" })), bearerToken: "" };
  showMCP.value = true;
}
function addMCPEnvironment() { mcpForm.value.environment.push({ name: "", value: "", secret: false, configured: false }); }
function removeMCPEnvironment(index: number) { mcpForm.value.environment.splice(index, 1); }
function mcpPayload(): Record<string, unknown> {
  const argumentsList = mcpForm.value.argumentsText.split("\n").map((value) => value.trim()).filter(Boolean);
  const environment = mcpForm.value.environment.map((entry) => ({ ...entry, value: entry.value || undefined }));
  if (mcpForm.value.transport === "streamable_http") {
    const existingBearer = editingMCP.value?.environment.find((entry) => entry.name === "MCP_BEARER_TOKEN");
    if (mcpForm.value.bearerToken || existingBearer?.configured) environment.push({ name: "MCP_BEARER_TOKEN", value: mcpForm.value.bearerToken || undefined, secret: true, configured: Boolean(existingBearer?.configured) });
    return { name: mcpForm.value.name, transport: "streamable_http", url: mcpForm.value.url, arguments: [], environment };
  }
  return { name: mcpForm.value.name, transport: "stdio", runner: mcpForm.value.runner, package: mcpForm.value.package, package_version: mcpForm.value.package_version, arguments: argumentsList, environment };
}
async function saveMCP() {
  clearFeedback();
  try {
    if (editingMCP.value) await api.updateMCPServer(editingMCP.value.id, mcpPayload(), editingMCP.value.version);
    else await api.createMCPServer(mcpPayload());
    showMCP.value = false; await refresh();
  } catch { showError("validation"); }
}
async function removeMCP(item: MCPServer) { if (!confirm(`${t("common.delete")} ${item.name}?`)) return; try { await api.deleteMCPServer(item.id); await refresh(); } catch { showError(); } }
async function testMCP(item: MCPServer) { try { const updated = await api.testMCPServer(item.id); mcp.value = mcp.value.map((entry) => entry.id === updated.id ? updated : entry); } catch { showError(); } }

function openNewSkill() { editingSkill.value = undefined; skillForm.value = { name: "", source: "git", git_url: "", git_ref: "main", archive: "" }; showSkill.value = true; }
function openSkill(item: Skill) { editingSkill.value = item; skillForm.value = { name: item.name, source: item.source, git_url: item.git_url ?? "", git_ref: item.git_ref ?? "main", archive: "" }; showSkill.value = true; }
async function selectSkillArchive(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  if (file.size > 10 * 1024 * 1024) { showError("validation"); return; }
  skillForm.value.archive = await fileToBase64(file);
}
async function saveSkill() {
  clearFeedback();
  try {
    if (editingSkill.value) {
      const input = editingSkill.value.source === "git" ? { git_ref: skillForm.value.git_ref } : { archive: skillForm.value.archive };
      await api.updateSkill(editingSkill.value.id, input, editingSkill.value.version);
    } else if (skillForm.value.source === "git") await api.createGitSkill({ name: skillForm.value.name, git_url: skillForm.value.git_url, git_ref: skillForm.value.git_ref });
    else await api.createUploadSkill({ name: skillForm.value.name, archive: skillForm.value.archive });
    showSkill.value = false; await refresh();
  } catch { showError("validation"); }
}
async function removeSkill(item: Skill) { if (!confirm(`${t("common.delete")} ${item.name}?`)) return; try { await api.deleteSkill(item.id); await refresh(); } catch { showError(); } }
async function fileToBase64(file: File): Promise<string> {
  const bytes = new Uint8Array(await file.arrayBuffer()); let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  return btoa(binary);
}
</script>

<template>
  <section class="page-surface settings-page">
    <header class="page-header"><div><p class="eyebrow">04 / PERSONAL DEFAULTS</p><h1>{{ t("settings.title") }}</h1><p>{{ t("settings.subtitle") }}</p></div></header>
    <div v-if="error" class="notice error-notice">{{ error }}</div><div v-if="notice" class="notice success-notice">{{ notice }}</div>
    <div class="settings-layout">
      <nav class="settings-nav">
        <button :class="{ active: section === 'personal' }" @click="section = 'personal'"><span>01</span>{{ t("settings.personality") }}</button>
        <button :class="{ active: section === 'models' }" @click="section = 'models'"><span>02</span>{{ t("settings.model") }}</button>
        <button :class="{ active: section === 'extensions' }" @click="section = 'extensions'"><span>03</span>{{ t("settings.extensions") }}</button>
      </nav>
      <div class="settings-canvas">
        <form v-if="section === 'personal' && settings" @submit.prevent="saveSettings">
          <div class="section-heading"><div><p class="eyebrow">BEHAVIOR / PERSONAL</p><h2>{{ t("settings.personality") }}</h2></div><button class="button primary">{{ t("common.save") }}</button></div>
          <div class="personality-grid"><label v-for="item in ['gentle_professional','direct_efficient','lively_friendly','custom']" :key="item" :class="{ selected: settings.personality === item }"><input v-model="settings.personality" type="radio" :value="item"><span>{{ item === 'gentle_professional' ? '◡' : item === 'direct_efficient' ? '→' : item === 'lively_friendly' ? '✦' : '⌁' }}</span><strong>{{ t(`settings.${item === 'gentle_professional' ? 'gentle' : item === 'direct_efficient' ? 'direct' : item === 'lively_friendly' ? 'lively' : 'custom'}`) }}</strong></label></div>
          <label class="block-label">{{ t("settings.instructions") }}<textarea v-model="settings.personality_instructions" rows="6" :required="settings.personality === 'custom'"></textarea></label>
          <div class="form-grid">
            <label>{{ t("settings.runtime") }}<select v-model="settings.default_runtime_engine"><option v-for="runtime in runtimes" :key="runtime.name" :value="runtime.name" :disabled="!runtime.available">{{ runtime.name === 'claude' ? 'Claude Code' : runtime.name === 'openclaw' ? 'OpenClaw' : runtime.name[0].toUpperCase() + runtime.name.slice(1) }} · {{ runtime.available ? t("settings.available") : t("settings.unavailable") }}</option></select></label>
            <label>{{ t("settings.language") }}<select v-model="settings.language"><option value="zh-CN">中文</option><option value="en-US">English</option></select></label><label>{{ t("settings.timezone") }}<input v-model="settings.timezone"></label>
            <fieldset class="full runtime-defaults"><legend>{{ t("settings.runtimeModels") }}</legend><label v-for="runtime in runtimes" :key="runtime.name"><span>{{ runtime.name === 'claude' ? 'Claude Code' : runtime.name === 'openclaw' ? 'OpenClaw' : runtime.name[0].toUpperCase() + runtime.name.slice(1) }}</span><select :value="runtimeDefault(runtime.name)" @change="setRuntimeDefaultFromEvent(runtime.name, $event)"><option value="">—</option><option v-for="item in selectableModels" :key="item.id" :value="item.id" :disabled="item.compatibility.find((compatibility) => compatibility.runtime_engine === runtime.name)?.status === 'incompatible'">{{ item.connection.name }} / {{ item.display_name }}</option></select></label></fieldset>
          </div>
        </form>
        <div v-if="section === 'models'">
          <div class="section-heading"><div><p class="eyebrow">MODEL PROVIDERS / WRITE-ONLY API KEYS</p><h2>{{ t("settings.providers") }}</h2></div><button class="button primary" @click="openNewConnection">＋ {{ t("settings.addProvider") }}</button></div>
          <div class="provider-grid"><article v-for="item in connections" :key="item.id" class="provider-card"><header><span class="resource-mark">{{ item.name.slice(0, 2).toUpperCase() }}</span><div><strong>{{ item.name }}</strong><p>{{ presets.find((preset) => preset.provider_type === item.provider_type)?.display_name ?? item.provider_type }}</p></div><span class="safe-chip" :class="{ unsafe: item.verification_status !== 'verified' }">{{ t(`settings.${item.verification_status}`) }}</span></header><p class="provider-endpoint">{{ item.endpoint }}</p><p v-if="item.last_sync_error || item.verification_error" class="provider-error">{{ item.last_sync_error || item.verification_error }}</p><div class="provider-model-summary"><strong>{{ item.models.filter((model) => model.available).length }}</strong><span>{{ t("settings.importedModels") }}</span><small v-if="item.last_synced_at">{{ new Date(item.last_synced_at).toLocaleString() }}</small></div><div class="provider-actions"><button class="button ghost" @click="refreshModels(item)">↻ {{ t("settings.refreshModels") }}</button><button class="button ghost" @click="openManualModel(item)">＋ {{ t("settings.manualModel") }}</button><button :aria-label="t('common.edit')" @click="openConnection(item)">✎</button><button :aria-label="t('common.delete')" @click="removeConnection(item)">×</button></div><details><summary>{{ t("settings.modelCatalog") }}</summary><div class="catalog-list"><span v-for="model in item.models" :key="model.id" :class="{ unavailable: !model.available }"><strong>{{ model.display_name }}</strong><small>{{ model.model_id }} · {{ model.model_type }}</small></span></div></details></article><div v-if="!connections.length" class="empty-inline"><span>M</span><p>{{ t("common.empty") }}</p></div></div>
        </div>
        <div v-if="section === 'extensions'">
          <div class="section-heading"><div><p class="eyebrow">EXTENSIONS / ISOLATED RUNTIME</p><h2>{{ t("settings.extensions") }}</h2></div></div>
          <nav class="subtabs"><button :class="{ active: extension === 'mcp' }" @click="extension = 'mcp'">{{ t("settings.mcp") }}</button><button :class="{ active: extension === 'skills' }" @click="extension = 'skills'">{{ t("settings.skills") }}</button><button :class="{ active: extension === 'cli' }" @click="extension = 'cli'">{{ t("settings.cli") }}</button></nav>
          <div v-if="extension === 'mcp'"><button class="button primary compact-action" @click="openNewMCP">＋ MCP</button><div class="resource-list"><article v-for="item in mcp" :key="item.id"><span class="resource-mark">MCP</span><div><strong>{{ item.name }}</strong><p>{{ item.transport }} · {{ item.url || `${item.runner} ${item.package}@${item.package_version}` }}<template v-if="item.test_error"> · {{ item.test_error }}</template></p></div><span class="safe-chip" :class="{ unsafe: !item.tested }">{{ item.test_pending ? t("settings.testPending") : item.tested ? t("settings.tested") : t("settings.testRequired") }}</span><button :aria-label="t('common.retry')" :disabled="item.test_pending" @click="testMCP(item)">↻</button><button :aria-label="t('common.edit')" @click="openMCP(item)">✎</button><button :aria-label="t('common.delete')" @click="removeMCP(item)">×</button></article></div></div>
          <div v-if="extension === 'skills'"><button class="button primary compact-action" @click="openNewSkill">＋ Skill</button><div class="resource-list"><article v-for="item in skills" :key="item.id"><span class="resource-mark">SK</span><div><strong>{{ item.name }}</strong><p>{{ item.git_url || item.source }} · {{ item.sha256.slice(0, 12) }}</p></div><button :aria-label="t('common.edit')" @click="openSkill(item)">✎</button><button :aria-label="t('common.delete')" @click="removeSkill(item)">×</button></article></div></div>
          <div v-if="extension === 'cli'" class="coming-soon"><span>⌘</span><h3>{{ t("settings.cli") }}</h3><p>{{ t("common.comingSoon") }}</p></div>
        </div>
      </div>
    </div>
  </section>
  <div v-if="showConnection" class="modal-layer" @click.self="showConnection = false"><form class="modal-card" @submit.prevent="saveConnection"><p class="eyebrow">MODEL PROVIDER</p><h2>{{ editingConnection ? t("common.edit") : t("settings.addProvider") }}</h2><label>{{ t("settings.provider") }}<select v-model="connectionForm.provider_type" :disabled="Boolean(editingConnection)" @change="choosePreset(connectionForm.provider_type)"><option v-for="preset in presets" :key="preset.provider_type" :value="preset.provider_type">{{ preset.display_name }}</option></select></label><label>{{ t("common.name") }}<input v-model="connectionForm.name" required></label><label>{{ t("settings.endpoint") }}<input v-model="connectionForm.endpoint" type="url" placeholder="https://…" required></label><fieldset><legend>{{ t("settings.protocols") }}</legend><label v-for="protocol in ['openai_responses','openai_chat','anthropic_messages','gemini']" :key="protocol" class="check-row"><input v-model="connectionForm.protocols" type="checkbox" :value="protocol"><span>{{ protocol }}</span></label></fieldset><label>API Key<input v-model="connectionForm.api_key" type="password" :required="!editingConnection" :placeholder="editingConnection ? t('settings.keepSecret') : ''"></label><div class="modal-actions"><button type="button" class="button ghost" @click="showConnection = false">{{ t("common.cancel") }}</button><button class="button primary">{{ t("common.save") }}</button></div></form></div>
  <div v-if="manualConnection" class="modal-layer" @click.self="manualConnection = undefined"><form class="modal-card" @submit.prevent="saveManualModel"><p class="eyebrow">{{ manualConnection.name }} / MODEL</p><h2>{{ t("settings.manualModel") }}</h2><label>{{ t("settings.modelId") }}<input v-model="manualModel.model_id" required></label><label>{{ t("common.name") }}<input v-model="manualModel.display_name"></label><label>{{ t("settings.modelType") }}<select v-model="manualModel.model_type"><option v-for="type in ['agent','text','embedding','image','audio','unknown']" :key="type" :value="type">{{ type }}</option></select></label><div class="modal-actions"><button type="button" class="button ghost" @click="manualConnection = undefined">{{ t("common.cancel") }}</button><button class="button primary">{{ t("common.save") }}</button></div></form></div>
  <div v-if="showMCP" class="modal-layer" @click.self="showMCP = false"><form class="modal-card" @submit.prevent="saveMCP"><p class="eyebrow">MCP SERVER</p><h2>{{ editingMCP ? t("common.edit") : t("common.new") }} MCP</h2><label>{{ t("common.name") }}<input v-model="mcpForm.name" required></label><label>{{ t("settings.transport") }}<select v-model="mcpForm.transport"><option value="streamable_http">Streamable HTTP</option><option value="stdio">stdio</option></select></label><template v-if="mcpForm.transport === 'streamable_http'"><label>URL<input v-model="mcpForm.url" type="url" required></label><label>{{ t("settings.bearerToken") }}<input v-model="mcpForm.bearerToken" type="password" :placeholder="editingMCP ? t('settings.keepSecret') : t('settings.optional')"></label></template><template v-else><label>Runner<select v-model="mcpForm.runner"><option value="npx">npx</option><option value="uvx">uvx</option></select></label><label>Package<input v-model="mcpForm.package" required></label><label>{{ t("settings.fixedVersion") }}<input v-model="mcpForm.package_version" required placeholder="1.2.3"></label><label>{{ t("settings.arguments") }}<textarea v-model="mcpForm.argumentsText" rows="4" :placeholder="t('settings.onePerLine')"></textarea></label></template><div><div v-for="(variable, index) in mcpForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><button type="button" class="text-button" @click="removeMCPEnvironment(index)">×</button></div><button type="button" class="button ghost" @click="addMCPEnvironment">＋ {{ t("settings.environment") }}</button></div><div class="modal-actions"><button type="button" class="button ghost" @click="showMCP = false">{{ t("common.cancel") }}</button><button class="button primary">{{ t("common.save") }}</button></div></form></div>
  <div v-if="showSkill" class="modal-layer" @click.self="showSkill = false"><form class="modal-card" @submit.prevent="saveSkill"><p class="eyebrow">SKILL</p><h2>{{ editingSkill ? t("common.edit") : t("common.new") }} Skill</h2><label>{{ t("common.name") }}<input v-model="skillForm.name" :disabled="Boolean(editingSkill)" required></label><label>{{ t("settings.source") }}<select v-model="skillForm.source" :disabled="Boolean(editingSkill)"><option value="git">Git</option><option value="upload">ZIP</option></select></label><template v-if="skillForm.source === 'git'"><label>Git URL<input v-model="skillForm.git_url" type="url" :disabled="Boolean(editingSkill)" required placeholder="https://github.com/…"></label><label>Git ref<input v-model="skillForm.git_ref" required></label></template><label v-else>ZIP<input type="file" accept=".zip,application/zip" :required="Boolean(editingSkill) || !skillForm.archive" @change="selectSkillArchive"></label><p class="muted">{{ t("settings.skillHint") }}</p><div class="modal-actions"><button type="button" class="button ghost" @click="showSkill = false">{{ t("common.cancel") }}</button><button class="button primary">{{ t("common.save") }}</button></div></form></div>
</template>
