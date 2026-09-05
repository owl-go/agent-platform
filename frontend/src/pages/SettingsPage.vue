<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, runtimeEngineDisplayName, type ModelProviderConnection, type ModelProviderPreset, type PersonalSettings, type Personality, type RuntimeEngine, type RuntimeEngineStatus } from "../api/client";
import { authContextKey } from "../auth/session";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";

type Section = "personal" | "models";

const api = inject(platformApiKey)!;
const auth = inject(authContextKey)!;
const { t, locale } = useI18n();
const canManageModels = computed(() => auth.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const settings = ref<PersonalSettings>();
const connections = ref<ModelProviderConnection[]>([]);
const presets = ref<ModelProviderPreset[]>([]);
const runtimes = ref<RuntimeEngineStatus[]>([]);
const section = ref<Section>("personal");
const error = ref("");
const notice = ref("");
const editingConnection = ref<ModelProviderConnection>();
const showConnection = ref(false);
const savingConnection = ref(false);
const connectionError = ref("");
const connectionForm = ref({ name: "", provider_type: "openai", endpoint: "", protocols: [] as string[], api_key: "" });
const manualConnection = ref<ModelProviderConnection>();
const manualModel = ref({ model_id: "" });
const pendingConnectionDelete = ref<ModelProviderConnection>();
const personalities: Personality[] = ["gentle_professional", "direct_efficient", "lively_friendly", "custom"];
const customPersonalityInstructions = ref("");
onMounted(() => { void refresh(); });
function clearFeedback() { error.value = ""; notice.value = ""; }
function showError(kind: "generic" | "validation" | "conflict" = "generic") { error.value = t(`errors.${kind}`); }
async function refresh() {
  clearFeedback();
  try {
    [settings.value, connections.value, presets.value, runtimes.value] = await Promise.all([api.getSettings(), api.listModelProviderConnections(), api.listModelProviderPresets(), api.listRuntimeEngines()]);
    if (settings.value.personality === "custom") customPersonalityInstructions.value = settings.value.personality_instructions;
  } catch { showError(); }
}
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

const selectableModels = computed(() => connections.value.flatMap((connection) => connection.models.filter((model) => model.available).map((model) => ({ ...model, connection }))));
function choosePreset(providerType: string) { const preset = presets.value.find((item) => item.provider_type === providerType); if (!preset || editingConnection.value) return; connectionForm.value.endpoint = preset.official_endpoint; connectionForm.value.protocols = [...preset.protocols]; if (!connectionForm.value.name) connectionForm.value.name = preset.display_name; }
function openNewConnection() { clearFeedback(); connectionError.value = ""; editingConnection.value = undefined; const preset = presets.value[0]; connectionForm.value = { name: preset?.display_name ?? "", provider_type: preset?.provider_type ?? "openai", endpoint: preset?.official_endpoint ?? "", protocols: [...(preset?.protocols ?? [])], api_key: "" }; showConnection.value = true; }
function openConnection(item: ModelProviderConnection) { clearFeedback(); connectionError.value = ""; editingConnection.value = item; connectionForm.value = { name: item.name, provider_type: item.provider_type, endpoint: item.endpoint, protocols: [...item.protocols], api_key: "" }; showConnection.value = true; }
function providerSaveError(cause: unknown) {
  if (cause instanceof ApiError && cause.kind === "validation") return t("settings.providerValidationFailed");
  if (cause instanceof ApiError && cause.kind === "conflict") return t("errors.conflict");
  return t("settings.providerSaveFailed");
}
async function saveConnection() {
	if (savingConnection.value) return;
  clearFeedback();
  connectionError.value = "";
  savingConnection.value = true;
  try {
    const saved = editingConnection.value
      ? await api.updateModelProviderConnection(editingConnection.value.id, connectionForm.value, editingConnection.value.version)
      : await api.createModelProviderConnection(connectionForm.value);
    const index = connections.value.findIndex((item) => item.id === saved.id);
    if (index === -1) connections.value = [saved, ...connections.value];
    else connections.value = connections.value.map((item) => item.id === saved.id ? saved : item);
    showConnection.value = false;
    notice.value = t("settings.providerSaved");
  } catch (cause) {
    connectionError.value = providerSaveError(cause);
  } finally {
    savingConnection.value = false;
  }
}
async function removeConnection() { if (!pendingConnectionDelete.value) return; try { await api.deleteModelProviderConnection(pendingConnectionDelete.value.id); pendingConnectionDelete.value = undefined; await refresh(); } catch { showError("conflict"); } }
async function refreshModels(item: ModelProviderConnection) { try { await api.refreshProviderModels(item.id); await refresh(); } catch { showError(); } }
function openManualModel(item: ModelProviderConnection) { manualConnection.value = item; manualModel.value = { model_id: "" }; }
async function saveManualModel() { if (!manualConnection.value) return; try { await api.createProviderModel(manualConnection.value.id, manualModel.value); manualConnection.value = undefined; await refresh(); } catch { showError("validation"); } }
function runtimeDefault(runtime: RuntimeEngine) { return settings.value?.runtime_model_defaults.find((item) => item.runtime_engine === runtime)?.provider_model_id ?? ""; }
function setRuntimeDefault(runtime: RuntimeEngine, modelID: string) { if (!settings.value) return; settings.value.runtime_model_defaults = settings.value.runtime_model_defaults.filter((item) => item.runtime_engine !== runtime); if (modelID) settings.value.runtime_model_defaults.push({ runtime_engine: runtime, provider_model_id: modelID }); }
function setRuntimeDefaultFromEvent(runtime: RuntimeEngine, event: Event) { setRuntimeDefault(runtime, (event.target as HTMLSelectElement).value); }
function selectPersonality(personality: Personality) {
  if (!settings.value || settings.value.personality === personality) return;
  if (settings.value.personality === "custom") customPersonalityInstructions.value = settings.value.personality_instructions;
  settings.value.personality = personality;
  settings.value.personality_instructions = personality === "custom"
    ? customPersonalityInstructions.value
    : t(`settings.${personality === "gentle_professional" ? "gentleInstructions" : personality === "direct_efficient" ? "directInstructions" : "livelyInstructions"}`);
}

</script>

<template>
  <section class="page-surface settings-page">
    <header class="page-header"><div><h1>{{ t("settings.title") }}</h1></div></header>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" /><ToastMessage v-if="notice" kind="success" :title="t('common.success')" :message="notice" :close-label="t('common.close')" @dismiss="notice = ''" />
    <div class="settings-layout">
      <nav class="settings-nav">
        <el-button text :class="{ active: section === 'personal' }" @click="section = 'personal'">{{ t("settings.personality") }}</el-button>
        <el-button v-if="canManageModels" text :class="{ active: section === 'models' }" @click="section = 'models'">{{ t("settings.model") }}</el-button>
      </nav>
      <div class="settings-canvas">
        <form v-if="section === 'personal' && settings" @submit.prevent="saveSettings">
          <div class="section-heading section-heading-actions"><el-button native-type="submit" type="primary">{{ t("common.save") }}</el-button></div>
          <div class="personality-grid"><label v-for="item in personalities" :key="item" :class="{ selected: settings.personality === item }"><input type="radio" :value="item" :checked="settings.personality === item" @change="selectPersonality(item)"><span>{{ item === 'gentle_professional' ? '◡' : item === 'direct_efficient' ? '→' : item === 'lively_friendly' ? '✦' : '⌁' }}</span><strong>{{ t(`settings.${item === 'gentle_professional' ? 'gentle' : item === 'direct_efficient' ? 'direct' : item === 'lively_friendly' ? 'lively' : 'custom'}`) }}</strong></label></div>
          <label class="block-label">{{ t("settings.instructions") }}<textarea v-model="settings.personality_instructions" rows="6" :required="settings.personality === 'custom'"></textarea></label>
          <div class="form-grid">
            <label>{{ t("settings.runtime") }}<select v-model="settings.default_runtime_engine"><option v-for="runtime in runtimes" :key="runtime.name" :value="runtime.name" :disabled="!runtime.available">{{ runtimeEngineDisplayName(runtime.name) }} · {{ runtime.available ? t("settings.available") : t("settings.unavailable") }}</option></select></label>
            <label>{{ t("settings.language") }}<select v-model="settings.language"><option value="zh-CN">中文</option><option value="en-US">English</option></select></label><label>{{ t("settings.timezone") }}<input v-model="settings.timezone"></label>
            <fieldset class="full runtime-defaults"><legend>{{ t("settings.runtimeModels") }}</legend><label v-for="runtime in runtimes" :key="runtime.name"><span>{{ runtimeEngineDisplayName(runtime.name) }}</span><select :value="runtimeDefault(runtime.name)" @change="setRuntimeDefaultFromEvent(runtime.name, $event)"><option value="">—</option><option v-for="item in selectableModels" :key="item.id" :value="item.id" :disabled="item.compatibility.find((compatibility) => compatibility.runtime_engine === runtime.name)?.status === 'incompatible'">{{ item.connection.name }} / {{ item.display_name }}</option></select></label></fieldset>
          </div>
        </form>
        <div v-if="section === 'models' && canManageModels">
          <div class="section-heading"><div><h2>{{ t("settings.providers") }}</h2></div><el-button type="primary" @click="openNewConnection">＋ {{ t("settings.addProvider") }}</el-button></div>
          <div class="provider-grid"><article v-for="item in connections" :key="item.id" class="provider-card el-card"><header><span class="resource-mark">{{ item.name.slice(0, 2).toUpperCase() }}</span><div><strong>{{ item.name }}</strong><p>{{ presets.find((preset) => preset.provider_type === item.provider_type)?.display_name ?? item.provider_type }}</p></div><el-tag :type="item.verification_status === 'verified' ? 'success' : 'warning'" size="small">{{ t(`settings.${item.verification_status}`) }}</el-tag></header><p class="provider-endpoint">{{ item.endpoint }}</p><el-alert v-if="item.last_sync_error || item.verification_error" type="error" :closable="false" :title="item.last_sync_error || item.verification_error" /><div class="provider-model-summary"><strong>{{ item.models.filter((model) => model.available).length }}</strong><span>{{ t("settings.importedModels") }}</span><small v-if="item.last_synced_at">{{ new Date(item.last_synced_at).toLocaleString() }}</small></div><div class="provider-actions"><el-button class="button" @click="refreshModels(item)">↻ {{ t("settings.refreshModels") }}</el-button><el-button class="button" @click="openManualModel(item)">＋ {{ t("settings.manualModel") }}</el-button><el-button circle :aria-label="t('common.edit')" @click="openConnection(item)">✎</el-button><el-button circle type="danger" plain :aria-label="t('common.delete')" @click="pendingConnectionDelete = item">×</el-button></div><el-collapse><el-collapse-item :title="t('settings.modelCatalog')"><div class="catalog-list"><span v-for="model in item.models" :key="model.id" :class="{ unavailable: !model.available }"><strong>{{ model.display_name }}</strong><small>{{ model.model_id }}</small></span></div></el-collapse-item></el-collapse></article><el-empty v-if="!connections.length" :description="t('common.empty')" /></div>
        </div>
      </div>
    </div>
  </section>
  <div v-if="showConnection" class="modal-layer" @click.self="showConnection = false"><form class="modal-card el-card" :aria-busy="savingConnection" @submit.prevent="saveConnection"><h2>{{ editingConnection ? t("common.edit") : t("settings.addProvider") }}</h2><label>{{ t("settings.provider") }}<select v-model="connectionForm.provider_type" :disabled="Boolean(editingConnection)" @change="choosePreset(connectionForm.provider_type)"><option v-for="preset in presets" :key="preset.provider_type" :value="preset.provider_type">{{ preset.display_name }}</option></select></label><label>{{ t("common.name") }}<input v-model="connectionForm.name" required></label><label>{{ t("settings.endpoint") }}<input v-model="connectionForm.endpoint" type="url" placeholder="http://… 或 https://…" required></label><fieldset><legend>{{ t("settings.protocols") }}</legend><label v-for="protocol in ['openai_responses','openai_chat','anthropic_messages','gemini']" :key="protocol" class="check-row"><input v-model="connectionForm.protocols" type="checkbox" :value="protocol"><span>{{ protocol }}</span></label></fieldset><label>API Key<input v-model="connectionForm.api_key" type="password" :required="!editingConnection" :placeholder="editingConnection ? t('settings.keepSecret') : ''"></label><ToastMessage v-if="connectionError" kind="error" :title="t('common.failed')" :message="connectionError" :close-label="t('common.close')" @dismiss="connectionError = ''" /><div class="modal-actions"><el-button :disabled="savingConnection" @click="showConnection = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary" :loading="savingConnection">{{ savingConnection ? t("common.saving") : t("common.save") }}</el-button></div></form></div>
  <div v-if="manualConnection" class="modal-layer" @click.self="manualConnection = undefined"><form class="modal-card el-card" @submit.prevent="saveManualModel"><h2>{{ t("settings.manualModel") }}</h2><label>{{ t("modelField") }}<input v-model="manualModel.model_id" placeholder="gpt-5.6-sol" required></label><div class="modal-actions"><el-button @click="manualConnection = undefined">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ t("common.save") }}</el-button></div></form></div>
  <ConfirmDialog :open="Boolean(pendingConnectionDelete)" :title="t('common.delete')" :message="pendingConnectionDelete ? `${t('common.delete')} ${pendingConnectionDelete.name}?` : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger @cancel="pendingConnectionDelete = undefined" @confirm="removeConnection" />
</template>
