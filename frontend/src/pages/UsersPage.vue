<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type CreditPolicy, type GovernanceAuditEvent, type IdentityGroup, type UserAccount } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import AdminRegistration from "../components/AdminRegistration.vue";
import AdminCredits from "../components/AdminCredits.vue";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const router = useRouter();
const users = ref<UserAccount[]>([]);
const groups = ref<IdentityGroup[]>([]);
const auditEvents = ref<GovernanceAuditEvent[]>([]);
const policy = ref<CreditPolicy>();
const error = ref("");
const busy = ref(false);
const revealed = ref("");
const showCreate = ref(false);
const pendingReset = ref<UserAccount>();
const statusTarget = ref<UserAccount>();
const roleTarget = ref<UserAccount>();
const budgetTarget = ref<IdentityGroup>();
const transferGroup = ref<IdentityGroup>();
const form = ref({ username: "", email: "", display_name: "" });
const statusReason = ref("");
const roleForm = ref({ administrator: false, resourcePublisher: false, reason: "" });
const budgetForm = ref({ enabled: false, daily: 0, reason: "" });
const transferForm = ref({ from: "", to: "", reason: "" });
const creditUser = ref<UserAccount>();
const creditForm = ref({ daily: 600, adjustment: 0, reason: "" });
const policyForm = ref({ defaultDaily: 600, warning: 80, redemptionCodes: false });
const tab = ref<"users" | "groups" | "audit" | "rates" | "codes" | "registration">("users");
const tabs = computed(() => policy.value?.redemption_codes_enabled ? (["users", "registration", "groups", "audit", "rates", "codes"] as const) : (["users", "registration", "groups", "audit", "rates"] as const));
const credits = (value?: number) => (Number(value ?? 0) / 100).toFixed(2);
const utilization = (item: UserAccount) => item.credit_balance?.daily_allocation_hundredths ? Math.round(Number(item.credit_balance.today_consumed_hundredths) / Number(item.credit_balance.daily_allocation_hundredths) * 100) : 0;
const groupMembers = (group: IdentityGroup) => users.value.filter((user) => user.groups?.some((item) => item.id === group.id));
const transferSources = computed(() => users.value.filter((user) => !user.enabled));
const transferTargets = computed(() => transferGroup.value ? groupMembers(transferGroup.value).filter((user) => user.enabled && user.resource_publisher) : []);

onMounted(refresh);

async function refresh() {
  try {
    const [nextUsers, nextPolicy, nextGroups, nextAudit] = await Promise.all([api.listUsers(), api.getCreditPolicy(), api.listIdentityGroups(), api.listGovernanceAuditEvents(100)]);
    users.value = nextUsers;
    policy.value = nextPolicy;
    groups.value = nextGroups;
    auditEvents.value = nextAudit;
    policyForm.value = { defaultDaily: Number(nextPolicy.default_daily_allocation_hundredths) / 100, warning: nextPolicy.warning_threshold_percent, redemptionCodes: nextPolicy.redemption_codes_enabled };
    if (!nextPolicy.redemption_codes_enabled && tab.value === "codes") tab.value = "users";
  } catch { error.value = t("errors.generic"); }
}

async function create() {
  try {
    const result = await api.createUser(form.value);
    revealed.value = result.temporary_password;
    showCreate.value = false;
    form.value = { username: "", email: "", display_name: "" };
    await refresh();
  } catch { error.value = t("errors.validation"); }
}

function requestStatusChange(item: UserAccount) { statusTarget.value = item; statusReason.value = ""; }
async function saveStatus() {
  const target = statusTarget.value;
  if (!target || !statusReason.value.trim()) return;
  busy.value = true;
  try { await api.setUserEnabled(target.id, !target.enabled, target.version, statusReason.value.trim()); statusTarget.value = undefined; await refresh(); }
  catch { error.value = t("errors.conflict"); }
  finally { busy.value = false; }
}

function editRoles(item: UserAccount) {
  roleTarget.value = item;
  roleForm.value = { administrator: item.administrator, resourcePublisher: Boolean(item.resource_publisher), reason: "" };
}
async function saveRoles() {
  const target = roleTarget.value;
  if (!target || !roleForm.value.reason.trim()) return;
  busy.value = true;
  try {
    await api.setUserRoles(target.id, { administrator: roleForm.value.administrator, resource_publisher: roleForm.value.resourcePublisher, expected_version: target.version, reason: roleForm.value.reason.trim() });
    roleTarget.value = undefined;
    await refresh();
  } catch { error.value = t("errors.conflict"); }
  finally { busy.value = false; }
}

