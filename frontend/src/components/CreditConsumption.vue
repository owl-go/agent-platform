<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { CreditConsumption } from "../api/client";

defineProps<{ value?: CreditConsumption }>();
const { t } = useI18n();
const credits = (hundredths: number) => (Number(hundredths) / 100).toFixed(2);
const multiplier = (micros: number) => (Number(micros) / 1_000_000).toFixed(2);
</script>

<template>
  <details v-if="value" class="credit-consumption">
    <summary>{{ t('credits.consumed', { value: credits(value.total_hundredths) }) }}</summary>
    <div class="credit-stage-breakdown">
      <article v-for="stage in value.stages" :key="stage.stage_position">
        <strong>{{ value.stages.length > 1 ? `${stage.stage_position}. ` : '' }}{{ stage.provider_model }}</strong>
        <span v-if="stage.estimated">{{ t('credits.estimatedFallback', { value: credits(stage.fallback_hundredths) }) }}</span>
        <template v-else>
          <span>{{ t('credits.inputUsage', { tokens: Number(stage.input_tokens).toLocaleString(), rate: multiplier(stage.input_multiplier_micros) }) }}</span>
          <span>{{ t('credits.outputUsage', { tokens: Number(stage.output_tokens).toLocaleString(), rate: multiplier(stage.output_multiplier_micros) }) }}</span>
        </template>
        <small>{{ t('credits.stageTotal', { value: credits(stage.amount_hundredths) }) }}</small>
      </article>
    </div>
  </details>
</template>
