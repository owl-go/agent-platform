<script setup lang="ts">
import { computed } from "vue";
import ProfileIcon from "./ProfileIcon.vue";
import feishuIcon from "../assets/feishu.png";
import dingtalkIcon from "../assets/dingtalk.png";
import openboostIcon from "../assets/openboost.svg";
import modaoIcon from "../assets/modao.png";
import notionIcon from "../assets/notion.svg";

const props = withDefaults(defineProps<{ icon?: string; size?: number }>(), { size: 28 });
const imageSource = computed(() => {
  if (props.icon === "feishu") return feishuIcon;
  if (props.icon === "dingtalk") return dingtalkIcon;
  if (props.icon === "openboost") return openboostIcon;
  if (props.icon === "modao") return modaoIcon;
  if (props.icon === "notion") return notionIcon;
  return props.icon?.startsWith("data:image/") ? props.icon : undefined;
});
</script>

<template>
  <span class="connector-icon" :style="{ width: `${size}px`, height: `${size}px` }" aria-hidden="true">
    <img v-if="imageSource" :src="imageSource" alt="" />
    <ProfileIcon v-else :icon="props.icon || 'terminal'" />
  </span>
</template>

<style scoped>
.connector-icon { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; overflow: hidden; border-radius: 9px; }
.connector-icon img { width: 100%; height: 100%; object-fit: cover; }
.connector-icon :deep(.profile-icon) { width: 100%; height: 100%; border-radius: inherit; }
</style>
