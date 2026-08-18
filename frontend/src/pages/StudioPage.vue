<script setup lang="ts">
import { useI18n } from "vue-i18n";
import SurfaceEmptyState from "./SurfaceEmptyState.vue";
const { t } = useI18n();
</script>
<template><SurfaceEmptyState :kicker="t('surfaces.studio.kicker')" :title="t('surfaces.studio.title')" :body="t('surfaces.studio.body')" :empty-title="t('surfaces.studio.emptyTitle')" :empty-body="t('surfaces.studio.emptyBody')" /></template>
