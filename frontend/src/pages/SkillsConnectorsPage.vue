<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import ExtensionManager from "../components/ExtensionManager.vue";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const initialTab = computed(() => route.query.tab === "connectors" ? "mcp" : "skills");
</script>

<template>
  <section class="page-surface resource-catalog">
    <header class="page-header"><div><h1>{{ t("resources.title") }}</h1><p>{{ t("resources.subtitle") }}</p></div></header>
    <ExtensionManager :initial-tab="initialTab" @tab-change="router.replace({ query: $event === 'skills' ? {} : { tab: 'connectors' } })" />
  </section>
</template>
