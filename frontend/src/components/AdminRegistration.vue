<script setup lang="ts">
import { saveErrorMessage } from "../api/saveErrors";
import { inject, onMounted, onUnmounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";
import { platformApiKey, type RegistrationMethod, type RegistrationMethodInput, type RegistrationProvider } from "../api/client";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const available = ref(false);
const loading = ref(true);
const error = ref("");
const methods = ref<RegistrationMethod[]>([]);
const forms = ref<Partial<Record<RegistrationProvider, RegistrationMethodInput>>>({});
const saving = ref<RegistrationProvider>();
const controller = new AbortController();
onMounted(load);
onUnmounted(() => controller.abort());
function formFor(item: RegistrationMethod): RegistrationMethodInput {
  return { enabled: Boolean(item.enabled), app_id: item.app_id ?? "", app_secret: "", official_account_id: item.official_account_id ?? "", verification_token: "", encoding_aes_key: "", expected_version: Number(item.version ?? 0), reason: "" };
}
async function load() {
  loading.value = true; error.value = "";
  try {
    const result = await api.getRegistrationSettings(controller.signal);
    available.value = result.available;
    methods.value = result.items;
    forms.value = Object.fromEntries(result.items.map((item) => [item.provider, formFor(item)]));
  } catch { if (!controller.signal.aborted) error.value = t("registration.loadFailed"); }
  finally { loading.value = false; }
}
async function save(item: RegistrationMethod) {
  const form = forms.value[item.provider];
  if (!form || !form.reason.trim() || saving.value) return;
  saving.value = item.provider; error.value = "";
  try {
    const saved = await api.updateRegistrationMethod(item.provider, { ...form, reason: form.reason.trim() }, controller.signal);
    methods.value = methods.value.map((entry) => entry.provider === item.provider ? saved : entry);
    forms.value[item.provider] = formFor(saved);
    ElMessage.success(t("registration.saved"));
  } catch (cause) {
    // A failed Keycloak synchronization may have persisted a closed pending
    // version. Reload its CAS before retrying, and discard typed secrets.
    await load(); error.value = saveErrorMessage(cause, t, "registration.failed");
  } finally { saving.value = undefined; }
}
</script>

<template>
  <section class="registration-settings" :aria-label="t('registration.title')">
    <h2>{{ t('registration.title') }}</h2><p class="registration-hint">{{ t('registration.hint') }}</p>
    <el-skeleton v-if="loading" :rows="5" animated />
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-button v-if="error" @click="load">{{ t('registration.retry') }}</el-button>
    <el-alert v-if="!loading && !error && !available" :title="t('registration.unavailable')" type="info" :closable="false" />
    <template v-if="!loading && available">
      <el-card v-for="item in methods" :key="item.provider" shadow="never">
        <template #header><strong>{{ t(`registration.${item.provider}`) }}</strong></template>
        <p class="registration-hint">{{ t(item.provider === 'feishu' ? 'registration.feishuHint' : 'registration.wechatHint') }}</p>
        <el-alert v-if="item.enabled && !item.ready" :title="t('registration.pending')" type="warning" :closable="false" />
        <el-form v-if="forms[item.provider]" label-position="top" @submit.prevent="save(item)">
          <el-form-item><el-switch v-model="forms[item.provider]!.enabled" :aria-label="t('registration.enabled')" :active-text="t('registration.enabled')" /></el-form-item>
          <div class="registration-fields">
            <el-form-item :label="t('registration.appID')"><el-input v-model="forms[item.provider]!.app_id" autocomplete="off" :maxlength="128" /></el-form-item>
            <el-form-item :label="t('registration.appSecret')"><el-input v-model="forms[item.provider]!.app_secret" type="password" autocomplete="new-password" :maxlength="512" :placeholder="item.app_secret_configured && forms[item.provider]!.app_id === item.app_id ? t('registration.configured') : ''" /></el-form-item>
            <template v-if="item.provider === 'wechat_official'">
              <el-form-item :label="t('registration.originalID')"><el-input v-model="forms[item.provider]!.official_account_id" autocomplete="off" :maxlength="128" /></el-form-item>
              <el-form-item :label="t('registration.token')"><el-input v-model="forms[item.provider]!.verification_token" type="password" autocomplete="new-password" :maxlength="128" :placeholder="item.verification_token_configured && forms[item.provider]!.app_id === item.app_id ? t('registration.configured') : ''" /></el-form-item>
              <el-form-item :label="t('registration.aesKey')"><el-input v-model="forms[item.provider]!.encoding_aes_key" type="password" autocomplete="new-password" :maxlength="43" :placeholder="item.encoding_aes_key_configured && forms[item.provider]!.app_id === item.app_id ? t('registration.configured') : ''" /></el-form-item>
            </template>
          </div>
          <el-form-item :label="t('registration.callback')"><el-input :model-value="item.callback_url" readonly /><small>{{ t('registration.callbackHint') }}</small></el-form-item>
          <p v-if="item.tenant_key" class="registration-hint">{{ t('registration.tenant') }}: {{ item.tenant_key }}</p>
          <el-form-item :label="t('registration.reason')"><el-input v-model="forms[item.provider]!.reason" :maxlength="500" /></el-form-item>
          <el-button type="primary" native-type="submit" :loading="saving === item.provider" :disabled="Boolean(saving) || !forms[item.provider]!.reason.trim()">{{ t('registration.save') }}</el-button>
        </el-form>
      </el-card>
    </template>
  </section>
</template>

<style scoped>
.registration-settings { display: grid; gap: var(--aw-space-4); }
.registration-settings h2, .registration-hint { margin: 0; }
.registration-hint, small { color: var(--aw-n7); font-size: var(--aw-font-size-body); line-height: 1.6; }
.registration-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 var(--aw-space-4); }
.registration-settings :deep(.el-alert), .registration-hint { margin-bottom: var(--aw-space-3); }
@media (max-width: 640px) { .registration-fields { grid-template-columns: minmax(0, 1fr); } }
</style>
