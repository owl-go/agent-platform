<script setup lang="ts">
import { computed, inject, nextTick, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { Search, Users, UserRound } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const experts = ref<Expert[]>([]);
const teams = ref<ExpertTeam[]>([]);
const query = ref("");
const tag = ref("");
const error = ref("");
const activeTab = computed<"experts" | "teams">(() => route.query.tab === "teams" ? "teams" : "experts");
const activeTags = computed(() => Array.from(new Set((activeTab.value === "experts" ? experts.value : teams.value).flatMap((item) => item.expertise_tags))).sort());
const visibleExperts = computed(() => filter(experts.value));
const visibleTeams = computed(() => filter(teams.value));

onMounted(refresh);
watch(activeTab, () => { query.value = ""; tag.value = ""; });

async function refresh() {
  try {
    [experts.value, teams.value] = await Promise.all([api.listExperts(), api.listExpertTeams()]);
  } catch {
    error.value = t("experts.loadFailed");
  }
}

function filter<T extends { name: string; capability_introduction: string; expertise_tags: string[] }>(items: T[]): T[] {
  const needle = query.value.trim().toLocaleLowerCase();
  return items.filter((item) => (!needle || `${item.name} ${item.capability_introduction}`.toLocaleLowerCase().includes(needle)) && (!tag.value || item.expertise_tags.includes(tag.value)));
}

function selectTab(tab: "experts" | "teams") {
  void router.replace({ query: tab === "teams" ? { tab: "teams" } : {} });
}
async function moveTab(tab: "experts" | "teams") {
  selectTab(tab);
  await nextTick();
  document.getElementById(`${tab}-tab`)?.focus();
}
</script>

<template>
  <section class="page-surface expert-catalog">
    <header class="expert-catalog-head">
      <div class="catalog-tabs" role="tablist" :aria-label="t('experts.catalog')">
        <button id="experts-tab" :class="{ active: activeTab === 'experts' }" role="tab" :tabindex="activeTab === 'experts' ? 0 : -1" :aria-selected="activeTab === 'experts'" aria-controls="experts-panel" @click="selectTab('experts')" @keydown.right.prevent="moveTab('teams')" @keydown.end.prevent="moveTab('teams')">{{ t('experts.title') }}</button>
        <button id="teams-tab" :class="{ active: activeTab === 'teams' }" role="tab" :tabindex="activeTab === 'teams' ? 0 : -1" :aria-selected="activeTab === 'teams'" aria-controls="teams-panel" @click="selectTab('teams')" @keydown.left.prevent="moveTab('experts')" @keydown.home.prevent="moveTab('experts')">{{ t('experts.teams') }}</button>
      </div>
      <RouterLink class="button primary" :to="activeTab === 'experts' ? '/experts/new' : '/expert-teams/new'">＋ {{ activeTab === 'experts' ? t('experts.new') : t('experts.createTeam') }}</RouterLink>
    </header>

    <div class="catalog-tools">
      <label class="catalog-search"><Search :size="17" /><input v-model="query" :placeholder="activeTab === 'experts' ? t('experts.searchExperts') : t('experts.searchTeams')"></label>
      <div class="tag-filter" :aria-label="t('experts.expertiseFilter')">
        <button :class="{ active: !tag }" @click="tag = ''">{{ t('experts.all') }}</button>
        <button v-for="item in activeTags" :key="item" :class="{ active: tag === item }" @click="tag = item">{{ item }}</button>
      </div>
    </div>

    <ToastMessage v-if="error" kind="error" :title="t('experts.operationFailed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />

    <div v-if="activeTab === 'experts'" id="experts-panel" class="expert-grid catalog-grid" role="tabpanel" aria-labelledby="experts-tab">
      <RouterLink v-for="expert in visibleExperts" :key="expert.id" class="expert-card" :to="`/experts/${expert.id}`">
        <span class="catalog-avatar"><UserRound :size="22" /></span>
        <div>
          <div class="card-title-line"><h2>{{ expert.name }}</h2><span v-if="!expert.available" class="status-pill warning">{{ t('experts.incomplete') }}</span></div>
          <p>{{ expert.capability_introduction }}</p>
          <div class="tag-row"><span v-for="item in expert.expertise_tags" :key="item">{{ item }}</span></div>
        </div>
      </RouterLink>
      <div v-if="!visibleExperts.length" class="catalog-empty">{{ t('experts.noExperts') }}</div>
    </div>

    <div v-else id="teams-panel" class="expert-grid catalog-grid" role="tabpanel" aria-labelledby="teams-tab">
      <RouterLink v-for="team in visibleTeams" :key="team.id" class="expert-card expert-team-card" :to="`/expert-teams/${team.id}`">
        <span class="catalog-avatar team"><Users :size="22" /></span>
        <div>
          <div class="card-title-line"><h2>{{ team.name }}</h2><span v-if="!team.available" class="status-pill warning">{{ t('experts.teamUnavailable') }}</span></div>
          <p>{{ team.capability_introduction }}</p>
          <ol class="member-preview"><li v-for="member in team.experts" :key="member.id">{{ member.name }}</li></ol>
          <div class="card-footer"><div class="tag-row"><span v-for="item in team.expertise_tags" :key="item">{{ item }}</span></div><strong>{{ t('experts.perRound', { count: team.experts.length }) }}</strong></div>
        </div>
      </RouterLink>
      <div v-if="!visibleTeams.length" class="catalog-empty">{{ t('experts.noTeams') }}</div>
    </div>
  </section>
</template>
