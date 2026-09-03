<script setup lang="ts">

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
</script>

<template>
  <el-dialog
    :model-value="open"
    class="app-confirm-dialog"
    width="430px"
    append-to-body
    align-center
    role="alertdialog"
    :close-on-click-modal="!busy"
    :close-on-press-escape="!busy"
    :show-close="!busy"
    @close="emit('cancel')"
  >
    <template #header><div class="confirm-heading"><span class="dialog-symbol" :class="{ danger }" aria-hidden="true">{{ danger ? '!' : '?' }}</span><h2 id="confirm-dialog-title">{{ title }}</h2></div></template>
    <p id="confirm-dialog-message">{{ message }}</p>
    <template #footer><el-button :disabled="busy" @click="emit('cancel')">{{ cancelLabel }}</el-button><el-button :type="danger ? 'danger' : 'primary'" :loading="busy" @click="emit('confirm')">{{ confirmLabel }}</el-button></template>
  </el-dialog>
</template>
