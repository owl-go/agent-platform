<script setup lang="ts">
withDefaults(defineProps<{
  label: string;
  tooltip?: string;
  tone?: "default" | "danger";
}>(), {
  tooltip: undefined,
  tone: "default",
});
</script>

<template>
  <button type="button" class="action-icon-button" :class="`action-icon-button--${tone}`" :aria-label="label">
    <slot />
    <span class="action-icon-tooltip" role="tooltip">{{ tooltip ?? label }}</span>
  </button>
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
  z-index: 12;
  right: 0;
  bottom: calc(100% + 8px);
  width: max-content;
  max-width: 150px;
  padding: 6px 8px;
  border-radius: 6px;
  color: #f7faf8;
  background: #25312c;
  box-shadow: 0 6px 18px rgba(22, 31, 27, .18);
  font-size: .68rem;
  font-weight: 650;
  line-height: 1;
  pointer-events: none;
  opacity: 0;
  transform: translateY(4px);
  transition: opacity .14s ease, transform .14s ease;
}

.action-icon-tooltip::after {
  content: "";
  position: absolute;
  top: 100%;
  right: 8px;
  border: 4px solid transparent;
  border-top-color: #25312c;
}

.action-icon-button:hover .action-icon-tooltip,
.action-icon-button:focus-visible .action-icon-tooltip {
  opacity: 1;
  transform: translateY(0);
}

@media (prefers-reduced-motion: reduce) {
  .action-icon-button,
  .action-icon-tooltip { transition: none; }
}
</style>
