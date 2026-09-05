<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowDown, ArrowLeft, ArrowUp, GripVertical, Plus, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam, type ExpertTeamInput } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const experts = ref<Expert[]>([]);
const team = ref<ExpertTeam>();
const selectedExpertID = ref("");
const draggedIndex = ref<number>();
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertTeamInput>({ name: "", icon: "users", icon_background: "sage", introduction: "", core_capability: "", members: [] });
const isNew = route.params.teamId === "new";
const members = computed(() => form.value.members.map((member) => ({ ...member, expert: experts.value.find((item) => item.id === member.expert_id) })));
const candidates = computed(() => experts.value.filter((item) => item.available));

onMounted(async () => {
  try {
    experts.value = await api.listExperts();
    if (!isNew) {
      team.value = await api.getExpertTeam(String(route.params.teamId));
      form.value = { name: team.value.name, icon: team.value.icon || "users", icon_background: team.value.icon_background || "sage", introduction: team.value.introduction || team.value.capability_introduction || "", core_capability: team.value.core_capability || "", members: team.value.members?.length ? team.value.members.map(({ id, name, expert_id, labels }) => ({ id, name, expert_id, labels: [...labels] })) : team.value.experts.map((item, index) => ({ id: `legacy-${index}-${item.id}`, name: item.name, expert_id: item.id, labels: [] })) };
    }
  } catch { toast.value = { kind: "error", message: t("experts.loadTeamFailed") }; }
});

function addMember() { const expert = experts.value.find((item) => item.id === selectedExpertID.value); if (expert && form.value.members.length < 10) form.value.members.push({ id: crypto.randomUUID(), name: expert.name, expert_id: expert.id, labels: [] }); selectedExpertID.value = ""; }
function move(index: number, offset: number) { const target = index + offset; if (target < 0 || target >= form.value.members.length) return; const [member] = form.value.members.splice(index, 1); form.value.members.splice(target, 0, member!); }
function dropMember(target: number) { const source = draggedIndex.value; draggedIndex.value = undefined; if (source === undefined || source === target) return; const [member] = form.value.members.splice(source, 1); form.value.members.splice(target, 0, member!); }
async function save() { saving.value = true; try { if (team.value) await api.updateExpertTeam(team.value.id, form.value, team.value.version); else await api.createExpertTeam(form.value); toast.value = { kind: "success", message: t("experts.teamSaved") }; window.setTimeout(() => void router.push("/experts?tab=teams"), 350); } catch { toast.value = { kind: "error", message: t("experts.teamSaveFailed") }; } finally { saving.value = false; } }
async function remove() { if (!team.value) return; try { await api.deleteExpertTeam(team.value.id); await router.push("/experts?tab=teams"); } catch { toast.value = { kind: "error", message: t("experts.deleteTeamFailed") }; } }
</script>

<template>
  <section class="page-surface editor-page">
    <el-button class="back-link" text :icon="ArrowLeft" @click="$router.push('/experts?tab=teams')">{{ t('experts.backTeams') }}</el-button>
    <header class="editor-header"><div><h1>{{ isNew ? t('experts.createTeam') : t('experts.editTeam') }}</h1></div><div class="editor-actions"><el-button v-if="team" type="danger" plain @click="confirmDelete = true">{{ t('common.delete') }}</el-button><el-button type="primary" :loading="saving" :disabled="form.members.length < 2" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</el-button></div></header>
    <ToastMessage v-if="toast" :kind="toast.kind" :title="toast.kind === 'success' ? t('experts.saveSucceeded') : t('experts.operationFailed')" :message="toast.message" :close-label="t('common.close')" @dismiss="toast = undefined" />
    <form class="editor-form" @submit.prevent="save">
      <section class="editor-section"><div><h2>{{ t('experts.teamInfo') }}</h2><p>{{ t('experts.teamInfoHint') }}</p></div><div class="form-grid"><label>{{ t('experts.teamName') }}<el-input v-model="form.name" maxlength="100" show-word-limit /></label><label>{{ t('experts.icon') }}<el-select v-model="form.icon"><el-option value="users" label="◎ Team" /><el-option value="sparkles" label="✦ Sparkles" /><el-option value="compass" label="⌖ Compass" /></el-select></label><label>{{ t('experts.iconBackground') }}<el-select v-model="form.icon_background"><el-option value="sage" :label="t('experts.sage')" /><el-option value="sand" :label="t('experts.sand')" /><el-option value="sky" :label="t('experts.sky')" /><el-option value="coral" :label="t('experts.coral')" /></el-select></label><label class="full">{{ t('experts.introduction') }}<el-input v-model="form.introduction" type="textarea" :rows="3" maxlength="2000" show-word-limit /></label><label class="full">{{ t('experts.coreCapability') }}<el-input v-model="form.core_capability" type="textarea" :rows="4" maxlength="20000" show-word-limit /></label></div></section>
      <section class="editor-section"><div><h2>{{ t('experts.members') }}</h2><p>{{ t('experts.membersHint') }}</p></div><div><div class="member-picker"><select v-model="selectedExpertID"><option value="">{{ t('experts.chooseExpert') }}</option><option v-for="item in candidates" :key="item.id" :value="item.id">{{ item.name }}</option></select><el-button :icon="Plus" :disabled="!selectedExpertID" @click="addMember">{{ t('experts.add') }}</el-button></div><ol class="ordered-members"><li v-for="(member, index) in members" :key="member.id" draggable="true" @dragstart="draggedIndex = index" @dragend="draggedIndex = undefined" @dragover.prevent @drop="dropMember(index)"><GripVertical :size="17" /><span class="member-order">{{ index + 1 }}</span><div class="member-fields"><el-input v-model="form.members[index]!.name" :aria-label="t('experts.memberName')" maxlength="100" /><small>{{ member.expert?.name }}</small><el-select v-model="form.members[index]!.labels" multiple allow-create filterable default-first-option :multiple-limit="5" :placeholder="t('experts.memberLabels')" /></div><el-button text circle :disabled="index === 0" :aria-label="t('experts.moveUp', { name: member.name })" @click="move(index, -1)"><ArrowUp :size="16" /></el-button><el-button text circle :disabled="index === members.length - 1" :aria-label="t('experts.moveDown', { name: member.name })" @click="move(index, 1)"><ArrowDown :size="16" /></el-button><el-button text circle type="danger" :aria-label="t('experts.removeMember', { name: member.name })" @click="form.members.splice(index, 1)"><X :size="16" /></el-button></li></ol><el-alert type="info" :closable="false" :title="`${t('experts.perRound', { count: members.length })} · ${t('experts.sequential')}`" /></div></section>
    </form>
  </section>
  <ConfirmDialog :open="confirmDelete" :title="t('experts.deleteExpertTitle', { name: team?.name })" :message="t('experts.deleteTeamHint')" :confirm-label="t('experts.confirmDelete')" :cancel-label="t('common.cancel')" danger @cancel="confirmDelete = false" @confirm="remove" />
</template>
