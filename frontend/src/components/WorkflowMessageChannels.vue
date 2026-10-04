<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ElForm, ElFormItem } from "element-plus";
import "element-plus/theme-chalk/el-form.css";
import "element-plus/theme-chalk/el-form-item.css";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type MessageChannel, type MessageChannelInput, type ChannelLogin } from "../api/client";
import ConfirmDialog from "./ConfirmDialog.vue";
import QRCode from "qrcode";
import { channelSetups } from "./messageChannelSetup";
import MessageChannelIcon from "./MessageChannelIcon.vue";

const props = defineProps<{ workflowId: string }>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const items = ref<MessageChannel[]>([]);
const available = ref(false);
const loading = ref(false);
const busy = ref(false);
const error = ref("");
const dialog = ref(false);
const editing = ref<MessageChannel>();
const confirmation = ref<{ channel: MessageChannel; action: string }>();
const form = reactive({ name: "", provider: "telegram", region: "feishu", senders: "", groups: "", direct: true, credentials: {} as Record<string,string> });
const providers = Object.keys(channelSetups);
const setup = computed(() => channelSetups[form.provider]!);
const fields = computed(() => setup.value.fields);
const login = ref<ChannelLogin>();
const qrImage = ref("");
const polling = ref(false);
const loginMethod = ref<"qr" | "credentials">("credentials");
const verificationCode = ref("");
const reconnecting = ref(false);
const authenticated = computed(() => login.value?.status === "connected" || (Boolean(editing.value) && !reconnecting.value));
const directOnly = computed(() => setup.value.audience === "direct");
const roomsOnly = computed(() => setup.value.audience === "rooms");
const credentialComplete = computed(() => fields.value.length > 0 && fields.value.every(field => form.credentials[field]?.trim()));
let loginExpiryTimer: ReturnType<typeof setTimeout> | undefined;
let pairingExpiryTimer: ReturnType<typeof setTimeout> | undefined;
let loginTimer: ReturnType<typeof setTimeout> | undefined;
let loginAbort: AbortController | undefined;
let loginGeneration = 0;
const ids = (text: string) => [...new Set(text.split(/[,\n]/).map(id => id.trim()).filter(Boolean))];
const canSave = computed(() => authenticated.value && form.name.trim() && ids(form.senders).length > 0 && (form.direct || ids(form.groups).length > 0));
const abort = new AbortController();
let timer: ReturnType<typeof setTimeout> | undefined;
let disposed = false;

