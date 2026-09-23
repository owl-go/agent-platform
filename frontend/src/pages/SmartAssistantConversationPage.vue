<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ArrowLeft, MessageCircle, RotateCcw, Square } from "@lucide/vue";
import { ApiError, platformApiKey, type AssistantConversation, type AssistantTurn, type SmartAssistantFAQ } from "../api/client";
import { renderMarkdown } from "../markdown";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const assistantID = computed(() => String(route.params.assistantId));
const conversationID = computed(() => String(route.params.conversationId));
const conversation = ref<AssistantConversation>();
const turns = ref<AssistantTurn[]>([]);
const faqs = ref<SmartAssistantFAQ[]>([]);
const history = ref<AssistantConversation[]>([]);
const draft = ref("");
const loading = ref(false);
const busy = ref(false);
const creating = ref(false);
const error = ref("");
const activeTurnID = ref("");
let controller: AbortController | undefined;

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [detail, conversations] = await Promise.all([
      api.getAssistantConversation(assistantID.value, conversationID.value),
      api.listAssistantConversations(assistantID.value),
    ]);
    conversation.value = detail.conversation;
    turns.value = detail.turns;
    faqs.value = detail.faqs;
    history.value = conversations;
    const running = detail.turns.find((turn) => turn.state === "generating");
    activeTurnID.value = running?.id ?? "";
    busy.value = !!running;
  } catch { error.value = t("aiApplications.chat.loadFailed"); }
  finally { loading.value = false; }
}

async function send(question = draft.value, faqID?: string) {
  const text = question.trim();
  if (busy.value || !text || !conversation.value) return;
  error.value = "";
  busy.value = true;
  activeTurnID.value = "";
  draft.value = "";
  const pending: AssistantTurn = { id: `pending-${Date.now()}`, conversation_id: conversationID.value, turn_number: turns.value.length + 1, question: text, answer: "", source: "", state: "generating", input_tokens: 0, output_tokens: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() };
  turns.value.push(pending);
  controller = new AbortController();
  try {
    await api.streamAssistantTurn(assistantID.value, conversationID.value, text, faqID, (event) => {
      if (event.type === "thinking") { pending.id = event.turn_id; activeTurnID.value = event.turn_id; }
      if (event.type === "delta") { pending.answer += event.text; }
      if (event.type === "done") { Object.assign(pending, event.turn); }
      if (event.type === "error") { error.value = event.message; }
    }, controller.signal);
  } catch (cause) {
    if (!(cause instanceof DOMException && cause.name === "AbortError")) error.value = t("aiApplications.chat.sendFailed");
  } finally {
    controller = undefined;
    await load();
  }
}

async function stop() {
  if (!busy.value) return;
  if (activeTurnID.value) {
    try { await api.cancelAssistantTurn(assistantID.value, conversationID.value, activeTurnID.value); }
    catch (cause) { if (!(cause instanceof ApiError && cause.status === 404)) error.value = t("aiApplications.chat.stopFailed"); }
  }
  controller?.abort();
  for (let attempt = 0; attempt < 10; attempt += 1) {
    await load();
    if (!busy.value) break;
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

async function clearConversation() {
  if (creating.value) return;
  if (busy.value) await stop();
  creating.value = true;
  try {
    const created = await api.createAssistantConversation(assistantID.value);
    await router.push(`/ai-apps/assistants/${encodeURIComponent(assistantID.value)}/conversations/${encodeURIComponent(created.id)}`);
  } catch { error.value = t("aiApplications.chat.newFailed"); }
  finally { creating.value = false; }
}

watch(() => route.params.conversationId, () => { controller?.abort(); void load(); }, { immediate: true });
onBeforeUnmount(() => controller?.abort());
</script>

<template>
  <section class="assistant-conversation-page">
    <header class="assistant-conversation-header">
      <el-button text :icon="ArrowLeft" @click="router.push('/ai-apps/assistants')">{{ t('common.back') }}</el-button>
      <h2>{{ conversation?.assistant_name || t('aiApplications.assistants.title') }}</h2>
      <div class="assistant-conversation-actions">
        <el-select v-if="history.length > 1" :model-value="conversationID" class="assistant-conversation-history" :aria-label="t('aiApplications.chat.history')" @change="router.push(`/ai-apps/assistants/${assistantID}/conversations/${$event}`)">
          <el-option v-for="item in history" :key="item.id" :label="new Date(item.created_at).toLocaleString()" :value="item.id" />
        </el-select>
        <el-button :icon="RotateCcw" :loading="creating" @click="clearConversation">{{ t('aiApplications.chat.new') }}</el-button>
      </div>
    </header>
    <el-alert v-if="error" :title="error" type="error" show-icon closable @close="error = ''" />
    <div v-loading="loading" class="assistant-conversation-thread" aria-live="polite">
      <div v-if="conversation" class="assistant-conversation-welcome">
        <MessageCircle :size="32" />
        <p>{{ conversation.welcome || t('aiApplications.chat.defaultWelcome') }}</p>
      </div>
      <div v-if="faqs.length" class="assistant-conversation-faqs">
        <button v-for="faq in faqs" :key="faq.id" type="button" :disabled="busy" @click="send(faq.question, faq.id)">{{ faq.question }}</button>
      </div>
      <div v-for="turn in turns" :key="turn.id" class="assistant-conversation-turn">
        <div class="assistant-conversation-question">{{ turn.question }}</div>
        <div v-if="turn.answer" class="assistant-conversation-answer markdown-body" v-html="renderMarkdown(turn.answer)" />
        <div v-else-if="turn.state === 'generating'" class="assistant-conversation-thinking">{{ t('aiApplications.chat.thinking') }}</div>
        <div v-else-if="turn.state === 'cancelled'" class="assistant-conversation-thinking">{{ t('aiApplications.chat.cancelled') }}</div>
        <div v-else-if="turn.state === 'failed'" class="assistant-conversation-thinking">{{ t('aiApplications.chat.failed') }}</div>
      </div>
    </div>
    <div class="assistant-conversation-composer">
      <el-input v-model="draft" type="textarea" :rows="2" :maxlength="4000" :disabled="busy || loading" :placeholder="t('aiApplications.chat.placeholder')" @keydown.enter.exact.prevent="send()" />
      <el-button v-if="busy" type="danger" :icon="Square" @click="stop">{{ t('aiApplications.chat.stop') }}</el-button>
      <el-button v-else type="primary" :disabled="!draft.trim() || loading" @click="send()">{{ t('aiApplications.chat.send') }}</el-button>
    </div>
  </section>
</template>
