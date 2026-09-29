<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Workflow, type WorkflowInput } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import { MoreHorizontal, Pencil, Trash2 } from "@lucide/vue";

type WorkflowFilter = "all" | "attention" | "scheduled" | "never_run";
type WorkflowSort = "attention" | "updated" | "frequent";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const router = useRouter();
const workflows = ref<Workflow[]>([]);
const loading = ref(true);
const showCreate = ref(false);
const creating = ref(false);
const error = ref("");
const renameTarget = ref<Workflow>();
const renameValue = ref("");
const renameBusy = ref(false);
const deleteTarget = ref<Workflow>();
const runningID = ref("");
const search = ref("");
const filter = ref<WorkflowFilter>("all");
const sort = ref<WorkflowSort>("attention");
const form = ref<WorkflowInput>({ name: "", goal: "", environment: [], knowledge_base_ids: [] });

const visibleWorkflows = computed(() => {
  const query = search.value.trim().toLocaleLowerCase();
  const filtered = workflows.value.filter((workflow) => {
    if (query && !`${workflow.name}\n${workflow.goal}`.toLocaleLowerCase().includes(query)) return false;
    if (filter.value === "attention") return workflow.needs_attention;
    if (filter.value === "scheduled") return Boolean(workflow.schedule?.enabled);
    if (filter.value === "never_run") return !workflow.last_run_state;
    return true;
  });
  return [...filtered].sort((left, right) => {
    if (sort.value === "frequent") return (right.run_count_30d || 0) - (left.run_count_30d || 0) || Date.parse(right.updated_at) - Date.parse(left.updated_at);
    if (sort.value === "updated") return Date.parse(right.updated_at) - Date.parse(left.updated_at);
    return Number(right.needs_attention) - Number(left.needs_attention) || Date.parse(right.last_run_at || right.updated_at) - Date.parse(left.last_run_at || left.updated_at);
  });
});

const deleteMessage = computed(() => deleteTarget.value ? t("workflows.deleteImpact", {
  name: deleteTarget.value.name,
  schedule: deleteTarget.value.schedule?.enabled ? t("workflows.deleteScheduleActive") : t("workflows.deleteScheduleInactive"),
  api: deleteTarget.value.api_credential_configured ? t("workflows.deleteApiActive") : t("workflows.deleteApiInactive"),
}) : "");

onMounted(refresh);

async function refresh() {
  loading.value = true;
  try { workflows.value = await api.listWorkflows(); }
  catch { error.value = t("errors.generic"); }
  finally { loading.value = false; }
}

async function create() {
  if (creating.value) return;
  creating.value = true;
  let item: Workflow | undefined;
  try {
    item = await api.createWorkflow({ name: form.value.name.trim(), goal: form.value.goal.trim(), environment: [], knowledge_base_ids: [] });
    const validationRun = await api.runWorkflow(item.id, {});
    showCreate.value = false;
    form.value = { name: "", goal: "", environment: [], knowledge_base_ids: [] };
    await router.push({ path: `/workflows/${item.id}`, query: { tab: "history", open_run: validationRun.id } });
  } catch {
    if (item) {
      showCreate.value = false;
      form.value = { name: "", goal: "", environment: [], knowledge_base_ids: [] };
      await router.push({ path: `/workflows/${item.id}`, query: { tab: "history", validation_error: "1" } });
    } else error.value = t("errors.validation");
  } finally { creating.value = false; }
}

async function run(item: Workflow) {
  if (runningID.value) return;
  if (item.last_run_id && (item.needs_attention || item.last_run_state === "running" || item.last_run_state === "queued")) {
    openRun(item);
    return;
  }
  runningID.value = item.id;
  try {
    const created = await api.runWorkflow(item.id);
    await router.push({ path: `/workflows/${item.id}`, query: { tab: "history", open_run: created.id } });
  } catch { error.value = t("errors.generic"); }
  finally { runningID.value = ""; }
}

