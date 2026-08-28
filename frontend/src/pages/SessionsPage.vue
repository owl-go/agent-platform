<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Archive, ArchiveRestore, Pencil, Square, Trash2 } from "@lucide/vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ModelProviderConnection, type PersonalSettings, type RuntimeEngineStatus, type Session, type SessionMessage, type SessionMessageSnapshot } from "../api/client";
import ActionIconButton from "../components/ActionIconButton.vue";
import ToastMessage from "../components/ToastMessage.vue";
import { renderMarkdown } from "../markdown";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const sessions = ref<Session[]>([]);
const archived = ref<Session[]>([]);
const experts = ref<Expert[]>([]);
const connections = ref<ModelProviderConnection[]>([]);
const runtimes = ref<RuntimeEngineStatus[]>([]);
const settings = ref<PersonalSettings>();
const selected = ref<Session>();
const messages = ref<SessionMessage[]>([]);
const loadingMessages = ref(false);
const copiedMessageID = ref<number>();
const editingSessionID = ref("");
const editingTitle = ref("");
const pendingDelete = ref<Session>();
const deleting = ref(false);
const deleteDialog = ref<HTMLElement>();
const draft = ref("");
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
const selectedExpert = computed(() => experts.value.find((item) => item.id === selected.value?.expert_id));
const selectableModels = computed(() => connections.value.flatMap((connection) => connection.models.filter((model) => model.available && ["agent", "text", "unknown"].includes(model.model_type)).map((model) => ({ ...model, connection }))));
const selectedModelID = computed({
  get: () => selected.value?.current_provider_model_id ?? settings.value?.runtime_model_defaults.find((item) => item.runtime_engine === settings.value?.default_runtime_engine)?.provider_model_id ?? "",
  set: (value: string) => { if (selected.value) selected.value.current_provider_model_id = value; },
});
const selectedModel = computed(() => selectableModels.value.find((item) => item.id === selectedModelID.value));
const selectedCompatibility = computed(() => selectedModel.value?.compatibility.find((item) => item.runtime_engine === settings.value?.default_runtime_engine));
const setupRequired = computed(() => selectableModels.value.length === 0 || !settings.value?.runtime_model_defaults.some((item) => item.runtime_engine === settings.value?.default_runtime_engine) || !runtimes.value.some((item) => item.name === settings.value?.default_runtime_engine && item.available));
const activeAssistant = computed(() => {
  for (let index = messages.value.length - 1; index >= 0; index--) {
    const message = messages.value[index];
    if (message?.role === "assistant" && (message.state === "queued" || message.state === "generating")) return message;
  }
  return undefined;
});
let pollTimer: ReturnType<typeof setTimeout> | undefined;
let pollGeneration = 0;
let composerObserver: ResizeObserver | undefined;
let responseController: AbortController | undefined;
let revealTimer: ReturnType<typeof setTimeout> | undefined;
let revealMessageID = 0;
let revealTarget = "";
let terminalSnapshot: SessionMessageSnapshot | undefined;
let copiedTimer: ReturnType<typeof setTimeout> | undefined;
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
    [sessions.value, archived.value, experts.value, connections.value, runtimes.value, settings.value] = await Promise.all([api.listSessions(false), api.listSessions(true), api.listExperts(), api.listModelProviderConnections(), api.listRuntimeEngines(), api.getSettings()]);
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
  selected.value = item; messages.value = []; loadingMessages.value = true;
  try {
    const loadedMessages = await api.listSessionMessages(item.id);
    if (generation !== pollGeneration || selected.value?.id !== item.id) return;
    messages.value = loadedMessages;
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
    const item = await api.createSession();
    sessions.value.unshift(item);
    await router.replace({ path: "/sessions" }); await open(item);
  } catch {
    error.value = t("errors.validation");
    if (route.query.new) await router.replace({ path: "/sessions" });
  } finally {
    creating.value = false;
  }
}
async function send() {
  if (!selected.value || !draft.value.trim() || !selectedModelID.value || selectedCompatibility.value?.status === "incompatible" || sending.value || activeAssistant.value) return;
  const content = draft.value.trim(); draft.value = ""; sending.value = true;
  try {
    const pair = await api.sendSessionMessage(selected.value.id, content, selectedModelID.value);
    messages.value.push(pair.user_message, pair.assistant_message);
    void streamAssistant(selected.value.id, pair.assistant_message.id, pollGeneration);
  } catch { draft.value = content; error.value = t("errors.generic"); }
  finally { sending.value = false; }
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
    messages.value = latest;
    const message = latest.find((item) => item.id === messageID);
    if (message && (message.state === "queued" || message.state === "generating")) {
      pollTimer = setTimeout(() => void pollAssistant(sessionID, messageID, generation), 900);
    } else {
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
    if (generation === pollGeneration && selected.value?.id === sessionID) await refreshSessionList(sessionID);
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
  const supported = ["preparing", "thinking", "using_tool", "working", "responding"];
  return t(`sessions.progress.${supported.includes(stage ?? "") ? stage : "thinking"}`);
}
async function copyMessage(message: SessionMessage) {
  try {
    await navigator.clipboard.writeText(message.content);
    copiedMessageID.value = message.id;
    if (copiedTimer) clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { copiedMessageID.value = undefined; }, 1600);
  } catch {
    error.value = t("errors.copy");
  }
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
    await api.deleteSession(item.id);
    pendingDelete.value = undefined;
    if (selected.value?.id === item.id) selected.value = undefined;
    await refresh();
  } catch { error.value = t("errors.generic"); }
  finally { deleting.value = false; }
}
function keyboard(event: KeyboardEvent) { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); if (!activeAssistant.value) void send(); } }
onBeforeUnmount(() => { pollGeneration += 1; if (pollTimer) clearTimeout(pollTimer); if (copiedTimer) clearTimeout(copiedTimer); responseController?.abort(); stopReveal(); composerObserver?.disconnect(); window.removeEventListener("resize", handleViewportResize); });
</script>

