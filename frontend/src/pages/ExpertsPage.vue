<script setup lang="ts">
import { computed, inject, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Search, ChevronDown, Plus } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import CatalogDetails from "../components/CatalogDetails.vue";
import ProfileIcon from "../components/ProfileIcon.vue";
import ResourceTrustMeta from "../components/ResourceTrustMeta.vue";
import CatalogLoading from "../components/CatalogLoading.vue";
import { authContextKey } from "../auth/session";

const props = withDefaults(defineProps<{ embedded?: boolean; catalogQuery?: string; availableOnly?: boolean }>(), { embedded: false, catalogQuery: "", availableOnly: true });

const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const experts = ref<Expert[]>([]);
const teams = ref<ExpertTeam[]>([]);
const query = ref("");
const error = ref("");
const loading = ref(true);
const packageFile = ref<HTMLInputElement>();
const importing = ref(false);
const detailExpert = ref<Expert>();
const detailTeam = ref<ExpertTeam>();
function summon(kind: "expert_id" | "expert_team_id", id: string) { void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), [kind]: id } }); }
function createTeamSession() { if (!administrator.value) return; void router.push({path:"/sessions",query:{new:crypto.randomUUID(),create_expert:"true",draft:"帮我创建一个专家团，请明确领队和各成员的独立指引。"}}); }
function createExpertSession() { void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), create_expert: "true", draft: "帮我创建一个 XXX 专家，擅长 XXXXX。我的经验是：[请补充你的行业背景、相关经验]" } }); }
const activeTab = computed<"experts" | "teams">(() => route.query.tab === "teams" ? "teams" : "experts");
const mineOnly = computed(() => route.query.scope === "mine");
const visibleExperts = computed(() => filter(experts.value));
const visibleTeams = computed(() => filter(teams.value));
const expertSections = computed(() => (mineOnly.value
  ? [{ key: "mine", title: t("experts.myExperts"), items: visibleExperts.value.filter((item) => !item.platform) }]
  : [
      { key: "platform", title: t("experts.platformExperts"), items: visibleExperts.value.filter((item) => item.platform) },
      { key: "mine", title: t("experts.myExperts"), items: visibleExperts.value.filter((item) => !item.platform) },
    ]).filter((section) => section.items.length));

onMounted(refresh);
watch(activeTab, () => { query.value = ""; });

async function refresh() {
  loading.value = true;
  error.value = "";
  try {
    [experts.value, teams.value] = await Promise.all([api.listExperts(), api.listExpertTeams()]);
  } catch {
    error.value = t("experts.loadFailed");
  } finally {
    loading.value = false;
  }
}

function addResource(command: string) {
  if (command === "import") packageFile.value?.click();
  else if (activeTab.value === "teams") createTeamSession();
  else createExpertSession();
}

async function importPackage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file || importing.value) return;
  if (!file.name.toLowerCase().endsWith(".zip") || file.size > 100 * 1024 * 1024) {
    error.value = t("experts.invalidPackageFile"); input.value = ""; return;
  }
  importing.value = true; error.value = "";
  try {
    const result = await api.importExpertPackage(file);
    await refresh(); detailExpert.value = result.expert; detailTeam.value = result.expert_team;
  } catch { error.value = t("experts.importFailed"); }
  finally { importing.value = false; input.value = ""; }
}

function filter<T extends { name: string; introduction?: string; capability_introduction?: string; available: boolean }>(items: T[]): T[] {
  const needle = (props.embedded ? props.catalogQuery : query.value).trim().toLocaleLowerCase();
  return items.filter((item) => (!props.availableOnly || item.available) && (!needle || `${item.name} ${item.introduction || item.capability_introduction || ""}`.toLocaleLowerCase().includes(needle)));
}