async function reset() {
  if (!pendingReset.value) return;
  try { revealed.value = (await api.resetUserPassword(pendingReset.value.id)).temporary_password; pendingReset.value = undefined; }
  catch { error.value = t("errors.generic"); }
}
async function copy() { await navigator.clipboard.writeText(revealed.value); }

function editCredits(item: UserAccount) {
  creditUser.value = item;
  creditForm.value = { daily: Number(item.credit_balance?.daily_allocation_hundredths ?? 60_000) / 100, adjustment: 0, reason: "" };
}
async function saveCredits() {
  if (!creditUser.value) return;
  try {
    await api.configureUserDailyCredits(creditUser.value.id, Math.round(creditForm.value.daily * 100));
    if (creditForm.value.adjustment !== 0) await api.adjustUserCredits(creditUser.value.id, Math.round(creditForm.value.adjustment * 100), creditForm.value.reason, crypto.randomUUID());
    creditUser.value = undefined;
    await refresh();
  } catch { error.value = t("errors.validation"); }
}
async function savePolicy() {
  if (!policy.value) return;
  try {
    policy.value = await api.updateCreditPolicy({ default_daily_allocation_hundredths: Math.round(policyForm.value.defaultDaily * 100), warning_threshold_percent: policyForm.value.warning, redemption_codes_enabled: policyForm.value.redemptionCodes, version: policy.value.version });
    await refresh();
  } catch { error.value = t("errors.conflict"); }
}

async function syncGroups() {
  busy.value = true;
  try { await api.syncIdentityGroups(); await refresh(); }
  catch { error.value = t("users.groupSyncFailed"); }
  finally { busy.value = false; }
}
function editBudget(group: IdentityGroup) {
  budgetTarget.value = group;
  budgetForm.value = { enabled: group.daily_credit_limit_hundredths !== undefined, daily: Number(group.daily_credit_limit_hundredths ?? 0) / 100, reason: "" };
}
async function saveBudget() {
  const target = budgetTarget.value;
  if (!target || !budgetForm.value.reason.trim()) return;
  busy.value = true;
  try {
    await api.updateIdentityGroupBudget(target.id, { daily_credit_limit_hundredths: budgetForm.value.enabled ? Math.round(budgetForm.value.daily * 100) : undefined, expected_version: target.version, reason: budgetForm.value.reason.trim() });
    budgetTarget.value = undefined;
    await refresh();
  } catch { error.value = t("errors.conflict"); }
  finally { busy.value = false; }
}
function openTransfer(group: IdentityGroup) { transferGroup.value = group; transferForm.value = { from: "", to: "", reason: "" }; }
async function transferResources() {
  const group = transferGroup.value;
  if (!group || !transferForm.value.from || !transferForm.value.to || !transferForm.value.reason.trim()) return;
  busy.value = true;
  try {
    const result = await api.transferGroupResources(group.id, { from_user_id: transferForm.value.from, to_user_id: transferForm.value.to, reason: transferForm.value.reason.trim() });
    transferGroup.value = undefined;
    ElMessage.success(t("users.transferCompleted", { count: result.knowledge_base_count }));
    await refresh();
  } catch { error.value = t("errors.validation"); }
  finally { busy.value = false; }
}
</script>