async function refresh() {
  if (loading.value || disposed) return;
  loading.value = true;
  try {
    const result = await api.listMessageChannels(props.workflowId, abort.signal);
    if (disposed) return;
    items.value = result.items; available.value = result.available;
    error.value = "";
  } catch { if (!disposed) error.value = t("errors.generic"); }
  finally {
    loading.value = false;
    clearTimeout(timer);
    if (!disposed && items.value.some(c => c.validation_state === "testing")) timer = setTimeout(refresh, 5000);
  }
}
function edit(channel?: MessageChannel, provider = "telegram") {
  clearLogin(); reconnecting.value = false;
  editing.value = channel;
  provider = channel?.provider ?? provider;
  Object.assign(form, { name: channel?.name ?? t(`channels.providers.${provider}`), provider, region: channel?.region || "feishu", senders: channel?.audience.sender_ids.join("\n") ?? "", groups: channel?.audience.group_ids.join("\n") ?? "", direct: provider === "matrix" ? false : channel?.audience.allow_direct ?? true, credentials: {} });
  loginMethod.value = channelSetups[provider]?.qr ? "qr" : "credentials";
  error.value = ""; dialog.value = true;
}
function clearSecrets() { form.credentials = {}; }
function clearLogin() {
  ++loginGeneration; clearTimeout(loginTimer); clearTimeout(loginExpiryTimer); clearTimeout(pairingExpiryTimer); polling.value = false; loginAbort?.abort();
  const pending = login.value; login.value = undefined; qrImage.value = ""; verificationCode.value = "";
  if (pending) void api.cancelChannelLogin(props.workflowId, pending.id).catch(() => {});
}
function reconnect() { clearLogin(); reconnecting.value = true; clearSecrets(); }
function chooseLoginMethod(method: "qr" | "credentials") { clearLogin(); clearSecrets(); loginMethod.value = method; }
async function applyLogin(result: ChannelLogin, generation: number) {
  if (disposed || generation !== loginGeneration || !dialog.value) return;
  login.value = result;
  clearTimeout(pairingExpiryTimer);
  if (result.pairing_expires_at && ["connecting", "waiting"].includes(result.pairing_status ?? "")) {
    pairingExpiryTimer = setTimeout(() => {
      if (generation === loginGeneration && login.value) { login.value = { ...login.value, pairing_status: "expired", pairing_code: "" }; clearTimeout(loginTimer); }
    }, Math.max(0, Date.parse(result.pairing_expires_at) - Date.now()));
  }
  clearTimeout(loginExpiryTimer);
  loginExpiryTimer = setTimeout(() => {
    if (generation === loginGeneration && login.value) { login.value = { ...login.value, status: "expired", qr_content: "" }; qrImage.value = ""; clearTimeout(loginTimer); }
  }, Math.max(0, Date.parse(result.expires_at) - Date.now()));
  if (result.status === "connected") {
    qrImage.value = ""; verificationCode.value = ""; clearSecrets();
    if (result.provider !== "feishu" && !form.senders.trim() && result.suggested_sender_id) form.senders = result.suggested_sender_id;
    if (["connecting", "waiting"].includes(result.pairing_status ?? "")) loginTimer = setTimeout(() => pollLogin(generation), 2000);
    return;
  }
  if (["expired", "failed"].includes(result.status)) { qrImage.value = ""; return; }
  if (result.qr_content && !qrImage.value) {
    const image = await QRCode.toDataURL(result.qr_content, { width: 240, margin: 2 });
    if (generation !== loginGeneration || !dialog.value) return;
    qrImage.value = image;
  }
  if (result.status !== "verification_required") loginTimer = setTimeout(() => pollLogin(generation), 2000);
}
async function startSenderPairing() {
  if (busy.value || polling.value || !authenticated.value || form.provider !== "feishu") return;
  const current = login.value?.status === "connected" && Date.now() < Date.parse(login.value.expires_at) ? login.value : undefined;
  if (!current && !editing.value) return;
  if (!current) clearLogin();
  const generation = loginGeneration;
  loginAbort?.abort(); loginAbort = new AbortController(); busy.value = true; error.value = "";
  try {
    const result = await api.startChannelSenderPairing(props.workflowId, current ? {login_id:current.id} : {channel_id:editing.value!.id,version:editing.value!.version}, loginAbort.signal);
    if (generation !== loginGeneration || !dialog.value || disposed) { void api.cancelChannelLogin(props.workflowId,result.id).catch(()=>{}); return; }
    await applyLogin(result,generation);
  } catch { if (!disposed && generation === loginGeneration) error.value = t("channels.pairing.failed"); }
  finally { if (generation === loginGeneration) busy.value = false; }
}
function acceptPairedSender() {
  const candidate = login.value;
  if (candidate?.status !== "connected" || candidate.pairing_status !== "recognized" || !candidate.suggested_sender_id) return;
  form.senders = [...new Set([...ids(form.senders),candidate.suggested_sender_id])].join("\n");
}
async function copyPairingCode() {
  if (login.value?.pairing_status !== "waiting" || !login.value.pairing_code) return;
  try { await navigator.clipboard.writeText(login.value.pairing_code); }
  catch { error.value = t("channels.pairing.copyFailed"); }
}
async function connectAccount() {
  if (busy.value) return;
  clearLogin(); const generation = loginGeneration; loginAbort = new AbortController();
  busy.value = true; error.value = "";
  try {
    const result = await api.startChannelLogin(props.workflowId, { provider: form.provider, region: form.provider === "feishu" ? form.region : "", method: loginMethod.value, credentials: loginMethod.value === "qr" ? {} : { ...form.credentials }, channel_id: editing.value?.id, version: editing.value?.version ?? 0 }, loginAbort.signal);
    if (generation !== loginGeneration || !dialog.value) { void api.cancelChannelLogin(props.workflowId, result.id).catch(() => {}); return; }
    await applyLogin(result, generation);
  } catch (failure) {
    if (!disposed && generation === loginGeneration) {
      const codes = ["feishu_credentials_rejected", "feishu_authentication_unavailable", "feishu_bot_unavailable", "feishu_bot_inactive", "feishu_tenant_permission_required", "feishu_tenant_unavailable"];
      const knownFailure = form.provider === "feishu" && failure instanceof ApiError && codes.includes(failure.code);
      error.value = t(knownFailure ? `channels.loginErrors.${failure.code}` : "channels.loginFailed");
      if (knownFailure && failure.providerCode) error.value += ` ${t("channels.providerErrorCode", { code: failure.providerCode })}`;
    }
  }
  finally { if (generation === loginGeneration) busy.value = false; }
}
async function pollLogin(generation = loginGeneration) {
  const current = login.value;
  if (!current || generation !== loginGeneration || disposed || polling.value) return;
  if (Date.now() >= Date.parse(current.expires_at)) { login.value = { ...current, status: "expired", qr_content: "" }; qrImage.value = ""; return; }
  polling.value = true; const code = verificationCode.value.trim(); verificationCode.value = "";
  try {
    const result = await api.pollChannelLogin(props.workflowId, current.id, code, loginAbort?.signal);
    if (generation === loginGeneration) error.value = "";
    await applyLogin(result, generation);
  } catch {
    if (!disposed && generation === loginGeneration && dialog.value) { error.value = t("channels.loginRetry"); loginTimer = setTimeout(() => pollLogin(generation), 5000); }
  } finally { if (generation === loginGeneration) polling.value = false; }
}
function closeConfiguration() { dialog.value = false; clearLogin(); clearSecrets(); }

