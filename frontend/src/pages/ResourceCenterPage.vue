<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import ExpertsPage from "./ExpertsPage.vue";
import SkillsConnectorsPage from "./SkillsConnectorsPage.vue";

type ResourceCenterTab = "experts" | "skills" | "connectors";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const activeTab = computed<ResourceCenterTab>(() => {
  if (route.query.tab === "skills") return "skills";
  if (route.query.tab === "connectors") return "connectors";
  return "experts";
});
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
      </el-tabs>
    </header>
    <div class="resource-center-content">
      <ExpertsPage v-if="activeTab === 'experts'" :key="activeTab" />
      <SkillsConnectorsPage v-else :key="activeTab" :show-tabs="false" />
    </div>
  </section>
</template>
