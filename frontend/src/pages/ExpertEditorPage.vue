<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeft } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertInput, type MCPServer, type Skill } from "../api/client";
import ExtensionManager from "../components/ExtensionManager.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import ToastMessage from "../components/ToastMessage.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const expert = ref<Expert>();
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertInput>({ name: "", icon: "sparkles", icon_background: "sage", introduction: "", core_capability: "", operating_procedure: "", output_standard: "", cautions: "", mcp_server_ids: [], skill_ids: [] });
const isNew = route.params.expertId === "new";

onMounted(async () => {
  try {
    const [mcpItems, skillItems] = await Promise.all([api.listMCPServers(), api.listSkills()]);
    mcp.value = mcpItems;
    skills.value = skillItems;
    if (!isNew) {
      expert.value = await api.getExpert(String(route.params.expertId));
      form.value = { name: expert.value.name, icon: expert.value.icon || "sparkles", icon_background: expert.value.icon_background || "sage", introduction: expert.value.introduction, core_capability: expert.value.core_capability, operating_procedure: expert.value.operating_procedure, output_standard: expert.value.output_standard, cautions: expert.value.cautions || "", mcp_server_ids: [...expert.value.mcp_server_ids], skill_ids: [...expert.value.skill_ids] };
    }
  } catch {
    toast.value = { kind: "error", message: t("experts.loadExpertFailed") };
  }
});

async function save() {
  saving.value = true;
  try {
    if (expert.value) await api.updateExpert(expert.value.id, form.value, expert.value.version);
    else await api.createExpert(form.value);
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

function syncExtensions(value: { mcp: MCPServer[]; skills: Skill[] }) {
  mcp.value = value.mcp;
  skills.value = value.skills;
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
        <label>{{ t('experts.icon') }}<el-select v-model="form.icon"><el-option value="sparkles" label="✦ Sparkles" /><el-option value="compass" label="⌖ Compass" /><el-option value="brain" label="◉ Brain" /><el-option value="code" label="‹› Code" /></el-select></label>
        <label>{{ t('experts.iconBackground') }}<el-select v-model="form.icon_background"><el-option value="sage" :label="t('experts.sage')" /><el-option value="sand" :label="t('experts.sand')" /><el-option value="sky" :label="t('experts.sky')" /><el-option value="coral" :label="t('experts.coral')" /></el-select></label>
        <label class="full">{{ t('experts.introduction') }}<el-input v-model="form.introduction" type="textarea" :rows="3" maxlength="2000" show-word-limit /><small>{{ t('experts.descriptionHint') }}</small></label>
        <label class="full">{{ t('experts.coreCapability') }}<el-input v-model="form.core_capability" type="textarea" :rows="4" maxlength="20000" show-word-limit /></label>
        <label class="full">{{ t('experts.operatingProcedure') }}<el-input v-model="form.operating_procedure" type="textarea" :rows="6" maxlength="20000" show-word-limit /></label>
        <label class="full">{{ t('experts.outputStandard') }}<el-input v-model="form.output_standard" type="textarea" :rows="4" maxlength="20000" show-word-limit /></label>
        <label class="full">{{ t('experts.cautions') }}<el-input v-model="form.cautions" type="textarea" :rows="3" maxlength="20000" show-word-limit /></label>
      </div></section>
      <section class="editor-section"><div><h2>{{ t('experts.extensions') }}</h2><p>{{ t('experts.extensionsHint') }}</p></div><ExtensionManager selectable :mcp-server-ids="form.mcp_server_ids" :skill-ids="form.skill_ids" @update:mcp-server-ids="form.mcp_server_ids = $event" @update:skill-ids="form.skill_ids = $event" @resources="syncExtensions" @error="toast = { kind: 'error', message: t('experts.extensionFailed') }" /></section>
    </form>
  </section>

  <ConfirmDialog :open="confirmDelete" :title="t('experts.deleteExpertTitle', { name: expert?.name })" :message="t('experts.deleteExpertHint')" :confirm-label="t('experts.confirmDelete')" :cancel-label="t('common.cancel')" danger @cancel="confirmDelete = false" @confirm="remove" />
</template>
