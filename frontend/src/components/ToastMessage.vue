<script setup lang="ts">
import { CircleCheck, TriangleAlert, X } from "@lucide/vue";
import { onBeforeUnmount, watch } from "vue";

const props = withDefaults(defineProps<{
  kind: "success" | "error";
  title: string;
  message: string;
  closeLabel: string;
  duration?: number;
}>(), { duration: 5000 });
const emit = defineEmits<{ dismiss: [] }>();
let timeout: number | undefined;

function scheduleDismiss() {
  if (timeout !== undefined) window.clearTimeout(timeout);
  if (props.message && props.duration > 0) timeout = window.setTimeout(() => emit("dismiss"), props.duration);
}

watch(() => [props.message, props.duration], scheduleDismiss, { immediate: true });
onBeforeUnmount(() => { if (timeout !== undefined) window.clearTimeout(timeout); });
</script>

<template>
  <Transition name="toast-pop" appear>
    <aside v-if="message" class="app-toast" :class="kind" :role="kind === 'error' ? 'alert' : 'status'" :aria-live="kind === 'error' ? 'assertive' : 'polite'">
      <span class="toast-symbol"><CircleCheck v-if="kind === 'success'" :size="19" /><TriangleAlert v-else :size="19" /></span>
      <span class="toast-copy"><strong>{{ title }}</strong><span>{{ message }}</span></span>
      <button type="button" :aria-label="closeLabel" @click="emit('dismiss')"><X :size="16" /></button>
      <i v-if="duration > 0" class="toast-life" :style="{ animationDuration: `${duration}ms` }"></i>
    </aside>
  </Transition>
</template>

<style scoped>
.app-toast { position: fixed; z-index: 120; top: 22px; right: 22px; width: min(390px, calc(100vw - 32px)); min-height: 70px; display: grid; grid-template-columns: 34px minmax(0, 1fr) 28px; align-items: start; gap: 10px; overflow: hidden; padding: 15px 13px 13px 15px; border: 1px solid; border-radius: 14px; background: rgba(255, 255, 255, .96); box-shadow: 0 18px 48px rgba(23, 32, 29, .18); backdrop-filter: blur(14px); }
.app-toast.success { color: #185d45; border-color: #b9d6c6; }
.app-toast.error { color: #8c3526; border-color: #e6bdb4; }
.toast-symbol { width: 32px; height: 32px; display: grid; place-items: center; border-radius: 10px; background: #e4f1e9; }
.error .toast-symbol { background: #f9e3de; }
.toast-copy { min-width: 0; display: grid; gap: 3px; padding-top: 1px; }
.toast-copy strong { color: #17201d; font-size: .78rem; }
.toast-copy > span { color: #5e6863; font-size: .76rem; line-height: 1.45; overflow-wrap: anywhere; }
button { width: 28px; height: 28px; display: grid; place-items: center; border: 0; border-radius: 8px; color: #68716d; background: transparent; cursor: pointer; }
button:hover { background: #edf0eb; color: #17201d; }
.toast-life { position: absolute; left: 0; right: 0; bottom: 0; height: 3px; background: currentColor; transform-origin: left; animation: toast-life linear forwards; opacity: .45; }
.toast-pop-enter-active, .toast-pop-leave-active { transition: opacity .2s ease, transform .24s cubic-bezier(.2, .8, .2, 1); }
.toast-pop-enter-from, .toast-pop-leave-to { opacity: 0; transform: translateY(-10px) scale(.98); }
@keyframes toast-life { to { transform: scaleX(0); } }
@media (max-width: 640px) { .app-toast { top: 12px; right: 16px; } }
@media (prefers-reduced-motion: reduce) { .toast-pop-enter-active, .toast-pop-leave-active { transition: none; }.toast-life { animation: none; } }
</style>
