<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import { Box } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import { renderMarkdown } from "../markdown";
import { displayArtifactNames } from "../artifactDisplay";
import ArtifactDisclosure from "./ArtifactDisclosure.vue";
import ConversationAttachments from "./ConversationAttachments.vue";
import CreditConsumption from "./CreditConsumption.vue";
import { runtimeEngineDisplayName, type Artifact, type ExpertStage } from "../api/client";
import type { ConversationActivityKind, ConversationMessage } from "../conversationThread";

const props = defineProps<{
  messages: ConversationMessage[];
  loadAttachment: (id: string) => Promise<Blob>;
}>();
const emit = defineEmits<{
  downloadArtifact: [artifact: Artifact];
  retry: [messageID: string];
  resourceAction: [messageID: string, decision: "confirm" | "cancel"];
  attachmentError: [];
  copyError: [];
}>();
const { t, locale } = useI18n();
const pendingStates = new Set(["queued", "running", "generating", "waiting_for_user"]);

const visibleStages = (message: ConversationMessage) => {
  const messageText = (message.content || message.error || "").replace(/\r\n/g, "\n").trim();
  return (message.stages ?? []).filter((stage) => {
    if (message.state === "cancelled" && !stage.final_text) return false;
    const stageText = (stage.final_text || stage.error || "").replace(/\r\n/g, "\n").trim();
    return !stageText || stageText !== messageText;
  });
};

const copiedID = ref("");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

function isPending(message: ConversationMessage) {
  return message.pending ?? pendingStates.has(message.state);
}
function stateLabel(message: ConversationMessage) {
  return message.stateLabel || message.state;
}
function terminalStateLabel(message: ConversationMessage) {
  if (message.state === "completed" || message.state === "succeeded") return t("common.success");
  if (message.state === "failed") return t("common.failed");
  if (message.state === "cancelled") return t("common.cancelled");
  return stateLabel(message);
}
function stageStateLabel(state: ExpertStage["state"]) {
  return state === "succeeded" ? t("common.success") : state === "failed" ? t("common.failed") : state === "cancelled" ? t("common.cancelled") : state === "running" ? t("common.running") : state;
}
function stageCopyKey(messageID: string, position: number) {
  return `${messageID}:${position}`;
}
function isCopied(id: string) {
  return copiedID.value === id;
}
async function copy(value: string, id: string) {
  try {
    await navigator.clipboard.writeText(value);
    copiedID.value = id;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { copiedID.value = ""; }, 1600);
  } catch {
    emit("copyError");
  }
}
function messageAriaLabel(message: ConversationMessage) {
  return message.role === "user" ? t("sessions.copyQuestion") : t("sessions.copyAnswer");
}
function messageTime(timestamp: string) {
  return new Date(timestamp).toLocaleTimeString(locale.value as SupportedLocale, { hour: "2-digit", minute: "2-digit" });
}
function activityKindLabel(kind?: ConversationActivityKind) {
  return t(`sessions.executionEvidence.kind.${kind ?? "activity"}`);
}
function hasExecutionEvidence(message: ConversationMessage) {
  return Boolean(message.activities?.length || message.stages?.length || message.artifacts?.length);
}
function executionEvidenceSummary(message: ConversationMessage) {
  const groups = message.activities ?? [];
  const toolCount = message.executionEvidenceCounts?.toolCalls ?? groups.reduce((total, group) => total + (group.toolCallCount ?? (group.kind === "tool" ? 1 : 0)), 0);
  const fileCount = message.executionEvidenceCounts?.fileChanges ?? groups.reduce((total, group) => total + (group.fileChangeCount ?? (group.kind === "file" ? 1 : 0)), 0);
  const stageCount = message.stages?.length ?? 0;
  const artifactCount = message.artifacts?.length ?? 0;
  const parts: string[] = [];
  if (toolCount) parts.push(t("sessions.executionEvidence.tools", { count: toolCount }));
  if (fileCount) parts.push(t("sessions.executionEvidence.files", { count: fileCount }));
  if (stageCount) parts.push(t("sessions.executionEvidence.stages", { count: stageCount }));
  if (artifactCount) parts.push(t("sessions.executionEvidence.artifacts", { count: artifactCount }));
  return parts.length ? parts.join(" · ") : t("sessions.executionEvidence.noExternal");
}
onBeforeUnmount(() => { if (copiedTimer) clearTimeout(copiedTimer); });
</script>

