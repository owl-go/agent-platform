<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Paperclip, X } from "@lucide/vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import { renderMarkdown } from "../markdown";
import { platformApiKey, runtimeEngineDisplayName, type Artifact, type Attachment, type Expert, type ExpertTeam, type ModelProviderConnection, type Run, type RunEvent, type RuntimeEngineStatus, type Workflow, type WorkflowInput, type WorkspaceEntry } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";

type Tab = "artifacts" | "workspace" | "history" | "settings";
const api = inject(platformApiKey)!;
const route = useRoute(); const router = useRouter(); const { t, locale } = useI18n();
const workflowID = computed(() => String(route.params.workflowId));
const origin = window.location.origin;
const runConversationElement = ref<HTMLElement>();
const tab = ref<Tab>((route.query.tab as Tab) || "artifacts"); const workflow = ref<Workflow>(); const experts = ref<Expert[]>([]); const expertTeams = ref<ExpertTeam[]>([]); const connections = ref<ModelProviderConnection[]>([]); const runtimes = ref<RuntimeEngineStatus[]>([]); const runs = ref<Run[]>([]); const selectedRun = ref<Run>(); const conversationRuns = ref<Run[]>([]); const runEvents = ref<RunEvent[]>([]); const eventRunID = ref(""); const streamingRunID = ref(""); const revealedRunOutput = ref(""); const followUpInput = ref(""); const sendingFollowUp = ref(false); const artifacts = ref<Artifact[]>([]); const entries = ref<WorkspaceEntry[]>([]); const workspacePath = ref(""); const workspaceUsage = ref({ used: 0, limit: 1 }); const loading = ref(true); const error = ref(""); const running = ref(false); const preview = ref<{ path: string; content: string }>(); const credential = ref<{ api_key: string; api_secret: string }>();
const pendingAttachments = ref<File[]>([]); const attachmentURLs = ref<Record<string, string>>({});
const copiedStageKey = ref("");
const models = computed(() => connections.value.flatMap((connection) => connection.models.filter((model) => model.available).map((model) => ({ ...model, connection_name: connection.name }))));
const settingsForm = ref<WorkflowInput>({ name: "", goal: "", environment: [] });
function modelIncompatible(model: (typeof models.value)[number]) {
  if (!settingsForm.value.runtime_engine) return false;
  return model.compatibility.find((item) => item.runtime_engine === settingsForm.value.runtime_engine)?.status === "incompatible";
}
watch(() => settingsForm.value.runtime_engine, () => {
  const selectedModel = models.value.find((model) => model.id === settingsForm.value.provider_model_id);
  if (selectedModel && modelIncompatible(selectedModel)) settingsForm.value.provider_model_id = undefined;
});
const tabs: Tab[] = ["artifacts", "workspace", "history", "settings"];
const fileArtifacts = computed(() => artifacts.value.filter((item) => item.kind === "file"));
const latestConversationRun = computed(() => conversationRuns.value.at(-1) ?? selectedRun.value);
const activeConversationRun = computed(() => conversationRuns.value.find((item) => item.state === "queued" || item.state === "running"));
const conversationElapsed = computed(() => conversationRuns.value.reduce((total, item) => total + item.elapsed_ms, 0));
const currentExpertStage = computed(() => [...runEvents.value].reverse().find((event) => event.type === "expert.stage.updated")?.payload);
const runtimeActivities = computed(() => {
  const activities: Array<{ sequence: number; label: string; detail: string }> = [];
  for (const event of runEvents.value) {
    const activity = runtimeActivity(event);
    if (!activity) continue;
    const previous = activities.at(-1);
    if (previous?.label === activity.label && previous.detail === activity.detail) previous.sequence = event.sequence;
    else activities.push({ sequence: event.sequence, ...activity });
  }
  return activities.slice(-8);
});
watch(tab, (value) => {
  void router.replace({ query: { ...route.query, tab: value } });
  if (value === "workspace" && workflow.value && !workflow.value.deleted) void loadDirectory(workspacePath.value);
});
let runTimer: ReturnType<typeof setInterval> | undefined;
let eventController: AbortController | undefined;
let revealTimer: ReturnType<typeof setTimeout> | undefined;
let revealTarget = "";
onMounted(async () => { await refresh(); runTimer = setInterval(() => void refreshRuns(), 1500); });
onBeforeUnmount(() => { if (runTimer) clearInterval(runTimer); eventController?.abort(); stopRunReveal(); });
async function refresh() { loading.value = true; error.value = ""; try { workflow.value = await api.getWorkflow(workflowID.value); settingsForm.value = { name: workflow.value.name, goal: workflow.value.goal, expert_id: workflow.value.expert_id, expert_team_id: workflow.value.expert_team_id, provider_model_id: workflow.value.provider_model_id, runtime_engine: workflow.value.runtime_engine, environment: workflow.value.environment ?? [], schedule: workflow.value.schedule }; [experts.value, expertTeams.value, connections.value, runtimes.value, runs.value, artifacts.value] = await Promise.all([api.listExperts(), api.listExpertTeams(), api.listModelProviderConnections(), api.listRuntimeEngines(), api.listRuns(workflowID.value), api.listArtifacts(workflowID.value)]); if (!workflow.value.deleted) await loadDirectory(""); else if (tab.value === "workspace" || tab.value === "settings") tab.value = "history"; } catch { error.value = t("errors.generic"); } finally { loading.value = false; } }
async function refreshRuns() { try { runs.value = await api.listRuns(workflowID.value); if (selectedRun.value) { selectedRun.value = runs.value.find((item) => item.id === selectedRun.value?.id) ?? selectedRun.value; conversationRuns.value = await api.listRunTurns(workflowID.value, selectedRun.value.id); } if (!runs.value.some((item) => item.state === "queued" || item.state === "running")) artifacts.value = await api.listArtifacts(workflowID.value); } catch { /* Keep the last usable projection during a transient poll failure. */ } }
async function runNow() { running.value = true; try { const created = await api.runWorkflow(workflowID.value); tab.value = "history"; runs.value = [created, ...runs.value.filter((item) => item.id !== created.id)]; await openRun(created); } catch { error.value = t("errors.generic"); } finally { running.value = false; } }
async function loadDirectory(path: string) { const result = await api.listWorkspace(workflowID.value, path); entries.value = result.items ?? []; workspacePath.value = path; workspaceUsage.value = { used: result.used_bytes, limit: result.limit_bytes }; }
async function openEntry(entry: WorkspaceEntry) { if (entry.directory) { await loadDirectory(entry.path); return; } if (entry.size > 1024 * 1024) { await downloadEntry(entry); return; } const file = await api.getWorkspaceFile(workflowID.value, entry.path); if (!file.content_type.startsWith("text/") && !file.content_type.includes("json") && !file.content_type.includes("xml")) { await downloadEntry(entry); return; } preview.value = { path: file.path, content: decodeBase64(file.content) }; }
async function downloadEntry(entry: WorkspaceEntry) { const blob = await api.downloadWorkspaceFile(workflowID.value, entry.path); const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = entry.name; anchor.click(); URL.revokeObjectURL(url); }
async function upload(event: Event) { const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; if (file.size > 100 * 1024 * 1024) { error.value = t("errors.validation"); input.value = ""; return; } const path = [workspacePath.value, file.name].filter(Boolean).join("/"); try { await api.uploadWorkspaceFile(workflowID.value, path, file); await loadDirectory(workspacePath.value); } catch { error.value = t("errors.validation"); } input.value = ""; }
async function mkdir() { const name = window.prompt(t("workflows.directoryName"))?.trim(); if (!name) return; await api.createWorkspaceDirectory(workflowID.value, [workspacePath.value, name].filter(Boolean).join("/")); await loadDirectory(workspacePath.value); }
async function clearWorkspace() { if (!workflow.value) return; const confirmation = window.prompt(`Enter “${workflow.value.name}” to clear the workspace`); if (confirmation !== workflow.value.name) return; await api.clearWorkspace(workflowID.value, confirmation); await loadDirectory(""); }
async function clone() { const url = window.prompt(t("workflows.gitURL"))?.trim(); if (!url) return; const branch = window.prompt(t("workflows.branch"), "main")?.trim() || "main"; const ssh = url.startsWith("ssh://") || /^[^@\s]+@[^:\s]+:/.test(url); const ssh_private_key = ssh ? window.prompt(t("workflows.privateKey"))?.trim() : undefined; if (ssh && !ssh_private_key) return; try { workflow.value = await api.cloneWorkspace(workflowID.value, { url, branch, ssh_private_key }); await loadDirectory(""); } catch { error.value = t("errors.validation"); } }
async function saveSettings() { if (!workflow.value) return; try { workflow.value = await api.updateWorkflow(workflowID.value, settingsForm.value, workflow.value.version); await refresh(); } catch { error.value = t("errors.conflict"); } }
async function generateCredential() { try { credential.value = await api.generateWorkflowCredential(workflowID.value); } catch { error.value = t("errors.generic"); } }
async function removeWorkflow() { if (!workflow.value || !window.confirm(`${t('common.delete')} “${workflow.value.name}”?`)) return; await api.deleteWorkflow(workflowID.value); await router.push("/workflows"); }
async function cancelRun(item: Run) { const turns = await api.listRunTurns(workflowID.value, item.id); const active = turns.find((turn) => turn.state === "queued" || turn.state === "running"); if (active) await api.cancelRun(workflowID.value, active.id); runs.value = await api.listRuns(workflowID.value); }
async function rerun(item: Run) { const turns = await api.listRunTurns(workflowID.value, item.id); const latest = turns.at(-1); if (latest) await api.rerunWorkflow(workflowID.value, latest.id); runs.value = await api.listRuns(workflowID.value); }
async function openRun(item: Run) {
	eventController?.abort();
	stopRunReveal();
	selectedRun.value = item;
	conversationRuns.value = await api.listRunTurns(workflowID.value, item.id);
	void hydrateAttachmentURLs(conversationRuns.value.flatMap((turn) => turn.attachments ?? []));
	runEvents.value = [];
	eventRunID.value = "";
	await scrollConversationToEnd();
	const active = activeConversationRun.value;
	if (active) void streamConversationTurn(active);
}
async function streamConversationTurn(item: Run) {
	eventController?.abort();
	eventController = new AbortController();
	streamingRunID.value = item.id;
	eventRunID.value = item.id;
	runEvents.value = [];
	stopRunReveal();
	revealedRunOutput.value = "";
	try {
		await api.streamRunEvents(workflowID.value, item.id, handleRunEvent, eventController.signal);
		const completed = await api.getRun(workflowID.value, item.id);
		if (completed.final_text) setRunRevealTarget(completed.final_text);
		else if (completed.final_json) setRunRevealTarget(`\`\`\`json\n${JSON.stringify(completed.final_json, null, 2)}\n\`\`\``);
		await waitForRunReveal(item.id);
		conversationRuns.value = conversationRuns.value.map((turn) => turn.id === completed.id ? completed : turn);
	} catch (streamError) {
		if (!(streamError instanceof DOMException && streamError.name === "AbortError")) error.value = t("errors.generic");
	} finally {
		if (streamingRunID.value === item.id) streamingRunID.value = "";
	}
}
function handleRunEvent(event: RunEvent) {
	runEvents.value.push(event);
	if (event.type === "expert.stage.updated" && event.payload.state === "running") {
		stopRunReveal();
		revealedRunOutput.value = "";
	} else if (event.type === "message.delta" && typeof event.payload.delta === "string") {
		setRunRevealTarget(revealTarget + event.payload.delta);
	} else if (event.type === "message.completed" && typeof event.payload.message === "string") {
		setRunRevealTarget(event.payload.message);
	}
	void scrollConversationToEnd();
}
function setRunRevealTarget(value: string) {
	if (!value.startsWith(revealedRunOutput.value)) revealedRunOutput.value = "";
	revealTarget = value;
	revealNextRunChunk();
}
function revealNextRunChunk() {
	if (revealTimer) return;
	const tick = () => {
		if (revealedRunOutput.value.length >= revealTarget.length) { revealTimer = undefined; return; }
		const remaining = revealTarget.length - revealedRunOutput.value.length;
		let end = revealedRunOutput.value.length + Math.min(24, Math.max(1, Math.ceil(remaining / 32)));
		const lastCode = revealTarget.charCodeAt(end - 1);
		if (lastCode >= 0xD800 && lastCode <= 0xDBFF) end += 1;
		revealedRunOutput.value = revealTarget.slice(0, end);
		revealTimer = setTimeout(tick, 22);
	};
	tick();
}
function stopRunReveal() {
	if (revealTimer) clearTimeout(revealTimer);
	revealTimer = undefined;
	revealTarget = "";
}
async function waitForRunReveal(runID: string) {
	while (streamingRunID.value === runID && (revealTimer || revealedRunOutput.value.length < revealTarget.length)) await new Promise((resolve) => setTimeout(resolve, 25));
}
async function sendFollowUp() {
	if (!selectedRun.value || (!followUpInput.value.trim() && pendingAttachments.value.length === 0) || activeConversationRun.value || sendingFollowUp.value) return;
	sendingFollowUp.value = true;
	try {
		const uploaded = await Promise.all(pendingAttachments.value.map((file) => api.uploadAttachment(file)));
		const created = await api.continueRunConversation(workflowID.value, selectedRun.value.id, followUpInput.value.trim(), uploaded.map((item) => item.id));
		followUpInput.value = "";
		pendingAttachments.value = [];
		conversationRuns.value.push(created);
		void hydrateAttachmentURLs(created.attachments ?? []);
		await scrollConversationToEnd();
		void streamConversationTurn(created);
	} catch { error.value = t("errors.generic"); } finally { sendingFollowUp.value = false; }
}
function chooseConversationAttachments(event: Event) {
	const input = event.target as HTMLInputElement; const files = [...(input.files ?? [])];
	if (files.some((file) => file.size > 100 * 1024 * 1024) || pendingAttachments.value.length + files.length > 10) error.value = t("sessions.attachmentLimits");
	else pendingAttachments.value.push(...files);
	input.value = "";
}
function removeConversationAttachment(index: number) { pendingAttachments.value.splice(index, 1); }
async function hydrateAttachmentURLs(attachments: Attachment[]) { await Promise.all(attachments.filter((item) => item.image && !attachmentURLs.value[item.id]).map(async (item) => { try { attachmentURLs.value[item.id] = (await api.getAttachmentDownload(item.id)).url; } catch { /* Keep the file card usable. */ } })); }
async function openTurnAttachment(attachment: Attachment) { try { window.open((await api.getAttachmentDownload(attachment.id)).url, "_blank", "noopener,noreferrer"); } catch { error.value = t("errors.generic"); } }
async function copyStage(runID: string, position: number, value: string) { try { await navigator.clipboard.writeText(value); copiedStageKey.value = `${runID}:${position}`; window.setTimeout(() => { copiedStageKey.value = ""; }, 1600); } catch { error.value = t("errors.copy"); } }
async function cancelConversationRun() { const active = activeConversationRun.value; if (!active) return; await api.cancelRun(workflowID.value, active.id); eventController?.abort(); conversationRuns.value = await api.listRunTurns(workflowID.value, selectedRun.value!.id); }
function closeRun() { eventController?.abort(); eventController = undefined; stopRunReveal(); selectedRun.value = undefined; conversationRuns.value = []; runEvents.value = []; eventRunID.value = ""; streamingRunID.value = ""; revealedRunOutput.value = ""; followUpInput.value = ""; pendingAttachments.value = []; }
function runInputText(item: Run, index: number) { const input = item.text_input || (item.json_input ? JSON.stringify(item.json_input, null, 2) : ""); return index === 0 ? [workflow.value?.goal, input].filter(Boolean).join("\n\n") : input; }
function runOutput(item: Run) { return (item.id === streamingRunID.value ? revealedRunOutput.value : "") || item.final_text || (item.final_json ? `\`\`\`json\n${JSON.stringify(item.final_json, null, 2)}\n\`\`\`` : "") || item.error || ""; }
function runtimeActivity(event: RunEvent) {
	if (event.type === "runtime.started") return { label: t("sessions.progress.preparing"), detail: typeof event.payload.runtime === "string" ? runtimeEngineDisplayName(event.payload.runtime as RuntimeEngineStatus["name"]) : "" };
	if (event.type === "command.requested") return { label: t("sessions.progress.using_tool"), detail: typeof event.payload.tool === "string" ? event.payload.tool : t("workflows.command") };
	if (event.type === "command.completed") return { label: t("workflows.toolCompleted"), detail: typeof event.payload.tool === "string" ? event.payload.tool : t("workflows.command") };
	if (event.type === "file.changed") return { label: t("workflows.updatingFiles"), detail: "" };
	if (event.type === "message.delta") return { label: t("workflows.streamingAnswer"), detail: "" };
	if (event.type === "message.completed") return { label: t("workflows.answerReady"), detail: "" };
	return undefined;
}
function visibleStages(item: Run) { const stages = item.expert_stages ?? []; return item.state === "failed" || item.state === "cancelled" ? stages : stages.slice(0, -1); }
async function scrollConversationToEnd() { await nextTick(); runConversationElement.value?.scrollTo?.({ top: runConversationElement.value.scrollHeight, behavior: "smooth" }); }
function addEnvironment() { settingsForm.value.environment.push({ name: "", value: "", secret: false, configured: false }); }
function removeEnvironment(index: number) { settingsForm.value.environment.splice(index, 1); }
function setWorkflowSpecialist(value: string) { settingsForm.value.expert_id = value.startsWith("expert:") ? value.slice(7) : undefined; settingsForm.value.expert_team_id = value.startsWith("team:") ? value.slice(5) : undefined; }
function enableSchedule() { settingsForm.value.schedule = settingsForm.value.schedule ?? { enabled: true, frequency: "daily", hour: 9, minute: 0, weekday: 1, timezone: "Asia/Shanghai" }; }
async function openArtifact(item: Artifact) { if (item.text_preview) { preview.value = { path: item.path || item.name, content: item.text_preview }; return; } if (item.kind === "file" && !item.expired) { try { const download = await api.getArtifactDownload(workflowID.value, item.id); window.open(download.url, "_blank", "noopener,noreferrer"); } catch { error.value = t("errors.generic"); } } }
function stateLabel(state: Run["state"]) { return state === "succeeded" ? t("common.success") : state === "failed" ? t("common.failed") : state === "running" ? t("common.running") : state === "queued" ? t("common.queued") : state; }
function stageStateLabel(state: string) { return state === "succeeded" ? t("common.success") : state === "failed" ? t("common.failed") : state === "cancelled" ? t("common.cancelled") : state === "running" ? t("common.running") : state; }
function triggerLabel(trigger: Run["trigger"]) { return t(`workflows.${trigger}`); }
function parentPath() { const parts = workspacePath.value.split("/").filter(Boolean); parts.pop(); return parts.join("/"); }
function decodeBase64(value: string) { try { return decodeURIComponent(escape(atob(value))); } catch { return atob(value); } }
</script>

