<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type CommandApproval } from "../api/client";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const approvals = ref<CommandApproval[]>([]);
const identities = ref<Record<string, "user" | "bot">>({});
let poll: number | undefined;
async function refresh() { try { approvals.value = await api.listCommandApprovals(); for (const item of approvals.value) identities.value[item.id] = item.identity ?? identities.value[item.id] ?? "user"; } catch { /* The page shell owns global connectivity feedback. */ } }
async function decide(item: CommandApproval, decision: "approved" | "rejected") { await api.decideCommandApproval(item.id, decision, decision === "approved" ? (identities.value[item.id] ?? "user") : undefined, item.version); await refresh(); }
onMounted(() => { void refresh(); poll = window.setInterval(refresh, 5000); });
onBeforeUnmount(() => { if (poll !== undefined) window.clearInterval(poll); });
</script>

<template>
  <aside v-if="approvals.length" class="approval-inbox" aria-live="polite">
    <article v-for="item in approvals" :key="item.id" class="approval-card">
      <div><strong>{{ t('approvals.title') }} · {{ item.connector_name }}</strong><p>{{ item.operation }} · {{ item.target }}</p><code>{{ item.redacted_arguments }}</code><small>{{ t('approvals.expires', { time: new Date(item.expires_at).toLocaleTimeString() }) }}</small></div>
      <select v-model="identities[item.id]" :disabled="Boolean(item.identity)" :aria-label="t('approvals.identity')"><option value="user">{{ t('approvals.user') }}</option><option value="bot">{{ t('approvals.bot') }}</option></select>
      <el-button @click="decide(item, 'rejected')">{{ t('approvals.reject') }}</el-button><el-button type="primary" @click="decide(item, 'approved')">{{ t('approvals.approveOnce') }}</el-button>
    </article>
  </aside>
</template>
