<script setup lang="ts">
import { computed, inject, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  ApiError, platformApiKey, type AgentRelease, type CodingTask, type CodingTaskSession,
  type Artifact, type CodingTaskLaunchOption, type RepositoryBinding, type Run, type RunApproval, type RunEvent,
} from "../api/client";
import { authContextKey } from "../auth/session";

const injectedApi = inject(platformApiKey);
const injectedAuth = inject(authContextKey);
if (!injectedApi || !injectedAuth) throw new Error("Conversation Workspace dependencies are required");
const api = injectedApi;
const auth = injectedAuth;
const route = useRoute();
const router = useRouter();
const { t, d } = useI18n();

const tasks = ref<CodingTask[]>([]);
const bindings = ref<RepositoryBinding[]>([]);
const releases = ref<AgentRelease[]>([]);
const launchOptions = ref<CodingTaskLaunchOption[]>([]);
const launchPrerequisite = ref("");
const session = ref<CodingTaskSession>();
const runs = ref<Run[]>([]);
const selectedRunID = ref("");
const runEvents = ref<RunEvent[]>([]);
const artifacts = ref<Artifact[]>([]);
const runApprovals = ref<RunApproval[]>([]);
const decisionReasons = reactive<Record<string, string>>({});
const streamState = ref<"idle" | "connecting" | "live" | "reconnecting" | "complete" | "error">("idle");
const streamError = ref("");
const artifactError = ref("");
const loading = ref(true);
const detailLoading = ref(false);
const saving = ref(false);
const controlling = ref(false);
const error = ref<ApiError>();
const notice = ref("");
const form = reactive({ source: "text", bindingID: "", releaseID: "", title: "", requestText: "", issueTitle: "", issueBody: "", issueURL: "" });
const intents = new Map<string, { fingerprint: string; key: string }>();
let refreshSequence = 0;
let detailSequence = 0;
let evidenceController: AbortController | undefined;

const teamID = computed(() => typeof route.query.team === "string" ? route.query.team : "");
const selectedTaskID = computed(() => typeof route.query.task === "string" ? route.query.task : "");
const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
function hasRole(roles: string[]) {
  return (currentUser.value?.role_grants ?? []).some((grant) => (!grant.team_id || grant.team_id === teamID.value) && roles.includes(grant.role ?? ""));
}
const canUse = computed(() => hasRole(["agent_user", "agent_builder", "platform_administrator"]));
const canDecideApproval = computed(() => hasRole(["agent_user", "agent_builder", "platform_administrator"]));
const canInterruptOrCancel = computed(() => hasRole(["agent_user", "agent_builder", "platform_administrator", "run_operator"]));
const canResume = computed(() => hasRole(["agent_user", "agent_builder", "platform_administrator"]));
const availableBindings = computed(() => bindings.value.filter((binding) => launchOptions.value.some((option) => option.repository_binding_id === binding.id)));
const launchableReleases = computed(() => releases.value.filter((release) => {
  const option = launchOptions.value.find((item) => item.agent_release_id === release.id && item.repository_binding_id === release.repository_binding_id);
  return Boolean(option)
    && (!form.bindingID || release.repository_binding_id === form.bindingID);
}));
const selectedTask = computed(() => tasks.value.find((task) => task.id === selectedTaskID.value));
const selectedRelease = computed(() => releases.value.find((release) => release.id === selectedTask.value?.agent_release_id));
const selectedRun = computed(() => runs.value.find((run) => run.id === selectedRunID.value) ?? runs.value[0]);
const prerequisite = computed(() => {
  if (launchOptions.value.length === 0) return launchPrerequisite.value || "release";
  return "";
});

watch(teamID, () => {
  tasks.value = []; bindings.value = []; releases.value = []; launchOptions.value = []; launchPrerequisite.value = "";
  session.value = undefined; runs.value = []; form.bindingID = ""; form.releaseID = "";
  void refresh();
}, { immediate: true });
watch(selectedTaskID, () => void loadSelectedTask());
watch(() => selectedRun.value?.id, (runID) => {
  runApprovals.value = [];
  if (runID) { void loadRunEvidence(runID); void loadRunApprovals(runID); }
});
onBeforeUnmount(() => evidenceController?.abort());
watch(() => form.bindingID, () => {
  if (!launchableReleases.value.some((release) => release.id === form.releaseID)) form.releaseID = launchableReleases.value[0]?.id ?? "";
});

