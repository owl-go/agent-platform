<script setup lang="ts">
withDefaults(defineProps<{
  source: string;
  permission: string;
  status: string;
  statusTone?: "success" | "warning" | "danger" | "info";
  detail?: string;
}>(), { statusTone: "info", detail: "" });
</script>

<template>
  <div class="resource-trust-meta">
    <span data-kind="source">{{ source }}</span>
    <span data-kind="permission">{{ permission }}</span>
    <span data-kind="status" :data-tone="statusTone">{{ status }}</span>
    <small v-if="detail">{{ detail }}</small>
  </div>
</template>

<style scoped>
.resource-trust-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 5px; min-width: 0; margin-top: 2px; }
.resource-trust-meta span { padding: 2px 6px; border: 1px solid var(--aw-n4); border-radius: 6px; color: var(--aw-n7); background: var(--aw-n2); font-size: .64rem; line-height: 1.35; }
.resource-trust-meta span[data-tone="success"] { border-color: var(--aw-success-border); color: var(--aw-success); background: var(--aw-success-soft); }
.resource-trust-meta span[data-tone="warning"] { border-color: var(--aw-warning-border); color: var(--aw-warning); background: var(--aw-warning-soft); }
.resource-trust-meta span[data-tone="danger"] { border-color: var(--aw-danger-border); color: var(--aw-danger); background: var(--aw-danger-soft); }
.resource-trust-meta small { flex-basis: 100%; overflow: hidden; color: var(--aw-n6); font-size: .64rem; line-height: 1.45; text-overflow: ellipsis; white-space: nowrap; }
</style>
