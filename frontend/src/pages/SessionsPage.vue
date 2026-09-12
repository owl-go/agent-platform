<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Archive, ArchiveRestore, Pencil, Trash2 } from "@lucide/vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type Artifact, type ExecutionActivity, type ModelProviderConnection, type PersonalSettings, type RuntimeEngineStatus, type Session, type SessionMessage, type SessionMessageSnapshot } from "../api/client";
import ActionIconButton from "../components/ActionIconButton.vue";
import ToastMessage from "../components/ToastMessage.vue";
import ConversationComposer from "../components/ConversationComposer.vue";
import ConversationThread from "../components/ConversationThread.vue";
import type { ComposerSubmission } from "../conversationDraft";
import { cliAuthorizationRequestFromActivities } from "../cliAuthorization";
import { summarizeExecutionActivities, type ExecutionActivitySummary } from "../executionActivitySummary";
import type { ConversationMessage } from "../conversationThread";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const sessions = ref<Session[]>([]);
const archived = ref<Session[]>([]);
const specialistName = ref("");
const connections = ref<ModelProviderConnection[]>([]);
const runtimes = ref<RuntimeEngineStatus[]>([]);
const settings = ref<PersonalSettings>();
const selected = ref<Session>();
const messages = ref<SessionMessage[]>([]);
const loadingMessages = ref(false);
const editingSessionID = ref("");
const editingTitle = ref("");
const pendingDelete = ref<Session>();
const deleting = ref(false);
const deleteDialog = ref<HTMLElement>();
const launchSkill = ref<{ sessionID: string; skillID: string }>();
const loading = ref(true);
const sending = ref(false);
const cancellingMessageID = ref<number>();
const creating = ref(false);
const showArchived = ref(false);
const error = ref("");
const messageStream = ref<HTMLElement>();
const composerLayer = ref<HTMLElement>();
const composerClearance = ref(154);
const showJumpToLatest = ref(false);
const keepAtLatest = ref(true);
const selectableModels = computed(() => connections.value.flatMap((connection) => connection.models.filter((model) => model.available).map((model) => ({ ...model, connection }))));
const setupRequired = computed(() => messages.value.length > 0 ? false : selectableModels.value.length === 0 || !settings.value?.runtime_model_defaults.some((item) => item.runtime_engine === settings.value?.default_runtime_engine) || !runtimes.value.some((item) => item.name === settings.value?.default_runtime_engine && item.available));
const activeAssistant = computed(() => {
  for (let index = messages.value.length - 1; index >= 0; index--) {
    const message = messages.value[index];
    if (message?.role === "assistant" && (message.state === "queued" || message.state === "generating" || message.state === "waiting_for_user")) return message;
  }
  return undefined;
});
const cliAuthorizationRequest = computed(() => {
  const latestAssistant = [...messages.value].reverse().find((message) => message.role === "assistant");
  return cliAuthorizationRequestFromActivities(latestAssistant?.activities);
});
const conversationMessages = computed<ConversationMessage[]>(() => messages.value.map((message, index) => {
  const summaries = message.role === "assistant" ? activitySummaries(message) : [];
  const pending = message.role === "assistant" && ["queued", "generating", "waiting_for_user"].includes(message.state);
  const identity = message.role === "assistant" ? responseIdentity(message) : undefined;
  return {
    id: String(message.id),
    role: message.role,
    content: message.role === "user" ? userMessageContent(message, index) : message.content,
    copyText: message.role === "user" ? userMessageContent(message, index) : message.content,
    state: message.state,
    timestamp: message.created_at,
    elapsedMs: message.elapsed_ms,
    error: message.error,
    pending,
    finalizing: pending && message.progress_stage === "finalizing",
    streaming: pending && message.role === "assistant",
    progressTitle: pending ? (message.progress_stage === "finalizing" ? progressLabel(message.progress_stage) : message.state === "waiting_for_user" ? t("common.waitingForUser") : t("sessions.thinking")) : undefined,
    progressDetail: pending ? activeStageLabel(message) : undefined,
    currentActivity: pending && summaries.length ? { id: summaries.at(-1)!.id, label: activitySummaryLabel(summaries.at(-1)!), detail: summaries.at(-1)!.detail } : undefined,
    activities: summaries.map((summary) => ({
      id: summary.id,
      label: activitySummaryLabel(summary),
      detail: summary.detail,
      state: summary.state,
      items: summary.activities.map((activity, activityIndex) => ({ id: activityIndex, label: activityLabel(activity, true), detail: activity.detail })),
    })),
    stages: message.role === "assistant" ? message.expert_stages : undefined,
    creditConsumption: message.credit_consumption,
    artifacts: message.artifacts,
    attachments: message.attachments,
    skills: message.role === "user" ? messageSkills(index) : undefined,
    meta: identity ? { label: `${identity.expertName ? `${identity.expertName} · ` : ""}${identity.modelName}`, title: `${identity.connection} · ${identity.modelID} · ${identity.runtime}` } : undefined,
    retryable: message.role === "assistant" && message.state === "failed",
  };
}));
let pollTimer: ReturnType<typeof setTimeout> | undefined;
let pollGeneration = 0;
let composerObserver: ResizeObserver | undefined;
let responseController: AbortController | undefined;
let revealTimer: ReturnType<typeof setTimeout> | undefined;
let revealMessageID = 0;
let revealTarget = "";
let terminalSnapshot: SessionMessageSnapshot | undefined;
let deleteReturnFocus: HTMLElement | undefined;

