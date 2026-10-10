<script setup lang="ts">
import { computed, inject, ref, watch } from "vue";
import {
  platformApiKey, type ResourceExpertProposalEnvelope, type ResourceCreationAction,
  type MCPServer, type Skill, type CLIConnectorDefinition, type CLIConnectorEnablement,
} from "../api/client";
import IconPicker from "./IconPicker.vue";
import ExpertResourceSelector from "./ExpertResourceSelector.vue";

const props = defineProps<{ actionId?: string }>();
const emit = defineEmits<{ close: []; confirmed: [] }>();
const api = inject(platformApiKey)!;
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const cliConnectors = ref<CLIConnectorDefinition[]>([]);
const cliEnablements = ref<CLIConnectorEnablement[]>([]);
const proposal = ref<ResourceExpertProposalEnvelope>();
const action = ref<ResourceCreationAction>();
const original = ref("");
const busy = ref(false);
const error = ref("");
const changed = computed(() => JSON.stringify(proposal.value) !== original.value);
const editable = computed(() => action.value?.state === "pending" || action.value?.state === "failed");
const profiles = computed(() => proposal.value?.expert
  ? [{ id: "expert", name: "", expert: proposal.value.expert }]
  : proposal.value?.expert_team?.members || []);

watch(() => props.actionId, async (id, _, onCleanup) => {
  let current = true;
  onCleanup(() => { current = false; });
  proposal.value = undefined;
  action.value = undefined;
  error.value = "";
  if (!id) return;
  busy.value = true;
  try {
    const [loaded, mcpItems, skillItems, cliItems, enablements] = await Promise.all([
      api.getResourceCreationAction(id), api.listMCPServers(), api.listSkills(),
      api.listCLIConnectorDefinitions(), api.listCLIConnectorEnablements(),
    ]);
    if (!current) return;
    mcp.value = mcpItems;
    skills.value = skillItems;
    cliConnectors.value = cliItems;
    cliEnablements.value = enablements;
    action.value = loaded;
    proposal.value = loaded.proposal;
    original.value = JSON.stringify(loaded.proposal);
  } catch {
    if (current) error.value = "无法读取创建预览，请刷新后重试。";
  } finally {
    if (current) busy.value = false;
  }
}, { immediate: true });

async function save() {
  if (!props.actionId || !proposal.value || !action.value || !editable.value) return;
  busy.value = true;
  error.value = "";
  try {
    action.value = await api.reviseResourceCreationAction(props.actionId, proposal.value, action.value.version);
    original.value = JSON.stringify(proposal.value);
  } catch {
    error.value = "修订失败，请检查指引、成员、资源或版本冲突。";
  } finally {
    busy.value = false;
  }
}
async function confirm() {
  if (!props.actionId || changed.value || !editable.value) return;
  busy.value = true;
  error.value = "";
  try {
    await api.decideResourceCreationAction(props.actionId, "confirm");
    emit("confirmed");
    emit("close");
  } catch {
    error.value = "创建失败，请修改预览或检查资源是否可用。";
  } finally {
    busy.value = false;
  }
}
</script>
<template>
<el-dialog :model-value="Boolean(actionId)" title="创建预览" width="min(720px, 96vw)" @close="emit('close')">
 <el-skeleton v-if="busy&&!proposal" :rows="4" /><el-alert v-if="error" :title="error" type="error" :closable="false" />
 <form v-if="proposal" class="editor-form" @submit.prevent="save">
  <template v-if="proposal.expert_team"><label>专家团名称<el-input v-model="proposal.expert_team.name" maxlength="100" /></label><label>简介<el-input v-model="proposal.expert_team.introduction" type="textarea" maxlength="2000" /></label><label>核心能力<el-input v-model="proposal.expert_team.core_capability" type="textarea" maxlength="20000" /></label><label>领队<el-select v-model="proposal.expert_team.lead_member_id"><el-option v-for="member in proposal.expert_team.members" :key="member.id" :value="member.id" :label="member.name" /></el-select></label></template>
  <section v-for="profile in profiles" :key="profile.id" class="editor-section">
   <label v-if="proposal.expert_team">成员名称<el-input v-model="profile.name" maxlength="100" /></label>
   <label>专家名称<el-input v-model="profile.expert.name" maxlength="100" /></label><IconPicker :model-value="profile.expert.icon || 'sparkles'" @update:model-value="profile.expert.icon=$event" @invalid="error='请选择有效图像。'" />
   <label>简介<el-input v-model="profile.expert.introduction" type="textarea" maxlength="2000" /></label><label>专家指引（Markdown）<el-input v-model="profile.expert.guidance" type="textarea" :rows="10" maxlength="100000" /></label>
   <div v-for="(_,index) in profile.expert.starter_prompts" :key="index"><label>开场提示 {{ index+1 }}<el-input v-model="profile.expert.starter_prompts![index]" maxlength="2000" /></label><el-button text @click="profile.expert.starter_prompts!.splice(index,1)">删除</el-button></div><el-button v-if="(profile.expert.starter_prompts?.length||0)<3" text @click="(profile.expert.starter_prompts ||= []).push('')">添加开场提示</el-button>
   <ExpertResourceSelector :mcp-servers="mcp" :skills="skills" :cli-connectors="cliConnectors" :cli-enablements="cliEnablements" :mcp-server-ids="profile.expert.mcp_server_ids" :skill-ids="profile.expert.skill_ids" :cli-connector-definition-ids="profile.expert.cli_connector_definition_ids" @update:mcp-server-ids="profile.expert.mcp_server_ids=$event" @update:skill-ids="profile.expert.skill_ids=$event" @update:cli-connector-definition-ids="profile.expert.cli_connector_definition_ids=$event" />
  </section>
 </form>
 <template #footer><el-button @click="emit('close')">关闭</el-button><el-button :disabled="!proposal||!changed||busy||!editable" @click="save">保存修订</el-button><el-button type="primary" :disabled="!proposal||changed||busy||!editable" @click="confirm">确认创建</el-button></template>
</el-dialog>
</template>
