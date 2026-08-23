<script setup lang="ts">
import { computed, inject, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  ApiError, platformApiKey, type AgentMemory, type CodingTask, type CodingTaskSession,
  type MemoryCandidate, type SessionMemory, type SessionMessage,
} from "../api/client";

const props = defineProps<{ task: CodingTask; session: CodingTaskSession; teamId: string; agentId: string; canUse: boolean; deliveredCommit?: string }>();
const emit = defineEmits<{ changed: [] }>();
const injectedApi = inject(platformApiKey);
if (!injectedApi) throw new Error("Task continuity API is required");
const api = injectedApi;
const { t, d } = useI18n();

const messages = ref<SessionMessage[]>([]);
const candidates = ref<MemoryCandidate[]>([]);
const memories = ref<AgentMemory[]>([]);
const continuation = ref("");
const busy = ref(false);
const error = ref<ApiError>();
const notice = ref("");
const intents = new Map<string, { fingerprint: string; key: string }>();
const memoryForm = reactive({ summary: "", decisions: "", results: "", snapshots: "" });
const memoryDrafts = reactive<Record<string, { content: string; enabled: boolean }>>({});
const memoryPolicies = [
  "quality-gate:test:unit", "quality-gate:test:integration", "quality-gate:test:parser-regression", "quality-gate:test:full",
  "workflow:tests-before-commit", "workflow:small-focused-changes", "workflow:backward-compatible-changes",
  "repository:preserve-public-api", "repository:follow-existing-conventions",
];
let loadSequence = 0;

const terminal = computed(() => ["completed", "cancelled"].includes(props.task.state ?? ""));
const canContinue = computed(() => props.task.state === "waiting_for_user");

watch(() => [props.task.id, props.teamId, props.agentId], () => void load(), { immediate: true });
watch(() => props.session, syncSessionMemory, { immediate: true, deep: true });

function syncSessionMemory() {
  const memory = props.session.memory;
  memoryForm.summary = memory?.summary ?? "";
  memoryForm.decisions = (memory?.confirmed_decisions ?? []).join("\n");
  memoryForm.results = (memory?.results ?? []).join("\n");
  memoryForm.snapshots = (memory?.workspace_snapshots ?? []).join("\n");
}

async function load() {
  const taskID = props.task.id;
  const teamID = props.teamId;
  const agentID = props.agentId;
  if (!taskID || !teamID || !agentID) return;
  const sequence = ++loadSequence;
  try {
    const [messageValues, candidateValues, memoryValues] = await Promise.all([
      loadAllMessages(taskID, teamID), api.listMemoryCandidates(taskID, teamID), api.listAgentMemories(agentID, teamID),
    ]);
    if (sequence !== loadSequence) return;
    messages.value = messageValues;
    candidates.value = candidateValues;
    memories.value = memoryValues;
    for (const memory of memoryValues) if (memory.id) memoryDrafts[memory.id] = { content: memory.content ?? "", enabled: memory.enabled ?? false };
  } catch (reason) { if (sequence === loadSequence) error.value = asApiError(reason); }
}

async function loadAllMessages(taskID: string, teamID: string) {
  const values: SessionMessage[] = [];
  let after = 0;
  for (;;) {
    const page = await api.listSessionMessages(taskID, teamID, after);
    values.push(...page);
    if (page.length < 200) return values;
    const next = page.at(-1)?.id ?? after;
    if (next <= after) throw new ApiError("unknown", 0, "message_cursor_invalid", "");
    after = next;
  }
}

function intent(scope: string, input: unknown) {
  const fingerprint = JSON.stringify(input);
  const current = intents.get(scope);
  if (current?.fingerprint === fingerprint) return current.key;
  const key = crypto.randomUUID(); intents.set(scope, { fingerprint, key }); return key;
}

async function continueTask() {
  const request = continuation.value.trim();
  if (!props.canUse || busy.value || !canContinue.value || !request || !props.task.id || !props.task.version || !props.session.version) return;
  const input = { request, taskVersion: props.task.version, sessionVersion: props.session.version };
  const scope = `coding-task.continue:${props.teamId}:${props.task.id}`;
  await mutate(async () => {
    await api.continueCodingTask(props.task.id!, props.teamId, request, props.task.version!, props.session.version!, intent(scope, input));
    intents.delete(scope); continuation.value = ""; notice.value = t("workspace.continuity.continued"); emit("changed");
  });
}