function selectTab(tab: string | number) {
  const query = { ...route.query };
  delete query.tab;
  if (tab === "teams") query.tab = "teams";
  void router.replace({ query });
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
      <div class="catalog-head-actions">
        <input v-if="activeTab === 'experts' || administrator" ref="packageFile" class="package-import-file" type="file" accept=".zip,application/zip" hidden @change="importPackage" />
        <el-button v-if="activeTab === 'experts'" class="my-resource-toggle" :type="mineOnly ? 'primary' : 'default'" @click="toggleMine">{{ t('experts.myExperts') }}</el-button>
        <el-dropdown v-if="activeTab === 'experts' || administrator" trigger="click" @command="addResource">
          <el-button type="primary" :loading="importing" :disabled="importing" :icon="Plus">{{ t(activeTab === 'experts' ? 'experts.addExpert' : 'experts.addTeam') }}<ChevronDown :size="16" /></el-button>
          <template #dropdown><el-dropdown-menu>
            <el-dropdown-item command="create">{{ t(activeTab === 'experts' ? 'experts.new' : 'experts.createTeam') }}</el-dropdown-item>
            <el-dropdown-item command="import">{{ t('experts.importPackage') }}</el-dropdown-item>
          </el-dropdown-menu></template>
        </el-dropdown>
      </div>
    </header>

    <div class="catalog-tools">
      <el-input v-if="!props.embedded" v-model="query" class="catalog-search" clearable :placeholder="activeTab === 'experts' ? t('experts.searchExperts') : t('experts.searchTeams')"><template #prefix><Search :size="17" /></template></el-input>
    </div>

    <ToastMessage v-if="error" kind="error" :title="t('experts.operationFailed')" :message="error" :close-label="t('common.close')" @dismiss="error = ''" />

    <CatalogLoading v-if="loading" />
    <div v-else-if="activeTab === 'experts'" class="catalog-groups">
      <section v-for="section in expertSections" :key="section.key" class="catalog-group">
        <h2 class="catalog-group-title">{{ section.title }}</h2>
        <div class="expert-grid catalog-grid">
      <article v-for="expert in section.items" :key="expert.id" class="expert-card-link catalog-activatable" role="button" tabindex="0" :aria-label="expert.name" @click="detailExpert = expert" @keydown.enter.self="detailExpert = expert" @keydown.space.self.prevent="detailExpert = expert">
        <el-card class="expert-card" shadow="hover"><el-button class="catalog-launch" type="primary" :disabled="!expert.available" @click.stop="summon('expert_id', expert.id)">{{ t('composer.summon') }}</el-button>
          <div class="expert-card-layout">
            <div class="expert-card-copy">
              <div class="expert-card-heading"><ProfileIcon :icon="expert.icon" :background="expert.icon_background" /><div class="expert-card-heading-copy"><div class="card-title-line"><h2>{{ expert.name }}</h2><el-tag v-if="!expert.complete" type="warning" effect="light" size="small">{{ t('experts.incomplete') }}</el-tag></div></div></div>
              <p>{{ expert.introduction }}</p>
              <ResourceTrustMeta :source="expert.platform ? t('resources.platformPublished') : t('resources.userPublished')" :permission="expert.platform ? t('resources.allAuthenticated') : t('resources.ownerOnly')" :status="expert.available ? t('resources.available') : expert.complete ? t('resources.unavailable') : t('experts.incomplete')" :status-tone="expert.available ? 'success' : 'warning'" :detail="expert.availability_reason || ''" />
              <small class="expert-execution-profile">{{ t('experts.resourceCounts', { skills: expert.skill_ids.length + (expert.bundled_skills?.length ?? 0), connectors: expert.mcp_server_ids.length + (expert.cli_connector_definition_ids?.length ?? 0) + (expert.connector_dependencies?.length ?? 0) }) }}</small>
            </div>
          </div>
        </el-card>
      </article>
      <div v-if="!section.items.length" class="empty-inline extension-empty"><span>◇</span><p>{{ t('experts.noExperts') }}</p></div>
        </div>
      </section>
      <el-empty v-if="!expertSections.length" class="catalog-empty" :description="t('experts.noExperts')" />
    </div>

    <div v-else class="expert-grid catalog-grid">
      <article v-for="team in visibleTeams" :key="team.id" class="expert-card-link catalog-activatable" role="button" tabindex="0" :aria-label="team.name" @click="detailTeam = team" @keydown.enter.self="detailTeam = team" @keydown.space.self.prevent="detailTeam = team">
        <el-card class="expert-card expert-team-card" shadow="hover"><el-button class="catalog-launch" type="primary" :disabled="!team.available" @click.stop="summon('expert_team_id', team.id)">{{ t('composer.summon') }}</el-button>
          <div class="expert-card-layout">
            <div class="expert-card-copy">
              <div class="expert-card-heading"><ProfileIcon :icon="team.icon" :background="team.icon_background" team /><div class="expert-card-heading-copy"><div class="card-title-line"><h2>{{ team.name }}</h2><el-tag v-if="!team.available" type="warning" effect="light" size="small">{{ t('experts.teamUnavailable') }}</el-tag></div></div></div>
              <p>{{ team.introduction }}</p>
              <ResourceTrustMeta :source="t('resources.platformPublished')" :permission="t('resources.allAuthenticated')" :status="team.available ? t('resources.available') : t('resources.unavailable')" :status-tone="team.available ? 'success' : 'warning'" />
              <ol class="member-preview"><li v-for="member in (team.members.length ? team.members : team.experts.map((expert) => ({ id: expert.id, name: expert.name, expert })))" :key="member.id"><span>{{ member.name }}</span><small>{{ member.expert.introduction }}</small></li></ol>
              <div class="card-footer"><strong>{{ t('experts.perRound', { count: team.members?.length ? team.members.length : team.experts.length }) }}</strong></div>
            </div>
          </div>
        </el-card>
      </article>
      <el-empty v-if="!visibleTeams.length" class="catalog-empty" :description="t('experts.noTeams')" />
    </div>
  </section>
  <CatalogDetails @saved="refresh" :expert="detailExpert" :team="detailTeam" @close="detailExpert = undefined; detailTeam = undefined" />
</template>