onMounted(async () => {
  if (typeof ResizeObserver !== "undefined") composerObserver = new ResizeObserver(measureComposer);
  if (composerLayer.value) composerObserver?.observe(composerLayer.value);
  window.addEventListener("resize", handleViewportResize);
  await refresh();
  if (route.query.new) await create();
});
watch(() => route.query.new, (value) => { if (value) void create(); });
watch(composerLayer, (current, previous) => {
  if (previous) composerObserver?.unobserve(previous);
  if (current) composerObserver?.observe(current);
  void nextTick(measureComposer);
});
watch(messages, async () => {
  const shouldKeepAtLatest = keepAtLatest.value;
  await nextTick();
  if (shouldKeepAtLatest) scrollToLatest("auto");
  else updateScrollState();
}, { deep: true, flush: "post" });

function updateScrollState() {
  const stream = messageStream.value;
  if (!stream) return;
  const distanceToLatest = stream.scrollHeight - stream.scrollTop - stream.clientHeight;
  keepAtLatest.value = distanceToLatest <= 32;
  showJumpToLatest.value = distanceToLatest > 32;
}
function measureComposer() {
  const height = composerLayer.value?.getBoundingClientRect().height ?? 0;
  if (height <= 0) return;
  const shouldKeepAtLatest = keepAtLatest.value;
  composerClearance.value = Math.ceil(height) + 16;
  void nextTick(() => {
    if (shouldKeepAtLatest) scrollToLatest("auto");
    else updateScrollState();
  });
}
function handleViewportResize() {
  measureComposer();
  updateScrollState();
}
function scrollToLatest(behavior: ScrollBehavior = "smooth") {
  const stream = messageStream.value;
  if (!stream) return;
  stream.scrollTo({ top: stream.scrollHeight, behavior });
  keepAtLatest.value = true;
  showJumpToLatest.value = false;
}

