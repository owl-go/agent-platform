<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type UserAccount } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import AdminCredits from "../components/AdminCredits.vue";
const api = inject(platformApiKey)!;
const { t } = useI18n(); const router = useRouter(); const users = ref<UserAccount[]>([]); const showCreate = ref(false); const pendingReset = ref<UserAccount>(); const revealed = ref(""); const error = ref(""); const form = ref({ username: "", email: "", display_name: "" });
const tab = ref<"users" | "rates" | "codes">("users"); const creditUser = ref<UserAccount>(); const creditForm = ref({ daily: 600, adjustment: 0, reason: "" }); const credits = (value?: number) => (Number(value ?? 0) / 100).toFixed(2);
onMounted(refresh); async function refresh() { try { users.value = await api.listUsers(); } catch { error.value = t("errors.generic"); } }
async function create() { try { const result = await api.createUser(form.value); revealed.value = result.temporary_password; showCreate.value = false; form.value = { username: "", email: "", display_name: "" }; await refresh(); } catch { error.value = t("errors.validation"); } }
async function toggle(item: UserAccount) { try { await api.setUserEnabled(item.id, !item.enabled, item.version); await refresh(); } catch { error.value = t("errors.conflict"); } }
async function reset() { if (!pendingReset.value) return; try { revealed.value = (await api.resetUserPassword(pendingReset.value.id)).temporary_password; pendingReset.value = undefined; } catch { error.value = t("errors.generic"); } }
async function copy() { await navigator.clipboard.writeText(revealed.value); }
function editCredits(item: UserAccount) { creditUser.value = item; creditForm.value = { daily: Number(item.credit_balance?.daily_allocation_hundredths ?? 60_000) / 100, adjustment: 0, reason: "" }; }
async function saveCredits() { if (!creditUser.value) return; try { await api.configureUserDailyCredits(creditUser.value.id, Math.round(creditForm.value.daily * 100)); if (creditForm.value.adjustment !== 0) await api.adjustUserCredits(creditUser.value.id, Math.round(creditForm.value.adjustment * 100), creditForm.value.reason); creditUser.value = undefined; await refresh(); } catch { error.value = t("errors.validation"); } }
</script>
<template>
  <section class="page-surface">
    <header class="page-header"><div><el-button class="back-link" text @click="router.push('/sessions')">← {{ t('common.back') }}</el-button><h1>{{ t('users.title') }}</h1><p>{{ t('users.subtitle') }}</p></div><el-button v-if="tab === 'users'" type="primary" @click="showCreate = true">＋ {{ t('users.create') }}</el-button></header>
    <div class="admin-tabs" role="tablist"><button v-for="item in (['users','rates','codes'] as const)" :key="item" :class="{ active: tab === item }" @click="tab = item">{{ t(`users.tabs.${item}`) }}</button></div>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <el-card v-if="revealed" class="secret-banner" shadow="never"><div><p class="eyebrow">{{ t('users.temporary') }}</p><code>{{ revealed }}</code><small>{{ t('users.forceChange') }}</small></div><el-button size="small" @click="copy">{{ t('common.copy') }}</el-button><el-button size="small" text @click="revealed = ''">×</el-button></el-card>
    <el-table v-if="tab === 'users'" :data="users" class="user-table" stripe>
      <el-table-column :label="t('users.user')" min-width="220"><template #default="{ row: item }"><div class="user-identity"><el-avatar :size="36">{{ (item as UserAccount).display_name.slice(0, 2).toUpperCase() }}</el-avatar><span><strong>{{ (item as UserAccount).display_name }}</strong><small>@{{ (item as UserAccount).username }}<template v-if="(item as UserAccount).administrator"> · {{ t('users.administrator') }}</template></small></span></div></template></el-table-column>
      <el-table-column :label="t('credits.balance')" width="120"><template #default="{ row: item }">✧ {{ credits((item as UserAccount).credit_balance?.total_hundredths) }}</template></el-table-column>
      <el-table-column :label="t('credits.todayConsumed')" width="120"><template #default="{ row: item }">{{ credits((item as UserAccount).credit_balance?.today_consumed_hundredths) }}</template></el-table-column>
      <el-table-column :label="t('users.dailyLimit')" width="120"><template #default="{ row: item }">{{ credits((item as UserAccount).credit_balance?.daily_allocation_hundredths) }}</template></el-table-column>
      <el-table-column :label="t('users.status')" width="120"><template #default="{ row: item }"><el-tag :type="(item as UserAccount).enabled ? 'success' : 'danger'" effect="light">{{ (item as UserAccount).enabled ? t('users.enabled') : t('users.disabled') }}</el-tag></template></el-table-column>
      <el-table-column :label="t('users.created')" width="140"><template #default="{ row: item }">{{ new Date((item as UserAccount).created_at).toLocaleDateString() }}</template></el-table-column>
      <el-table-column width="260" fixed="right"><template #default="{ row: item }"><el-space><el-button size="small" @click="editCredits(item as UserAccount)">{{ t('users.credits') }}</el-button><template v-if="!(item as UserAccount).administrator"><el-button size="small" @click="pendingReset = item as UserAccount">{{ t('users.reset') }}</el-button><el-button size="small" :type="(item as UserAccount).enabled ? 'danger' : 'success'" plain @click="toggle(item as UserAccount)">{{ (item as UserAccount).enabled ? t('users.disabled') : t('users.enabled') }}</el-button></template></el-space></template></el-table-column>
    </el-table>
    <AdminCredits v-else :section="tab" />
  </section>
  <el-dialog v-model="showCreate" width="min(480px, calc(100vw - 32px))" align-center><template #header><h2>{{ t('users.create') }}</h2></template><el-form :model="form" label-position="top"><el-form-item :label="t('users.username')" required><el-input v-model="form.username" /></el-form-item><el-form-item :label="t('users.displayName')" required><el-input v-model="form.display_name" /></el-form-item><el-form-item :label="t('users.email')" required><el-input v-model="form.email" type="email" /></el-form-item></el-form><template #footer><el-button @click="showCreate = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :disabled="!form.username || !form.display_name || !form.email" @click="create">{{ t('users.create') }}</el-button></template></el-dialog>
  <el-dialog :model-value="Boolean(creditUser)" width="min(480px, calc(100vw - 32px))" align-center @close="creditUser = undefined"><template #header><h2>{{ t('users.creditSettings') }} · {{ creditUser?.display_name }}</h2></template><el-form label-position="top"><el-form-item :label="t('users.dailyLimit')"><el-input-number v-model="creditForm.daily" :min="0" :precision="2" /></el-form-item><el-form-item :label="t('users.adjustment')"><el-input-number v-model="creditForm.adjustment" :precision="2" /></el-form-item><el-form-item :label="t('users.adjustmentReason')" :required="creditForm.adjustment !== 0"><el-input v-model="creditForm.reason" /></el-form-item></el-form><template #footer><el-button @click="creditUser = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :disabled="creditForm.adjustment !== 0 && !creditForm.reason.trim()" @click="saveCredits">{{ t('common.save') }}</el-button></template></el-dialog>
  <ConfirmDialog :open="Boolean(pendingReset)" :title="t('users.reset')" :message="pendingReset ? `${t('users.reset')} — ${pendingReset.username}?` : ''" :confirm-label="t('users.reset')" :cancel-label="t('common.cancel')" danger @cancel="pendingReset = undefined" @confirm="reset" />
</template>
