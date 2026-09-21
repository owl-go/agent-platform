<script setup lang="ts">
import { RouterLink, RouterView, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";

const route = useRoute();
const { t } = useI18n();
const tabs = [
  { name: "smart-assistants", path: "/ai-apps/assistants", key: "assistants" },
  { name: "digital-humans", path: "/ai-apps/digital-humans", key: "digitalHumans" },
  { name: "image-creation", path: "/ai-apps/image-creation", key: "imageCreation" },
];
</script>

<template>
  <section class="ai-applications-page">
    <header class="ai-applications-header">
      <div>
        <p class="eyebrow">{{ t('nav.ai-applications') }}</p>
        <h1>{{ t('aiApplications.title') }}</h1>
        <p>{{ t('aiApplications.subtitle') }}</p>
      </div>
    </header>
    <nav class="ai-applications-tabs" aria-label="AI Applications">
      <RouterLink v-for="tab in tabs" :key="tab.name" :to="tab.path" :class="{ active: route.name === tab.name }">
        <strong>{{ t(`aiApplications.tabs.${tab.key}.label`) }}</strong><small>{{ t(`aiApplications.tabs.${tab.key}.english`) }}</small>
      </RouterLink>
    </nav>
    <RouterView />
  </section>
</template>
