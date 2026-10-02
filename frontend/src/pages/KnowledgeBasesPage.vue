<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { ArrowLeft, Download, Eye, FileText, FolderOpen, Globe2, LayoutGrid, List as ListIcon, LockKeyhole, MoreHorizontal, Pencil, Plus, Search, Trash2, Upload } from "@lucide/vue";
import { platformApiKey, type KnowledgeBase, type KnowledgeCategory, type KnowledgeDocument, type KnowledgeSearchResult } from "../api/client";
import { authContextKey } from "../auth/session";
import { useI18n } from "vue-i18n";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import ResourceTrustMeta from "../components/ResourceTrustMeta.vue";

const props = withDefaults(defineProps<{ embedded?: boolean; catalogQuery?: string; availableOnly?: boolean }>(), { embedded: false, catalogQuery: "", availableOnly: false });
const emit = defineEmits<{ "detail-open": [open: boolean] }>();

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
const searchBusy = ref(false);
const searchQuery = ref("");
const searchResults = ref<KnowledgeSearchResult[]>([]);
const searchRan = ref(false);
const searchIndexReady = ref(true);
const searchError = ref("");
const error = ref("");
const displayMode = ref<DisplayMode>((localStorage.getItem("knowledge-base-display") as DisplayMode) || "card");
const showBaseDialog = ref(false);
const editingBase = ref<KnowledgeBase>();
const deleteTarget = ref<KnowledgeBase>();
const newCategory = ref("");
const uploadCategory = ref("");
const sourceURL = ref("");
const fileInput = ref<HTMLInputElement>();
const preview = ref<{ document: KnowledgeDocument; kind: PreviewKind; url?: string; text?: string }>();
const form = ref({ name: "", description: "", scope: "private" as "private" | "group" | "platform", group_id: "" });

const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const canCreatePlatform = computed(() => Boolean(currentUser.value?.administrator));
const departmentGroups = computed(() => (currentUser.value?.groups ?? []).filter((group) => group.department));
const canCreateGroup = computed(() => Boolean(currentUser.value?.resource_publisher && departmentGroups.value.length));
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
const visibleBases = computed(() => {
  const needle = props.catalogQuery.trim().toLocaleLowerCase();
  return bases.value.filter((item) => {
    if (props.availableOnly && readyDocumentCount(item) === 0) return false;
    return !needle || `${item.name} ${item.description}`.toLocaleLowerCase().includes(needle);
  });
});
const baseSections = computed(() => [
  { key: "platform", title: t("resources.platformKnowledge"), items: visibleBases.value.filter((item) => knowledgeScope(item) === "platform") },
  { key: "department", title: t("knowledgeBases.departmentKnowledge"), items: visibleBases.value.filter((item) => knowledgeScope(item) === "group") },
  { key: "mine", title: t("resources.myKnowledge"), items: visibleBases.value.filter((item) => knowledgeScope(item) === "private") },
].filter((section) => section.items.length));

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
  emit("detail-open", true);
  resetSearch();
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
  emit("detail-open", false);
  resetSearch();
  categories.value = [];
  documents.value = [];
  activeCategory.value = null;
  uploadCategory.value = "";
}

function resetSearch() {
  searchQuery.value = "";
  searchResults.value = [];
  searchRan.value = false;
  searchIndexReady.value = true;
  searchError.value = "";
}

async function searchKnowledge() {
  const baseID = selected.value?.id;
  const query = searchQuery.value.trim();
  if (!baseID || !query || searchBusy.value) return;
  searchBusy.value = true;
  searchError.value = "";
  searchRan.value = false;
  try {
    const response = await api.searchKnowledgeBase(baseID, query);
    if (selected.value?.id !== baseID) return;
    searchResults.value = response.items;
    searchIndexReady.value = response.index_ready;
    searchRan.value = true;
  } catch {
    if (selected.value?.id === baseID) searchError.value = t("knowledgeBases.searchFailed");
  } finally {
    searchBusy.value = false;
  }
}

function setDisplayMode(mode: DisplayMode) {
  displayMode.value = mode;
  localStorage.setItem("knowledge-base-display", mode);
}

