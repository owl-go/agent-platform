<script setup lang="ts">
import { useI18n } from "vue-i18n";
import SurfaceEmptyState from "./SurfaceEmptyState.vue";
const { t } = useI18n();
</script>
<template><SurfaceEmptyState :kicker="t('surfaces.workspace.kicker')" :title="t('surfaces.workspace.title')" :body="t('surfaces.workspace.body')" :empty-title="t('surfaces.workspace.emptyTitle')" :empty-body="t('surfaces.workspace.emptyBody')" /></template>
