<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { CommandApproval } from "../api/client";

defineProps<{ items: CommandApproval[]; identities: Record<string, "user" | "bot">; decidingId?: string; embedded?: boolean }>();
const emit = defineEmits<{ decide: [item: CommandApproval, decision: "approved" | "rejected"]; identity: [approvalID: string, identity: "user" | "bot"] }>();
const { t } = useI18n();

function changeIdentity(approvalID: string, event: Event) {
  emit("identity", approvalID, (event.target as HTMLSelectElement).value as "user" | "bot");
}

function isMessageSend(operation: string) {
  return ["im_messages_send", "send message", "message send"].includes(operation.trim().toLowerCase());
}

function isTaskCreate(operation: string) { return operation.trim().toLowerCase() === "task_create"; }

function operationLabel(operation: string) {
  if (isMessageSend(operation)) return t("approvals.sendMessage");
  if (isTaskCreate(operation)) return t("approvals.createTask");
  return operation;
}
</script>

<template>
  <aside v-if="items.length" class="approval-inbox" :class="{ 'is-embedded': embedded }" aria-live="polite">
    <article v-for="item in items" :key="item.id" class="approval-card">
      <div class="approval-card-content">
        <strong>{{ t('approvals.title', { connector: item.connector_name }) }}</strong>
        <p>{{ isMessageSend(item.operation) ? t('approvals.messageSendDescription', { connector: item.connector_name }) : isTaskCreate(item.operation) ? t('approvals.taskCreateDescription', { connector: item.connector_name }) : t('approvals.otherDescription', { connector: item.connector_name }) }}</p>
        <p><b>{{ t('approvals.operation') }}：</b>{{ operationLabel(item.operation) }}</p>
        <p><b>{{ item.target.startsWith('oc_') ? t('approvals.groupTarget') : t('approvals.target') }}：</b>{{ item.target }}</p>
        <p class="approval-card-caution">{{ t('approvals.hiddenArguments') }}</p>
        <small>{{ t('approvals.expires', { time: new Date(item.expires_at).toLocaleTimeString() }) }}</small>
      </div>
      <div class="approval-card-actions">
        <label class="approval-identity"><span>{{ t('approvals.identity') }}</span><select :value="identities[item.id] ?? item.identity ?? 'user'" :disabled="Boolean(item.identity || decidingId)" @change="changeIdentity(item.id, $event)"><option value="user">{{ t('approvals.user') }}</option><option value="bot">{{ t('approvals.bot') }}</option></select></label>
        <el-button :disabled="Boolean(decidingId)" @click="emit('decide', item, 'rejected')">{{ t('approvals.reject') }}</el-button>
        <el-button type="primary" :loading="decidingId === item.id" :disabled="Boolean(decidingId)" @click="emit('decide', item, 'approved')">{{ t('approvals.approveOnce') }}</el-button>
      </div>
    </article>
  </aside>
</template>
