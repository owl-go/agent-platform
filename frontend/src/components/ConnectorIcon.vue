<script setup lang="ts">
import ProfileIcon from "./ProfileIcon.vue";

const props = withDefaults(defineProps<{ icon?: string; size?: number }>(), { size: 28 });
const isImage = (value?: string) => Boolean(value?.startsWith("data:image/"));
</script>

<template>
  <span class="connector-icon" :style="{ width: `${size}px`, height: `${size}px` }" aria-hidden="true">
    <img v-if="isImage(props.icon)" :src="props.icon" alt="" />
    <ProfileIcon v-else :icon="props.icon || 'terminal'" />
  </span>
</template>

<style scoped>
.connector-icon { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; overflow: hidden; border-radius: 9px; }
.connector-icon img { width: 100%; height: 100%; object-fit: cover; }
.connector-icon :deep(.profile-icon) { width: 100%; height: 100%; border-radius: inherit; }
</style>
