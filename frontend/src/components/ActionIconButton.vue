<script setup lang="ts">
withDefaults(defineProps<{
  label: string;
  tooltip?: string;
  tone?: "default" | "danger";
}>(), {
  tooltip: undefined,
  tone: "default",
});
const emit = defineEmits<{ click: [event: MouseEvent] }>();
</script>

<template>
  <el-tooltip :content="tooltip ?? label" placement="top" :show-after="350">
    <el-button circle text class="action-icon-button" :class="`action-icon-button--${tone}`" :aria-label="label" @click="emit('click', $event)"><slot /><span class="action-icon-tooltip" role="tooltip">{{ tooltip ?? label }}</span></el-button>
  </el-tooltip>
</template>

<style scoped>
.action-icon-button {
  position: relative;
  width: 28px;
  height: 28px;
  display: inline-grid;
  place-items: center;
  flex: 0 0 auto;
  padding: 0;
  border: 0;
  border-radius: 7px;
  color: #64706a;
  background: transparent;
  cursor: pointer;
  transition: color .16s ease, background-color .16s ease, transform .16s ease;
}

.action-icon-button :deep(svg) {
  width: 15px;
  height: 15px;
  stroke-width: 1.8;
}

.action-icon-button:hover,
.action-icon-button:focus-visible {
  color: #275e48;
  background: #fff;
  outline: none;
  transform: translateY(-1px);
}

.action-icon-button:focus-visible {
  box-shadow: 0 0 0 2px #b8d1c1;
}

.action-icon-button--danger:hover,
.action-icon-button--danger:focus-visible {
  color: #a44232;
  background: #fff1ed;
}

.action-icon-tooltip {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

@media (prefers-reduced-motion: reduce) {
  .action-icon-button { transition: none; }
}
</style>
