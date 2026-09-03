<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";

const props = withDefaults(defineProps<{
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  cancelLabel: string;
  danger?: boolean;
  busy?: boolean;
}>(), { danger: false, busy: false });

const emit = defineEmits<{ confirm: []; cancel: [] }>();
function onKeydown(event: KeyboardEvent) {
  if (props.open && event.key === "Escape" && !props.busy) emit("cancel");
}
onMounted(() => window.addEventListener("keydown", onKeydown));
onUnmounted(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <Teleport to="body">
    <Transition name="dialog">
      <div v-if="open" class="modal-layer app-dialog-layer" @click.self="!busy && emit('cancel')">
        <section class="modal-card app-confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirm-dialog-title" aria-describedby="confirm-dialog-message">
          <span class="dialog-symbol" :class="{ danger }" aria-hidden="true">{{ danger ? '!' : '?' }}</span>
          <div>
            <h2 id="confirm-dialog-title">{{ title }}</h2>
            <p id="confirm-dialog-message">{{ message }}</p>
          </div>
          <div class="modal-actions">
            <button type="button" class="button ghost" :disabled="busy" @click="emit('cancel')">{{ cancelLabel }}</button>
            <button type="button" class="button" :class="danger ? 'danger' : 'primary'" :disabled="busy" @click="emit('confirm')">{{ confirmLabel }}</button>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
