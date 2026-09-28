<script setup lang="ts">
import { computed, inject, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type AssistantPublicationStats, type SmartAssistant, type SmartAssistantFAQ, type SmartAssistantInput } from "../api/client";

const props = defineProps<{ modelValue: boolean; assistant?: SmartAssistant }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; updated: [assistant: SmartAssistant]; error: [message: string] }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const saving = ref(false);
const shareToken = ref("");
const previewOpen = ref(false);
const stats = ref<AssistantPublicationStats>();
const faqs = ref<SmartAssistantFAQ[]>([]);
const open = computed({ get: () => props.modelValue, set: (value: boolean) => emit("update:modelValue", value) });
const allowedOrigins = computed({
  get: () => props.assistant?.share.allowed_origins?.join("\n") || "",
  set: (value: string) => { if (props.assistant) props.assistant.share.allowed_origins = value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean); },
});
const embedSnippet = computed(() => props.assistant && shareToken.value ? `<iframe src="${window.location.origin}/embed/assistant/${shareToken.value}" width="${props.assistant.share.width}" height="${props.assistant.share.height}"></iframe>` : "");

function input(): SmartAssistantInput | undefined {
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
    digital_human_id: assistant.digital_human_id || undefined,
    state: assistant.state,
    share: { enabled: assistant.share.enabled, allowed_origins: assistant.share.allowed_origins ?? [], width: assistant.share.width || "100%", height: assistant.share.height || 600, free_text_enabled: assistant.share.free_text_enabled ?? false, daily_call_limit: assistant.share.daily_call_limit ?? 0, data_processing_acknowledged: assistant.share.data_processing_acknowledged ?? false },
  };
}
function errorMessage(cause: unknown) {
  if (cause instanceof ApiError && cause.code === "version_conflict") return t("aiApplications.versionConflict");
  return t("aiApplications.saveFailed");
}
async function saveShare() {
  const assistant = props.assistant;
  const payload = input();
  if (!assistant || !payload) return;
  if (payload.share?.enabled && (!payload.share.allowed_origins?.length || !payload.share.daily_call_limit || !payload.share.data_processing_acknowledged)) {
    emit("error", t("aiApplications.share.controlsRequired"));
    return;
  }
  saving.value = true;
  try {
    const updated = await api.updateSmartAssistant(assistant.id, payload, assistant.version);
    emit("updated", updated);
    shareToken.value = updated.share.token ?? "";
    if (!shareToken.value) open.value = false;
  } catch (cause) {
    emit("error", errorMessage(cause));
  } finally {
    saving.value = false;
  }
}
async function regenerateToken() {
  const assistant = props.assistant;
  if (!assistant) return;
  try {
    const result = await api.regenerateAssistantShareToken(assistant.id, assistant.version);
    emit("updated", result.assistant);
    shareToken.value = result.token;
  } catch (cause) {
    emit("error", errorMessage(cause));
  }
}

watch(() => props.modelValue, async (value) => {
  if (!value || !props.assistant) return;
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
    <el-form v-if="assistant" label-position="top">
      <el-form-item :label="t('aiApplications.share.enabled')"><el-switch v-model="assistant.share.enabled" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.width')"><el-input v-model="assistant.share.width" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.height')"><el-input-number v-model="assistant.share.height" :min="400" :max="1600" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.allowedOrigins')"><el-input v-model="allowedOrigins" type="textarea" :rows="3" :placeholder="t('aiApplications.share.allowedOriginsPlaceholder')" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.dailyLimit')"><el-input-number v-model="assistant.share.daily_call_limit" :min="1" :max="100000" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.freeText')"><el-switch v-model="assistant.share.free_text_enabled" /></el-form-item>
      <el-alert v-if="assistant.share.enabled" type="warning" :closable="false" :title="t('aiApplications.share.impactTitle')" :description="t('aiApplications.share.impactDescription')" show-icon />
      <el-checkbox v-if="assistant.share.enabled" v-model="assistant.share.data_processing_acknowledged" data-testid="share-acknowledgement">{{ t('aiApplications.share.acknowledge') }}</el-checkbox>
      <div class="share-dialog-actions">
        <el-button @click="previewOpen = !previewOpen">{{ t('aiApplications.share.visitorPreview') }}</el-button>
        <el-button :disabled="!assistant.share.enabled" @click="regenerateToken">{{ t('aiApplications.share.regenerate') }}</el-button>
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
