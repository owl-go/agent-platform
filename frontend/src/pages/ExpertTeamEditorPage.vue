<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowDown, ArrowLeft, ArrowUp, GripVertical, Plus, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertInput, type ExpertTeam, type ExpertTeamInput, type MCPServer, type Skill, type CLIConnectorDefinition, type CLIConnectorEnablement } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import IconPicker from "../components/IconPicker.vue";
import ExpertBindingsEditor from "../components/ExpertBindingsEditor.vue";
import ExpertResourceSelector from "../components/ExpertResourceSelector.vue";
import { authContextKey } from "../auth/session";

const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const experts = ref<Expert[]>([]);
const team = ref<ExpertTeam>();
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const cliConnectors = ref<CLIConnectorDefinition[]>([]);
const cliEnablements = ref<CLIConnectorEnablement[]>([]);
const selectedExpertID = ref("");
const draggedIndex = ref<number>();
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertTeamInput>({ lead_member_id: "", starter_prompts: [], name: "", icon: "users", icon_background: "sage", introduction: "", core_capability: "", members: [] });
const isNew = route.name === "expert-team-new" || route.params.teamId === "new";
const canMaintain = computed(() => administrator.value && (isNew || team.value?.mutable === true));
const members = computed(() => form.value.members.map((member) => ({ ...member, expert: experts.value.find((item) => item.id === member.expert_id) })));
const candidates = computed(() => experts.value.filter((item) => item.available));

onMounted(async () => {
  if (!administrator.value) return;
  try {
    if (!isNew) {
      team.value = await api.getExpertTeam(String(route.params.teamId));
      return;
    }
    [experts.value, mcp.value, skills.value, cliConnectors.value, cliEnablements.value] = await Promise.all([api.listExperts(), api.listMCPServers(), api.listSkills(), api.listCLIConnectorDefinitions(), api.listCLIConnectorEnablements()]);
  } catch { toast.value = { kind: "error", message: t("experts.loadTeamFailed") }; }
});

function copyDefinition(expert: Expert): ExpertInput {
  return { starter_prompts:expert.starter_prompts || [], connector_dependencies:expert.connector_dependencies || [], name: expert.name, icon: expert.icon, icon_background: expert.icon_background, introduction: expert.introduction, guidance: expert.guidance || "", mcp_server_ids: [...expert.mcp_server_ids], skill_ids: [...expert.skill_ids], cli_connector_definition_ids: [...expert.cli_connector_definition_ids] };
}
function addMember() { const expert = experts.value.find((item) => item.id === selectedExpertID.value); if (expert && form.value.members.length < 10) form.value.members.push({ id: crypto.randomUUID(), name: expert.name, expert_id: expert.id, labels: [], definition: copyDefinition(expert) }); selectedExpertID.value = ""; }
function move(index: number, offset: number) { const target = index + offset; if (target < 0 || target >= form.value.members.length) return; const [member] = form.value.members.splice(index, 1); form.value.members.splice(target, 0, member!); }
function dropMember(target: number) { const source = draggedIndex.value; draggedIndex.value = undefined; if (source === undefined || source === target) return; const [member] = form.value.members.splice(source, 1); form.value.members.splice(target, 0, member!); }
async function save() { if (!canMaintain.value || !form.value.lead_member_id) return; saving.value = true; try { if (team.value) await api.updateExpertTeam(team.value.id, form.value, team.value.version); else await api.createExpertTeam(form.value); toast.value = { kind: "success", message: t("experts.teamSaved") }; window.setTimeout(() => void router.push("/experts?tab=teams"), 350); } catch { toast.value = { kind: "error", message: t("experts.teamSaveFailed") }; } finally { saving.value = false; } }
async function remove() { if (!canMaintain.value || !team.value) return; try { await api.deleteExpertTeam(team.value.id); await router.push("/experts?tab=teams"); } catch { toast.value = { kind: "error", message: t("experts.deleteTeamFailed") }; } }
</script>

