<script setup lang="ts">
import { RouterView } from "vue-router";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
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
    <RouterView />
  </section>
</template>
