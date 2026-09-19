<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { ArrowLeft, Download, Eye, FileText, FolderOpen, Globe2, LayoutGrid, List as ListIcon, LockKeyhole, MoreHorizontal, Pencil, Plus, Trash2, Upload } from "@lucide/vue";
import { platformApiKey, type KnowledgeBase, type KnowledgeCategory, type KnowledgeDocument } from "../api/client";
import { authContextKey } from "../auth/session";
import { useI18n } from "vue-i18n";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";

const api = inject(platformApiKey)!;
const auth = inject(authContextKey)!;
const { t } = useI18n();

type DisplayMode = "card" | "list";
type PreviewKind = "image" | "pdf" | "text" | "unsupported";

const bases = ref<KnowledgeBase[]>([]);
const categories = ref<KnowledgeCategory[]>([]);
const documents = ref<KnowledgeDocument[]>([]);
const selected = ref<KnowledgeBase>();
const activeCategory = ref<string | null>(null);
const loading = ref(true);
const busy = ref(false);
const previewBusy = ref(false);
const error = ref("");
const displayMode = ref<DisplayMode>((localStorage.getItem("knowledge-base-display") as DisplayMode) || "card");
const showBaseDialog = ref(false);
const editingBase = ref<KnowledgeBase>();
const deleteTarget = ref<KnowledgeBase>();
const newCategory = ref("");
const uploadCategory = ref("");
const sourceURL = ref("");
const preview = ref<{ document: KnowledgeDocument; kind: PreviewKind; url?: string; text?: string }>();
const form = ref({ name: "", description: "", visibility: "private" as "private" | "public" });

const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const canCreatePlatform = computed(() => Boolean(currentUser.value?.administrator));
const canManageSelected = computed(() => selected.value ? canManageBase(selected.value) : false);
const activeCategoryName = computed(() => {
  if (activeCategory.value === null) return t("knowledgeBases.allDocuments");
  if (activeCategory.value === "unclassified") return t("knowledgeBases.unclassified");
  return categories.value.find((category) => category.id === activeCategory.value)?.name || t("knowledgeBases.allDocuments");
});
const visibleDocuments = computed(() => {
  if (activeCategory.value === null) return documents.value;
  return documents.value.filter((document) => activeCategory.value === "unclassified" ? !document.category_id : document.category_id === activeCategory.value);
});
const categoryCounts = computed(() => new Map(categories.value.map((category) => [category.id, documents.value.filter((document) => document.category_id === category.id).length])));
const unclassifiedCount = computed(() => documents.value.filter((document) => !document.category_id).length);
const previewOpen = computed({ get: () => Boolean(preview.value), set: (open: boolean) => { if (!open) closePreview(); } });

async function refresh() {
  loading.value = true;
  error.value = "";
  try {
    const previousID = selected.value?.id;
    bases.value = await api.listKnowledgeBases();
    if (previousID) {
      const current = bases.value.find((item) => item.id === previousID);
      if (current) selected.value = current;
      else closeBase();
    }
  } catch {
    error.value = t("knowledgeBases.loadFailed");
  } finally {
    loading.value = false;
  }
}

async function openBase(item: KnowledgeBase) {
  selected.value = item;
  activeCategory.value = null;
  uploadCategory.value = "";
  error.value = "";
  try {
    [categories.value, documents.value] = await Promise.all([api.listKnowledgeCategories(item.id), api.listKnowledgeDocuments(item.id)]);
  } catch {
    error.value = t("knowledgeBases.loadFailed");
  }
}

function closeBase() {
  selected.value = undefined;
  categories.value = [];
  documents.value = [];
  activeCategory.value = null;
  uploadCategory.value = "";
}

function setDisplayMode(mode: DisplayMode) {
  displayMode.value = mode;
  localStorage.setItem("knowledge-base-display", mode);
}

function canManageBase(item: KnowledgeBase) {
  return !item.platform || item.owner_id === currentUser.value?.id;
}

