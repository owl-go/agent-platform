<script setup lang="ts">
import { useI18n } from "vue-i18n";
import RuntimeCatalogPanel from "./RuntimeCatalogPanel.vue";
import ModelCatalogPanel from "./ModelCatalogPanel.vue";
const { t } = useI18n();
</script>
<template><section class="surface"><header class="surface-heading reveal"><div><p class="kicker">{{ t('surfaces.studio.kicker') }}</p><h1>{{ t('surfaces.studio.title') }}</h1><p>{{ t('surfaces.studio.body') }}</p></div></header><RuntimeCatalogPanel /><ModelCatalogPanel /></section></template>
