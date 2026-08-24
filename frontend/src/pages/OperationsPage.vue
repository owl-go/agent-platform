<script setup lang="ts">
import { computed, inject, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type Artifact, type AuditEvent, type CodingTask, type Run, type RunEvent, type RunSearchFilters } from "../api/client";

const injectedApi = inject(platformApiKey);
if (!injectedApi) throw new Error("Operations Console API is required");
const api = injectedApi;
const searchRuns = api.searchRuns;
if (!api.listAuditEvents) throw new Error("Operations Audit API is required");
const listAuditEvents = api.listAuditEvents.bind(api);
const route = useRoute();
const router = useRouter();
const { t, d } = useI18n();
const runs = ref<Run[]>([]);
const selectedRun = ref<Run>();
const relatedTask = ref<CodingTask>();
const events = ref<RunEvent[]>([]);
const artifacts = ref<Artifact[]>([]);
const auditEvents = ref<AuditEvent[]>([]);
const loading = ref(false);
const detailLoading = ref(false);
const error = ref<ApiError>();
const detailError = ref<ApiError>();
const auditError = ref<ApiError>();
const controlling = ref(false);
const auditLoading = ref(false);
const nextPageToken = ref("");
const streamState = ref<"idle" | "live" | "complete" | "error">("idle");
const form = reactive({ agent: "", binding: "", task: "", state: "", runtime: "", from: "", to: "", sort: "desc" as "asc" | "desc" });
const auditForm = reactive({ actor: "", operation: "", resource: "", outcome: "", from: "", to: "" });
const controlReason = ref("");
const intents = new Map<string, string>();
let listSequence = 0;
let detailSequence = 0;
let detailController: AbortController | undefined;

const teamID = computed(() => typeof route.query.team === "string" ? route.query.team : "");
const selectedRunID = computed(() => typeof route.query.run === "string" ? route.query.run : "");
const runtimeSnapshot = computed(() => selectedRun.value?.runtime_image_snapshot as Record<string, unknown> | undefined);
const repositorySnapshot = computed(() => selectedRun.value?.repository_binding_snapshot as Record<string, unknown> | undefined);
const modelSnapshot = computed(() => selectedRun.value?.configured_model_snapshot as Record<string, unknown> | undefined);

watch(() => route.query, () => { hydrateForm(); void search(); void loadAudit(); }, { immediate: true });
watch(selectedRunID, () => void loadDetail(), { immediate: true });
onBeforeUnmount(() => detailController?.abort());

