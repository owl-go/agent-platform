<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type CommandApproval } from "../api/client";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const approvals = ref<CommandApproval[]>([]);
const identities = ref<Record<string, "user" | "bot">>({});
let poll: number | undefined;
let disposed = false;
let refreshing = false;
let refreshRequested = false;
function visible() { return !disposed && document.visibilityState !== "hidden"; }
function clearPoll() { if (poll !== undefined) window.clearTimeout(poll); poll = undefined; }
async function refresh(immediate = false) {
  if (!visible()) return;
  if (refreshing) { refreshRequested ||= immediate; return; }
  clearPoll(); refreshRequested = false; refreshing = true;
  try {
    const latest = await api.listCommandApprovals();
    if (disposed) return;
    approvals.value = latest;
    for (const item of latest) identities.value[item.id] = item.identity ?? identities.value[item.id] ?? "user";
  } catch { /* The page shell owns global connectivity feedback. */ }
  finally {
    refreshing = false;
    if (visible()) {
      if (refreshRequested) { refreshRequested = false; void refresh(); }
      else poll = window.setTimeout(() => { void refresh(); }, approvals.value.length ? 5000 : 30_000);
    }
  }
}
function onVisibilityChange() { clearPoll(); if (visible()) void refresh(true); }
async function decide(item: CommandApproval, decision: "approved" | "rejected") { await api.decideCommandApproval(item.id, decision, decision === "approved" ? (identities.value[item.id] ?? "user") : undefined, item.version); await refresh(true); }
onMounted(() => { document.addEventListener("visibilitychange", onVisibilityChange); void refresh(); });
onBeforeUnmount(() => { disposed = true; clearPoll(); document.removeEventListener("visibilitychange", onVisibilityChange); });
</script>

<template>
  <aside v-if="approvals.length" class="approval-inbox" aria-live="polite">
    <article v-for="item in approvals" :key="item.id" class="approval-card">
      <div><strong>{{ t('approvals.title') }} · {{ item.connector_name }}</strong><p>{{ item.operation }} · {{ item.target }}</p><small v-if="item.external_display_name">{{ t('approvals.externalAccount', { account: item.external_display_name }) }}</small><code>{{ item.redacted_arguments }}</code><small>{{ t('approvals.expires', { time: new Date(item.expires_at).toLocaleTimeString() }) }}</small></div>
      <select v-model="identities[item.id]" :disabled="Boolean(item.identity)" :aria-label="t('approvals.identity')"><option value="user">{{ t('approvals.user') }}</option><option value="bot">{{ t('approvals.bot') }}</option></select>
      <el-button @click="decide(item, 'rejected')">{{ t('approvals.reject') }}</el-button><el-button type="primary" @click="decide(item, 'approved')">{{ t('approvals.approveOnce') }}</el-button>
    </article>
  </aside>
</template>
