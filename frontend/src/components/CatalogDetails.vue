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
const emit = defineEmits<{ close: []; editSkill: [skill: Skill]; copied: [] }>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const router = useRouter();
const { t } = useI18n();
const content = ref("");
const loading = ref(false), error = ref("");
const copying=ref(false),copyOpen=ref(false),copyName=ref("");
async function copyPackage() {
  const value = props.expert ?? props.team;
  if (!value || !copyName.value.trim() || props.team && !administrator.value) return;
  copying.value = true;
  try {
    const archive = await api.exportExpertPackage(props.team ? "expert_team" : "expert", value.id);
    await api.importExpertPackage(new File([archive], "expert-package.zip", { type: "application/zip" }), { copy_name: copyName.value.trim() });
    copyOpen.value = false;
    emit("copied");
    emit("close");
  } catch {
    error.value = t("experts.importFailed");
  } finally {
    copying.value = false;
  }
}
const resourceMeta = ref<Record<string, { name: string; icon?: string }>>({});
const item = computed(() => props.skill ?? props.expert ?? props.team);
const open = computed(() => Boolean(item.value));
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const canEdit = computed(() => {
  if (props.team) return administrator.value && props.team.mutable === true && !props.team.immutable;
  if (props.skill?.immutable || props.expert?.immutable) return false;
  return (!props.skill?.platform && !props.expert?.platform) || administrator.value;
});
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
async function exportPackage() {
  const value = props.expert ?? props.team; if (!value) return;
  try {
    const archive = await api.exportExpertPackage(props.team ? "expert_team" : "expert", value.id);
    const url = URL.createObjectURL(archive); const link = document.createElement("a"); link.href = url; link.download = "expert-package.zip"; link.click(); window.setTimeout(() => URL.revokeObjectURL(url), 0);
  } catch { error.value = t("experts.exportFailed"); }
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
      <section v-if="(expert?.starter_prompts || team?.starter_prompts)?.length"><h3>开场提示</h3><p v-for="prompt in (expert?.starter_prompts || team?.starter_prompts)" :key="prompt">{{ prompt }}</p></section>
      <section v-if="team"><h3>{{ t('experts.coreCapability') }}</h3><div class="markdown-body" v-html="renderMarkdown(team.core_capability)"></div><p class="muted">{{ t('experts.perRound', { count: team.members.length }) }}</p></section>
      <section v-for="(member, index) in members" :key="member.expert.id + ':' + index" class="catalog-member-detail">
        <h3 v-if="team">{{ index + 1 }}. {{ member.name }} · {{ member.expert.name }}</h3>
        <div v-if="member.labels.length" class="tag-row"><el-tag v-for="label in member.labels" :key="label" size="small">{{ label }}</el-tag></div>
        <section v-if="member.expert.guidance"><h3>{{ t('experts.guidance') }}</h3><div class="markdown-body" v-html="renderMarkdown(member.expert.guidance)"></div></section><template v-else><template v-for="field in fields" :key="field"><section v-if="member.expert[field]"><h3>{{ t(fieldLabels[field]) }}</h3><div class="markdown-body" v-html="renderMarkdown(member.expert[field])"></div></section></template></template>
        <h3>{{ t('composer.skills') }}</h3><div class="tag-row"><el-tag v-for="skill in member.expert.bundled_skills" :key="skill.id">{{ skill.name }}</el-tag><el-tag v-for="id in member.expert.skill_ids" :key="id">{{ resourceMeta[id]?.name || t('composer.resourceUnavailable') }}</el-tag><span v-if="!member.expert.skill_ids.length && !member.expert.bundled_skills?.length" class="muted">{{ t('common.empty') }}</span></div>
        <h3>{{ t('composer.connectors') }}</h3><div class="tag-row connector-detail-tags"><el-tag v-for="dependency in member.expert.connector_dependencies" :key="dependency.source">{{ dependency.source }} · {{ dependency.version }}</el-tag><el-tag v-for="id in [...member.expert.mcp_server_ids, ...(member.expert.cli_connector_definition_ids ?? [])]" :key="id"><ConnectorIcon :icon="resourceMeta[id]?.icon" :size="18" />{{ resourceMeta[id]?.name || t('composer.resourceUnavailable') }}</el-tag><span v-if="!member.expert.mcp_server_ids.length && !member.expert.cli_connector_definition_ids?.length && !member.expert.connector_dependencies?.length" class="muted">{{ t('common.empty') }}</span></div>
      </section>
    </template>
    <template #footer><el-button v-if="expert || team && administrator" @click="copyOpen=true">另存副本</el-button><el-button v-if="expert || team" @click="exportPackage">{{ t('experts.exportPackage') }}</el-button><el-button v-if="canEdit" @click="edit">{{ t('common.edit') }}</el-button><el-button type="primary" :disabled="Boolean(expert && !expert.available || team && !team.available)" @click="launch">{{ skill ? t('composer.useSkill') : t('composer.summon') }}</el-button></template>
  </el-drawer>
 <el-dialog v-model="copyOpen" title="另存副本" width="min(420px, 96vw)"><label>副本名称<el-input v-model="copyName" maxlength="100" /></label><template #footer><el-button @click="copyOpen=false">取消</el-button><el-button type="primary" :loading="copying" :disabled="!copyName.trim()" @click="copyPackage">保存副本</el-button></template></el-dialog>
</template>
