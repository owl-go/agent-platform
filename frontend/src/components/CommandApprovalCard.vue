<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import type { CommandApproval } from "../api/client";

const props = defineProps<{ approval: CommandApproval; busy?: boolean }>();
const emit = defineEmits<{ decide: [decision: "approved" | "rejected", identity?: "user" | "bot"] }>();
const { t } = useI18n();
const identity = ref<"user" | "bot">(props.approval.identity ?? "user");
function decide(decision: "approved" | "rejected") {
  emit("decide", decision, decision === "approved" ? identity.value : undefined);
}
</script>

<template>
  <section class="connector-action-card connector-approval-card" aria-live="polite">
    <div>
      <strong>{{ t('approvals.title') }} · {{ approval.connector_name }}</strong>
      <span>{{ approval.operation }}<template v-if="approval.target"> · {{ approval.target }}</template></span>
      <small v-if="approval.external_display_name">{{ t('approvals.externalAccount', { account: approval.external_display_name }) }}</small>
      <small v-if="approval.redacted_arguments">{{ t('approvals.input', { input: approval.redacted_arguments }) }}</small>
      <small>{{ t('approvals.expires', { time: new Date(approval.expires_at).toLocaleTimeString() }) }}</small>
    </div>
    <div class="connector-action-buttons">
      <select v-if="!approval.identity" v-model="identity" :aria-label="t('approvals.identity')"><option value="user">{{ t('approvals.user') }}</option><option value="bot">{{ t('approvals.bot') }}</option></select>
      <el-button size="small" :disabled="busy" @click="decide('rejected')">{{ t('approvals.reject') }}</el-button>
      <el-button size="small" type="primary" :loading="busy" @click="decide('approved')">{{ t('approvals.approveOnce') }}</el-button>
    </div>
  </section>
</template>
