<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { Plus } from "@element-plus/icons-vue";
import { Ellipsis, MessageCircle, Pause, Play, Search, Share2, ShieldCheck, Trash2 } from "@lucide/vue";
import { ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import IconPicker from "../components/IconPicker.vue";
import { assistantModelOptions } from "../assistantModels";
import SmartAssistantShareDialog from "../components/SmartAssistantShareDialog.vue";
import { ApiError, platformApiKey, type AssistantPublicationValidation, type ModelProviderConnection, type SmartAssistant, type SmartAssistantInput } from "../api/client";

const api = inject(platformApiKey)!;
const router = useRouter();
const { t } = useI18n();
const items = ref<SmartAssistant[]>([]);
const connections = ref<ModelProviderConnection[]>([]);
const modelOptions = computed(() => assistantModelOptions(connections.value));
const loading = ref(false);
const error = ref("");
const search = ref("");
const scenario = ref("");
const createDialogOpen = ref(false);
const creating = ref(false);
const form = ref<SmartAssistantInput & { icon: string }>({ name: "", icon: "", description: "", scenario: "custom", introduction: "", prompt: "", preprocess_prompt: "", provider_model_id: "", response_style: "" });
const iconFile = ref<File>();
const iconPreviewUrl = ref("");
const shareAssistant = ref<SmartAssistant>();
const shareDialogOpen = ref(false);
const publicationValidation = ref<AssistantPublicationValidation>();
const publicationDialogOpen = ref(false);
const scenarios = ["customer-consultation", "pre-sales-advisor", "after-sales-support", "product-guide", "enterprise-knowledge", "recruitment", "training", "custom"];
const filteredItems = computed(() => {
  const query = search.value.trim().toLocaleLowerCase();
  return items.value.filter((item) => (!query || item.name.toLocaleLowerCase().includes(query)) && (!scenario.value || item.scenario === scenario.value));
});

async function refresh() {
  loading.value = true;
  try { [items.value, connections.value] = await Promise.all([api.listSmartAssistants(), api.listModelProviderConnections()]); }
  catch { error.value = t("aiApplications.loadFailed"); }
  finally { loading.value = false; }
}
async function create() {
  if (!form.value.name.trim() || !iconFile.value || !form.value.provider_model_id) return;
  creating.value = true;
  let created: SmartAssistant | undefined;
  try {
    created = await api.createSmartAssistant({ ...form.value, icon: "", share: { enabled: false, width: "100%", height: 600 } });
    const item = await api.uploadSmartAssistantIcon(created.id, iconFile.value, created.version);
    items.value.unshift(item);
    form.value = { name: "", icon: "", description: "", scenario: "custom", introduction: "", prompt: "", preprocess_prompt: "", provider_model_id: "", response_style: "" };
    iconFile.value = undefined;
    replaceIconPreview();
    createDialogOpen.value = false;
  } catch (cause) {
    if (created) await api.deleteSmartAssistant(created.id).catch(() => undefined);
    error.value = created ? t("aiApplications.iconUploadFailed") : cause instanceof ApiError && cause.code === "assistant_model_unavailable" ? t("aiApplications.modelUnavailable") : t("aiApplications.saveFailed");
  } finally { creating.value = false; }
}
function openCreate() { createDialogOpen.value = true; }
function replaceIconPreview(value = "") {
  if (iconPreviewUrl.value.startsWith("blob:")) URL.revokeObjectURL(iconPreviewUrl.value);
  iconPreviewUrl.value = value;
}
function selectIcon(file: File) {
  iconFile.value = file;
  error.value = "";
  if (typeof URL.createObjectURL === "function") replaceIconPreview(URL.createObjectURL(file));
}
function invalidIcon() { error.value = t("aiApplications.iconInvalid"); }
function openShare(item: SmartAssistant) { shareAssistant.value = item; shareDialogOpen.value = true; }
function updateSharedAssistant(updated: SmartAssistant) { items.value = items.value.map((item) => item.id === updated.id ? updated : item); shareAssistant.value = updated; }
function showShareError(message: string) { error.value = message; }
async function startConversation(item: SmartAssistant) {
  try {
    const history = await api.listAssistantConversations(item.id);
    if (!history.length && item.state !== "enabled") {
      error.value = t("aiApplications.startRequiresEnabled");
      return;
    }
    const conversation = history[0] ?? await api.createAssistantConversation(item.id);
    await router.push(`/ai-apps/assistants/${encodeURIComponent(item.id)}/conversations/${encodeURIComponent(conversation.id)}`);
  } catch (cause) { error.value = cause instanceof ApiError && cause.code === "assistant_model_unavailable" ? t("aiApplications.chat.modelUnavailable") : t("aiApplications.startFailed"); }
}
async function toggleState(item: SmartAssistant) {
  const next = item.state === "enabled" ? "disabled" : "enabled";
  try {
    if (next === "enabled") {
      publicationValidation.value = await api.runAssistantPublicationCheck(item.id);
      if (!publicationValidation.value.ready) {
        publicationDialogOpen.value = true;
        return;
      }
    }
    const updated = await api.setSmartAssistantState(item.id, next, item.version);
    items.value = items.value.map((candidate) => candidate.id === updated.id ? updated : candidate);
  } catch { error.value = t("aiApplications.saveFailed"); }
}
async function inspectPublication(item: SmartAssistant) {
  try {
    publicationValidation.value = await api.runAssistantPublicationCheck(item.id);
    if (publicationValidation.value.ready) await refresh();
    publicationDialogOpen.value = true;
  } catch {
    error.value = t("aiApplications.publication.checkFailed");
  }
}
function handleCardAction(item: SmartAssistant, command: string) {
  if (command === "toggle") void toggleState(item);
  if (command === "check") void inspectPublication(item);
  if (command === "share") openShare(item);
  if (command === "delete") void remove(item);
}
async function remove(item: SmartAssistant) {
  try {
    await ElMessageBox.confirm(t("aiApplications.confirmDelete", { name: item.name }), t("common.delete"), { type: "warning", confirmButtonText: t("common.delete"), cancelButtonText: t("common.cancel") });
    await api.deleteSmartAssistant(item.id);
    items.value = items.value.filter((candidate) => candidate.id !== item.id);
  } catch (cause) {
    if (cause !== "cancel" && cause !== "close") error.value = t("aiApplications.deleteFailed");
  }
}
onMounted(refresh);
onBeforeUnmount(() => replaceIconPreview());
</script>

<template>
  <section class="application-catalog-page">
    <div class="application-catalog-toolbar">
      <div><h2>{{ t('aiApplications.assistants.title') }}</h2></div>
      <el-button class="application-create-trigger" type="primary" :icon="Plus" @click="openCreate">{{ t('aiApplications.create') }}</el-button>
    </div>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <el-dialog v-model="createDialogOpen" class="application-create-dialog" :title="t('aiApplications.create')" width="min(560px, 92vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="create">
        <el-form-item :label="t('aiApplications.name')"><el-input v-model="form.name" autofocus :placeholder="t('aiApplications.assistantNamePlaceholder')" /></el-form-item>
        <el-form-item :label="t('aiApplications.icon')" required><IconPicker v-model="form.icon" upload-only remote-upload :preview-url="iconPreviewUrl" :uploading="creating" @file-selected="selectIcon" @invalid="invalidIcon" /></el-form-item>
        <el-form-item :label="t('aiApplications.scenario')"><el-select v-model="form.scenario"><el-option v-for="value in scenarios" :key="value" :label="t(`aiApplications.scenarios.${value}`)" :value="value" /></el-select></el-form-item>
        <el-form-item :label="t('aiApplications.model')" required><el-select v-model="form.provider_model_id" data-testid="assistant-model-select" :placeholder="t('aiApplications.modelPlaceholder')"><el-option v-for="option in modelOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select><small v-if="!modelOptions.length">{{ t('aiApplications.modelUnavailable') }}</small></el-form-item>
        <el-form-item :label="t('aiApplications.description')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="createDialogOpen = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="creating" :disabled="!form.provider_model_id" @click="create">{{ t('common.save') }}</el-button></template>
    </el-dialog>
    <div class="application-list-filters">
      <el-input v-model="search" class="assistant-search" clearable :placeholder="t('aiApplications.search')" />
      <el-select v-model="scenario" clearable :placeholder="t('aiApplications.allScenarios')"><el-option v-for="value in scenarios" :key="value" :label="t(`aiApplications.scenarios.${value}`)" :value="value" /></el-select>
      <el-button class="assistant-search-button" type="primary" :icon="Search" @click="search = search.trim()">{{ t('aiApplications.searchAction') }}</el-button>
    </div>
    <div v-loading="loading" class="application-card-grid">
      <el-card v-for="item in filteredItems" :key="item.id" class="application-card" role="button" tabindex="0" @click="router.push(`/ai-apps/assistants/${item.id}`)" @keydown.enter="router.push(`/ai-apps/assistants/${item.id}`)">
        <div class="application-card-heading">
          <div><h3>{{ item.name }}</h3><p>{{ item.description || item.introduction || t('aiApplications.assistants.defaultIntro') }}</p></div>
          <div class="application-card-actions" @click.stop @keydown.stop>
            <el-tooltip :content="t('aiApplications.startConversation')" placement="top">
              <el-button data-testid="assistant-chat" class="application-card-action-button" text :icon="MessageCircle" :aria-label="t('aiApplications.startConversation')" @click.stop="startConversation(item)" />
            </el-tooltip>
            <el-dropdown trigger="click" placement="bottom-end" @command="handleCardAction(item, $event)">
              <el-button data-testid="assistant-more" class="application-card-action-button" text :icon="Ellipsis" :aria-label="t('common.more')" :title="t('common.more')" @click.stop />
              <template #dropdown>
                <el-dropdown-menu class="assistant-card-action-menu">
                  <el-dropdown-item command="toggle" :title="item.state === 'enabled' ? t('aiApplications.disable') : t('aiApplications.enable')"><component :is="item.state === 'enabled' ? Pause : Play" :size="16" />{{ item.state === 'enabled' ? t('aiApplications.disable') : t('aiApplications.enable') }}</el-dropdown-item>
                  <el-dropdown-item command="share" :title="t('aiApplications.shareAction')"><Share2 :size="16" />{{ t('aiApplications.shareAction') }}</el-dropdown-item>
                  <el-dropdown-item command="check" :title="t('aiApplications.publication.runCheck')"><ShieldCheck :size="16" />{{ t('aiApplications.publication.runCheck') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" class="assistant-card-delete-action" :title="t('common.delete')"><Trash2 :size="16" />{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
        <small>{{ t(`aiApplications.states.${item.state}`) }} · {{ item.share.enabled ? t('aiApplications.publication.controlledVisitors') : t('aiApplications.publication.authenticatedOwner') }} · {{ t('aiApplications.publication.boundKnowledge', { count: item.knowledge_base_ids.length }) }}</small>
        <small>{{ item.last_validated_at && item.validated_version === item.version ? t('aiApplications.publication.validatedAt', { time: new Date(item.last_validated_at).toLocaleString() }) : t('aiApplications.publication.notValidated') }}</small>
      </el-card>
      <el-empty v-if="!loading && !filteredItems.length" :description="t('aiApplications.assistants.empty')" />
    </div>
    <SmartAssistantShareDialog v-model="shareDialogOpen" :assistant="shareAssistant" @updated="updateSharedAssistant" @error="showShareError" />
    <el-dialog v-model="publicationDialogOpen" class="assistant-publication-dialog" :title="t('aiApplications.publication.title')" width="min(640px, 92vw)" data-testid="assistant-publication-dialog">
      <el-result v-if="publicationValidation" :icon="publicationValidation.ready ? 'success' : 'warning'" :title="publicationValidation.ready ? t('aiApplications.publication.ready') : t('aiApplications.publication.blocked')">
        <template #extra>
          <ul class="publication-check-list">
            <li v-for="check in publicationValidation.checks" :key="check.code"><el-tag :type="check.ready ? 'success' : 'danger'" effect="plain">{{ check.ready ? t('aiApplications.publication.passed') : t('aiApplications.publication.failed') }}</el-tag><span>{{ t(`aiApplications.publication.checks.${check.code}`) }}</span></li>
          </ul>
        </template>
      </el-result>
    </el-dialog>
  </section>
</template>

<style scoped>
.application-card > :deep(.el-card__body) { display: grid; gap: 8px; }
.publication-check-list { display: grid; gap: 10px; margin: 0; padding: 0; text-align: left; list-style: none; }
.publication-check-list li { display: grid; grid-template-columns: 72px minmax(0, 1fr); align-items: center; gap: 10px; }
@media (max-width: 640px) { .publication-check-list li { grid-template-columns: 1fr; } }
</style>
