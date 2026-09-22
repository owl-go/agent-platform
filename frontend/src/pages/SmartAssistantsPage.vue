<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { Plus } from "@element-plus/icons-vue";
import { Pause, Play, Search, Share2, Trash2 } from "@lucide/vue";
import { ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import IconPicker from "../components/IconPicker.vue";
import SmartAssistantShareDialog from "../components/SmartAssistantShareDialog.vue";
import { platformApiKey, type SmartAssistant, type SmartAssistantInput } from "../api/client";

const api = inject(platformApiKey)!;
const router = useRouter();
const { t } = useI18n();
const items = ref<SmartAssistant[]>([]);
const loading = ref(false);
const error = ref("");
const search = ref("");
const scenario = ref("");
const createDialogOpen = ref(false);
const form = ref<SmartAssistantInput & { icon: string }>({ name: "", icon: "", description: "", scenario: "custom", introduction: "", prompt: "", preprocess_prompt: "", response_style: "" });
const shareAssistant = ref<SmartAssistant>();
const shareDialogOpen = ref(false);
const scenarios = ["customer-consultation", "pre-sales-advisor", "after-sales-support", "product-guide", "enterprise-knowledge", "recruitment", "training", "custom"];
const filteredItems = computed(() => {
  const query = search.value.trim().toLocaleLowerCase();
  return items.value.filter((item) => (!query || item.name.toLocaleLowerCase().includes(query)) && (!scenario.value || item.scenario === scenario.value));
});

async function refresh() {
  loading.value = true;
  try { items.value = await api.listSmartAssistants(); }
  catch { error.value = t("aiApplications.loadFailed"); }
  finally { loading.value = false; }
}
async function create() {
  if (!form.value.name.trim() || !form.value.icon?.trim()) return;
  try {
    const item = await api.createSmartAssistant({ ...form.value, share: { enabled: false, width: "100%", height: 600 } });
    items.value.unshift(item);
    form.value = { name: "", icon: "", description: "", scenario: "custom", introduction: "", prompt: "", preprocess_prompt: "", response_style: "" };
    createDialogOpen.value = false;
  } catch { error.value = t("aiApplications.saveFailed"); }
}
function openCreate() { createDialogOpen.value = true; }
function openShare(item: SmartAssistant) { shareAssistant.value = item; shareDialogOpen.value = true; }
function updateSharedAssistant(updated: SmartAssistant) { items.value = items.value.map((item) => item.id === updated.id ? updated : item); shareAssistant.value = updated; }
function showShareError(message: string) { error.value = message; }
async function toggleState(item: SmartAssistant) {
  const next = item.state === "enabled" ? "disabled" : "enabled";
  try {
    const updated = await api.setSmartAssistantState(item.id, next, item.version);
    items.value = items.value.map((candidate) => candidate.id === updated.id ? updated : candidate);
  } catch { error.value = t("aiApplications.saveFailed"); }
}
async function remove(item: SmartAssistant) {
  try {
    await ElMessageBox.confirm(t("aiApplications.confirmDelete", { name: item.name }), t("common.delete"), { type: "warning", confirmButtonText: t("common.delete"), cancelButtonText: t("common.cancel") });
    await api.deleteSmartAssistant(item.id);
    items.value = items.value.filter((candidate) => candidate.id !== item.id);
  } catch (cause) {
    if (cause !== "cancel" && cause !== "close") error.value = t("aiApplications.deleteFailed");
  }
}
onMounted(refresh);
</script>

<template>
  <section class="application-catalog-page">
    <div class="application-catalog-toolbar">
      <div><h2>{{ t('aiApplications.assistants.title') }}</h2></div>
      <el-button class="application-create-trigger" type="primary" :icon="Plus" @click="openCreate">{{ t('aiApplications.create') }}</el-button>
    </div>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <el-dialog v-model="createDialogOpen" class="application-create-dialog" :title="t('aiApplications.create')" width="min(560px, 92vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="create">
        <el-form-item :label="t('aiApplications.name')"><el-input v-model="form.name" autofocus :placeholder="t('aiApplications.assistantNamePlaceholder')" /></el-form-item>
        <el-form-item :label="t('aiApplications.icon')" required><IconPicker v-model="form.icon" upload-only /></el-form-item>
        <el-form-item :label="t('aiApplications.scenario')"><el-select v-model="form.scenario"><el-option v-for="value in scenarios" :key="value" :label="t(`aiApplications.scenarios.${value}`)" :value="value" /></el-select></el-form-item>
        <el-form-item :label="t('aiApplications.description')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="createDialogOpen = false">{{ t('common.cancel') }}</el-button><el-button type="primary" @click="create">{{ t('common.save') }}</el-button></template>
    </el-dialog>
    <div class="application-list-filters">
      <el-input v-model="search" class="assistant-search" clearable :placeholder="t('aiApplications.search')" />
      <el-select v-model="scenario" clearable :placeholder="t('aiApplications.allScenarios')"><el-option v-for="value in scenarios" :key="value" :label="t(`aiApplications.scenarios.${value}`)" :value="value" /></el-select>
      <el-button class="assistant-search-button" type="primary" :icon="Search" @click="search = search.trim()">{{ t('aiApplications.searchAction') }}</el-button>
    </div>
    <div v-loading="loading" class="application-card-grid">
      <el-card v-for="item in filteredItems" :key="item.id" class="application-card" role="button" tabindex="0" @click="router.push(`/ai-apps/assistants/${item.id}`)" @keydown.enter="router.push(`/ai-apps/assistants/${item.id}`)">
        <div class="application-card-heading">
          <div><h3>{{ item.name }}</h3><p>{{ item.description || item.introduction || t('aiApplications.assistants.defaultIntro') }}</p></div>
          <div class="application-card-actions">
            <el-button text :icon="item.state === 'enabled' ? Pause : Play" :aria-label="item.state === 'enabled' ? t('aiApplications.disable') : t('aiApplications.enable')" @click.stop="toggleState(item)" />
            <el-button text :icon="Share2" @click.stop="openShare(item)">{{ t('aiApplications.share.title') }}</el-button>
            <el-button text type="danger" :icon="Trash2" :aria-label="t('common.delete')" @click.stop="remove(item)" />
          </div>
        </div>
        <small>{{ t(`aiApplications.states.${item.state}`) }} · {{ item.share.enabled ? t('aiApplications.shared') : t('aiApplications.private') }}</small>
      </el-card>
      <el-empty v-if="!loading && !filteredItems.length" :description="t('aiApplications.assistants.empty')" />
    </div>
    <SmartAssistantShareDialog v-model="shareDialogOpen" :assistant="shareAssistant" @updated="updateSharedAssistant" @error="showShareError" />
  </section>
</template>
