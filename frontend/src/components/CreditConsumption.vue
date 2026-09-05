<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { CreditConsumption } from "../api/client";

defineProps<{ value?: CreditConsumption }>();
const { t } = useI18n();
const credits = (hundredths?: number) => (Number(hundredths ?? 0) / 100).toFixed(2);
</script>

<template>
  <p v-if="value" class="credit-consumption">{{ t('credits.consumed', { value: credits(value.total_hundredths) }) }}</p>
</template>
