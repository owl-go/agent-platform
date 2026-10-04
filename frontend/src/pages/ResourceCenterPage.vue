<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import ExpertsPage from "./ExpertsPage.vue";
import SkillsConnectorsPage from "./SkillsConnectorsPage.vue";
import KnowledgeBasesPage from "./KnowledgeBasesPage.vue";

type ResourceCenterTab = "experts" | "skills" | "connectors" | "knowledge";

const route = useRoute();
const activeTab = computed<ResourceCenterTab>(() => {
  if (route.query.tab === "skills") return "skills";
  if (route.query.tab === "connectors") return "connectors";
  if (route.query.tab === "knowledge") return "knowledge";
  return "experts";
});
</script>

<template>
  <section class="resource-center-page">
    <div class="resource-center-content">
      <ExpertsPage v-if="activeTab === 'experts'" :key="activeTab" embedded :available-only="false" />
      <SkillsConnectorsPage v-else-if="activeTab === 'skills' || activeTab === 'connectors'" :key="activeTab" :show-tabs="false" :available-only="false" />
      <KnowledgeBasesPage v-else :key="activeTab" embedded :available-only="false" />
    </div>
  </section>
</template>