function canManageBase(item: KnowledgeBase) {
  const scope = knowledgeScope(item);
  if (scope === "private" || scope === "platform") return item.owner_id === currentUser.value?.id;
  return Boolean(currentUser.value?.resource_publisher && currentUser.value.groups?.some((group) => group.id === item.group_id));
}

function openCreate() {
  editingBase.value = undefined;
  form.value = { name: "", description: "", scope: "private", group_id: "" };
  showBaseDialog.value = true;
}

function openEdit(item: KnowledgeBase) {
  editingBase.value = item;
  form.value = { name: item.name, description: item.description, scope: knowledgeScope(item), group_id: item.group_id ?? "" };
  showBaseDialog.value = true;
}

function handleBaseAction(command: string | number | object, item: KnowledgeBase) {
  if (command === "edit") openEdit(item);
  if (command === "delete") deleteTarget.value = item;
}

function handleDocumentAction(command: string | number | object, document: KnowledgeDocument) {
  if (!canManageSelected.value) return;
  if (command === "regenerate" || command === "retry") void retryDocument(document);
  if (command === "delete") void deleteDocument(document);
}

async function saveBase() {
  const name = form.value.name.trim();
  if (!name || busy.value) return;
  busy.value = true;
  try {
    const scope = form.value.scope;
    const input = { name, description: form.value.description.trim(), scope, group_id: scope === "group" ? form.value.group_id : undefined, visibility: scope === "platform" ? "public" as const : "private" as const, platform: scope === "platform" };
    if (editingBase.value) {
      const updated = await api.updateKnowledgeBase(editingBase.value.id, input, editingBase.value.version);
      bases.value = bases.value.map((item) => item.id === updated.id ? updated : item);
      if (selected.value?.id === updated.id) selected.value = updated;
    } else {
      const created = await api.createKnowledgeBase(input);
      bases.value = [created, ...bases.value];
      await openBase(created);
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

function chooseFile() {
  fileInput.value?.click();
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
    const baseID = selected.value.id;
    if (documentState(document) === "ready") await api.regenerateKnowledgeDocument(baseID, document.id);
    else await api.retryKnowledgeDocument(baseID, document.id);
    const updated = await api.listKnowledgeDocuments(baseID);
    if (selected.value?.id === baseID) documents.value = updated;
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
function knowledgeScope(item: KnowledgeBase): "private" | "group" | "platform" { return item.scope ?? (item.platform ? "platform" : "private"); }
function scopeLabel(item: KnowledgeBase) {
  if (knowledgeScope(item) === "platform") return t("knowledgeBases.platformScope");
  if (knowledgeScope(item) === "group") return item.group_name || t("knowledgeBases.departmentScope");
  return t("knowledgeBases.private");
}
function trustSource(item: KnowledgeBase) {
  if (knowledgeScope(item) === "platform") return t("resources.platformPublished");
  if (knowledgeScope(item) === "group") return t("knowledgeBases.departmentPublished", { name: item.group_name || t("knowledgeBases.departmentScope") });
  return t("resources.userPublished");
}
function trustPermission(item: KnowledgeBase) {
  if (knowledgeScope(item) === "platform") return canManageBase(item) ? t("resources.allReadOwnerEdit") : t("resources.allReadOnly");
  if (knowledgeScope(item) === "group") return canManageBase(item) ? t("knowledgeBases.departmentReadPublisherEdit") : t("knowledgeBases.departmentReadOnly");
  return t("resources.ownerOnly");
}
function readyDocumentCount(item: KnowledgeBase) { return Number(item.ready_document_count || 0); }
function documentCount(item: KnowledgeBase) { return Number(item.document_count || 0); }
function knowledgeStatus(item: KnowledgeBase) {
  if (readyDocumentCount(item) > 0) return { label: t("resources.retrievalReady"), tone: "success" as const };
  if (documentCount(item) > 0) return { label: t("resources.indexNotReady"), tone: "warning" as const };
  return { label: t("resources.emptyKnowledge"), tone: "info" as const };
}
function knowledgeEvidence(item: KnowledgeBase) {
  const counts = t("resources.knowledgeDocumentEvidence", { ready: readyDocumentCount(item), total: documentCount(item) });
  return item.last_ready_at ? `${counts} · ${t("resources.lastIndexed", { date: formatDate(item.last_ready_at) })}` : counts;
}
function formatSize(size?: number) { if (!size) return "—"; if (size < 1024) return `${size} B`; if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`; return `${(size / 1024 / 1024).toFixed(1)} MB`; }
function documentState(document: KnowledgeDocument) { return document.latest_revision?.state || document.state; }
function documentStatusType(document: KnowledgeDocument): "success" | "danger" | "warning" | "info" {
  switch (documentState(document)) {
    case "ready": return "success";
    case "failed": return "danger";
    case "blocked": return "warning";
    default: return "info";
  }
}
function documentStatusLabel(document: KnowledgeDocument) {
  switch (documentState(document)) {
    case "ready": return t("common.success");
    case "failed": return t("common.failed");
    case "blocked": return t("knowledgeBases.blocked");
    case "accepted": return t("common.queued");
    default: return t("common.running");
  }
}

let statusTimer: ReturnType<typeof setInterval> | undefined;
async function refreshPendingDocuments() {
  const baseID = selected.value?.id;
  if (!baseID || !documents.value.some((document) => ["accepted", "processing"].includes(documentState(document)))) return;
  try {
    const updated = await api.listKnowledgeDocuments(baseID);
    if (selected.value?.id === baseID) documents.value = updated;
  } catch { /* Retry on the next poll. */ }
}

onMounted(() => { void refresh(); statusTimer = setInterval(() => void refreshPendingDocuments(), 5000); });
onUnmounted(() => { if (statusTimer) clearInterval(statusTimer); closePreview(); });
</script>

<template>
  <section class="page-surface knowledge-page">
    <header v-if="!props.embedded" class="page-header">
      <div><p class="eyebrow">{{ t("knowledgeBases.eyebrow") }}</p><h1>{{ t("knowledgeBases.title") }}</h1><p>{{ t("knowledgeBases.subtitle") }}</p></div>
      <div v-if="!selected" class="knowledge-header-actions"><div class="view-toggle" role="group" :aria-label="t('knowledgeBases.displayMode')"><button :class="{ active: displayMode === 'card' }" :aria-label="t('knowledgeBases.cardView')" :title="t('knowledgeBases.cardView')" @click="setDisplayMode('card')"><LayoutGrid :size="16" /></button><button :class="{ active: displayMode === 'list' }" :aria-label="t('knowledgeBases.listView')" :title="t('knowledgeBases.listView')" @click="setDisplayMode('list')"><ListIcon :size="16" /></button></div><el-button type="primary" @click="openCreate"><Plus :size="16" />{{ t("knowledgeBases.new") }}</el-button></div>
    </header>
    <div v-else-if="!selected" class="resource-toolbar knowledge-embedded-toolbar"><div class="view-toggle" role="group" :aria-label="t('knowledgeBases.displayMode')"><button :class="{ active: displayMode === 'card' }" :aria-label="t('knowledgeBases.cardView')" :title="t('knowledgeBases.cardView')" @click="setDisplayMode('card')"><LayoutGrid :size="16" /></button><button :class="{ active: displayMode === 'list' }" :aria-label="t('knowledgeBases.listView')" :title="t('knowledgeBases.listView')" @click="setDisplayMode('list')"><ListIcon :size="16" /></button></div><el-button type="primary" @click="openCreate"><Plus :size="16" />{{ t("knowledgeBases.new") }}</el-button></div>

    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <el-skeleton v-if="loading" :rows="8" animated class="page-loading" />

    <template v-else-if="!selected">
      <div class="catalog-heading"><div><strong>{{ t("knowledgeBases.catalog") }}</strong><span>{{ visibleBases.length }}</span></div><small>{{ t("knowledgeBases.catalogHint") }}</small></div>
      <el-empty v-if="!visibleBases.length" :description="props.catalogQuery ? t('resources.noMatchingResources') : t('knowledgeBases.empty')"><el-button v-if="!props.catalogQuery" type="primary" @click="openCreate">{{ t("knowledgeBases.new") }}</el-button></el-empty>
      <div v-else class="catalog-groups knowledge-catalog-groups">
      <section v-for="section in baseSections" :key="section.key" class="catalog-group"><h2 class="catalog-group-title">{{ section.title }}</h2><div class="knowledge-grid" :class="`is-${displayMode}`">
        <el-card v-for="item in section.items" :key="item.id" class="knowledge-card" shadow="never" role="button" tabindex="0" @click="openBase(item)" @keydown.enter="openBase(item)" @keydown.space.prevent="openBase(item)">
          <div class="knowledge-card-head"><div class="knowledge-icon"><FolderOpen :size="20" /></div><div class="knowledge-card-top-actions"><el-tag size="small" effect="plain"><Globe2 v-if="knowledgeScope(item) === 'platform'" :size="12" /><LockKeyhole v-else :size="12" />{{ scopeLabel(item) }}</el-tag><el-dropdown v-if="canManageBase(item)" trigger="click" @command="handleBaseAction($event, item)"><el-button class="card-more" text circle :aria-label="t('common.more')" :title="t('common.more')" @click.stop><MoreHorizontal :size="18" /></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item command="edit"><Pencil :size="14" />{{ t("common.edit") }}</el-dropdown-item><el-dropdown-item command="delete" divided><Trash2 :size="14" />{{ t("common.delete") }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown></div></div>
          <div class="knowledge-card-copy"><div class="knowledge-card-title"><h2>{{ item.name }}</h2></div><p>{{ item.description || t("knowledgeBases.noDescription") }}</p><ResourceTrustMeta :source="trustSource(item)" :permission="trustPermission(item)" :status="knowledgeStatus(item).label" :status-tone="knowledgeStatus(item).tone" :detail="knowledgeEvidence(item)" /></div>
          <footer><span>{{ formatDate(item.updated_at) }}</span></footer>
        </el-card>
      </div></section></div>
    </template>

    <template v-else>
      <input ref="fileInput" type="file" hidden @change="upload" />
      <header class="knowledge-detail-header">
        <button class="back-link knowledge-back" @click="closeBase"><ArrowLeft :size="16" />{{ t("knowledgeBases.backToCatalog") }}</button>
        <div class="knowledge-detail-copy"><div class="detail-title-line"><h2>{{ selected.name }}</h2><el-tag size="small" effect="plain">{{ scopeLabel(selected) }}</el-tag></div><p v-if="selected.description">{{ selected.description }}</p></div>
        <div v-if="canManageSelected" class="detail-actions"><el-button text @click="openEdit(selected)"><Pencil :size="15" />{{ t("common.edit") }}</el-button><el-button type="danger" text @click="deleteTarget = selected"><Trash2 :size="15" />{{ t("common.delete") }}</el-button></div>
      </header>

      <section class="knowledge-search-panel" :aria-label="t('knowledgeBases.search')">
        <form class="knowledge-search-controls" @submit.prevent="searchKnowledge">
          <el-input v-model="searchQuery" :maxlength="500" :placeholder="t('knowledgeBases.searchPlaceholder')" :aria-label="t('knowledgeBases.search')" clearable />
          <el-button type="primary" native-type="submit" :loading="searchBusy" :disabled="!searchQuery.trim()"><Search :size="16" />{{ t("knowledgeBases.search") }}</el-button>
        </form>
        <p v-if="searchError" class="knowledge-search-feedback" role="alert">{{ searchError }}</p>
        <p v-else-if="searchRan && !searchIndexReady" class="knowledge-search-feedback">{{ t("knowledgeBases.searchNotReady") }}</p>
        <p v-else-if="searchRan && !searchResults.length" class="knowledge-search-feedback">{{ t("knowledgeBases.searchEmpty") }}</p>
        <ol v-else-if="searchRan" class="knowledge-search-results">
          <li v-for="item in searchResults" :key="`${item.revision_id}-${item.document_id}-${item.text.slice(0, 30)}`">
            <p>{{ item.text }}</p>
            <small>{{ t("knowledgeBases.searchSource") }}：{{ item.document_name }}<span v-if="item.category_name"> · {{ item.category_name }}</span></small>
          </li>
        </ol>
      </section>

      <section class="documents-panel">
        <div class="section-heading documents-heading">
          <div class="documents-title"><h3>{{ activeCategoryName }}</h3><span>{{ visibleDocuments.length }} {{ t("knowledgeBases.documentCount") }}</span></div>
          <form v-if="canManageSelected" class="category-create" @submit.prevent="createCategory">
            <el-input v-model="newCategory" :placeholder="t('knowledgeBases.categoryPlaceholder')" :aria-label="t('knowledgeBases.categories')" />
            <el-button native-type="submit" :disabled="!newCategory.trim()"><Plus :size="15" />{{ t("knowledgeBases.addCategory") }}</el-button>
          </form>
        </div>
        <nav v-if="categories.length" class="category-navigation" :aria-label="t('knowledgeBases.categories')">
          <button :aria-pressed="activeCategory === null" @click="openCategory(null)">{{ t("knowledgeBases.allDocuments") }}<span>{{ documents.length }}</span></button>
          <button v-for="category in categories" :key="category.id" :aria-pressed="activeCategory === category.id" @click="openCategory(category.id)">{{ category.name }}<span>{{ categoryCounts.get(category.id) || 0 }}</span></button>
          <button v-if="unclassifiedCount" :aria-pressed="activeCategory === 'unclassified'" @click="openCategory('unclassified')">{{ t("knowledgeBases.unclassified") }}<span>{{ unclassifiedCount }}</span></button>
        </nav>
        <div v-if="canManageSelected" class="source-controls">
          <div class="document-upload-controls">
            <el-select v-model="uploadCategory" :placeholder="t('knowledgeBases.unclassified')" :aria-label="t('knowledgeBases.categories')" clearable><el-option v-for="category in categories" :key="category.id" :value="category.id" :label="category.name" /></el-select>
            <el-button type="primary" @click="chooseFile"><Upload :size="15" />{{ t("knowledgeBases.upload") }}</el-button>
          </div>
          <form class="document-url-controls" @submit.prevent="importURL">
            <el-input v-model="sourceURL" :placeholder="t('knowledgeBases.urlPlaceholder')" :aria-label="t('knowledgeBases.urlPlaceholder')" />
            <el-button native-type="submit" :loading="busy" :disabled="!sourceURL.trim()"><Globe2 :size="15" />{{ t("knowledgeBases.importURL") }}</el-button>
          </form>
        </div>
        <div v-if="visibleDocuments.length" class="document-table-scroll" role="region" :aria-label="t('knowledgeBases.document')" tabindex="0">
          <el-table :data="visibleDocuments" class="document-table">
            <el-table-column min-width="240" :label="t('knowledgeBases.document')"><template #default="scope"><div class="document-name"><span class="document-icon"><FileText :size="16" /></span><span><strong :title="(scope.row as KnowledgeDocument).name">{{ (scope.row as KnowledgeDocument).name }}</strong><small>{{ formatSize((scope.row as KnowledgeDocument).latest_revision?.size) }}</small></span></div></template></el-table-column>
            <el-table-column :label="t('knowledgeBases.state')" width="110"><template #default="scope"><el-tag size="small" :type="documentStatusType(scope.row as KnowledgeDocument)" :title="(scope.row as KnowledgeDocument).latest_revision?.error || (scope.row as KnowledgeDocument).error || ''">{{ documentStatusLabel(scope.row as KnowledgeDocument) }}</el-tag></template></el-table-column>
            <el-table-column prop="source_type" :label="t('knowledgeBases.source')" width="90" />
            <el-table-column :label="t('knowledgeBases.updated')" width="120"><template #default="scope">{{ formatDate((scope.row as KnowledgeDocument).updated_at) }}</template></el-table-column>
            <el-table-column width="240" align="right"><template #default="scope"><div class="document-actions">
              <el-button text @click="previewDocument(scope.row as KnowledgeDocument)"><Eye :size="15" />{{ t("knowledgeBases.preview") }}</el-button>
              <el-button text @click="downloadDocument(scope.row as KnowledgeDocument)"><Download :size="15" />{{ t("common.download") }}</el-button>
              <el-dropdown v-if="canManageSelected" trigger="click" :popper-options="{ modifiers: [{ name: 'preventOverflow', options: { tether: false } }] }" @command="handleDocumentAction($event, scope.row as KnowledgeDocument)">
                <el-button text :aria-label="t('common.more')"><MoreHorizontal :size="15" />{{ t("common.more") }}</el-button>
                <template #dropdown><el-dropdown-menu>
                  <el-dropdown-item v-if="['ready', 'failed'].includes(documentState(scope.row as KnowledgeDocument))" :command="documentState(scope.row as KnowledgeDocument) === 'ready' ? 'regenerate' : 'retry'">{{ documentState(scope.row as KnowledgeDocument) === 'ready' ? t('knowledgeBases.regenerate') : t('knowledgeBases.retry') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" class="danger-text" :divided="['ready', 'failed'].includes(documentState(scope.row as KnowledgeDocument))"><Trash2 :size="15" />{{ t("common.delete") }}</el-dropdown-item>
                </el-dropdown-menu></template>
              </el-dropdown>
            </div></template></el-table-column>
          </el-table>
        </div>
        <el-empty v-else :description="t('knowledgeBases.noDocuments')"><el-button v-if="canManageSelected" type="primary" plain @click="chooseFile"><Upload :size="15" />{{ t("knowledgeBases.upload") }}</el-button></el-empty>
      </section>
    </template>
  </section>

  <el-dialog v-model="showBaseDialog" class="resource-dialog" width="min(560px, calc(100vw - 32px))" align-center :title="editingBase ? t('knowledgeBases.edit') : t('knowledgeBases.new')"><el-form label-position="top" @submit.prevent="saveBase"><el-form-item :label="t('common.name')" required><el-input v-model="form.name" maxlength="100" autofocus /></el-form-item><el-form-item :label="t('knowledgeBases.description')"><el-input v-model="form.description" type="textarea" :rows="3" /></el-form-item><el-form-item :label="t('knowledgeBases.scope')"><el-select v-model="form.scope" :disabled="Boolean(editingBase)"><el-option value="private" :label="t('knowledgeBases.privateScope')" /><el-option v-if="canCreateGroup" value="group" :label="t('knowledgeBases.departmentScope')" /><el-option v-if="canCreatePlatform" value="platform" :label="t('knowledgeBases.platformScope')" /></el-select><small class="scope-hint">{{ editingBase ? t('knowledgeBases.scopeImmutable') : t(`knowledgeBases.scopeHint.${form.scope}`) }}</small></el-form-item><el-form-item v-if="form.scope === 'group'" :label="t('knowledgeBases.department')" required><el-select v-model="form.group_id"><el-option v-for="group in departmentGroups" :key="group.id" :value="group.id" :label="group.name" /></el-select></el-form-item></el-form><template #footer><el-button @click="showBaseDialog = false">{{ t("common.cancel") }}</el-button><el-button type="primary" :loading="busy" :disabled="!form.name.trim() || (form.scope === 'group' && !form.group_id)" @click="saveBase">{{ t("common.save") }}</el-button></template></el-dialog>
  <ConfirmDialog :open="Boolean(deleteTarget)" :title="t('knowledgeBases.deleteBase')" :message="deleteTarget ? `${t('common.delete')} “${deleteTarget.name}”?` : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger :busy="busy" @cancel="deleteTarget = undefined" @confirm="removeBase" />
  <el-dialog v-model="previewOpen" class="document-preview-dialog" width="min(980px, calc(100vw - 32px))" align-center :title="preview?.document.name" @close="closePreview"><div v-if="previewBusy" class="preview-loading">{{ t("common.loading") }}</div><template v-else-if="preview"><img v-if="preview.kind === 'image'" :src="preview.url" :alt="preview.document.name" class="preview-image" /><iframe v-else-if="preview.kind === 'pdf'" :src="preview.url" :title="preview.document.name" class="preview-frame" /><pre v-else-if="preview.kind === 'text'" class="preview-text">{{ preview.text }}</pre><div v-else class="preview-unsupported"><FileText :size="32" /><p>{{ t("knowledgeBases.previewUnsupported") }}</p><el-button type="primary" @click="downloadDocument(preview.document)"><Download :size="15" />{{ t("common.download") }}</el-button></div></template></el-dialog>
</template>

<style scoped>
.knowledge-page { max-width: 1480px; }
.knowledge-embedded-toolbar { justify-content: flex-end; margin-bottom: 18px; }
.knowledge-catalog-groups { gap: 24px; }
.knowledge-header-actions, .view-toggle { display: flex; align-items: center; gap: 8px; }
.view-toggle { padding: 3px; border: 1px solid var(--line); border-radius: 10px; background: color-mix(in srgb, var(--aw-n0) 72%, transparent); }
.view-toggle button { width: 32px; height: 30px; display: grid; place-items: center; border: 0; border-radius: 7px; color: var(--muted); background: transparent; }
.view-toggle button.active { color: var(--aw-primary); background: var(--aw-primary-soft); }
.catalog-heading { display: flex; align-items: end; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.catalog-heading > div { display: flex; align-items: center; gap: 9px; }
.catalog-heading > div span { color: var(--muted); font-size: .75rem; }
.catalog-heading small { color: var(--muted); font-size: .73rem; }
.knowledge-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.knowledge-grid.is-list { grid-template-columns: 1fr; }
.knowledge-card { min-width: 0; border-color: var(--aw-n5); border-radius: 15px; background: color-mix(in srgb, var(--aw-n0) 86%, transparent); cursor: pointer; transition: transform .2s ease, border-color .2s ease, box-shadow .2s ease; }
.knowledge-card:hover, .knowledge-card:focus-visible { border-color: var(--aw-primary-border); box-shadow: var(--aw-shadow-overlay); transform: translateY(-2px); outline: none; }
.knowledge-card :deep(.el-card__body) { display: flex; flex-direction: column; min-height: 186px; padding: 20px; }
.knowledge-card-head, .knowledge-card-title, .knowledge-card footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.knowledge-card-head { width: 100%; }
.knowledge-icon { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; color: var(--aw-primary); background: var(--aw-primary-soft); }
.knowledge-card-top-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-left: auto; }
.card-more { color: var(--muted); }
.knowledge-card-copy { min-width: 0; margin-top: 18px; }
.knowledge-card-title h2 { min-width: 0; margin: 0; overflow: hidden; font-size: 1.05rem; text-overflow: ellipsis; white-space: nowrap; }
.knowledge-card-title .el-tag { display: inline-flex; align-items: center; gap: 4px; flex: 0 0 auto; }
.knowledge-card-copy p { min-height: 2.6em; margin: 8px 0 14px; color: var(--muted); font-size: .78rem; line-height: 1.5; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; }
.knowledge-card footer { justify-content: flex-end; margin-top: auto; color: var(--muted); font-size: .7rem; }
.knowledge-grid.is-list .knowledge-card :deep(.el-card__body) { min-height: 0; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; grid-template-rows: auto auto; gap: 8px 16px; align-items: center; }
.knowledge-grid.is-list .knowledge-card-head { display: contents; }
.knowledge-grid.is-list .knowledge-icon { grid-column: 1; grid-row: 1 / span 2; align-self: center; }
.knowledge-grid.is-list .knowledge-card-top-actions { grid-column: 3; grid-row: 1; align-self: start; }
.knowledge-grid.is-list .knowledge-card-copy { grid-column: 2; grid-row: 1 / span 2; margin: 0; align-self: center; }
.knowledge-grid.is-list .knowledge-card-copy p { min-height: 0; margin: 5px 0 0; }
.knowledge-grid.is-list .knowledge-card footer { grid-column: 3; grid-row: 2; display: flex; flex-direction: column; align-items: flex-end; gap: 7px; }
.preview-loading, .preview-unsupported { display: grid; place-items: center; min-height: 300px; gap: 12px; color: var(--muted); text-align: center; }
.preview-image { display: block; max-width: 100%; max-height: 68vh; margin: 0 auto; object-fit: contain; }
.preview-frame { width: 100%; height: 68vh; border: 0; }
.preview-text { max-height: 68vh; margin: 0; padding: 16px; overflow: auto; border-radius: 9px; background: var(--aw-n2); color: var(--ink); font: .78rem/1.65 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; }
.scope-hint { display: block; margin-top: 7px; color: var(--muted); line-height: 1.45; }
@media (max-width: 1050px) { .knowledge-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 760px) { .knowledge-header-actions { align-items: flex-start; flex-direction: column; width: 100%; flex-wrap: wrap; } .knowledge-header-actions .el-button { flex: 1; justify-content: center; } .knowledge-grid { grid-template-columns: 1fr; } .knowledge-grid.is-list .knowledge-card :deep(.el-card__body) { grid-template-columns: auto minmax(0, 1fr) auto; } .knowledge-grid.is-list .knowledge-card footer { flex-direction: column; align-items: flex-end; justify-content: flex-end; } }
</style>
