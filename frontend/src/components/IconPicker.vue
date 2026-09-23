<script setup lang="ts">
import { computed, ref } from "vue";
import { Loading, Picture } from "@element-plus/icons-vue";
import ProfileIcon from "./ProfileIcon.vue";

const props = withDefaults(defineProps<{ modelValue: string; fallback?: string; team?: boolean; uploadOnly?: boolean; remoteUpload?: boolean; previewUrl?: string; uploading?: boolean }>(), { fallback: "", team: false, uploadOnly: false, remoteUpload: false, previewUrl: "", uploading: false });
const emit = defineEmits<{ "update:modelValue": [value: string]; fileSelected: [file: File]; invalid: [reason: "type" | "size"] }>();
const presets = ["sparkles", "compass", "code", "terminal", "users"];
const custom = computed(() => Boolean(props.previewUrl) || props.modelValue.startsWith("data:image/"));
const fileInput = ref<HTMLInputElement>();

function choose(icon: string) { emit("update:modelValue", icon); }
function openFilePicker() { fileInput.value?.click(); }
function readImage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  if (!["image/png", "image/jpeg", "image/webp", "image/gif"].includes(file.type)) { emit("invalid", "type"); return; }
  if (file.size > 2 * 1024 * 1024) { emit("invalid", "size"); return; }
  if (props.remoteUpload) { emit("fileSelected", file); return; }
  const reader = new FileReader();
  reader.onload = () => { if (typeof reader.result === "string") emit("update:modelValue", reader.result); };
  reader.readAsDataURL(file);
}
</script>

<template>
  <div class="icon-picker">
    <div class="icon-picker-grid" role="radiogroup" aria-label="图标">
      <template v-if="!uploadOnly"><button v-for="icon in presets" :key="icon" type="button" class="icon-picker-option" :class="{ selected: modelValue === icon || (!modelValue && fallback === icon) }" :aria-label="icon" :aria-pressed="modelValue === icon" @click="choose(icon)"><ProfileIcon :icon="icon" :team="team && icon === 'users'" /></button></template>
      <button data-testid="icon-picker-upload" type="button" class="icon-picker-option icon-picker-upload" :class="{ selected: custom }" title="上传图标" aria-label="上传图标" :disabled="uploading" @click="openFilePicker"><Loading v-if="uploading" class="icon-picker-upload-icon is-loading" /><ProfileIcon v-else-if="custom" :icon="previewUrl || modelValue" :team="team" /><Picture v-else class="icon-picker-upload-icon" /></button><input ref="fileInput" data-testid="icon-picker-file" class="icon-picker-file-input" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="readImage">
    </div>
  </div>
</template>