function openCreate() {
  editingBase.value = undefined;
  form.value = { name: "", description: "", visibility: "private" };
  showBaseDialog.value = true;
}

function openEdit(item: KnowledgeBase) {
  editingBase.value = item;
  form.value = { name: item.name, description: item.description, visibility: item.visibility };
  showBaseDialog.value = true;
}

function handleBaseAction(command: string | number | object, item: KnowledgeBase) {
  if (command === "edit") openEdit(item);
  if (command === "delete") deleteTarget.value = item;
}

async function saveBase() {
  const name = form.value.name.trim();
  if (!name || busy.value) return;
  busy.value = true;
  try {
    const input = { name, description: form.value.description.trim(), visibility: form.value.visibility, platform: editingBase.value?.platform ?? canCreatePlatform.value };
    if (editingBase.value) {
      const updated = await api.updateKnowledgeBase(editingBase.value.id, input, editingBase.value.version);
      bases.value = bases.value.map((item) => item.id === updated.id ? updated : item);
      if (selected.value?.id === updated.id) selected.value = updated;
    } else {
      const created = await api.createKnowledgeBase(input);
      bases.value = [created, ...bases.value];
    }
    showBaseDialog.value = false;
  } catch {
    error.value = t("knowledgeBases.saveFailed");
  } finally {
    busy.value = false;
  }
}

async function removeBase() {
  const item = deleteTarget.value;
  if (!item || busy.value) return;
  busy.value = true;
  try {
    await api.deleteKnowledgeBase(item.id);
    bases.value = bases.value.filter((base) => base.id !== item.id);
    if (selected.value?.id === item.id) closeBase();
    deleteTarget.value = undefined;
  } catch {
    error.value = t("knowledgeBases.deleteFailed");
  } finally {
    busy.value = false;
  }
}

function openCategory(categoryID: string | null) {
  activeCategory.value = categoryID;
}

async function createCategory() {
  if (!selected.value || !newCategory.value.trim()) return;
  try {
    const category = await api.createKnowledgeCategory(selected.value.id, newCategory.value.trim());
    categories.value = [...categories.value, category];
    newCategory.value = "";
  } catch {
    error.value = t("knowledgeBases.saveFailed");
  }
}

async function upload(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!selected.value || !file) return;
  try {
    const document = await api.uploadKnowledgeDocument(selected.value.id, file, uploadCategory.value || undefined);
    documents.value = [document, ...documents.value];
    ElMessage.success(t("knowledgeBases.accepted"));
  } catch {
    error.value = t("knowledgeBases.uploadFailed");
  } finally {
    (event.target as HTMLInputElement).value = "";
  }
}

async function importURL() {
  if (!selected.value || !sourceURL.value.trim() || busy.value) return;
  busy.value = true;
  try {
    const document = await api.importKnowledgeDocument(selected.value.id, sourceURL.value.trim(), uploadCategory.value || undefined);
    documents.value = [document, ...documents.value];
    sourceURL.value = "";
    ElMessage.success(t("knowledgeBases.accepted"));
  } catch {
    error.value = t("knowledgeBases.importFailed");
  } finally {
    busy.value = false;
  }
}

async function downloadDocument(document: KnowledgeDocument) {
  if (!selected.value) return;
  try {
    const blob = await api.downloadKnowledgeDocument(selected.value.id, document.id);
    const href = URL.createObjectURL(blob);
    const anchor = window.document.createElement("a");
    anchor.href = href;
    anchor.download = document.name;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(href), 1000);
  } catch {
    error.value = t("knowledgeBases.downloadFailed");
  }
}