function openWorkflow(item: Workflow) { void router.push(`/workflows/${item.id}`); }
function openRun(item: Workflow) { void router.push({ path: `/workflows/${item.id}`, query: { tab: "history", open_run: item.last_run_id } }); }
function openRename(item: Workflow) { renameTarget.value = item; renameValue.value = item.name; }

async function renameWorkflow() {
  const item = renameTarget.value;
  const name = renameValue.value.trim();
  if (!item || !name || renameBusy.value) return;
  renameBusy.value = true;
  try {
    const updated = await api.updateWorkflow(item.id, { name, goal: item.goal, expert_id: item.expert_id, expert_team_id: item.expert_team_id, environment: item.environment, schedule: item.schedule }, item.version);
    workflows.value = workflows.value.map((workflow) => workflow.id === updated.id ? { ...workflow, ...updated } : workflow);
    renameTarget.value = undefined;
  } catch { error.value = t("errors.generic"); }
  finally { renameBusy.value = false; }
}

async function removeWorkflow() {
  const item = deleteTarget.value;
  if (!item) return;
  try {
    await api.deleteWorkflow(item.id);
    workflows.value = workflows.value.filter((workflow) => workflow.id !== item.id);
    deleteTarget.value = undefined;
  } catch { error.value = t("errors.generic"); }
}

function handleWorkflowAction(command: string | number | object, item: Workflow) {
  if (command === "rename") openRename(item);
  if (command === "delete") deleteTarget.value = item;
}

function stateLabel(workflow: Workflow) {
  if (!workflow.last_run_state) return t("workflows.notRun");
  return t(`workflows.runState.${workflow.last_run_state}`);
}

function stateType(workflow: Workflow) {
  if (workflow.last_run_state === "succeeded") return "success";
  if (workflow.last_run_state === "failed") return "danger";
  if (workflow.last_run_state === "waiting_for_user") return "warning";
  return "info";
}

function actionLabel(workflow: Workflow) {
  if (workflow.last_run_state === "waiting_for_user") return t("workflows.continueAction");
  if (workflow.last_run_state === "failed") return t("workflows.recoverAction");
  if (workflow.last_run_state === "running" || workflow.last_run_state === "queued") return t("workflows.viewRun");
  if (!workflow.last_run_state) return t("workflows.validateAction");
  return t("workflows.runNow");
}

function successRate(workflow: Workflow) {
  if (!workflow.run_count_30d) return "—";
  return `${Math.round((workflow.succeeded_run_count_30d || 0) / workflow.run_count_30d * 100)}%`;
}
</script>

