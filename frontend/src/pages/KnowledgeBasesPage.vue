<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { Folder, Plus, Upload } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { platformApiKey, type KnowledgeBase, type KnowledgeCategory, type KnowledgeDocument } from "../api/client";
import { authContextKey } from "../auth/session";
import { useI18n } from "vue-i18n";

const api = inject(platformApiKey)!;
const auth = inject(authContextKey)!;
const { t } = useI18n();
const bases = ref<KnowledgeBase[]>([]);
const categories = ref<KnowledgeCategory[]>([]);
const documents = ref<KnowledgeDocument[]>([]);
const selected = ref<KnowledgeBase>();
const loading = ref(true);
const busy = ref(false);
const error = ref("");
const showCreate = ref(false);
const newCategory = ref("");
const selectedCategory = ref("");
const sourceURL = ref("");
const form = ref({ name: "", description: "", visibility: "private" as "private" | "public", platform: false });
const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const canCreatePlatform = computed(() => Boolean(currentUser.value?.administrator));

async function refresh() {
  loading.value = true; error.value = "";
  try { bases.value = await api.listKnowledgeBases(); if (selected.value) selected.value = bases.value.find((item) => item.id === selected.value?.id); if (!selected.value && bases.value[0]) await selectBase(bases.value[0]); }
  catch { error.value = t("knowledgeBases.loadFailed"); }
  finally { loading.value = false; }
}
async function selectBase(item: KnowledgeBase) {
  selected.value = item;
  selectedCategory.value = "";
  try { [categories.value, documents.value] = await Promise.all([api.listKnowledgeCategories(item.id), api.listKnowledgeDocuments(item.id)]); }
  catch { error.value = t("knowledgeBases.loadFailed"); }
}
async function createBase() {
  if (!form.value.name.trim() || busy.value) return;
  busy.value = true;
  try { const created = await api.createKnowledgeBase({ ...form.value, platform: canCreatePlatform.value }); bases.value = [created, ...bases.value]; showCreate.value = false; form.value = { name: "", description: "", visibility: "private", platform: false }; await selectBase(created); }
  catch { error.value = t("knowledgeBases.saveFailed"); }
  finally { busy.value = false; }
}
async function createCategory() {
  if (!selected.value || !newCategory.value.trim()) return;
  try { const category = await api.createKnowledgeCategory(selected.value.id, newCategory.value.trim()); categories.value = [...categories.value, category]; newCategory.value = ""; }
  catch { error.value = t("knowledgeBases.saveFailed"); }
}
async function upload(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!selected.value || !file) return;
  try { const document = await api.uploadKnowledgeDocument(selected.value.id, file, selectedCategory.value || undefined); documents.value = [document, ...documents.value]; ElMessage.success(t("knowledgeBases.accepted")); }
  catch { error.value = t("knowledgeBases.uploadFailed"); }
  finally { (event.target as HTMLInputElement).value = ""; }
}
async function importURL() {
  if (!selected.value || !sourceURL.value.trim() || busy.value) return;
  busy.value = true;
  try { const document = await api.importKnowledgeDocument(selected.value.id, sourceURL.value.trim(), selectedCategory.value || undefined); documents.value = [document, ...documents.value]; sourceURL.value = ""; ElMessage.success(t("knowledgeBases.accepted")); }
  catch { error.value = t("knowledgeBases.importFailed"); }
  finally { busy.value = false; }
}
async function downloadDocument(document: KnowledgeDocument) {
  if (!selected.value) return;
  try { const blob = await api.downloadKnowledgeDocument(selected.value.id, document.id); const href = URL.createObjectURL(blob); const anchor = window.document.createElement("a"); anchor.href = href; anchor.download = document.name; anchor.click(); URL.revokeObjectURL(href); }
  catch { error.value = t("knowledgeBases.downloadFailed"); }
}
async function retryDocument(document: KnowledgeDocument) {
  if (!selected.value) return;
  try { await api.retryKnowledgeDocument(selected.value.id, document.id); await selectBase(selected.value); ElMessage.success(t("knowledgeBases.retryAccepted")); }
  catch { error.value = t("knowledgeBases.retryFailed"); }
}
async function deleteDocument(document: KnowledgeDocument) {
  if (!selected.value || !window.confirm(`${t("knowledgeBases.deleteDocument")}?`)) return;
  try { await api.deleteKnowledgeDocument(selected.value.id, document.id); await selectBase(selected.value); ElMessage.success(t("knowledgeBases.deleteAccepted")); }
  catch { error.value = t("knowledgeBases.deleteFailed"); }
}
async function deleteBase() {
  if (!selected.value || !window.confirm(`${t("knowledgeBases.deleteBase")}?`)) return;
  const deletedID = selected.value.id;
  try { await api.deleteKnowledgeBase(deletedID); selected.value = undefined; categories.value = []; documents.value = []; await refresh(); ElMessage.success(t("knowledgeBases.deleteAccepted")); }
  catch { error.value = t("knowledgeBases.deleteFailed"); }
}
onMounted(() => void refresh());
</script>

