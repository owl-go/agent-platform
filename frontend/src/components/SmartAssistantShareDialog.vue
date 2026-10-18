<script setup lang="ts">
import { saveErrorMessage } from "../api/saveErrors";
import { computed, inject, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { assistantEmbedSnippet } from "../assistantEmbedding";
import { platformApiKey, type AssistantPublicationStats, type SmartAssistant, type SmartAssistantFAQ, type SmartAssistantInput } from "../api/client";

const props = defineProps<{ modelValue: boolean; assistant?: SmartAssistant }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; updated: [assistant: SmartAssistant]; error: [message: string] }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const saving = ref(false);
const saveError = ref("");
const shareDraft = ref<SmartAssistant["share"]>({ enabled: false, width: "100%", height: 600 });
const allowedOrigins = ref("");
const shareToken = ref("");
const pendingIcon = ref<File>();
const iconPreview = ref("");
const iconInput = ref<HTMLInputElement>();
let iconRequest = 0;
const previewOpen = ref(false);
const stats = ref<AssistantPublicationStats>();
const faqs = ref<SmartAssistantFAQ[]>([]);
const open = computed({ get: () => props.modelValue, set: (value: boolean) => emit("update:modelValue", value) });
const embedSnippet = computed(() => props.assistant && shareToken.value ? assistantEmbedSnippet(window.location.origin, shareToken.value, props.assistant.name, shareDraft.value, { open: t("aiApplications.share.openChat"), close: t("aiApplications.share.closeChat") }) : "");
function replaceIconPreview(value = "") {
  if (iconPreview.value.startsWith("blob:")) URL.revokeObjectURL(iconPreview.value);
  iconPreview.value = value;
}
function selectIcon(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  if (!["image/png", "image/jpeg", "image/webp", "image/gif"].includes(file.type) || file.size > 2 * 1024 * 1024) {
    showError(t("aiApplications.share.invalidIcon"));
    (event.target as HTMLInputElement).value = "";
    return;
  }
  iconRequest++;
  pendingIcon.value = file;
  replaceIconPreview(URL.createObjectURL(file));
  saveError.value = "";
}
function resetIcon() {
  iconRequest++;
  pendingIcon.value = undefined;
  shareDraft.value.widget_icon = "";
  replaceIconPreview();
  if (iconInput.value) iconInput.value.value = "";
}
async function copySnippet() {
  try { await navigator.clipboard.writeText(embedSnippet.value); }
  catch { showError(t("aiApplications.share.copyFailed")); }
}