<template>
  <section class="page-surface">
    <header class="page-header">
      <div><h1>{{ t('workflows.title') }}</h1><p>{{ t('workflows.subtitle') }}</p></div>
      <el-button type="primary" @click="showCreate = true">＋ {{ t('workflows.new') }}</el-button>
    </header>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <div v-if="!loading && workflows.length" class="workflow-toolbar">
      <el-input v-model="search" clearable :placeholder="t('workflows.searchPlaceholder')" />
      <el-select v-model="filter" :aria-label="t('workflows.filterLabel')">
        <el-option value="all" :label="t('workflows.filterAll')" />
        <el-option value="attention" :label="t('workflows.filterAttention')" />
        <el-option value="scheduled" :label="t('workflows.filterScheduled')" />
        <el-option value="never_run" :label="t('workflows.filterNeverRun')" />
      </el-select>
      <el-select v-model="sort" :aria-label="t('workflows.sortLabel')">
        <el-option value="attention" :label="t('workflows.sortAttention')" />
        <el-option value="updated" :label="t('workflows.sortUpdated')" />
        <el-option value="frequent" :label="t('workflows.sortFrequent')" />
      </el-select>
    </div>
    <el-skeleton v-if="loading" :rows="8" animated class="page-loading" />
    <el-empty v-else-if="workflows.length === 0" :description="t('common.empty')"><el-button type="primary" @click="showCreate = true">{{ t('workflows.new') }}</el-button></el-empty>
    <el-empty v-else-if="visibleWorkflows.length === 0" :description="t('workflows.noMatching')" />
    <div v-else class="workflow-grid">
      <el-card v-for="workflow in visibleWorkflows" :key="workflow.id" class="workflow-card" shadow="hover" role="button" tabindex="0" :aria-label="workflow.name" @click="openWorkflow(workflow)" @keydown.enter="openWorkflow(workflow)" @keydown.space.prevent="openWorkflow(workflow)">
        <div class="workflow-card-top" @click.stop @keydown.stop>
          <el-tag :type="stateType(workflow)" size="small">{{ stateLabel(workflow) }}</el-tag>
          <el-dropdown trigger="click" @command="handleWorkflowAction($event, workflow)"><el-button class="workflow-more" text circle :aria-label="t('common.more')" :title="t('common.more')"><MoreHorizontal /></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="rename"><Pencil :size="14" />{{ t('common.rename') }}</el-dropdown-item><el-dropdown-item command="delete" divided><Trash2 :size="14" />{{ t('common.delete') }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown>
        </div>
        <h2>{{ workflow.name }}</h2>
        <p class="workflow-goal">{{ workflow.goal }}</p>
        <div class="workflow-metrics">
          <span><small>{{ t('workflows.success30d') }}</small><strong>{{ successRate(workflow) }}</strong></span>
          <span><small>{{ t('workflows.runs30d') }}</small><strong>{{ workflow.run_count_30d || 0 }}</strong></span>
        </div>
        <div class="workflow-timing">
          <small>{{ t('workflows.lastRun') }}</small><span>{{ workflow.last_run_at ? new Date(workflow.last_run_at).toLocaleString() : t('workflows.notRun') }}</span>
          <small>{{ t('workflows.nextRun') }}</small><span>{{ workflow.next_scheduled_at ? new Date(workflow.next_scheduled_at).toLocaleString() : t('workflows.noSchedule') }}</span>
        </div>
        <footer @click.stop><el-button type="primary" :loading="runningID === workflow.id" @click="run(workflow)">{{ actionLabel(workflow) }}</el-button></footer>
      </el-card>
    </div>
  </section>
  <el-dialog :model-value="Boolean(renameTarget)" class="resource-dialog" width="min(480px, calc(100vw - 32px))" align-center :title="t('common.rename')" @close="renameTarget = undefined"><el-form @submit.prevent="renameWorkflow"><el-form-item :label="t('workflows.name')" required><el-input v-model="renameValue" maxlength="100" autofocus /></el-form-item></el-form><template #footer><el-button @click="renameTarget = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="renameBusy" :disabled="!renameValue.trim()" @click="renameWorkflow">{{ t('common.save') }}</el-button></template></el-dialog>
  <ConfirmDialog :open="Boolean(deleteTarget)" :title="t('workflows.deleteTitle')" :message="deleteMessage" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger @cancel="deleteTarget = undefined" @confirm="removeWorkflow" />
  <el-dialog v-model="showCreate" class="resource-dialog" width="min(680px, calc(100vw - 32px))" align-center><template #header><h2>{{ t('workflows.new') }}</h2><p class="muted">{{ t('workflows.createHint') }}</p></template><el-form :model="form" label-position="top" @submit.prevent="create"><div class="form-grid"><el-form-item :label="t('workflows.name')" required><el-input v-model="form.name" maxlength="100" autofocus /></el-form-item><el-form-item class="full" :label="t('workflows.goal')" required><el-input v-model="form.goal" type="textarea" :rows="7" /></el-form-item></div></el-form><template #footer><el-button :disabled="creating" @click="showCreate = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="creating" :disabled="!form.name.trim() || !form.goal.trim()" @click="create">{{ t('workflows.createAndValidate') }}</el-button></template></el-dialog>
</template>
