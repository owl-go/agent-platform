<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { CreditConsumption } from "../api/client";

const props = defineProps<{ value?: CreditConsumption; state?: string }>();
const { t } = useI18n();
const credits = (hundredths?: number) => (Number(hundredths ?? 0) / 100).toFixed(2);
const stopped = computed(() => props.state === "failed" || props.state === "cancelled");
const hasCharge = computed(() => Number(props.value?.total_hundredths ?? 0) > 0 || Boolean(props.value?.stages?.some((stage) => Number(stage.amount_hundredths) > 0)));
</script>

<template>
  <div v-if="value && (!stopped || hasCharge)" class="credit-consumption">
    <p>{{ t('credits.consumed', { value: credits(value.total_hundredths) }) }}<small v-if="stopped"> · {{ t('credits.consumedBeforeStop') }}</small></p>
    <details v-if="(value.stages?.length ?? 0) > 1"><summary>{{ t('credits.stageSummary') }}</summary><ol><li v-for="stage in value.stages" :key="stage.stage_position"><span>{{ t('credits.stage', { position: stage.stage_position }) }}</span><strong>✧ {{ credits(stage.amount_hundredths) }}</strong><small>{{ t(stage.estimated ? 'credits.estimatedCharge' : 'credits.measuredCharge') }}</small></li></ol></details>
  </div>
  <p v-else-if="stopped" class="credit-consumption credit-consumption-empty">{{ t('credits.notCharged') }}</p>
</template>
