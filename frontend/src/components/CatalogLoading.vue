<script setup lang="ts">
import { useI18n } from "vue-i18n";

withDefaults(defineProps<{ compact?: boolean }>(), { compact: false });
const { t } = useI18n();
</script>

<template>
  <div class="catalog-loading" :class="{ compact }" role="status" :aria-label="t('common.loading')" aria-busy="true">
    <el-skeleton v-for="item in 4" :key="item" animated class="catalog-loading-card" aria-hidden="true">
      <template #template>
        <div class="catalog-loading-heading">
          <el-skeleton-item variant="image" class="catalog-loading-icon" />
          <el-skeleton-item variant="text" class="catalog-loading-title" />
        </div>
        <el-skeleton-item variant="p" />
        <el-skeleton-item v-if="!compact" variant="p" />
        <el-skeleton-item v-if="!compact" variant="text" class="catalog-loading-meta" />
      </template>
    </el-skeleton>
  </div>
</template>
