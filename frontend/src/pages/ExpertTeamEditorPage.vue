<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowDown, ArrowLeft, ArrowUp, GripVertical, Plus, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam, type ExpertTeamInput } from "../api/client";
import ToastMessage from "../components/ToastMessage.vue";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const experts = ref<Expert[]>([]);
const team = ref<ExpertTeam>();
const tagDraft = ref("");
const selectedExpertID = ref("");
const draggedIndex = ref<number>();
const saving = ref(false);
const confirmDelete = ref(false);
const toast = ref<{ kind: "success" | "error"; message: string }>();
const form = ref<ExpertTeamInput>({ name: "", capability_introduction: "", expertise_tags: [], expert_ids: [] });
const isNew = route.params.teamId === "new";
const members = computed(() => form.value.expert_ids.map((id) => experts.value.find((item) => item.id === id)).filter((item): item is Expert => Boolean(item)));
const candidates = computed(() => experts.value.filter((item) => item.available && !form.value.expert_ids.includes(item.id)));

onMounted(async () => {
  try {
    experts.value = await api.listExperts();
    if (!isNew) {
      team.value = await api.getExpertTeam(String(route.params.teamId));
      form.value = { name: team.value.name, capability_introduction: team.value.capability_introduction, expertise_tags: [...team.value.expertise_tags], expert_ids: team.value.experts.map((item) => item.id) };
    }
  } catch { toast.value = { kind: "error", message: t("experts.loadTeamFailed") }; }
});

function addMember() { if (selectedExpertID.value && form.value.expert_ids.length < 10) form.value.expert_ids.push(selectedExpertID.value); selectedExpertID.value = ""; }
function move(index: number, offset: number) { const target = index + offset; if (target < 0 || target >= form.value.expert_ids.length) return; const [id] = form.value.expert_ids.splice(index, 1); form.value.expert_ids.splice(target, 0, id!); }
function dropMember(target: number) { const source = draggedIndex.value; draggedIndex.value = undefined; if (source === undefined || source === target) return; const [id] = form.value.expert_ids.splice(source, 1); form.value.expert_ids.splice(target, 0, id!); }
function addTag() { const value = tagDraft.value.trim(); if (!value || form.value.expertise_tags.some((item) => item.toLowerCase() === value.toLowerCase()) || form.value.expertise_tags.length >= 10) return; form.value.expertise_tags.push(value.slice(0, 20)); tagDraft.value = ""; }
async function save() { saving.value = true; try { if (team.value) await api.updateExpertTeam(team.value.id, form.value, team.value.version); else await api.createExpertTeam(form.value); toast.value = { kind: "success", message: t("experts.teamSaved") }; window.setTimeout(() => void router.push("/experts?tab=teams"), 350); } catch { toast.value = { kind: "error", message: t("experts.teamSaveFailed") }; } finally { saving.value = false; } }
async function remove() { if (!team.value) return; try { await api.deleteExpertTeam(team.value.id); await router.push("/experts?tab=teams"); } catch { toast.value = { kind: "error", message: t("experts.deleteTeamFailed") }; } }
</script>

<template>
  <section class="page-surface editor-page">
    <button class="back-link" type="button" @click="$router.push('/experts?tab=teams')"><ArrowLeft :size="16" /> {{ t('experts.backTeams') }}</button>
    <header class="editor-header"><div><p class="eyebrow">EXPERT TEAM</p><h1>{{ isNew ? t('experts.createTeam') : t('experts.editTeam') }}</h1></div><div class="editor-actions"><button v-if="team" class="button danger-ghost" @click="confirmDelete = true">{{ t('common.delete') }}</button><button class="button primary" :disabled="saving || form.expert_ids.length < 2" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</button></div></header>
    <ToastMessage v-if="toast" :kind="toast.kind" :title="toast.kind === 'success' ? t('experts.saveSucceeded') : t('experts.operationFailed')" :message="toast.message" :close-label="t('common.close')" @dismiss="toast = undefined" />
    <form class="editor-form" @submit.prevent="save">
      <section class="editor-section"><div><span>01</span><h2>{{ t('experts.teamInfo') }}</h2><p>{{ t('experts.teamInfoHint') }}</p></div><div class="form-grid"><label>{{ t('experts.teamName') }}<input v-model="form.name" maxlength="100" required></label><label class="full">{{ t('experts.capability') }}<textarea v-model="form.capability_introduction" rows="4" maxlength="2000" required></textarea></label><label class="full tag-input">{{ t('experts.expertise') }}<div><input v-model="tagDraft" maxlength="20" :placeholder="t('experts.tagPlaceholder')" @keydown.enter.prevent="addTag"><button type="button" :aria-label="t('experts.addTag')" @click="addTag"><Plus :size="17" /></button></div></label><div class="editable-tags full"><span v-for="(item, index) in form.expertise_tags" :key="item">{{ item }}<button type="button" :aria-label="`${t('common.delete')} ${item}`" @click="form.expertise_tags.splice(index, 1)"><X :size="13" /></button></span></div></div></section>
      <section class="editor-section"><div><span>02</span><h2>{{ t('experts.members') }}</h2><p>{{ t('experts.membersHint') }}</p></div><div><div class="member-picker"><select v-model="selectedExpertID"><option value="">{{ t('experts.chooseExpert') }}</option><option v-for="item in candidates" :key="item.id" :value="item.id">{{ item.name }}</option></select><button class="button ghost" type="button" :disabled="!selectedExpertID" @click="addMember"><Plus :size="16" /> {{ t('experts.add') }}</button></div><ol class="ordered-members"><li v-for="(member, index) in members" :key="member.id" draggable="true" @dragstart="draggedIndex = index" @dragend="draggedIndex = undefined" @dragover.prevent @drop="dropMember(index)"><GripVertical :size="17" /><span class="member-order">{{ index + 1 }}</span><div><strong>{{ member.name }}</strong><small>{{ member.capability_introduction }}</small></div><button type="button" :disabled="index === 0" :aria-label="t('experts.moveUp', { name: member.name })" @click="move(index, -1)"><ArrowUp :size="16" /></button><button type="button" :disabled="index === members.length - 1" :aria-label="t('experts.moveDown', { name: member.name })" @click="move(index, 1)"><ArrowDown :size="16" /></button><button type="button" :aria-label="t('experts.removeMember', { name: member.name })" @click="form.expert_ids.splice(index, 1)"><X :size="16" /></button></li></ol><p class="team-cost">{{ t('experts.perRound', { count: members.length }) }} · {{ t('experts.sequential') }}</p></div></section>
    </form>
  </section>
  <div v-if="confirmDelete" class="modal-layer" @click.self="confirmDelete = false"><section class="modal-card"><p class="eyebrow">DELETE EXPERT TEAM</p><h2>{{ t('experts.deleteExpertTitle', { name: team?.name }) }}</h2><p>{{ t('experts.deleteTeamHint') }}</p><div class="modal-actions"><button class="button ghost" @click="confirmDelete = false">{{ t('common.cancel') }}</button><button class="button danger" @click="remove">{{ t('experts.confirmDelete') }}</button></div></section></div>
</template>