<template>
  <div class="conversation-thread">
    <div v-for="message in props.messages" :key="message.id" class="message" :class="message.role">
      <span v-if="message.role === 'assistant'" class="agent-avatar" aria-hidden="true">AI</span>
      <div class="message-content">
        <div v-if="message.role === 'assistant'" class="message-identity">
          <strong>{{ message.meta?.label || 'Agent Workspace' }}</strong><span>Agent</span><span v-if="!isPending(message)" class="message-terminal-state" :class="`is-${message.state}`">{{ terminalStateLabel(message) }}</span><time :datetime="message.timestamp">{{ messageTime(message.timestamp) }}</time><small v-if="message.elapsedMs">{{ t('sessions.elapsed', { value: formatDuration(message.elapsedMs, locale as SupportedLocale) }) }}</small>
        </div>
        <div v-if="message.role === 'assistant' && isPending(message) && !message.finalizing" class="thinking-state">
          <span class="thinking-dots" aria-hidden="true"><i></i><i></i><i></i></span>
          <strong>{{ message.state === 'waiting_for_user' ? t('common.waitingForUser') : (message.progressTitle || t('sessions.thinking')) }}</strong>
          <small>{{ message.progressDetail || t('sessions.progress.thinking') }}</small>
        </div>
        <div v-else-if="message.role === 'assistant' && isPending(message) && message.finalizing" class="finalizing-state">{{ message.progressTitle || t('sessions.progress.finalizing') }}</div>

        <div v-if="message.role === 'assistant' && hasExecutionEvidence(message)" class="runtime-activity" aria-live="polite">
          <div v-if="message.currentActivity" class="runtime-activity-current">
            <span class="activity-pulse active"></span><strong>{{ message.currentActivity.label }}</strong><small v-if="message.currentActivity.detail">{{ message.currentActivity.detail }}</small>
          </div>
          <details class="runtime-activity-history">
            <summary><strong>{{ t('sessions.executionEvidence.title') }}</strong><small>{{ executionEvidenceSummary(message) }}</small></summary>
            <div class="activity-summary-list">
              <details v-for="group in message.activities" :key="`${message.id}-${group.id}`" class="activity-summary-group" :class="`is-${group.state || 'completed'}`">
                <summary><span class="activity-summary-mark" aria-hidden="true"></span><span class="activity-kind">{{ activityKindLabel(group.kind) }}</span><strong>{{ group.label }}</strong><small class="activity-state">{{ t(`sessions.executionEvidence.${group.state === 'running' ? 'running' : 'completed'}`) }}</small></summary>
                <ol class="activity-detail-list">
                  <li v-for="activity in group.items" :key="`${message.id}-${group.id}-${activity.id}`"><span></span><div><strong>{{ activity.label }}</strong><small v-if="activity.detail">{{ activity.detail }}</small></div></li>
                </ol>
              </details>
            </div>
          </details>
        </div>

        <div v-if="message.content && message.role === 'assistant'" class="markdown-body" :class="{ streaming: message.streaming }" v-html="renderMarkdown(displayArtifactNames(message.content, message.artifacts))"></div>
        <p v-else-if="message.content">{{ message.content }}</p>
        <section v-if="message.role === 'assistant' && message.state === 'failed'" class="failure-card" role="alert">
          <header>{{ t('sessions.failureTitle') }}</header>
          <dl><div><dt>{{ t('sessions.failureLocation') }}</dt><dd>{{ message.progressDetail || t('sessions.failureResponse') }}</dd></div><div><dt>{{ t('sessions.failureReason') }}</dt><dd>{{ message.error || stateLabel(message) }}</dd></div><div><dt>{{ t('sessions.failureCompleted') }}</dt><dd>{{ message.content ? t('sessions.failurePartialKept') : t('sessions.failureNoResult') }}</dd></div></dl>
          <footer v-if="message.retryable"><el-button type="primary" @click="emit('retry', message.id)">{{ t('sessions.retryStep') }}</el-button></footer>
        </section>
        <p v-else-if="message.role !== 'assistant' && message.state !== 'cancelled' && (message.error || message.state === 'failed')">{{ message.error || stateLabel(message) }}</p>
        <div v-if="message.role === 'user' && message.skills?.length" class="message-skill-badges" :aria-label="t('sessions.usedSkills')">
          <span v-for="skill in message.skills" :key="skill.id" class="message-skill-badge"><Box :size="14" aria-hidden="true" />{{ skill.name }}</span>
        </div>
        <ArtifactDisclosure v-if="message.role === 'assistant' && message.artifacts?.length" :artifacts="message.artifacts" @download="emit('downloadArtifact', $event)" />
        <section v-if="message.role === 'assistant' && message.resourceAction" class="resource-action-card" :class="`resource-action-${message.resourceAction.state}`" aria-live="polite">
          <div class="resource-action-heading"><strong>{{ t(`sessions.${message.resourceAction.kind === 'skill' ? 'resourceActionSkill' : message.resourceAction.kind === 'connector' ? 'resourceActionConnector' : 'resourceActionExpert'}`) }}</strong><span>{{ message.resourceAction.name }}</span></div>
          <p v-if="message.resourceAction.description">{{ message.resourceAction.description }}</p>
          <div v-if="message.resourceAction.state === 'pending'" class="resource-action-actions"><el-button type="primary" @click="emit('resourceAction', message.id, 'confirm')">{{ t('sessions.resourceActionConfirm') }}</el-button><el-button @click="emit('resourceAction', message.id, 'cancel')">{{ t('common.cancel') }}</el-button></div>
          <small v-else-if="message.resourceAction.state === 'confirmed'">{{ t('sessions.resourceActionConfirmed') }}</small>
          <small v-else-if="message.resourceAction.state === 'cancelled'">{{ t('sessions.resourceActionCancelled') }}</small>
          <small v-else-if="message.resourceAction.error" class="resource-action-error">{{ message.resourceAction.error }}</small>
          <small v-else>{{ t('sessions.resourceActionExpired') }}</small>
        </section>
        <ConversationAttachments v-if="message.attachments?.length" :attachments="message.attachments" :load-attachment="props.loadAttachment" @error="emit('attachmentError')" />
        <p v-if="message.state === 'cancelled'" class="cancelled-response">{{ t('sessions.cancelled') }}</p>

        <div v-if="message.role === 'assistant' && visibleStages(message).length" class="expert-stage-list">
          <details v-for="stage in visibleStages(message)" :key="`${message.id}-${stage.position}-${stage.expert_id}`">
            <summary><span>{{ stage.position }}/{{ stage.total || message.stages?.length }} · {{ stage.expert_name }}</span><small>{{ stageStateLabel(stage.state) }}<template v-if="stage.provider_model_name"> · {{ stage.provider_model_name }}</template><template v-if="stage.runtime_engine"> · {{ runtimeEngineDisplayName(stage.runtime_engine) }}</template><template v-if="stage.elapsed_ms"> · {{ formatDuration(stage.elapsed_ms, locale as SupportedLocale) }}</template></small></summary>
            <div v-if="stage.final_text" class="markdown-body" v-html="renderMarkdown(displayArtifactNames(stage.final_text, message.artifacts))"></div>
            <p v-else-if="stage.error">{{ stage.error }}</p>
            <button v-if="stage.final_text" type="button" class="stage-copy" @click="copy(stage.final_text, stageCopyKey(message.id, stage.position))">{{ isCopied(stageCopyKey(message.id, stage.position)) ? t('common.copied') : t('common.copy') }}</button>
          </details>
        </div>
        <CreditConsumption v-if="message.role === 'assistant'" :value="message.creditConsumption" />
        <div class="message-actions">
          <small v-if="message.role === 'user'" class="message-meta">{{ messageTime(message.timestamp) }}</small>
          <button v-if="message.copyText || message.content" type="button" class="message-copy" :class="{ copied: isCopied(`message:${message.id}`) }" :aria-label="messageAriaLabel(message)" @click="copy(message.copyText || message.content, `message:${message.id}`)"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="7" y="7" width="9" height="9" rx="2"/><path d="M13 7V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/></svg><span>{{ isCopied(`message:${message.id}`) ? t('common.copied') : t('common.copy') }}</span></button>
        </div>
      </div>
    </div>
  </div>
</template>