async function refresh() {
  loading.value = true; error.value = "";
  try {
    [sessions.value, archived.value, connections.value, runtimes.value, settings.value] = await Promise.all([api.listSessions(false), api.listSessions(true), api.listModelProviderConnections(), api.listRuntimeEngines(), api.getSettings()]);
    if (!selected.value && sessions.value[0]) await open(sessions.value[0]);
  } catch { error.value = t("errors.generic"); }
  finally { loading.value = false; }
}
async function open(item: Session) {
  const generation = ++pollGeneration;
  if (pollTimer) clearTimeout(pollTimer);
  responseController?.abort(); stopReveal();
  cancellingMessageID.value = undefined;
  keepAtLatest.value = true; showJumpToLatest.value = false;
  selected.value = item; specialistName.value = ""; messages.value = []; loadingMessages.value = true;
  try {
    const loadedMessages = await api.listSessionMessages(item.id);
    if (generation !== pollGeneration || selected.value?.id !== item.id) return;
    messages.value = loadedMessages.map((message) => ({ ...message, attachments: message.attachments ?? [] }));
    await nextTick(); scrollToLatest("auto");
    const pending = [...messages.value].reverse().find((message) => message.role === "assistant" && (message.state === "queued" || message.state === "generating"));
    if (pending) void streamAssistant(item.id, pending.id, generation);
  } catch {
    if (generation === pollGeneration) error.value = t("errors.generic");
  } finally {
    if (generation === pollGeneration && selected.value?.id === item.id) loadingMessages.value = false;
  }
}
async function create() {
  if (creating.value) return;
  creating.value = true;
  try {
    const expertID = typeof route.query.expert_id === "string" ? route.query.expert_id : undefined;
    const teamID = typeof route.query.expert_team_id === "string" ? route.query.expert_team_id : undefined;
    const skillID = typeof route.query.skill_id === "string" ? route.query.skill_id : undefined;
    const item = expertID || teamID ? await api.createSession({ expert_id: expertID, expert_team_id: teamID }) : await api.createSession();
    launchSkill.value = skillID ? { sessionID: item.id, skillID } : undefined;
    sessions.value.unshift(item);
    await router.replace({ path: "/sessions" }); await open(item);
  } catch {
    error.value = t("errors.validation");
    if (route.query.new) await router.replace({ path: "/sessions" });
  } finally {
    creating.value = false;
  }
}
async function send(message: ComposerSubmission) {
  if (!selected.value || setupRequired.value || sending.value || activeAssistant.value) throw new Error("conversation_not_ready");
  const sessionID = selected.value.id, generation = pollGeneration;
  sending.value = true;
  try {
    const pair = await api.sendSessionMessage(sessionID, message.content, message.attachmentIDs, undefined, message.input);
    if (selected.value?.id !== sessionID || generation !== pollGeneration) return;
    messages.value.push(pair.user_message, pair.assistant_message);
    void streamAssistant(sessionID, pair.assistant_message.id, generation);
  } finally { sending.value = false; }
}
async function downloadSessionArtifact(artifact: Artifact) {
  if (!selected.value || artifact.expired) return;
  try {
    const blob = await api.getSessionArtifactDownload(selected.value.id, artifact.id);
    const url = URL.createObjectURL(blob);
    triggerBrowserDownload(url, artifact.name);
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  } catch { error.value = t("errors.generic"); }
}
function triggerBrowserDownload(url: string, name: string) {
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.rel = "noopener noreferrer";
  anchor.click();
}
async function retry(index: number) {
  if (!selected.value || sending.value || activeAssistant.value) return;
  const original = [...messages.value.slice(0, index)].reverse().find((message) => message.role === "user");
  if (!original) return;
  sending.value = true;
  try {
    const pair = await api.retrySessionMessage(selected.value.id, original.id);
    messages.value.push(pair.user_message, pair.assistant_message);
    void streamAssistant(selected.value.id, pair.assistant_message.id, pollGeneration);
  } catch { error.value = t("errors.generic"); }
  finally { sending.value = false; }
}
async function pollAssistant(sessionID: string, messageID: number, generation: number) {
  try {
    const latest = await api.listSessionMessages(sessionID);
    if (generation !== pollGeneration || selected.value?.id !== sessionID) return;
    messages.value = latest.map((message) => ({ ...message, attachments: message.attachments ?? [] }));
    const message = latest.find((item) => item.id === messageID);
    if (message && (message.state === "queued" || message.state === "generating")) {
      pollTimer = setTimeout(() => void pollAssistant(sessionID, messageID, generation), 900);
    } else {
      if (cancellingMessageID.value === messageID) cancellingMessageID.value = undefined;
      window.dispatchEvent(new Event("credits-updated"));
      const refreshed = sessions.value.find((item) => item.id === sessionID);
      if (refreshed) {
        const current = await api.listSessions(false);
        sessions.value = current;
        selected.value = current.find((item) => item.id === sessionID) ?? selected.value;
      }
    }
  } catch {
    if (generation === pollGeneration) pollTimer = setTimeout(() => void pollAssistant(sessionID, messageID, generation), 1800);
  }
}
async function streamAssistant(sessionID: string, messageID: number, generation: number) {
  responseController?.abort();
  const controller = new AbortController();
  responseController = controller;
  try {
    await api.streamSessionMessage(sessionID, messageID, (snapshot) => {
      if (generation !== pollGeneration || selected.value?.id !== sessionID) return;
      applySnapshot(messageID, snapshot);
    }, controller.signal);
    await waitForReveal(messageID, generation);
    if (!controller.signal.aborted && generation === pollGeneration && selected.value?.id === sessionID) await pollAssistant(sessionID, messageID, generation);
  } catch (streamError) {
    if (!controller.signal.aborted && generation === pollGeneration) void pollAssistant(sessionID, messageID, generation);
  } finally {
    if (responseController === controller) responseController = undefined;
  }
}
function applySnapshot(messageID: number, snapshot: SessionMessageSnapshot) {
  const message = messages.value.find((item) => item.id === messageID);
  if (!message) return;
  message.progress_stage = snapshot.progress_stage;
  message.error = snapshot.error;
  message.elapsed_ms = snapshot.elapsed_ms;
  message.expert_stages = snapshot.expert_stages ?? message.expert_stages;
  message.credit_consumption = snapshot.credit_consumption ?? message.credit_consumption;
  message.activities = snapshot.activities ?? message.activities;
  if (snapshot.state === "queued" || snapshot.state === "generating") message.state = snapshot.state;
  else if (snapshot.state === "cancelled") {
    message.state = "cancelled";
    if (cancellingMessageID.value === messageID) cancellingMessageID.value = undefined;
  }
  if (!snapshot.content.startsWith(message.content)) message.content = "";
  revealMessageID = messageID;
  revealTarget = snapshot.content;
  terminalSnapshot = snapshot.state === "completed" || snapshot.state === "failed" || snapshot.state === "cancelled" ? snapshot : undefined;
  revealNextChunk();
}
async function cancelGeneration() {
  const sessionID = selected.value?.id;
  const message = activeAssistant.value;
  if (!sessionID || !message || cancellingMessageID.value === message.id) return;
  cancellingMessageID.value = message.id;
  try {
    const updated = await api.cancelSessionMessage(sessionID, message.id);
    if (selected.value?.id !== sessionID) return;
    Object.assign(message, updated);
    if (updated.state === "cancelled") {
      responseController?.abort();
      stopReveal();
      cancellingMessageID.value = undefined;
    } else {
      responseController?.abort();
      stopReveal();
      await pollAssistant(sessionID, message.id, pollGeneration);
    }
  } catch {
    if (selected.value?.id === sessionID) {
      cancellingMessageID.value = undefined;
      error.value = t("errors.generic");
    }
  }
}
function revealNextChunk() {
  if (revealTimer) return;
  const tick = () => {
    const message = messages.value.find((item) => item.id === revealMessageID);
    if (!message) { stopReveal(); return; }
    if (message.content.length < revealTarget.length) {
      const remaining = revealTarget.length - message.content.length;
      let end = message.content.length + Math.min(24, Math.max(1, Math.ceil(remaining / 32)));
      const lastCode = revealTarget.charCodeAt(end - 1);
      if (lastCode >= 0xD800 && lastCode <= 0xDBFF) end += 1;
      message.content = revealTarget.slice(0, end);
      revealTimer = setTimeout(tick, 22);
      return;
    }
    revealTimer = undefined;
    if (terminalSnapshot) {
      message.content = terminalSnapshot.content;
      message.state = terminalSnapshot.state;
      message.error = terminalSnapshot.error;
      message.progress_stage = terminalSnapshot.progress_stage;
      message.elapsed_ms = terminalSnapshot.elapsed_ms;
      terminalSnapshot = undefined;
    }
  };
  tick();
}
function stopReveal() {
  if (revealTimer) clearTimeout(revealTimer);
  revealTimer = undefined; revealMessageID = 0; revealTarget = ""; terminalSnapshot = undefined;
}
async function waitForReveal(messageID: number, generation: number) {
  while (generation === pollGeneration && revealMessageID === messageID && (revealTimer || terminalSnapshot)) {
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
}
async function refreshSessionList(sessionID: string) {
  const current = await api.listSessions(false);
  sessions.value = current;
  selected.value = current.find((item) => item.id === sessionID) ?? selected.value;
}
function progressLabel(stage?: string) {
  const supported = ["preparing", "thinking", "using_tool", "working", "responding", "finalizing"];
  return t(`sessions.progress.${supported.includes(stage ?? "") ? stage : "thinking"}`);
}
function activeStageLabel(message: SessionMessage) {
  const stage = [...(message.expert_stages ?? [])].reverse().find((item) => item.state === "running");
  return stage ? `${stage.position}/${stage.total || message.expert_stages?.length || stage.position} · ${stage.expert_name}` : progressLabel(message.progress_stage);
}
function responseIdentity(message: SessionMessage) {
	const snapshot = message.response_snapshot;
	const stage = snapshot?.stages?.at(-1);
	const expertName = message.expert_stages?.at(-1)?.expert_name;
	if (stage) return { connection: stage.provider_model.connection_name, modelID: stage.provider_model.model_id, modelName: stage.provider_model.name, runtime: stage.runtime_engine, expertName };
	if (snapshot?.model_name) return { connection: snapshot.connection_name, modelID: snapshot.model_id, modelName: snapshot.model_name, runtime: snapshot.runtime_engine, expertName };
	return undefined;
}
function messageSkills(index: number) {
  if (messages.value[index]?.role !== "user") return [];
  const response = messages.value[index + 1];
  if (response?.role !== "assistant") return [];
  const skills = response.response_snapshot?.stages?.flatMap((stage) => stage.skills ?? []) ?? [];
  return skills.filter((skill, skillIndex) => skills.findIndex((candidate) => candidate.id === skill.id) === skillIndex);
}
function userMessageContent(message: SessionMessage, index: number) {
  let content = message.content;
  for (const skill of messageSkills(index)) {
    const prefix = `[${skill.name}]`;
    if (content.trimStart().startsWith(prefix)) content = content.trimStart().slice(prefix.length).trimStart();
  }
  return content;
}
function activityLabel(activity: ExecutionActivity, historical = false) {
  if (activity.type === "runtime.started") return historical ? t("workflows.runtimePrepared") : t("sessions.progress.preparing");
  if (activity.type === "reasoning.summary") return t("workflows.reasoningSummary");
  if (activity.type === "command.requested") return t("sessions.progress.using_tool");
  if (activity.type === "command.completed") return t("workflows.toolCompleted");
  if (activity.type === "file.changed") return t("workflows.updatingFiles");
  return t("sessions.progress.working");
}
function activitySummaries(message: SessionMessage) {
  return summarizeExecutionActivities(message.activities ?? []);
}
function activitySummaryLabel(summary: ExecutionActivitySummary) {
  if (summary.kind === "reasoning" && summary.detail) return summary.detail;
  return t(`sessions.activitySummary.${summary.kind}.${summary.state}`);
}
async function archive(item: Session) {
  try { await api.archiveSession(item.id, !item.archived, item.version); if (selected.value?.id === item.id) selected.value = undefined; await refresh(); }
  catch { error.value = t("errors.conflict"); }
}
function startRename(item: Session) {
  editingSessionID.value = item.id;
  editingTitle.value = item.title;
  void nextTick(() => {
    const input = document.querySelector<HTMLInputElement>(".session-title-input");
    input?.focus();
    input?.select();
  });
}
function cancelRename() {
  editingSessionID.value = "";
  editingTitle.value = "";
}
async function saveRename(item: Session) {
  if (editingSessionID.value !== item.id) return;
  const title = editingTitle.value.trim();
  cancelRename();
  if (!title || title === item.title) return;
  try { const updated = await api.renameSession(item.id, title, item.version); Object.assign(item, updated); if (selected.value?.id === item.id) selected.value = updated; }
  catch { error.value = t("errors.conflict"); }
}
function requestRemove(item: Session, event?: Event) {
  pendingDelete.value = item;
  deleteReturnFocus = event?.currentTarget instanceof HTMLElement ? event.currentTarget : undefined;
  void nextTick(() => deleteDialog.value?.focus());
}
function cancelRemove() {
  if (deleting.value) return;
  pendingDelete.value = undefined;
  void nextTick(() => deleteReturnFocus?.focus());
}
async function confirmRemove() {
  const item = pendingDelete.value;
  if (!item || deleting.value) return;
  deleting.value = true;
  try {
    try {
      await api.deleteSession(item.id);
    } catch (cause) {
      // DELETE is idempotent from the UI perspective: another tab may have
      // removed the Session after this list was loaded.
      if (!(cause instanceof ApiError) || cause.kind !== "not_found") {
        error.value = t("errors.generic");
        return;
      }
    }
    pendingDelete.value = undefined;
    if (selected.value?.id === item.id) selected.value = undefined;
    try { await refresh(); }
    catch { error.value = t("errors.generic"); }
  }
  finally { deleting.value = false; }
}
onBeforeUnmount(() => { pollGeneration += 1; if (pollTimer) clearTimeout(pollTimer); responseController?.abort(); stopReveal(); composerObserver?.disconnect(); window.removeEventListener("resize", handleViewportResize); });
</script>

<template>
  <section class="session-layout">
    <aside class="collection-panel">
      <div class="collection-head"><div><h1>{{ t('sessions.title') }}</h1></div><el-button circle type="primary" class="icon-button" :loading="creating" :aria-label="t('sessions.new')" @click="create">＋</el-button></div>
      <p class="muted collection-subtitle">{{ t('sessions.subtitle') }}</p>
      <el-skeleton v-if="loading" :rows="6" animated class="collection-loading" />
      <div v-else class="session-groups">
        <p class="group-label">{{ t('sessions.active') }} · {{ sessions.length }}</p>
        <div v-for="item in sessions" :key="item.id" class="session-row" :class="{ active: selected?.id === item.id, editing: editingSessionID === item.id }" role="button" tabindex="0" @click="open(item)" @keydown.enter="open(item)">
          <span class="session-glyph">◌</span><span><input v-if="editingSessionID === item.id" v-model="editingTitle" class="session-title-input" maxlength="120" @click.stop @keydown.enter.stop.prevent="saveRename(item)" @keydown.esc.stop.prevent="cancelRename" @blur="saveRename(item)"><strong v-else>{{ item.title }}</strong><small>{{ new Date(item.updated_at).toLocaleString() }}</small></span>
          <span class="row-actions">
            <ActionIconButton :label="t('common.rename')" @click.stop="startRename(item)"><Pencil aria-hidden="true" /></ActionIconButton>
            <ActionIconButton :label="t('common.archive')" @click.stop="archive(item)"><Archive aria-hidden="true" /></ActionIconButton>
            <ActionIconButton :label="t('sessions.deleteAction', { title: item.title })" :tooltip="t('common.delete')" tone="danger" @click.stop="requestRemove(item, $event)"><Trash2 aria-hidden="true" /></ActionIconButton>
          </span>
        </div>
        <el-button class="archive-toggle" text @click="showArchived = !showArchived"><span>▸</span>{{ t('sessions.archived') }} · {{ archived.length }}</el-button>
        <div v-if="showArchived">
          <div v-for="item in archived" :key="item.id" class="session-row archived" role="button" tabindex="0" @click="open(item)" @keydown.enter="open(item)">
            <span class="session-glyph">□</span><span><strong>{{ item.title }}</strong><small>{{ new Date(item.updated_at).toLocaleDateString() }}</small></span>
            <span class="row-actions">
              <ActionIconButton :label="t('common.unarchive')" @click.stop="archive(item)"><ArchiveRestore aria-hidden="true" /></ActionIconButton>
              <ActionIconButton :label="t('sessions.deleteAction', { title: item.title })" :tooltip="t('common.delete')" tone="danger" @click.stop="requestRemove(item, $event)"><Trash2 aria-hidden="true" /></ActionIconButton>
            </span>
          </div>
        </div>
      </div>
    </aside>
    <article class="conversation-panel">
      <ToastMessage v-if="error" kind="error" :title="t('common.failed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />
      <div v-if="setupRequired" class="notice setup-guide"><strong>{{ t('sessions.setupTitle') }}</strong><span>1. {{ t('sessions.setupModel') }}</span><span>2. {{ t('sessions.setupRuntime') }}</span><span>3. {{ t('sessions.setupStart') }}</span><el-button @click="router.push('/settings')">{{ t('nav.settings') }} →</el-button></div>
      <template v-if="selected">
        <header class="conversation-head"><div><h2>{{ selected.title }}</h2><p><template v-if="specialistName">{{ specialistName }} <span>·</span> </template>{{ selected.archived ? t('sessions.archived') : t('sessions.active') }}</p></div></header>
        <div ref="messageStream" class="message-stream" :style="{ paddingBottom: `${composerClearance}px` }" @scroll.passive="updateScrollState">
          <el-skeleton v-if="loadingMessages" :rows="4" animated class="message-loading" :aria-label="t('common.loading')" />
          <div v-else-if="messages.length === 0" class="chat-welcome"><span class="welcome-orb">✦</span><h2>{{ selected.title }}</h2><p>{{ t('sessions.welcome') }}</p></div>
          <ConversationThread :messages="conversationMessages" :load-attachment="api.getAttachmentDownload" @download-artifact="downloadSessionArtifact" @retry="(id) => retry(messages.findIndex((message) => String(message.id) === id))" @attachment-error="error = t('errors.generic')" @copy-error="error = t('errors.copy')" />
        </div>
        <div ref="composerLayer" class="composer-layer">
          <el-button v-if="showJumpToLatest" class="jump-to-latest" circle :aria-label="t('sessions.jumpToLatest')" @click="scrollToLatest()"><svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5.5 8 4.5 4.5L14.5 8" /></svg></el-button>
          <ConversationComposer :key="selected.id" :scope="{ session_id: selected.id }" :disabled="selected.archived" :send-disabled="setupRequired" :active="Boolean(activeAssistant)" :stopping="Boolean(activeAssistant) && cancellingMessageID === activeAssistant?.id" :initial-skill-id="launchSkill?.sessionID === selected.id ? launchSkill.skillID : undefined" :authorization-request="cliAuthorizationRequest" :approval-execution-id="activeAssistant?.state === 'waiting_for_user' ? activeAssistant.id : undefined" :submit="send" @launch-consumed="launchSkill = undefined" @selection-changed="specialistName = $event.name" @stop="cancelGeneration" />
        </div>
      </template>
      <div v-else class="chat-welcome center"><span class="welcome-orb">◌</span><h2>{{ t('sessions.title') }}</h2><p>{{ t('sessions.subtitle') }}</p><el-button type="primary" :loading="creating" @click="create">{{ t('sessions.new') }}</el-button></div>
    </article>
  </section>
  <div v-if="pendingDelete" class="modal-layer session-delete-layer" @click.self="cancelRemove">
    <section ref="deleteDialog" class="modal-card destructive-dialog" role="alertdialog" aria-modal="true" aria-labelledby="session-delete-title" aria-describedby="session-delete-description" tabindex="-1" @keydown.esc.stop="cancelRemove">
      <div class="delete-dialog-head">
        <span class="delete-mark" aria-hidden="true"><i></i></span>
        <div><h2 id="session-delete-title">{{ t('sessions.deleteTitle') }}</h2></div>
      </div>
      <p id="session-delete-description" class="delete-description">{{ t('sessions.deleteDescription') }}</p>
      <div class="delete-target"><small>{{ t('sessions.deleteTarget') }}</small><strong>{{ pendingDelete.title }}</strong></div>
      <p class="delete-warning"><span aria-hidden="true">!</span>{{ t('sessions.deleteWarning') }}</p>
      <div class="modal-actions delete-actions">
        <el-button class="button ghost" :disabled="deleting" @click="cancelRemove">{{ t('common.cancel') }}</el-button>
        <el-button class="button danger" type="danger" :loading="deleting" @click="confirmRemove">{{ deleting ? t('sessions.deleting') : t('sessions.deleteConfirm') }}</el-button>
      </div>
    </section>
  </div>
</template>
