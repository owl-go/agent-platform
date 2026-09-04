<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeft, Plus } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, runtimeEngineDisplayName, type Expert, type ExpertInput, type MCPServer, type ModelProviderConnection, type RuntimeEngineStatus, type Skill } from "../api/client";
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
const connections = ref<ModelProviderConnection[]>([]);
const runtimes = ref<RuntimeEngineStatus[]>([]);
const tagDraft = ref("");
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertInput>({ name: "", capability_introduction: "", execution_instruction: "", provider_model_id: "", runtime_engine: "codex", expertise_tags: [], mcp_server_ids: [], skill_ids: [] });
const isNew = route.params.expertId === "new";
const executionProfileConfirmed = ref(!isNew);
const selectedModel = computed(() => connections.value.flatMap((connection) => connection.models).find((model) => model.id === form.value.provider_model_id));
const selectedCompatibility = computed(() => selectedModel.value?.compatibility.find((item) => item.runtime_engine === form.value.runtime_engine));

onMounted(async () => {
  try {
    const [mcpItems, skillItems, connectionItems, runtimeItems, settings] = await Promise.all([api.listMCPServers(), api.listSkills(), api.listModelProviderConnections(), api.listRuntimeEngines(), api.getSettings()]);
    mcp.value = mcpItems;
    skills.value = skillItems;
    connections.value = connectionItems;
    runtimes.value = runtimeItems;
    if (!isNew) {
      expert.value = await api.getExpert(String(route.params.expertId));
      form.value = { name: expert.value.name, capability_introduction: expert.value.capability_introduction, execution_instruction: expert.value.execution_instruction, provider_model_id: expert.value.provider_model_id, runtime_engine: expert.value.runtime_engine || "codex", expertise_tags: [...expert.value.expertise_tags], mcp_server_ids: [...expert.value.mcp_server_ids], skill_ids: [...expert.value.skill_ids] };
    } else {
      form.value.runtime_engine = settings.default_runtime_engine;
      form.value.provider_model_id = settings.runtime_model_defaults.find((item) => item.runtime_engine === settings.default_runtime_engine)?.provider_model_id ?? "";
    }
  } catch {
    toast.value = { kind: "error", message: t("experts.loadExpertFailed") };
  }
});

function addTag() {
  const value = tagDraft.value.trim();
  if (!value || form.value.expertise_tags.some((item) => item.toLocaleLowerCase() === value.toLocaleLowerCase()) || form.value.expertise_tags.length >= 10) return;
  form.value.expertise_tags.push(value.slice(0, 20));
  tagDraft.value = "";
}

async function save() {
  if (!executionProfileConfirmed.value) {
    toast.value = { kind: "error", message: t("experts.confirmProfileRequired") };
    return;
  }
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
        <label class="full">{{ t('experts.capability') }}<el-input v-model="form.capability_introduction" type="textarea" :rows="4" maxlength="2000" show-word-limit /></label>
        <label class="full">{{ t('experts.instruction') }}<el-input v-model="form.execution_instruction" type="textarea" :rows="8" maxlength="20000" show-word-limit /><small>{{ t('experts.instructionHint') }}</small></label>
      </div></section>
      <section class="editor-section"><div><h2>{{ t('workflows.execution') }}</h2><p>{{ t('experts.basicHint') }}</p></div><div class="form-grid">
        <label>{{ t('workflows.runtime') }}<el-select v-model="form.runtime_engine" @change="executionProfileConfirmed = false"><el-option v-for="runtime in runtimes" :key="runtime.name" :value="runtime.name" :label="`${runtimeEngineDisplayName(runtime.name)} · ${runtime.available ? t('settings.available') : t('settings.unavailable')}`" :disabled="!runtime.available" /></el-select></label>
        <label>{{ t('settings.model') }}<el-select v-model="form.provider_model_id" filterable @change="executionProfileConfirmed = false"><el-option-group v-for="connection in connections" :key="connection.id" :label="connection.name"><el-option v-for="model in connection.models" :key="model.id" :value="model.id" :label="model.display_name" :disabled="!model.available || model.compatibility.find((item) => item.runtime_engine === form.runtime_engine)?.status === 'incompatible'" /></el-option-group></el-select><small v-if="selectedCompatibility?.status === 'unverified'">{{ t('sessions.modelUnverified') }}</small></label>
        <el-checkbox v-model="executionProfileConfirmed" class="profile-confirm full">{{ t('experts.confirmProfile') }}</el-checkbox>
      </div></section>
      <section class="editor-section"><div><h2>{{ t('experts.expertise') }}</h2><p>{{ t('experts.expertiseHint') }}</p></div><div><label class="tag-input">{{ t('experts.addTag') }}<div><el-input v-model="tagDraft" maxlength="20" :placeholder="t('experts.tagPlaceholder')" @keydown.enter.prevent="addTag" /><el-button :icon="Plus" :aria-label="t('experts.addTag')" @click="addTag" /></div></label><div class="editable-tags"><el-tag v-for="(item, index) in form.expertise_tags" :key="item" closable type="info" @close="form.expertise_tags.splice(index, 1)">{{ item }}</el-tag></div></div></section>
      <section class="editor-section"><div><h2>{{ t('experts.extensions') }}</h2><p>{{ t('experts.extensionsHint') }}</p></div><ExtensionManager selectable :mcp-server-ids="form.mcp_server_ids" :skill-ids="form.skill_ids" @update:mcp-server-ids="form.mcp_server_ids = $event" @update:skill-ids="form.skill_ids = $event" @resources="syncExtensions" @error="toast = { kind: 'error', message: t('experts.extensionFailed') }" /></section>
    </form>
  </section>

  <ConfirmDialog :open="confirmDelete" :title="t('experts.deleteExpertTitle', { name: expert?.name })" :message="t('experts.deleteExpertHint')" :confirm-label="t('experts.confirmDelete')" :cancel-label="t('common.cancel')" danger @cancel="confirmDelete = false" @confirm="remove" />
</template>
