<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import { Box, PanelRightOpen, Workflow } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import { renderInlineMarkdown, renderMarkdown } from "../markdown";
import { displayArtifactNames } from "../artifactDisplay";
import ArtifactDisclosure from "./ArtifactDisclosure.vue";
import ConversationAttachments from "./ConversationAttachments.vue";
import CreditConsumption from "./CreditConsumption.vue";
import type { Artifact, Evidence, ExpertStage } from "../api/client";
import type { ConversationActivityKind, ConversationMessage } from "../conversationThread";
import { hasTaskWorkspaceContent } from "../taskWorkspace";

const props = defineProps<{
  messages: ConversationMessage[];
  loadAttachment: (id: string) => Promise<Blob>;
  selectedTaskId?: string;
  planActionsInPanelId?: string;
}>();
const emit = defineEmits<{
  downloadArtifact: [artifact: Artifact];
  openEvidence: [evidence: Evidence];
  retry: [messageID: string];
  resourceAction: [messageID: string, decision: "confirm" | "cancel"];
  planDecision: [messageID: string, decision: "start" | "direct" | "cancel"];
  editPlan: [messageID: string];
  attachmentError: [];
  copyError: [];
  selectTask: [messageID: string];
  saveWorkflow: [messageID: string];
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
  return Boolean(message.activities?.length || message.stages?.length || message.artifacts?.length || message.evidence?.length);
}
function executionEvidenceSummary(message: ConversationMessage) {
  const groups = message.activities ?? [];
  const toolCount = message.executionEvidenceCounts?.toolCalls ?? groups.reduce((total, group) => total + (group.toolCallCount ?? (group.kind === "tool" ? 1 : 0)), 0);
  const fileCount = message.executionEvidenceCounts?.fileChanges ?? groups.reduce((total, group) => total + (group.fileChangeCount ?? (group.kind === "file" ? 1 : 0)), 0);
  const stageCount = message.stages?.length ?? 0;
  const artifactCount = message.artifacts?.length ?? 0;
  const evidenceCount = message.evidence?.filter((item) => item.state === "succeeded").length ?? 0;
  const parts: string[] = [];
  if (toolCount) parts.push(t("sessions.executionEvidence.tools", { count: toolCount }));
  if (fileCount) parts.push(t("sessions.executionEvidence.files", { count: fileCount }));
  if (stageCount) parts.push(t("sessions.executionEvidence.stages", { count: stageCount }));
  if (artifactCount) parts.push(t("sessions.executionEvidence.artifacts", { count: artifactCount }));
  if (evidenceCount) parts.push(t("sessions.executionEvidence.sources", { count: evidenceCount }));
  return parts.length ? parts.join(" · ") : t("sessions.executionEvidence.noExternal");
}
function evidenceKindLabel(evidence: Evidence) {
  return t(`sessions.executionEvidence.sourceKind.${evidence.kind}`);
}
function evidenceStateLabel(evidence: Evidence) {
  return t(`sessions.executionEvidence.sourceState.${evidence.state}`);
}
function evidenceActionLabel(evidence: Evidence) {
  const known: Record<string, string> = {
    "retrieved indexed source": "retrieved",
    "no relevant indexed source": "noMatch",
    "retrieval failed": "retrievalFailed",
    "retrieval unavailable": "retrievalUnavailable",
    "ready index unavailable": "indexUnavailable",
    "not invoked": "notInvoked",
  };
  const key = known[evidence.action];
  return key ? t(`sessions.executionEvidence.sourceAction.${key}`) : evidence.action;
}
function canOpenEvidence(evidence: Evidence) {
  return evidence.kind === "knowledge" && evidence.state === "succeeded" && Boolean(evidence.container_id && evidence.citation?.revision_id);
}
function planStateLabel(plan: NonNullable<ConversationMessage["executionPlan"]>) {
  if (plan.state === "approved" && !plan.reasons?.includes("user_requested")) return t("sessions.executionPlan.automaticStart");
  return t(`sessions.executionPlan.states.${plan.state}`);
}
function planStepStateLabel(state: string) { return t(`sessions.executionPlan.stepStates.${state}`); }
function planCodeLabel(group: "reasons" | "sideEffects", code: string) { return t(`sessions.executionPlan.${group}.${code}`); }
function planCredits(hundredths: number | undefined) {
  const amount = Number(hundredths ?? 0);
  return Number.isFinite(amount) && amount >= 0 ? (amount / 100).toFixed(2) : "—";
}
onBeforeUnmount(() => { if (copiedTimer) clearTimeout(copiedTimer); });
</script>