<template>
  <el-alert v-if="!administrator" type="info" :closable="false" :title="t('experts.teamAdministratorOnly')" />
  <section v-else class="page-surface editor-page">
    <el-button class="back-link" text :icon="ArrowLeft" @click="$router.push('/experts?tab=teams')">{{ t('experts.backTeams') }}</el-button>
    <header class="editor-header"><div><h1>{{ isNew ? t('experts.createTeam') : t('experts.editTeam') }}</h1></div><div v-if="isNew" class="editor-actions"><el-button v-if="team && canMaintain" type="danger" plain @click="confirmDelete = true">{{ t('common.delete') }}</el-button><el-button type="primary" :loading="saving" :disabled="!canMaintain || form.members.length < 2 || !form.lead_member_id" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</el-button></div></header>
    <ToastMessage v-if="toast" :kind="toast.kind" :title="toast.kind === 'success' ? t('experts.saveSucceeded') : t('experts.operationFailed')" :message="toast.message" :close-label="t('common.close')" @dismiss="toast = undefined" />
    <ExpertBindingsEditor v-if="team" :key="team.id" :team="team" @saved="router.push('/experts?tab=teams')" @cancel="router.push('/experts?tab=teams')" />
    <form v-else-if="canMaintain" class="editor-form" @submit.prevent="save">
      <section class="editor-section"><div><h2>{{ t('experts.teamInfo') }}</h2><p>{{ t('experts.teamInfoHint') }}</p></div><div class="form-grid"><label>{{ t('experts.teamName') }}<el-input v-model="form.name" maxlength="100" show-word-limit /></label><div class="form-field"><span>{{ t('experts.icon') }}</span><IconPicker @invalid="toast = {kind: 'error', message: '请选择不超过 2 MiB 的有效图像'}" v-model="form.icon" fallback="users" team /></div><label>{{ t('experts.iconBackground') }}<el-select v-model="form.icon_background"><el-option value="sage" :label="t('experts.sage')" /><el-option value="sand" :label="t('experts.sand')" /><el-option value="sky" :label="t('experts.sky')" /><el-option value="coral" :label="t('experts.coral')" /></el-select></label><div class="form-field full"><span>开场提示（最多三条）</span><div v-for="(_, index) in form.starter_prompts" :key="index"><el-input v-model="form.starter_prompts![index]" maxlength="2000" :aria-label="`开场提示 ${index + 1}`" /><el-button text @click="form.starter_prompts!.splice(index, 1)">删除</el-button></div><el-button v-if="(form.starter_prompts?.length || 0) < 3" text @click="(form.starter_prompts ||= []).push('')">添加开场提示</el-button></div>
        <label class="full">{{ t('experts.introduction') }}<el-input v-model="form.introduction" type="textarea" :rows="3" maxlength="2000" show-word-limit /></label><label class="full">{{ t('experts.coreCapability') }}<el-input v-model="form.core_capability" type="textarea" :rows="4" maxlength="20000" show-word-limit /></label></div></section>
      <section class="editor-section"><div><h2>{{ t('experts.members') }}</h2><p>{{ t('experts.membersHint') }}</p></div><div><div class="member-picker"><select v-model="selectedExpertID"><option value="">{{ t('experts.chooseExpert') }}</option><option v-for="item in candidates" :key="item.id" :value="item.id">{{ item.name }}</option></select><el-button :icon="Plus" :disabled="!selectedExpertID" @click="addMember">{{ t('experts.add') }}</el-button></div><label class="lead-field">{{ t('experts.teamLead') }}<select v-model="form.lead_member_id" class="lead-selector"><option value="">{{ t('experts.chooseLead') }}</option><option v-for="member in form.members" :key="member.id" :value="member.id">{{ member.name }}</option></select></label><ol class="ordered-members"><li v-for="(member, index) in members" :key="member.id" draggable="true" @dragstart="draggedIndex = index" @dragend="draggedIndex = undefined" @dragover.prevent @drop="dropMember(index)"><GripVertical :size="17" /><span class="member-order">{{ index + 1 }}</span><div class="member-fields"><el-input v-model="form.members[index]!.name" :aria-label="t('experts.memberName')" maxlength="100" /><small>{{ member.definition?.name || member.expert?.name }}</small><el-select v-model="form.members[index]!.labels" multiple allow-create filterable default-first-option :multiple-limit="5" :placeholder="t('experts.memberLabels')" /><template v-if="member.definition"><IconPicker v-model="member.definition.icon" @invalid="toast = {kind: 'error', message: '请选择有效图像'}" /><div v-for="(_, starterIndex) in member.definition.starter_prompts" :key="starterIndex"><el-input v-model="member.definition.starter_prompts![starterIndex]" maxlength="2000" :aria-label="`成员开场提示 ${starterIndex + 1}`" /><el-button text @click="member.definition.starter_prompts!.splice(starterIndex, 1)">删除</el-button></div><el-button v-if="(member.definition.starter_prompts?.length || 0) < 3" text @click="(member.definition.starter_prompts ||= []).push('')">添加成员开场提示</el-button><label>{{ t('experts.introduction') }}<el-input v-model="member.definition.introduction" type="textarea" :rows="2" maxlength="2000" /></label><label class="member-guidance">{{ t('experts.guidance') }}<el-input v-model="member.definition.guidance" type="textarea" :rows="6" maxlength="100000" /></label><ExpertResourceSelector :mcp-servers="mcp" :skills="skills" :cli-connectors="cliConnectors" :cli-enablements="cliEnablements" :mcp-server-ids="member.definition.mcp_server_ids" :skill-ids="member.definition.skill_ids" :cli-connector-definition-ids="member.definition.cli_connector_definition_ids" @update:mcp-server-ids="member.definition.mcp_server_ids = $event" @update:skill-ids="member.definition.skill_ids = $event" @update:cli-connector-definition-ids="member.definition.cli_connector_definition_ids = $event" /></template></div><el-button text circle :disabled="index === 0" :aria-label="t('experts.moveUp', { name: member.name })" @click="move(index, -1)"><ArrowUp :size="16" /></el-button><el-button text circle :disabled="index === members.length - 1" :aria-label="t('experts.moveDown', { name: member.name })" @click="move(index, 1)"><ArrowDown :size="16" /></el-button><el-button text circle type="danger" :aria-label="t('experts.removeMember', { name: member.name })" @click="form.members.splice(index, 1)"><X :size="16" /></el-button></li></ol><el-alert type="info" :closable="false" :title="`${t('experts.perRound', { count: members.length })} · ${t('experts.sequential')}`" /></div></section>
    </form>
  </section>
  <ConfirmDialog :open="confirmDelete" :title="t('experts.deleteExpertTitle', { name: team?.name })" :message="t('experts.deleteTeamHint')" :confirm-label="t('experts.confirmDelete')" :cancel-label="t('common.cancel')" danger @cancel="confirmDelete = false" @confirm="remove" />
</template>
