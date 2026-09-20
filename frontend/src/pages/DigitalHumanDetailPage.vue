<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type DigitalHuman } from "../api/client";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const human = ref<DigitalHuman>();
const error = ref("");
const saving = ref(false);
const id = String(route.params.digitalHumanId);
async function refresh() { try { human.value = await api.getDigitalHuman(id); } catch { error.value = t("aiApplications.loadFailed"); } }
async function save() { if (!human.value) return; saving.value = true; try { human.value = await api.updateDigitalHuman(id, { name: human.value.name, avatar_object_key: human.value.avatar_object_key, voice: human.value.voice, language: human.value.language, expression_style: human.value.expression_style, scene_description: human.value.scene_description }, human.value.version); } catch { error.value = t("aiApplications.saveFailed"); } finally { saving.value = false; } }
onMounted(refresh);
</script>
<template>
  <section class="application-detail-page"><header class="application-detail-header"><el-button text @click="router.push('/ai-apps/digital-humans')">← {{ t('common.back') }}</el-button><div><p class="eyebrow">{{ t('aiApplications.digitalHumans.title') }}</p><h1>{{ human?.name || t('common.loading') }}</h1></div><el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button></header><el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" /><el-card v-if="human" class="application-detail-card"><el-form label-position="top"><el-form-item :label="t('aiApplications.name')"><el-input v-model="human.name" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.avatar')"><el-input v-model="human.avatar_object_key" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.voice')"><el-input v-model="human.voice" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.language')"><el-input v-model="human.language" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.expression')"><el-input v-model="human.expression_style" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.scene')"><el-input v-model="human.scene_description" type="textarea" :rows="4" /></el-form-item></el-form><el-alert type="info" :closable="false" :title="t('aiApplications.digitalHumans.configOnly')" /></el-card></section>
</template>