function normalizedOrigins(): string[] {
  return [...new Set(allowedOrigins.value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean).map((origin) => {
    try {
      const url = new URL(origin);
      const localHTTP = url.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname);
      if ((url.protocol !== "https:" && !localHTTP) || url.username || url.password || url.hostname.includes("*") || !/^https?:\/\/[^/?#]+\/?$/i.test(origin)) throw new Error();
      return url.origin;
    } catch { throw new Error(t("aiApplications.share.invalidOrigins")); }
  }))];
}
function input(origins: string[]): SmartAssistantInput | undefined {
  const assistant = props.assistant;
  if (!assistant) return undefined;
  return {
    name: assistant.name.trim(),
    icon: assistant.icon,
    description: assistant.description ?? "",
    introduction: assistant.introduction,
    scenario: assistant.scenario,
    prompt: assistant.prompt ?? "",
    preprocess_prompt: assistant.preprocess_prompt ?? "",
    provider_model_id: assistant.provider_model_id,
    service_goal: assistant.service_goal,
    answer_scope: assistant.answer_scope,
    operating_rules: assistant.operating_rules,
    response_style: assistant.response_style,
    knowledge_base_ids: assistant.knowledge_base_ids,
    expert_id: assistant.expert_id,
    expert_team_id: assistant.expert_team_id,
    state: assistant.state,
    share: { ...shareDraft.value, token: undefined, embed_type: shareDraft.value.embed_type ?? "fullscreen", widget_default_open: shareDraft.value.widget_default_open ?? false, enabled: shareDraft.value.enabled, allowed_origins: origins, width: shareDraft.value.width.trim() || "100%", height: shareDraft.value.height, free_text_enabled: true, daily_call_limit: 0, data_processing_acknowledged: shareDraft.value.data_processing_acknowledged ?? false },
  };
}
function errorMessage(cause: unknown) {
  return saveErrorMessage(cause, t, "aiApplications.saveFailed");
}
function showError(message: string) {
  saveError.value = message;
  emit("error", message);
}
async function persistShare(regenerate: boolean) {
  if (saving.value) return;
  saveError.value = "";
  const assistant = props.assistant;
  let origins: string[];
  try { origins = normalizedOrigins(); }
  catch (cause) { showError((cause as Error).message); return; }
  const payload = input(origins);
  if (!assistant || !payload) return;
  if (payload.share?.enabled && (!payload.share.allowed_origins?.length || !payload.share.data_processing_acknowledged)) {
    showError(t("aiApplications.share.controlsRequired"));
    return;
  }
  const width = payload.share?.width ?? "100%";
  const pixels = /^([0-9]+)px$/.exec(width);
  if (width !== "100%" && (!pixels || Number(pixels[1]) < 320 || Number(pixels[1]) > 1920)) {
    showError(t("aiApplications.share.invalidWidth")); return;
  }
  if (!Number.isInteger(shareDraft.value.height) || shareDraft.value.height < 400 || shareDraft.value.height > 1600) {
    showError(t("aiApplications.share.invalidHeight")); return;
  }
  saving.value = true;
  try {
    const updated = pendingIcon.value
      ? await api.uploadSmartAssistantWidgetIcon(assistant.id, pendingIcon.value, payload, assistant.version)
      : await api.updateSmartAssistant(assistant.id, payload, assistant.version);
    pendingIcon.value = undefined;
    emit("updated", updated);
    shareDraft.value = { embed_type: "fullscreen", widget_default_open: false, ...updated.share, allowed_origins: [...(updated.share.allowed_origins ?? [])] };
    allowedOrigins.value = origins.join("\n");
    shareToken.value = updated.share.enabled ? updated.share.token ?? shareToken.value : "";
    if (!updated.share.enabled) open.value = false;
    if (regenerate && updated.share.enabled && !updated.share.token) {
      shareToken.value = "";
      const result = await api.regenerateAssistantShareToken(updated.id, updated.version);
      emit("updated", result.assistant);
      shareToken.value = result.token;
    }
  } catch (cause) {
    showError(errorMessage(cause));
  } finally {
    saving.value = false;
  }
}
async function saveShare() { await persistShare(false); }
async function regenerateToken() {
  if (!shareDraft.value.enabled) return;
  await persistShare(true);
}

watch(() => props.modelValue, async (value) => {
  const request = ++iconRequest;
  pendingIcon.value = undefined;
  replaceIconPreview();
  if (!value || !props.assistant) return;
  shareDraft.value = { embed_type: "fullscreen", widget_default_open: false, ...props.assistant.share, allowed_origins: [...(props.assistant.share.allowed_origins ?? [])] };
  allowedOrigins.value = props.assistant.share.allowed_origins?.join("\n") ?? "";
  saveError.value = "";
  shareToken.value = "";
  previewOpen.value = false;
  if (props.assistant.share.widget_icon) {
    void api.getSmartAssistantWidgetIcon(props.assistant.id).then((blob) => {
      if (request === iconRequest && props.modelValue && !pendingIcon.value) replaceIconPreview(URL.createObjectURL(blob));
    }).catch(() => { if (request === iconRequest) showError(t("aiApplications.share.iconLoadFailed")); });
  }
  try {
    [stats.value, faqs.value] = await Promise.all([
      api.getAssistantPublicationStats(props.assistant.id),
      api.listAssistantFAQs(props.assistant.id),
    ]);
  } catch {
    stats.value = undefined;
    faqs.value = [];
  }
}, { immediate: true });
onBeforeUnmount(() => { iconRequest++; replaceIconPreview(); });
</script>