<template>
  <div class="conversation-thread">
    <div v-for="message in props.messages" :key="message.id" class="message" :class="[message.role, { 'is-task-selected': message.id === selectedTaskId }]">
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

        <section v-if="message.role === 'assistant' && message.executionPlan" class="execution-plan-card" :class="`is-${message.executionPlan.state}`" aria-live="polite">
          <header><div><small>{{ t('sessions.executionPlan.title') }}</small><strong>{{ message.executionPlan.objective }}</strong></div><span>{{ planStateLabel(message.executionPlan) }}</span></header>
          <p v-if="message.executionPlan.generator === 'model_failed'" class="muted" role="status">{{ t('sessions.executionPlan.modelFailedHint') }}</p>
          <ol class="execution-plan-steps">
            <li v-for="step in message.executionPlan.steps ?? []" :key="step.id" :class="`is-${step.state}`"><span>{{ step.position }}</span><div><strong>{{ step.label || t(`taskWorkspace.stepKinds.${step.kind}`) }}</strong><small>{{ planStepStateLabel(step.state) }}</small></div></li>
          </ol>
          <dl class="execution-plan-meta">
            <div v-if="message.executionPlan.resources?.length"><dt>{{ t('sessions.executionPlan.resources') }}</dt><dd>{{ message.executionPlan.resources.map((item) => item.name).join(' · ') }}</dd></div>
            <div v-if="message.executionPlan.side_effects?.length"><dt>{{ t('sessions.executionPlan.sideEffectsTitle') }}</dt><dd>{{ message.executionPlan.side_effects.map((item) => planCodeLabel('sideEffects', item)).join(' · ') }}</dd></div>
            <div><dt>{{ t('sessions.executionPlan.estimate') }}</dt><dd>{{ t('sessions.executionPlan.estimateValue', { calls: message.executionPlan.estimated_model_calls ?? 0, credits: planCredits(message.executionPlan.estimated_credit_hundredths) }) }}</dd></div>
            <div><dt>{{ t('sessions.executionPlan.generationCost') }}</dt><dd>{{ t(message.executionPlan.generator === 'model' || message.executionPlan.generator === 'model_failed' ? 'sessions.executionPlan.modelGenerationCostValue' : 'sessions.executionPlan.generationCostValue', { credits: planCredits(message.executionPlan.generation_credit_hundredths) }) }}</dd></div>
          </dl>
          <small v-if="message.executionPlan.reasons?.length" class="execution-plan-reasons">{{ message.executionPlan.reasons.map((item) => planCodeLabel('reasons', item)).join(' · ') }}</small>
          <footer v-if="message.executionPlan.state === 'pending' && message.id !== props.planActionsInPanelId">
            <el-button type="primary" @click="emit('planDecision', message.id, 'start')">{{ t('sessions.executionPlan.start') }}</el-button>
            <el-button @click="emit('editPlan', message.id)">{{ t('sessions.executionPlan.edit') }}</el-button>
            <el-button v-if="!message.executionPlan.side_effects?.length" @click="emit('planDecision', message.id, 'direct')">{{ t('sessions.executionPlan.direct') }}</el-button>
            <el-button text @click="emit('planDecision', message.id, 'cancel')">{{ t('common.cancel') }}</el-button>
          </footer>
        </section>

        <div v-if="message.role === 'assistant' && hasExecutionEvidence(message)" class="runtime-activity" aria-live="polite">
          <div v-if="message.currentActivity" class="runtime-activity-current">
            <span class="activity-pulse active" aria-hidden="true"></span><strong v-html="renderInlineMarkdown(message.currentActivity.label)"></strong><small v-if="message.currentActivity.detail && message.currentActivity.detail !== message.currentActivity.label">{{ message.currentActivity.detail }}</small>
          </div>
          <details class="runtime-activity-history">
            <summary><strong>{{ t('sessions.executionEvidence.title') }}</strong><small>{{ executionEvidenceSummary(message) }}</small></summary>
            <div class="activity-summary-list">
              <details v-for="group in message.activities" :key="`${message.id}-${group.id}`" class="activity-summary-group" :class="`is-${group.state || 'completed'}`">
                <summary><span class="activity-summary-mark" aria-hidden="true"></span><span class="activity-kind">{{ activityKindLabel(group.kind) }}</span><strong v-html="renderInlineMarkdown(group.label)"></strong><small class="activity-state">{{ t(`sessions.executionEvidence.${group.state === 'running' ? 'running' : 'completed'}`) }}</small></summary>
                <ol class="activity-detail-list">
                  <li v-for="activity in group.items" :key="`${message.id}-${group.id}-${activity.id}`"><span></span><div><strong>{{ activity.label }}</strong><small v-if="activity.detail">{{ activity.detail }}</small></div></li>
                </ol>
              </details>
              <article v-for="evidence in message.evidence" :key="`${message.id}-evidence-${evidence.id}`" class="execution-source" :class="`is-${evidence.state}`">
                <div class="execution-source-heading"><span>{{ evidenceKindLabel(evidence) }}</span><strong>{{ evidence.source_name }}</strong><small>{{ evidenceStateLabel(evidence) }}</small></div>
                <p>{{ evidenceActionLabel(evidence) }} · {{ t('sessions.executionEvidence.stage', { position: evidence.stage_position }) }}</p>
                <p v-if="evidence.citation?.source_location" class="execution-source-location">{{ evidence.citation.source_location }}</p>
                <button v-if="canOpenEvidence(evidence)" type="button" class="text-button" @click="emit('openEvidence', evidence)">{{ t('sessions.executionEvidence.openSource') }}</button>
              </article>
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
            <summary><span>{{ stage.position }}/{{ stage.total || message.stages?.length }} · {{ stage.expert_name }}</span><small>{{ stageStateLabel(stage.state) }}<template v-if="stage.provider_model_name"> · {{ stage.provider_model_name }}</template><template v-if="stage.elapsed_ms"> · {{ formatDuration(stage.elapsed_ms, locale as SupportedLocale) }}</template></small></summary>
            <div v-if="stage.final_text" class="markdown-body" v-html="renderMarkdown(displayArtifactNames(stage.final_text, message.artifacts))"></div>
            <p v-else-if="stage.error">{{ stage.error }}</p>
            <button v-if="stage.final_text" type="button" class="stage-copy" @click="copy(stage.final_text, stageCopyKey(message.id, stage.position))">{{ isCopied(stageCopyKey(message.id, stage.position)) ? t('common.copied') : t('common.copy') }}</button>
          </details>
        </div>
        <CreditConsumption v-if="message.role === 'assistant'" :value="message.creditConsumption" :state="message.state" />
        <div class="message-actions">
          <small v-if="message.role === 'user'" class="message-meta">{{ messageTime(message.timestamp) }}</small>
          <button v-if="message.canSaveWorkflow || message.workflowLink" type="button" class="message-task" @click="emit('saveWorkflow', message.id)"><Workflow :size="14" /><span>{{ message.workflowLink ? t('sessions.workflowSave.open') : t('sessions.workflowSave.action') }}</span></button>
          <button v-if="hasTaskWorkspaceContent(message)" type="button" class="message-task" :aria-pressed="message.id === selectedTaskId" @click="emit('selectTask', message.id)"><PanelRightOpen :size="14" /><span>{{ t('taskWorkspace.open') }}</span></button>
          <button v-if="message.copyText || message.content" type="button" class="message-copy" :class="{ copied: isCopied(`message:${message.id}`) }" :aria-label="messageAriaLabel(message)" @click="copy(message.copyText || message.content, `message:${message.id}`)"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="7" y="7" width="9" height="9" rx="2"/><path d="M13 7V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/></svg><span>{{ isCopied(`message:${message.id}`) ? t('common.copied') : t('common.copy') }}</span></button>
        </div>
      </div>
    </div>
  </div>
</template>
