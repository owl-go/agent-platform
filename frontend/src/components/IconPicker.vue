<script setup lang="ts">
import { computed } from "vue";
import { Picture } from "@element-plus/icons-vue";
import ProfileIcon from "./ProfileIcon.vue";

const props = withDefaults(defineProps<{ modelValue: string; fallback?: string; team?: boolean; uploadOnly?: boolean }>(), { fallback: "", team: false, uploadOnly: false });
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const presets = ["sparkles", "compass", "code", "terminal", "users"];
const custom = computed(() => props.modelValue.startsWith("data:image/"));

function choose(icon: string) { emit("update:modelValue", icon); }
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
    <div class="icon-picker-current"><ProfileIcon v-if="modelValue" :icon="modelValue || fallback" :team="team" /><div v-else class="icon-picker-empty"><Picture /></div><div><strong>{{ custom ? '自定义图片' : modelValue ? '已上传图标' : '未上传图标' }}</strong><small>{{ uploadOnly ? '请上传助手图标' : '选择一个预设，或上传你的图片' }}</small></div></div>
    <div class="icon-picker-grid" role="radiogroup" aria-label="图标">
      <template v-if="!uploadOnly"><button v-for="icon in presets" :key="icon" type="button" class="icon-picker-option" :class="{ selected: modelValue === icon || (!modelValue && fallback === icon) }" :aria-label="icon" :aria-pressed="modelValue === icon" @click="choose(icon)"><ProfileIcon :icon="icon" :team="team && icon === 'users'" /></button></template>
      <label data-testid="icon-picker-upload" class="icon-picker-option icon-picker-upload" :class="{ selected: custom }" title="上传图标" aria-label="上传图标"><Picture class="icon-picker-upload-icon" /><input data-testid="icon-picker-file" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="readImage"></label>
    </div>
  </div>
</template>