<template>
  <section class="detail-page workflow-detail-page" :class="{ 'workflow-run-view': selectedRun }">
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <div v-if="selectedRun" class="run-page">
      <header class="run-conversation-head"><div><button class="back-link" @click="closeRun">← {{ t('common.back') }}</button><p class="eyebrow">{{ workflow?.name ?? selectedRun.workflow_name }} / RUN {{ selectedRun.id.slice(0, 8) }}</p><h2>{{ t('workflows.conversation') }}</h2><p v-if="latestConversationRun"><span><i class="run-state" :class="latestConversationRun.state"></i>{{ stateLabel(latestConversationRun.state) }}</span><span>{{ triggerLabel(selectedRun.trigger) }}</span><span>{{ formatDuration(conversationElapsed, locale as SupportedLocale) }}</span><span>{{ new Date(latestConversationRun.started_at || latestConversationRun.queued_at).toLocaleString() }}</span></p></div></header>
      <div ref="runConversationElement" class="run-conversation">
        <template v-for="(turn, index) in conversationRuns" :key="turn.id">
          <article class="message user"><div class="message-content"><p v-if="runInputText(turn, index)">{{ runInputText(turn, index) }}</p><div v-if="turn.attachments?.length" class="turn-attachments"><button v-for="attachment in turn.attachments" :key="attachment.id" type="button" class="turn-attachment" @click="openTurnAttachment(attachment)"><img v-if="attachment.image && attachmentURLs[attachment.id]" :src="attachmentURLs[attachment.id]" :alt="attachment.name"><span v-else class="attachment-file-mark">FILE</span><span><strong>{{ attachment.name }}</strong><small>{{ (attachment.size / 1024).toFixed(1) }} KB</small></span></button></div><small>{{ new Date(turn.queued_at).toLocaleString() }}</small></div></article>
          <article class="message assistant"><div class="message-content"><div v-if="turn.id === eventRunID && runtimeActivities.length" class="runtime-activity" aria-live="polite"><div class="runtime-activity-current"><span class="activity-pulse" :class="{ active: turn.id === streamingRunID }"></span><strong>{{ runtimeActivities.at(-1)?.label }}</strong><small v-if="runtimeActivities.at(-1)?.detail">{{ runtimeActivities.at(-1)?.detail }}</small></div><details v-if="runtimeActivities.length > 1"><summary>{{ t('workflows.activityDetails') }}</summary><ol><li v-for="activity in runtimeActivities" :key="activity.sequence"><span></span><div><strong>{{ activity.label }}</strong><small v-if="activity.detail">{{ activity.detail }}</small></div></li></ol></details></div><div v-if="runOutput(turn)" class="markdown-body" :class="{ streaming: turn.id === streamingRunID }" v-html="renderMarkdown(runOutput(turn))"></div><div v-else-if="turn.state === 'queued' || turn.state === 'running'" class="thinking-state"><span class="thinking-dots"><i></i><i></i><i></i></span><strong>{{ t('sessions.thinking') }}</strong><small v-if="turn.id === streamingRunID && currentExpertStage">{{ currentExpertStage.position }}/{{ currentExpertStage.total || '' }} · {{ currentExpertStage.expert_name }}</small><small v-else>{{ t('sessions.progress.thinking') }}</small></div><p v-else class="muted">{{ stateLabel(turn.state) }}</p><div v-if="visibleStages(turn).length" class="expert-stage-list"><details v-for="stage in visibleStages(turn)" :key="`${stage.position}-${stage.expert_id}`"><summary><span>{{ stage.position }}/{{ stage.total || turn.expert_stages?.length }} · {{ stage.expert_name }}</span><small>{{ stageStateLabel(stage.state) }} · {{ formatDuration(stage.elapsed_ms, locale as SupportedLocale) }}</small></summary><div v-if="stage.final_text" class="markdown-body" v-html="renderMarkdown(stage.final_text)"></div><p v-else-if="stage.error">{{ stage.error }}</p><button v-if="stage.final_text" type="button" class="stage-copy" @click="copyStage(turn.id, stage.position, stage.final_text)">{{ copiedStageKey === `${turn.id}:${stage.position}` ? t('common.copied') : t('common.copy') }}</button></details></div><div v-if="fileArtifacts.some((item) => item.run_id === turn.id)" class="run-attachments"><button v-for="item in fileArtifacts.filter((artifact) => artifact.run_id === turn.id)" :key="item.id" type="button" @click="openArtifact(item)"><span class="file-icon">FILE</span><span><strong>{{ item.name }}</strong><small>{{ item.size }} B</small></span></button></div><small>{{ turn.ended_at ? new Date(turn.ended_at).toLocaleString() : stateLabel(turn.state) }}</small></div></article>
        </template>
      </div>
      <form v-if="!workflow?.deleted" class="run-composer" @submit.prevent="sendFollowUp"><div v-if="pendingAttachments.length" class="pending-attachments"><span v-for="(file, index) in pendingAttachments" :key="`${file.name}-${index}`">{{ file.name }}<button type="button" :aria-label="t('sessions.removeAttachment', { name: file.name })" @click="removeConversationAttachment(index)"><X /></button></span></div><textarea v-model="followUpInput" rows="2" :placeholder="t('workflows.followUpPlaceholder')" :disabled="Boolean(activeConversationRun)" @keydown.enter.exact.prevent="sendFollowUp"></textarea><label class="attachment-picker" :title="t('sessions.addAttachment')"><Paperclip aria-hidden="true"/><input type="file" multiple :disabled="Boolean(activeConversationRun) || sendingFollowUp" @change="chooseConversationAttachments"></label><button v-if="activeConversationRun" type="button" class="run-stop" :aria-label="t('sessions.stopGeneration')" @click="cancelConversationRun">■</button><button v-else type="submit" :disabled="sendingFollowUp || (!followUpInput.trim() && pendingAttachments.length === 0)" :aria-label="t('common.send')">↑</button></form>
    </div>
    <template v-else>
      <header class="detail-hero"><button class="back-link" @click="router.push('/workflows')">← {{ t('common.back') }}</button><div v-if="workflow"><p class="eyebrow">WORKFLOW / {{ workflow.id.slice(0, 8).toUpperCase() }}</p><h1>{{ workflow.name }}</h1><p>{{ workflow.goal }}</p></div><button v-if="workflow && !workflow.deleted" class="button primary" :disabled="running" @click="runNow">{{ running ? t('common.running') : '▶ ' + t('workflows.runNow') }}</button><span v-else-if="workflow" class="engine-chip">{{ t('common.readOnly') }}</span></header>
      <div v-if="loading" class="quiet-state large">{{ t('common.loading') }}</div>
      <template v-else-if="workflow">
      <nav class="tabs"><button v-for="item in tabs" :key="item" :class="{ active: tab === item }" @click="tab = item">{{ t(`workflows.${item}`) }}</button></nav>
      <div v-if="tab === 'artifacts'" class="tab-content"><div class="section-heading"><div><p class="eyebrow">OUTPUT / 90 DAY FILE RETENTION</p></div></div><div v-if="!fileArtifacts.length" class="empty-inline"><span>◇</span><p>{{ t('common.empty') }}</p></div><div v-else class="artifact-list"><article v-for="item in fileArtifacts" :key="item.id" role="button" tabindex="0" @click="openArtifact(item)" @keydown.enter="openArtifact(item)"><span class="file-icon">FILE</span><div><strong>{{ item.name }}</strong><small>{{ item.path }} · {{ item.size }} B <template v-if="item.expired">· {{ t('workflows.expired') }}</template></small></div><code>{{ (item.sha256 || '').slice(0, 12) }}</code></article></div></div>
      <div v-if="tab === 'workspace'" class="tab-content"><div class="section-heading"><div><p class="eyebrow">PERSISTENT / {{ Math.round(workspaceUsage.used / 1024 / 1024) }} MB / 1 GB</p><p class="path-line">/{{ workspacePath }}</p></div><div><button class="button ghost" @click="clone">Git clone</button><button class="button ghost" @click="mkdir">＋ {{ t('common.folder') }}</button><label class="button primary file-button">↑ {{ t('common.upload') }}<input type="file" @change="upload"></label></div></div><div class="usage-track"><i :style="{ width: `${Math.min(100, workspaceUsage.used / workspaceUsage.limit * 100)}%` }"></i></div><div class="file-browser"><button v-if="workspacePath" class="file-row" @click="loadDirectory(parentPath())"><span class="file-icon">UP</span><strong>..</strong></button><div v-for="entry in entries" :key="entry.path" class="file-row" role="button" tabindex="0" @click="openEntry(entry)" @keydown.enter="openEntry(entry)"><span class="file-icon">{{ entry.directory ? 'DIR' : 'DOC' }}</span><strong>{{ entry.name }}</strong><small>{{ entry.directory ? '—' : `${entry.size} B` }}</small><time>{{ new Date(entry.modified_at).toLocaleString() }}</time><button v-if="!entry.directory" class="text-button" :aria-label="t('common.download')" @click.stop="downloadEntry(entry)">↓</button></div><div v-if="!entries.length" class="empty-inline"><span>□</span><p>{{ t('common.empty') }}</p></div></div><button class="danger-link" @click="clearWorkspace">{{ t('workflows.clearWorkspace') }}</button></div>
      <div v-if="tab === 'history'" class="tab-content"><div class="section-heading"><div><p class="eyebrow">EXECUTION / IMMUTABLE SNAPSHOTS</p></div></div><div v-if="!runs.length" class="empty-inline"><span>◷</span><p>{{ t('workflows.noRuns') }}</p></div><div v-else class="run-table"><div class="run-row run-head"><span>{{ t('workflows.started') }}</span><span>{{ t('workflows.trigger') }}</span><span>{{ t('workflows.state') }}</span><span>{{ t('workflows.duration') }}</span><span></span></div><div v-for="item in runs" :key="item.id" class="run-row" role="button" tabindex="0" @click="openRun(item)" @keydown.enter="openRun(item)"><span><strong>{{ new Date(item.started_at || item.queued_at).toLocaleString() }}</strong><small>{{ item.id.slice(0, 8) }}</small></span><span>{{ triggerLabel(item.trigger) }}</span><span><i class="run-state" :class="item.state"></i>{{ stateLabel(item.state) }}</span><span>{{ formatDuration(item.elapsed_ms, locale as SupportedLocale) }}</span><span class="run-actions"><button v-if="item.state === 'queued' || item.state === 'running'" @click.stop="cancelRun(item)">{{ t('common.cancel') }}</button><button v-else-if="!workflow.deleted" @click.stop="rerun(item)">↻</button></span></div></div></div>
      <form v-if="tab === 'settings' && !workflow.deleted" class="tab-content settings-form" @submit.prevent="saveSettings">
        <div class="section-heading"><div><p class="eyebrow">CONFIGURATION / FUTURE RUNS</p></div><button class="button primary">{{ t('common.save') }}</button></div>
        <details class="settings-section" open><summary><span class="section-number">01</span><h3>{{ t('workflows.basic') }}</h3></summary><div class="form-grid"><label>{{ t('workflows.name') }}<input v-model="settingsForm.name" required></label><label>{{ t('workflows.expert') }}<select :value="settingsForm.expert_team_id ? `team:${settingsForm.expert_team_id}` : settingsForm.expert_id ? `expert:${settingsForm.expert_id}` : 'none'" @change="setWorkflowSpecialist(($event.target as HTMLSelectElement).value)"><option value="none">{{ t('sessions.noExpert') }}</option><optgroup :label="t('experts.title')"><option v-for="expert in experts" :key="expert.id" :value="`expert:${expert.id}`" :disabled="!expert.available">{{ expert.name }}</option></optgroup><optgroup :label="t('experts.teams')"><option v-for="team in expertTeams" :key="team.id" :value="`team:${team.id}`" :disabled="!team.available">{{ team.name }}</option></optgroup></select></label><label class="full">{{ t('workflows.goal') }}<textarea v-model="settingsForm.goal" rows="7" required></textarea></label></div></details>
        <details class="settings-section" open><summary><span class="section-number">02</span><h3>{{ t('workflows.execution') }}</h3></summary><div class="form-grid"><label>{{ t('settings.model') }}<select v-model="settingsForm.provider_model_id"><option :value="undefined">{{ t('workflows.personalDefault') }}</option><option v-for="model in models" :key="model.id" :value="model.id" :disabled="modelIncompatible(model)">{{ model.connection_name }} / {{ model.display_name }}</option></select></label><label>{{ t('workflows.runtime') }}<select v-model="settingsForm.runtime_engine"><option :value="undefined">{{ t('workflows.personalDefault') }}</option><option v-for="runtime in runtimes" :key="runtime.name" :value="runtime.name" :disabled="!runtime.available">{{ runtimeEngineDisplayName(runtime.name) }} · {{ runtime.available ? t('settings.available') : t('settings.unavailable') }}</option></select></label><div class="full"><div v-for="(variable, index) in settingsForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><button type="button" class="text-button" @click="removeEnvironment(index)">×</button></div><button type="button" class="button ghost" @click="addEnvironment">＋ {{ t('workflows.environment') }}</button></div></div></details>
        <details class="settings-section" open><summary><span class="section-number">03</span><h3>{{ t('workflows.schedule') }}</h3></summary><button v-if="!settingsForm.schedule" type="button" class="button ghost" @click="enableSchedule">{{ t('workflows.enableSchedule') }}</button><div v-else class="form-grid"><label><input v-model="settingsForm.schedule.enabled" type="checkbox"> {{ t('common.enabled') }}</label><label>{{ t('workflows.frequency') }}<select v-model="settingsForm.schedule.frequency"><option value="hourly">{{ t('workflows.hourly') }}</option><option value="daily">{{ t('workflows.daily') }}</option><option value="weekly">{{ t('workflows.weekly') }}</option></select></label><label>{{ t('workflows.hour') }}<input v-model.number="settingsForm.schedule.hour" type="number" min="0" max="23"></label><label>{{ t('workflows.minute') }}<input v-model.number="settingsForm.schedule.minute" type="number" min="0" max="59"></label><label v-if="settingsForm.schedule.frequency === 'weekly'">{{ t('workflows.weekday') }}<input v-model.number="settingsForm.schedule.weekday" type="number" min="0" max="6"></label><label>{{ t('workflows.timezone') }}<input v-model="settingsForm.schedule.timezone"></label></div></details>
        <details class="settings-section" open><summary><span class="section-number">04</span><h3>{{ t('workflows.apiCredential') }}</h3></summary><p class="muted">HTTP Basic: API Key / API Secret · POST /api/v1/workflows/{{ workflow.id }}/runs</p><button type="button" class="button ghost" @click="generateCredential">{{ workflow.api_credential_configured ? t('workflows.regenerate') : t('workflows.generate') }}</button><div v-if="credential" class="secret-reveal"><p>{{ t('workflows.copySecret') }}</p><code>{{ credential.api_key }}</code><code>{{ credential.api_secret }}</code><code>curl -u '{{ credential.api_key }}:{{ credential.api_secret }}' -H 'Idempotency-Key: unique-request' -H 'Content-Type: application/json' -d '{"text_input":"Run now"}' {{ origin }}/api/v1/workflows/{{ workflow.id }}/runs</code></div></details>
        <details class="settings-section" open><summary><span class="section-number">05</span><h3>{{ t('workflows.gitSource') }}</h3></summary><p v-if="workflow.git_source" class="muted">{{ workflow.git_source.url }} · {{ workflow.git_source.branch }} · {{ workflow.git_source.private_ssh ? 'SSH' : 'HTTPS' }}</p><p v-else class="muted">{{ t('workflows.gitDescription') }}</p><button type="button" class="button ghost" @click="tab = 'workspace'">{{ t('workflows.openWorkspace') }}</button></details>
        <section class="danger-zone"><h3>{{ t('workflows.deleteTitle') }}</h3><p>{{ t('workflows.deleteDescription') }}</p><button type="button" class="button danger" @click="removeWorkflow">{{ t('common.delete') }}</button></section>
      </form>
      </template>
    </template>
  </section>
  <div v-if="preview" class="modal-layer" @click.self="preview = undefined"><div class="modal-card preview-card"><div class="section-heading"><h2>{{ preview.path }}</h2><button class="icon-button" @click="preview = undefined">×</button></div><pre>{{ preview.content }}</pre></div></div>
</template>
