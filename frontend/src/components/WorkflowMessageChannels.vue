<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type ChannelDelivery, type MessageChannel, type MessageChannelInput } from "../api/client";
import ConfirmDialog from "./ConfirmDialog.vue";

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
const deliveries = ref<ChannelDelivery[]>([]);
const deliveryChannel = ref<MessageChannel>();
const confirmation = ref<{ channel: MessageChannel; action: string; delivery?: ChannelDelivery }>();
const form = reactive({ name: "", provider: "telegram", region: "feishu", senders: "", groups: "", direct: true, credentials: {} as Record<string,string> });
const providers = ["telegram", "discord", "slack", "dingtalk", "feishu"];
const fields = computed(() => form.provider === "dingtalk" ? ["client_id", "client_secret", "corp_id"] : form.provider === "feishu" ? ["app_id", "app_secret", "tenant_key"] : form.provider === "slack" ? ["bot_token", "signing_secret"] : ["bot_token"]);
const ids = (text: string) => [...new Set(text.split(/[,\n]/).map(id => id.trim()).filter(Boolean))];
const canSave = computed(() => form.name.trim() && ids(form.senders).length > 0 && (form.direct || ids(form.groups).length > 0));
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
    if (deliveryChannel.value) deliveryChannel.value = items.value.find(c => c.id === deliveryChannel.value?.id);
    error.value = "";
  } catch { if (!disposed) error.value = t("errors.generic"); }
  finally {
    loading.value = false;
    clearTimeout(timer);
    if (!disposed && items.value.some(c => c.validation_state === "testing")) timer = setTimeout(refresh, 5000);
  }
}
function edit(channel?: MessageChannel) {
  editing.value = channel;
  Object.assign(form, { name: channel?.name ?? "", provider: channel?.provider ?? "telegram", region: channel?.region || "feishu", senders: channel?.audience.sender_ids.join("\n") ?? "", groups: channel?.audience.group_ids.join("\n") ?? "", direct: channel?.audience.allow_direct ?? true, credentials: {} });
  error.value = ""; dialog.value = true;
}
watch(() => form.provider, () => { form.credentials = {}; });
function clearSecrets() { form.credentials = {}; }
async function save() {
  if (!canSave.value || busy.value) return;
  busy.value = true;
  const credentials = Object.fromEntries(Object.entries(form.credentials).filter(([,v]) => v.trim() !== ""));
  const input: MessageChannelInput = { channel_id: editing.value?.id, version: editing.value?.version ?? 0, provider: form.provider, name: form.name.trim(), region: form.provider === "feishu" ? form.region : "", audience: { sender_ids:ids(form.senders), group_ids:ids(form.groups), allow_direct:form.direct }, credentials };
  try { await api.saveMessageChannel(props.workflowId, input, abort.signal); clearSecrets(); dialog.value = false; await refresh(); }
  catch { if (!disposed) error.value = t("channels.saveFailed"); }
  finally { busy.value = false; }
}
async function control(channel: MessageChannel, action: string) {
  if (busy.value) return; busy.value = true;
  try { await api.controlMessageChannel(props.workflowId, channel.id, channel.version, action, abort.signal); confirmation.value = undefined; await refresh(); }
  catch { if (!disposed) error.value = t("channels.actionFailed"); }
  finally { busy.value = false; }
}
async function showDeliveries(channel: MessageChannel) {
  busy.value = true; error.value = "";
  try { deliveries.value = await api.listChannelDeliveries(props.workflowId, channel.id, abort.signal); deliveryChannel.value = channel; }
  catch { if (!disposed) error.value = t("errors.generic"); }
  finally { busy.value = false; }
}
async function confirm() {
  const pending = confirmation.value; if (!pending) return;
  if (!pending.delivery) { await control(pending.channel, pending.action); return; }
  busy.value = true;
  try { await api.retryChannelDelivery(props.workflowId, pending.channel.id, pending.delivery.id, pending.channel.version, pending.delivery.state === "outcome_unknown", abort.signal); confirmation.value = undefined; await showDeliveries(pending.channel); }
  catch { if (!disposed) error.value = t("channels.actionFailed"); }
  finally { busy.value = false; }
}
const confirmText = computed(() => confirmation.value?.delivery ? t(confirmation.value.delivery.state === "outcome_unknown" ? "channels.unknownHint" : "channels.retryHint") : t(`channels.${confirmation.value?.action ?? "enable"}Hint`));
onMounted(refresh);
onBeforeUnmount(() => { disposed = true; abort.abort(); clearTimeout(timer); clearSecrets(); });
</script>