async function refresh() {
  const requestedTeam = teamID.value;
  if (!requestedTeam) return;
  const sequence = ++refreshSequence;
  loading.value = true; error.value = undefined;
  try {
    const [taskValues, bindingValues, agentValues, launchCatalog] = await Promise.all([
      api.listCodingTasks(requestedTeam), api.listRepositoryBindings(requestedTeam), api.listAgents(requestedTeam), api.listCodingTaskLaunchOptions(requestedTeam),
    ]);
    const releaseValues = (await Promise.all(agentValues.filter((agent) => agent.id).map((agent) => api.listAgentReleases(agent.id!, requestedTeam)))).flat();
    if (sequence !== refreshSequence || teamID.value !== requestedTeam) return;
    tasks.value = taskValues; bindings.value = bindingValues; releases.value = releaseValues;
    launchOptions.value = launchCatalog.items; launchPrerequisite.value = launchCatalog.prerequisite;
    form.bindingID = availableBindings.value.some((binding) => binding.id === form.bindingID) ? form.bindingID : availableBindings.value[0]?.id ?? "";
    form.releaseID = launchableReleases.value.some((release) => release.id === form.releaseID) ? form.releaseID : launchableReleases.value[0]?.id ?? "";
    if (selectedTaskID.value && !tasks.value.some((task) => task.id === selectedTaskID.value)) {
      tasks.value.unshift(await api.getCodingTask(selectedTaskID.value, requestedTeam));
    }
    await loadSelectedTask();
  } catch (reason) {
    if (sequence === refreshSequence) error.value = asApiError(reason);
  } finally {
    if (sequence === refreshSequence) loading.value = false;
  }
}

async function loadSelectedTask() {
  const taskID = selectedTaskID.value;
  const requestedTeam = teamID.value;
  const sequence = ++detailSequence;
  session.value = undefined; runs.value = [];
  selectedRunID.value = ""; runEvents.value = []; artifacts.value = []; runApprovals.value = []; evidenceController?.abort();
  if (!taskID || !requestedTeam) return;
  detailLoading.value = true;
  try {
    const [sessionValue, runValues] = await Promise.all([api.getCodingTaskSession(taskID, requestedTeam), api.listRuns(requestedTeam, taskID)]);
    if (sequence === detailSequence && selectedTaskID.value === taskID && teamID.value === requestedTeam) {
      session.value = sessionValue; runs.value = runValues; selectedRunID.value = runValues[0]?.id ?? "";
    }
  } catch (reason) {
    if (sequence === detailSequence) error.value = asApiError(reason);
  } finally {
    if (sequence === detailSequence) detailLoading.value = false;
  }
}

async function loadRunApprovals(runID: string) {
  try {
    const values = await api.listRunApprovals(runID);
    if (selectedRun.value?.id === runID) runApprovals.value = values;
  } catch (reason) {
    if (selectedRun.value?.id === runID) error.value = asApiError(reason);
  }
}

async function refreshSelectedRun(runID: string) {
  const value = await api.getRun(runID);
  const index = runs.value.findIndex((run) => run.id === runID);
  if (index >= 0) runs.value.splice(index, 1, value);
  await loadRunApprovals(runID);
}

