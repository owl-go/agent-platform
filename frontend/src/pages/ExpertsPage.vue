<script setup lang="ts">
import { computed, inject, onMounted, ref, watch } from "vue";
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

function filter<T extends { name: string; introduction?: string; capability_introduction?: string; expertise_tags: string[] }>(items: T[]): T[] {
  const needle = query.value.trim().toLocaleLowerCase();
  return items.filter((item) => (!needle || `${item.name} ${item.introduction || item.capability_introduction || ""}`.toLocaleLowerCase().includes(needle)) && (!tag.value || item.expertise_tags.includes(tag.value)));
}

function selectTab(tab: string | number) {
  void router.replace({ query: tab === "teams" ? { tab: "teams" } : {} });
}
</script>

<template>
  <section class="page-surface expert-catalog">
    <header class="expert-catalog-head">
      <el-tabs class="catalog-tabs" :model-value="activeTab" :aria-label="t('experts.catalog')" @tab-change="selectTab">
        <el-tab-pane :label="t('experts.title')" name="experts" />
        <el-tab-pane :label="t('experts.teams')" name="teams" />
      </el-tabs>
      <RouterLink class="el-button el-button--primary" :to="activeTab === 'experts' ? '/experts/new' : '/expert-teams/new'">＋ {{ activeTab === 'experts' ? t('experts.new') : t('experts.createTeam') }}</RouterLink>
    </header>

    <div class="catalog-tools">
      <el-input v-model="query" class="catalog-search" clearable :placeholder="activeTab === 'experts' ? t('experts.searchExperts') : t('experts.searchTeams')"><template #prefix><Search :size="17" /></template></el-input>
      <div class="tag-filter" :aria-label="t('experts.expertiseFilter')">
        <el-check-tag :checked="!tag" @change="tag = ''">{{ t('experts.all') }}</el-check-tag>
        <el-check-tag v-for="item in activeTags" :key="item" :checked="tag === item" @change="tag = item">{{ item }}</el-check-tag>
      </div>
    </div>

    <ToastMessage v-if="error" kind="error" :title="t('experts.operationFailed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />

    <div v-if="activeTab === 'experts'" class="expert-grid catalog-grid">
      <RouterLink v-for="expert in visibleExperts" :key="expert.id" class="expert-card-link" :to="`/experts/${expert.id}`">
        <el-card class="expert-card" shadow="hover">
          <div class="expert-card-layout">
            <span class="catalog-avatar"><UserRound :size="22" /></span>
            <div class="expert-card-copy">
              <div class="card-title-line"><h2>{{ expert.name }}</h2><el-tag v-if="!expert.complete" type="warning" effect="light" round size="small">{{ t('experts.incomplete') }}</el-tag><el-tag v-else-if="expert.tag_projection_status === 'queued' || expert.tag_projection_status === 'running'" type="info" effect="light" round size="small">{{ t('experts.tagGenerating') }}</el-tag><el-tag v-else-if="expert.tag_projection_status === 'failed'" type="warning" effect="light" round size="small" :title="expert.tag_projection_error">{{ t('experts.tagFailed') }}</el-tag></div>
              <p>{{ expert.introduction }}</p>
              <small class="expert-execution-profile">{{ t('experts.resourceCounts', { skills: expert.skill_ids.length, connectors: expert.mcp_server_ids.length + (expert.cli_connector_definition_ids?.length ?? 0) }) }}</small>
              <div v-if="expert.expertise_tags.length" class="tag-row"><el-tag v-for="item in expert.expertise_tags" :key="item" effect="light" round size="small">{{ item }}</el-tag></div>
            </div>
          </div>
        </el-card>
      </RouterLink>
      <el-empty v-if="!visibleExperts.length" class="catalog-empty" :description="t('experts.noExperts')" />
    </div>

    <div v-else class="expert-grid catalog-grid">
      <RouterLink v-for="team in visibleTeams" :key="team.id" class="expert-card-link" :to="`/expert-teams/${team.id}`">
        <el-card class="expert-card expert-team-card" shadow="hover">
          <div class="expert-card-layout">
            <span class="catalog-avatar team"><Users :size="22" /></span>
            <div class="expert-card-copy">
              <div class="card-title-line"><h2>{{ team.name }}</h2><el-tag v-if="!team.available" type="warning" effect="light" round size="small">{{ t('experts.teamUnavailable') }}</el-tag></div>
              <p>{{ team.introduction }}</p>
              <ol class="member-preview"><li v-for="member in (team.members.length ? team.members : team.experts.map((expert) => ({ id: expert.id, name: expert.name, expert })))" :key="member.id"><span>{{ member.name }}</span><small>{{ member.expert.introduction }}</small></li></ol>
              <div class="card-footer"><div class="tag-row"><el-tag v-for="item in team.expertise_tags" :key="item" effect="light" round size="small">{{ item }}</el-tag></div><strong>{{ t('experts.perRound', { count: team.members?.length ? team.members.length : team.experts.length }) }}</strong></div>
            </div>
          </div>
        </el-card>
      </RouterLink>
      <el-empty v-if="!visibleTeams.length" class="catalog-empty" :description="t('experts.noTeams')" />
    </div>
  </section>
</template>
