<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ArrowUp, Copy, Eye, EyeOff, FileText, Folder } from "@lucide/vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { formatDuration, type SupportedLocale } from "../i18n";
import { ApiError, platformApiKey, runtimeEngineDisplayName, type Artifact, type Expert, type ExpertTeam, type GitSourceInput, type KnowledgeBase, type Run, type RunEvent, type RuntimeEngineStatus, type Workflow, type WorkflowInput, type WorkspaceEntry } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import ConversationComposer from "../components/ConversationComposer.vue";
import ConversationThread from "../components/ConversationThread.vue";
import ExecutionStatusBar from "../components/ExecutionStatusBar.vue";
import type { ComposerSubmission } from "../conversationDraft";
import { cliAuthorizationRequestFromEvents } from "../cliAuthorization";
import type { ConversationActivityKind, ConversationMessage } from "../conversationThread";

type Tab = "artifacts" | "workspace" | "history" | "settings";
const api = inject(platformApiKey)!;
const route = useRoute(); const router = useRouter(); const { t, locale } = useI18n();
const workflowID = computed(() => String(route.params.workflowId));
const origin = window.location.origin;
const runConversationElement = ref<HTMLElement>();
const runComposerLayer = ref<HTMLElement>();
const runComposerClearance = ref(154);
const tab = ref<Tab>((route.query.tab as Tab) || "artifacts"); const workflow = ref<Workflow>(); const experts = ref<Expert[]>([]); const expertTeams = ref<ExpertTeam[]>([]); const knowledgeBases = ref<KnowledgeBase[]>([]); const runs = ref<Run[]>([]); const selectedRun = ref<Run>(); const conversationRuns = ref<Run[]>([]); const runEvents = ref<RunEvent[]>([]); const runEventsByID = ref<Record<string, RunEvent[]>>({}); const eventRunID = ref(""); const streamingRunID = ref(""); const revealedRunOutput = ref(""); const sendingFollowUp = ref(false); const artifacts = ref<Artifact[]>([]); const entries = ref<WorkspaceEntry[]>([]); const workspacePath = ref(""); const loading = ref(true); const error = ref(""); const running = ref(false); const preview = ref<{ path: string; content: string }>(); const credential = ref<{ api_key: string; api_secret: string }>();
const credentialError = ref("");
const revealApiSecret = ref(false);
const nowMS = ref(Date.now());
const notice = ref(""); const confirmWorkflowDelete = ref(false); const savingGit = ref(false);
const integrationGuideOpen = ref(false);
type CopyTarget = "api_key" | "api_secret" | "token" | "run" | "stream" | "full";
const copiedTarget = ref<CopyTarget>();
let copyResetTimer: ReturnType<typeof setTimeout> | undefined;
const gitForm = ref<GitSourceInput>({ url: "", branch: "main", authentication: "none", ssh_config: "", config: [] });
const editingGitCredential = ref(false);
const gitCredentialSaved = computed(() => Boolean(workflow.value?.git_source?.credential_configured && workflow.value.git_source.authentication === gitForm.value.authentication));
const showGitCredentialInput = computed(() => !gitCredentialSaved.value || editingGitCredential.value);
const gitError = ref(""); const gitNotice = ref(""); const gitFeedback = ref<HTMLElement>();
const settingsForm = ref<WorkflowInput>({ name: "", goal: "", environment: [], knowledge_base_ids: [] });
const selectedKnowledgeBaseIDs = computed<string[]>({
  get: () => settingsForm.value.knowledge_base_ids ?? [],
  set: (value) => { settingsForm.value.knowledge_base_ids = [...new Set(value)]; },
});
const tabs: Tab[] = ["artifacts", "workspace", "history", "settings"];
const fileArtifacts = computed(() => artifacts.value.filter((item) => item.kind === "file"));
const latestConversationRun = computed(() => conversationRuns.value.at(-1) ?? selectedRun.value);
const activeConversationRun = computed(() => conversationRuns.value.find((item) => item.state === "queued" || item.state === "running" || item.state === "waiting_for_user"));
const statusConversationRun = computed(() => activeConversationRun.value ?? latestConversationRun.value);
const statusConversationModel = computed(() => statusConversationRun.value?.expert_stages?.at(-1)?.provider_model_name);
const conversationElapsed = computed(() => conversationRuns.value.reduce((total, item) => {
  const stored = Number.isFinite(item.elapsed_ms) ? Math.max(0, item.elapsed_ms) : 0;
  if (item.state !== "queued" && item.state !== "running") return total + stored;
  const started = Date.parse(item.started_at || item.queued_at);
  return total + (Number.isFinite(started) ? Math.max(stored, nowMS.value - started) : stored);
}, 0));
const currentExpertStage = computed(() => [...runEvents.value].reverse().find((event) => event.type === "expert.stage.updated")?.payload);
const cliAuthorizationRequest = computed(() => cliAuthorizationRequestFromEvents(runEvents.value));
function summarizeRuntimeActivities(events: RunEvent[]) {
  const activities: Array<{ sequence: number; label: string; historyLabel: string; detail: string; kind: ConversationActivityKind; state: "running" | "completed"; toolCallCount?: number; fileChangeCount?: number }> = [];
  for (const event of events) {
    const activity = runtimeActivity(event);
    if (!activity) continue;
    const previous = activities.at(-1);
    const pendingTool = event.type === "command.completed" ? [...activities].reverse().find((item) => item.kind === "tool" && item.state === "running" && item.detail === activity.detail) : undefined;
    if (pendingTool) Object.assign(pendingTool, activity, { sequence: event.sequence });
    else if (previous?.label === activity.label && previous.detail === activity.detail) previous.sequence = event.sequence;
    else activities.push({ sequence: event.sequence, ...activity });
  }
  return activities.sort((left, right) => left.sequence - right.sequence).slice(-8);
}
const conversationMessages = computed<ConversationMessage[]>(() => conversationRuns.value.flatMap((turn, index) => {
  const input = runInputText(turn, index);
  const output = runOutput(turn);
  const pending = isActiveRun(turn);
  const streaming = turn.id === streamingRunID.value;
  const turnEvents = runEventsByID.value[turn.id] ?? (turn.id === eventRunID.value ? runEvents.value : []);
  const turnActivities = summarizeRuntimeActivities(turnEvents);
  const activity = streaming ? turnActivities.at(-1) : undefined;
  return [
    { id: `${turn.id}:user`, role: "user", content: input, copyText: input, state: "succeeded", timestamp: turn.queued_at, attachments: turn.attachments },
    {
      id: turn.id,
      role: "assistant",
      content: output,
      copyText: output,
      state: turn.state,
      stateLabel: stateLabel(turn.state),
      timestamp: turn.ended_at || turn.queued_at,
      elapsedMs: turn.elapsed_ms,
      error: turn.error,
      pending,
      streaming,
      progressTitle: pending ? (turn.state === "waiting_for_user" ? t("common.waitingForUser") : t("sessions.thinking")) : undefined,
      progressDetail: pending ? (turn.state === "queued" && turn.queue_position ? `${t("workflows.queuePosition")}: ${turn.queue_position}` : streaming && currentExpertStage.value ? `${currentExpertStage.value.position}/${currentExpertStage.value.total || ""} · ${currentExpertStage.value.expert_name}` : t("sessions.progress.thinking")) : undefined,
      currentActivity: activity ? { id: activity.sequence, label: activity.label, detail: activity.detail } : undefined,
      activities: turnActivities.map((item) => ({ id: item.sequence, label: item.historyLabel, detail: item.detail, kind: item.kind, toolCallCount: item.toolCallCount, fileChangeCount: item.fileChangeCount, state: item.state, items: [{ id: item.sequence, label: item.historyLabel, detail: item.detail }] })),
      executionEvidenceCounts: runtimeEvidenceCounts(turnEvents),
      stages: turn.expert_stages,
      creditConsumption: turn.credit_consumption,
      artifacts: runArtifacts(turn),
    },
  ];
}));
watch(tab, (value) => {
  void router.replace({ query: { ...route.query, tab: value } });
  if (value === "workspace" && workflow.value && !workflow.value.deleted) void loadDirectory(workspacePath.value);
  if (value === "settings" && workflow.value && !workflow.value.deleted) void loadCredential();
  if (!loading.value && (value === "history" || value === "artifacts")) void refreshRuns(value === "artifacts");
});
watch(() => gitForm.value.authentication, () => { clearGitCredential(); editingGitCredential.value = false; });
let runTimer: ReturnType<typeof setInterval> | undefined;
let disposed = false;
let refreshingRuns = false;
let lastRunRefresh = 0;
let artifactsRefreshRequested = false;
let eventController: AbortController | undefined;
let revealTimer: ReturnType<typeof setTimeout> | undefined;
let revealTarget = "";
let runComposerObserver: ResizeObserver | undefined;
onMounted(async () => {
  if (typeof ResizeObserver !== "undefined") runComposerObserver = new ResizeObserver(measureRunComposer);
  window.addEventListener("resize", measureRunComposer);
  document.addEventListener("visibilitychange", resumeRunPolling);
  await refresh();
  if (disposed) return;
  if (tab.value === "settings" && workflow.value && !workflow.value.deleted) await loadCredential();
  lastRunRefresh = Date.now();
  runTimer = setInterval(() => {
    if (!canRefreshRuns()) return;
    nowMS.value = Date.now();
    const active = Boolean(activeConversationRun.value) || runs.value.some(isActiveRun);
    if (Date.now() - lastRunRefresh >= (active ? 1500 : 30_000)) void refreshRuns();
  }, 1500);
});
watch(runComposerLayer, (current, previous) => {
  if (previous) runComposerObserver?.unobserve(previous);
  if (current) runComposerObserver?.observe(current);
  void nextTick(measureRunComposer);
});
onBeforeUnmount(() => {
  disposed = true;
  clearGitCredential();
  if (copyResetTimer) clearTimeout(copyResetTimer);
  if (runTimer) clearInterval(runTimer);
  eventController?.abort();
  stopRunReveal();
  runComposerObserver?.disconnect();
  window.removeEventListener("resize", measureRunComposer);
  document.removeEventListener("visibilitychange", resumeRunPolling);
});
function measureRunComposer() {
  const height = runComposerLayer.value?.getBoundingClientRect().height ?? 0;
  if (height <= 0) return;
  runComposerClearance.value = Math.ceil(height) + 16;
  void scrollConversationToEnd("auto");
}
async function refresh() { loading.value = true; error.value = ""; try { workflow.value = await api.getWorkflow(workflowID.value); settingsForm.value = { name: workflow.value.name, goal: workflow.value.goal, expert_id: workflow.value.expert_id, expert_team_id: workflow.value.expert_team_id, knowledge_base_ids: Array.isArray(workflow.value.knowledge_base_ids) ? [...workflow.value.knowledge_base_ids] : [], environment: workflow.value.environment ?? [], schedule: workflow.value.schedule }; editingGitCredential.value = false; const source = workflow.value.git_source; gitForm.value = source ? { url: source.url, branch: source.branch, authentication: source.authentication || "none", username: source.username, ssh_config: source.ssh_config ?? "", config: source.config ?? [] } : { url: "", branch: "main", authentication: "none", ssh_config: "", config: [] }; const knowledgeBasesRequest = typeof api.listKnowledgeBases === "function" ? api.listKnowledgeBases() : Promise.resolve([]); [experts.value, expertTeams.value, knowledgeBases.value, runs.value, artifacts.value] = await Promise.all([api.listExperts(), api.listExpertTeams(), knowledgeBasesRequest, api.listRuns(workflowID.value), api.listArtifacts(workflowID.value)]); if (!workflow.value.deleted) await loadDirectory(""); else if (tab.value === "workspace" || tab.value === "settings") tab.value = "history"; } catch { error.value = t("errors.generic"); } finally { loading.value = false; } }
async function loadCredential() {
  credential.value = undefined;
  credentialError.value = "";
  revealApiSecret.value = false;
  if (!workflow.value?.api_credential_configured) return;
  try {
    credential.value = await api.getWorkflowCredential(workflowID.value);
  } catch (cause) {
    if (cause instanceof ApiError && cause.code === "workflow_credential_secret_unavailable") credentialError.value = t("workflows.credentialUnavailable");
  }
}
function isActiveRun(item: Run) { return item.state === "queued" || item.state === "running" || item.state === "waiting_for_user"; }
function canRefreshRuns() { return !disposed && !loading.value && document.visibilityState !== "hidden" && (Boolean(selectedRun.value) || tab.value === "history" || tab.value === "artifacts"); }
function resumeRunPolling() { if (canRefreshRuns()) void refreshRuns(tab.value === "artifacts"); }
function runRevision(items: Run[]) { return JSON.stringify(items.map((item) => [item.id, item.turn_number, item.state, item.ended_at])); }
async function refreshRuns(refreshArtifacts = false) {
  if (!canRefreshRuns()) return;
  artifactsRefreshRequested ||= refreshArtifacts;
  if (refreshingRuns) return;
  refreshingRuns = true;
  const id = workflowID.value;
  const selectedID = selectedRun.value?.id;
  try {
    const latest = await api.listRuns(id);
    if (!canRefreshRuns() || id !== workflowID.value) return;
    artifactsRefreshRequested ||= runRevision(latest) !== runRevision(runs.value);
    runs.value = latest;
    if (selectedID && selectedRun.value?.id === selectedID) {
      selectedRun.value = latest.find((item) => item.id === selectedID) ?? selectedRun.value;
      const turns = await api.listRunTurns(id, selectedID);
      if (!canRefreshRuns() || selectedRun.value?.id !== selectedID) return;
      conversationRuns.value = turns;
    }
    if (artifactsRefreshRequested && canRefreshRuns()) {
      const latestArtifacts = await api.listArtifacts(id);
      if (!canRefreshRuns() || id !== workflowID.value) return;
      artifacts.value = latestArtifacts;
      artifactsRefreshRequested = false;
    }
  } catch { /* Keep the last usable projection during a transient poll failure. */ }
  finally { refreshingRuns = false; lastRunRefresh = Date.now(); }
}
async function runNow() { running.value = true; try { const created = await api.runWorkflow(workflowID.value); tab.value = "history"; runs.value = [created, ...runs.value.filter((item) => item.id !== created.id)]; await openRun(created); } catch { error.value = t("errors.generic"); } finally { running.value = false; } }
async function loadDirectory(path: string) { const result = await api.listWorkspace(workflowID.value, path); entries.value = result.items ?? []; workspacePath.value = path; }
async function openEntry(entry: WorkspaceEntry) { if (entry.directory) { await loadDirectory(entry.path); return; } if (entry.size > 1024 * 1024) { await downloadEntry(entry); return; } const file = await api.getWorkspaceFile(workflowID.value, entry.path); if (!file.content_type.startsWith("text/") && !file.content_type.includes("json") && !file.content_type.includes("xml")) { await downloadEntry(entry); return; } preview.value = { path: file.path, content: decodeBase64(file.content) }; }
async function downloadEntry(entry: WorkspaceEntry) { const blob = await api.downloadWorkspaceFile(workflowID.value, entry.path); const url = URL.createObjectURL(blob); const anchor = document.createElement("a"); anchor.href = url; anchor.download = entry.name; anchor.click(); URL.revokeObjectURL(url); }
function addGitConfig() { gitForm.value.config.push({ key: "", value: "" }); }
function removeGitConfig(index: number) { gitForm.value.config.splice(index, 1); }
function clearGitCredential() { gitForm.value.ssh_private_key = undefined; gitForm.value.password = undefined; }
function editGitCredential() { clearGitCredential(); editingGitCredential.value = true; }
async function saveGitSource() {
  if (savingGit.value) return;
  savingGit.value = true; gitError.value = ""; gitNotice.value = "";
  try {
    workflow.value = await api.configureWorkflowGitSource(workflowID.value, gitForm.value);
    clearGitCredential(); editingGitCredential.value = false;
    gitNotice.value = t("workflows.gitSaved");
    await loadDirectory("");
  } catch (cause) {
    const codes = ["git_source_invalid", "git_credentials_required", "git_workspace_not_empty", "git_server_unavailable", "git_ssh_host_untrusted", "git_ssh_key_invalid", "git_authentication_failed", "git_branch_not_found", "git_repository_unavailable", "git_connection_failed", "git_repository_too_large", "git_clone_failed"];
    const code = cause instanceof ApiError && codes.includes(cause.code) ? cause.code : "git_clone_failed";
    gitError.value = t(`workflows.gitErrors.${code}`);
  } finally {
    savingGit.value = false;
    await nextTick(); gitFeedback.value?.scrollIntoView?.({ block: "nearest", behavior: "smooth" });
  }
}
async function saveSettings() { if (!workflow.value) return; try { workflow.value = await api.updateWorkflow(workflowID.value, settingsForm.value, workflow.value.version); await refresh(); } catch { error.value = t("errors.conflict"); } }
async function generateCredential() { try { credential.value = await api.generateWorkflowCredential(workflowID.value); credentialError.value = ""; copiedTarget.value = undefined; revealApiSecret.value = false; } catch { error.value = t("errors.generic"); } }
const tokenCommand = computed(() => `JWT_TOKEN=$(curl -sS -u "$API_KEY:$API_SECRET" -X POST ${origin}/api/v1/workflows/${workflowID.value}/api-token | jq -r '.jwt_token')`);
const runCommand = computed(() => `RUN_ID=$(curl -sS -H "Authorization: Bearer $JWT_TOKEN" -H 'Idempotency-Key: unique-request' -H 'Content-Type: application/json' -d '{"text_input":"Run now"}' ${origin}/api/v1/workflows/${workflowID.value}/runs | jq -r '.id')`);
const streamCommand = computed(() => `curl -N -H "Authorization: Bearer $JWT_TOKEN" -H "Accept: text/event-stream" ${origin}/api/v1/workflows/${workflowID.value}/runs/$RUN_ID/events`);
const fullOutputCommand = computed(() => `curl -sS -H "Authorization: Bearer $JWT_TOKEN" ${origin}/api/v1/workflows/${workflowID.value}/runs/$RUN_ID`);
async function copyValue(value: string, target: CopyTarget) {
  try {
    await navigator.clipboard.writeText(value);
    copiedTarget.value = target;
    if (copyResetTimer) clearTimeout(copyResetTimer);
    copyResetTimer = setTimeout(() => { copiedTarget.value = undefined; }, 1800);
  } catch { error.value = t("errors.copy"); }
}
function copyCredential(target: "api_key" | "api_secret") {
  if (!credential.value) return;
  const value = target === "api_key" ? credential.value.api_key : credential.value.api_secret;
  void copyValue(value, target);
}
function maskCredential(value: string) {
  if (value.length <= 8) return "*".repeat(value.length);
  return `${value.slice(0, 4)}${"*".repeat(Math.max(4, value.length - 8))}${value.slice(-4)}`;
}
function copyIntegrationCommand(value: string, target: "token" | "run" | "stream" | "full") { void copyValue(value, target); }
async function removeWorkflow() { if (!workflow.value) return; await api.deleteWorkflow(workflowID.value); confirmWorkflowDelete.value = false; await router.push("/workflows"); }
async function cancelRun(item: Run) { const turns = await api.listRunTurns(workflowID.value, item.id); const active = turns.find((turn) => turn.state === "queued" || turn.state === "running" || turn.state === "waiting_for_user"); if (active) await api.cancelRun(workflowID.value, active.id); runs.value = await api.listRuns(workflowID.value); }
async function rerun(item: Run) { const turns = await api.listRunTurns(workflowID.value, item.id); const latest = turns.at(-1); if (latest) await api.rerunWorkflow(workflowID.value, latest.id); runs.value = await api.listRuns(workflowID.value); }
async function openRun(item: Run) {
	eventController?.abort();
	stopRunReveal();
	selectedRun.value = item;
	conversationRuns.value = await api.listRunTurns(workflowID.value, item.id);
	runEvents.value = [];
	runEventsByID.value = Object.fromEntries(conversationRuns.value.map((turn) => [turn.id, []]));
	eventRunID.value = "";
	await scrollConversationToEnd();
	const active = activeConversationRun.value;
	if (active) void streamConversationTurn(active);
	void loadRunHistoryEvents(conversationRuns.value, item.id);
}
async function loadRunHistoryEvents(turns: Run[], conversationID: string) {
	await Promise.all(turns.filter((turn) => !isActiveRun(turn)).map(async (turn) => {
		const events: RunEvent[] = [];
		try {
			await api.streamRunEvents(workflowID.value, turn.id, (event) => events.push(event));
		} catch {
			return;
		}
		if (selectedRun.value?.id !== conversationID) return;
		runEventsByID.value = { ...runEventsByID.value, [turn.id]: events };
	}));
}
async function streamConversationTurn(item: Run) {
	eventController?.abort();
	eventController = new AbortController();
	streamingRunID.value = item.id;
	eventRunID.value = item.id;
	runEvents.value = [];
	runEventsByID.value = { ...runEventsByID.value, [item.id]: [] };
	stopRunReveal();
	revealedRunOutput.value = "";
	try {
		await api.streamRunEvents(workflowID.value, item.id, handleRunEvent, eventController.signal);
		const completed = await api.getRun(workflowID.value, item.id);
		if (completed.final_text) setRunRevealTarget(completed.final_text);
		else if (completed.final_json) setRunRevealTarget(`\`\`\`json\n${JSON.stringify(completed.final_json, null, 2)}\n\`\`\``);
		await waitForRunReveal(item.id);
		conversationRuns.value = conversationRuns.value.map((turn) => turn.id === completed.id ? completed : turn);
		void refreshRuns(true);
		window.dispatchEvent(new Event("credits-updated"));
	} catch (streamError) {
		if (!(streamError instanceof DOMException && streamError.name === "AbortError")) error.value = t("errors.generic");
	} finally {
		if (streamingRunID.value === item.id) streamingRunID.value = "";
	}
}
function handleRunEvent(event: RunEvent) {
	runEvents.value.push(event);
	const runID = streamingRunID.value;
	if (runID) runEventsByID.value = { ...runEventsByID.value, [runID]: [...(runEventsByID.value[runID] ?? []), event] };
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
async function sendFollowUp(message: ComposerSubmission) {
  if (!selectedRun.value || sendingFollowUp.value) throw new Error("conversation_not_ready");
  const rootID = selectedRun.value.id;
  sendingFollowUp.value = true;
  try {
    const created = await api.continueRunConversation(workflowID.value, rootID, message.content, message.attachmentIDs, undefined, message.input);
    if (selectedRun.value?.id !== rootID) return;
    conversationRuns.value.push(created);
    await scrollConversationToEnd();
    void streamConversationTurn(created);
  } finally { sendingFollowUp.value = false; }
}

async function cancelConversationRun() { const active = activeConversationRun.value; if (!active) return; await api.cancelRun(workflowID.value, active.id); eventController?.abort(); conversationRuns.value = await api.listRunTurns(workflowID.value, selectedRun.value!.id); }
function closeRun() { eventController?.abort(); eventController = undefined; stopRunReveal(); selectedRun.value = undefined; conversationRuns.value = []; runEvents.value = []; runEventsByID.value = {}; eventRunID.value = ""; streamingRunID.value = ""; revealedRunOutput.value = ""; }
function runInputText(item: Run, index: number) { const input = item.text_input || (item.json_input ? JSON.stringify(item.json_input, null, 2) : ""); return index === 0 ? [workflow.value?.goal, input].filter(Boolean).join("\n\n") : input; }
function runOutput(item: Run) { return (item.id === streamingRunID.value ? revealedRunOutput.value : "") || item.final_text || (item.final_json ? `\`\`\`json\n${JSON.stringify(item.final_json, null, 2)}\n\`\`\`` : "") || ""; }
function runArtifacts(item: Run) { return fileArtifacts.value.filter((artifact) => artifact.run_id === item.id); }
function runtimeActivity(event: RunEvent) {
	if (event.type === "runtime.started") return { label: t("sessions.progress.preparing"), historyLabel: t("workflows.runtimePrepared"), detail: typeof event.payload.runtime === "string" ? runtimeEngineDisplayName(event.payload.runtime as RuntimeEngineStatus["name"]) : "", kind: "runtime" as const, state: "completed" as const };
	if (event.type === "reasoning.summary") return { label: t("workflows.reasoningSummary"), historyLabel: t("workflows.reasoningSummary"), detail: typeof event.payload.summary === "string" ? event.payload.summary : "", kind: "reasoning" as const, state: "completed" as const };
	if (event.type === "command.requested") return { label: t("sessions.progress.using_tool"), historyLabel: t("sessions.progress.using_tool"), detail: runtimeCommandDetail(event), kind: "tool" as const, state: "running" as const, toolCallCount: 1 };
	if (event.type === "command.completed") return { label: t("workflows.toolCompleted"), historyLabel: t("workflows.toolCompleted"), detail: runtimeCommandDetail(event), kind: "tool" as const, state: "completed" as const, toolCallCount: 1 };
	if (event.type === "file.changed") return { label: t("workflows.updatingFiles"), historyLabel: t("workflows.updatingFiles"), detail: "", kind: "file" as const, state: "completed" as const, fileChangeCount: 1 };
	if (event.type === "message.delta") return { label: t("workflows.streamingAnswer"), historyLabel: t("workflows.streamingAnswer"), detail: "", kind: "activity" as const, state: "running" as const };
	if (event.type === "message.completed") return { label: t("workflows.answerReady"), historyLabel: t("workflows.answerReady"), detail: "", kind: "activity" as const, state: "completed" as const };
	return undefined;
}
function runtimeCommandDetail(event: RunEvent) {
	if (typeof event.payload.command === "string") return event.payload.command;
	if (typeof event.payload.tool === "string") return event.payload.tool;
	return t("workflows.command");
}
function runtimeEvidenceCounts(events: RunEvent[]) {
	const requested = events.filter((event) => event.type === "command.requested").length;
	return {
		toolCalls: requested || events.filter((event) => event.type === "command.completed").length,
		fileChanges: events.filter((event) => event.type === "file.changed").length,
	};
}
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
      <header class="run-conversation-head"><div><el-button class="back-link" text @click="closeRun">← {{ t('common.back') }}</el-button><h2>{{ t('workflows.conversation') }}</h2><p v-if="latestConversationRun"><span>{{ triggerLabel(selectedRun.trigger) }}</span><span>{{ new Date(latestConversationRun.started_at || latestConversationRun.queued_at).toLocaleString() }}</span></p></div></header>
      <ExecutionStatusBar v-if="statusConversationRun" :state="statusConversationRun.state" :elapsed-ms="conversationElapsed" :model="statusConversationModel" :credit-consumption="statusConversationRun.credit_consumption" :can-stop="Boolean(activeConversationRun)" @stop="cancelConversationRun" />
      <div ref="runConversationElement" class="run-conversation" :style="{ paddingBottom: `${runComposerClearance}px` }">
        <ConversationThread :messages="conversationMessages" :load-attachment="api.getAttachmentDownload" @download-artifact="openArtifact" @attachment-error="error = t('errors.generic')" @copy-error="error = t('errors.copy')" />
      </div>
      <div v-if="!workflow?.deleted" ref="runComposerLayer" class="composer-layer run-composer-layer"><ConversationComposer :key="selectedRun.id" class="run-composer" :scope="{ workflow_id: workflowID, run_id: selectedRun.id }" :authorization-request="cliAuthorizationRequest" :active="Boolean(activeConversationRun)" :submit="sendFollowUp" @stop="cancelConversationRun" /></div>
    </div>
    <template v-else>
      <header class="detail-hero"><el-button class="back-link" text @click="router.push('/workflows')">← {{ t('common.back') }}</el-button><div v-if="workflow"><h2>{{ workflow.name }}</h2></div><el-button v-if="workflow && !workflow.deleted" class="button primary" type="primary" :loading="running" @click="runNow">{{ running ? t('common.running') : '▶ ' + t('workflows.runNow') }}</el-button><el-tag v-else-if="workflow" type="info">{{ t('common.readOnly') }}</el-tag></header>
      <el-skeleton v-if="loading" :rows="10" animated class="page-loading" />
      <template v-else-if="workflow">
      <nav class="tabs"><el-button v-for="item in tabs" :key="item" text :class="{ active: tab === item }" @click="tab = item">{{ t(`workflows.${item}`) }}</el-button></nav>
      <div v-if="tab === 'artifacts'" class="tab-content"><div v-if="!fileArtifacts.length" class="empty-inline"><span>◇</span><p>{{ t('common.empty') }}</p></div><div v-else class="artifact-list"><article v-for="item in fileArtifacts" :key="item.id" role="button" tabindex="0" @click="openArtifact(item)" @keydown.enter="openArtifact(item)"><span class="file-icon" aria-hidden="true"><FileText /></span><div><strong>{{ item.name }}</strong><small>{{ formatFileSize(item.size) }} <template v-if="item.expired">· {{ t('workflows.expired') }}</template></small></div><code>{{ (item.sha256 || '').slice(0, 12) }}</code></article></div></div>
      <div v-if="tab === 'workspace'" class="tab-content"><div class="file-browser"><button v-if="workspacePath" class="file-row" @click="loadDirectory(parentPath())"><span class="file-icon" aria-hidden="true"><ArrowUp /></span><strong>..</strong></button><div v-for="entry in entries" :key="entry.path" class="file-row" role="button" tabindex="0" @click="openEntry(entry)" @keydown.enter="openEntry(entry)"><span class="file-icon" aria-hidden="true"><Folder v-if="entry.directory" /><FileText v-else /></span><strong>{{ entry.name }}</strong><small>{{ entry.directory ? '—' : `${entry.size} B` }}</small><time>{{ new Date(entry.modified_at).toLocaleString() }}</time><button v-if="!entry.directory" class="text-button" :aria-label="t('common.download')" @click.stop="downloadEntry(entry)">↓</button></div><div v-if="!entries.length" class="empty-inline"><span>□</span><p>{{ t('common.empty') }}</p></div></div></div>
      <div v-if="tab === 'history'" class="tab-content"><el-empty v-if="!runs.length" :description="t('workflows.noRuns')" /><div v-else class="run-table"><div class="run-row run-head"><span>{{ t('workflows.started') }}</span><span>{{ t('workflows.trigger') }}</span><span>{{ t('workflows.state') }}</span><span>{{ t('workflows.duration') }}</span><span></span></div><div v-for="item in runs" :key="item.id" class="run-row" role="button" tabindex="0" @click="openRun(item)" @keydown.enter="openRun(item)"><span><strong>{{ new Date(item.started_at || item.queued_at).toLocaleString() }}</strong><small>{{ item.id.slice(0, 8) }}</small></span><span>{{ triggerLabel(item.trigger) }}</span><span><el-tag :type="item.state === 'succeeded' ? 'success' : item.state === 'failed' ? 'danger' : 'primary'" size="small">{{ stateLabel(item.state) }}</el-tag><small v-if="item.state === 'queued' && item.queue_position">{{ t('workflows.queuePosition') }}: {{ item.queue_position }}</small></span><span>{{ formatDuration(item.elapsed_ms, locale as SupportedLocale) }}</span><span class="run-actions"><el-button v-if="item.state === 'queued' || item.state === 'running' || item.state === 'waiting_for_user'" size="small" @click.stop="cancelRun(item)">{{ t('common.cancel') }}</el-button><el-button v-else-if="!workflow.deleted" circle size="small" @click.stop="rerun(item)">↻</el-button></span></div></div></div>
      <form v-if="tab === 'settings' && !workflow.deleted" class="tab-content settings-form" @submit.prevent="saveSettings">
        <details class="settings-section" open><summary><h3>{{ t('workflows.basic') }}</h3></summary><div class="form-grid"><label>{{ t('workflows.name') }}<input v-model="settingsForm.name" required></label><label>{{ t('workflows.expert') }}<select :value="settingsForm.expert_team_id ? `team:${settingsForm.expert_team_id}` : settingsForm.expert_id ? `expert:${settingsForm.expert_id}` : 'none'" @change="setWorkflowSpecialist(($event.target as HTMLSelectElement).value)"><option value="none">{{ t('sessions.noExpert') }}</option><optgroup :label="t('experts.title')"><option v-for="expert in experts" :key="expert.id" :value="`expert:${expert.id}`" :disabled="!expert.available">{{ expert.name }}</option></optgroup><optgroup :label="t('experts.teams')"><option v-for="team in expertTeams" :key="team.id" :value="`team:${team.id}`" :disabled="!team.available">{{ teamSelectionLabel(team) }}</option></optgroup></select></label><label class="full">{{ t('workflows.goal') }}<textarea v-model="settingsForm.goal" rows="7" required></textarea></label></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.knowledgeBases') }}</h3></summary><div class="form-grid"><fieldset class="full knowledge-base-picker"><legend>{{ t('workflows.knowledgeBases') }}</legend><div v-if="knowledgeBases.length" class="knowledge-base-options" role="group" :aria-label="t('workflows.knowledgeBases')"><label v-for="knowledgeBase in knowledgeBases" :key="knowledgeBase.id" class="knowledge-base-option" :class="{ selected: selectedKnowledgeBaseIDs.includes(knowledgeBase.id) }"><input v-model="selectedKnowledgeBaseIDs" type="checkbox" :value="knowledgeBase.id"><span><strong>{{ knowledgeBase.name }}</strong><small>{{ knowledgeBase.visibility === 'public' ? t('knowledgeBases.public') : t('knowledgeBases.private') }}</small></span></label></div><p v-else class="muted knowledge-base-empty">{{ t('common.empty') }}</p><small class="muted">{{ t('workflows.knowledgeBasesHint') }}</small></fieldset></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.execution') }}</h3></summary><div class="form-grid"><div class="full"><div v-for="(variable, index) in settingsForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><button type="button" class="text-button" @click="removeEnvironment(index)">×</button></div><button type="button" class="button ghost" @click="addEnvironment">＋ {{ t('workflows.environment') }}</button></div></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.schedule') }}</h3></summary><button v-if="!settingsForm.schedule" type="button" class="button ghost" @click="enableSchedule">{{ t('workflows.enableSchedule') }}</button><div v-else class="form-grid"><label><input v-model="settingsForm.schedule.enabled" type="checkbox"> {{ t('common.enabled') }}</label><label>{{ t('workflows.frequency') }}<select v-model="settingsForm.schedule.frequency"><option value="hourly">{{ t('workflows.hourly') }}</option><option value="daily">{{ t('workflows.daily') }}</option><option value="weekly">{{ t('workflows.weekly') }}</option></select></label><label>{{ t('workflows.hour') }}<input v-model.number="settingsForm.schedule.hour" type="number" min="0" max="23"></label><label>{{ t('workflows.minute') }}<input v-model.number="settingsForm.schedule.minute" type="number" min="0" max="59"></label><label v-if="settingsForm.schedule.frequency === 'weekly'">{{ t('workflows.weekday') }}<input v-model.number="settingsForm.schedule.weekday" type="number" min="0" max="6"></label><label>{{ t('workflows.timezone') }}<input v-model="settingsForm.schedule.timezone"></label></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.apiCredential') }}</h3></summary><p class="muted">{{ t('workflows.apiTokenDescription') }}</p><div class="api-credential-actions"><button type="button" class="button ghost" @click="generateCredential">{{ workflow.api_credential_configured ? t('workflows.regenerate') : t('workflows.generate') }}</button><button type="button" class="button ghost" @click="integrationGuideOpen = true">{{ t('workflows.howToIntegrate') }}</button></div><p v-if="credentialError" class="muted credential-warning">{{ credentialError }}</p><div v-if="credential" class="secret-reveal"><div class="credential-row"><strong>{{ t('workflows.apiKeyLabel') }}</strong><code>{{ credential.api_key }}</code><div class="credential-row-actions"><button type="button" class="credential-icon-button" :aria-label="copiedTarget === 'api_key' ? t('common.copied') : t('common.copy')" :title="copiedTarget === 'api_key' ? t('common.copied') : t('common.copy')" @click="copyCredential('api_key')"><Copy :size="16" :stroke-width="1.8" aria-hidden="true" /></button></div></div><div class="credential-row"><strong>{{ t('workflows.apiSecretLabel') }}</strong><code>{{ revealApiSecret ? credential.api_secret : maskCredential(credential.api_secret) }}</code><div class="credential-row-actions"><button type="button" class="credential-icon-button" :aria-label="revealApiSecret ? t('workflows.hideSecret') : t('workflows.showSecret')" :title="revealApiSecret ? t('workflows.hideSecret') : t('workflows.showSecret')" @click="revealApiSecret = !revealApiSecret"><EyeOff v-if="revealApiSecret" :size="16" :stroke-width="1.8" aria-hidden="true" /><Eye v-else :size="16" :stroke-width="1.8" aria-hidden="true" /></button><button type="button" class="credential-icon-button" :aria-label="copiedTarget === 'api_secret' ? t('common.copied') : t('common.copy')" :title="copiedTarget === 'api_secret' ? t('common.copied') : t('common.copy')" @click="copyCredential('api_secret')"><Copy :size="16" :stroke-width="1.8" aria-hidden="true" /></button></div></div></div></details>
        <details class="settings-section" open><summary><h3>{{ t('workflows.gitSource') }}</h3></summary><div class="form-grid git-settings"><label class="full">{{ t('workflows.gitURL') }}<input v-model="gitForm.url" type="text" required spellcheck="false" autocapitalize="off" placeholder="git@github.com:team/project.git"><small class="muted">{{ t('workflows.gitURLHelp') }}</small></label><label>{{ t('workflows.branch') }}<input v-model="gitForm.branch" required></label><label>{{ t('workflows.gitAuthentication') }}<select v-model="gitForm.authentication"><option value="none">{{ t('workflows.gitPublic') }}</option><option value="basic">{{ t('workflows.gitAccount') }}</option><option value="ssh">SSH Private Key</option></select></label><template v-if="gitForm.authentication === 'basic'"><label>{{ t('workflows.gitUsername') }}<input v-model="gitForm.username" required autocomplete="username"></label><label v-if="showGitCredentialInput">{{ t(editingGitCredential ? 'workflows.gitPasswordDraft' : 'workflows.gitPassword') }}<input v-model="gitForm.password" type="password" :disabled="savingGit" required autocomplete="new-password"></label></template><template v-if="gitForm.authentication === 'ssh'"><label v-if="showGitCredentialInput" class="full">{{ t(editingGitCredential ? 'workflows.gitPrivateKeyDraft' : 'workflows.privateKey') }}<textarea v-model="gitForm.ssh_private_key" rows="6" :disabled="savingGit" required autocomplete="off" autocapitalize="off" spellcheck="false" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"></textarea></label><label class="full">{{ t('workflows.sshConfig') }}<textarea v-model="gitForm.ssh_config" name="ssh-config" rows="8" :placeholder="t('workflows.sshConfigPlaceholder')"></textarea><small class="muted">{{ t('workflows.sshConfigHelp') }}</small></label></template><div v-if="gitForm.authentication !== 'none' && !showGitCredentialInput" class="full git-credential-status"><p class="muted" role="status">{{ t('workflows.gitCredentialSaved') }}</p><el-button :disabled="savingGit" @click="editGitCredential">{{ t('workflows.replaceGitCredential') }}</el-button></div><div class="full git-config-list"><div class="section-heading"><strong>Git config</strong><button type="button" class="button ghost" @click="addGitConfig">＋ {{ t('common.new') }}</button></div><div v-for="(entry, index) in gitForm.config" :key="index" class="inline-fields"><input v-model="entry.key" placeholder="user.name" required><input v-model="entry.value" :placeholder="t('settings.value')" required><button type="button" class="text-button" @click="removeGitConfig(index)">×</button></div><small class="muted">user.name, user.email, core.autocrlf, core.filemode, pull.rebase, init.defaultBranch</small></div><div v-if="gitError || gitNotice" ref="gitFeedback" class="full git-feedback"><el-alert v-if="gitError" :title="gitError" type="error" show-icon :closable="false" /><el-alert v-else :title="gitNotice" type="success" show-icon :closable="false" /></div><button type="button" class="button primary" :disabled="savingGit" @click="saveGitSource">{{ savingGit ? t('common.saving') : t('workflows.cloneRepository') }}</button></div></details>
        <div class="settings-actions-bottom"><el-button type="danger" @click="confirmWorkflowDelete = true">{{ t('common.delete') }}</el-button><el-button native-type="submit" type="primary">{{ t('common.save') }}</el-button></div>
      </form>
      </template>
    </template>
  </section>
  <el-dialog :model-value="Boolean(preview)" width="min(820px, calc(100vw - 32px))" align-center @close="preview = undefined"><template #header><h2>{{ preview?.path }}</h2></template><pre class="preview-content">{{ preview?.content }}</pre></el-dialog>
  <el-dialog v-model="integrationGuideOpen" class="integration-guide-dialog" width="min(720px, calc(100vw - 32px))" align-center><template #header><h2>{{ t('workflows.integrationGuideTitle') }}</h2></template><div class="integration-guide"><section><h3>{{ t('workflows.integrationStepToken') }}</h3><p class="muted">{{ t('workflows.integrationTokenHint') }}</p><div class="integration-code"><pre><code>{{ tokenCommand }}</code></pre><button type="button" class="text-button" @click="copyIntegrationCommand(tokenCommand, 'token')">{{ copiedTarget === 'token' ? t('common.copied') : t('common.copy') }}</button></div></section><section><h3>{{ t('workflows.integrationStepInvoke') }}</h3><p class="muted">{{ t('workflows.integrationRunHint') }} {{ t('workflows.integrationIdempotencyHint') }}</p><div class="integration-code"><pre><code>{{ runCommand }}</code></pre><button type="button" class="text-button" @click="copyIntegrationCommand(runCommand, 'run')">{{ copiedTarget === 'run' ? t('common.copied') : t('common.copy') }}</button></div></section><section><h3>{{ t('workflows.integrationStepStream') }}</h3><p class="muted">{{ t('workflows.integrationStreamHint') }}</p><div class="integration-code"><pre><code>{{ streamCommand }}</code></pre><button type="button" class="text-button" @click="copyIntegrationCommand(streamCommand, 'stream')">{{ copiedTarget === 'stream' ? t('common.copied') : t('common.copy') }}</button></div></section><section><h3>{{ t('workflows.integrationStepFullOutput') }}</h3><p class="muted">{{ t('workflows.integrationFullOutputHint') }}</p><div class="integration-code"><pre><code>{{ fullOutputCommand }}</code></pre><button type="button" class="text-button" @click="copyIntegrationCommand(fullOutputCommand, 'full')">{{ copiedTarget === 'full' ? t('common.copied') : t('common.copy') }}</button></div></section></div><template #footer><el-button @click="integrationGuideOpen = false">{{ t('common.close') }}</el-button></template></el-dialog>
  <ConfirmDialog :open="confirmWorkflowDelete" :title="t('workflows.deleteTitle')" :message="workflow ? `${t('common.delete')} “${workflow.name}”?` : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger @cancel="confirmWorkflowDelete = false" @confirm="removeWorkflow" />
</template>