async function previewDocument(document: KnowledgeDocument) {
  if (!selected.value || previewBusy.value) return;
  closePreview();
  previewBusy.value = true;
  try {
    const blob = await api.downloadKnowledgeDocument(selected.value.id, document.id);
    const contentType = (document.latest_revision?.content_type || blob.type || "").toLowerCase();
    const isText = contentType.startsWith("text/") || /\.(md|markdown|txt|csv|json|xml|html?)$/i.test(document.name);
    const kind: PreviewKind = contentType.startsWith("image/") ? "image" : contentType.includes("pdf") ? "pdf" : isText ? "text" : "unsupported";
    if (kind === "text") preview.value = { document, kind, text: await blob.text() };
    else if (kind === "image" || kind === "pdf") preview.value = { document, kind, url: URL.createObjectURL(blob) };
    else preview.value = { document, kind };
  } catch {
    error.value = t("knowledgeBases.previewFailed");
  } finally {
    previewBusy.value = false;
  }
}

function closePreview() {
  if (preview.value?.url) URL.revokeObjectURL(preview.value.url);
  preview.value = undefined;
}

async function retryDocument(document: KnowledgeDocument) {
  if (!selected.value) return;
  try {
    await api.retryKnowledgeDocument(selected.value.id, document.id);
    await openBase(selected.value);
    ElMessage.success(t("knowledgeBases.retryAccepted"));
  } catch {
    error.value = t("knowledgeBases.retryFailed");
  }
}

async function deleteDocument(document: KnowledgeDocument) {
  if (!selected.value || !window.confirm(`${t("knowledgeBases.deleteDocument")}?`)) return;
  try {
    await api.deleteKnowledgeDocument(selected.value.id, document.id);
    await openBase(selected.value);
    ElMessage.success(t("knowledgeBases.deleteAccepted"));
  } catch {
    error.value = t("knowledgeBases.deleteFailed");
  }
}