<template>
  <el-dialog v-model="open" class="application-share-dialog" :title="t('aiApplications.share.title')" width="min(620px, 92vw)" destroy-on-close>
    <el-alert v-if="saveError" :title="saveError" type="error" show-icon :closable="false" />
    <el-form v-if="assistant" label-position="top">
      <el-form-item :label="t('aiApplications.share.enabled')"><el-switch v-model="shareDraft.enabled" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.embedType')">
        <el-radio-group v-model="shareDraft.embed_type" data-testid="embed-type">
          <el-radio value="fullscreen">{{ t('aiApplications.share.fullscreen') }}</el-radio>
          <el-radio value="floating">{{ t('aiApplications.share.floating') }}</el-radio>
        </el-radio-group>
      </el-form-item>
      <template v-if="shareDraft.embed_type === 'floating'">
        <el-form-item :label="t('aiApplications.share.initialDisplay')">
          <el-radio-group v-model="shareDraft.widget_default_open" data-testid="widget-default-open">
            <el-radio :value="true">{{ t('aiApplications.share.defaultOpen') }}</el-radio>
            <el-radio :value="false">{{ t('aiApplications.share.defaultIcon') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('aiApplications.share.chatIcon')">
          <div class="share-widget-icon">
            <img v-if="iconPreview" :src="iconPreview" :alt="t('aiApplications.share.chatIcon')" />
            <input ref="iconInput" data-testid="widget-icon-input" type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden @change="selectIcon" />
            <el-button :disabled="saving" @click="iconInput?.click()">{{ t('aiApplications.share.uploadIcon') }}</el-button>
            <el-button v-if="iconPreview || shareDraft.widget_icon" :disabled="saving" @click="resetIcon">{{ t('aiApplications.share.resetIcon') }}</el-button>
          </div>
        </el-form-item>
      </template>
      <el-form-item :label="t('aiApplications.share.width')"><el-input v-model="shareDraft.width" placeholder="100% / 640px" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.height')"><el-input-number v-model="shareDraft.height" :min="400" :max="1600" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.allowedOrigins')"><el-input v-model="allowedOrigins" type="textarea" :rows="3" :placeholder="t('aiApplications.share.allowedOriginsPlaceholder')" /></el-form-item>
      <el-alert v-if="shareDraft.enabled" type="warning" :closable="false" :title="t('aiApplications.share.impactTitle')" :description="t('aiApplications.share.impactDescription')" show-icon />
      <el-checkbox v-if="shareDraft.enabled" v-model="shareDraft.data_processing_acknowledged" data-testid="share-acknowledgement">{{ t('aiApplications.share.acknowledge') }}</el-checkbox>
      <div class="share-dialog-actions">
        <el-button @click="previewOpen = !previewOpen">{{ t('aiApplications.share.visitorPreview') }}</el-button>
        <el-button :disabled="!shareDraft.enabled || saving" @click="regenerateToken">{{ t('aiApplications.share.regenerate') }}</el-button>
      </div>
      <div v-if="shareToken" class="share-code">
        <el-input class="share-snippet" :model-value="embedSnippet" readonly type="textarea" :rows="6" :aria-label="t('aiApplications.share.embedCode')" />
        <el-button @click="copySnippet">{{ t('aiApplications.share.copyCode') }}</el-button>
      </div>
      <el-alert v-if="shareToken" type="success" :closable="false" :title="t('aiApplications.share.tokenOnce')" />
      <section v-if="previewOpen" class="share-visitor-preview" data-testid="share-visitor-preview">
        <small>{{ t('aiApplications.share.previewNotice') }}</small>
        <h3>{{ assistant.name }}</h3>
        <p>{{ assistant.introduction }}</p>
        <el-tag v-for="faq in faqs.filter((item) => item.enabled)" :key="faq.id" effect="plain">{{ faq.question }}</el-tag>
      </section>
      <section v-if="stats" class="share-aggregate-stats" data-testid="share-aggregate-stats">
        <strong>{{ t('aiApplications.share.aggregateStats', { days: stats.window_days }) }}</strong>
        <span>{{ t('aiApplications.share.statConversations', { count: stats.external_conversations }) }}</span>
        <span>{{ t('aiApplications.share.statCalls', { count: stats.free_text_calls }) }}</span>
        <span>{{ t('aiApplications.share.statErrors', { count: stats.failed_or_cancelled_answers }) }}</span>
        <span>{{ t('aiApplications.share.statCredits', { count: (stats.credit_consumed_hundredths / 100).toFixed(2) }) }}</span>
      </section>
    </el-form>
    <template #footer><el-button @click="open = false">{{ t('common.cancel') }}</el-button><el-button data-testid="share-save" type="primary" :loading="saving" @click="saveShare">{{ t('common.save') }}</el-button></template>
  </el-dialog>
</template>

<style scoped>
.share-widget-icon { display: flex; align-items: center; gap: var(--aw-space-2); }
.share-widget-icon img { width: 48px; height: 48px; object-fit: cover; border-radius: 50%; }
.share-code > .el-button { margin-top: var(--aw-space-2); }

.share-dialog-actions,
.share-aggregate-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
}
.share-visitor-preview,
.share-aggregate-stats {
  margin-top: 14px;
  padding: 14px;
  border: 1px solid var(--color-border, #dce8df);
  border-radius: 12px;
}
.share-visitor-preview h3 { margin: 10px 0 4px; }
.share-visitor-preview p { margin: 0 0 10px; }
.share-visitor-preview .el-tag { margin: 0 8px 8px 0; }
.share-aggregate-stats strong { flex-basis: 100%; }
</style>

<style>
.application-share-dialog .el-dialog__body { max-height: calc(100dvh - 230px); overflow-y: auto; }
</style>
