<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import ExtensionManager from "../components/ExtensionManager.vue";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const initialTab = computed(() => route.query.tab === "connectors" ? "mcp" : "skills");
const mineOnly = computed(() => route.query.scope === "mine");
const mineLabel = computed(() => initialTab.value === "mcp" ? t("resources.myConnectors") : t("resources.mySkills"));

function toggleMine() {
  const query = { ...route.query };
  if (mineOnly.value) delete query.scope;
  else query.scope = "mine";
  void router.replace({ query });
}
</script>

<template>
  <section class="page-surface resource-catalog">
    <header class="page-header"><div><h1>{{ t("resources.title") }}</h1><p>{{ t("resources.subtitle") }}</p></div><el-button class="my-resource-toggle" :type="mineOnly ? 'primary' : 'default'" @click="toggleMine">{{ mineLabel }}</el-button></header>
    <ExtensionManager :initial-tab="initialTab" :mine-only="mineOnly" @tab-change="router.replace({ query: $event === 'skills' ? {} : { tab: 'connectors' } })" />
  </section>
</template>
