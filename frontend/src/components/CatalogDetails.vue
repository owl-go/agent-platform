<script setup lang="ts">
import { computed, inject, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam, type Skill } from "../api/client";
import { authContextKey } from "../auth/session";
import { renderMarkdown } from "../markdown";
import ProfileIcon from "./ProfileIcon.vue";
import ConnectorIcon from "./ConnectorIcon.vue";

const props = defineProps<{ skill?: Skill; expert?: Expert; team?: ExpertTeam }>();
const emit = defineEmits<{ close: []; editSkill: [skill: Skill] }>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const router = useRouter();
const { t } = useI18n();
const content = ref("");
const loading = ref(false), error = ref("");
const resourceMeta = ref<Record<string, { name: string; icon?: string }>>({});
const item = computed(() => props.skill ?? props.expert ?? props.team);
const open = computed(() => Boolean(item.value));
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const canEdit = computed(() => !props.skill?.platform && !props.expert?.platform || administrator.value || Boolean(props.team));
const members = computed(() => props.expert ? [{ name: "", expert: props.expert, labels: [] as string[] }] : props.team?.members ?? []);
const fields = ["core_capability", "operating_procedure", "output_standard", "cautions"] as const;
const fieldLabels = { core_capability: "experts.coreCapability", operating_procedure: "experts.operatingProcedure", output_standard: "experts.outputStandard", cautions: "experts.cautions" };
let generation = 0;
watch(item, async (value) => {
  const current = ++generation; content.value = ""; error.value = "";
  if (!value) return;
  loading.value = true;
  try {
    if (props.skill) {
      const document = await api.getSkillDocument(props.skill.id);
      if (current === generation) content.value = document.content;
    } else {
      const [skills, mcp, cli] = await Promise.all([api.listSkills(), api.listMCPServers(), api.listCLIConnectorDefinitions()]);
      if (current === generation) resourceMeta.value = Object.fromEntries([...skills, ...mcp, ...cli].map((entry) => [entry.id, { name: entry.name, icon: "icon" in entry ? entry.icon : undefined }]));
    }
  } catch { if (current === generation) error.value = t("errors.generic"); }
  finally { if (current === generation) loading.value = false; }
}, { immediate: true });
function launch() {
  if (!item.value) return;
  const field = props.skill ? "skill_id" : props.team ? "expert_team_id" : "expert_id";
  void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), [field]: item.value.id } }); emit("close");
}
function edit() {
  if (props.skill) emit("editSkill", props.skill);
  else if (props.expert) void router.push(`/experts/${props.expert.id}`);
  else if (props.team) void router.push(`/expert-teams/${props.team.id}`);
  emit("close");
}
</script>
<template>
  <el-drawer :model-value="open" class="catalog-details" :title="item?.name" size="min(680px, 100vw)" destroy-on-close @close="emit('close')">
    <el-skeleton v-if="loading" :rows="5" animated />
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <template v-if="skill"><p class="catalog-source">{{ skill.source === 'git' ? skill.git_url : t('composer.localSkill') }} · {{ t('composer.version', { version: skill.version }) }}</p><div class="markdown-body skill-document" v-html="renderMarkdown(content)"></div></template>
    <template v-else>
      <div class="catalog-detail-intro"><ProfileIcon :icon="expert?.icon || team?.icon" :background="expert?.icon_background || team?.icon_background" :team="Boolean(team)" /><p>{{ expert?.introduction || team?.introduction }}</p></div>
      <section v-if="team"><h3>{{ t('experts.coreCapability') }}</h3><div class="markdown-body" v-html="renderMarkdown(team.core_capability)"></div><p class="muted">{{ t('experts.perRound', { count: team.members.length }) }}</p></section>
      <section v-for="(member, index) in members" :key="member.expert.id + ':' + index" class="catalog-member-detail">
        <h3 v-if="team">{{ index + 1 }}. {{ member.name }} · {{ member.expert.name }}</h3>
        <div v-if="member.labels.length" class="tag-row"><el-tag v-for="label in member.labels" :key="label" size="small">{{ label }}</el-tag></div>
        <template v-for="field in fields" :key="field"><section v-if="member.expert[field]"><h3>{{ t(fieldLabels[field]) }}</h3><div class="markdown-body" v-html="renderMarkdown(member.expert[field])"></div></section></template>
        <h3>{{ t('composer.skills') }}</h3><div class="tag-row"><el-tag v-for="id in member.expert.skill_ids" :key="id">{{ resourceMeta[id]?.name || t('composer.resourceUnavailable') }}</el-tag><span v-if="!member.expert.skill_ids.length" class="muted">{{ t('common.empty') }}</span></div>
        <h3>{{ t('composer.connectors') }}</h3><div class="tag-row connector-detail-tags"><el-tag v-for="id in [...member.expert.mcp_server_ids, ...(member.expert.cli_connector_definition_ids ?? [])]" :key="id"><ConnectorIcon :icon="resourceMeta[id]?.icon" :size="18" />{{ resourceMeta[id]?.name || t('composer.resourceUnavailable') }}</el-tag><span v-if="!member.expert.mcp_server_ids.length && !member.expert.cli_connector_definition_ids?.length" class="muted">{{ t('common.empty') }}</span></div>
      </section>
    </template>
    <template #footer><el-button v-if="canEdit" @click="edit">{{ t('common.edit') }}</el-button><el-button type="primary" :disabled="Boolean(expert && !expert.available || team && !team.available)" @click="launch">{{ skill ? t('composer.useSkill') : t('composer.summon') }}</el-button></template>
  </el-drawer>
</template>
