<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ArrowUp, RotateCcw, Square } from "@lucide/vue";
import { ApiError } from "../api/client";
import type { PublicAssistantApi, PublicAssistantProfile } from "../api/publicAssistant";
import AssistantConversationThread, { type AssistantChatTurn } from "../components/AssistantConversationThread.vue";

const props = defineProps<{ api: PublicAssistantApi; width: string; height: number; embedded?: boolean }>();
const { t } = useI18n();
const profile = ref<PublicAssistantProfile>();
const turns = ref<AssistantChatTurn[]>([]);
const draft = ref("");
const loading = ref(true);
const busy = ref(false);
const error = ref("");
const avatarFailed = ref(false);
const thread = ref<InstanceType<typeof AssistantConversationThread>>();
const dimensions = computed(() => ({ width: props.embedded ? "100%" : props.width, height: props.embedded ? "100dvh" : `${props.height}px` }));
let conversationID = "";
let controller: AbortController | undefined;
const loadController = new AbortController();
let sequence = 0;

async function scrollToAnswer() {
  await nextTick();
  const element = thread.value?.$el as HTMLElement | undefined;
  if (element) element.scrollTop = element.scrollHeight;
}
async function send(question = draft.value, faqID?: string) {
  const text = question.trim();
  if (!text || busy.value || !profile.value || (!faqID && !profile.value.free_text_enabled)) return;
  error.value = "";
  draft.value = "";
  busy.value = true;
  const turn = reactive<AssistantChatTurn>({ id: `visitor-turn-${++sequence}`, question: text, answer: "", state: "generating" });
  turns.value.push(turn);
  const request = new AbortController();
  controller = request;
  await scrollToAnswer();
  try {
    await props.api.stream(text, conversationID, faqID, (event) => {
      if (request.signal.aborted) return;
      if (event.type === "thinking") conversationID = event.conversation_id;
      if (event.type === "delta") turn.answer += event.text;
      if (event.type === "done") {
        turn.answer = event.answer;
        turn.state = event.state;
        turn.failure_code = event.failure_code;
        if (event.conversation_id) conversationID = event.conversation_id;
      }
      if (event.type === "error") turn.failure_code = event.code;
      void scrollToAnswer();
    }, request.signal);
  } catch (cause) {
    if (request.signal.aborted) turn.state = "cancelled";
    else {
      turn.state = "failed";
      error.value = cause instanceof ApiError && cause.status === 429 ? t("aiApplications.share.callLimited") : t("aiApplications.chat.sendFailed");
    }
  } finally {
    if (controller === request) { controller = undefined; busy.value = false; }
  }
}
function stop() {
  controller?.abort();
  const turn = turns.value.at(-1);
  if (turn?.state === "generating") turn.state = "cancelled";
}
function clearConversation() {
  stop();
  controller = undefined;
  busy.value = false;
  conversationID = "";
  turns.value = [];
  draft.value = "";
  error.value = "";
}
onMounted(async () => {
  try { profile.value = await props.api.profile(loadController.signal); }
  catch { error.value = t("aiApplications.share.unavailable"); }
  finally { loading.value = false; }
});
onBeforeUnmount(() => { controller?.abort(); loadController.abort(); });
</script>

<template>
  <section class="assistant-conversation-page assistant-embed-page" :style="dimensions" v-loading="loading">
    <header class="assistant-conversation-header">
      <h2>{{ profile?.name }}</h2>
      <el-button :icon="RotateCcw" :disabled="!profile" @click="clearConversation">{{ t('aiApplications.chat.new') }}</el-button>
    </header>
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
    <AssistantConversationThread ref="thread" :name="profile?.name || ''" :welcome="profile?.introduction || ''" :avatar-url="avatarFailed ? '' : api.iconURL" :faqs="profile?.faqs || []" :turns="turns" :busy="busy" @faq="send" @avatar-error="avatarFailed = true" />
    <div class="assistant-conversation-composer">
      <div class="assistant-conversation-composer-shell">
        <el-input v-model="draft" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" :maxlength="4000" :disabled="busy || !profile?.free_text_enabled" :placeholder="profile && !profile.free_text_enabled ? t('aiApplications.share.faqOnly') : t('aiApplications.chat.placeholder')" :aria-label="t('aiApplications.chat.placeholder')" @keydown.enter.exact.prevent="send()" />
        <div class="assistant-conversation-composer-actions">
          <el-button v-if="busy" type="danger" class="assistant-conversation-send" :icon="Square" @click="stop">{{ t('aiApplications.chat.stop') }}</el-button>
          <el-button v-else type="primary" class="assistant-conversation-send" :icon="ArrowUp" :disabled="!draft.trim() || !profile?.free_text_enabled" @click="send()">{{ t('aiApplications.chat.send') }}</el-button>
        </div>
      </div>
    </div>
  </section>
</template>
