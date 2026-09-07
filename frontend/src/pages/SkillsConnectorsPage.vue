<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import ExtensionManager from "../components/ExtensionManager.vue";
import ToastMessage from "../components/ToastMessage.vue";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const initialTab = computed(() => route.query.tab === "connectors" ? "mcp" : "skills");
const error = ref(false);
</script>

<template>
  <section class="page-surface resource-catalog">
    <header class="page-header"><div><h1>{{ t("resources.title") }}</h1><p>{{ t("resources.subtitle") }}</p></div></header>
    <ExtensionManager :initial-tab="initialTab" @tab-change="router.replace({ query: $event === 'skills' ? {} : { tab: 'connectors' } })" @error="error = true" />
    <ToastMessage v-if="error" kind="error" :title="t('experts.operationFailed')" :message="t('resources.operationFailed')" :close-label="t('common.close')" @dismiss="error = false" />
  </section>
</template>
