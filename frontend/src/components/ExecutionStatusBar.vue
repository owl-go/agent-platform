<script setup lang="ts">
import { computed } from "vue";
import { Square } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import type { CreditConsumption } from "../api/client";

const props = defineProps<{
  state: string;
  elapsedMs?: number;
  model?: string;
  creditConsumption?: CreditConsumption;
  canStop?: boolean;
  stopping?: boolean;
}>();
const emit = defineEmits<{ stop: [] }>();
const { t, locale } = useI18n();

const normalizedState = computed(() => {
  if (props.state === "generating") return "running";
  if (props.state === "completed") return "succeeded";
  return props.state;
});
const stateLabel = computed(() => {
  if (normalizedState.value === "succeeded") return t("common.success");
  if (normalizedState.value === "failed") return t("common.failed");
  if (normalizedState.value === "cancelled") return t("common.cancelled");
  if (normalizedState.value === "waiting_for_user") return t("common.waitingForUser");
  if (normalizedState.value === "queued") return t("sessions.thinking");
  return t("common.running");
});
const tokenCount = computed(() => props.creditConsumption?.stages.reduce((total, stage) => total + stage.input_tokens + stage.output_tokens, 0));
const formattedTokens = computed(() => {
  const value = tokenCount.value;
  if (value === undefined) return "";
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}m`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
});
</script>

<template>
  <div class="execution-status-bar" :class="`is-${normalizedState}`" role="status" aria-live="polite">
    <span class="execution-status-dot" aria-hidden="true"></span>
    <strong>{{ stateLabel }}</strong>
    <span v-if="elapsedMs !== undefined" class="execution-status-meta">{{ formatDuration(elapsedMs, locale as SupportedLocale) }}</span>
    <span v-if="formattedTokens" class="execution-status-meta">{{ formattedTokens }} tokens</span>
    <span v-if="model" class="execution-status-model" :title="model">{{ model }}</span>
    <span class="execution-status-spacer"></span>
    <el-button v-if="canStop" class="execution-status-stop" :loading="stopping" @click="emit('stop')"><Square :size="12" aria-hidden="true" />{{ stopping ? t('sessions.stopping') : t('sessions.stopGeneration') }}</el-button>
  </div>
</template>