function queryString(name: string) { return typeof route.query[name] === "string" ? String(route.query[name]) : ""; }
function hydrateForm() {
  form.agent = queryString("agent"); form.binding = queryString("binding"); form.task = queryString("task");
  form.state = queryString("state"); form.runtime = queryString("runtime"); form.from = queryString("from"); form.to = queryString("to");
  form.sort = queryString("sort") === "asc" ? "asc" : "desc";
}
function filters(): RunSearchFilters {
  return {
    teamID: teamID.value, agentID: queryString("agent"), repositoryBindingID: queryString("binding"), taskID: queryString("task"),
    state: queryString("state"), runtime: queryString("runtime"), createdFrom: queryString("from"), createdTo: queryString("to"),
    sortDirection: queryString("sort") === "asc" ? "asc" : "desc", pageToken: queryString("page"), limit: 25,
  };
}
async function applyFilters() {
  const query: Record<string, string> = { team: teamID.value, sort: form.sort };
  for (const [key, value] of Object.entries({ agent: form.agent, binding: form.binding, task: form.task, state: form.state, runtime: form.runtime, from: form.from, to: form.to })) if (value) query[key] = value;
  await router.push({ name: "operations", query });
}
async function search() {
  const requestedTeam = teamID.value;
  if (!requestedTeam) return;
  const sequence = ++listSequence;
  loading.value = true; error.value = undefined;
  try {
    const page = await searchRuns(filters());
    if (sequence !== listSequence || teamID.value !== requestedTeam) return;
    runs.value = page.items; nextPageToken.value = page.nextPageToken;
  } catch (reason) {
    if (sequence === listSequence) { runs.value = []; nextPageToken.value = ""; error.value = asApiError(reason); }
  } finally { if (sequence === listSequence) loading.value = false; }
}
async function chooseRun(runID: string) { await router.push({ name: "operations", query: { ...route.query, run: runID } }); }
async function nextPage() {
  if (nextPageToken.value) await router.push({ name: "operations", query: { ...route.query, page: nextPageToken.value, run: undefined } });
}
async function loadDetail() {
  detailController?.abort();
  const runID = selectedRunID.value;
  const requestedTeam = teamID.value;
  selectedRun.value = undefined; relatedTask.value = undefined; events.value = []; artifacts.value = [];
  detailError.value = undefined; streamState.value = "idle";
  if (!runID || !requestedTeam) return;
  const sequence = ++detailSequence;
  const controller = new AbortController(); detailController = controller; detailLoading.value = true;
  try {
    const run = await api.getRun(runID, controller.signal);
    if (sequence !== detailSequence) return;
    selectedRun.value = run;
    const [artifactValues, task] = await Promise.all([
      api.listRunArtifacts(runID, controller.signal),
      run.coding_task_id ? api.getCodingTask(run.coding_task_id, requestedTeam, controller.signal) : Promise.resolve(undefined),
    ]);
    if (sequence !== detailSequence) return;
    artifacts.value = artifactValues; relatedTask.value = task;
    streamState.value = "live";
    void api.streamRunEvents(runID, 0, (event) => { if (sequence === detailSequence) events.value.push(event); }, controller.signal)
      .then(() => { if (sequence === detailSequence) streamState.value = "complete"; })
      .catch((reason) => { if (sequence === detailSequence && !controller.signal.aborted) { streamState.value = "error"; detailError.value = asApiError(reason); } });
  } catch (reason) {
    if (sequence === detailSequence && !controller.signal.aborted) detailError.value = asApiError(reason);
  } finally { if (sequence === detailSequence) detailLoading.value = false; }
}
function asApiError(reason: unknown) { return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "unknown_error", ""); }
function errorLabel(value: ApiError) {
  const key = { unauthenticated: "authentication", forbidden: "forbidden", not_found: "notFound", conflict: "conflict", validation: "validation", rate_limited: "rateLimited", unavailable: "offline", unknown: "server" }[value.kind];
  return t(`errors.${key}`);
}
function json(value: unknown) { return JSON.stringify(value ?? {}, null, 2); }
function intent(scope: string, fingerprint: unknown) {
  const key = `${scope}:${JSON.stringify(fingerprint)}`;
  const existing = intents.get(key);
  if (existing) return existing;
  const created = crypto.randomUUID(); intents.set(key, created); return created;
}
function clearProtectedState() {
  listSequence++; detailSequence++; detailController?.abort();
  runs.value = []; selectedRun.value = undefined; relatedTask.value = undefined; events.value = []; artifacts.value = []; auditEvents.value = [];
}
const latestAttempt = computed(() => selectedRun.value?.attempts?.at(-1));
const canInterrupt = computed(() => ["provisioning", "running"].includes(selectedRun.value?.state ?? ""));
const canCancelOrKill = computed(() => ["queued", "provisioning", "running", "interrupting", "interrupted", "resuming"].includes(selectedRun.value?.state ?? ""));
const canRecover = computed(() => selectedRun.value?.state === "recovery_required" && latestAttempt.value?.infrastructure_failure === true && ["failed", "lost"].includes(latestAttempt.value.state ?? ""));
async function control(action: "interrupt" | "cancel" | "kill" | "recover") {
  const run = selectedRun.value;
  const reason = controlReason.value.trim();
  if (!run?.id || !run.version || controlling.value || ((action === "kill" || action === "recover") && (reason.length < 3 || reason.length > 500))) return;
  if (action === "kill" && !window.confirm(t("operations.killConfirm"))) return;
  const fingerprint = { action, version: run.version, reason };
  const scope = `operations.control:${teamID.value}:${run.id}:${action}`;
  controlling.value = true; detailError.value = undefined;
  try {
    const updated = await api.controlRun(run.id, action, run.version, intent(scope, fingerprint), reason || undefined);
    intents.delete(`${scope}:${JSON.stringify(fingerprint)}`); selectedRun.value = updated; controlReason.value = "";
    const index = runs.value.findIndex((value) => value.id === run.id); if (index >= 0) runs.value.splice(index, 1, updated);
    await loadAudit();
  } catch (reason) {
    detailError.value = asApiError(reason);
    if (detailError.value.kind === "conflict") await loadDetail();
    if (detailError.value.kind === "forbidden" || detailError.value.kind === "unauthenticated") clearProtectedState();
  } finally { controlling.value = false; }
}
async function loadAudit() {
  if (!teamID.value) return;
  auditLoading.value = true; auditError.value = undefined;
  try {
    auditEvents.value = await listAuditEvents({ teamID: teamID.value, actorUserID: auditForm.actor, action: auditForm.operation, resourceType: auditForm.resource, outcome: auditForm.outcome as "succeeded" | "failed" || undefined, createdFrom: auditForm.from, createdTo: auditForm.to, limit: 100 });
  } catch (reason) {
    auditError.value = asApiError(reason); auditEvents.value = [];
    if (auditError.value.kind === "forbidden" || auditError.value.kind === "unauthenticated") clearProtectedState();
  } finally { auditLoading.value = false; }
}
</script>