<template>
  <section class="knowledge-page">
    <header class="page-heading">
      <div><h1>{{ t("knowledgeBases.title") }}</h1><p>{{ t("knowledgeBases.subtitle") }}</p></div>
      <el-button type="primary" :icon="Plus" @click="showCreate = true">{{ t("knowledgeBases.new") }}</el-button>
    </header>
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" class="feedback" />
    <div v-if="loading" class="empty-state">{{ t("common.loading") }}</div>
    <div v-else class="knowledge-layout">
      <el-card class="base-list" shadow="never">
        <div class="section-title"><strong>{{ t("knowledgeBases.catalog") }}</strong><span>{{ bases.length }}</span></div>
        <button v-for="item in bases" :key="item.id" class="base-row" :class="{ selected: selected?.id === item.id }" @click="selectBase(item)">
          <el-icon><Folder /></el-icon><span><strong>{{ item.name }}</strong><small>{{ item.visibility === "public" ? t("knowledgeBases.public") : t("knowledgeBases.private") }}</small></span>
        </button>
        <div v-if="!bases.length" class="empty-state compact">{{ t("knowledgeBases.empty") }}</div>
      </el-card>
      <el-card v-if="selected" class="base-detail" shadow="never">
        <div class="detail-heading"><div><h2>{{ selected.name }} <el-tag size="small" effect="plain">{{ selected.visibility === "public" ? t("knowledgeBases.public") : t("knowledgeBases.private") }}</el-tag></h2><p>{{ selected.description || t("knowledgeBases.noDescription") }}</p></div><div class="detail-actions"><el-button type="danger" plain @click="deleteBase">{{ t("common.delete") }}</el-button><label class="upload-button"><el-icon><Upload /></el-icon>{{ t("knowledgeBases.upload") }}<input type="file" hidden @change="upload" /></label></div></div>
        <div class="category-bar"><el-input v-model="newCategory" :placeholder="t('knowledgeBases.categoryPlaceholder')" @keyup.enter="createCategory" /><el-button @click="createCategory">{{ t("knowledgeBases.addCategory") }}</el-button></div>
        <div class="category-chips"><el-tag v-for="category in categories" :key="category.id" effect="plain">{{ category.name }}</el-tag><el-tag v-if="documents.some((item) => !item.category_id)" type="info" effect="plain">{{ t("knowledgeBases.unclassified") }}</el-tag></div>
        <div class="source-controls"><el-select v-model="selectedCategory" :placeholder="t('knowledgeBases.unclassified')" clearable><el-option v-for="category in categories" :key="category.id" :value="category.id" :label="category.name" /></el-select><el-input v-model="sourceURL" :placeholder="t('knowledgeBases.urlPlaceholder')" @keyup.enter="importURL" /><el-button :loading="busy" @click="importURL">{{ t("knowledgeBases.importURL") }}</el-button></div>
        <el-table :data="documents" class="document-table"><el-table-column prop="name" :label="t('knowledgeBases.document')" /><el-table-column prop="state" :label="t('knowledgeBases.state')" width="140" /><el-table-column prop="source_type" :label="t('knowledgeBases.source')" width="120" /><el-table-column prop="updated_at" :label="t('knowledgeBases.updated')" width="190" /><el-table-column width="230"><template #default="scope"><el-button v-if="(scope.row as KnowledgeDocument).state === 'failed'" text @click="retryDocument(scope.row as KnowledgeDocument)">{{ t("knowledgeBases.retry") }}</el-button><el-button text @click="downloadDocument(scope.row as KnowledgeDocument)">{{ t("common.download") }}</el-button><el-button type="danger" text @click="deleteDocument(scope.row as KnowledgeDocument)">{{ t("common.delete") }}</el-button></template></el-table-column></el-table>
        <div v-if="!documents.length" class="empty-state compact">{{ t("knowledgeBases.noDocuments") }}</div>
      </el-card>
      <el-card v-else class="base-detail" shadow="never"><div class="empty-state">{{ t("knowledgeBases.select") }}</div></el-card>
    </div>
    <el-dialog v-model="showCreate" :title="t('knowledgeBases.new')" width="min(520px, 92vw)">
      <el-form label-position="top" @submit.prevent="createBase"><el-form-item :label="t('common.name')"><el-input v-model="form.name" autofocus /></el-form-item><el-form-item :label="t('knowledgeBases.description')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item><el-form-item v-if="canCreatePlatform" :label="t('knowledgeBases.visibility')"><el-select v-model="form.visibility"><el-option value="private" :label="t('knowledgeBases.private')" /><el-option value="public" :label="t('knowledgeBases.public')" /></el-select></el-form-item></el-form>
      <template #footer><el-button @click="showCreate = false">{{ t("common.cancel") }}</el-button><el-button type="primary" :loading="busy" @click="createBase">{{ t("common.save") }}</el-button></template>
    </el-dialog>
  </section>
