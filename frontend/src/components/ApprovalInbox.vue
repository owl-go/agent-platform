<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type CommandApproval } from "../api/client";
import { embeddedSessionApprovalID } from "../commandApprovalPlacement";
import CommandApprovalCards from "./CommandApprovalCards.vue";

const api = inject(platformApiKey)!;
const { t } = useI18n();
const approvals = ref<CommandApproval[]>([]);
const identities = ref<Record<string, "user" | "bot">>({});
const decidingID = ref<string>();
const decisionNotice = ref<{ kind: "success" | "error"; message: string; executionKind: CommandApproval["execution_kind"]; executionID: string }>();
const embeddedApprovals = computed(() => approvals.value.filter((item) => item.execution_kind === "session" && item.execution_id === embeddedSessionApprovalID.value));
const globalApprovals = computed(() => approvals.value.filter((item) => !embeddedApprovals.value.includes(item)));
const embeddedNotice = computed(() => decisionNotice.value?.executionKind === "session" && decisionNotice.value.executionID === embeddedSessionApprovalID.value ? decisionNotice.value : undefined);
const globalNotice = computed(() => embeddedNotice.value ? undefined : decisionNotice.value);
let poll: number | undefined;
let noticeTimer: number | undefined;
const settledIDs = new Set<string>();
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
    approvals.value = latest.filter((item) => item.state === "pending" && !settledIDs.has(item.id));
    for (const item of approvals.value) identities.value[item.id] = item.identity ?? identities.value[item.id] ?? "user";
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
async function decide(item: CommandApproval, decision: "approved" | "rejected") {
  if (decidingID.value) return;
  decidingID.value = item.id;
  if (noticeTimer !== undefined) window.clearTimeout(noticeTimer);
  decisionNotice.value = undefined;
  const notice = (kind: "success" | "error", message: string) => {
    decisionNotice.value = { kind, message, executionKind: item.execution_kind, executionID: item.execution_id };
    noticeTimer = window.setTimeout(() => { decisionNotice.value = undefined; noticeTimer = undefined; }, 10_000);
  };
  try {
    await api.decideCommandApproval(item.id, decision, decision === "approved" ? (identities.value[item.id] ?? "user") : undefined, item.version);
    settledIDs.add(item.id);
    approvals.value = approvals.value.filter((approval) => approval.id !== item.id);
    notice("success", t(decision === "approved" ? "approvals.approvedNotice" : "approvals.rejectedNotice"));
    void refresh(true);
  } catch (error) {
    if (error instanceof ApiError && (error.kind === "conflict" || error.kind === "not_found")) {
      notice("error", t("approvals.staleNotice"));
      void refresh(true);
    } else {
      notice("error", t("approvals.failedNotice"));
    }
  } finally {
    decidingID.value = undefined;
  }
}
function setIdentity(approvalID: string, identity: "user" | "bot") { identities.value[approvalID] = identity; }
onMounted(() => { document.addEventListener("visibilitychange", onVisibilityChange); void refresh(); });
watch(embeddedSessionApprovalID, () => void refresh(true));
onBeforeUnmount(() => { disposed = true; clearPoll(); if (noticeTimer !== undefined) window.clearTimeout(noticeTimer); document.removeEventListener("visibilitychange", onVisibilityChange); });
</script>

<template>
  <CommandApprovalCards :items="globalApprovals" :identities="identities" :deciding-id="decidingID" @decide="decide" @identity="setIdentity" />
  <p v-if="globalNotice" class="approval-decision-notice" :class="globalNotice.kind" :role="globalNotice.kind === 'error' ? 'alert' : 'status'">{{ globalNotice.message }}</p>
  <Teleport v-if="embeddedApprovals.length || embeddedNotice" to="#session-command-approval-slot">
    <CommandApprovalCards embedded :items="embeddedApprovals" :identities="identities" :deciding-id="decidingID" @decide="decide" @identity="setIdentity" />
    <p v-if="embeddedNotice" class="approval-decision-notice" :class="embeddedNotice.kind" :role="embeddedNotice.kind === 'error' ? 'alert' : 'status'">{{ embeddedNotice.message }}</p>
  </Teleport>
</template>
