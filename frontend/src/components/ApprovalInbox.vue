<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { platformApiKey, type CommandApproval } from "../api/client";
import { embeddedSessionApprovalID } from "../commandApprovalPlacement";
import CommandApprovalCards from "./CommandApprovalCards.vue";

const api = inject(platformApiKey)!;
const approvals = ref<CommandApproval[]>([]);
const identities = ref<Record<string, "user" | "bot">>({});
const embeddedApprovals = computed(() => approvals.value.filter((item) => item.execution_kind === "session" && item.execution_id === embeddedSessionApprovalID.value));
const globalApprovals = computed(() => approvals.value.filter((item) => !embeddedApprovals.value.includes(item)));
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
function setIdentity(approvalID: string, identity: "user" | "bot") { identities.value[approvalID] = identity; }
onMounted(() => { document.addEventListener("visibilitychange", onVisibilityChange); void refresh(); });
watch(embeddedSessionApprovalID, () => void refresh(true));
onBeforeUnmount(() => { disposed = true; clearPoll(); document.removeEventListener("visibilitychange", onVisibilityChange); });
</script>

<template>
  <CommandApprovalCards :items="globalApprovals" :identities="identities" @decide="decide" @identity="setIdentity" />
  <Teleport v-if="embeddedApprovals.length" to="#session-command-approval-slot"><CommandApprovalCards embedded :items="embeddedApprovals" :identities="identities" @decide="decide" @identity="setIdentity" /></Teleport>
</template>
