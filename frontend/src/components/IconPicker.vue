<script setup lang="ts">
import { computed, ref } from "vue";
import { Picture } from "@element-plus/icons-vue";
import ProfileIcon from "./ProfileIcon.vue";

const props = withDefaults(defineProps<{ modelValue: string; fallback?: string; team?: boolean; uploadOnly?: boolean }>(), { fallback: "", team: false, uploadOnly: false });
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const presets = ["sparkles", "compass", "code", "terminal", "users"];
const custom = computed(() => props.modelValue.startsWith("data:image/"));
const fileInput = ref<HTMLInputElement>();

function choose(icon: string) { emit("update:modelValue", icon); }
function openFilePicker() { fileInput.value?.click(); }
function readImage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file || !["image/png", "image/jpeg", "image/webp", "image/gif"].includes(file.type) || file.size > 384 * 1024) return;
  const reader = new FileReader();
  reader.onload = () => { if (typeof reader.result === "string") emit("update:modelValue", reader.result); };
  reader.readAsDataURL(file);
}
</script>

<template>
  <div class="icon-picker">
    <div class="icon-picker-grid" role="radiogroup" aria-label="图标">
      <template v-if="!uploadOnly"><button v-for="icon in presets" :key="icon" type="button" class="icon-picker-option" :class="{ selected: modelValue === icon || (!modelValue && fallback === icon) }" :aria-label="icon" :aria-pressed="modelValue === icon" @click="choose(icon)"><ProfileIcon :icon="icon" :team="team && icon === 'users'" /></button></template>
      <button data-testid="icon-picker-upload" type="button" class="icon-picker-option icon-picker-upload" :class="{ selected: custom }" title="上传图标" aria-label="上传图标" @click="openFilePicker"><ProfileIcon v-if="custom" :icon="modelValue" :team="team" /><Picture v-else class="icon-picker-upload-icon" /></button><input ref="fileInput" data-testid="icon-picker-file" class="icon-picker-file-input" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="readImage">
    </div>
  </div>
</template>
