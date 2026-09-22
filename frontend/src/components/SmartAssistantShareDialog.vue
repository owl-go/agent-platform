<script setup lang="ts">
import { computed, inject, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type SmartAssistant, type SmartAssistantInput } from "../api/client";

const props = defineProps<{ modelValue: boolean; assistant?: SmartAssistant }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean]; updated: [assistant: SmartAssistant]; error: [message: string] }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const saving = ref(false);
const shareToken = ref("");
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
    service_goal: assistant.service_goal,
    answer_scope: assistant.answer_scope,
    operating_rules: assistant.operating_rules,
    response_style: assistant.response_style,
    knowledge_base_ids: assistant.knowledge_base_ids,
    expert_id: assistant.expert_id,
    expert_team_id: assistant.expert_team_id,
    digital_human_id: assistant.digital_human_id || undefined,
    state: assistant.state,
    share: { enabled: assistant.share.enabled, allowed_origins: assistant.share.allowed_origins ?? [], width: assistant.share.width || "100%", height: assistant.share.height || 600, free_text_enabled: assistant.share.free_text_enabled ?? false, daily_call_limit: assistant.share.daily_call_limit ?? 0 },
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
  saving.value = true;
  try {
    emit("updated", await api.updateSmartAssistant(assistant.id, payload, assistant.version));
    open.value = false;
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
</script>

<template>
  <el-dialog v-model="open" class="application-share-dialog" :title="t('aiApplications.share.title')" width="min(620px, 92vw)" destroy-on-close>
    <el-form v-if="assistant" label-position="top">
      <el-form-item :label="t('aiApplications.share.enabled')"><el-switch v-model="assistant.share.enabled" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.width')"><el-input v-model="assistant.share.width" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.height')"><el-input-number v-model="assistant.share.height" :min="400" :max="1600" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.allowedOrigins')"><el-input v-model="allowedOrigins" type="textarea" :rows="3" :placeholder="t('aiApplications.share.allowedOriginsPlaceholder')" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.dailyLimit')"><el-input-number v-model="assistant.share.daily_call_limit" :min="0" :max="100000" /></el-form-item>
      <el-form-item :label="t('aiApplications.share.freeText')"><el-switch v-model="assistant.share.free_text_enabled" /></el-form-item>
      <el-button :disabled="!assistant.share.enabled" @click="regenerateToken">{{ t('aiApplications.share.regenerate') }}</el-button>
      <el-input v-if="shareToken" class="share-snippet" :model-value="embedSnippet" readonly type="textarea" :rows="3" />
    </el-form>
    <template #footer><el-button @click="open = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="saving" @click="saveShare">{{ t('common.save') }}</el-button></template>
  </el-dialog>
</template>
