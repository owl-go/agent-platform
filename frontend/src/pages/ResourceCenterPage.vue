<script setup lang="ts">
import { computed } from "vue";
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
const activeTab = computed<ResourceCenterTab>(() => {
  if (route.query.tab === "skills") return "skills";
  if (route.query.tab === "connectors") return "connectors";
  if (route.query.tab === "knowledge") return "knowledge";
  return "experts";
});
const search = computed({
  get: () => String(route.query.q ?? ""),
  set: (value: string) => updateQuery("q", value.trim() || undefined),
});
const availableOnly = computed(() => route.query.status !== "all");
function updateQuery(key: string, value?: string) {
  const query = { ...route.query };
  if (value) query[key] = value;
  else delete query[key];
  void router.replace({ query });
}
function selectStatus(value: string | number | boolean | undefined) {
  updateQuery("status", String(value) === "all" ? "all" : undefined);
}
function selectTab(value: string | number) {
  const tab = String(value) as ResourceCenterTab;
  const query = { ...route.query };
  delete query.scope;
  delete query.tab;
  if (tab !== "experts") query.tab = tab;
  void router.replace({ query });
}
</script>

<template>
  <section class="resource-center-page">
    <header class="resource-center-header">
      <el-tabs class="resource-center-tabs" :model-value="activeTab" :aria-label="t('resources.centerTitle')" @tab-change="selectTab">
        <el-tab-pane :label="t('experts.title')" name="experts" />
        <el-tab-pane :label="t('resources.skills')" name="skills" />
        <el-tab-pane :label="t('resources.connectors')" name="connectors" />
        <el-tab-pane :label="t('knowledgeBases.title')" name="knowledge" />
      </el-tabs>
    </header>
    <div class="resource-center-toolbar">
      <el-input v-model="search" clearable :placeholder="t('resources.taskSearch')" :aria-label="t('resources.taskSearch')"><template #prefix><Search :size="17" /></template></el-input>
      <el-radio-group :model-value="availableOnly ? 'available' : 'all'" @change="selectStatus"><el-radio-button value="available">{{ t('resources.availableOnly') }}</el-radio-button><el-radio-button value="all">{{ t('resources.allStates') }}</el-radio-button></el-radio-group>
    </div>
    <p class="resource-center-filter-hint">{{ availableOnly ? t('resources.availableOnlyHint') : t('resources.allStatesHint') }}</p>
    <div class="resource-center-content">
      <ExpertsPage v-if="activeTab === 'experts'" :key="activeTab" embedded :catalog-query="search" :available-only="availableOnly" />
      <SkillsConnectorsPage v-else-if="activeTab === 'skills' || activeTab === 'connectors'" :key="activeTab" :show-tabs="false" :catalog-query="search" :available-only="availableOnly" />
      <KnowledgeBasesPage v-else :key="activeTab" embedded :catalog-query="search" :available-only="availableOnly" />
    </div>
  </section>
</template>