function formatDate(value: string) { return new Date(value).toLocaleDateString(); }
function formatSize(size?: number) { if (!size) return "—"; if (size < 1024) return `${size} B`; if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`; return `${(size / 1024 / 1024).toFixed(1)} MB`; }

onMounted(() => void refresh());
onUnmounted(closePreview);
</script>

<template>
  <section class="page-surface knowledge-page">
    <header class="page-header">
      <div><p class="eyebrow">{{ t("knowledgeBases.eyebrow") }}</p><h1>{{ t("knowledgeBases.title") }}</h1><p>{{ t("knowledgeBases.subtitle") }}</p></div>
      <div v-if="!selected" class="knowledge-header-actions"><div class="view-toggle" role="group" :aria-label="t('knowledgeBases.displayMode')"><button :class="{ active: displayMode === 'card' }" :aria-label="t('knowledgeBases.cardView')" :title="t('knowledgeBases.cardView')" @click="setDisplayMode('card')"><LayoutGrid :size="16" /></button><button :class="{ active: displayMode === 'list' }" :aria-label="t('knowledgeBases.listView')" :title="t('knowledgeBases.listView')" @click="setDisplayMode('list')"><ListIcon :size="16" /></button></div><el-button type="primary" @click="openCreate"><Plus :size="16" />{{ t("knowledgeBases.new") }}</el-button></div>
    </header>

    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <el-skeleton v-if="loading" :rows="8" animated class="page-loading" />

    <template v-else-if="!selected">
      <div class="catalog-heading"><div><strong>{{ t("knowledgeBases.catalog") }}</strong><span>{{ bases.length }}</span></div><small>{{ t("knowledgeBases.catalogHint") }}</small></div>
      <el-empty v-if="!bases.length" :description="t('knowledgeBases.empty')"><el-button type="primary" @click="openCreate">{{ t("knowledgeBases.new") }}</el-button></el-empty>
      <div v-else class="knowledge-grid" :class="`is-${displayMode}`">
        <el-card v-for="item in bases" :key="item.id" class="knowledge-card" shadow="never" role="button" tabindex="0" @click="openBase(item)" @keydown.enter="openBase(item)" @keydown.space.prevent="openBase(item)">
          <div class="knowledge-card-head"><div class="knowledge-icon"><FolderOpen :size="20" /></div><el-dropdown v-if="canManageBase(item)" trigger="click" @command="handleBaseAction($event, item)"><el-button class="card-more" text circle :aria-label="t('common.more')" :title="t('common.more')" @click.stop><MoreHorizontal :size="18" /></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="edit"><Pencil :size="14" />{{ t("common.edit") }}</el-dropdown-item><el-dropdown-item command="delete" divided><Trash2 :size="14" />{{ t("common.delete") }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown></div>
          <div class="knowledge-card-copy"><div class="knowledge-card-title"><h2>{{ item.name }}</h2><el-tag size="small" effect="plain"><Globe2 v-if="item.visibility === 'public'" :size="12" /><LockKeyhole v-else :size="12" />{{ item.visibility === "public" ? t("knowledgeBases.public") : t("knowledgeBases.private") }}</el-tag></div><p>{{ item.description || t("knowledgeBases.noDescription") }}</p></div>
          <footer><span>{{ formatDate(item.updated_at) }}</span><strong>{{ t("knowledgeBases.open") }} <ArrowLeft :size="14" /></strong></footer>
        </el-card>
      </div>
    </template>

    <template v-else>
      <button class="back-link knowledge-back" @click="closeBase"><ArrowLeft :size="16" />{{ t("knowledgeBases.backToCatalog") }}</button>
      <header class="knowledge-detail-header"><div><div class="detail-title-line"><h2>{{ selected.name }}</h2><el-tag size="small" effect="plain">{{ selected.visibility === "public" ? t("knowledgeBases.public") : t("knowledgeBases.private") }}</el-tag></div><p>{{ selected.description || t("knowledgeBases.noDescription") }}</p></div><div v-if="canManageSelected" class="detail-actions"><el-button plain @click="openEdit(selected)"><Pencil :size="15" />{{ t("common.edit") }}</el-button><el-button type="danger" plain @click="deleteTarget = selected"><Trash2 :size="15" />{{ t("common.delete") }}</el-button><label class="upload-button"><Upload :size="15" />{{ t("knowledgeBases.upload") }}<input type="file" hidden @change="upload" /></label></div></header>

      <section v-if="categories.length" class="category-section"><div class="section-heading"><div><h3>{{ t("knowledgeBases.categories") }}</h3><p>{{ t("knowledgeBases.categoriesHint") }}</p></div><el-button text @click="openCategory(null)">{{ t("knowledgeBases.viewAll") }}</el-button></div><div class="category-grid"><button class="category-card" :class="{ active: activeCategory === null }" @click="openCategory(null)"><span class="category-card-icon"><FolderOpen :size="18" /></span><span><strong>{{ t("knowledgeBases.allDocuments") }}</strong><small>{{ documents.length }} {{ t("knowledgeBases.documentCount") }}</small></span></button><button v-for="category in categories" :key="category.id" class="category-card" :class="{ active: activeCategory === category.id }" @click="openCategory(category.id)"><span class="category-card-icon"><FolderOpen :size="18" /></span><span><strong>{{ category.name }}</strong><small>{{ categoryCounts.get(category.id) || 0 }} {{ t("knowledgeBases.documentCount") }}</small></span></button><button v-if="unclassifiedCount" class="category-card" :class="{ active: activeCategory === 'unclassified' }" @click="openCategory('unclassified')"><span class="category-card-icon muted"><FileText :size="18" /></span><span><strong>{{ t("knowledgeBases.unclassified") }}</strong><small>{{ unclassifiedCount }} {{ t("knowledgeBases.documentCount") }}</small></span></button></div></section>

      <section class="documents-panel"><div class="section-heading"><div><p class="eyebrow">{{ t("knowledgeBases.document") }}</p><h3>{{ activeCategoryName }}</h3><p>{{ visibleDocuments.length }} {{ t("knowledgeBases.documentCount") }}</p></div><div v-if="canManageSelected" class="category-create"><el-input v-model="newCategory" :placeholder="t('knowledgeBases.categoryPlaceholder')" @keyup.enter="createCategory" /><el-button @click="createCategory">{{ t("knowledgeBases.addCategory") }}</el-button></div></div><div v-if="canManageSelected" class="source-controls"><el-select v-model="uploadCategory" :placeholder="t('knowledgeBases.unclassified')" clearable><el-option v-for="category in categories" :key="category.id" :value="category.id" :label="category.name" /></el-select><el-input v-model="sourceURL" :placeholder="t('knowledgeBases.urlPlaceholder')" @keyup.enter="importURL" /><el-button :loading="busy" @click="importURL"><Globe2 :size="15" />{{ t("knowledgeBases.importURL") }}</el-button></div>
        <el-table v-if="visibleDocuments.length" :data="visibleDocuments" class="document-table"><el-table-column min-width="280" :label="t('knowledgeBases.document')"><template #default="scope"><div class="document-name"><span class="document-icon"><FileText :size="16" /></span><span><strong>{{ (scope.row as KnowledgeDocument).name }}</strong><small>{{ formatSize((scope.row as KnowledgeDocument).latest_revision?.size) }}</small></span></div></template></el-table-column><el-table-column prop="state" :label="t('knowledgeBases.state')" width="130" /><el-table-column prop="source_type" :label="t('knowledgeBases.source')" width="100" /><el-table-column :label="t('knowledgeBases.updated')" width="130"><template #default="scope">{{ formatDate((scope.row as KnowledgeDocument).updated_at) }}</template></el-table-column><el-table-column width="280"><template #default="scope"><div class="document-actions"><el-button text @click="previewDocument(scope.row as KnowledgeDocument)"><Eye :size="15" />{{ t("knowledgeBases.preview") }}</el-button><el-button text @click="downloadDocument(scope.row as KnowledgeDocument)"><Download :size="15" />{{ t("common.download") }}</el-button><el-button v-if="canManageSelected && (scope.row as KnowledgeDocument).state === 'failed'" text @click="retryDocument(scope.row as KnowledgeDocument)">{{ t("knowledgeBases.retry") }}</el-button><el-button v-if="canManageSelected" type="danger" text @click="deleteDocument(scope.row as KnowledgeDocument)"><Trash2 :size="15" />{{ t("common.delete") }}</el-button></div></template></el-table-column></el-table><el-empty v-else :description="t('knowledgeBases.noDocuments')"><label v-if="canManageSelected" class="upload-button"><Upload :size="15" />{{ t("knowledgeBases.upload") }}<input type="file" hidden @change="upload" /></label></el-empty>
      </section>
    </template>
  </section>

  <el-dialog v-model="showBaseDialog" class="resource-dialog" width="min(560px, calc(100vw - 32px))" align-center :title="editingBase ? t('knowledgeBases.edit') : t('knowledgeBases.new')"><el-form label-position="top" @submit.prevent="saveBase"><el-form-item :label="t('common.name')" required><el-input v-model="form.name" maxlength="100" autofocus /></el-form-item><el-form-item :label="t('knowledgeBases.description')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item><el-form-item v-if="canCreatePlatform" :label="t('knowledgeBases.visibility')"><el-select v-model="form.visibility"><el-option value="private" :label="t('knowledgeBases.private')" /><el-option value="public" :label="t('knowledgeBases.public')" /></el-select></el-form-item></el-form><template #footer><el-button @click="showBaseDialog = false">{{ t("common.cancel") }}</el-button><el-button type="primary" :loading="busy" :disabled="!form.name.trim()" @click="saveBase">{{ t("common.save") }}</el-button></template></el-dialog>
  <ConfirmDialog :open="Boolean(deleteTarget)" :title="t('knowledgeBases.deleteBase')" :message="deleteTarget ? `${t('common.delete')} “${deleteTarget.name}”?` : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger :busy="busy" @cancel="deleteTarget = undefined" @confirm="removeBase" />
  <el-dialog v-model="previewOpen" class="document-preview-dialog" width="min(980px, calc(100vw - 32px))" align-center :title="preview?.document.name" @close="closePreview"><div v-if="previewBusy" class="preview-loading">{{ t("common.loading") }}</div><template v-else-if="preview"><img v-if="preview.kind === 'image'" :src="preview.url" :alt="preview.document.name" class="preview-image" /><iframe v-else-if="preview.kind === 'pdf'" :src="preview.url" :title="preview.document.name" class="preview-frame" /><pre v-else-if="preview.kind === 'text'" class="preview-text">{{ preview.text }}</pre><div v-else class="preview-unsupported"><FileText :size="32" /><p>{{ t("knowledgeBases.previewUnsupported") }}</p><el-button type="primary" @click="downloadDocument(preview.document)"><Download :size="15" />{{ t("common.download") }}</el-button></div></template></el-dialog>
</template>

<style scoped>
.knowledge-page { max-width: 1480px; }
.knowledge-header-actions, .detail-actions, .view-toggle, .document-actions, .category-create { display: flex; align-items: center; gap: 8px; }
.view-toggle { padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: rgba(255,255,255,.72); }
.view-toggle button { width: 32px; height: 30px; display: grid; place-items: center; border: 0; border-radius: 7px; color: var(--muted); background: transparent; }
.view-toggle button.active { color: var(--green); background: #e5efe8; }
.catalog-heading { display: flex; align-items: end; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.catalog-heading > div { display: flex; align-items: center; gap: 9px; }
.catalog-heading > div span { color: var(--muted); font-size: .75rem; }
.catalog-heading small { color: var(--muted); font-size: .73rem; }
.knowledge-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.knowledge-grid.is-list { grid-template-columns: 1fr; }
.knowledge-card { min-width: 0; border-color: #d9e0da; border-radius: 15px; background: rgba(255,255,255,.86); cursor: pointer; transition: transform .2s ease, border-color .2s ease, box-shadow .2s ease; }
.knowledge-card:hover, .knowledge-card:focus-visible { border-color: #91ad9b; box-shadow: 0 12px 30px rgba(25,58,43,.08); transform: translateY(-2px); outline: none; }
.knowledge-card :deep(.el-card__body) { display: flex; flex-direction: column; min-height: 186px; padding: 20px; }
.knowledge-card-head, .knowledge-card-title, .knowledge-card footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.knowledge-icon, .category-card-icon { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; color: var(--green); background: #e5efe8; }
.card-more { color: var(--muted); }
.knowledge-card-copy { min-width: 0; margin-top: 18px; }
.knowledge-card-title h2 { min-width: 0; margin: 0; overflow: hidden; font-size: 1.05rem; text-overflow: ellipsis; white-space: nowrap; }
.knowledge-card-title .el-tag { display: inline-flex; align-items: center; gap: 4px; flex: 0 0 auto; }
.knowledge-card-copy p { min-height: 2.6em; margin: 8px 0 14px; color: var(--muted); font-size: .78rem; line-height: 1.5; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; }
.knowledge-card footer { margin-top: auto; color: var(--muted); font-size: .7rem; }
.knowledge-card footer strong { display: inline-flex; align-items: center; gap: 3px; color: var(--green); }
.knowledge-grid.is-list .knowledge-card :deep(.el-card__body) { min-height: 0; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 16px; align-items: center; }
.knowledge-grid.is-list .knowledge-card-copy { margin: 0; }
.knowledge-grid.is-list .knowledge-card-copy p { min-height: 0; margin: 5px 0 0; }
.knowledge-grid.is-list .knowledge-card footer { display: flex; flex-direction: column; align-items: flex-end; gap: 7px; }
.knowledge-back { margin: 0 0 20px; }
.knowledge-detail-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 30px; }
.detail-title-line { display: flex; align-items: center; gap: 10px; }
.detail-title-line h2 { margin: 0; font-family: "Iowan Old Style", "Palatino Linotype", serif; font-size: clamp(1.7rem, 2.5vw, 2.35rem); font-weight: 600; letter-spacing: -.035em; }
.knowledge-detail-header p { margin: 7px 0 0; color: var(--muted); font-size: .85rem; }
.upload-button { display: inline-flex; align-items: center; gap: 7px; border-radius: 9px; background: var(--green); color: white; padding: 9px 13px; cursor: pointer; font-size: .8rem; font-weight: 700; white-space: nowrap; }
.section-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 13px; }
.section-heading h3 { margin: 2px 0 4px; font-size: 1.05rem; }
.section-heading p { margin: 0; color: var(--muted); font-size: .75rem; }
.category-section { margin-bottom: 30px; }
.category-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 10px; }
.category-card { display: flex; align-items: center; gap: 11px; min-width: 0; padding: 13px; border: 1px solid var(--line); border-radius: 12px; color: var(--ink); background: rgba(255,255,255,.7); text-align: left; transition: border-color .18s ease, background .18s ease; }
.category-card:hover, .category-card.active { border-color: #8daf9b; background: #eef6ef; }
.category-card-icon { width: 36px; height: 36px; flex: 0 0 auto; }
.category-card-icon.muted { color: var(--muted); background: #eef0ed; }
.category-card > span:last-child { min-width: 0; display: grid; gap: 3px; }
.category-card strong, .category-card small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.category-card strong { font-size: .78rem; }
.category-card small { color: var(--muted); font-size: .68rem; }
.documents-panel { padding: 22px; border: 1px solid var(--line); border-radius: 16px; background: rgba(255,255,255,.58); }
.category-create .el-input { width: 230px; }
.source-controls { display: grid; grid-template-columns: 200px minmax(0, 1fr) auto; gap: 9px; margin-bottom: 15px; }
.document-table { background: transparent; }
.document-name { display: flex; align-items: center; gap: 10px; min-width: 0; }
.document-name > span:last-child { min-width: 0; display: grid; gap: 3px; }
.document-name strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.document-name small { color: var(--muted); font-size: .68rem; }
.document-icon { display: grid; place-items: center; width: 31px; height: 31px; flex: 0 0 auto; border-radius: 8px; color: var(--green); background: #e9f0ea; }
.document-actions { justify-content: flex-end; flex-wrap: wrap; }
.document-actions .el-button { margin-left: 0; }
.preview-loading, .preview-unsupported { display: grid; place-items: center; min-height: 300px; gap: 12px; color: var(--muted); text-align: center; }
.preview-image { display: block; max-width: 100%; max-height: 68vh; margin: 0 auto; object-fit: contain; }
.preview-frame { width: 100%; height: 68vh; border: 0; }
.preview-text { max-height: 68vh; margin: 0; padding: 16px; overflow: auto; border-radius: 9px; background: #f3f5f1; color: var(--ink); font: .78rem/1.65 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; }
@media (max-width: 1050px) { .knowledge-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 760px) { .knowledge-header-actions, .knowledge-detail-header, .section-heading { align-items: flex-start; flex-direction: column; } .knowledge-header-actions, .detail-actions { width: 100%; flex-wrap: wrap; } .knowledge-header-actions .el-button, .detail-actions .upload-button { flex: 1; justify-content: center; } .knowledge-grid { grid-template-columns: 1fr; } .knowledge-grid.is-list .knowledge-card :deep(.el-card__body) { grid-template-columns: auto minmax(0, 1fr); } .knowledge-grid.is-list .knowledge-card footer { grid-column: 2; flex-direction: row; align-items: center; justify-content: space-between; } .category-create { width: 100%; } .category-create .el-input { width: auto; flex: 1; } .source-controls { grid-template-columns: 1fr; } .documents-panel { padding: 16px; } .document-table { overflow-x: auto; } }
</style>