<template>
  <section class="session-layout">
    <aside class="collection-panel">
      <div class="collection-head"><div><p class="eyebrow">01 / CONVERSATIONS</p><h1>{{ t('sessions.title') }}</h1></div><button class="icon-button" :disabled="creating" @click="create">＋</button></div>
      <p class="muted collection-subtitle">{{ t('sessions.subtitle') }}</p>
      <div v-if="loading" class="quiet-state">{{ t('common.loading') }}</div>
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
        <button class="archive-toggle" @click="showArchived = !showArchived"><span>▸</span>{{ t('sessions.archived') }} · {{ archived.length }}</button>
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
      <div v-if="setupRequired" class="notice setup-guide"><strong>{{ t('sessions.setupTitle') }}</strong><span>1. {{ t('sessions.setupModel') }}</span><span>2. {{ t('sessions.setupRuntime') }}</span><span>3. {{ t('sessions.setupStart') }}</span><button class="button ghost" @click="router.push('/settings')">{{ t('nav.settings') }} →</button></div>
      <template v-if="selected">
        <header class="conversation-head"><div><h2>{{ selected.title }}</h2><p>{{ selectedExpert?.name ?? t('sessions.noExpert') }} <span>·</span> {{ selected.archived ? t('sessions.archived') : t('sessions.active') }}</p></div><span class="engine-chip">AUTO RUNTIME</span></header>
        <div ref="messageStream" class="message-stream" :style="{ paddingBottom: `${composerClearance}px` }" @scroll.passive="updateScrollState">
          <div v-if="loadingMessages" class="message-loading" :aria-label="t('common.loading')"><span></span><span></span><span></span></div>
          <div v-else-if="messages.length === 0" class="chat-welcome"><span class="welcome-orb">✦</span><h2>{{ selected.title }}</h2><p>{{ t('sessions.welcome') }}</p></div>
          <div v-for="(message, index) in messages" :key="message.id" class="message" :class="message.role">
            <div class="message-content">
              <div v-if="message.role === 'assistant' && (message.state === 'queued' || message.state === 'generating')" class="thinking-state"><span class="thinking-dots" aria-hidden="true"><i></i><i></i><i></i></span><strong>{{ t('sessions.thinking') }}</strong><small>{{ progressLabel(message.progress_stage) }}</small></div>
              <div v-if="message.content && message.role === 'assistant'" class="markdown-body" :class="{ streaming: message.state === 'queued' || message.state === 'generating' }" v-html="renderMarkdown(message.content)"></div>
              <p v-else-if="message.content">{{ message.content }}</p><p v-else-if="message.state === 'failed'">{{ message.error }}</p>
              <p v-if="message.state === 'cancelled'" class="cancelled-response">{{ t('sessions.cancelled') }}</p>
              <div class="message-actions">
                <small class="message-meta">{{ new Date(message.created_at).toLocaleTimeString() }}<template v-if="message.elapsed_ms"> · {{ t('sessions.elapsed', { value: `${(message.elapsed_ms / 1000).toFixed(1)}s` }) }}</template><span v-if="message.response_snapshot" class="message-model" :title="`${message.response_snapshot.connection_name} · ${message.response_snapshot.model_id} · ${message.response_snapshot.runtime_engine}`"> · {{ message.response_snapshot.model_name }}</span></small>
                <button v-if="message.content" type="button" class="message-copy" :class="{ copied: copiedMessageID === message.id }" :aria-label="message.role === 'user' ? t('sessions.copyQuestion') : t('sessions.copyAnswer')" @click="copyMessage(message)"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="7" y="7" width="9" height="9" rx="2"/><path d="M13 7V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/></svg><span>{{ copiedMessageID === message.id ? t('common.copied') : t('common.copy') }}</span></button>
                <button v-if="message.role === 'assistant' && message.state === 'failed'" class="text-button" @click="retry(index)">{{ t('common.retry') }}</button>
              </div>
            </div>
          </div>
        </div>
        <div ref="composerLayer" class="composer-layer">
          <button v-if="showJumpToLatest" class="jump-to-latest" type="button" :aria-label="t('sessions.jumpToLatest')" @click="scrollToLatest()"><svg viewBox="0 0 20 20" aria-hidden="true"><path d="m5.5 8 4.5 4.5L14.5 8" /></svg></button>
          <footer class="composer">
            <textarea v-model="draft" :placeholder="t('sessions.placeholder')" :disabled="selected.archived || sending" @keydown="keyboard"></textarea>
            <div class="composer-model-control"><label><span class="model-dot" :class="selectedCompatibility?.status"></span><select v-model="selectedModelID" :aria-label="t('sessions.modelSelector')" :disabled="selected.archived || sending || Boolean(activeAssistant)"><optgroup v-for="connection in connections" :key="connection.id" :label="connection.name"><option v-for="model in connection.models.filter((item) => item.available && ['agent','text','unknown'].includes(item.model_type))" :key="model.id" :value="model.id" :disabled="model.compatibility.find((item) => item.runtime_engine === settings?.default_runtime_engine)?.status === 'incompatible'">{{ model.display_name }}</option></optgroup></select></label><small v-if="selectedCompatibility?.status === 'unverified'">{{ t('sessions.modelUnverified') }}</small></div>
            <button v-if="activeAssistant" type="button" class="stop-generation" :class="{ stopping: cancellingMessageID === activeAssistant.id }" :disabled="cancellingMessageID === activeAssistant.id" :aria-label="cancellingMessageID === activeAssistant.id ? t('sessions.stopping') : t('sessions.stopGeneration')" :title="cancellingMessageID === activeAssistant.id ? t('sessions.stopping') : t('sessions.stopGeneration')" @click="cancelGeneration"><Square aria-hidden="true" /></button>
            <button v-else type="button" :disabled="!draft.trim() || !selectedModelID || selectedCompatibility?.status === 'incompatible' || sending || selected.archived" @click="send">↑</button>
          </footer>
        </div>
      </template>
      <div v-else class="chat-welcome center"><span class="welcome-orb">◌</span><h2>{{ t('sessions.title') }}</h2><p>{{ t('sessions.subtitle') }}</p><button class="button primary" :disabled="creating" @click="create">{{ t('sessions.new') }}</button></div>
    </article>
  </section>
  <div v-if="pendingDelete" class="modal-layer session-delete-layer" @click.self="cancelRemove">
    <section ref="deleteDialog" class="modal-card destructive-dialog" role="alertdialog" aria-modal="true" aria-labelledby="session-delete-title" aria-describedby="session-delete-description" tabindex="-1" @keydown.esc.stop="cancelRemove">
      <div class="delete-dialog-head">
        <span class="delete-mark" aria-hidden="true"><i></i></span>
        <div><p class="eyebrow">PERMANENT / DELETE</p><h2 id="session-delete-title">{{ t('sessions.deleteTitle') }}</h2></div>
      </div>
      <p id="session-delete-description" class="delete-description">{{ t('sessions.deleteDescription') }}</p>
      <div class="delete-target"><small>{{ t('sessions.deleteTarget') }}</small><strong>{{ pendingDelete.title }}</strong></div>
      <p class="delete-warning"><span aria-hidden="true">!</span>{{ t('sessions.deleteWarning') }}</p>
      <div class="modal-actions delete-actions">
        <button type="button" class="button ghost" :disabled="deleting" @click="cancelRemove">{{ t('common.cancel') }}</button>
        <button type="button" class="button danger" :disabled="deleting" @click="confirmRemove">{{ deleting ? t('sessions.deleting') : t('sessions.deleteConfirm') }}</button>
      </div>
    </section>
  </div>
</template>