async function loadRunEvidence(runID: string) {
  evidenceController?.abort(); const controller = new AbortController(); evidenceController = controller;
  runEvents.value = []; artifacts.value = []; streamError.value = ""; artifactError.value = ""; streamState.value = "connecting";
  try { artifacts.value = await api.listRunArtifacts(runID, controller.signal); }
  catch (reason) {
    if (!controller.signal.aborted) artifactError.value = asApiError(reason).code || "artifact_query_failed";
  }
  let cursor = 0;
  let missingTerminalRetries = 0;
  while (!controller.signal.aborted && selectedRun.value?.id === runID) {
    try {
      const connectionCursor = cursor;
      const result = await api.streamRunEvents(runID, cursor, (event) => {
        if (!runEvents.value.some((current) => current.sequence === event.sequence)) runEvents.value.push(event);
        cursor = event.sequence; streamState.value = "live";
      }, controller.signal);
      cursor = result.cursor;
      if (result.terminal) { streamState.value = "complete"; return; }
      if (["completed", "failed", "cancelled", "killed"].includes(selectedRun.value?.state ?? "")) {
        missingTerminalRetries = result.cursor > connectionCursor ? 0 : missingTerminalRetries + 1;
        if (missingTerminalRetries >= 3) throw new ApiError("unavailable", 503, "event_terminal_missing", "");
      }
      streamState.value = "reconnecting";
      await reconnectDelay(controller.signal);
    } catch (reason) {
      if (controller.signal.aborted) return;
      const failure = asApiError(reason); streamError.value = failure.code || "event_stream_failed"; streamState.value = "error"; return;
    }
  }
}

function reconnectDelay(signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const timer = window.setTimeout(resolve, 250);
    signal.addEventListener("abort", () => { window.clearTimeout(timer); resolve(); }, { once: true });
  });
}

async function downloadArtifact(artifact: Artifact) {
  if (!artifact.id) return;
  try {
    const download = await api.getArtifactDownload(artifact.id);
    if (!download.url) throw new ApiError("unavailable", 503, "artifact_download_unavailable", "");
    window.open(download.url, "_blank", "noopener,noreferrer");
  } catch (reason) { error.value = asApiError(reason); }
}

function eventCategory(type: string) {
  if (["run.completed", "run.failed", "run.cancelled", "run.killed"].includes(type)) return "terminal";
  if (type.startsWith("approval.")) return "approval";
  if (type.includes("plan")) return "plan";
  if (type.includes("diff")) return "diff";
  if (type.includes("file")) return "file";
  if (type.includes("validation")) return "validation";
  if (type.includes("command")) return "command";
  if (type.includes("cost")) return "cost";
  if (type.includes("usage")) return "usage";
  if (type.includes("failed") || type.includes("error")) return "error";
  return type.startsWith("run.") ? "run" : "runtime";
}

function eventPayload(event: RunEvent) {
  return previewPayload(event.payload);
}

function previewPayload(payload: unknown) {
  const value = JSON.stringify(payload, null, 2) ?? "null";
  return value.length <= 16_384 ? value : `${value.slice(0, 16_384)}\n[display truncated at 16384 characters]`;
}

function intent(scope: string, input: unknown) {
  const fingerprint = JSON.stringify(input);
  const current = intents.get(scope);
  if (current?.fingerprint === fingerprint) return current.key;
  const key = crypto.randomUUID(); intents.set(scope, { fingerprint, key }); return key;
}

async function createTask() {
  if (!canUse.value || saving.value || prerequisite.value || !form.releaseID) return;
  const input = form.source === "issue"
    ? { agent_release_id: form.releaseID, title: form.issueTitle, request_text: form.issueBody, issue_snapshot: { title: form.issueTitle, body: form.issueBody, ...(form.issueURL ? { url: form.issueURL } : {}) } }
    : { agent_release_id: form.releaseID, title: form.title, request_text: form.requestText };
  const scope = `coding-task.create:${teamID.value}`;
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    const launch = await api.createCodingTask(teamID.value, input, intent(scope, input));
    intents.delete(scope);
    const taskID = launch.task?.id;
    if (!taskID) throw new Error("Coding Task launch response is incomplete");
    Object.assign(form, { title: "", requestText: "", issueTitle: "", issueBody: "", issueURL: "" });
    notice.value = t("workspace.notice.created");
    await router.push({ name: "workspace", query: { team: teamID.value, task: taskID } });
    await refresh();
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function decideApproval(approval: RunApproval, approved: boolean) {
  if (!canDecideApproval.value || controlling.value || !approval.id || !approval.version) return;
  const reason = decisionReasons[approval.id] ?? "";
  if (!approved && !reason.trim()) return;
  const input = { approved, reason, version: approval.version };
  const scope = `run-approval.decide:${teamID.value}:${approval.id}`;
  controlling.value = true; error.value = undefined; notice.value = "";
  try {
    await api.decideRunApproval(approval.id, approved, reason, approval.version, intent(scope, input));
    intents.delete(scope);
    await refreshSelectedRun(approval.run_id ?? selectedRun.value?.id ?? "");
    notice.value = t(approved ? "workspace.notice.approved" : "workspace.notice.rejected");
  } catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict" && approval.run_id) await refreshSelectedRun(approval.run_id);
  } finally { controlling.value = false; }
}

