<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { CircleAlert, CircleHelp, MessageCircle } from "@lucide/vue";
import type { AssistantTurn } from "../api/client";
import { renderMarkdown } from "../markdown";

export type AssistantChatTurn = Pick<AssistantTurn, "id" | "question" | "answer" | "state" | "failure_code">;
const props = defineProps<{ name: string; welcome: string; avatarUrl?: string; faqs: { id: string; question: string }[]; turns: AssistantChatTurn[]; busy: boolean }>();
const emit = defineEmits<{ faq: [question: string, id: string]; "avatar-error": [] }>();
const { t } = useI18n();
const scrollContainer = ref<HTMLElement>();
let followingLatest = true;
let resizeObserver: ResizeObserver | undefined;
async function revealLatest() {
  await nextTick();
  if (followingLatest && scrollContainer.value) scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight;
}
function trackScroll() {
  const element = scrollContainer.value;
  if (element) followingLatest = element.scrollHeight - element.scrollTop - element.clientHeight <= 24;
}
watch(() => props.turns.map(turn => [turn.id, turn.question, turn.answer, turn.state]), (turns, previous) => {
  if (turns.length !== previous?.length || turns.at(-1)?.[0] !== previous?.at(-1)?.[0]) followingLatest = true;
  void revealLatest();
}, { flush: "post", immediate: true });
onMounted(() => {
  if (typeof ResizeObserver !== "undefined" && scrollContainer.value) {
    resizeObserver = new ResizeObserver(() => { void revealLatest(); });
    resizeObserver.observe(scrollContainer.value);
  }
});
onBeforeUnmount(() => resizeObserver?.disconnect());
function failureMessage(code = "") {
  const keys: Record<string, string> = {
    model_authentication: "failureAuthentication", model_rate_limited: "failureRateLimited",
    model_unavailable: "failureUnavailable", model_configuration: "failureConfiguration",
    model_response_invalid: "failureInvalidResponse",
  };
  return t(`aiApplications.chat.${keys[code] ?? "failed"}`);
}
</script>

<template>
    <div ref="scrollContainer" class="assistant-conversation-thread" aria-live="polite" @scroll="trackScroll" @load.capture="revealLatest">
      <div v-if="name" class="assistant-conversation-message assistant-conversation-message--assistant assistant-conversation-message--welcome">
        <div class="assistant-conversation-avatar">
          <img v-if="avatarUrl" :src="avatarUrl" :alt="name" @error="emit('avatar-error')" />
          <MessageCircle v-else :size="20" aria-hidden="true" />
        </div>
        <div class="assistant-conversation-bubble">{{ welcome || t('aiApplications.chat.defaultWelcome') }}</div>
      </div>
      <div v-if="faqs.length" class="assistant-conversation-faqs">
        <span class="assistant-conversation-faq-label"><CircleHelp :size="14" aria-hidden="true" />{{ t('aiApplications.faq.title') }}</span>
        <button v-for="faq in faqs" :key="faq.id" type="button" class="assistant-conversation-faq" :disabled="busy" @click="emit('faq', faq.question, faq.id)">{{ faq.question }}</button>
      </div>
      <div v-for="turn in turns" :key="turn.id" class="assistant-conversation-turn">
        <div class="assistant-conversation-message assistant-conversation-message--user">
          <div class="assistant-conversation-bubble">{{ turn.question }}</div>
        </div>
        <div v-if="turn.answer || turn.state === 'generating' || turn.state === 'cancelled' || turn.state === 'failed'" class="assistant-conversation-message assistant-conversation-message--assistant" :class="{ 'assistant-conversation-message--failed': turn.state === 'failed' }">
          <div class="assistant-conversation-avatar">
            <img v-if="avatarUrl" :src="avatarUrl" :alt="name || ''" @error="emit('avatar-error')" />
            <MessageCircle v-else :size="20" aria-hidden="true" />
          </div>
          <div class="assistant-conversation-bubble">
            <div v-if="turn.state === 'failed'" class="assistant-conversation-failure" role="alert"><CircleAlert :size="17" aria-hidden="true" />{{ failureMessage(turn.failure_code) }}</div>
            <div v-if="turn.answer" class="markdown-body" v-html="renderMarkdown(turn.answer)" />
            <div v-else-if="turn.state !== 'failed'" class="assistant-conversation-thinking">{{ t(`aiApplications.chat.${turn.state === 'generating' ? 'thinking' : turn.state}`) }}</div>
          </div>
        </div>
      </div>
    </div>
</template>
