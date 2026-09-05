<script setup lang="ts">
import { computed, inject, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type ModelCreditRate, type RedemptionCodeBatch, type RedemptionCodeStatus } from "../api/client";

const props = defineProps<{ section: "rates" | "codes" }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const rates = ref<ModelCreditRate[]>([]);
const error = ref("");
const rateForm = ref({ provider_type: "", api_protocol: "", provider_model_id: "", input: 1, output: 1, fallback: 10 });
const codeForm = ref({ count: 1, value: 100, expires_at: "" });
const createdBatch = ref<RedemptionCodeBatch>();
const codeStatuses = ref<RedemptionCodeStatus[]>([]);
const codesNextCursor = ref("");
const currentRates = computed(() => rates.value.filter((item) => !item.superseded_at));
const credits = (value: number | undefined) => (Number(value ?? 0) / 100).toFixed(2);

async function refreshRates() { try { rates.value = await api.listModelCreditRates(); } catch { error.value = t("errors.generic"); } }
async function createRate() {
  try {
    const exact = Boolean(rateForm.value.provider_type || rateForm.value.api_protocol || rateForm.value.provider_model_id);
    const existing = currentRates.value.find((item) => exact ? item.provider_type === rateForm.value.provider_type && item.api_protocol === rateForm.value.api_protocol && item.provider_model_id === rateForm.value.provider_model_id : !item.provider_type);
    await api.createModelCreditRate({ ...(exact ? { provider_type: rateForm.value.provider_type, api_protocol: rateForm.value.api_protocol, provider_model_id: rateForm.value.provider_model_id } : {}), input_multiplier_micros: Math.round(rateForm.value.input * 1_000_000), output_multiplier_micros: Math.round(rateForm.value.output * 1_000_000), fallback_hundredths: Math.round(rateForm.value.fallback * 100), expected_revision_id: existing?.revision_id });
    await refreshRates();
  } catch { error.value = t("errors.validation"); }
}
async function createCodes() { try { createdBatch.value = await api.createRedemptionCodeBatch(codeForm.value.count, Math.round(codeForm.value.value * 100), codeForm.value.expires_at ? new Date(codeForm.value.expires_at).toISOString() : undefined); } catch { error.value = t("errors.validation"); } }
async function refreshCodes(cursor = "") { try { const page = await api.listRedemptionCodes(cursor); codeStatuses.value = cursor ? [...codeStatuses.value, ...(page.items ?? [])] : (page.items ?? []); codesNextCursor.value = page.next_cursor ?? ""; } catch { error.value = t("errors.generic"); } }
async function voidCode(id: string) { try { const updated = await api.voidRedemptionCode(id); codeStatuses.value = codeStatuses.value.map((item) => item.id === id ? updated : item); } catch { error.value = t("errors.generic"); } }
async function copyCodes() { if (createdBatch.value) await navigator.clipboard.writeText(createdBatch.value.codes.map((item) => item.plaintext).join("\n")); }
function downloadCodes() {
  if (!createdBatch.value) return;
  const rows = ["code,value", ...createdBatch.value.codes.map((item) => `${item.plaintext},${credits(createdBatch.value?.value_hundredths)}`)];
  const url = URL.createObjectURL(new Blob([rows.join("\n")], { type: "text/csv;charset=utf-8" }));
  const link = document.createElement("a"); link.href = url; link.download = `redemption-codes-${createdBatch.value.id}.csv`; link.click(); URL.revokeObjectURL(url);
}
onMounted(() => props.section === "rates" ? refreshRates() : refreshCodes());
watch(() => props.section, (section) => section === "rates" ? refreshRates() : refreshCodes());
</script>

<template>
  <p v-if="error" class="credit-error" role="alert">{{ error }}</p>
  <section v-if="section === 'rates'" class="admin-credit-grid">
    <el-card shadow="never"><template #header><strong>{{ t('users.rateEditor') }}</strong></template><el-form label-position="top"><div class="form-grid"><el-form-item :label="t('users.providerType')"><el-input v-model="rateForm.provider_type" /></el-form-item><el-form-item :label="t('users.protocol')"><el-input v-model="rateForm.api_protocol" /></el-form-item><el-form-item :label="t('users.modelId')"><el-input v-model="rateForm.provider_model_id" /></el-form-item><el-form-item :label="t('users.inputMultiplier')"><el-input-number v-model="rateForm.input" :min="0" :precision="2" /></el-form-item><el-form-item :label="t('users.outputMultiplier')"><el-input-number v-model="rateForm.output" :min="0" :precision="2" /></el-form-item><el-form-item :label="t('users.fallback')"><el-input-number v-model="rateForm.fallback" :min="0" :precision="2" /></el-form-item></div><el-button type="primary" @click="createRate">{{ t('common.save') }}</el-button></el-form></el-card>
    <el-card shadow="never"><template #header><strong>{{ t('users.rateHistory') }}</strong></template><div class="rate-list"><article v-for="rate in rates" :key="rate.revision_id"><span><strong>{{ rate.provider_model_id || t('users.platformDefault') }}</strong><small>{{ rate.provider_type ? `${rate.provider_type} · ${rate.api_protocol}` : rate.revision_id }}</small></span><span>↑ {{ (Number(rate.input_multiplier_micros) / 1e6).toFixed(2) }} · ↓ {{ (Number(rate.output_multiplier_micros) / 1e6).toFixed(2) }} · {{ credits(rate.fallback_hundredths) }}</span></article></div></el-card>
  </section>
  <section v-else class="admin-credit-grid"><el-card shadow="never"><template #header><strong>{{ t('users.createCodes') }}</strong></template><el-form label-position="top"><el-form-item :label="t('users.codeCount')"><el-input-number v-model="codeForm.count" :min="1" :max="100" /></el-form-item><el-form-item :label="t('users.codeValue')"><el-input-number v-model="codeForm.value" :min="0.01" :precision="2" /></el-form-item><el-form-item :label="t('users.expiry')"><el-input v-model="codeForm.expires_at" type="datetime-local" /></el-form-item><el-button type="primary" @click="createCodes">{{ t('users.generateCodes') }}</el-button></el-form></el-card><el-card v-if="createdBatch" class="one-time-codes" shadow="never"><template #header><strong>{{ t('users.copyCodesNow') }}</strong></template><code v-for="item in createdBatch.codes" :key="item.id">{{ item.plaintext }}</code><footer><el-button @click="copyCodes">{{ t('common.copy') }}</el-button><el-button @click="downloadCodes">CSV</el-button></footer></el-card><el-card shadow="never"><template #header><strong>{{ t('users.codeStatus') }}</strong></template><div class="rate-list"><article v-for="item in codeStatuses" :key="item.id"><span><strong>{{ item.identifier }}</strong><small>{{ credits(item.value_hundredths) }} · {{ t(`users.codeState.${item.state}`) }}</small></span><el-button v-if="item.state === 'available'" size="small" @click="voidCode(item.id)">{{ t('users.voidCode') }}</el-button></article></div><el-button v-if="codesNextCursor" @click="refreshCodes(codesNextCursor)">{{ t('common.loadMore') }}</el-button></el-card></section>
</template>
