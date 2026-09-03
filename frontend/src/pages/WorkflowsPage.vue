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
</script>

<template>
  <section class="page-surface">
    <header class="page-header"><div><p class="eyebrow">02 / REPEATABLE EXECUTION</p><h1>{{ t('workflows.title') }}</h1><p>{{ t('workflows.subtitle') }}</p></div><div><button class="button ghost" @click="showDeleted = !showDeleted">{{ t('workflows.deletedRecords') }} · {{ deleted.length }}</button><button class="button primary" @click="showCreate = true">＋ {{ t('workflows.new') }}</button></div></header>
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <div v-if="loading" class="quiet-state large">{{ t('common.loading') }}</div>
    <div v-else-if="workflows.length === 0" class="empty-workflows"><div class="blueprint"><i></i><i></i><span>⌁</span></div><h2>{{ t('common.empty') }}</h2><button class="button primary" @click="showCreate = true">{{ t('workflows.new') }}</button></div>
    <div v-else class="workflow-grid">
      <article v-for="(workflow, index) in workflows" :key="workflow.id" class="workflow-card" :style="{ '--delay': `${index * 45}ms` }">
        <div class="workflow-card-top"><span class="workflow-index">WF-{{ String(index + 1).padStart(2, '0') }}</span><span class="status-dot"></span></div><h2>{{ workflow.name }}</h2><p>{{ workflow.goal }}</p>
        <div class="workflow-meta"><span>{{ workflow.runtime_engine ? runtimeEngineDisplayName(workflow.runtime_engine) : 'AUTO' }}</span><span>{{ specialistName(workflow) }}</span></div>
        <footer><button class="button ghost" @click="router.push(`/workflows/${workflow.id}`)">{{ t('workflows.open') }} →</button><button class="round-run" :aria-label="t('workflows.runNow')" @click="run(workflow)">▶</button></footer>
      </article>
    </div>
    <section v-if="showDeleted" class="deleted-records"><div v-if="!deleted.length" class="empty-inline"><p>{{ t('common.empty') }}</p></div><div v-else class="workflow-grid"><article v-for="workflow in deleted" :key="workflow.id" class="workflow-card"><div class="workflow-card-top"><span class="workflow-index">{{ t('common.readOnly') }}</span></div><h2>{{ workflow.name }}</h2><p>{{ workflow.goal }}</p><footer><button class="button ghost" @click="router.push(`/workflows/${workflow.id}?tab=history`)">{{ t('workflows.open') }} →</button></footer></article></div></section>
  </section>
  <div v-if="showCreate" class="modal-layer" @click.self="showCreate = false"><form class="modal-card wide-modal" @submit.prevent="create"><p class="eyebrow">NEW / WORKFLOW</p><h2>{{ t('workflows.new') }}</h2><div class="form-grid"><label>{{ t('workflows.name') }}<input v-model="form.name" required maxlength="100"></label><label>{{ t('workflows.expert') }}<select :value="form.expert_team_id ? `team:${form.expert_team_id}` : form.expert_id ? `expert:${form.expert_id}` : 'none'" @change="setSpecialist(($event.target as HTMLSelectElement).value)"><option value="none">{{ t('sessions.noExpert') }}</option><optgroup :label="t('experts.title')"><option v-for="expert in experts" :key="expert.id" :value="`expert:${expert.id}`" :disabled="!expert.available">{{ expert.name }}</option></optgroup><optgroup :label="t('experts.teams')"><option v-for="team in expertTeams" :key="team.id" :value="`team:${team.id}`" :disabled="!team.available">{{ team.name }}</option></optgroup></select></label><label class="full">{{ t('workflows.goal') }}<textarea v-model="form.goal" required rows="7"></textarea></label></div><div class="modal-actions"><button type="button" class="button ghost" @click="showCreate = false">{{ t('common.cancel') }}</button><button class="button primary">{{ t('workflows.new') }}</button></div></form></div>
</template>
