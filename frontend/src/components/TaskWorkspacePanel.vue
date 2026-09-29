<script setup lang="ts">
import { computed } from "vue";
import { FileText, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import type { Artifact, Attachment, Evidence } from "../api/client";
import type { ConversationMessage } from "../conversationThread";

const props = defineProps<{ message: ConversationMessage; loadAttachment: (id: string) => Promise<Blob> }>();
const emit = defineEmits<{ close: []; downloadArtifact: [artifact: Artifact]; openEvidence: [evidence: Evidence]; attachmentError: []; saveWorkflow: [messageID: string]; planDecision: [messageID: string, decision: "start" | "direct" | "cancel"]; editPlan: [messageID: string] }>();
const { t, locale } = useI18n();
const consumedCredits = computed(() => {
  const raw: unknown = props.message.creditConsumption?.total_hundredths;
  if (raw === null || raw === undefined || raw === "") return undefined;
  const amount = Number(raw);
  return Number.isFinite(amount) && amount >= 0 ? (amount / 100).toFixed(2) : undefined;
});

function stateLabel(state: string) {
  if (state === "completed" || state === "succeeded") return t("common.success");
  if (state === "failed") return t("common.failed");
  if (state === "cancelled") return t("common.cancelled");
  if (state === "queued") return t("common.queued");
  if (state === "running" || state === "generating") return t("common.running");
  return t("common.waitingForUser");
}
function evidenceState(evidence: Evidence) { return t(`sessions.executionEvidence.sourceState.${evidence.state}`); }
function evidenceAction(evidence: Evidence) {
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
function canOpenEvidence(evidence: Evidence) { return evidence.kind === "knowledge" && evidence.state === "succeeded" && Boolean(evidence.container_id && evidence.citation?.revision_id); }
function planStepState(state: string) { return t(`sessions.executionPlan.stepStates.${state}`); }
function modelCallCount(message: ConversationMessage) { return message.stages?.length ?? message.creditConsumption?.stages.length ?? 0; }
function download(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = name; anchor.click(); window.setTimeout(() => URL.revokeObjectURL(url), 0);
}
async function downloadAttachment(item: Attachment) {
  try { download(await props.loadAttachment(item.id), item.name); } catch { emit("attachmentError"); }
}
</script>

<template>
  <aside class="task-workspace-panel" :aria-label="t('taskWorkspace.title')">
    <header><div><small>{{ t('taskWorkspace.eyebrow') }}</small><h2>{{ t('taskWorkspace.title') }}</h2></div><el-button text circle :aria-label="t('taskWorkspace.close')" @click="emit('close')"><X :size="18" /></el-button></header>
    <div class="task-workspace-scroll">
      <section v-if="message.executionPlan" class="task-workspace-section">
        <h3>{{ t('taskWorkspace.plan') }}</h3><p class="task-workspace-objective">{{ message.executionPlan.objective }}</p>
        <ol class="task-workspace-steps"><li v-for="step in message.executionPlan.steps ?? []" :key="step.id" :class="`is-${step.state}`"><span></span><div><strong>{{ step.label || t(`taskWorkspace.stepKinds.${step.kind}`) }}</strong><small>{{ planStepState(step.state) }}</small></div></li></ol>
        <div v-if="message.executionPlan.state === 'pending'" class="task-workspace-plan-actions">
          <el-button type="primary" @click="emit('planDecision', message.id, 'start')">{{ t('sessions.executionPlan.start') }}</el-button>
          <el-button @click="emit('editPlan', message.id)">{{ t('sessions.executionPlan.edit') }}</el-button>
          <el-button v-if="!message.executionPlan.side_effects?.length" @click="emit('planDecision', message.id, 'direct')">{{ t('sessions.executionPlan.direct') }}</el-button>
          <el-button text @click="emit('planDecision', message.id, 'cancel')">{{ t('common.cancel') }}</el-button>
        </div>
      </section>
      <section v-if="message.evidence?.length || message.activities?.length || message.stages?.length" class="task-workspace-section">
        <h3>{{ t('taskWorkspace.evidence') }}</h3>
        <div class="task-workspace-sources"><article v-for="item in message.evidence" :key="item.id"><div><strong>{{ item.source_name }}</strong><small>{{ evidenceState(item) }}</small></div><p>{{ evidenceAction(item) }}<template v-if="item.citation?.source_location"> · {{ item.citation.source_location }}</template></p><button v-if="canOpenEvidence(item)" type="button" @click="emit('openEvidence', item)">{{ t('sessions.executionEvidence.openSource') }}</button></article></div>
        <dl class="task-workspace-counts"><div v-if="message.activities?.length"><dt>{{ t('taskWorkspace.activities') }}</dt><dd>{{ message.activities.length }}</dd></div><div v-if="modelCallCount(message)"><dt>{{ t('taskWorkspace.modelCalls') }}</dt><dd>{{ modelCallCount(message) }}</dd></div></dl>
      </section>
      <section v-if="message.taskAttachments?.length || message.attachments?.length || message.artifacts?.length" class="task-workspace-section">
        <h3>{{ t('taskWorkspace.files') }}</h3>
        <div class="task-workspace-files"><button v-for="item in message.taskAttachments ?? message.attachments" :key="`attachment-${item.id}`" type="button" @click="downloadAttachment(item)"><FileText :size="16" /><span><strong>{{ item.name }}</strong><small>{{ t('taskWorkspace.inputFile') }}</small></span></button><button v-for="item in message.artifacts" :key="`artifact-${item.id}`" type="button" :disabled="item.expired" @click="emit('downloadArtifact', item)"><FileText :size="16" /><span><strong>{{ item.name }}</strong><small>{{ item.expired ? t('workflows.expired') : t('taskWorkspace.outputFile') }}</small></span></button></div>
      </section>
      <section class="task-workspace-section task-workspace-result">
        <h3>{{ t('taskWorkspace.result') }}</h3>
        <dl><div><dt>{{ t('taskWorkspace.state') }}</dt><dd>{{ stateLabel(message.state) }}</dd></div><div v-if="message.elapsedMs"><dt>{{ t('taskWorkspace.elapsed') }}</dt><dd>{{ formatDuration(message.elapsedMs, locale as SupportedLocale) }}</dd></div><div v-if="consumedCredits !== undefined"><dt>{{ t('taskWorkspace.credits') }}</dt><dd>{{ consumedCredits }}</dd></div></dl>
        <el-button v-if="message.canSaveWorkflow || message.workflowLink" type="primary" plain @click="emit('saveWorkflow', message.id)">{{ message.workflowLink ? t('sessions.workflowSave.open') : t('sessions.workflowSave.action') }}</el-button>
      </section>
    </div>
  </aside>
</template>
