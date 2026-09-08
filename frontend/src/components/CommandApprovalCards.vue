<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { CommandApproval } from "../api/client";

defineProps<{ items: CommandApproval[]; identities: Record<string, "user" | "bot">; embedded?: boolean }>();
const emit = defineEmits<{ decide: [item: CommandApproval, decision: "approved" | "rejected"]; identity: [approvalID: string, identity: "user" | "bot"] }>();
const { t } = useI18n();

function changeIdentity(approvalID: string, event: Event) {
  emit("identity", approvalID, (event.target as HTMLSelectElement).value as "user" | "bot");
}
</script>

<template>
  <aside v-if="items.length" class="approval-inbox" :class="{ 'is-embedded': embedded }" aria-live="polite">
    <article v-for="item in items" :key="item.id" class="approval-card">
      <div><strong>{{ t('approvals.title') }} · {{ item.connector_name }}</strong><p>{{ item.operation }} · {{ item.target }}</p><code>{{ item.redacted_arguments }}</code><small>{{ t('approvals.expires', { time: new Date(item.expires_at).toLocaleTimeString() }) }}</small></div>
      <select :value="identities[item.id] ?? item.identity ?? 'user'" :disabled="Boolean(item.identity)" :aria-label="t('approvals.identity')" @change="changeIdentity(item.id, $event)"><option value="user">{{ t('approvals.user') }}</option><option value="bot">{{ t('approvals.bot') }}</option></select>
      <el-button @click="emit('decide', item, 'rejected')">{{ t('approvals.reject') }}</el-button><el-button type="primary" @click="emit('decide', item, 'approved')">{{ t('approvals.approveOnce') }}</el-button>
    </article>
  </aside>
</template>
