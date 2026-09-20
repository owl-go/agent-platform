<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type KnowledgeBase, type KnowledgeDocument } from "../api/client";

const api = inject(platformApiKey)!;
const router = useRouter();
const { t } = useI18n();
const bases = ref<KnowledgeBase[]>([]);
const selected = ref<KnowledgeBase>();
const documents = ref<KnowledgeDocument[]>([]);
const error = ref("");
const loading = ref(false);
const baseDraft = ref({ name: "", description: "" });
const documentDraft = ref({ name: "", content: "" });

async function loadDocuments(base?: KnowledgeBase) {
  selected.value = base;
  documents.value = base ? await api.listKnowledgeDocuments(base.id) : [];
}
async function refresh() {
  loading.value = true;
  try {
    bases.value = await api.listKnowledgeBases();
    await loadDocuments(selected.value ? bases.value.find((item) => item.id === selected.value?.id) : bases.value[0]);
  } catch {
    error.value = t("aiApplications.loadFailed");
  } finally {
    loading.value = false;
  }
}
async function createBase() {
  if (!baseDraft.value.name.trim()) return;
  try {
    const base = await api.createKnowledgeBase(baseDraft.value);
    bases.value.unshift(base);
    baseDraft.value = { name: "", description: "" };
    await loadDocuments(base);
  } catch {
    error.value = t("aiApplications.saveFailed");
  }
}
async function addDocument() {
  if (!selected.value || !documentDraft.value.name.trim() || !documentDraft.value.content.trim()) return;
  try {
    documents.value.unshift(await api.createKnowledgeDocument(selected.value.id, documentDraft.value));
    documentDraft.value = { name: "", content: "" };
  } catch {
    error.value = t("aiApplications.saveFailed");
  }
}
onMounted(refresh);
</script>

<template>
  <section class="application-detail-page knowledge-base-page">
    <header class="application-detail-header">
      <div><el-button text @click="router.push('/ai-apps/assistants')">← {{ t('common.back') }}</el-button><p class="eyebrow">{{ t('aiApplications.title') }}</p><h1>{{ t('aiApplications.knowledge.title') }}</h1><p class="application-card-copy">{{ t('aiApplications.knowledge.subtitle') }}</p></div>
    </header>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <el-card class="application-detail-card">
      <template #header><strong>{{ t('aiApplications.knowledge.create') }}</strong></template>
      <el-form :inline="true" @submit.prevent="createBase"><el-form-item :label="t('aiApplications.name')"><el-input v-model="baseDraft.name" /></el-form-item><el-form-item :label="t('aiApplications.knowledge.description')"><el-input v-model="baseDraft.description" /></el-form-item><el-form-item><el-button type="primary" native-type="submit">{{ t('aiApplications.create') }}</el-button></el-form-item></el-form>
    </el-card>
    <div v-loading="loading" class="knowledge-base-layout">
      <el-card class="application-detail-card">
        <template #header><strong>{{ t('aiApplications.knowledge.list') }}</strong></template>
        <el-empty v-if="!bases.length" :description="t('aiApplications.knowledge.empty')" />
        <button v-for="base in bases" :key="base.id" type="button" class="knowledge-base-item" :class="{ active: selected?.id === base.id }" @click="loadDocuments(base)"><strong>{{ base.name }}</strong><small>{{ base.description || t('aiApplications.knowledge.noDescription') }}</small></button>
      </el-card>
      <el-card class="application-detail-card">
        <template #header><strong>{{ selected?.name || t('aiApplications.knowledge.selectTitle') }}</strong></template>
        <template v-if="selected">
          <div v-for="document in documents" :key="document.id" class="faq-row"><div><strong>{{ document.name }}</strong><p>{{ document.state }}</p></div><el-tag size="small">{{ document.content_sha256.slice(0, 8) }}</el-tag></div>
          <el-empty v-if="!documents.length" :description="t('aiApplications.knowledge.documentsEmpty')" />
          <el-form class="faq-create-form" label-position="top" @submit.prevent="addDocument"><el-form-item :label="t('aiApplications.knowledge.documentName')"><el-input v-model="documentDraft.name" /></el-form-item><el-form-item :label="t('aiApplications.knowledge.documentContent')"><el-input v-model="documentDraft.content" type="textarea" :rows="7" /></el-form-item><el-button type="primary" native-type="submit">{{ t('aiApplications.knowledge.addDocument') }}</el-button></el-form>
        </template>
        <el-empty v-else :description="t('aiApplications.knowledge.selectHint')" />
      </el-card>
    </div>
  </section>
</template>
