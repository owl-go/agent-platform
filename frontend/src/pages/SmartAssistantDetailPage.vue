<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Download, Pencil, Trash2, Upload } from "@lucide/vue";
import * as XLSX from "xlsx";
import IconPicker from "../components/IconPicker.vue";
import { assistantModelOptions } from "../assistantModels";
import { ApiError, platformApiKey, type ApplicationKnowledgeBase, type DigitalHuman, type ModelProviderConnection, type SmartAssistant, type SmartAssistantFAQ, type SmartAssistantInput } from "../api/client";
import { renderMarkdown } from "../markdown";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const assistant = ref<SmartAssistant>();
const faqs = ref<SmartAssistantFAQ[]>([]);
const knowledgeBases = ref<ApplicationKnowledgeBase[]>([]);
const digitalHumans = ref<DigitalHuman[]>([]);
const connections = ref<ModelProviderConnection[]>([]);
const modelOptions = computed(() => assistantModelOptions(connections.value));
const error = ref("");
const saving = ref(false);
const iconUploading = ref(false);
const iconPreviewUrl = ref("");
const activeTab = ref<"basic" | "faq">("basic");
const faqDialogOpen = ref(false);
const editingFAQ = ref<SmartAssistantFAQ>();
const faqDraft = ref({ question: "", answer_markdown: "" });
const id = String(route.params.assistantId);
const assistantEnabled = computed({ get: () => assistant.value?.state === "enabled", set: (value: boolean) => { if (assistant.value) assistant.value.state = value ? "enabled" : "disabled"; } });

