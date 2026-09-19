<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { Plus } from "@element-plus/icons-vue";
import { Trash2 } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type DigitalHuman } from "../api/client";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const items = ref<DigitalHuman[]>([]);
const loading = ref(false);
const error = ref("");
const name = ref("");
async function refresh() { loading.value = true; try { items.value = await api.listDigitalHumans(); } catch { error.value = t("aiApplications.loadFailed"); } finally { loading.value = false; } }
async function create() { if (!name.value.trim()) return; try { const item = await api.createDigitalHuman({ name: name.value }); items.value.unshift(item); name.value = ""; } catch { error.value = t("aiApplications.saveFailed"); } }
async function remove(id: string) { try { await api.deleteDigitalHuman(id); items.value = items.value.filter((item) => item.id !== id); } catch { error.value = t("aiApplications.deleteFailed"); } }
onMounted(refresh);
</script>
<template>
  <section class="application-catalog-page">
    <div class="application-catalog-toolbar"><div><h2>{{ t('aiApplications.digitalHumans.title') }}</h2><p>{{ t('aiApplications.digitalHumans.subtitle') }}</p></div><el-button type="primary" :icon="Plus" @click="create">{{ t('aiApplications.create') }}</el-button></div>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <el-form class="application-create-form" :inline="true" @submit.prevent="create"><el-form-item :label="t('aiApplications.name')"><el-input v-model="name" :placeholder="t('aiApplications.digitalHumanNamePlaceholder')" /></el-form-item><el-form-item><el-button native-type="submit" type="primary">{{ t('common.save') }}</el-button></el-form-item></el-form>
    <div v-loading="loading" class="application-card-grid"><el-card v-for="item in items" :key="item.id" class="application-card"><div class="application-card-heading"><div><h3>{{ item.name }}</h3><p>{{ item.language || t('aiApplications.digitalHumans.defaultIntro') }}</p></div><el-button text type="danger" :icon="Trash2" :aria-label="t('common.delete')" @click="remove(item.id)" /></div><small>{{ item.voice || t('aiApplications.digitalHumans.configOnly') }}</small></el-card><el-empty v-if="!loading && !items.length" :description="t('aiApplications.digitalHumans.empty')" /></div>
  </section>
</template>