async function save() {
  if (!canSave.value || busy.value) return;
  busy.value = true;
  const credentials = Object.fromEntries(Object.entries(form.credentials).filter(([,v]) => v.trim() !== ""));
  const input: MessageChannelInput = { channel_id: editing.value?.id, version: editing.value?.version ?? 0, provider: form.provider, name: form.name.trim(), region: form.provider === "feishu" ? form.region : "", audience: { sender_ids:ids(form.senders), group_ids:ids(form.groups), allow_direct:form.direct }, credentials, login_id: login.value?.status === "connected" ? login.value.id : undefined };
  try { await api.saveMessageChannel(props.workflowId, input, abort.signal); clearSecrets(); login.value = undefined; closeConfiguration(); await refresh(); }
  catch { if (!disposed) error.value = t("channels.saveFailed"); }
  finally { busy.value = false; }
}
async function control(channel: MessageChannel, action: string) {
  if (busy.value) return; busy.value = true;
  try { await api.controlMessageChannel(props.workflowId, channel.id, channel.version, action, abort.signal); confirmation.value = undefined; await refresh(); }
  catch { if (!disposed) error.value = t("channels.actionFailed"); }
  finally { busy.value = false; }
}
async function confirm() {
  const pending = confirmation.value;
  if (pending) await control(pending.channel, pending.action);
}
const confirmText = computed(() => t(`channels.${confirmation.value?.action ?? "enable"}Hint`));
onMounted(refresh);
onBeforeUnmount(() => { disposed = true; abort.abort(); clearTimeout(timer); clearLogin(); clearSecrets(); });
</script>