function replaceIconPreview(value = "") {
  if (iconPreviewUrl.value.startsWith("blob:")) URL.revokeObjectURL(iconPreviewUrl.value);
  iconPreviewUrl.value = value;
}
async function loadIconPreview() {
  if (!assistant.value?.icon.startsWith("ai-applications/assistant-icons/")) { replaceIconPreview(); return; }
  try { replaceIconPreview(URL.createObjectURL(await api.getSmartAssistantIcon(id))); }
  catch { error.value = t("aiApplications.iconLoadFailed"); }
}
async function refresh() {
  try {
    [assistant.value, faqs.value, knowledgeBases.value, digitalHumans.value, connections.value] = await Promise.all([api.getSmartAssistant(id), api.listAssistantFAQs(id), api.listApplicationKnowledgeBases(), api.listDigitalHumans(), api.listModelProviderConnections()]);
    await loadIconPreview();
  } catch (cause) { error.value = errorMessage(cause, "aiApplications.loadFailed"); }
}
function input(state = assistant.value?.state): SmartAssistantInput | undefined {
  if (!assistant.value) return undefined;
  return { name: assistant.value.name.trim(), icon: assistant.value.icon, description: assistant.value.description ?? "", introduction: assistant.value.introduction, scenario: assistant.value.scenario, prompt: assistant.value.prompt ?? "", preprocess_prompt: assistant.value.preprocess_prompt ?? "", provider_model_id: assistant.value.provider_model_id, response_style: assistant.value.response_style, knowledge_base_ids: assistant.value.knowledge_base_ids, expert_id: assistant.value.expert_id, expert_team_id: assistant.value.expert_team_id, digital_human_id: assistant.value.digital_human_id || undefined, state, share: { enabled: assistant.value.share.enabled, allowed_origins: assistant.value.share.allowed_origins ?? [], width: assistant.value.share.width || "100%", height: assistant.value.share.height || 600, free_text_enabled: assistant.value.share.free_text_enabled ?? false, daily_call_limit: assistant.value.share.daily_call_limit ?? 0, data_processing_acknowledged: assistant.value.share.data_processing_acknowledged ?? false } };
}
function errorMessage(cause: unknown, fallback: string) {
  if (!(cause instanceof ApiError)) return t(fallback);
  if (cause.code === "invalid_input") return t("aiApplications.validationFailed");
  if (cause.code === "version_conflict") return t("aiApplications.versionConflict");
  if (cause.code === "resource_conflict") return t("aiApplications.conflict");
  if (cause.code === "assistant_model_unavailable") return t("aiApplications.modelUnavailable");
  return t(fallback);
}
async function save(state = assistant.value?.state) { if (!assistant.value) return false; const payload = input(state); if (!payload) return false; if (!payload.provider_model_id || !modelOptions.value.some((option) => option.value === payload.provider_model_id)) { error.value = t("aiApplications.modelUnavailable"); return false; } saving.value = true; try { assistant.value = await api.updateSmartAssistant(id, payload, assistant.value.version); return true; } catch (cause) { error.value = errorMessage(cause, "aiApplications.saveFailed"); return false; } finally { saving.value = false; } }
async function uploadIcon(file: File) {
  if (!assistant.value) return;
  const draft = assistant.value;
  error.value = "";
  iconUploading.value = true;
  const previousPreview = iconPreviewUrl.value;
  if (typeof URL.createObjectURL === "function") replaceIconPreview(URL.createObjectURL(file));
  try {
    const updated = await api.uploadSmartAssistantIcon(id, file, draft.version);
    assistant.value = { ...draft, icon: updated.icon, version: updated.version, updated_at: updated.updated_at };
  }
  catch (cause) {
    replaceIconPreview(previousPreview.startsWith("blob:") ? "" : previousPreview);
    error.value = errorMessage(cause, "aiApplications.iconUploadFailed");
    await loadIconPreview();
  } finally { iconUploading.value = false; }
}
function invalidIcon() { error.value = t("aiApplications.iconInvalid"); }
function resetFAQDraft() { faqDraft.value = { question: "", answer_markdown: "" }; }
function openAddFAQ() { editingFAQ.value = undefined; resetFAQDraft(); faqDialogOpen.value = true; }
function openEditFAQ(faq: SmartAssistantFAQ) { editingFAQ.value = faq; faqDraft.value = { question: faq.question, answer_markdown: faq.answer_markdown }; faqDialogOpen.value = true; }
async function submitFAQ() {
  const question = faqDraft.value.question.trim();
  const answer_markdown = faqDraft.value.answer_markdown.trim();
  if (!question || !answer_markdown) return;
  try {
    if (editingFAQ.value) {
      const faq = editingFAQ.value;
      const updated = await api.updateAssistantFAQ(id, faq.id, { question, answer_markdown, display_order: faq.display_order, category: faq.category, tag: faq.tag, icon: faq.icon, enabled: faq.enabled }, faq.version);
      faqs.value = faqs.value.map((candidate) => candidate.id === updated.id ? updated : candidate);
    } else {
      const created = await api.createAssistantFAQ(id, { question, answer_markdown, display_order: faqs.value.length, category: "", tag: "", icon: "", enabled: true });
      faqs.value = [...faqs.value, created].sort((left, right) => left.display_order - right.display_order);
    }
    faqDialogOpen.value = false;
  } catch (cause) { error.value = errorMessage(cause, "aiApplications.saveFailed"); }
}
async function toggleFAQ(faq: SmartAssistantFAQ) { try { const updated = await api.updateAssistantFAQ(id, faq.id, { question: faq.question, answer_markdown: faq.answer_markdown, display_order: faq.display_order, category: faq.category, tag: faq.tag, icon: faq.icon, enabled: !faq.enabled }, faq.version); faqs.value = faqs.value.map((candidate) => candidate.id === updated.id ? updated : candidate); } catch (cause) { error.value = errorMessage(cause, "aiApplications.saveFailed"); } }
async function moveFAQ(faq: SmartAssistantFAQ, direction: -1 | 1) { const index = faqs.value.findIndex((candidate) => candidate.id === faq.id); const other = faqs.value[index + direction]; if (!other) return; try { const [updated, swapped] = await Promise.all([api.updateAssistantFAQ(id, faq.id, { question: faq.question, answer_markdown: faq.answer_markdown, display_order: other.display_order, category: faq.category, tag: faq.tag, icon: faq.icon, enabled: faq.enabled }, faq.version), api.updateAssistantFAQ(id, other.id, { question: other.question, answer_markdown: other.answer_markdown, display_order: faq.display_order, category: other.category, tag: other.tag, icon: other.icon, enabled: other.enabled }, other.version)]); faqs.value = faqs.value.map((candidate) => candidate.id === updated.id ? updated : candidate.id === swapped.id ? swapped : candidate).sort((left, right) => left.display_order - right.display_order); } catch (cause) { error.value = errorMessage(cause, "aiApplications.saveFailed"); } }
async function removeFAQ(faq: SmartAssistantFAQ) { try { await api.deleteAssistantFAQ(id, faq.id); faqs.value = faqs.value.filter((item) => item.id !== faq.id); } catch (cause) { error.value = errorMessage(cause, "aiApplications.deleteFailed"); } }
function headerIndex(value: unknown) { return String(value ?? "").replace(/^\uFEFF/, "").trim().toLowerCase(); }
function rowValue(row: unknown[], index: number) { return String(row[index] ?? "").trim(); }
async function importFAQs(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  try {
    const bytes = typeof file.arrayBuffer === "function" ? await file.arrayBuffer() : await new Response(file).arrayBuffer();
    const workbook = XLSX.read(bytes, { type: "array" });
    const sheetName = workbook.SheetNames[0];
    if (!sheetName) throw new Error("missing_sheet");
    const rows = XLSX.utils.sheet_to_json<unknown[]>(workbook.Sheets[sheetName], { header: 1, defval: "" });
    const headers = rows[0] ?? [];
    const questionIndex = headers.findIndex((value) => ["问题", "question"].includes(headerIndex(value)));
    const answerIndex = headers.findIndex((value) => ["答案", "answer", "answer (markdown)", "answer_markdown"].includes(headerIndex(value)));
    if (questionIndex < 0 || answerIndex < 0) throw new Error("invalid_headers");
    for (const row of rows.slice(1)) {
      const question = rowValue(row, questionIndex);
      const answer_markdown = rowValue(row, answerIndex);
      if (!question || !answer_markdown) continue;
      const existing = faqs.value.find((faq) => faq.question.trim() === question);
      if (existing) {
        const updated = await api.updateAssistantFAQ(id, existing.id, { question, answer_markdown, display_order: existing.display_order, category: existing.category, tag: existing.tag, icon: existing.icon, enabled: existing.enabled }, existing.version);
        faqs.value = faqs.value.map((faq) => faq.id === updated.id ? updated : faq);
      } else {
        const created = await api.createAssistantFAQ(id, { question, answer_markdown, display_order: faqs.value.length, category: "", tag: "", icon: "", enabled: true });
        faqs.value = [...faqs.value, created];
      }
    }
    faqs.value = [...faqs.value].sort((left, right) => left.display_order - right.display_order);
  } catch { error.value = t("aiApplications.faq.importFailed"); }
}
function exportFAQs() {
  const sheet = XLSX.utils.json_to_sheet(faqs.value.map((faq) => ({ 问题: faq.question, 答案: faq.answer_markdown })), { header: ["问题", "答案"] });
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, sheet, "FAQ");
  const bytes = XLSX.write(workbook, { type: "array", bookType: "xlsx" });
  const url = URL.createObjectURL(new Blob([bytes], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `${assistant.value?.name || "assistant"}-FAQ.xlsx`;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}
onMounted(refresh);
onBeforeUnmount(() => replaceIconPreview());
</script>

<template>
  <section class="application-detail-page">
    <header class="application-detail-header"><el-button text @click="router.push('/ai-apps/assistants')">← {{ t('common.back') }}</el-button></header>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <template v-if="assistant">
      <nav class="assistant-detail-tabs" role="tablist"><button type="button" :class="{ active: activeTab === 'basic' }" role="tab" :aria-selected="activeTab === 'basic'" @click="activeTab = 'basic'">{{ t('aiApplications.basic') }}</button><button type="button" :class="{ active: activeTab === 'faq' }" role="tab" :aria-selected="activeTab === 'faq'" @click="activeTab = 'faq'">{{ t('aiApplications.faq.title') }}</button></nav>
      <el-card v-if="activeTab === 'basic'" class="application-detail-card"><el-form label-position="top"><el-form-item :label="t('aiApplications.name')"><el-input v-model="assistant.name" /></el-form-item><el-form-item :label="t('aiApplications.icon')" required><IconPicker v-model="assistant.icon" upload-only remote-upload :preview-url="iconPreviewUrl" :uploading="iconUploading" @file-selected="uploadIcon" @invalid="invalidIcon" /></el-form-item><el-form-item :label="t('aiApplications.model')" required><el-select v-model="assistant.provider_model_id" data-testid="assistant-model-select" :placeholder="t('aiApplications.modelPlaceholder')"><el-option v-for="option in modelOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select><small v-if="assistant.provider_model_id && !modelOptions.some((option) => option.value === assistant!.provider_model_id)">{{ t('aiApplications.modelUnavailable') }}</small></el-form-item><el-form-item :label="t('aiApplications.welcome')"><el-input v-model="assistant.introduction" type="textarea" :rows="3" :placeholder="t('aiApplications.welcomePlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.description')"><el-input v-model="assistant.description" type="textarea" :rows="3" /></el-form-item><el-form-item :label="t('aiApplications.prompt')"><el-input v-model="assistant.prompt" type="textarea" :rows="6" /></el-form-item><el-form-item :label="t('aiApplications.preprocessPrompt')"><el-input v-model="assistant.preprocess_prompt" type="textarea" :rows="5" /></el-form-item><el-form-item :label="t('aiApplications.responseStyle')"><el-input v-model="assistant.response_style" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.binding')"><el-select v-model="assistant.digital_human_id" clearable :placeholder="t('aiApplications.digitalHuman.selectPlaceholder')"><el-option v-for="human in digitalHumans.filter((item) => item.state === 'enabled')" :key="human.id" :label="human.name" :value="human.id" /></el-select></el-form-item><el-form-item :label="t('aiApplications.state')"><el-switch v-model="assistantEnabled" /></el-form-item><el-form-item :label="t('aiApplications.knowledge.selected')"><el-select v-model="assistant.knowledge_base_ids" multiple collapse-tags :placeholder="t('aiApplications.knowledge.selectPlaceholder')"><el-option v-for="base in knowledgeBases" :key="base.id" :label="base.name" :value="base.id" /></el-select></el-form-item></el-form></el-card>
      <el-card v-else class="application-detail-card"><template #header><div class="faq-list-header"><strong>{{ t('aiApplications.faq.title') }}</strong><div class="faq-list-actions"><el-tooltip :content="t('aiApplications.faq.import')" placement="top"><el-button data-testid="faq-import" class="faq-tool-button" text circle :icon="Upload" :aria-label="t('aiApplications.faq.import')" @click="($refs.faqImport as HTMLInputElement)?.click()" /></el-tooltip><input ref="faqImport" data-testid="faq-import-file" class="faq-import-input" type="file" accept=".xlsx,.xls" @change="importFAQs"><el-tooltip :content="t('aiApplications.faq.export')" placement="top"><el-button data-testid="faq-export" class="faq-tool-button" text circle :icon="Download" :aria-label="t('aiApplications.faq.export')" @click="exportFAQs" /></el-tooltip><el-button data-testid="faq-add" type="primary" @click="openAddFAQ">{{ t('aiApplications.faq.add') }}</el-button></div></div></template><el-table v-if="faqs.length" :data="faqs" class="faq-table"><el-table-column min-width="240" :label="t('aiApplications.faq.question')"><template #default="scope"><strong>{{ (scope.row as SmartAssistantFAQ).question }}</strong></template></el-table-column><el-table-column min-width="360" :label="t('aiApplications.faq.answer')"><template #default="scope"><div class="markdown-body faq-answer" v-html="renderMarkdown((scope.row as SmartAssistantFAQ).answer_markdown)"></div></template></el-table-column><el-table-column width="150" :label="t('aiApplications.actions')" align="right"><template #default="scope"><div class="faq-actions"><el-button text :icon="Pencil" @click="openEditFAQ(scope.row as SmartAssistantFAQ)">{{ t('common.edit') }}</el-button><el-button text type="danger" :icon="Trash2" @click="removeFAQ(scope.row as SmartAssistantFAQ)">{{ t('common.delete') }}</el-button></div></template></el-table-column></el-table><el-empty v-else :description="t('aiApplications.faq.empty')" /><el-dialog v-model="faqDialogOpen" class="faq-dialog" :title="editingFAQ ? t('aiApplications.faq.edit') : t('aiApplications.faq.add')" width="min(620px, 92vw)" destroy-on-close data-testid="faq-dialog"><el-form label-position="top"><el-form-item data-testid="faq-dialog-question" :label="t('aiApplications.faq.question')" required><el-input v-model="faqDraft.question" /></el-form-item><el-form-item data-testid="faq-dialog-answer" :label="t('aiApplications.faq.answer')" required><el-input v-model="faqDraft.answer_markdown" type="textarea" :rows="6" /></el-form-item></el-form><template #footer><el-button @click="faqDialogOpen = false">{{ t('common.cancel') }}</el-button><el-button data-testid="faq-dialog-submit" type="primary" @click="submitFAQ">{{ t('common.save') }}</el-button></template></el-dialog></el-card>
      <footer class="assistant-detail-footer"><el-button type="primary" :loading="saving || iconUploading" :disabled="iconUploading" @click="save()">{{ t('common.save') }}</el-button></footer>
    </template>
  </section>
</template>
