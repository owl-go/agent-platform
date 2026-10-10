<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeft } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type CLIConnectorDefinition, type CLIConnectorEnablement, type Expert, type ExpertInput, type MCPServer, type Skill } from "../api/client";
import ExpertResourceSelector from "../components/ExpertResourceSelector.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import ToastMessage from "../components/ToastMessage.vue";
import IconPicker from "../components/IconPicker.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const expert = ref<Expert>();
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const cliConnectors = ref<CLIConnectorDefinition[]>([]);
const cliEnablements = ref<CLIConnectorEnablement[]>([]);
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertInput>({ starter_prompts: [], name: "", icon: "sparkles", icon_background: "sage", introduction: "", guidance: "", core_capability: "", operating_procedure: "", output_standard: "", cautions: "", mcp_server_ids: [], skill_ids: [], cli_connector_definition_ids: [] });
const isNew = route.name === "expert-new" || route.params.expertId === "new";

onMounted(async () => {
  try {
    const [mcpItems, skillItems, cliItems, enablementItems] = await Promise.all([api.listMCPServers(), api.listSkills(), api.listCLIConnectorDefinitions(), api.listCLIConnectorEnablements()]);
    mcp.value = mcpItems;
    skills.value = skillItems;
    cliConnectors.value = cliItems;
    cliEnablements.value = enablementItems;
    if (!isNew) {
      expert.value = await api.getExpert(String(route.params.expertId));
      form.value = { connector_dependencies:expert.value.connector_dependencies || [], starter_prompts: expert.value.starter_prompts || [], name: expert.value.name, icon: expert.value.icon || "sparkles", icon_background: expert.value.icon_background || "sage", introduction: expert.value.introduction, guidance: expert.value.guidance || "", core_capability: expert.value.core_capability, operating_procedure: expert.value.operating_procedure, output_standard: expert.value.output_standard, cautions: expert.value.cautions || "", mcp_server_ids: [...expert.value.mcp_server_ids], skill_ids: [...expert.value.skill_ids], cli_connector_definition_ids: [...expert.value.cli_connector_definition_ids] };
    }
  } catch {
    toast.value = { kind: "error", message: t("experts.loadExpertFailed") };
  }
});

async function save() {
  saving.value = true;
  try {
    const {core_capability: _core,operating_procedure: _procedure,output_standard: _output,cautions: _cautions,...input}=form.value;
    if (expert.value) await api.updateExpert(expert.value.id, input, expert.value.version);
    else await api.createExpert(input);
    toast.value = { kind: "success", message: t("experts.saved") };
    window.setTimeout(() => void router.push("/experts"), 350);
  } catch {
    toast.value = { kind: "error", message: t("experts.saveFailed") };
  } finally {
    saving.value = false;
  }
}

async function remove() {
  if (!expert.value) return;
  try {
    await api.deleteExpert(expert.value.id);
    await router.push("/experts");
  } catch {
    toast.value = { kind: "error", message: t("experts.deleteExpertFailed") };
  }
}

</script>

<template>
  <section class="page-surface editor-page">
    <el-button class="back-link" text :icon="ArrowLeft" @click="$router.push('/experts')">{{ t('experts.backCatalog') }}</el-button>
    <header class="editor-header"><div><h1>{{ isNew ? t('experts.new') : t('experts.editExpert') }}</h1></div><div class="editor-actions"><el-button v-if="expert" type="danger" plain @click="confirmDelete = true">{{ t('common.delete') }}</el-button><el-button type="primary" :loading="saving" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</el-button></div></header>
    <ToastMessage v-if="toast" :kind="toast.kind" :title="toast.kind === 'success' ? t('experts.saveSucceeded') : t('experts.operationFailed')" :message="toast.message" :close-label="t('common.close')" @dismiss="toast = undefined" />

    <form class="editor-form" @submit.prevent="save">
      <section class="editor-section"><div><h2>{{ t('experts.basic') }}</h2><p>{{ t('experts.basicHint') }}</p></div><div class="form-grid">
        <label>{{ t('experts.name') }}<el-input v-model="form.name" maxlength="100" show-word-limit /></label>
        <div class="form-field"><span>{{ t('experts.icon') }}</span><IconPicker @invalid="toast = {kind: 'error', message: '请选择不超过 2 MiB 的有效图像'}" v-model="form.icon" fallback="sparkles" /></div>
        <label>{{ t('experts.iconBackground') }}<el-select v-model="form.icon_background"><el-option value="sage" :label="t('experts.sage')" /><el-option value="sand" :label="t('experts.sand')" /><el-option value="sky" :label="t('experts.sky')" /><el-option value="coral" :label="t('experts.coral')" /></el-select></label>
        <div class="form-field full"><span>开场提示（最多三条）</span><div v-for="(_, index) in form.starter_prompts" :key="index"><el-input v-model="form.starter_prompts![index]" maxlength="2000" :aria-label="`开场提示 ${index + 1}`" /><el-button text @click="form.starter_prompts!.splice(index, 1)">删除</el-button></div><el-button v-if="(form.starter_prompts?.length || 0) < 3" text @click="(form.starter_prompts ||= []).push('')">添加开场提示</el-button></div>
        <label class="full">{{ t('experts.introduction') }}<el-input v-model="form.introduction" type="textarea" :rows="3" maxlength="2000" show-word-limit /><small>{{ t('experts.descriptionHint') }}</small></label>
        <label class="full">{{ t('experts.guidance') }}<el-input v-model="form.guidance" type="textarea" :rows="14" maxlength="100000" show-word-limit /><small>{{ t('experts.guidanceHint') }}</small></label>
      </div></section>
      <section class="editor-section"><div><h2>{{ t('experts.extensions') }}</h2><p>{{ t('experts.extensionsHint') }}</p></div><ExpertResourceSelector :mcp-servers="mcp" :skills="skills" :cli-connectors="cliConnectors" :cli-enablements="cliEnablements" :mcp-server-ids="form.mcp_server_ids" :skill-ids="form.skill_ids" :cli-connector-definition-ids="form.cli_connector_definition_ids" @update:mcp-server-ids="form.mcp_server_ids = $event" @update:skill-ids="form.skill_ids = $event" @update:cli-connector-definition-ids="form.cli_connector_definition_ids = $event" /></section>
    </form>
  </section>

  <ConfirmDialog :open="confirmDelete" :title="t('experts.deleteExpertTitle', { name: expert?.name })" :message="t('experts.deleteExpertHint')" :confirm-label="t('experts.confirmDelete')" :cancel-label="t('common.cancel')" danger @cancel="confirmDelete = false" @confirm="remove" />
</template>
