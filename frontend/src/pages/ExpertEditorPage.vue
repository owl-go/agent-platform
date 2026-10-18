<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeft } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert } from "../api/client";
import ExpertBindingsEditor from "../components/ExpertBindingsEditor.vue";

const api = inject(platformApiKey)!;
const route = useRoute(), router = useRouter();
const { t } = useI18n();
const expert = ref<Expert>(), error = ref("");
onMounted(async () => {
  if (route.name === "expert-new" || route.params.expertId === "new") {
    await router.replace({ path: "/sessions", query: { new: crypto.randomUUID(), create_expert: "true", draft: "帮我创建一个专家，请根据我的需求确定能力描述、常见任务和执行指引。" } });
    return;
  }
  try { expert.value = await api.getExpert(String(route.params.expertId)); }
  catch { error.value = t("experts.loadExpertFailed"); }
});
</script>
<template>
  <section class="page-surface editor-page">
    <el-button class="back-link" text :icon="ArrowLeft" @click="router.push('/experts')">{{ t('experts.backCatalog') }}</el-button>
    <header class="editor-header"><h1>{{ expert?.name || t('experts.editExpert') }}</h1></header>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <ExpertBindingsEditor v-if="expert" :key="expert.id" :expert="expert" @saved="router.push('/experts')" @cancel="router.push('/experts')" />
    <el-skeleton v-else-if="!error" :rows="3" animated />
  </section>
</template>