<template>
  <section class="message-channels" :aria-label="t('channels.title')">
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-alert v-if="!loading && !available && !error" :title="t('channels.platformDisabled')" type="info" :closable="false" />
    <div class="channel-actions"><el-button :loading="loading" :disabled="busy" @click="refresh">{{ t('common.refresh') }}</el-button></div>
    <div class="channel-provider-grid">
      <button v-for="provider in providers" :key="provider" type="button" class="channel-provider-card" :data-provider="provider" :disabled="!available || busy || loading" :aria-label="t('channels.configureProvider', { provider: t(`channels.providers.${provider}`) })" @click="edit(items.find(channel => channel.provider === provider), provider)">
        <span class="channel-provider-identity"><MessageChannelIcon :provider="provider" /><strong>{{ t(`channels.providers.${provider}`) }}</strong></span>
        <span class="channel-provider-action">{{ t(items.some(channel => channel.provider === provider) ? 'channels.configured' : 'channels.configure') }}</span>
      </button>
    </div>
    <p v-if="loading && !items.length" role="status">{{ t('common.loading') }}</p>
    <article v-for="channel in items" :key="channel.id" class="channel-card">
      <div class="channel-heading"><MessageChannelIcon :provider="channel.provider" :size="24" /><strong>{{ channel.name }}</strong><span>{{ t(`channels.providers.${channel.provider}`) }} · {{ channel.account_name }}</span><el-tag>{{ t('channels.configured') }}</el-tag></div>
      <p v-if="!channel.enabled">{{ t(`channels.states.${channel.validation_state}`) }}</p>
      <p v-if="channel.error_code" class="muted">{{ t('channels.connectionError') }}</p>
      <p v-if="channel.callback_url" class="channel-callback">{{ t('channels.callback') }} <code>{{ channel.callback_url }}</code></p>
      <p v-if="channel.validation_state === 'testing'" role="status">{{ t(channelSetups[channel.provider]?.audience === 'direct' ? 'channels.verifyDirectInstruction' : 'channels.verifyInstruction') }} <code>{{ channel.validation_code }}</code> · {{ channel.validation_until ? new Date(channel.validation_until).toLocaleTimeString() : '' }}</p>
      <div class="channel-actions">
        <el-button :disabled="busy || channel.enabled || !available" @click="edit(channel)">{{ t('common.edit') }}</el-button>
        <el-button :disabled="busy || channel.enabled || !available" @click="control(channel,'validate')">{{ t('channels.validate') }}</el-button>
        <el-button v-if="!channel.enabled" :disabled="busy || !available || channel.validation_state !== 'passed'" @click="confirmation = {channel,action:'enable'}">{{ t('channels.enable') }}</el-button>
        <el-button v-else :disabled="busy" @click="control(channel,'disable')">{{ t('channels.disable') }}</el-button>
        <el-button :disabled="busy" @click="confirmation = {channel,action:'reset'}">{{ t('channels.reset') }}</el-button>
        <el-button type="danger" :disabled="busy" @click="confirmation = {channel,action:'delete'}">{{ t('common.delete') }}</el-button>
      </div>
    </article>
  </section>
  <el-dialog v-model="dialog" :title="t('channels.configureProvider', { provider: t(`channels.providers.${form.provider}`) })" width="min(720px, calc(100vw - 32px))" align-center append-to-body :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy" @close="clearLogin" @closed="clearSecrets">
    <template #header="{ titleId, titleClass }"><h2 :id="titleId" :class="[titleClass, 'channel-dialog-title']"><MessageChannelIcon :provider="form.provider" :size="24" />{{ t('channels.configureProvider', { provider: t(`channels.providers.${form.provider}`) }) }}</h2></template>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-form class="channel-config-form" label-position="top" @submit.prevent="save">
      <section class="channel-account-setup">
        <h3>{{ t('channels.accountSetup') }}</h3>
        <p class="muted">{{ t(`channels.setup.${form.provider}`) }}</p>
        <div class="channel-actions">
          <a v-if="form.provider === 'telegram'" class="channel-botfather-link" href="https://t.me/BotFather" target="_blank" rel="noopener noreferrer">{{ t('channels.botFather') }}</a>
          <a :href="setup.docs" target="_blank" rel="noopener noreferrer">{{ t('channels.setupGuide') }}</a>
        </div>
        <el-form-item v-if="form.provider === 'feishu'" :label="t('channels.region')"><el-select v-model="form.region" :disabled="busy || Boolean(login)" @change="reconnect"><el-option value="feishu" :label="t('channels.providers.feishu')" /><el-option value="lark" label="Lark" /></el-select></el-form-item>
        <template v-if="authenticated">
          <p role="status">{{ t(editing && !reconnecting ? 'channels.savedAccount' : 'channels.accountConnected') }} · {{ login?.account_name || editing?.account_name || login?.account_id || editing?.account_id }}</p>
          <el-button :disabled="busy" @click="reconnect">{{ t('channels.reconnect') }}</el-button>
        </template>
        <template v-else>
          <div v-if="setup.qr && fields.length" class="channel-actions">
            <el-button :disabled="busy || loginMethod === 'qr'" @click="chooseLoginMethod('qr')">{{ t('channels.qrLogin') }}</el-button>
            <el-button :disabled="busy || loginMethod === 'credentials'" @click="chooseLoginMethod('credentials')">{{ t('channels.manualLogin') }}</el-button>
          </div>
          <template v-if="loginMethod === 'credentials'">
            <el-form-item v-for="field in fields" :key="field" :label="t(`channels.fields.${field}`)"><el-input v-model="form.credentials[field]" :type="field.includes('secret') || field.includes('token') || field === 'password' ? 'password' : 'text'" :disabled="busy" autocomplete="off" /></el-form-item>
            <el-button type="primary" :loading="busy" :disabled="!credentialComplete" @click="connectAccount">{{ t('channels.connectAccount') }}</el-button>
          </template>
          <template v-else>
            <div v-if="qrImage" class="channel-qr"><img :src="qrImage" :alt="t('channels.qrAlt', {provider:t(`channels.providers.${form.provider}`)})" width="240" height="240" /></div>
            <p v-if="login" role="status">{{ t(`channels.loginStates.${login.status}`) }}</p>
            <p v-else class="muted">{{ t('channels.qrInstruction', {provider:form.provider === 'qqbot' ? 'QQ' : t(`channels.providers.${form.provider}`)}) }}</p>
            <template v-if="login?.status === 'verification_required'">
              <el-form-item :label="t('channels.verificationCode')"><el-input v-model="verificationCode" autocomplete="off" maxlength="32" /></el-form-item>
              <el-button :disabled="polling || !verificationCode.trim()" @click="pollLogin()">{{ t('common.confirm') }}</el-button>
            </template>
            <el-button :loading="busy" :disabled="Boolean(login && !['expired','failed'].includes(login.status))" @click="connectAccount">{{ t(login ? 'channels.refreshQR' : 'channels.showQR') }}</el-button>
          </template>
        </template>
      </section>
      <section v-if="authenticated" class="channel-message-setup">
        <h3>{{ t('channels.messageSetup') }}</h3>
        <p class="muted">{{ t(`channels.receive.${setup.receive}`) }}</p>
        <el-form-item :label="t('workflows.name')"><el-input v-model="form.name" :disabled="busy" maxlength="100" /></el-form-item>
        <el-form-item :label="t('channels.senderLabel', {kind:setup.sender})"><el-input v-model="form.senders" type="textarea" :disabled="busy" :placeholder="setup.sender" /></el-form-item>
        <div v-if="form.provider === 'feishu'" class="channel-sender-pairing">
          <el-button v-if="!['connecting','waiting'].includes(login?.pairing_status ?? '')" :loading="busy" :disabled="busy || polling" @click="startSenderPairing">{{ t('channels.pairing.generate') }}</el-button>
          <p v-if="login?.pairing_status" role="status">{{ t(`channels.pairing.${login.pairing_status}`) }}</p>
          <template v-if="login?.pairing_status === 'waiting' && login.pairing_code">
            <p>{{ t('channels.pairing.instruction') }}</p>
            <code class="channel-pairing-code">{{ login.pairing_code }}</code>
            <el-button :disabled="busy" @click="copyPairingCode">{{ t('channels.pairing.copy') }}</el-button>
            <p class="muted">{{ t('channels.pairing.expires', { time: login.pairing_expires_at ? new Date(login.pairing_expires_at).toLocaleTimeString() : '' }) }}</p>
          </template>
          <template v-if="login?.pairing_status === 'recognized' && login.suggested_sender_id">
            <code>{{ login.suggested_sender_id }}</code>
            <el-button :disabled="busy || ids(form.senders).includes(login.suggested_sender_id)" @click="acceptPairedSender">{{ t('channels.pairing.accept') }}</el-button>
          </template>
        </div>
        <el-checkbox v-if="!directOnly && !roomsOnly" v-model="form.direct" :disabled="busy">{{ t('channels.allowDirect') }}</el-checkbox>
        <p v-if="directOnly" class="muted">{{ t('channels.directOnly') }}</p>
        <el-form-item v-if="!directOnly" :label="t(roomsOnly ? 'channels.roomLabel' : 'channels.groupLabel', {kind:setup.group})"><el-input v-model="form.groups" type="textarea" :disabled="busy" :placeholder="setup.group" /></el-form-item>
        <p class="muted">{{ t(roomsOnly ? 'channels.roomAudience' : directOnly ? 'channels.directAudience' : 'channels.audienceHint') }}</p>
      </section>
    </el-form>
    <template #footer><el-button :disabled="busy" @click="closeConfiguration">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!canSave" @click="save">{{ t('common.save') }}</el-button></template>
  </el-dialog>
  <ConfirmDialog :open="Boolean(confirmation)" :title="t(`channels.${confirmation?.action ?? 'enable'}`)" :message="confirmText" :confirm-label="t('common.confirm')" :cancel-label="t('common.cancel')" :busy="busy" :danger="confirmation?.action === 'delete'" @confirm="confirm" @cancel="confirmation = undefined" />
