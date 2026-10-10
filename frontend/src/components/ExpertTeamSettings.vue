<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertInput, type ExpertTeam, type ExpertTeamInput } from "../api/client";
import { authContextKey } from "../auth/session";

const props = defineProps<{ team?: ExpertTeam }>();
const emit = defineEmits<{ saved: [team: ExpertTeam]; cancel: [] }>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const { t } = useI18n();
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const canMaintain = computed(() => administrator.value && (!props.team || props.team.mutable === true && !props.team.immutable));
function copyDefinition(expert: Expert): ExpertInput {
  return { name: expert.name, icon: expert.icon, icon_background: expert.icon_background,
    introduction: expert.introduction, guidance: expert.guidance,
    starter_prompts: [...(expert.starter_prompts ?? [])],
    connector_dependencies: expert.connector_dependencies?.map(item => ({ ...item })),
    skill_ids: [...expert.skill_ids], mcp_server_ids: [...expert.mcp_server_ids],
    cli_connector_definition_ids: [...expert.cli_connector_definition_ids] };
}
const form = ref<ExpertTeamInput>({
  name: props.team?.name ?? "", introduction: props.team?.introduction ?? "",
  icon: props.team?.icon ?? "users", icon_background: props.team?.icon_background ?? "sage",
  core_capability: props.team?.core_capability ?? "", starter_prompts: [...(props.team?.starter_prompts ?? [])],
  lead_member_id: props.team?.lead_member_id ?? "",
  members: (props.team?.members ?? []).map(member => ({ id: member.id, name: member.name,
    expert_id: "", labels: [...member.labels], definition: copyDefinition(member.expert) })),
});
const experts = ref<Expert[]>([]), loading = ref(false), ready = ref(false), saving = ref(false), error = ref("");
let disposed = false;
onBeforeUnmount(() => { disposed = true; });
const selectedMembers = computed(() => form.value.members.map(member => `member:${member.id}`));
const candidates = computed(() => experts.value.filter(expert => expert.available && !form.value.members.some(member => member.name.trim().toLocaleLowerCase() === expert.name.trim().toLocaleLowerCase())));
function updateMembers(values: string[]) {
  if (saving.value || !canMaintain.value || !ready.value) return;
  const previous = new Map(form.value.members.map(member => [`member:${member.id}`, member]));
  const next: ExpertTeamInput["members"] = [];
  const names = new Set<string>();
  for (const value of [...new Set(values)].slice(0, 10)) {
    const existing = previous.get(value);
    const expert = value.startsWith("expert:") ? experts.value.find(item => item.id === value.slice(7) && item.available) : undefined;
    const member = existing ?? (expert ? { id: crypto.randomUUID(), name: expert.name, expert_id: expert.id, labels: [], definition: copyDefinition(expert) } : undefined);
    if (!member || names.has(member.name.trim().toLocaleLowerCase())) continue;
    names.add(member.name.trim().toLocaleLowerCase()); next.push(member);
  }
  form.value.members = next;
  if (!next.some(member => member.id === form.value.lead_member_id)) form.value.lead_member_id = "";
}
async function load() {
  if (!canMaintain.value || loading.value || disposed) return;
  loading.value = true; error.value = "";
  try {
    const items = await api.listExperts();
    if (!disposed) { experts.value = items; ready.value = true; }
  } catch { if (!disposed) error.value = t("experts.loadTeamExpertsFailed"); }
  finally { if (!disposed) loading.value = false; }
}
onMounted(load);
function valid() {
  const bytes = (value: string) => new TextEncoder().encode(value.trim()).length;
  return bytes(form.value.name) >= 1 && bytes(form.value.name) <= 100 && bytes(form.value.introduction) >= 1 && bytes(form.value.introduction) <= 2000
    && form.value.members.length >= 2 && form.value.members.length <= 10
    && form.value.members.some(member => member.id === form.value.lead_member_id);
}
async function save() {
  if (!canMaintain.value || !ready.value || saving.value || disposed) return;
  if (!valid()) { error.value = t("experts.teamSettingsInvalid"); return; }
  saving.value = true; error.value = "";
  const input = JSON.parse(JSON.stringify(form.value)) as ExpertTeamInput;
  input.name = input.name.trim(); input.introduction = input.introduction.trim();
  // The single description also satisfies the existing creation contract.
  if (!props.team) input.core_capability = input.introduction;
  try {
    const saved = props.team ? await api.updateExpertTeam(props.team.id, input, props.team.version) : await api.createExpertTeam(input);
    if (!disposed) emit("saved", saved);
  } catch { if (!disposed) error.value = t("experts.teamSettingsSaveFailed"); }
  finally { if (!disposed) saving.value = false; }
}
</script>

<template>
  <el-alert v-if="!canMaintain" type="info" :closable="false" :title="t(administrator ? 'experts.readOnly' : 'experts.teamAdministratorOnly')" />
  <form v-else class="expert-team-settings" @submit.prevent="save">
    <el-alert v-if="error" type="error" :closable="false" :title="error" />
    <el-skeleton v-if="loading" :rows="4" animated />
    <el-button v-if="!ready && !loading" @click="load">{{ t('common.retry') }}</el-button>
    <fieldset v-if="ready" :disabled="saving">
      <label>{{ t('experts.teamName') }}<el-input v-model="form.name" :disabled="saving" maxlength="100" :aria-label="t('experts.teamName')" /></label>
      <label>{{ t('experts.teamDescription') }}<el-input v-model="form.introduction" :disabled="saving" type="textarea" :rows="3" maxlength="2000" :aria-label="t('experts.teamDescription')" /></label>
      <label>{{ t('experts.members') }}
        <el-select class="team-members-select" :model-value="selectedMembers" :disabled="saving" multiple filterable collapse-tags collapse-tags-tooltip :max-collapse-tags="3" :multiple-limit="10" :placeholder="t('experts.chooseExpert')" :aria-label="t('experts.members')" @update:model-value="updateMembers">
          <el-option v-for="member in form.members" :key="`member:${member.id}`" :value="`member:${member.id}`" :label="member.name" />
          <el-option v-for="expert in candidates" :key="`expert:${expert.id}`" :value="`expert:${expert.id}`" :label="expert.name" />
        </el-select>
      </label>
      <label>{{ t('experts.teamLead') }}
        <el-select v-model="form.lead_member_id" class="team-lead-select" :disabled="saving || !form.members.length" :placeholder="t('experts.chooseLead')" :aria-label="t('experts.teamLead')">
          <el-option v-for="member in form.members" :key="member.id" :value="member.id" :label="member.name" />
        </el-select>
      </label>
    </fieldset>
    <div class="expert-bindings-actions"><el-button :disabled="saving" @click="emit('cancel')">{{ t('common.cancel') }}</el-button><el-button native-type="submit" type="primary" :loading="saving" :disabled="!ready || saving">{{ t('common.save') }}</el-button></div>
  </form>
</template>
