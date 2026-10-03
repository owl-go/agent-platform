<script setup lang="ts">
import { computed, inject, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type AssistantPublicationStats, type SmartAssistant, type SmartAssistantFAQ, type SmartAssistantInput } from "../api/client";

const props = defineProps<{ modelValue: boolean; assistant?: SmartAssistant }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; updated: [assistant: SmartAssistant]; error: [message: string] }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const saving = ref(false);
const saveError = ref("");
const shareDraft = ref<SmartAssistant["share"]>({ enabled: false, width: "100%", height: 600 });
const allowedOrigins = ref("");
const shareToken = ref("");
const previewOpen = ref(false);
const stats = ref<AssistantPublicationStats>();
const faqs = ref<SmartAssistantFAQ[]>([]);
const open = computed({ get: () => props.modelValue, set: (value: boolean) => emit("update:modelValue", value) });
const embedSnippet = computed(() => props.assistant && shareToken.value ? `<iframe src="${window.location.origin}/embed/assistant/${shareToken.value}" width="${props.assistant.share.width}" height="${props.assistant.share.height}"></iframe>` : "");

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
    share: { enabled: shareDraft.value.enabled, allowed_origins: origins, width: shareDraft.value.width.trim() || "100%", height: shareDraft.value.height, free_text_enabled: true, daily_call_limit: 0, data_processing_acknowledged: shareDraft.value.data_processing_acknowledged ?? false },
  };
}
function errorMessage(cause: unknown) {
  if (cause instanceof ApiError && cause.code === "version_conflict") return t("aiApplications.versionConflict");
  if (cause instanceof ApiError && cause.code === "assistant_model_unavailable") return t("aiApplications.modelUnavailable");
  if (cause instanceof ApiError && cause.kind === "validation") return t("aiApplications.share.validationFailed");
  return t("aiApplications.saveFailed");
}
function showError(message: string) {
  saveError.value = message;
  emit("error", message);
}
async function saveShare() {
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
    const updated = await api.updateSmartAssistant(assistant.id, payload, assistant.version);
    emit("updated", updated);
    shareDraft.value = { ...updated.share, allowed_origins: [...(updated.share.allowed_origins ?? [])] };
    allowedOrigins.value = origins.join("\n");
    shareToken.value = updated.share.token ?? "";
    if (!shareToken.value) open.value = false;
  } catch (cause) {
    showError(errorMessage(cause));
  } finally {
    saving.value = false;
  }
}
async function regenerateToken() {
  const assistant = props.assistant;
  if (!assistant?.share.enabled || !shareDraft.value.enabled || saving.value) return;
  saving.value = true;
  saveError.value = "";
  try {
    const result = await api.regenerateAssistantShareToken(assistant.id, assistant.version);
    emit("updated", result.assistant);
    shareToken.value = result.token;
  } catch (cause) {
    showError(errorMessage(cause));
  } finally {
    saving.value = false;
  }
}

watch(() => props.modelValue, async (value) => {
  if (!value || !props.assistant) return;
  shareDraft.value = { ...props.assistant.share, allowed_origins: [...(props.assistant.share.allowed_origins ?? [])] };
  allowedOrigins.value = props.assistant.share.allowed_origins?.join("\n") ?? "";
  saveError.value = "";
  shareToken.value = "";
  previewOpen.value = false;
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
</script>

<template>
  <el-dialog v-model="open" class="application-share-dialog" :title="t('aiApplications.share.title')" width="min(620px, 92vw)" destroy-on-close>
    <el-alert v-if="saveError" :title="saveError" type="error" show-icon :closable="false" />
    <el-form v-if="assistant" label-position="top">
      <el-form-item :label="t('aiApplications.share.enabled')"><el-switch v-model="shareDraft.enabled" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.width')"><el-input v-model="shareDraft.width" placeholder="100% / 640px" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.height')"><el-input-number v-model="shareDraft.height" :min="400" :max="1600" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.allowedOrigins')"><el-input v-model="allowedOrigins" type="textarea" :rows="3" :placeholder="t('aiApplications.share.allowedOriginsPlaceholder')" /></el-form-item>
      <el-alert v-if="shareDraft.enabled" type="warning" :closable="false" :title="t('aiApplications.share.impactTitle')" :description="t('aiApplications.share.impactDescription')" show-icon />
      <el-checkbox v-if="shareDraft.enabled" v-model="shareDraft.data_processing_acknowledged" data-testid="share-acknowledgement">{{ t('aiApplications.share.acknowledge') }}</el-checkbox>
      <div class="share-dialog-actions">
        <el-button @click="previewOpen = !previewOpen">{{ t('aiApplications.share.visitorPreview') }}</el-button>
        <el-button :disabled="!assistant.share.enabled || !shareDraft.enabled || saving" @click="regenerateToken">{{ t('aiApplications.share.regenerate') }}</el-button>
      </div>
      <el-input v-if="shareToken" class="share-snippet" :model-value="embedSnippet" readonly type="textarea" :rows="3" />
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
