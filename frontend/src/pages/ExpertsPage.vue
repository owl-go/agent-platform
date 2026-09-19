<script setup lang="ts">
import { computed, inject, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { Search, Users, UserRound } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import CatalogDetails from "../components/CatalogDetails.vue";
import ProfileIcon from "../components/ProfileIcon.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const experts = ref<Expert[]>([]);
const teams = ref<ExpertTeam[]>([]);
const query = ref("");
const category = ref("");
const error = ref("");
const detailExpert = ref<Expert>();
const detailTeam = ref<ExpertTeam>();
function summon(kind: "expert_id" | "expert_team_id", id: string) { void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), [kind]: id } }); }
function createExpertSession() { void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), create_expert: "true", draft: "帮我创建一个 XXX 专家，擅长 XXXXX。我的经验是：[请补充你的行业背景、相关经验]" } }); }
const activeTab = computed<"experts" | "teams">(() => route.query.tab === "teams" ? "teams" : "experts");
const mineOnly = computed(() => route.query.scope === "mine");
const activeCategories = computed(() => Array.from(new Set((activeTab.value === "experts" ? experts.value : teams.value).map((item) => item.expertise_tags[0]).filter(Boolean))).sort());
const visibleExperts = computed(() => filter(experts.value));
const visibleTeams = computed(() => filter(teams.value));
const expertSections = computed(() => mineOnly.value
  ? [{ key: "mine", title: t("experts.myExperts"), items: visibleExperts.value.filter((item) => !item.platform) }]
  : [{ key: "platform", title: t("experts.platformExperts"), items: visibleExperts.value.filter((item) => item.platform) }]);

onMounted(refresh);
watch(activeTab, () => { query.value = ""; category.value = ""; });

async function refresh() {
  try {
    [experts.value, teams.value] = await Promise.all([api.listExperts(), api.listExpertTeams()]);
  } catch {
    error.value = t("experts.loadFailed");
  }
}

function filter<T extends { name: string; introduction?: string; capability_introduction?: string; expertise_tags: string[] }>(items: T[]): T[] {
  const needle = query.value.trim().toLocaleLowerCase();
  return items.filter((item) => (!needle || `${item.name} ${item.introduction || item.capability_introduction || ""}`.toLocaleLowerCase().includes(needle)) && (!category.value || item.expertise_tags[0] === category.value));
}

function selectTab(tab: string | number) {
  void router.replace({ query: tab === "teams" ? { tab: "teams" } : {} });
}

function toggleMine() {
  const query = { ...route.query };
  if (mineOnly.value) delete query.scope;
  else query.scope = "mine";
  void router.replace({ query });
}
</script>

