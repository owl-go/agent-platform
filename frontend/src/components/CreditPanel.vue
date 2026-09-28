<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type CreditBalance, type CreditLedgerEntry } from "../api/client";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: []; updated: [CreditBalance] }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const balance = ref<CreditBalance>();
const ledger = ref<CreditLedgerEntry[]>([]);
const nextCursor = ref("");
const code = ref("");
const loading = ref(false);
const error = ref("");
const credits = (hundredths: number | undefined) => (Number(hundredths ?? 0) / 100).toFixed(2);
const usagePercent = computed(() => balance.value?.daily_allocation_hundredths ? Math.min(100, Math.round(Number(balance.value.today_consumed_hundredths) / Number(balance.value.daily_allocation_hundredths) * 100)) : 0);
const budgetState = computed(() => {
  if (!balance.value || balance.value.daily_allocation_hundredths <= 0) return "";
  if (Number(balance.value.today_consumed_hundredths) >= Number(balance.value.daily_allocation_hundredths)) return "exhausted";
  if (usagePercent.value >= Number(balance.value.warning_threshold_percent || 80)) return "warning";
  return "";
});

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [nextBalance, page] = await Promise.all([api.getCreditBalance(), api.listCreditLedger()]);
    balance.value = nextBalance;
    ledger.value = page.items ?? [];
    nextCursor.value = page.next_cursor ?? "";
  } catch { error.value = t("errors.generic"); }
  finally { loading.value = false; }
}

async function loadMore() {
  if (!nextCursor.value || loading.value) return;
  loading.value = true;
  try {
    const page = await api.listCreditLedger(nextCursor.value);
    ledger.value.push(...(page.items ?? []));
    nextCursor.value = page.next_cursor ?? "";
  } catch { error.value = t("errors.generic"); }
  finally { loading.value = false; }
}

async function redeem() {
  if (!code.value.trim()) return;
  loading.value = true;
  error.value = "";
  try {
    balance.value = await api.redeemCreditCode(code.value.trim());
    code.value = "";
    emit("updated", balance.value);
    ledger.value = (await api.listCreditLedger()).items ?? [];
  } catch { error.value = t("credits.codeUnavailable"); }
  finally { loading.value = false; }
}

onMounted(() => { if (props.open) void load(); });
defineExpose({ load });
</script>

<template>
  <el-drawer :model-value="open" :title="t('credits.title')" size="min(440px, 100vw)" @open="load" @close="emit('close')">
    <el-skeleton v-if="loading && !balance" :rows="6" animated />
    <template v-else-if="balance">
      <section class="credit-hero"><span class="credit-spark">✧</span><div><small>{{ t('credits.available') }}</small><strong>{{ credits(balance.available_hundredths) }}</strong></div></section>
      <el-alert v-if="budgetState" :type="budgetState === 'exhausted' ? 'error' : 'warning'" :closable="false" :title="budgetState === 'warning' ? t('credits.warning', { percent: balance.warning_threshold_percent || 80 }) : t('credits.exhausted')" :description="t('credits.contactAdministrator')" />
      <dl class="credit-grid">
        <div><dt>{{ t('credits.dailyRemaining') }}</dt><dd>{{ credits(balance.daily_remaining_hundredths) }}</dd></div>
        <div><dt>{{ t('credits.reserved') }}</dt><dd>{{ credits(balance.reserved_hundredths) }}</dd></div>
        <div><dt>{{ t('credits.todayConsumed') }}</dt><dd>{{ credits(balance.today_consumed_hundredths) }}</dd></div>
        <div><dt>{{ t('credits.nextReset') }}</dt><dd>{{ new Date(balance.next_allocation_at).toLocaleString() }}</dd></div>
      </dl>
      <el-progress v-if="balance.daily_allocation_hundredths > 0" :percentage="usagePercent" :status="budgetState === 'exhausted' ? 'exception' : budgetState === 'warning' ? 'warning' : 'success'" />
      <form v-if="balance.redemption_codes_enabled" class="credit-redeem" @submit.prevent="redeem"><el-input v-model="code" :placeholder="t('credits.codePlaceholder')" autocomplete="off" /><el-button native-type="submit" type="primary" :loading="loading">{{ t('credits.redeem') }}</el-button></form>
      <p v-if="error" class="credit-error" role="alert">{{ error }}</p>
      <h3 class="credit-ledger-title">{{ t('credits.ledger') }}</h3>
      <div class="credit-ledger">
        <article v-for="entry in ledger" :key="entry.id"><span><strong>{{ t(`credits.entry.${entry.type}`) }}</strong><small>{{ new Date(entry.created_at).toLocaleString() }}<template v-if="entry.reason"> · {{ entry.reason }}</template></small></span><b :class="{ negative: Number(entry.amount_hundredths) < 0 }">{{ Number(entry.amount_hundredths) > 0 ? '+' : '' }}{{ credits(entry.amount_hundredths) }}</b></article>
      </div>
      <el-button v-if="nextCursor" :loading="loading" @click="loadMore">{{ t('common.loadMore') }}</el-button>
    </template>
  </el-drawer>
</template>
