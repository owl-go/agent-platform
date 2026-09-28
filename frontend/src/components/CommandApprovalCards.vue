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
    <article v-for="item in items" :key="item.id" class="approval-card" role="alertdialog" :aria-label="`${item.operation} · ${item.target}`">
      <header>{{ item.operation }} · {{ item.target }}</header>
      <div class="approval-card-body">
        <p class="approval-connector">{{ item.connector_name }} · {{ t('approvals.expires', { time: new Date(item.expires_at).toLocaleTimeString() }) }}</p>
        <dl class="approval-impact">
          <div><dt>{{ t('approvals.impact') }}</dt><dd>{{ item.target }}</dd></div>
          <div><dt>{{ t('approvals.irreversible') }}</dt><dd>{{ t('approvals.irreversibleHint') }}</dd></div>
          <div><dt>{{ t('approvals.basis') }}</dt><dd><code>{{ item.redacted_arguments }}</code></dd></div>
        </dl>
      </div>
      <footer>
        <select :value="identities[item.id] ?? item.identity ?? 'user'" :disabled="Boolean(item.identity)" :aria-label="t('approvals.identity')" @change="changeIdentity(item.id, $event)"><option value="user">{{ t('approvals.user') }}</option><option value="bot">{{ t('approvals.bot') }}</option></select>
        <el-button type="warning" @click="emit('decide', item, 'approved')">{{ t('approvals.approveOnce') }}</el-button>
        <el-button class="approval-reject" @click="emit('decide', item, 'rejected')">{{ t('approvals.reject') }}</el-button>
      </footer>
    </article>
  </aside>
</template>
