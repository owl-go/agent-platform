<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeft } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type ExpertTeam } from "../api/client";
import { authContextKey } from "../auth/session";
import ExpertTeamSettings from "../components/ExpertTeamSettings.vue";

const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const route = useRoute(), router = useRouter();
const { t } = useI18n();
const team = ref<ExpertTeam>(), error = ref("");
const isNew = route.name === "expert-team-new" || route.params.teamId === "new";
onMounted(async () => {
  if (!administrator.value || isNew) return;
  try { team.value = await api.getExpertTeam(String(route.params.teamId)); }
  catch { error.value = t("experts.loadTeamFailed"); }
});
</script>
<template>
  <section class="page-surface editor-page">
    <el-button class="back-link" text :icon="ArrowLeft" @click="router.push('/experts?tab=teams')">{{ t('experts.backTeams') }}</el-button>
    <header class="editor-header"><h1>{{ isNew ? t('experts.createTeam') : t('experts.editTeam') }}</h1></header>
    <el-alert v-if="!administrator" type="info" :closable="false" :title="t('experts.teamAdministratorOnly')" />
    <el-alert v-else-if="error" type="error" :closable="false" :title="error" />
    <ExpertTeamSettings v-else-if="isNew || team" :key="team?.id || 'new-team'" :team="team" @saved="router.push('/experts?tab=teams')" @cancel="router.push('/experts?tab=teams')" />
    <el-skeleton v-else :rows="4" animated />
  </section>
</template>
