<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Workflow, type WorkflowInput } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import { MoreHorizontal, Pencil, Trash2 } from "@lucide/vue";

const api = inject(platformApiKey)!;
const { t } = useI18n(); const router = useRouter();
const workflows = ref<Workflow[]>([]); const loading = ref(true); const showCreate = ref(false); const creating = ref(false); const error = ref(""); const renameTarget = ref<Workflow>(); const renameValue = ref(""); const renameBusy = ref(false); const deleteTarget = ref<Workflow>();
const form = ref<WorkflowInput>({ name: "", goal: "", environment: [], knowledge_base_ids: [] });
onMounted(refresh);
async function refresh() { loading.value = true; try { workflows.value = await api.listWorkflows(); } catch { error.value = t("errors.generic"); } finally { loading.value = false; } }
async function create() {
  if (creating.value) return;
  creating.value = true;
  let item: Workflow | undefined;
  try {
    item = await api.createWorkflow({ name: form.value.name.trim(), goal: form.value.goal.trim(), environment: [], knowledge_base_ids: [] });
    const validationRun = await api.runWorkflow(item.id, { plan_preference: "always" });
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
async function run(item: Workflow) { try { await api.runWorkflow(item.id); await router.push(`/workflows/${item.id}?tab=history`); } catch { error.value = t("errors.generic"); } }
function openWorkflow(item: Workflow) { void router.push(`/workflows/${item.id}`); }
function openRename(item: Workflow) { renameTarget.value = item; renameValue.value = item.name; }
async function renameWorkflow() {
  const item = renameTarget.value;
  const name = renameValue.value.trim();
  if (!item || !name || renameBusy.value) return;
  renameBusy.value = true;
  try {
    const updated = await api.updateWorkflow(item.id, { name, goal: item.goal, expert_id: item.expert_id, expert_team_id: item.expert_team_id, environment: item.environment, schedule: item.schedule }, item.version);
    workflows.value = workflows.value.map((workflow) => workflow.id === updated.id ? updated : workflow);
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
</script>

<template>
  <section class="page-surface">
    <header class="page-header"><div><h1>{{ t('workflows.title') }}</h1><p>{{ t('workflows.subtitle') }}</p></div><el-button type="primary" @click="showCreate = true">＋ {{ t('workflows.new') }}</el-button></header>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <el-skeleton v-if="loading" :rows="8" animated class="page-loading" />
    <el-empty v-else-if="workflows.length === 0" :description="t('common.empty')"><el-button type="primary" @click="showCreate = true">{{ t('workflows.new') }}</el-button></el-empty>
    <div v-else class="workflow-grid">
      <el-card v-for="workflow in workflows" :key="workflow.id" class="workflow-card" shadow="hover" role="button" tabindex="0" :aria-label="workflow.name" @click="openWorkflow(workflow)" @keydown.enter="openWorkflow(workflow)" @keydown.space.prevent="openWorkflow(workflow)">
        <div class="workflow-card-top" @click.stop @keydown.stop><el-dropdown trigger="click" @command="handleWorkflowAction($event, workflow)"><el-button class="workflow-more" text circle :aria-label="t('common.more')" :title="t('common.more')"><MoreHorizontal /></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="rename"><Pencil :size="14" />{{ t('common.rename') }}</el-dropdown-item><el-dropdown-item command="delete" divided><Trash2 :size="14" />{{ t('common.delete') }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown></div>
        <h2>{{ workflow.name }}</h2><p class="workflow-created-at">{{ new Date(workflow.created_at).toLocaleString() }}</p>
        <footer><el-button type="primary" circle :aria-label="t('workflows.runNow')" :title="t('workflows.runNow')" @click.stop="run(workflow)">▶</el-button></footer>
      </el-card>
    </div>
  </section>
  <el-dialog :model-value="Boolean(renameTarget)" class="resource-dialog" width="min(480px, calc(100vw - 32px))" align-center :title="t('common.rename')" @close="renameTarget = undefined"><el-form @submit.prevent="renameWorkflow"><el-form-item :label="t('workflows.name')" required><el-input v-model="renameValue" maxlength="100" autofocus /></el-form-item></el-form><template #footer><el-button @click="renameTarget = undefined">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="renameBusy" :disabled="!renameValue.trim()" @click="renameWorkflow">{{ t('common.save') }}</el-button></template></el-dialog>
  <ConfirmDialog :open="Boolean(deleteTarget)" :title="t('workflows.deleteTitle')" :message="deleteTarget ? `${t('common.delete')} “${deleteTarget.name}”?` : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger @cancel="deleteTarget = undefined" @confirm="removeWorkflow" />
  <el-dialog v-model="showCreate" class="resource-dialog" width="min(680px, calc(100vw - 32px))" align-center><template #header><h2>{{ t('workflows.new') }}</h2><p class="muted">{{ t('workflows.createHint') }}</p></template><el-form :model="form" label-position="top" @submit.prevent="create"><div class="form-grid"><el-form-item :label="t('workflows.name')" required><el-input v-model="form.name" maxlength="100" autofocus /></el-form-item><el-form-item class="full" :label="t('workflows.goal')" required><el-input v-model="form.goal" type="textarea" :rows="7" /></el-form-item></div></el-form><template #footer><el-button :disabled="creating" @click="showCreate = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="creating" :disabled="!form.name.trim() || !form.goal.trim()" @click="create">{{ t('workflows.createAndValidate') }}</el-button></template></el-dialog>
</template>
