<script setup lang="ts">
import { useI18n } from "vue-i18n";
import SurfaceEmptyState from "./SurfaceEmptyState.vue";
const { t } = useI18n();
</script>
<template><SurfaceEmptyState :kicker="t('surfaces.operations.kicker')" :title="t('surfaces.operations.title')" :body="t('surfaces.operations.body')" :empty-title="t('surfaces.operations.emptyTitle')" :empty-body="t('surfaces.operations.emptyBody')" /></template>
