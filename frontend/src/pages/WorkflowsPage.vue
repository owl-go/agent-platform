<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, runtimeEngineDisplayName, type Expert, type ExpertTeam, type Workflow, type WorkflowInput } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";

const api = inject(platformApiKey)!;
const { t } = useI18n(); const router = useRouter();
const workflows = ref<Workflow[]>([]); const deleted = ref<Workflow[]>([]); const experts = ref<Expert[]>([]); const expertTeams = ref<ExpertTeam[]>([]); const loading = ref(true); const showCreate = ref(false); const showDeleted = ref(false); const error = ref("");
const form = ref<WorkflowInput>({ name: "", goal: "", environment: [] });
onMounted(refresh);
async function refresh() { loading.value = true; try { [workflows.value, deleted.value, experts.value, expertTeams.value] = await Promise.all([api.listWorkflows(), api.listWorkflows(true), api.listExperts(), api.listExpertTeams()]); } catch { error.value = t("errors.generic"); } finally { loading.value = false; } }
async function create() { try { const item = await api.createWorkflow(form.value); showCreate.value = false; form.value = { name: "", goal: "", environment: [] }; await router.push(`/workflows/${item.id}`); } catch { error.value = t("errors.validation"); } }
async function run(item: Workflow) { try { await api.runWorkflow(item.id); await router.push(`/workflows/${item.id}?tab=history`); } catch { error.value = t("errors.generic"); } }
function setSpecialist(value: string) { form.value.expert_id = value.startsWith("expert:") ? value.slice(7) : undefined; form.value.expert_team_id = value.startsWith("team:") ? value.slice(5) : undefined; }
function specialistName(workflow: Workflow) { return workflow.expert_team_id ? expertTeams.value.find((team) => team.id === workflow.expert_team_id)?.name : workflow.expert_id ? experts.value.find((expert) => expert.id === workflow.expert_id)?.name : t('sessions.noExpert'); }
function teamSelectionLabel(team: ExpertTeam): string { const compatibility = team.experts.some((item) => item.compatibility === "incompatible") ? t("experts.incompatible") : team.experts.some((item) => item.compatibility === "unverified") ? t("settings.unverified") : t("settings.verified"); return `${team.name} · ${compatibility}`; }
</script>

<template>
  <section class="page-surface">
    <header class="page-header"><div><h1>{{ t('workflows.title') }}</h1><p>{{ t('workflows.subtitle') }}</p></div><el-space wrap><el-button @click="showDeleted = !showDeleted">{{ t('workflows.deletedRecords') }} · {{ deleted.length }}</el-button><el-button type="primary" @click="showCreate = true">＋ {{ t('workflows.new') }}</el-button></el-space></header>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <el-skeleton v-if="loading" :rows="8" animated class="page-loading" />
    <el-empty v-else-if="workflows.length === 0" :description="t('common.empty')"><el-button type="primary" @click="showCreate = true">{{ t('workflows.new') }}</el-button></el-empty>
    <div v-else class="workflow-grid">
      <el-card v-for="workflow in workflows" :key="workflow.id" class="workflow-card" shadow="hover">
        <div class="workflow-card-top"><span class="status-dot"></span></div><h2>{{ workflow.name }}</h2><p>{{ workflow.goal }}</p>
        <div class="workflow-meta"><el-tag size="small" type="info" effect="plain">{{ specialistName(workflow) }}</el-tag></div>
        <footer><el-button @click="router.push(`/workflows/${workflow.id}`)">{{ t('workflows.open') }} →</el-button><el-button type="primary" circle :aria-label="t('workflows.runNow')" @click="run(workflow)">▶</el-button></footer>
      </el-card>
    </div>
    <section v-if="showDeleted" class="deleted-records"><el-empty v-if="!deleted.length" :description="t('common.empty')" :image-size="72" /><div v-else class="workflow-grid"><el-card v-for="workflow in deleted" :key="workflow.id" class="workflow-card" shadow="never"><div class="workflow-card-top"><el-tag type="info" size="small">{{ t('common.readOnly') }}</el-tag></div><h2>{{ workflow.name }}</h2><p>{{ workflow.goal }}</p><footer><el-button @click="router.push(`/workflows/${workflow.id}?tab=history`)">{{ t('workflows.open') }} →</el-button></footer></el-card></div></section>
  </section>
  <el-dialog v-model="showCreate" class="resource-dialog" width="min(680px, calc(100vw - 32px))" align-center><template #header><h2>{{ t('workflows.new') }}</h2></template><el-form :model="form" label-position="top" @submit.prevent="create"><div class="form-grid"><el-form-item :label="t('workflows.name')" required><el-input v-model="form.name" maxlength="100" /></el-form-item><el-form-item :label="t('workflows.expert')"><el-select :model-value="form.expert_team_id ? `team:${form.expert_team_id}` : form.expert_id ? `expert:${form.expert_id}` : 'none'" @change="setSpecialist"><el-option value="none" :label="t('sessions.noExpert')" /><el-option-group :label="t('experts.title')"><el-option v-for="expert in experts" :key="expert.id" :value="`expert:${expert.id}`" :label="expert.name" :disabled="!expert.available" /></el-option-group><el-option-group :label="t('experts.teams')"><el-option v-for="team in expertTeams" :key="team.id" :value="`team:${team.id}`" :label="teamSelectionLabel(team)" :disabled="!team.available" /></el-option-group></el-select></el-form-item><el-form-item class="full" :label="t('workflows.goal')" required><el-input v-model="form.goal" type="textarea" :rows="7" /></el-form-item></div></el-form><template #footer><el-button @click="showCreate = false">{{ t('common.cancel') }}</el-button><el-button type="primary" :disabled="!form.name.trim() || !form.goal.trim()" @click="create">{{ t('workflows.new') }}</el-button></template></el-dialog>
</template>
