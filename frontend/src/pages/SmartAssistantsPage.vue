<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { Plus } from "@element-plus/icons-vue";
import { Trash2 } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type SmartAssistant } from "../api/client";

const api = inject(platformApiKey)!;
const router = useRouter();
const { t } = useI18n();
const items = ref<SmartAssistant[]>([]);
const loading = ref(false);
const error = ref("");
const form = ref({ name: "", service_goal: "", operating_rules: "", response_style: "" });
async function refresh() { loading.value = true; try { items.value = await api.listSmartAssistants(); } catch { error.value = t("aiApplications.loadFailed"); } finally { loading.value = false; } }
async function create() { if (!form.value.name.trim()) return; try { const item = await api.createSmartAssistant({ ...form.value, share: { enabled: false, width: "100%", height: 600 } }); items.value.unshift(item); form.value = { name: "", service_goal: "", operating_rules: "", response_style: "" }; } catch { error.value = t("aiApplications.saveFailed"); } }
async function remove(id: string) { try { await api.deleteSmartAssistant(id); items.value = items.value.filter((item) => item.id !== id); } catch { error.value = t("aiApplications.deleteFailed"); } }
onMounted(refresh);
</script>
<template>
  <section class="application-catalog-page">
    <div class="application-catalog-toolbar"><div><h2>{{ t('aiApplications.assistants.title') }}</h2><p>{{ t('aiApplications.assistants.subtitle') }}</p></div><el-button type="primary" :icon="Plus" @click="create">{{ t('aiApplications.create') }}</el-button></div>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <el-form class="application-create-form" :inline="true" @submit.prevent="create"><el-form-item :label="t('aiApplications.name')"><el-input v-model="form.name" :placeholder="t('aiApplications.assistantNamePlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.goal')"><el-input v-model="form.service_goal" /></el-form-item><el-form-item><el-button native-type="submit" type="primary">{{ t('common.save') }}</el-button></el-form-item></el-form>
    <div v-loading="loading" class="application-card-grid"><el-card v-for="item in items" :key="item.id" class="application-card" role="button" tabindex="0" @click="router.push(`/ai-apps/assistants/${item.id}`)"><div class="application-card-heading"><div><h3>{{ item.name }}</h3><p>{{ item.service_goal || item.introduction || t('aiApplications.assistants.defaultIntro') }}</p></div><el-button text type="danger" :icon="Trash2" :aria-label="t('common.delete')" @click.stop="remove(item.id)" /></div><small>{{ item.share.enabled ? t('aiApplications.shared') : t('aiApplications.private') }}</small></el-card><el-empty v-if="!loading && !items.length" :description="t('aiApplications.assistants.empty')" /></div>
  </section>
</template>
