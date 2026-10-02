<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Search } from "@lucide/vue";
import ExpertsPage from "./ExpertsPage.vue";
import SkillsConnectorsPage from "./SkillsConnectorsPage.vue";
import KnowledgeBasesPage from "./KnowledgeBasesPage.vue";

type ResourceCenterTab = "experts" | "skills" | "connectors" | "knowledge";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const knowledgeDetailOpen = ref(false);
const activeTab = computed<ResourceCenterTab>(() => {
  if (route.query.tab === "skills") return "skills";
  if (route.query.tab === "connectors") return "connectors";
  if (route.query.tab === "knowledge") return "knowledge";
  return "experts";
});
watch(activeTab, () => { knowledgeDetailOpen.value = false; });
const search = computed({
  get: () => String(route.query.q ?? ""),
  set: (value: string) => updateQuery("q", value.trim() || undefined),
});
function updateQuery(key: string, value?: string) {
  const query = { ...route.query };
  if (value) query[key] = value;
  else delete query[key];
  void router.replace({ query });
}
</script>

<template>
  <section class="resource-center-page">
    <div v-if="activeTab !== 'knowledge' || !knowledgeDetailOpen" class="resource-center-toolbar">
      <el-input v-model="search" clearable :placeholder="t('resources.taskSearch')" :aria-label="t('resources.taskSearch')"><template #prefix><Search :size="17" /></template></el-input>
    </div>
    <div class="resource-center-content">
      <ExpertsPage v-if="activeTab === 'experts'" :key="activeTab" embedded :catalog-query="search" :available-only="false" />
      <SkillsConnectorsPage v-else-if="activeTab === 'skills' || activeTab === 'connectors'" :key="activeTab" :show-tabs="false" :catalog-query="search" :available-only="false" />
      <KnowledgeBasesPage v-else :key="activeTab" embedded :catalog-query="search" :available-only="false" @detail-open="knowledgeDetailOpen = $event" />
    </div>
  </section>
</template>