</template>

<style scoped>
.message-channels { display:grid; gap:var(--aw-space-3); padding:var(--aw-space-4); min-width:0; }
.channel-provider-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,220px),1fr)); gap:var(--aw-space-3); }
.channel-provider-card { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--aw-space-3); padding:var(--aw-space-4); border:1px solid var(--aw-n4); border-radius:var(--aw-radius-input); background:var(--aw-n0); color:var(--aw-n10); text-align:left; font:inherit; cursor:pointer; }
.channel-provider-card strong { overflow-wrap:anywhere; }
.channel-provider-identity { display:flex; align-items:center; gap:var(--aw-space-2); min-width:0; }
.channel-provider-action { flex-shrink:0; color:var(--aw-primary); font-size:var(--aw-font-size-body); }
.channel-dialog-title { display:flex; align-items:center; gap:var(--aw-space-2); margin:0; }
.channel-sender-pairing { display:flex; flex-wrap:wrap; align-items:center; gap:var(--aw-space-2); margin-bottom:var(--aw-space-4); }
.channel-sender-pairing p { width:100%; margin:0; }
.channel-pairing-code { overflow-wrap:anywhere; user-select:all; }
.channel-provider-card:hover:not(:disabled) { background:var(--aw-n2); border-color:var(--aw-primary); }
.channel-provider-card:focus-visible { outline:2px solid var(--aw-primary); outline-offset:2px; }
.channel-provider-card:disabled { cursor:not-allowed; opacity:.55; }
.channel-config-form { max-height:min(65vh,640px); overflow-y:auto; padding-inline-end:var(--aw-space-2); }
.channel-botfather-link { font-weight:600; }
.channel-account-setup,.channel-message-setup { display:grid; gap:var(--aw-space-3); }
.channel-message-setup { margin-top:var(--aw-space-4); padding-top:var(--aw-space-4); border-top:1px solid var(--aw-n4); }
.channel-qr { width:240px; max-width:100%; }
.channel-qr img { display:block; max-width:100%; height:auto; }
.channel-actions,.channel-heading { display:flex; flex-wrap:wrap; align-items:center; gap:var(--aw-space-2); }
.channel-card { padding:var(--aw-space-4); border:1px solid var(--aw-n4); border-radius:var(--aw-radius-card); }
.channel-callback { overflow-wrap:anywhere; }
</style>
