<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import ExtensionManager from "../components/ExtensionManager.vue";
import ConnectorPackagePanel from "../components/ConnectorPackagePanel.vue";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const props = withDefaults(defineProps<{ showTabs?: boolean }>(), { showTabs: true });
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
    <div v-if="!props.showTabs" class="resource-child-actions"><el-button class="my-resource-toggle" :type="mineOnly ? 'primary' : 'default'" @click="toggleMine">{{ mineLabel }}</el-button></div>
    <ExtensionManager :initial-tab="initialTab" :mine-only="mineOnly" :show-tabs="props.showTabs" @tab-change="router.replace({ query: $event === 'skills' ? {} : { tab: 'connectors' } })">
      <template #tab-actions><el-button v-if="props.showTabs" class="my-resource-toggle" :type="mineOnly ? 'primary' : 'default'" @click="toggleMine">{{ mineLabel }}</el-button></template>
    </ExtensionManager>
    <ConnectorPackagePanel v-if="initialTab === 'mcp'" />
  </section>
</template>
