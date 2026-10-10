<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertInput, type ExpertTeam, type ExpertTeamInput, type MCPServer, type Skill, type CLIConnectorDefinition, type CLIConnectorEnablement } from "../api/client";
import { authContextKey } from "../auth/session";
import ExpertResourceSelector from "./ExpertResourceSelector.vue";

const props = defineProps<{ expert?: Expert; team?: ExpertTeam }>();
const emit = defineEmits<{ saved: [resource: Expert | ExpertTeam]; cancel: [] }>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const { t } = useI18n();
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const canEdit = computed(() => props.team
  ? administrator.value && props.team.mutable === true && !props.team.immutable
  : Boolean(props.expert && !props.expert.immutable && (!props.expert.platform || administrator.value)));
function definition(expert: Expert): ExpertInput {
  return { name: expert.name, icon: expert.icon, icon_background: expert.icon_background,
    introduction: expert.introduction, guidance: expert.guidance,
    starter_prompts: [...(expert.starter_prompts ?? [])],
    connector_dependencies: expert.connector_dependencies?.map(item => ({ ...item })),
    skill_ids: [...expert.skill_ids], mcp_server_ids: [...expert.mcp_server_ids],
    cli_connector_definition_ids: [...expert.cli_connector_definition_ids] };
}
const expertInput = ref(props.expert ? definition(props.expert) : undefined);
const teamInput = ref<ExpertTeamInput | undefined>(props.team ? {
  name: props.team.name, icon: props.team.icon, icon_background: props.team.icon_background,
  introduction: props.team.introduction, core_capability: props.team.core_capability,
  lead_member_id: props.team.lead_member_id, starter_prompts: [...(props.team.starter_prompts ?? [])],
  members: props.team.members.map(member => ({ id: member.id, name: member.name,
    labels: [...member.labels], expert_id: "", definition: definition(member.expert) })),
} : undefined);
const profiles = computed(() => expertInput.value ? [{ id: props.expert!.id, name: props.expert!.name, definition: expertInput.value }]
  : (teamInput.value?.members ?? []).map(member => ({ id: member.id, name: member.name, definition: member.definition! })));
const skills = ref<Skill[]>([]), mcp = ref<MCPServer[]>([]);
const cli = ref<CLIConnectorDefinition[]>([]), enablements = ref<CLIConnectorEnablement[]>([]);
const loading = ref(true), ready = ref(false), saving = ref(false), error = ref("");
let disposed = false;
onBeforeUnmount(() => { disposed = true; });
onMounted(async () => {
  if (!canEdit.value) { loading.value = false; return; }
  try {
    const values = await Promise.all([api.listSkills(), api.listMCPServers(), api.listCLIConnectorDefinitions(), api.listCLIConnectorEnablements()]);
    if (disposed) return;
    [skills.value, mcp.value, cli.value, enablements.value] = values;
    ready.value = true;
  } catch { if (!disposed) error.value = t("experts.loadExpertFailed"); }
  finally { if (!disposed) loading.value = false; }
});
async function save() {
  if (!canEdit.value || !ready.value || saving.value) return;
  saving.value = true; error.value = "";
  try {
    const result = props.expert && expertInput.value
      ? await api.updateExpert(props.expert.id, JSON.parse(JSON.stringify(expertInput.value)) as ExpertInput, props.expert.version)
      : await api.updateExpertTeam(props.team!.id, JSON.parse(JSON.stringify(teamInput.value)) as ExpertTeamInput, props.team!.version);
    if (!disposed) emit("saved", result);
  } catch { if (!disposed) error.value = t("experts.saveFailed"); }
  finally { if (!disposed) saving.value = false; }
}
</script>

<template>
  <el-alert v-if="!canEdit" type="info" :closable="false" :title="t('experts.readOnly')" />
  <form v-else class="expert-bindings-editor" @submit.prevent="save">
    <el-skeleton v-if="loading" :rows="3" animated />
    <el-alert v-if="error" type="error" :closable="false" :title="error" />
    <fieldset v-if="ready" :disabled="saving">
      <section v-for="profile in profiles" :key="profile.id">
        <h3 v-if="team">{{ profile.name }}</h3>
        <ExpertResourceSelector :disabled="saving" :skills="skills" :mcp-servers="mcp" :cli-connectors="cli" :cli-enablements="enablements"
          v-model:skill-ids="profile.definition.skill_ids" v-model:mcp-server-ids="profile.definition.mcp_server_ids" v-model:cli-connector-definition-ids="profile.definition.cli_connector_definition_ids" />
      </section>
    </fieldset>
    <div class="expert-bindings-actions"><el-button :disabled="saving" @click="emit('cancel')">{{ t('common.cancel') }}</el-button><el-button native-type="submit" type="primary" :loading="saving" :disabled="!ready || saving">{{ t('common.save') }}</el-button></div>
  </form>
</template>