<template>
  <section class="message-channels" :aria-label="t('channels.title')">
    <p class="muted">{{ t('channels.description') }}</p>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-alert v-if="!loading && !available" :title="t('channels.platformDisabled')" type="info" :closable="false" />
    <div class="channel-actions"><el-button :loading="loading" :disabled="busy" @click="refresh">{{ t('common.refresh') }}</el-button><el-button type="primary" :disabled="!available || busy || loading" @click="edit()">{{ t('channels.add') }}</el-button></div>
    <p v-if="loading && !items.length" role="status">{{ t('common.loading') }}</p>
    <el-empty v-else-if="!items.length" :description="t('channels.empty')" />
    <article v-for="channel in items" :key="channel.id" class="channel-card">
      <div class="channel-heading"><strong>{{ channel.name }}</strong><span>{{ t(`channels.providers.${channel.provider}`) }} · {{ channel.account_name }}</span><el-tag>{{ t(channel.enabled ? 'common.enabled' : 'common.disabled') }}</el-tag></div>
      <p>{{ t(`channels.states.${channel.validation_state}`) }} · {{ t(`channels.health.${channel.health}`) }}</p>
      <p v-if="channel.error_code" class="muted">{{ t('channels.connectionError') }}</p>
      <p v-if="channel.callback_url" class="channel-callback">{{ t('channels.callback') }} <code>{{ channel.callback_url }}</code></p>
      <p v-if="channel.validation_state === 'testing'" role="status">{{ t('channels.verifyInstruction') }} <code>{{ channel.validation_code }}</code> · {{ channel.validation_until ? new Date(channel.validation_until).toLocaleTimeString() : '' }}</p>
      <div class="channel-actions">
        <el-button :disabled="busy || channel.enabled || !available" @click="edit(channel)">{{ t('common.edit') }}</el-button>
        <el-button :disabled="busy || channel.enabled || !available" @click="control(channel,'validate')">{{ t('channels.validate') }}</el-button>
        <el-button v-if="!channel.enabled" :disabled="busy || !available || channel.validation_state !== 'passed'" @click="confirmation = {channel,action:'enable'}">{{ t('channels.enable') }}</el-button>
        <el-button v-else :disabled="busy" @click="control(channel,'disable')">{{ t('channels.disable') }}</el-button>
        <el-button :disabled="busy" @click="showDeliveries(channel)">{{ t('channels.deliveries') }}</el-button>
        <el-button :disabled="busy" @click="confirmation = {channel,action:'reset'}">{{ t('channels.reset') }}</el-button>
        <el-button type="danger" :disabled="busy" @click="confirmation = {channel,action:'delete'}">{{ t('common.delete') }}</el-button>
      </div>
    </article>
    <section v-if="deliveryChannel" class="channel-deliveries">
      <strong>{{ deliveryChannel.name }} · {{ t('channels.deliveries') }}</strong><p class="muted">{{ t('channels.retryHint') }}</p>
      <el-empty v-if="!deliveries.length" :description="t('common.empty')" />
      <div v-for="delivery in deliveries" :key="delivery.id" class="channel-delivery">
        <span>{{ t(`channels.deliveryStates.${delivery.state}`) }} · {{ t('channels.chunk', {number:delivery.chunk}) }} · {{ new Date(delivery.created_at).toLocaleString() }}</span>
        <el-button v-if="delivery.state === 'failed' || delivery.state === 'outcome_unknown'" :disabled="busy || !deliveryChannel.enabled" @click="confirmation = {channel:deliveryChannel,action:'retry',delivery}">{{ t('channels.retry') }}</el-button>
      </div>
    </section>
  </section>
  <el-dialog v-model="dialog" :title="t(editing ? 'channels.edit' : 'channels.add')" append-to-body :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy" @closed="clearSecrets">
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-form label-position="top" @submit.prevent="save">
      <el-form-item :label="t('workflows.name')"><el-input v-model="form.name" :disabled="busy" maxlength="100" /></el-form-item>
      <el-form-item :label="t('channels.provider')"><el-select v-model="form.provider" :disabled="busy || Boolean(editing)"><el-option v-for="provider in providers" :key="provider" :value="provider" :label="t(`channels.providers.${provider}`)" /></el-select></el-form-item>
      <el-form-item v-if="form.provider === 'feishu'" :label="t('channels.region')"><el-select v-model="form.region" :disabled="busy"><el-option value="feishu" :label="t('channels.providers.feishu')" /><el-option value="lark" label="Lark" /></el-select></el-form-item>
      <p class="muted">{{ t(`channels.setup.${form.provider}`) }}</p>
      <p v-if="editing" class="muted">{{ t('channels.keepCredentials') }}</p>
      <el-form-item v-for="field in fields" :key="field" :label="t(`channels.fields.${field}`)"><el-input v-model="form.credentials[field]" :type="field.includes('secret') || field.includes('token') ? 'password' : 'text'" :disabled="busy" autocomplete="off" /></el-form-item>
      <el-form-item :label="t('channels.senders')"><el-input v-model="form.senders" type="textarea" :disabled="busy" :placeholder="t('channels.idsHint')" /></el-form-item>
      <el-checkbox v-model="form.direct" :disabled="busy">{{ t('channels.allowDirect') }}</el-checkbox>
      <el-form-item :label="t('channels.groups')"><el-input v-model="form.groups" type="textarea" :disabled="busy" :placeholder="t('channels.idsHint')" /></el-form-item>
      <p class="muted">{{ t('channels.audienceHint') }}</p>
    </el-form>
    <template #footer><el-button :disabled="busy" @click="dialog = false; clearSecrets()">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="busy" :disabled="!canSave" @click="save">{{ t('common.save') }}</el-button></template>
  </el-dialog>
  <ConfirmDialog :open="Boolean(confirmation)" :title="t(confirmation?.delivery ? 'channels.retry' : `channels.${confirmation?.action ?? 'enable'}`)" :message="confirmText" :confirm-label="t('common.confirm')" :cancel-label="t('common.cancel')" :busy="busy" :danger="confirmation?.action === 'delete'" @confirm="confirm" @cancel="confirmation = undefined" />
</template>

<style scoped>
.message-channels { display:grid; gap:var(--aw-space-3); }
.channel-actions,.channel-heading { display:flex; flex-wrap:wrap; align-items:center; gap:var(--aw-space-2); }
.channel-card { padding:var(--aw-space-4); border:1px solid var(--aw-n4); border-radius:var(--aw-radius-card); }
.channel-callback { overflow-wrap:anywhere; }
.channel-delivery { display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; gap:var(--aw-space-2); padding-block:var(--aw-space-2); }
</style>