</template>

<style scoped>
.knowledge-page { max-width: 1180px; margin: 0 auto; padding: 28px clamp(18px, 4vw, 52px) 56px; }
.page-heading, .detail-heading, .section-title, .category-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.detail-actions { display: flex; align-items: center; gap: 10px; }
.page-heading { margin-bottom: 24px; } h1, h2 { margin: 4px 0; } h1 { font-size: clamp(28px, 4vw, 42px); } h2 { font-size: 24px; } p { color: var(--text-muted); margin: 8px 0 0; }
.eyebrow { color: var(--accent); font-size: 11px; font-weight: 800; letter-spacing: .14em; text-transform: uppercase; }
.feedback { margin-bottom: 18px; }
.knowledge-layout { display: grid; grid-template-columns: minmax(220px, 300px) 1fr; gap: 18px; align-items: start; }
.base-list, .base-detail { border: 1px solid var(--line); border-radius: 18px; }
.section-title { margin-bottom: 12px; } .section-title span { color: var(--text-muted); font-size: 13px; }
.base-row { width: 100%; display: flex; align-items: center; gap: 10px; border: 0; border-radius: 12px; background: transparent; color: inherit; padding: 12px; text-align: left; cursor: pointer; } .base-row:hover, .base-row.selected { background: color-mix(in srgb, var(--accent) 11%, transparent); }
.base-row span { display: grid; gap: 3px; } .base-row small { color: var(--text-muted); }
.upload-button { display: inline-flex; align-items: center; gap: 7px; border-radius: 10px; background: var(--accent); color: white; padding: 10px 13px; cursor: pointer; font-weight: 700; white-space: nowrap; }
.category-bar { margin: 24px 0 12px; } .category-bar .el-input { max-width: 320px; } .category-chips { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 18px; }
.source-controls { display: grid; grid-template-columns: minmax(160px, 220px) 1fr auto; gap: 10px; margin: 12px 0 18px; }
.empty-state { display: grid; place-items: center; min-height: 220px; color: var(--text-muted); } .empty-state.compact { min-height: 100px; }
@media (max-width: 760px) { .knowledge-layout { grid-template-columns: 1fr; } .page-heading, .detail-heading { align-items: flex-start; flex-direction: column; } .detail-actions { width: 100%; } .upload-button { width: 100%; justify-content: center; } .source-controls { grid-template-columns: 1fr; } }
</style>
