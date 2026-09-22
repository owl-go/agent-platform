<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { Plus } from "@element-plus/icons-vue";
import { Eye, Pencil, Trash2 } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type DigitalHuman } from "../api/client";

const api = inject(platformApiKey)!;
const router = useRouter();
const { t } = useI18n();
const items = ref<DigitalHuman[]>([]);
const loading = ref(false);
const error = ref("");
const name = ref("");
const createDialogOpen = ref(false);
async function refresh() { loading.value = true; try { items.value = await api.listDigitalHumans(); } catch { error.value = t("aiApplications.loadFailed"); } finally { loading.value = false; } }
async function create() { if (!name.value.trim()) return; try { const item = await api.createDigitalHuman({ name: name.value }); items.value.unshift(item); name.value = ""; createDialogOpen.value = false; } catch { error.value = t("aiApplications.saveFailed"); } }
function openCreate() { createDialogOpen.value = true; }
async function remove(id: string) { try { await api.deleteDigitalHuman(id); items.value = items.value.filter((item) => item.id !== id); } catch { error.value = t("aiApplications.deleteFailed"); } }
function formatDate(value: string) { return new Date(value).toLocaleString(); }
async function openEdit(id: string) { await router.push(`/ai-apps/digital-humans/${id}?mode=edit`); }
async function openDetail(id: string) { await router.push(`/ai-apps/digital-humans/${id}`); }
onMounted(refresh);
</script>
<template>
  <section class="application-catalog-page">
    <div class="application-catalog-toolbar"><div><h2>{{ t('aiApplications.digitalHumans.title') }}</h2></div><el-button class="application-create-trigger" type="primary" :icon="Plus" @click="openCreate">{{ t('aiApplications.create') }}</el-button></div>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <el-dialog v-model="createDialogOpen" class="application-create-dialog" :title="t('aiApplications.create')" width="min(560px, 92vw)" destroy-on-close>
      <el-form label-position="top" @submit.prevent="create"><el-form-item :label="t('aiApplications.name')"><el-input v-model="name" autofocus :placeholder="t('aiApplications.digitalHumanNamePlaceholder')" /></el-form-item></el-form>
      <template #footer><el-button @click="createDialogOpen = false">{{ t('common.cancel') }}</el-button><el-button type="primary" @click="create">{{ t('common.save') }}</el-button></template>
    </el-dialog>
    <el-table v-loading="loading" :data="items" row-key="id" class="digital-human-table"><el-table-column prop="name" :label="t('common.name')" min-width="260" /><el-table-column :label="t('aiApplications.createdAt')" width="220"><template #default="{ row }">{{ formatDate((row as DigitalHuman).created_at) }}</template></el-table-column><el-table-column :label="t('aiApplications.actions')" width="280" align="right"><template #default="{ row }"><div class="digital-human-actions"><el-button text :icon="Pencil" @click.stop="openEdit((row as DigitalHuman).id)">{{ t('common.edit') }}</el-button><el-button text :icon="Eye" @click.stop="openDetail((row as DigitalHuman).id)">{{ t('aiApplications.detail') }}</el-button><el-button text type="danger" :icon="Trash2" @click.stop="remove((row as DigitalHuman).id)">{{ t('common.delete') }}</el-button></div></template></el-table-column><template #empty><el-empty :description="t('aiApplications.digitalHumans.empty')" /></template></el-table>
  </section>
</template>
