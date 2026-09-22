<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type DigitalHuman, type DigitalHumanPreview } from "../api/client";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const human = ref<DigitalHuman>();
const preview = ref<DigitalHumanPreview>();
const error = ref("");
const saving = ref(false);
const id = String(route.params.digitalHumanId);
const editing = computed(() => route.query.mode === "edit");
const avatarPreview = computed(() => human.value?.avatar_object_key?.startsWith("data:image/") ? human.value.avatar_object_key : "");
async function refresh() { try { human.value = await api.getDigitalHuman(id); } catch { error.value = t("aiApplications.loadFailed"); } }
async function save() { if (!human.value) return; saving.value = true; try { human.value = await api.updateDigitalHuman(id, { name: human.value.name, avatar_object_key: human.value.avatar_object_key, voice: human.value.voice, language: human.value.language, expression_style: human.value.expression_style, scene_description: human.value.scene_description }, human.value.version); } catch { error.value = t("aiApplications.saveFailed"); } finally { saving.value = false; } }
async function toggleState() { if (!human.value) return; try { human.value = await api.setDigitalHumanState(id, human.value.state === "enabled" ? "disabled" : "enabled", human.value.version); } catch { error.value = t("aiApplications.saveFailed"); } }
async function loadPreview() { try { preview.value = await api.previewDigitalHuman(id); } catch { error.value = t("aiApplications.loadFailed"); } }
function selectAvatar(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file || !human.value) return;
  if (!file.type.startsWith("image/") || file.size > 2 * 1024 * 1024) {
    error.value = t("aiApplications.digitalHuman.avatarInvalid");
    return;
  }
  const reader = new FileReader();
  reader.onload = () => { if (typeof reader.result === "string" && human.value) human.value.avatar_object_key = reader.result; };
  reader.readAsDataURL(file);
}
onMounted(refresh);
</script>
<template>
  <section class="application-detail-page"><header class="application-detail-header"><el-button text @click="router.push('/ai-apps/digital-humans')">← {{ t('common.back') }}</el-button><div class="assistant-detail-actions"><el-button v-if="!editing" @click="router.push({ query: { mode: 'edit' } })">{{ t('common.edit') }}</el-button><el-button v-if="editing" @click="toggleState">{{ human?.state === 'enabled' ? t('aiApplications.disable') : t('aiApplications.enable') }}</el-button><el-button v-if="editing" type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button></div></header><el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" /><el-card v-if="human" class="application-detail-card"><el-form label-position="top"><el-form-item :label="t('aiApplications.name')"><el-input v-model="human.name" :disabled="!editing" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.avatar')"><div class="digital-human-avatar-field"><img v-if="avatarPreview" :src="avatarPreview" :alt="t('aiApplications.digitalHuman.avatar')" class="digital-human-avatar-preview"><span v-else class="digital-human-avatar-empty">{{ t('aiApplications.digitalHuman.avatarEmpty') }}</span><label v-if="editing" class="digital-human-avatar-upload"><span>{{ t('aiApplications.digitalHuman.uploadAvatar') }}</span><input data-testid="digital-human-avatar-file" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="selectAvatar"></label><el-input v-model="human.avatar_object_key" :disabled="!editing" :placeholder="t('aiApplications.digitalHuman.avatarPlaceholder')" /></div></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.voice')"><el-input v-model="human.voice" :disabled="!editing" :placeholder="t('aiApplications.digitalHuman.voicePlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.language')"><el-input v-model="human.language" :disabled="!editing" :placeholder="t('aiApplications.digitalHuman.languagePlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.expression')"><el-input v-model="human.expression_style" :disabled="!editing" :placeholder="t('aiApplications.digitalHuman.expressionPlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.digitalHuman.scene')"><el-input v-model="human.scene_description" :disabled="!editing" type="textarea" :rows="4" :placeholder="t('aiApplications.digitalHuman.scenePlaceholder')" /></el-form-item></el-form><el-button data-testid="digital-human-preview" type="primary" @click="loadPreview">{{ t('aiApplications.digitalHuman.preview') }}</el-button><el-card v-if="preview" class="application-detail-card"><strong>{{ preview.name }}</strong><p>{{ preview.preview_text }}</p><p>{{ preview.scene_description }}</p></el-card></el-card></section>
</template>
