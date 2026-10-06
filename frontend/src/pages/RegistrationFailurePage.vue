<script setup lang="ts">
import { useI18n } from "vue-i18n";
const { t } = useI18n();
</script>
<template>
  <main class="auth-screen" aria-labelledby="registration-failure-title">
    <el-card class="auth-card" shadow="never">
      <div class="auth-brand"><span class="logo-mark" aria-hidden="true">AW</span><strong>{{ t('product') }}</strong></div>
      <div class="auth-content" role="alert"><h1 id="registration-failure-title">{{ t('registration.failureTitle') }}</h1><p>{{ t('registration.failureHint') }}</p><a href="/">{{ t('registration.back') }}</a></div>
    </el-card>
  </main>
</template>