<template>
  <section class="surface operations-shell">
    <header class="surface-heading operations-heading reveal"><div><span>{{ t('operations.kicker') }}</span><h1>{{ t('operations.title') }}</h1><p>{{ t('operations.body') }}</p></div><strong>{{ t('operations.scope') }}</strong></header>
    <form class="operations-filters reveal delay-1" data-testid="operations-filters" @submit.prevent="applyFilters">
      <label><span>{{ t('operations.agent') }}</span><input v-model.trim="form.agent" data-testid="filter-agent"></label>
      <label><span>{{ t('operations.binding') }}</span><input v-model.trim="form.binding" data-testid="filter-binding"></label>
      <label><span>{{ t('operations.task') }}</span><input v-model.trim="form.task" data-testid="filter-task"></label>
      <label><span>{{ t('operations.state') }}</span><select v-model="form.state" data-testid="filter-state"><option value="">{{ t('operations.all') }}</option><option v-for="state in ['queued','provisioning','running','waiting_confirmation','interrupting','interrupted','resuming','recovery_required','completed','failed','cancelled']" :key="state" :value="state">{{ state }}</option></select></label>
      <label><span>{{ t('operations.runtime') }}</span><select v-model="form.runtime" data-testid="filter-runtime"><option value="">{{ t('operations.all') }}</option><option v-for="runtime in ['claude','codex','hermes','openclaw']" :key="runtime" :value="runtime">{{ runtime }}</option></select></label>
      <label><span>{{ t('operations.from') }}</span><input v-model="form.from" type="datetime-local" data-testid="filter-from"></label>
      <label><span>{{ t('operations.to') }}</span><input v-model="form.to" type="datetime-local" data-testid="filter-to"></label>
      <label><span>{{ t('operations.sort') }}</span><select v-model="form.sort" data-testid="filter-sort"><option value="desc">{{ t('operations.newest') }}</option><option value="asc">{{ t('operations.oldest') }}</option></select></label>
      <button class="primary-action" data-testid="search-runs">{{ t('operations.search') }}</button>
    </form>
    <div v-if="error" class="error-state" role="alert"><strong>{{ errorLabel(error) }}</strong><code>{{ error.code }}</code><button @click="search">{{ t('operations.retry') }}</button></div>
    <div v-else-if="loading" class="loading-state" aria-live="polite"><span></span><p>{{ t('operations.loading') }}</p></div>
    <div v-else class="operations-grid">
      <aside class="run-index">
        <header><span>{{ t('operations.results') }}</span><strong>{{ runs.length }}</strong></header>
        <p v-if="runs.length === 0" class="task-empty" data-testid="operations-empty">{{ t('operations.empty') }}</p>
        <button v-for="run in runs" :key="run.id" :class="{ active: run.id === selectedRunID }" :data-testid="`operation-run-${run.id}`" @click="chooseRun(run.id!)"><strong>{{ run.state }}</strong><code>{{ run.id }}</code><small>{{ d(new Date(run.created_at!), 'long') }} · {{ run.attempt_count }} {{ t('operations.attempts') }}</small></button>
        <button v-if="nextPageToken" class="page-next" data-testid="next-page" @click="nextPage">{{ t('operations.next') }}</button>
      </aside>
      <article class="operations-detail">
        <div v-if="!selectedRunID" class="workspace-placeholder"><span>RUN / DIAGNOSTICS</span><h3>{{ t('operations.select') }}</h3></div>
        <div v-else-if="detailLoading" class="loading-state"><span></span><p>{{ t('operations.loadingDetail') }}</p></div>
        <div v-else-if="detailError && !selectedRun" class="error-state" role="alert"><strong>{{ errorLabel(detailError) }}</strong><code>{{ detailError.code }}</code></div>
        <template v-else-if="selectedRun">
          <header class="operation-run-heading"><div><span>{{ t('operations.run') }}</span><h2>{{ selectedRun.id }}</h2></div><em>{{ selectedRun.state }}</em></header>
          <section class="operator-controls" data-testid="operator-controls">
            <header><div><span>{{ t('operations.controlKicker') }}</span><h3>{{ t('operations.controls') }}</h3></div><code>v{{ selectedRun.version }}</code></header>
            <p>{{ t('operations.controlBoundary') }}</p>
            <label><span>{{ t('operations.reason') }}</span><textarea v-model.trim="controlReason" maxlength="500" :placeholder="t('operations.reasonHint')"></textarea></label>
            <div><button :disabled="!canInterrupt || controlling" data-testid="operator-interrupt" @click="control('interrupt')">{{ t('operations.interrupt') }}</button><button :disabled="!canCancelOrKill || controlling" data-testid="operator-cancel" @click="control('cancel')">{{ t('operations.cancel') }}</button><button class="danger" :disabled="!canCancelOrKill || controlling || controlReason.length < 3" data-testid="operator-kill" @click="control('kill')">{{ t('operations.kill') }}</button><button :disabled="!canRecover || controlling || controlReason.length < 3" data-testid="operator-recover" @click="control('recover')">{{ t('operations.recover') }}</button></div>
            <small v-if="selectedRun.state === 'failed'">{{ t('operations.notRecoverable') }}</small>
          </section>
          <p v-if="relatedTask" class="related-task"><strong>{{ relatedTask.title }}</strong><span>{{ relatedTask.state }} · {{ relatedTask.id }}</span></p>
          <dl class="diagnostic-facts">
            <div><dt>{{ t('operations.release') }}</dt><dd><code>{{ selectedRun.agent_release_id }}</code></dd></div>
            <div><dt>{{ t('operations.runtimeDigest') }}</dt><dd><code>{{ runtimeSnapshot?.image_digest ?? '—' }}</code></dd></div>
            <div><dt>{{ t('operations.runtime') }}</dt><dd>{{ runtimeSnapshot?.runtime ?? '—' }} · {{ runtimeSnapshot?.cli_version ?? '—' }}</dd></div>
            <div><dt>{{ t('operations.model') }}</dt><dd>{{ modelSnapshot?.name ?? '—' }} · <code>{{ modelSnapshot?.model_id ?? '—' }}</code></dd></div>
            <div><dt>{{ t('operations.binding') }}</dt><dd>{{ repositorySnapshot?.name ?? '—' }} · <code>{{ repositorySnapshot?.repository_ssh_url ?? '—' }}</code> · {{ repositorySnapshot?.default_branch ?? '—' }}</dd></div>
            <div><dt>{{ t('operations.lease') }}</dt><dd v-if="selectedRun.lease">{{ selectedRun.lease.worker_id }} · {{ d(new Date(selectedRun.lease.expires_at!), 'long') }}</dd><dd v-else>{{ t('operations.noLease') }}</dd></div>
          </dl>
          <section class="diagnostic-json"><div><h3>{{ t('operations.modelBinding') }}</h3><pre>{{ json(selectedRun.model_binding) }}</pre></div><div><h3>{{ t('operations.budget') }}</h3><pre>{{ json(selectedRun.model_budget) }}</pre></div><div><h3>{{ t('operations.usageCost') }}</h3><pre>{{ json(selectedRun.usage) }}</pre><strong>{{ selectedRun.cost_amount || '0' }} USD</strong></div><div><h3>{{ t('operations.limits') }}</h3><pre>{{ json(selectedRun.execution_limits) }}</pre></div></section>
          <section v-if="selectedRun.terminal_error" class="terminal-diagnostic"><h3>{{ t('operations.error') }}</h3><pre>{{ json(selectedRun.terminal_error) }}</pre></section>
          <section class="attempt-timeline"><h3>{{ t('operations.attemptTimeline') }}</h3><p v-if="!selectedRun.attempts?.length">{{ t('operations.noAttempts') }}</p><article v-for="attempt in selectedRun.attempts ?? []" :key="attempt.id"><strong>#{{ attempt.number }} · {{ attempt.state }}</strong><span>{{ d(new Date(attempt.started_at!), 'long') }} → {{ attempt.ended_at ? d(new Date(attempt.ended_at), 'long') : t('operations.active') }}</span><small>{{ attempt.worker_id }} · {{ attempt.infrastructure_failure ? t('operations.infrastructureFailure') : t('operations.applicationAttempt') }} · {{ t('operations.sandboxState', { state: attempt.state }) }}</small><pre v-if="attempt.error">{{ json(attempt.error) }}</pre></article></section>
          <section class="operator-evidence"><header><h3>{{ t('operations.events') }}</h3><span>{{ streamState }}</span></header><ol><li v-for="event in events" :key="event.sequence"><code>#{{ event.sequence }}</code><div><strong>{{ event.event_type }}</strong><details><summary>{{ t('operations.payload') }}</summary><pre>{{ json(event.payload) }}</pre></details></div><time>{{ d(new Date(event.created_at), 'long') }}</time></li><li v-if="events.length === 0">{{ t('operations.noEvents') }}</li></ol></section>
          <section class="operator-artifacts"><h3>{{ t('operations.artifacts') }}</h3><p v-if="artifacts.length === 0">{{ t('operations.noArtifacts') }}</p><article v-for="artifact in artifacts" :key="artifact.id"><strong>{{ artifact.kind }}</strong><code>{{ artifact.sha256 }}</code><span>{{ artifact.size_bytes }} B · {{ artifact.content_type }}</span></article></section>
        </template>
      </article>
    </div>
    <section class="audit-console reveal" data-testid="audit-console">
      <header><div><span>{{ t('operations.auditKicker') }}</span><h2>{{ t('operations.audit') }}</h2></div><strong>{{ auditEvents.length }}</strong></header>
      <form @submit.prevent="loadAudit"><label><span>{{ t('operations.actor') }}</span><input v-model.trim="auditForm.actor"></label><label><span>{{ t('operations.operation') }}</span><input v-model.trim="auditForm.operation" placeholder="run.kill"></label><label><span>{{ t('operations.resource') }}</span><input v-model.trim="auditForm.resource" placeholder="run"></label><label><span>{{ t('operations.outcome') }}</span><select v-model="auditForm.outcome"><option value="">{{ t('operations.all') }}</option><option value="succeeded">{{ t('operations.succeeded') }}</option><option value="failed">{{ t('operations.failed') }}</option></select></label><label><span>{{ t('operations.from') }}</span><input v-model="auditForm.from" type="datetime-local"></label><label><span>{{ t('operations.to') }}</span><input v-model="auditForm.to" type="datetime-local"></label><button>{{ t('operations.searchAudit') }}</button></form>
      <div v-if="auditError" class="error-state" role="alert"><strong>{{ errorLabel(auditError) }}</strong><code>{{ auditError.code }}</code></div><p v-else-if="auditLoading">{{ t('operations.loadingAudit') }}</p>
      <div v-else class="audit-list"><article v-for="event in auditEvents" :key="event.id"><time>{{ d(new Date(event.created_at!), 'long') }}</time><strong>{{ event.action }}</strong><code>{{ event.resource_type }} / {{ event.resource_id }}</code><span>{{ event.actor_user_id || 'system' }} · {{ event.outcome }}</span><details><summary>{{ t('operations.safeMetadata') }}</summary><pre>{{ json(event.details) }}</pre></details></article><p v-if="auditEvents.length === 0">{{ t('operations.noAudit') }}</p></div>
    </section>
  </section>
</template>