async function controlSelectedRun(action: "interrupt" | "resume" | "cancel") {
  const run = selectedRun.value;
  if (!run?.id || !run.version || controlling.value) return;
  if (action === "cancel" && !window.confirm(t("workspace.controls.cancelConfirm"))) return;
  const input = { action, version: run.version };
  const scope = `run.control:${teamID.value}:${run.id}:${action}`;
  controlling.value = true; error.value = undefined; notice.value = "";
  try {
    const updated = await api.controlRun(run.id, action, run.version, intent(scope, input));
    intents.delete(scope);
    const index = runs.value.findIndex((value) => value.id === run.id);
    if (index >= 0) runs.value.splice(index, 1, updated);
    notice.value = t(`workspace.notice.${action}`);
  } catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict") await refreshSelectedRun(run.id);
  } finally { controlling.value = false; }
}

function approvalRequest(approval: RunApproval) {
  return previewPayload(approval.request ?? {});
}

function approvalRisk(approval: RunApproval) {
  const request = approval.request as Record<string, unknown> | undefined;
  return String(request?.risk_reason ?? request?.reason ?? request?.summary ?? t("workspace.approval.unspecifiedRisk"));
}

async function selectTask(task: CodingTask) {
  if (!task.id) return;
  await router.push({ name: "workspace", query: { team: teamID.value, task: task.id } });
}

function asApiError(reason: unknown) {
  return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "workspace_failed", "");
}

function errorLabel(value: ApiError) {
  const prerequisites: Record<string, string> = {
    coding_task_runtime_unavailable: "workspace.prerequisite.runtime",
    coding_task_model_unavailable: "workspace.prerequisite.model",
    coding_task_binding_unavailable: "workspace.prerequisite.binding",
  };
  if (prerequisites[value.code]) return t(prerequisites[value.code]!);
  return t(value.kind === "forbidden" ? "errors.forbidden" : value.kind === "validation" ? "errors.validation" : value.kind === "conflict" ? "errors.conflict" : "errors.server");
}

function safeIssueURL(value?: string) {
  if (!value) return "";
  try { const parsed = new URL(value); return parsed.protocol === "https:" && !parsed.username && !parsed.password ? parsed.href : ""; }
  catch { return ""; }
}
</script>

