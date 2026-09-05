<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
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
      <section class="credit-hero"><span class="credit-spark">✧</span><div><small>{{ t('credits.balance') }}</small><strong>{{ credits(balance.total_hundredths) }}</strong></div></section>
      <dl class="credit-grid">
        <div><dt>{{ t('credits.dailyRemaining') }}</dt><dd>{{ credits(balance.daily_remaining_hundredths) }}</dd></div>
        <div><dt>{{ t('credits.persistent') }}</dt><dd>{{ credits(balance.persistent_hundredths) }}</dd></div>
        <div><dt>{{ t('credits.todayConsumed') }}</dt><dd>{{ credits(balance.today_consumed_hundredths) }}</dd></div>
        <div><dt>{{ t('credits.nextReset') }}</dt><dd>{{ new Date(balance.next_allocation_at).toLocaleString() }}</dd></div>
      </dl>
      <form class="credit-redeem" @submit.prevent="redeem"><el-input v-model="code" :placeholder="t('credits.codePlaceholder')" autocomplete="off" /><el-button native-type="submit" type="primary" :loading="loading">{{ t('credits.redeem') }}</el-button></form>
      <p v-if="error" class="credit-error" role="alert">{{ error }}</p>
      <h3 class="credit-ledger-title">{{ t('credits.ledger') }}</h3>
      <div class="credit-ledger">
        <article v-for="entry in ledger" :key="entry.id"><span><strong>{{ t(`credits.entry.${entry.type}`) }}</strong><small>{{ new Date(entry.created_at).toLocaleString() }}<template v-if="entry.reason"> · {{ entry.reason }}</template></small></span><b :class="{ negative: Number(entry.amount_hundredths) < 0 }">{{ Number(entry.amount_hundredths) > 0 ? '+' : '' }}{{ credits(entry.amount_hundredths) }}</b></article>
      </div>
      <el-button v-if="nextCursor" :loading="loading" @click="loadMore">{{ t('common.loadMore') }}</el-button>
    </template>
  </el-drawer>
</template>
