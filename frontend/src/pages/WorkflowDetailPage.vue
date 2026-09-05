<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ArrowUp, FileText, Folder, Paperclip, X } from "@lucide/vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import { renderMarkdown } from "../markdown";
import { displayArtifactNames } from "../artifactDisplay";
import { platformApiKey, runtimeEngineDisplayName, type Artifact, type Attachment, type Expert, type ExpertTeam, type GitSourceInput, type Run, type RunEvent, type RuntimeEngineStatus, type Workflow, type WorkflowInput, type WorkspaceEntry } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import CreditConsumption from "../components/CreditConsumption.vue";
import ArtifactDisclosure from "../components/ArtifactDisclosure.vue";

type Tab = "artifacts" | "workspace" | "history" | "settings";
const api = inject(platformApiKey)!;
const route = useRoute(); const router = useRouter(); const { t, locale } = useI18n();
const workflowID = computed(() => String(route.params.workflowId));
const origin = window.location.origin;
const runConversationElement = ref<HTMLElement>();
const runComposerLayer = ref<HTMLElement>();
const runComposerClearance = ref(154);
const tab = ref<Tab>((route.query.tab as Tab) || "artifacts"); const workflow = ref<Workflow>(); const experts = ref<Expert[]>([]); const expertTeams = ref<ExpertTeam[]>([]); const runs = ref<Run[]>([]); const selectedRun = ref<Run>(); const conversationRuns = ref<Run[]>([]); const runEvents = ref<RunEvent[]>([]); const eventRunID = ref(""); const streamingRunID = ref(""); const revealedRunOutput = ref(""); const followUpInput = ref(""); const sendingFollowUp = ref(false); const artifacts = ref<Artifact[]>([]); const entries = ref<WorkspaceEntry[]>([]); const workspacePath = ref(""); const loading = ref(true); const error = ref(""); const running = ref(false); const preview = ref<{ path: string; content: string }>(); const credential = ref<{ api_key: string; api_secret: string }>();
const pendingAttachments = ref<File[]>([]); const attachmentURLs = ref<Record<string, string>>({});
const copiedStageKey = ref("");
const nowMS = ref(Date.now());
const notice = ref(""); const confirmWorkflowDelete = ref(false); const savingGit = ref(false);
const gitForm = ref<GitSourceInput>({ url: "", branch: "main", authentication: "none", ssh_config: "", config: [] });
const settingsForm = ref<WorkflowInput>({ name: "", goal: "", environment: [] });
const tabs: Tab[] = ["artifacts", "workspace", "history", "settings"];
const fileArtifacts = computed(() => artifacts.value.filter((item) => item.kind === "file"));
const latestConversationRun = computed(() => conversationRuns.value.at(-1) ?? selectedRun.value);
const activeConversationRun = computed(() => conversationRuns.value.find((item) => item.state === "queued" || item.state === "running"));
const conversationElapsed = computed(() => conversationRuns.value.reduce((total, item) => {
  const stored = Number.isFinite(item.elapsed_ms) ? Math.max(0, item.elapsed_ms) : 0;
  if (item.state !== "queued" && item.state !== "running") return total + stored;
  const started = Date.parse(item.started_at || item.queued_at);
  return total + (Number.isFinite(started) ? Math.max(stored, nowMS.value - started) : stored);
}, 0));
const currentExpertStage = computed(() => [...runEvents.value].reverse().find((event) => event.type === "expert.stage.updated")?.payload);
const runtimeActivities = computed(() => {
  const activities: Array<{ sequence: number; label: string; historyLabel: string; detail: string }> = [];
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
let runComposerObserver: ResizeObserver | undefined;
onMounted(async () => {
  if (typeof ResizeObserver !== "undefined") runComposerObserver = new ResizeObserver(measureRunComposer);
  window.addEventListener("resize", measureRunComposer);
  await refresh();
  runTimer = setInterval(() => { nowMS.value = Date.now(); void refreshRuns(); }, 1500);
});
watch(runComposerLayer, (current, previous) => {
  if (previous) runComposerObserver?.unobserve(previous);
  if (current) runComposerObserver?.observe(current);
  void nextTick(measureRunComposer);
});
onBeforeUnmount(() => {
  if (runTimer) clearInterval(runTimer);
  eventController?.abort();
  stopRunReveal();
  clearAttachmentURLs();
  runComposerObserver?.disconnect();
  window.removeEventListener("resize", measureRunComposer);
});
function measureRunComposer() {
  const height = runComposerLayer.value?.getBoundingClientRect().height ?? 0;
  if (height <= 0) return;
  runComposerClearance.value = Math.ceil(height) + 16;
  void scrollConversationToEnd("auto");
}
async function refresh() { loading.value = true; error.value = ""; try { workflow.value = await api.getWorkflow(workflowID.value); settingsForm.value = { name: workflow.value.name, goal: workflow.value.goal, expert_id: workflow.value.expert_id, expert_team_id: workflow.value.expert_team_id, environment: workflow.value.environment ?? [], schedule: workflow.value.schedule }; const source = workflow.value.git_source; gitForm.value = source ? { url: source.url, branch: source.branch, authentication: source.authentication || "none", username: source.username, ssh_config: source.ssh_config ?? "", config: source.config ?? [] } : { url: "", branch: "main", authentication: "none", ssh_config: "", config: [] }; [experts.value, expertTeams.value, runs.value, artifacts.value] = await Promise.all([api.listExperts(), api.listExpertTeams(), api.listRuns(workflowID.value), api.listArtifacts(workflowID.value)]); if (!workflow.value.deleted) await loadDirectory(""); else if (tab.value === "workspace" || tab.value === "settings") tab.value = "history"; } catch { error.value = t("errors.generic"); } finally { loading.value = false; } }
async function refreshRuns() { try { runs.value = await api.listRuns(workflowID.value); if (selectedRun.value) { selectedRun.value = runs.value.find((item) => item.id === selectedRun.value?.id) ?? selectedRun.value; conversationRuns.value = await api.listRunTurns(workflowID.value, selectedRun.value.id); } if (!runs.value.some((item) => item.state === "queued" || item.state === "running")) artifacts.value = await api.listArtifacts(workflowID.value); } catch { /* Keep the last usable projection during a transient poll failure. */ } }
async function runNow() { running.value = true; try { const created = await api.runWorkflow(workflowID.value); tab.value = "history"; runs.value = [created, ...runs.value.filter((item) => item.id !== created.id)]; await openRun(created); } catch { error.value = t("errors.generic"); } finally { running.value = false; } }
async function loadDirectory(path: string) { const result = await api.listWorkspace(workflowID.value, path); entries.value = result.items ?? []; workspacePath.value = path; }
async function openEntry(entry: WorkspaceEntry) { if (entry.directory) { await loadDirectory(entry.path); return; } if (entry.size > 1024 * 1024) { await downloadEntry(entry); return; } const file = await api.getWorkspaceFile(workflowID.value, entry.path); if (!file.content_type.startsWith("text/") && !file.content_type.includes("json") && !file.content_type.includes("xml")) { await downloadEntry(entry); return; } preview.value = { path: file.path, content: decodeBase64(file.content) }; }
async function downloadEntry(entry: WorkspaceEntry) { const blob = await api.downloadWorkspaceFile(workflowID.value, entry.path); const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = entry.name; anchor.click(); URL.revokeObjectURL(url); }
function addGitConfig() { gitForm.value.config.push({ key: "", value: "" }); }
function removeGitConfig(index: number) { gitForm.value.config.splice(index, 1); }
async function saveGitSource() { savingGit.value = true; error.value = ""; try { workflow.value = await api.configureWorkflowGitSource(workflowID.value, gitForm.value); notice.value = t("workflows.gitSaved"); await loadDirectory(""); } catch { error.value = t("errors.validation"); } finally { savingGit.value = false; } }
async function saveSettings() { if (!workflow.value) return; try { workflow.value = await api.updateWorkflow(workflowID.value, settingsForm.value, workflow.value.version); await refresh(); } catch { error.value = t("errors.conflict"); } }
async function generateCredential() { try { credential.value = await api.generateWorkflowCredential(workflowID.value); } catch { error.value = t("errors.generic"); } }
async function removeWorkflow() { if (!workflow.value) return; await api.deleteWorkflow(workflowID.value); confirmWorkflowDelete.value = false; await router.push("/workflows"); }
async function cancelRun(item: Run) { const turns = await api.listRunTurns(workflowID.value, item.id); const active = turns.find((turn) => turn.state === "queued" || turn.state === "running"); if (active) await api.cancelRun(workflowID.value, active.id); runs.value = await api.listRuns(workflowID.value); }
async function rerun(item: Run) { const turns = await api.listRunTurns(workflowID.value, item.id); const latest = turns.at(-1); if (latest) await api.rerunWorkflow(workflowID.value, latest.id); runs.value = await api.listRuns(workflowID.value); }
async function openRun(item: Run) {
	eventController?.abort();
	stopRunReveal();
	clearAttachmentURLs();
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
		window.dispatchEvent(new Event("credits-updated"));
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
async function hydrateAttachmentURLs(attachments: Attachment[]) { await Promise.all(attachments.filter((item) => item.image && !attachmentURLs.value[item.id]).map(async (item) => { try { attachmentURLs.value[item.id] = URL.createObjectURL(await api.getAttachmentDownload(item.id)); } catch { /* Keep the file card usable. */ } })); }
function clearAttachmentURLs() { for (const url of Object.values(attachmentURLs.value)) URL.revokeObjectURL(url); attachmentURLs.value = {}; }
function clearAttachmentURL(id: string) { const url = attachmentURLs.value[id]; if (!url) return; URL.revokeObjectURL(url); delete attachmentURLs.value[id]; }
async function openTurnAttachment(attachment: Attachment) { try { const url = URL.createObjectURL(await api.getAttachmentDownload(attachment.id)); const anchor = document.createElement("a"); anchor.href = url; anchor.download = attachment.name; anchor.click(); window.setTimeout(() => URL.revokeObjectURL(url), 0); } catch { error.value = t("errors.generic"); } }
async function copyStage(runID: string, position: number, value: string) { try { await navigator.clipboard.writeText(value); copiedStageKey.value = `${runID}:${position}`; window.setTimeout(() => { copiedStageKey.value = ""; }, 1600); } catch { error.value = t("errors.copy"); } }
async function cancelConversationRun() { const active = activeConversationRun.value; if (!active) return; await api.cancelRun(workflowID.value, active.id); eventController?.abort(); conversationRuns.value = await api.listRunTurns(workflowID.value, selectedRun.value!.id); }
function closeRun() { eventController?.abort(); eventController = undefined; stopRunReveal(); clearAttachmentURLs(); selectedRun.value = undefined; conversationRuns.value = []; runEvents.value = []; eventRunID.value = ""; streamingRunID.value = ""; revealedRunOutput.value = ""; followUpInput.value = ""; pendingAttachments.value = []; }
function runInputText(item: Run, index: number) { const input = item.text_input || (item.json_input ? JSON.stringify(item.json_input, null, 2) : ""); return index === 0 ? [workflow.value?.goal, input].filter(Boolean).join("\n\n") : input; }
function runOutput(item: Run) { return (item.id === streamingRunID.value ? revealedRunOutput.value : "") || item.final_text || (item.final_json ? `\`\`\`json\n${JSON.stringify(item.final_json, null, 2)}\n\`\`\`` : "") || item.error || ""; }
function runArtifacts(item: Run) { return fileArtifacts.value.filter((artifact) => artifact.run_id === item.id); }
function runtimeActivity(event: RunEvent) {
	if (event.type === "runtime.started") return { label: t("sessions.progress.preparing"), historyLabel: t("workflows.runtimePrepared"), detail: typeof event.payload.runtime === "string" ? runtimeEngineDisplayName(event.payload.runtime as RuntimeEngineStatus["name"]) : "" };
	if (event.type === "reasoning.summary") return { label: t("workflows.reasoningSummary"), historyLabel: t("workflows.reasoningSummary"), detail: typeof event.payload.summary === "string" ? event.payload.summary : "" };
	if (event.type === "command.requested") return { label: t("sessions.progress.using_tool"), historyLabel: t("sessions.progress.using_tool"), detail: runtimeCommandDetail(event) };
	if (event.type === "command.completed") return { label: t("workflows.toolCompleted"), historyLabel: t("workflows.toolCompleted"), detail: runtimeCommandDetail(event) };
	if (event.type === "file.changed") return { label: t("workflows.updatingFiles"), historyLabel: t("workflows.updatingFiles"), detail: "" };
	if (event.type === "message.delta") return { label: t("workflows.streamingAnswer"), historyLabel: t("workflows.streamingAnswer"), detail: "" };
	if (event.type === "message.completed") return { label: t("workflows.answerReady"), historyLabel: t("workflows.answerReady"), detail: "" };
	return undefined;
}
function runtimeCommandDetail(event: RunEvent) {
	if (typeof event.payload.command === "string") return event.payload.command;
	if (typeof event.payload.tool === "string") return event.payload.tool;
	return t("workflows.command");
}
function visibleStages(item: Run) { return item.expert_stages ?? []; }
async function scrollConversationToEnd(behavior: ScrollBehavior = "smooth") { await nextTick(); runConversationElement.value?.scrollTo?.({ top: runConversationElement.value.scrollHeight, behavior }); }
function addEnvironment() { settingsForm.value.environment.push({ name: "", value: "", secret: false, configured: false }); }
function removeEnvironment(index: number) { settingsForm.value.environment.splice(index, 1); }
function setWorkflowSpecialist(value: string) { settingsForm.value.expert_id = value.startsWith("expert:") ? value.slice(7) : undefined; settingsForm.value.expert_team_id = value.startsWith("team:") ? value.slice(5) : undefined; }
function teamSelectionLabel(team: ExpertTeam): string { const compatibility = team.experts.some((item) => item.compatibility === "incompatible") ? t("experts.incompatible") : team.experts.some((item) => item.compatibility === "unverified") ? t("settings.unverified") : t("settings.verified"); return `${team.name} · ${compatibility}`; }
function enableSchedule() { settingsForm.value.schedule = settingsForm.value.schedule ?? { enabled: true, frequency: "daily", hour: 9, minute: 0, weekday: 1, timezone: "Asia/Shanghai" }; }
async function openArtifact(item: Artifact) { if (item.kind === "file" && !item.expired) { try { const blob = await api.getArtifactDownload(workflowID.value, item.id); const url = URL.createObjectURL(blob); triggerBrowserDownload(url, item.name); window.setTimeout(() => URL.revokeObjectURL(url), 0); } catch { error.value = t("errors.generic"); } } }
function triggerBrowserDownload(url: string, name: string) { const anchor = document.createElement("a"); anchor.href = url; anchor.download = name; anchor.rel = "noopener noreferrer"; anchor.click(); }
function formatFileSize(size: number) { if (size < 1024) return `${size} B`; if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`; return `${(size / (1024 * 1024)).toFixed(1)} MB`; }
function stateLabel(state: Run["state"]) { return state === "succeeded" ? t("common.success") : state === "failed" ? t("common.failed") : state === "running" ? t("common.running") : state === "waiting_for_user" ? t("common.waitingForUser") : state === "queued" ? t("common.queued") : state; }
function stageStateLabel(state: string) { return state === "succeeded" ? t("common.success") : state === "failed" ? t("common.failed") : state === "cancelled" ? t("common.cancelled") : state === "running" ? t("common.running") : state; }
function triggerLabel(trigger: Run["trigger"]) { return t(`workflows.${trigger}`); }
function parentPath() { const parts = workspacePath.value.split("/").filter(Boolean); parts.pop(); return parts.join("/"); }
function decodeBase64(value: string) { try { return decodeURIComponent(escape(atob(value))); } catch { return atob(value); } }
</script>

<template>
  <section class="detail-page workflow-detail-page" :class="{ 'workflow-run-view': selectedRun }">
    <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
    <ToastMessage v-if="notice" kind="success" :title="t('common.success')" :message="notice" :close-label="t('common.close')" @dismiss="notice = ''" />
    <div v-if="selectedRun" class="run-page">
      <header class="run-conversation-head"><div><el-button class="back-link" text @click="closeRun">← {{ t('common.back') }}</el-button><h2>{{ t('workflows.conversation') }}</h2><p v-if="latestConversationRun"><el-tag :type="latestConversationRun.state === 'succeeded' ? 'success' : latestConversationRun.state === 'failed' ? 'danger' : 'primary'" size="small">{{ stateLabel(latestConversationRun.state) }}</el-tag><span>{{ triggerLabel(selectedRun.trigger) }}</span><span>{{ formatDuration(conversationElapsed, locale as SupportedLocale) }}</span><span>{{ new Date(latestConversationRun.started_at || latestConversationRun.queued_at).toLocaleString() }}</span></p></div></header>
      <div ref="runConversationElement" class="run-conversation" :style="{ paddingBottom: `${runComposerClearance}px` }">
        <template v-for="(turn, index) in conversationRuns" :key="turn.id">
          <article class="message user"><div class="message-content"><p v-if="runInputText(turn, index)">{{ runInputText(turn, index) }}</p><div v-if="turn.attachments?.length" class="turn-attachments"><button v-for="attachment in turn.attachments" :key="attachment.id" type="button" class="turn-attachment" @click="openTurnAttachment(attachment)"><img v-if="attachment.image && attachmentURLs[attachment.id]" :src="attachmentURLs[attachment.id]" :alt="attachment.name" @error="clearAttachmentURL(attachment.id)"><span v-else class="attachment-file-mark">{{ attachment.image ? 'IMG' : 'FILE' }}</span><span><strong>{{ attachment.name }}</strong><small>{{ (attachment.size / 1024).toFixed(1) }} KB</small></span></button></div><small>{{ new Date(turn.queued_at).toLocaleString() }}</small></div></article>
          <article class="message assistant"><div class="message-content"><div v-if="turn.id === eventRunID && runtimeActivities.length" class="runtime-activity" aria-live="polite"><div v-if="turn.id === streamingRunID" class="runtime-activity-current"><span class="activity-pulse active"></span><strong>{{ runtimeActivities.at(-1)?.label }}</strong><small v-if="runtimeActivities.at(-1)?.detail">{{ runtimeActivities.at(-1)?.detail }}</small></div><details v-if="runtimeActivities.length > 1 || turn.id !== streamingRunID"><summary>{{ t('workflows.activityDetails') }}</summary><ol><li v-for="activity in runtimeActivities" :key="activity.sequence"><span></span><div><strong>{{ activity.historyLabel }}</strong><small v-if="activity.detail">{{ activity.detail }}</small></div></li></ol></details></div><div v-if="runOutput(turn)" class="markdown-body" :class="{ streaming: turn.id === streamingRunID }" v-html="renderMarkdown(displayArtifactNames(runOutput(turn), runArtifacts(turn)))"></div><div v-else-if="turn.state === 'queued' || turn.state === 'running' || turn.state === 'waiting_for_user'" class="thinking-state"><span class="thinking-dots"><i></i><i></i><i></i></span><strong>{{ turn.state === 'waiting_for_user' ? t('common.waitingForUser') : t('sessions.thinking') }}</strong><small v-if="turn.id === streamingRunID && currentExpertStage">{{ currentExpertStage.position }}/{{ currentExpertStage.total || '' }} · {{ currentExpertStage.expert_name }}</small><small v-else>{{ t('sessions.progress.thinking') }}</small></div><p v-else class="muted">{{ stateLabel(turn.state) }}</p><div v-if="visibleStages(turn).length" class="expert-stage-list"><details v-for="stage in visibleStages(turn)" :key="`${stage.position}-${stage.expert_id}`"><summary><span>{{ stage.position }}/{{ stage.total || turn.expert_stages?.length }} · {{ stage.expert_name }}</span><small>{{ stageStateLabel(stage.state) }}<template v-if="stage.provider_model_name"> · {{ stage.provider_model_name }}</template><template v-if="stage.runtime_engine"> · {{ runtimeEngineDisplayName(stage.runtime_engine) }}</template> · {{ formatDuration(stage.elapsed_ms, locale as SupportedLocale) }}</small></summary><div v-if="stage.final_text" class="markdown-body" v-html="renderMarkdown(stage.final_text)"></div><p v-else-if="stage.error">{{ stage.error }}</p><button v-if="stage.final_text" type="button" class="stage-copy" @click="copyStage(turn.id, stage.position, stage.final_text)">{{ copiedStageKey === `${turn.id}:${stage.position}` ? t('common.copied') : t('common.copy') }}</button></details></div><CreditConsumption :value="turn.credit_consumption" /><ArtifactDisclosure v-if="runArtifacts(turn).length" :artifacts="runArtifacts(turn)" @download="openArtifact" /><small>{{ turn.ended_at ? new Date(turn.ended_at).toLocaleString() : stateLabel(turn.state) }}</small></div></article>
        </template>
      </div>
      <div v-if="!workflow?.deleted" ref="runComposerLayer" class="composer-layer run-composer-layer"><form class="composer run-composer" @submit.prevent="sendFollowUp"><div v-if="pendingAttachments.length" class="pending-attachments"><span v-for="(file, index) in pendingAttachments" :key="`${file.name}-${index}`">{{ file.name }}<button type="button" :aria-label="t('sessions.removeAttachment', { name: file.name })" @click="removeConversationAttachment(index)"><X /></button></span></div><textarea v-model="followUpInput" rows="2" :placeholder="t('workflows.followUpPlaceholder')" :disabled="Boolean(activeConversationRun)" @keydown.enter.exact.prevent="sendFollowUp"></textarea><label class="attachment-picker" :title="t('sessions.addAttachment')"><Paperclip aria-hidden="true"/><input type="file" multiple :disabled="Boolean(activeConversationRun) || sendingFollowUp" @change="chooseConversationAttachments"></label><el-button v-if="activeConversationRun" type="danger" class="run-stop" :aria-label="t('sessions.stopGeneration')" @click="cancelConversationRun">■</el-button><el-button v-else native-type="submit" type="primary" :loading="sendingFollowUp" :disabled="!followUpInput.trim() && pendingAttachments.length === 0" :aria-label="t('common.send')">↑</el-button></form></div>
    </div>
    <template v-else>
      <header class="detail-hero"><el-button class="back-link" text @click="router.push('/workflows')">← {{ t('common.back') }}</el-button><div v-if="workflow"><h1>{{ workflow.name }}</h1><p>{{ workflow.goal }}</p></div><el-button v-if="workflow && !workflow.deleted" class="button primary" type="primary" :loading="running" @click="runNow">{{ running ? t('common.running') : '▶ ' + t('workflows.runNow') }}</el-button><el-tag v-else-if="workflow" type="info">{{ t('common.readOnly') }}</el-tag></header>
      <el-skeleton v-if="loading" :rows="10" animated class="page-loading" />
      <template v-else-if="workflow">
      <nav class="tabs"><el-button v-for="item in tabs" :key="item" text :class="{ active: tab === item }" @click="tab = item">{{ t(`workflows.${item}`) }}</el-button></nav>
      <div v-if="tab === 'artifacts'" class="tab-content"><div v-if="!fileArtifacts.length" class="empty-inline"><span>◇</span><p>{{ t('common.empty') }}</p></div><div v-else class="artifact-list"><article v-for="item in fileArtifacts" :key="item.id" role="button" tabindex="0" @click="openArtifact(item)" @keydown.enter="openArtifact(item)"><span class="file-icon" aria-hidden="true"><FileText /></span><div><strong>{{ item.name }}</strong><small>{{ formatFileSize(item.size) }} <template v-if="item.expired">· {{ t('workflows.expired') }}</template></small></div><code>{{ (item.sha256 || '').slice(0, 12) }}</code></article></div></div>
      <div v-if="tab === 'workspace'" class="tab-content"><div class="file-browser"><button v-if="workspacePath" class="file-row" @click="loadDirectory(parentPath())"><span class="file-icon" aria-hidden="true"><ArrowUp /></span><strong>..</strong></button><div v-for="entry in entries" :key="entry.path" class="file-row" role="button" tabindex="0" @click="openEntry(entry)" @keydown.enter="openEntry(entry)"><span class="file-icon" aria-hidden="true"><Folder v-if="entry.directory" /><FileText v-else /></span><strong>{{ entry.name }}</strong><small>{{ entry.directory ? '—' : `${entry.size} B` }}</small><time>{{ new Date(entry.modified_at).toLocaleString() }}</time><button v-if="!entry.directory" class="text-button" :aria-label="t('common.download')" @click.stop="downloadEntry(entry)">↓</button></div><div v-if="!entries.length" class="empty-inline"><span>□</span><p>{{ t('common.empty') }}</p></div></div></div>
      <div v-if="tab === 'history'" class="tab-content"><el-empty v-if="!runs.length" :description="t('workflows.noRuns')" /><div v-else class="run-table"><div class="run-row run-head"><span>{{ t('workflows.started') }}</span><span>{{ t('workflows.trigger') }}</span><span>{{ t('workflows.state') }}</span><span>{{ t('workflows.duration') }}</span><span></span></div><div v-for="item in runs" :key="item.id" class="run-row" role="button" tabindex="0" @click="openRun(item)" @keydown.enter="openRun(item)"><span><strong>{{ new Date(item.started_at || item.queued_at).toLocaleString() }}</strong><small>{{ item.id.slice(0, 8) }}</small></span><span>{{ triggerLabel(item.trigger) }}</span><span><el-tag :type="item.state === 'succeeded' ? 'success' : item.state === 'failed' ? 'danger' : 'primary'" size="small">{{ stateLabel(item.state) }}</el-tag></span><span>{{ formatDuration(item.elapsed_ms, locale as SupportedLocale) }}</span><span class="run-actions"><el-button v-if="item.state === 'queued' || item.state === 'running' || item.state === 'waiting_for_user'" size="small" @click.stop="cancelRun(item)">{{ t('common.cancel') }}</el-button><el-button v-else-if="!workflow.deleted" circle size="small" @click.stop="rerun(item)">↻</el-button></span></div></div></div>
      <form v-if="tab === 'settings' && !workflow.deleted" class="tab-content settings-form" @submit.prevent="saveSettings">
        <div class="section-heading section-heading-actions"><el-button native-type="submit" type="primary">{{ t('common.save') }}</el-button></div>
        <details class="settings-section" open><summary><h3>{{ t('workflows.basic') }}</h3></summary><div class="form-grid"><label>{{ t('workflows.name') }}<input v-model="settingsForm.name" required></label><label>{{ t('workflows.expert') }}<select :value="settingsForm.expert_team_id ? `team:${settingsForm.expert_team_id}` : settingsForm.expert_id ? `expert:${settingsForm.expert_id}` : 'none'" @change="setWorkflowSpecialist(($event.target as HTMLSelectElement).value)"><option value="none">{{ t('sessions.noExpert') }}</option><optgroup :label="t('experts.title')"><option v-for="expert in experts" :key="expert.id" :value="`expert:${expert.id}`" :disabled="!expert.available">{{ expert.name }}</option></optgroup><optgroup :label="t('experts.teams')"><option v-for="team in expertTeams" :key="team.id" :value="`team:${team.id}`" :disabled="!team.available">{{ teamSelectionLabel(team) }}</option></optgroup></select></label><label class="full">{{ t('workflows.goal') }}<textarea v-model="settingsForm.goal" rows="7" required></textarea></label></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.execution') }}</h3></summary><div class="form-grid"><div class="full"><div v-for="(variable, index) in settingsForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><button type="button" class="text-button" @click="removeEnvironment(index)">×</button></div><button type="button" class="button ghost" @click="addEnvironment">＋ {{ t('workflows.environment') }}</button></div></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.schedule') }}</h3></summary><button v-if="!settingsForm.schedule" type="button" class="button ghost" @click="enableSchedule">{{ t('workflows.enableSchedule') }}</button><div v-else class="form-grid"><label><input v-model="settingsForm.schedule.enabled" type="checkbox"> {{ t('common.enabled') }}</label><label>{{ t('workflows.frequency') }}<select v-model="settingsForm.schedule.frequency"><option value="hourly">{{ t('workflows.hourly') }}</option><option value="daily">{{ t('workflows.daily') }}</option><option value="weekly">{{ t('workflows.weekly') }}</option></select></label><label>{{ t('workflows.hour') }}<input v-model.number="settingsForm.schedule.hour" type="number" min="0" max="23"></label><label>{{ t('workflows.minute') }}<input v-model.number="settingsForm.schedule.minute" type="number" min="0" max="59"></label><label v-if="settingsForm.schedule.frequency === 'weekly'">{{ t('workflows.weekday') }}<input v-model.number="settingsForm.schedule.weekday" type="number" min="0" max="6"></label><label>{{ t('workflows.timezone') }}<input v-model="settingsForm.schedule.timezone"></label></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.apiCredential') }}</h3></summary><p class="muted">{{ t('workflows.apiTokenDescription') }}</p><button type="button" class="button ghost" @click="generateCredential">{{ workflow.api_credential_configured ? t('workflows.regenerate') : t('workflows.generate') }}</button><div v-if="credential" class="secret-reveal"><p>{{ t('workflows.copySecret') }}</p><code>API_KEY={{ credential.api_key }}</code><code>API_SECRET={{ credential.api_secret }}</code><code>JWT_TOKEN=$(curl -sS -u "$API_KEY:$API_SECRET" -X POST {{ origin }}/api/v1/workflows/{{ workflow.id }}/api-token | jq -r '.jwt_token')</code><code>curl -H "Authorization: Bearer $JWT_TOKEN" -H 'Idempotency-Key: unique-request' -H 'Content-Type: application/json' -d '{"text_input":"Run now"}' {{ origin }}/api/v1/workflows/{{ workflow.id }}/runs</code></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.gitSource') }}</h3></summary><div class="form-grid git-settings"><label class="full">{{ t('workflows.gitURL') }}<input v-model="gitForm.url" type="text" required spellcheck="false" autocapitalize="off" placeholder="git@github.com:team/project.git"><small class="muted">{{ t('workflows.gitURLHelp') }}</small></label><label>{{ t('workflows.branch') }}<input v-model="gitForm.branch" required></label><label>{{ t('workflows.gitAuthentication') }}<select v-model="gitForm.authentication"><option value="none">{{ t('workflows.gitPublic') }}</option><option value="basic">{{ t('workflows.gitAccount') }}</option><option value="ssh">SSH Private Key</option></select></label><template v-if="gitForm.authentication === 'basic'"><label>{{ t('workflows.gitUsername') }}<input v-model="gitForm.username" required autocomplete="username"></label><label>{{ t('workflows.gitPassword') }}<input v-model="gitForm.password" type="password" required autocomplete="new-password"></label></template><template v-if="gitForm.authentication === 'ssh'"><label class="full">{{ t('workflows.privateKey') }}<textarea v-model="gitForm.ssh_private_key" rows="6" required placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"></textarea></label><label class="full">{{ t('workflows.sshConfig') }}<textarea v-model="gitForm.ssh_config" name="ssh-config" rows="8" :placeholder="t('workflows.sshConfigPlaceholder')"></textarea><small class="muted">{{ t('workflows.sshConfigHelp') }}</small></label></template><div class="full git-config-list"><div class="section-heading"><strong>Git config</strong><button type="button" class="button ghost" @click="addGitConfig">＋ {{ t('common.new') }}</button></div><div v-for="(entry, index) in gitForm.config" :key="index" class="inline-fields"><input v-model="entry.key" placeholder="user.name" required><input v-model="entry.value" :placeholder="t('settings.value')" required><button type="button" class="text-button" @click="removeGitConfig(index)">×</button></div><small class="muted">user.name, user.email, core.autocrlf, core.filemode, pull.rebase, init.defaultBranch</small></div><button type="button" class="button primary" :disabled="savingGit" @click="saveGitSource">{{ savingGit ? t('common.saving') : t('workflows.cloneRepository') }}</button></div></details>
        <section class="danger-zone"><h3>{{ t('workflows.deleteTitle') }}</h3><p>{{ t('workflows.deleteDescription') }}</p><el-button type="danger" @click="confirmWorkflowDelete = true">{{ t('common.delete') }}</el-button></section>
      </form>
      </template>
    </template>
  </section>
  <el-dialog :model-value="Boolean(preview)" width="min(820px, calc(100vw - 32px))" align-center @close="preview = undefined"><template #header><h2>{{ preview?.path }}</h2></template><pre class="preview-content">{{ preview?.content }}</pre></el-dialog>
  <ConfirmDialog :open="confirmWorkflowDelete" :title="t('workflows.deleteTitle')" :message="workflow ? `${t('common.delete')} “${workflow.name}”?` : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger @cancel="confirmWorkflowDelete = false" @confirm="removeWorkflow" />
</template>