async function saveSessionMemory() {
  if (!props.canUse || busy.value || !props.task.id || !props.session.version) return;
  const memory: SessionMemory = {
    summary: memoryForm.summary.trim(), confirmed_decisions: lines(memoryForm.decisions),
    results: lines(memoryForm.results), workspace_snapshots: lines(memoryForm.snapshots),
  };
  const scope = `session-memory.update:${props.teamId}:${props.task.id}`;
  await mutate(async () => {
    await api.updateSessionMemory(props.task.id!, props.teamId, memory, props.session.version!, intent(scope, { memory, version: props.session.version }));
    intents.delete(scope); notice.value = t("workspace.continuity.memorySaved"); emit("changed");
  });
}

async function decideCandidate(candidate: MemoryCandidate, approve: boolean) {
  if (!props.canUse || busy.value || !candidate.id || candidate.state !== "pending") return;
  const scope = `memory-candidate.decide:${props.teamId}:${candidate.id}`;
  await mutate(async () => {
    await api.decideMemoryCandidate(candidate.id!, props.teamId, approve, intent(scope, { approve }));
    intents.delete(scope); notice.value = t(approve ? "workspace.continuity.candidateApproved" : "workspace.continuity.candidateRejected"); await load();
  });
}

async function saveAgentMemory(memory: AgentMemory) {
  const draft = memory.id ? memoryDrafts[memory.id] : undefined;
  if (!props.canUse || busy.value || !memory.id || !memory.version || !draft) return;
  const input = { content: draft.content.trim(), enabled: draft.enabled, version: memory.version };
  const scope = `agent-memory.update:${props.teamId}:${memory.id}`;
  await mutate(async () => {
    await api.updateAgentMemory(memory.id!, props.teamId, input.content, input.enabled, memory.version!, intent(scope, input));
    intents.delete(scope); notice.value = t("workspace.continuity.agentMemorySaved"); await load();
  });
}

async function deleteAgentMemory(memory: AgentMemory) {
  if (!props.canUse || busy.value || !memory.id || !memory.version || !window.confirm(t("workspace.continuity.deleteConfirm"))) return;
  const scope = `agent-memory.delete:${props.teamId}:${memory.id}`;
  await mutate(async () => {
    await api.deleteAgentMemory(memory.id!, props.teamId, memory.version!, intent(scope, { version: memory.version }));
    intents.delete(scope); notice.value = t("workspace.continuity.agentMemoryDeleted"); await load();
  });
}

async function closeTask(state: "completed" | "cancelled") {
  if (!props.canUse || busy.value || terminal.value || !props.task.id || !props.task.version) return;
  if (state === "cancelled" && !window.confirm(t("workspace.continuity.cancelConfirm"))) return;
  const scope = `coding-task.state:${props.teamId}:${props.task.id}:${state}`;
  await mutate(async () => {
    await api.updateCodingTaskState(props.task.id!, props.teamId, state, props.task.version!, intent(scope, { state, version: props.task.version }));
    intents.delete(scope); notice.value = t(`workspace.continuity.${state}`); emit("changed");
  });
}

async function mutate(operation: () => Promise<void>) {
  busy.value = true; error.value = undefined; notice.value = "";
  try { await operation(); }
  catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict") { intents.clear(); await load(); emit("changed"); }
  }
  finally { busy.value = false; }
}

function lines(value: string) { return value.split("\n").map((item) => item.trim()).filter(Boolean); }
function asApiError(reason: unknown) { return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "task_continuity_failed", ""); }
</script>

