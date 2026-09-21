<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type ApplicationKnowledgeBase, type SmartAssistant, type SmartAssistantFAQ } from "../api/client";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const assistant = ref<SmartAssistant>();
const faqs = ref<SmartAssistantFAQ[]>([]);
const knowledgeBases = ref<ApplicationKnowledgeBase[]>([]);
const error = ref("");
const saving = ref(false);
const faqDraft = ref({ question: "", answer_markdown: "", display_order: 0, category: "", tag: "", icon: "", enabled: true });
const shareToken = ref("");
const embedSnippet = computed(() => assistant.value && shareToken.value ? `<iframe src="${window.location.origin}/embed/assistant/${shareToken.value}" width="${assistant.value.share.width}" height="${assistant.value.share.height}"></iframe>` : "");
const allowedOrigins = computed({ get: () => assistant.value?.share.allowed_origins?.join("\n") || "", set: (value: string) => { if (assistant.value) assistant.value.share.allowed_origins = value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean); } });
const id = String(route.params.assistantId);

async function refresh() { try { [assistant.value, faqs.value, knowledgeBases.value] = await Promise.all([api.getSmartAssistant(id), api.listAssistantFAQs(id), api.listApplicationKnowledgeBases()]); } catch { error.value = t("aiApplications.loadFailed"); } }
async function save() { if (!assistant.value) return; saving.value = true; try { assistant.value = await api.updateSmartAssistant(id, assistant.value, assistant.value.version); } catch { error.value = t("aiApplications.saveFailed"); } finally { saving.value = false; } }
async function addFAQ() { if (!faqDraft.value.question.trim() || !faqDraft.value.answer_markdown.trim()) return; try { faqs.value.push(await api.createAssistantFAQ(id, faqDraft.value)); faqDraft.value = { question: "", answer_markdown: "", display_order: faqs.value.length, category: "", tag: "", icon: "", enabled: true }; } catch { error.value = t("aiApplications.saveFailed"); } }
async function removeFAQ(faq: SmartAssistantFAQ) { try { await api.deleteAssistantFAQ(id, faq.id); faqs.value = faqs.value.filter((item) => item.id !== faq.id); } catch { error.value = t("aiApplications.deleteFailed"); } }
async function regenerateToken() { if (!assistant.value) return; try { const result = await api.regenerateAssistantShareToken(id, assistant.value.version); assistant.value = result.assistant; shareToken.value = result.token; } catch { error.value = t("aiApplications.saveFailed"); } }
async function startConversation() { try { const session = await api.createAssistantSession(id); await router.push(`/sessions?open=${encodeURIComponent(session.id)}`); } catch { error.value = t("aiApplications.saveFailed"); } }
onMounted(refresh);
</script>

<template>
  <section class="application-detail-page">
    <header class="application-detail-header"><el-button text @click="router.push('/ai-apps/assistants')">← {{ t('common.back') }}</el-button><div><p class="eyebrow">{{ t('aiApplications.assistants.title') }}</p><h1>{{ assistant?.name || t('common.loading') }}</h1></div><div><el-button @click="startConversation">{{ t('aiApplications.startConversation') }}</el-button><el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button></div></header>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <template v-if="assistant">
      <el-card class="application-detail-card"><el-form label-position="top"><el-form-item :label="t('aiApplications.name')"><el-input v-model="assistant.name" /></el-form-item><el-form-item :label="t('aiApplications.goal')"><el-input v-model="assistant.service_goal" /></el-form-item><el-form-item :label="t('aiApplications.answerScope')"><el-input v-model="assistant.answer_scope" type="textarea" :rows="4" :placeholder="t('aiApplications.answerScopePlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.rules')"><el-input v-model="assistant.operating_rules" type="textarea" :rows="4" /></el-form-item><el-form-item :label="t('aiApplications.responseStyle')"><el-input v-model="assistant.response_style" /></el-form-item><el-form-item :label="t('aiApplications.state')"><el-switch v-model="assistant.state" active-value="enabled" inactive-value="disabled" /></el-form-item></el-form></el-card>
      <el-card class="application-detail-card"><template #header><strong>{{ t('aiApplications.faq.title') }}</strong></template><div v-for="faq in faqs" :key="faq.id" class="faq-row"><div><strong>{{ faq.question }}</strong><p>{{ faq.answer_markdown }}</p></div><el-button text type="danger" @click="removeFAQ(faq)">{{ t('common.delete') }}</el-button></div><el-form class="faq-create-form" label-position="top"><el-form-item :label="t('aiApplications.faq.question')"><el-input v-model="faqDraft.question" /></el-form-item><el-form-item :label="t('aiApplications.faq.answer')"><el-input v-model="faqDraft.answer_markdown" type="textarea" :rows="3" /></el-form-item><el-button type="primary" @click="addFAQ">{{ t('aiApplications.faq.add') }}</el-button></el-form></el-card>
      <el-card class="application-detail-card"><template #header><strong>{{ t('aiApplications.knowledge.title') }}</strong></template><p class="application-card-copy">{{ t('aiApplications.knowledge.hint') }}</p><el-form-item :label="t('aiApplications.knowledge.selected')"><el-select v-model="assistant.knowledge_base_ids" multiple collapse-tags :placeholder="t('aiApplications.knowledge.selectPlaceholder')"><el-option v-for="base in knowledgeBases" :key="base.id" :label="base.name" :value="base.id" /></el-select></el-form-item><el-button text type="primary" @click="router.push('/ai-apps/knowledge-bases')">{{ t('aiApplications.knowledge.manage') }}</el-button></el-card>
      <el-card class="application-detail-card"><template #header><strong>{{ t('aiApplications.share.title') }}</strong></template><el-form label-position="top"><el-form-item :label="t('aiApplications.share.enabled')"><el-switch v-model="assistant.share.enabled" /></el-form-item><el-form-item :label="t('aiApplications.share.width')"><el-input v-model="assistant.share.width" /></el-form-item><el-form-item :label="t('aiApplications.share.height')"><el-input-number v-model="assistant.share.height" :min="400" :max="1600" /></el-form-item><el-form-item :label="t('aiApplications.share.allowedOrigins')"><el-input v-model="allowedOrigins" type="textarea" :rows="3" :placeholder="t('aiApplications.share.allowedOriginsPlaceholder')" /></el-form-item><el-form-item :label="t('aiApplications.share.dailyLimit')"><el-input-number v-model="assistant.share.daily_call_limit" :min="0" :max="100000" /></el-form-item><el-form-item :label="t('aiApplications.share.freeText')"><el-switch v-model="assistant.share.free_text_enabled" /></el-form-item><el-button :disabled="!assistant.share.enabled" @click="regenerateToken">{{ t('aiApplications.share.regenerate') }}</el-button><el-input v-if="shareToken" class="share-snippet" :model-value="embedSnippet" readonly type="textarea" :rows="3" /></el-form></el-card>
    </template>
  </section>
</template>