<template>
  <section class="surface workspace-shell">
    <header class="catalog-header reveal">
      <div><p class="eyebrow">{{ t('workspace.kicker') }}</p><h2>{{ t('workspace.title') }}</h2><p>{{ t('workspace.body') }}</p></div>
      <span v-if="!canUse" class="read-only-badge">{{ t('workspace.readOnly') }}</span>
    </header>
    <p v-if="notice" class="catalog-notice" role="status">{{ notice }}</p>
    <div v-if="error" class="catalog-error" role="alert"><strong>{{ errorLabel(error) }}</strong><span>{{ t('workspace.errorBody') }}</span><small v-if="error.requestID">{{ error.requestID }}</small></div>
    <div v-if="loading" class="catalog-loading"><i></i>{{ t('workspace.loading') }}</div>
    <template v-else>
      <section class="launch-board reveal delay-1">
        <div class="launch-copy"><span>01 / {{ t('workspace.launch') }}</span><h3>{{ t('workspace.newTask') }}</h3><p>{{ t('workspace.launchHint') }}</p></div>
        <form class="task-form" @submit.prevent="createTask">
          <div class="source-switch" role="group" :aria-label="t('workspace.source')">
            <button type="button" :class="{ active: form.source === 'text' }" @click="form.source = 'text'">{{ t('workspace.freeText') }}</button>
            <button type="button" :class="{ active: form.source === 'issue' }" @click="form.source = 'issue'">{{ t('workspace.issueSnapshot') }}</button>
          </div>
          <div class="form-grid compact-form">
            <label><span>{{ t('workspace.repositoryBinding') }}</span><select v-model="form.bindingID" data-testid="binding-select" required><option v-for="binding in availableBindings" :key="binding.id" :value="binding.id">{{ binding.name }}</option></select></label>
            <label><span>{{ t('workspace.agentRelease') }}</span><select v-model="form.releaseID" data-testid="release-select" required><option v-for="release in launchableReleases" :key="release.id" :value="release.id">{{ release.runtime_image_snapshot?.runtime }} / R{{ release.release_number }} / {{ release.configured_model_snapshot?.name }}</option></select></label>
            <template v-if="form.source === 'text'">
              <label class="wide"><span>{{ t('workspace.taskTitle') }}</span><input v-model="form.title" data-testid="task-title" maxlength="200" required></label>
              <label class="wide"><span>{{ t('workspace.requestText') }}</span><textarea v-model="form.requestText" data-testid="request-text" maxlength="100000" required></textarea></label>
            </template>
            <template v-else>
              <label><span>{{ t('workspace.issueTitle') }}</span><input v-model="form.issueTitle" data-testid="issue-title" maxlength="500" required></label>
              <label><span>{{ t('workspace.issueURL') }}</span><input v-model="form.issueURL" data-testid="issue-url" type="url" maxlength="2000"></label>
              <label class="wide"><span>{{ t('workspace.issueBody') }}</span><textarea v-model="form.issueBody" data-testid="issue-body" maxlength="100000" required></textarea></label>
            </template>
          </div>
          <p v-if="prerequisite" class="prerequisite" data-testid="launch-prerequisite">{{ t(`workspace.prerequisite.${prerequisite}`) }}</p>
          <button class="primary-action" data-testid="create-task" :disabled="saving || !canUse || Boolean(prerequisite)">{{ saving ? t('workspace.launching') : t('workspace.launch') }}</button>
        </form>
      </section>

      <section class="workspace-grid">
        <aside class="task-index" :aria-label="t('workspace.tasks')">
          <header><span>02</span><h3>{{ t('workspace.tasks') }}</h3></header>
          <p v-if="tasks.length === 0" class="task-empty">{{ t('workspace.noTasks') }}</p>
          <button v-for="task in tasks" :key="task.id" :class="{ active: task.id === selectedTaskID }" @click="selectTask(task)">
            <strong>{{ task.title }}</strong><small>{{ task.state }} · {{ task.created_at ? d(new Date(task.created_at), 'long') : '—' }}</small>
          </button>
        </aside>
        <article class="workspace-detail">
          <div v-if="!selectedTask" class="workspace-placeholder"><span>03</span><h3>{{ t('workspace.selectTask') }}</h3></div>
          <div v-else-if="detailLoading" class="catalog-loading"><i></i>{{ t('workspace.loadingTask') }}</div>
          <template v-else>
            <header class="detail-title"><div><span>{{ selectedTask.state }}</span><h3>{{ selectedTask.title }}</h3></div><strong>{{ t('workspace.runCount', { count: session?.run_count ?? 0 }) }}</strong></header>
            <p class="task-request">{{ selectedTask.request_text }}</p>
            <dl class="workspace-facts">
              <div><dt>{{ t('workspace.session') }}</dt><dd>{{ session?.id }}</dd></div>
              <div><dt>{{ t('workspace.reviewBranch') }}</dt><dd>{{ session?.review_branch }}</dd></div>
              <div><dt>{{ t('workspace.targetBranch') }}</dt><dd>{{ session?.target_branch }}</dd></div>
              <div><dt>{{ t('workspace.repositoryBinding') }}</dt><dd>{{ selectedRelease?.repository_binding_snapshot?.name }}</dd></div>
              <div><dt>{{ t('workspace.agentRelease') }}</dt><dd>R{{ selectedRelease?.release_number }} · {{ selectedRelease?.release_risk }}</dd></div>
              <div><dt>{{ t('workspace.runtime') }}</dt><dd>{{ selectedRelease?.runtime_image_snapshot?.runtime }} @ {{ selectedRelease?.runtime_image_snapshot?.image_digest }}</dd></div>
              <div><dt>{{ t('workspace.model') }}</dt><dd>{{ selectedRelease?.configured_model_snapshot?.name }} / {{ selectedRelease?.configured_model_snapshot?.model_id }}</dd></div>
              <div><dt>{{ t('workspace.firstRun') }}</dt><dd>{{ selectedRun?.id }} · {{ selectedRun?.state }}</dd></div>
            </dl>
            <section v-if="selectedRun" class="run-controls" data-testid="run-controls">
              <header><div><span>04</span><h4>{{ t('workspace.controls.title') }}</h4></div><small>{{ t('workspace.controls.version', { version: selectedRun.version }) }}</small></header>
              <p>{{ t('workspace.controls.body') }}</p>
              <div class="control-actions">
                <button v-if="canInterruptOrCancel && ['provisioning', 'running'].includes(selectedRun.state ?? '')" type="button" data-testid="interrupt-run" :disabled="controlling" @click="controlSelectedRun('interrupt')">{{ t('workspace.controls.interrupt') }}</button>
                <button v-if="canResume && selectedRun.state === 'interrupted'" type="button" data-testid="resume-run" :disabled="controlling" @click="controlSelectedRun('resume')">{{ t('workspace.controls.resume') }}</button>
                <button v-if="canInterruptOrCancel && !['waiting_confirmation', 'completed', 'failed', 'cancelled'].includes(selectedRun.state ?? '')" type="button" class="danger-action" data-testid="cancel-run" :disabled="controlling" @click="controlSelectedRun('cancel')">{{ t('workspace.controls.cancel') }}</button>
              </div>
            </section>
            <section v-if="runApprovals.length" class="run-approvals" data-testid="run-approvals">
              <header><div><span>05</span><h4>{{ t('workspace.approval.title') }}</h4></div><small>{{ t('workspace.approval.distinct') }}</small></header>
              <article v-for="approval in runApprovals" :key="approval.id" :class="`approval-${approval.state}`">
                <div class="approval-heading"><strong>{{ t(`workspace.approval.kind.${approval.kind}`) }}</strong><span>{{ t(`workspace.approval.state.${approval.state}`) }}</span></div>
                <dl><div><dt>{{ t('workspace.approval.run') }}</dt><dd>{{ approval.run_id }} · {{ selectedRun.state }}</dd></div><div><dt>{{ t('workspace.approval.requestedBy') }}</dt><dd>{{ approval.requested_by || '—' }}</dd></div><div><dt>{{ t('workspace.approval.risk') }}</dt><dd>{{ approvalRisk(approval) }}</dd></div></dl>
                <details><summary>{{ t('workspace.approval.request') }}</summary><pre>{{ approvalRequest(approval) }}</pre></details>
                <div v-if="approval.state === 'pending' && canDecideApproval" class="approval-decision">
                  <label><span>{{ t('workspace.approval.reason') }}</span><textarea v-model="decisionReasons[approval.id!]" maxlength="4000"></textarea></label>
                  <button type="button" :data-testid="`approve-run-${approval.id}`" :disabled="controlling" @click="decideApproval(approval, true)">{{ t('workspace.approval.approve') }}</button>
                  <button type="button" class="danger-action" :data-testid="`reject-run-${approval.id}`" :disabled="controlling || !decisionReasons[approval.id!]?.trim()" @click="decideApproval(approval, false)">{{ t('workspace.approval.reject') }}</button>
                </div>
                <p v-else-if="approval.state === 'pending'" class="read-only-badge">{{ t('workspace.approval.noDecisionGrant') }}</p>
                <p v-else>{{ approval.decision_reason || t('workspace.approval.noDecisionReason') }} · {{ approval.decided_by || t('workspace.approval.systemActor') }}</p>
              </article>
            </section>
            <section v-if="runs.length" class="run-evidence" data-testid="run-evidence">
              <header><div><span>06</span><h4>{{ t('workspace.evidence.title') }}</h4></div><small :class="`stream-${streamState}`">{{ t(`workspace.evidence.stream.${streamState}`) }}</small></header>
              <div class="run-tabs" role="tablist" :aria-label="t('workspace.evidence.runs')">
                <button v-for="(run, index) in runs" :key="run.id" type="button" :class="{ active: run.id === selectedRun?.id }" @click="selectedRunID = run.id ?? ''">{{ t('workspace.evidence.run', { number: runs.length - index }) }} · {{ t(`status.${run.state}`) }}</button>
              </div>
              <p v-if="streamError" class="contract-error" role="alert">{{ t('workspace.evidence.contractError') }} · {{ streamError }}</p>
              <div class="attempt-strip">
                <article v-for="attempt in selectedRun?.attempts ?? []" :key="attempt.id"><strong>{{ t('workspace.evidence.attempt', { number: attempt.number }) }}</strong><span>{{ t(`status.${attempt.state}`) }}</span><small>{{ attempt.infrastructure_failure ? t('workspace.evidence.infrastructureFailure') : attempt.worker_id }}</small></article>
                <p v-if="!(selectedRun?.attempts?.length)">{{ t('workspace.evidence.noAttempts') }}</p>
              </div>
              <ol class="event-timeline">
                <li v-for="event in runEvents" :key="event.sequence" :class="`event-${eventCategory(event.event_type)}`">
                  <span>{{ event.sequence }}</span><div><em>{{ t(`workspace.evidence.categories.${eventCategory(event.event_type)}`) }}</em><strong>{{ event.event_type }}</strong><time>{{ d(new Date(event.created_at), 'long') }}</time><pre>{{ eventPayload(event) }}</pre></div>
                </li>
                <li v-if="runEvents.length === 0" class="empty-evidence">{{ t('workspace.evidence.noEvents') }}</li>
              </ol>
              <div class="artifact-grid">
                <p v-if="artifactError" class="contract-error" role="alert">{{ t('workspace.evidence.artifactError') }} · {{ artifactError }}</p>
                <button v-for="artifact in artifacts" :key="artifact.id" type="button" @click="downloadArtifact(artifact)"><strong>{{ artifact.kind }}</strong><span>{{ artifact.content_type }} · {{ artifact.size_bytes }} B</span><small>SHA-256 {{ artifact.sha256 }}</small></button>
                <p v-if="artifacts.length === 0">{{ t('workspace.evidence.noArtifacts') }}</p>
              </div>
              <dl class="run-usage"><div><dt>{{ t('workspace.evidence.usage') }}</dt><dd>{{ previewPayload(selectedRun?.usage ?? {}) }}</dd></div><div><dt>{{ t('workspace.evidence.cost') }}</dt><dd>{{ selectedRun?.cost_amount }}</dd></div><div><dt>{{ t('workspace.evidence.capabilities') }}</dt><dd>{{ previewPayload(selectedRelease?.runtime_image_snapshot?.capabilities ?? {}) }}</dd></div><div v-if="selectedRun?.terminal_error"><dt>{{ t('workspace.evidence.terminalError') }}</dt><dd>{{ previewPayload(selectedRun.terminal_error) }}</dd></div></dl>
            </section>
            <section v-if="selectedTask.issue_snapshot" class="issue-snapshot"><span>{{ t('workspace.immutableIssue') }}</span><h4>{{ selectedTask.issue_snapshot.title }}</h4><p>{{ selectedTask.issue_snapshot.body }}</p><a v-if="safeIssueURL(selectedTask.issue_snapshot.url)" :href="safeIssueURL(selectedTask.issue_snapshot.url)" target="_blank" rel="noreferrer">{{ selectedTask.issue_snapshot.url }}</a></section>
          </template>
        </article>
      </section>
    </template>
  </section>
</template>
