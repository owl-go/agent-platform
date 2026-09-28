<script setup lang="ts">
import { computed } from "vue";
import { RouterView, useRoute } from "vue-router";

const route = useRoute();
const isAssistantConversation = computed(
  () => route.name === "smart-assistant-conversation",
);
</script>

<template>
  <section
    class="ai-applications-page"
    :class="{ 'ai-applications-page--conversation': isAssistantConversation }"
  >
    <RouterView />
  </section>
</template>