<template>
  <section class="page-surface">
    <header class="page-header"><div><el-button class="back-link" text @click="router.push('/home')">← {{ t('common.back') }}</el-button><h1>{{ t('users.title') }}</h1><p>{{ t('users.subtitle') }}</p></div><div class="header-actions"><el-button v-if="tab === 'groups'" :loading="busy" @click="syncGroups">{{ t('users.syncGroups') }}</el-button><el-button v-if="tab === 'users'" type="primary" @click="showCreate = true">＋ {{ t('users.create') }}</el-button></div></header>
    <div class="admin-tabs" role="tablist"><button v-for="item in tabs" :key="item" :class="{ active: tab === item }" @click="tab = item">{{ t(`users.tabs.${item}`) }}</button></div>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <el-card v-if="revealed" class="secret-banner" shadow="never"><div><p class="eyebrow">{{ t('users.temporary') }}</p><code>{{ revealed }}</code><small>{{ t('users.forceChange') }}</small></div><el-button size="small" @click="copy">{{ t('common.copy') }}</el-button><el-button size="small" text @click="revealed = ''">×</el-button></el-card>

    <AdminRegistration v-if="tab === 'registration'" />
    <template v-if="tab === 'users'">
      <el-card v-if="policy" class="credit-policy-card" shadow="never"><template #header><div><strong>{{ t('users.creditPolicy') }}</strong><small>{{ t('users.creditPolicyHint') }}</small></div></template><el-form inline><el-form-item :label="t('users.defaultDailyLimit')"><el-input-number v-model="policyForm.defaultDaily" :min="0" :precision="2" /></el-form-item><el-form-item :label="t('users.warningThreshold')"><el-input-number v-model="policyForm.warning" :min="1" :max="99" /><span>%</span></el-form-item><el-form-item><el-checkbox v-model="policyForm.redemptionCodes">{{ t('users.enableCodes') }}</el-checkbox></el-form-item><el-button type="primary" @click="savePolicy">{{ t('common.save') }}</el-button></el-form></el-card>
      <el-table :data="users" class="user-table" stripe>
        <el-table-column :label="t('users.user')" min-width="240"><template #default="{ row: item }"><div class="user-identity"><el-avatar :size="36">{{ (item as UserAccount).display_name.slice(0, 2).toUpperCase() }}</el-avatar><span><strong>{{ (item as UserAccount).display_name }}</strong><small>@{{ (item as UserAccount).username }}</small><span class="role-tags"><el-tag v-if="(item as UserAccount).bootstrap_administrator" size="small">{{ t('users.bootstrapAdministrator') }}</el-tag><el-tag v-else-if="(item as UserAccount).administrator" size="small">{{ t('users.administrator') }}</el-tag><el-tag v-if="(item as UserAccount).resource_publisher" size="small" type="success">{{ t('users.resourcePublisher') }}</el-tag></span></span></div></template></el-table-column>
        <el-table-column :label="t('users.departments')" min-width="150"><template #default="{ row: item }"><span>{{ (item as UserAccount).groups?.filter((group) => group.department).map((group) => group.name).join('、') || '—' }}</span></template></el-table-column>
        <el-table-column :label="t('credits.available')" width="125"><template #default="{ row: item }"><div>✧ {{ credits((item as UserAccount).credit_balance?.available_hundredths) }}</div><small v-if="(item as UserAccount).credit_balance?.group_budget" class="budget-note">{{ t('users.groupBudgetBound', { name: (item as UserAccount).credit_balance?.group_budget?.group_name }) }}</small></template></el-table-column>
        <el-table-column :label="t('credits.todayConsumed')" width="170"><template #default="{ row: item }"><span>{{ credits((item as UserAccount).credit_balance?.today_consumed_hundredths) }}</span><el-tag v-if="utilization(item as UserAccount) >= Number(policy?.warning_threshold_percent || 80)" :type="utilization(item as UserAccount) >= 100 ? 'danger' : 'warning'" size="small" class="credit-budget-tag">{{ utilization(item as UserAccount) }}%</el-tag></template></el-table-column>
        <el-table-column :label="t('users.status')" width="110"><template #default="{ row: item }"><el-tag :type="(item as UserAccount).enabled ? 'success' : 'danger'" effect="light">{{ (item as UserAccount).enabled ? t('users.enabled') : t('users.disabled') }}</el-tag></template></el-table-column>
        <el-table-column width="330" fixed="right"><template #default="{ row: item }"><el-space wrap><el-button size="small" @click="editCredits(item as UserAccount)">{{ t('users.credits') }}</el-button><el-button size="small" :disabled="Boolean((item as UserAccount).bootstrap_administrator)" :title="(item as UserAccount).bootstrap_administrator ? t('users.bootstrapLocked') : ''" @click="editRoles(item as UserAccount)">{{ t('users.roles') }}</el-button><el-button v-if="!(item as UserAccount).bootstrap_administrator" size="small" @click="pendingReset = item as UserAccount">{{ t('users.reset') }}</el-button><el-button v-if="!(item as UserAccount).administrator" size="small" :type="(item as UserAccount).enabled ? 'danger' : 'success'" plain @click="requestStatusChange(item as UserAccount)">{{ (item as UserAccount).enabled ? t('users.disabled') : t('users.enabled') }}</el-button></el-space></template></el-table-column>
      </el-table>
    </template>

    <template v-else-if="tab === 'groups'">
      <el-alert :closable="false" type="info" :title="t('users.identityReadOnly')" />
      <el-table :data="groups" class="governance-table" stripe>
        <el-table-column prop="name" :label="t('users.group')" min-width="180" /><el-table-column prop="path" :label="t('users.groupPath')" min-width="190" /><el-table-column :label="t('users.groupType')" width="120"><template #default="{ row }"><el-tag :type="(row as IdentityGroup).department ? 'success' : 'info'">{{ (row as IdentityGroup).department ? t('users.department') : t('users.identityGroup') }}</el-tag></template></el-table-column><el-table-column prop="member_count" :label="t('users.memberCount')" width="100" /><el-table-column :label="t('users.groupDailyBudget')" width="150"><template #default="{ row }">{{ !(row as IdentityGroup).department ? '—' : (row as IdentityGroup).daily_credit_limit_hundredths === undefined ? t('users.unlimited') : credits((row as IdentityGroup).daily_credit_limit_hundredths) }}</template></el-table-column><el-table-column :label="t('users.lastSynced')" width="180"><template #default="{ row }">{{ new Date((row as IdentityGroup).last_synced_at).toLocaleString() }}</template></el-table-column><el-table-column width="190"><template #default="{ row }"><template v-if="(row as IdentityGroup).department"><el-button size="small" @click="editBudget(row as IdentityGroup)">{{ t('users.budget') }}</el-button><el-button size="small" @click="openTransfer(row as IdentityGroup)">{{ t('users.transfer') }}</el-button></template></template></el-table-column>
      </el-table>
    </template>

    <template v-else-if="tab === 'audit'">
      <el-alert :closable="false" type="info" :title="t('users.auditPrivacy')" />
      <el-table :data="auditEvents" class="governance-table" stripe><el-table-column :label="t('users.occurredAt')" width="190"><template #default="{ row }">{{ new Date((row as GovernanceAuditEvent).occurred_at).toLocaleString() }}</template></el-table-column><el-table-column prop="action" :label="t('users.action')" min-width="210" /><el-table-column :label="t('users.target')" min-width="230"><template #default="{ row }">{{ (row as GovernanceAuditEvent).target_type }} · {{ (row as GovernanceAuditEvent).target_id }}</template></el-table-column><el-table-column prop="reason" :label="t('users.reason')" min-width="260" /><el-table-column :label="t('users.metrics')" min-width="170"><template #default="{ row }">{{ (row as GovernanceAuditEvent).metrics.map((metric) => `${metric.key}: ${metric.value}`).join(' · ') || '—' }}</template></el-table-column></el-table>
    </template>
    <AdminCredits v-else :section="tab as 'rates' | 'codes'" />
  </section>

  <el-dialog v-model="showCreate" width="min(480px, calc(100vw - 32px))" align-center><template #header><h2>{{ t('users.create') }}</h2></template><el-form :model="form" label-position="top"><el-form-item :label="t('users.username')" required><el-input v-model="form.username" /></el-form-item><el-form-item :label="t('users.displayName')" required><el-input v-model="form.display_name" /></el-form-item><el-form-item :label="t('users.email')" required><el-input v-model="form.email" type="email" /></el-form-item></el-form><template #footer><el-button @click="showCreate = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :disabled="!form.username || !form.display_name || !form.email" @click="create">{{ t('users.create') }}</el-button></template></el-dialog>
  <el-dialog :model-value="Boolean(roleTarget)" width="min(480px, calc(100vw - 32px))" align-center @close="roleTarget = undefined"><template #header><h2>{{ t('users.roles') }} · {{ roleTarget?.display_name }}</h2></template><el-form label-position="top"><el-form-item><el-checkbox v-model="roleForm.administrator">{{ t('users.administrator') }}</el-checkbox></el-form-item><el-form-item><el-checkbox v-model="roleForm.resourcePublisher">{{ t('users.resourcePublisher') }}</el-checkbox></el-form-item><el-form-item :label="t('users.reason')" required><el-input v-model="roleForm.reason" type="textarea" maxlength="500" show-word-limit /></el-form-item></el-form><template #footer><el-button @click="roleTarget = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!roleForm.reason.trim()" @click="saveRoles">{{ t('common.save') }}</el-button></template></el-dialog>
  <el-dialog :model-value="Boolean(statusTarget)" width="min(480px, calc(100vw - 32px))" align-center @close="statusTarget = undefined"><template #header><h2>{{ statusTarget?.enabled ? t('users.disableUser') : t('users.enableUser') }}</h2></template><p>{{ statusTarget?.display_name }} · @{{ statusTarget?.username }}</p><el-form label-position="top"><el-form-item :label="t('users.reason')" required><el-input v-model="statusReason" type="textarea" maxlength="500" show-word-limit /></el-form-item></el-form><template #footer><el-button @click="statusTarget = undefined">{{ t('common.cancel') }}</el-button><el-button :type="statusTarget?.enabled ? 'danger' : 'primary'" :loading="busy" :disabled="!statusReason.trim()" @click="saveStatus">{{ t('common.confirm') }}</el-button></template></el-dialog>
  <el-dialog :model-value="Boolean(creditUser)" width="min(480px, calc(100vw - 32px))" align-center @close="creditUser = undefined"><template #header><h2>{{ t('users.creditSettings') }} · {{ creditUser?.display_name }}</h2></template><el-form label-position="top"><el-form-item :label="t('users.dailyLimit')"><el-input-number v-model="creditForm.daily" :min="0" :precision="2" /></el-form-item><el-form-item :label="t('users.adjustment')"><el-input-number v-model="creditForm.adjustment" :precision="2" /></el-form-item><el-form-item :label="t('users.adjustmentReason')" :required="creditForm.adjustment !== 0"><el-input v-model="creditForm.reason" /></el-form-item></el-form><template #footer><el-button @click="creditUser = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :disabled="creditForm.adjustment !== 0 && !creditForm.reason.trim()" @click="saveCredits">{{ t('common.save') }}</el-button></template></el-dialog>
  <el-dialog :model-value="Boolean(budgetTarget)" width="min(480px, calc(100vw - 32px))" align-center @close="budgetTarget = undefined"><template #header><h2>{{ t('users.groupDailyBudget') }} · {{ budgetTarget?.name }}</h2></template><el-form label-position="top"><el-form-item><el-switch v-model="budgetForm.enabled" :active-text="t('users.limitBudget')" :inactive-text="t('users.unlimited')" /></el-form-item><el-form-item v-if="budgetForm.enabled" :label="t('users.dailyLimit')"><el-input-number v-model="budgetForm.daily" :min="0" :precision="2" /></el-form-item><el-form-item :label="t('users.reason')" required><el-input v-model="budgetForm.reason" type="textarea" maxlength="500" show-word-limit /></el-form-item></el-form><template #footer><el-button @click="budgetTarget = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!budgetForm.reason.trim()" @click="saveBudget">{{ t('common.save') }}</el-button></template></el-dialog>
  <el-dialog :model-value="Boolean(transferGroup)" width="min(520px, calc(100vw - 32px))" align-center @close="transferGroup = undefined"><template #header><h2>{{ t('users.transfer') }} · {{ transferGroup?.name }}</h2></template><el-alert :closable="false" type="warning" :title="t('users.transferBoundary')" /><el-form label-position="top"><el-form-item :label="t('users.transferFrom')" required><el-select v-model="transferForm.from"><el-option v-for="user in transferSources" :key="user.id" :value="user.id" :label="`${user.display_name} (@${user.username})`" /></el-select></el-form-item><el-form-item :label="t('users.transferTo')" required><el-select v-model="transferForm.to"><el-option v-for="user in transferTargets" :key="user.id" :value="user.id" :label="`${user.display_name} (@${user.username})`" /></el-select></el-form-item><el-form-item :label="t('users.reason')" required><el-input v-model="transferForm.reason" type="textarea" maxlength="500" show-word-limit /></el-form-item></el-form><template #footer><el-button @click="transferGroup = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!transferForm.from || !transferForm.to || !transferForm.reason.trim()" @click="transferResources">{{ t('users.transfer') }}</el-button></template></el-dialog>
  <ConfirmDialog :open="Boolean(pendingReset)" :title="t('users.reset')" :message="pendingReset ? `${t('users.reset')} — ${pendingReset.username}?` : ''" :confirm-label="t('users.reset')" :cancel-label="t('common.cancel')" danger @cancel="pendingReset = undefined" @confirm="reset" />
</template>

<style scoped>
.header-actions, .role-tags { display: flex; align-items: center; gap: 7px; flex-wrap: wrap; }
.role-tags { margin-top: 5px; }
.budget-note { display: block; max-width: 120px; margin-top: 4px; color: var(--muted); line-height: 1.35; }
.governance-table { margin-top: 14px; }
</style>