<template>
  <section class="continuity-panel" data-testid="task-continuity" :data-run-count="session.run_count ?? 0">
    <header><div><span>07</span><h4>{{ t('workspace.continuity.title') }}</h4></div><small>{{ t('workspace.continuity.runCount', { count: session.run_count ?? 0 }) }}</small></header>
    <p v-if="notice" class="catalog-notice" role="status">{{ notice }}</p>
    <p v-if="error" class="contract-error" role="alert">{{ error.code || t('errors.server') }}<small v-if="error.requestID"> · {{ error.requestID }}</small></p>
    <div class="continuity-grid">
      <article class="memory-kind"><span>{{ t('workspace.continuity.workingMemory') }}</span><p>{{ t('workspace.continuity.workingMemoryBody') }}</p></article>
      <article class="memory-kind"><span>{{ t('workspace.continuity.sessionMemory') }}</span><p>{{ t('workspace.continuity.sessionMemoryBody') }}</p></article>
      <article class="memory-kind"><span>{{ t('workspace.continuity.memoryCandidate') }}</span><p>{{ t('workspace.continuity.memoryCandidateBody') }}</p></article>
      <article class="memory-kind"><span>{{ t('workspace.continuity.agentMemory') }}</span><p>{{ t('workspace.continuity.agentMemoryBody') }}</p></article>
    </div>

    <form class="continuation-form" @submit.prevent="continueTask">
      <label><span>{{ t('workspace.continuity.followUp') }}</span><textarea v-model="continuation" maxlength="100000" :disabled="!canContinue || busy" data-testid="continue-text"></textarea></label>
      <button class="primary-action" :disabled="!canUse || !canContinue || busy || !continuation.trim()" data-testid="continue-task">{{ t('workspace.continuity.continue') }}</button>
    </form>

    <div class="session-ledger">
      <h5>{{ t('workspace.continuity.messages') }}</h5>
      <ol><li v-for="message in messages" :key="message.id"><span>{{ message.author }} · {{ message.run_id || t('workspace.continuity.system') }}</span><p>{{ message.content?.text || message.content?.status || message.content?.type }}</p><time v-if="message.created_at">{{ d(new Date(message.created_at), 'long') }}</time></li></ol>
      <p v-if="messages.length === 0">{{ t('workspace.continuity.noMessages') }}</p>
    </div>

    <form class="session-memory-form" @submit.prevent="saveSessionMemory">
      <h5>{{ t('workspace.continuity.sessionMemory') }} · V{{ session.version }}</h5>
      <label><span>{{ t('workspace.continuity.summary') }}</span><textarea v-model="memoryForm.summary" maxlength="10000" data-testid="session-memory-summary"></textarea></label>
      <label><span>{{ t('workspace.continuity.decisions') }}</span><textarea v-model="memoryForm.decisions"></textarea></label>
      <label><span>{{ t('workspace.continuity.results') }}</span><textarea v-model="memoryForm.results"></textarea></label>
      <label><span>{{ t('workspace.continuity.snapshots') }}</span><textarea v-model="memoryForm.snapshots"></textarea></label>
      <button :disabled="!canUse || busy" data-testid="save-session-memory">{{ t('workspace.continuity.saveSessionMemory') }}</button>
    </form>

    <div class="candidate-list">
      <h5>{{ t('workspace.continuity.memoryCandidates') }}</h5>
      <article v-for="candidate in candidates" :key="candidate.id"><p>{{ candidate.proposed_content }}</p><span>{{ t(`workspace.continuity.candidateState.${candidate.state}`) }}</span><div v-if="candidate.state === 'pending'"><button :disabled="!canUse || busy" :data-testid="`approve-memory-${candidate.id}`" @click="decideCandidate(candidate, true)">{{ t('workspace.continuity.approveCandidate') }}</button><button :disabled="!canUse || busy" :data-testid="`reject-memory-${candidate.id}`" @click="decideCandidate(candidate, false)">{{ t('workspace.continuity.rejectCandidate') }}</button></div></article>
      <p v-if="candidates.length === 0">{{ t('workspace.continuity.noCandidates') }}</p>
    </div>

    <div class="agent-memory-list">
      <h5>{{ t('workspace.continuity.agentMemories') }}</h5>
      <article v-for="memory in memories" :key="memory.id"><select v-model="memoryDrafts[memory.id!].content" :data-testid="`agent-memory-content-${memory.id}`"><option v-for="policy in memoryPolicies" :key="policy" :value="policy">{{ policy }}</option></select><label><input v-model="memoryDrafts[memory.id!].enabled" type="checkbox"><span>{{ t('workspace.continuity.enabled') }}</span></label><small>V{{ memory.version }} · {{ memory.source_task_id }}</small><div><button :disabled="!canUse || busy" :data-testid="`save-agent-memory-${memory.id}`" @click="saveAgentMemory(memory)">{{ t('workspace.continuity.save') }}</button><button class="danger-action" :disabled="!canUse || busy" :data-testid="`delete-agent-memory-${memory.id}`" @click="deleteAgentMemory(memory)">{{ t('workspace.continuity.delete') }}</button></div></article>
      <p v-if="memories.length === 0">{{ t('workspace.continuity.noAgentMemories') }}</p>
    </div>

    <div class="delivery-status"><strong>{{ t('workspace.continuity.reviewBranch') }}</strong><code>{{ session.review_branch }}</code><p v-if="deliveredCommit" data-testid="latest-commit">{{ t('workspace.continuity.latestCommit') }} · {{ deliveredCommit }}</p><p v-else data-testid="delivery-pending">{{ t('workspace.continuity.deliveryPending') }}</p></div>
    <footer class="task-closure"><button :disabled="!canUse || terminal || busy" data-testid="complete-task" @click="closeTask('completed')">{{ t('workspace.continuity.complete') }}</button><button class="danger-action" :disabled="!canUse || terminal || busy" data-testid="cancel-task" @click="closeTask('cancelled')">{{ t('workspace.continuity.cancel') }}</button></footer>
  </section>
</template>
