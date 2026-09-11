<script setup lang="ts">
import { computed } from "vue";
import { Picture, Upload } from "@element-plus/icons-vue";
import ProfileIcon from "./ProfileIcon.vue";

const props = withDefaults(defineProps<{ modelValue: string; fallback: string; team?: boolean }>(), { team: false });
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
    <div class="icon-picker-current"><ProfileIcon :icon="modelValue || fallback" :team="team" /><div><strong>{{ custom ? '自定义图片' : '默认图标' }}</strong><small>选择一个预设，或上传你的图片</small></div></div>
    <div class="icon-picker-grid" role="radiogroup" aria-label="图标">
      <button v-for="icon in presets" :key="icon" type="button" class="icon-picker-option" :class="{ selected: modelValue === icon || (!modelValue && fallback === icon) }" :aria-label="icon" :aria-pressed="modelValue === icon" @click="choose(icon)"><ProfileIcon :icon="icon" :team="team && icon === 'users'" /></button>
      <label data-testid="icon-picker-upload" class="icon-picker-option icon-picker-upload" :class="{ selected: custom }" title="上传图标"><Picture :size="22" /><input data-testid="icon-picker-file" type="file" accept="image/png,image/jpeg,image/webp,image/gif" @change="readImage"><span>上传图片</span></label>
    </div>
    <small class="icon-picker-help"><Upload :size="13" /> 支持 PNG、JPEG、WebP 或 GIF，大小不超过 384KB。</small>
  </div>
</template>
