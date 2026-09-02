<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeft, Plus, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertInput, type MCPServer, type Skill } from "../api/client";
import ExtensionManager from "../components/ExtensionManager.vue";
import ToastMessage from "../components/ToastMessage.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const expert = ref<Expert>();
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const tagDraft = ref("");
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertInput>({ name: "", capability_introduction: "", execution_instruction: "", expertise_tags: [], mcp_server_ids: [], skill_ids: [] });
const isNew = route.params.expertId === "new";

onMounted(async () => {
  try {
    [mcp.value, skills.value] = await Promise.all([api.listMCPServers(), api.listSkills()]);
    if (!isNew) {
      expert.value = await api.getExpert(String(route.params.expertId));
      form.value = { name: expert.value.name, capability_introduction: expert.value.capability_introduction, execution_instruction: expert.value.execution_instruction, expertise_tags: [...expert.value.expertise_tags], mcp_server_ids: [...expert.value.mcp_server_ids], skill_ids: [...expert.value.skill_ids] };
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
    <button class="back-link" type="button" @click="$router.push('/experts')"><ArrowLeft :size="16" /> {{ t('experts.backCatalog') }}</button>
    <header class="editor-header"><div><p class="eyebrow">EXPERT PROFILE</p><h1>{{ isNew ? t('experts.new') : t('experts.editExpert') }}</h1></div><div class="editor-actions"><button v-if="expert" class="button danger-ghost" type="button" @click="confirmDelete = true">{{ t('common.delete') }}</button><button class="button primary" :disabled="saving" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</button></div></header>
    <ToastMessage v-if="toast" :kind="toast.kind" :title="toast.kind === 'success' ? t('experts.saveSucceeded') : t('experts.operationFailed')" :message="toast.message" :close-label="t('common.close')" @dismiss="toast = undefined" />

    <form class="editor-form" @submit.prevent="save">
      <section class="editor-section"><div><span>01</span><h2>{{ t('experts.basic') }}</h2><p>{{ t('experts.basicHint') }}</p></div><div class="form-grid">
        <label>{{ t('experts.name') }}<input v-model="form.name" maxlength="100" required></label>
        <label class="full">{{ t('experts.capability') }}<textarea v-model="form.capability_introduction" rows="4" maxlength="2000" required></textarea></label>
        <label class="full">{{ t('experts.instruction') }}<textarea v-model="form.execution_instruction" rows="8" maxlength="20000" required></textarea><small>{{ t('experts.instructionHint') }}</small></label>
      </div></section>
      <section class="editor-section"><div><span>02</span><h2>{{ t('experts.expertise') }}</h2><p>{{ t('experts.expertiseHint') }}</p></div><div><label class="tag-input">{{ t('experts.addTag') }}<div><input v-model="tagDraft" maxlength="20" :placeholder="t('experts.tagPlaceholder')" @keydown.enter.prevent="addTag"><button type="button" :aria-label="t('experts.addTag')" @click="addTag"><Plus :size="17" /></button></div></label><div class="editable-tags"><span v-for="(item, index) in form.expertise_tags" :key="item">{{ item }}<button type="button" :aria-label="`${t('common.delete')} ${item}`" @click="form.expertise_tags.splice(index, 1)"><X :size="13" /></button></span></div></div></section>
      <section class="editor-section"><div><span>03</span><h2>{{ t('experts.extensions') }}</h2><p>{{ t('experts.extensionsHint') }}</p></div><ExtensionManager selectable :mcp-server-ids="form.mcp_server_ids" :skill-ids="form.skill_ids" @update:mcp-server-ids="form.mcp_server_ids = $event" @update:skill-ids="form.skill_ids = $event" @resources="syncExtensions" @error="toast = { kind: 'error', message: t('experts.extensionFailed') }" /></section>
    </form>
  </section>

  <div v-if="confirmDelete" class="modal-layer" @click.self="confirmDelete = false"><section class="modal-card"><p class="eyebrow">DELETE EXPERT</p><h2>{{ t('experts.deleteExpertTitle', { name: expert?.name }) }}</h2><p>{{ t('experts.deleteExpertHint') }}</p><div class="modal-actions"><button class="button ghost" @click="confirmDelete = false">{{ t('common.cancel') }}</button><button class="button danger" @click="remove">{{ t('experts.confirmDelete') }}</button></div></section></div>
</template>