<template>
  <section class="page-surface expert-catalog">
    <header class="expert-catalog-head">
      <el-tabs class="catalog-tabs" :model-value="activeTab" :aria-label="t('experts.catalog')" @tab-change="selectTab">
        <el-tab-pane :label="t('experts.title')" name="experts" />
        <el-tab-pane :label="t('experts.teams')" name="teams" />
      </el-tabs>
      <div class="catalog-head-actions"><el-button v-if="activeTab === 'experts'" class="my-resource-toggle" :type="mineOnly ? 'primary' : 'default'" @click="toggleMine">{{ t('experts.myExperts') }}</el-button><RouterLink v-if="activeTab === 'experts'" class="el-button el-button--primary" to="/experts/new" @click.prevent="createExpertSession">＋ {{ t('experts.new') }}</RouterLink><RouterLink v-else class="el-button el-button--primary" to="/expert-teams/new">＋ {{ t('experts.createTeam') }}</RouterLink></div>
    </header>

    <div class="catalog-tools">
      <el-input v-model="query" class="catalog-search" clearable :placeholder="activeTab === 'experts' ? t('experts.searchExperts') : t('experts.searchTeams')"><template #prefix><Search :size="17" /></template></el-input>
      <div class="tag-filter" :aria-label="t('experts.categoryFilter')">
        <el-check-tag :checked="!category" @change="category = ''">{{ t('experts.all') }}</el-check-tag>
        <el-check-tag v-for="item in activeCategories" :key="item" :checked="category === item" @change="category = item">{{ item }}</el-check-tag>
      </div>
    </div>

    <ToastMessage v-if="error" kind="error" :title="t('experts.operationFailed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />

    <div v-if="activeTab === 'experts'" class="catalog-groups">
      <section v-for="section in expertSections" :key="section.key" class="catalog-group">
        <h2 class="catalog-group-title">{{ section.title }}</h2>
        <div class="expert-grid catalog-grid">
      <article v-for="expert in section.items" :key="expert.id" class="expert-card-link catalog-activatable" role="button" tabindex="0" :aria-label="expert.name" @click="detailExpert = expert" @keydown.enter.self="detailExpert = expert" @keydown.space.self.prevent="detailExpert = expert">
        <el-card class="expert-card" shadow="hover"><el-button class="catalog-launch" type="primary" :disabled="!expert.available" @click.stop="summon('expert_id', expert.id)">{{ t('composer.summon') }}</el-button>
          <div class="expert-card-layout">
            <ProfileIcon :icon="expert.icon" :background="expert.icon_background" />
            <div class="expert-card-copy">
              <div class="card-title-line"><h2>{{ expert.name }}</h2><el-tag v-if="!expert.complete" type="warning" effect="light" round size="small">{{ t('experts.incomplete') }}</el-tag><el-tag v-else-if="expert.tag_projection_status === 'queued' || expert.tag_projection_status === 'running'" type="info" effect="light" round size="small">{{ t('experts.tagGenerating') }}</el-tag><el-tag v-else-if="expert.tag_projection_status === 'failed'" type="warning" effect="light" round size="small" :title="expert.tag_projection_error">{{ t('experts.tagFailed') }}</el-tag></div>
              <p>{{ expert.introduction }}</p>
              <small class="expert-execution-profile">{{ t('experts.resourceCounts', { skills: expert.skill_ids.length, connectors: expert.mcp_server_ids.length + (expert.cli_connector_definition_ids?.length ?? 0) }) }}</small>
              <div v-if="expert.expertise_tags.length > 1" class="tag-row expert-tags"><el-tag v-for="item in expert.expertise_tags.slice(1, 5)" :key="item" effect="light" round size="small">{{ item }}</el-tag><span v-if="expert.expertise_tags.length > 5" class="expert-tag-more">+{{ expert.expertise_tags.length - 5 }}</span></div>
            </div>
          </div>
        </el-card>
      </article>
      <div v-if="!section.items.length" class="empty-inline extension-empty"><span>◇</span><p>{{ t('experts.noExperts') }}</p></div>
        </div>
      </section>
    </div>

    <div v-else class="expert-grid catalog-grid">
      <article v-for="team in visibleTeams" :key="team.id" class="expert-card-link catalog-activatable" role="button" tabindex="0" :aria-label="team.name" @click="detailTeam = team" @keydown.enter.self="detailTeam = team" @keydown.space.self.prevent="detailTeam = team">
        <el-card class="expert-card expert-team-card" shadow="hover"><el-button class="catalog-launch" type="primary" :disabled="!team.available" @click.stop="summon('expert_team_id', team.id)">{{ t('composer.summon') }}</el-button>
          <div class="expert-card-layout">
            <ProfileIcon :icon="team.icon" :background="team.icon_background" team />
            <div class="expert-card-copy">
              <div class="card-title-line"><h2>{{ team.name }}</h2><el-tag v-if="!team.available" type="warning" effect="light" round size="small">{{ t('experts.teamUnavailable') }}</el-tag></div>
              <p>{{ team.introduction }}</p>
              <ol class="member-preview"><li v-for="member in (team.members.length ? team.members : team.experts.map((expert) => ({ id: expert.id, name: expert.name, expert })))" :key="member.id"><span>{{ member.name }}</span><small>{{ member.expert.introduction }}</small></li></ol>
              <div class="card-footer"><div class="tag-row expert-tags"><el-tag v-for="item in team.expertise_tags.slice(1, 5)" :key="item" effect="light" round size="small">{{ item }}</el-tag><span v-if="team.expertise_tags.length > 5" class="expert-tag-more">+{{ team.expertise_tags.length - 5 }}</span></div><strong>{{ t('experts.perRound', { count: team.members?.length ? team.members.length : team.experts.length }) }}</strong></div>
            </div>
          </div>
        </el-card>
      </article>
      <el-empty v-if="!visibleTeams.length" class="catalog-empty" :description="t('experts.noTeams')" />
    </div>
  </section>
  <CatalogDetails :expert="detailExpert" :team="detailTeam" @close="detailExpert = undefined; detailTeam = undefined" />
</template>
