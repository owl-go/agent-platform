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
    <el-button class="back-link" text :icon="ArrowLeft" @click="$router.push('/experts?tab=teams')">{{ t('experts.backTeams') }}</el-button>
    <header class="editor-header"><div><p class="eyebrow">EXPERT TEAM</p><h1>{{ isNew ? t('experts.createTeam') : t('experts.editTeam') }}</h1></div><div class="editor-actions"><el-button v-if="team" type="danger" plain @click="confirmDelete = true">{{ t('common.delete') }}</el-button><el-button type="primary" :loading="saving" :disabled="form.expert_ids.length < 2" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</el-button></div></header>
    <ToastMessage v-if="toast" :kind="toast.kind" :title="toast.kind === 'success' ? t('experts.saveSucceeded') : t('experts.operationFailed')" :message="toast.message" :close-label="t('common.close')" @dismiss="toast = undefined" />
    <form class="editor-form" @submit.prevent="save">
      <section class="editor-section"><div><span>01</span><h2>{{ t('experts.teamInfo') }}</h2><p>{{ t('experts.teamInfoHint') }}</p></div><div class="form-grid"><label>{{ t('experts.teamName') }}<el-input v-model="form.name" maxlength="100" show-word-limit /></label><label class="full">{{ t('experts.capability') }}<el-input v-model="form.capability_introduction" type="textarea" :rows="4" maxlength="2000" show-word-limit /></label><label class="full tag-input">{{ t('experts.expertise') }}<div><el-input v-model="tagDraft" maxlength="20" :placeholder="t('experts.tagPlaceholder')" @keydown.enter.prevent="addTag" /><el-button :icon="Plus" :aria-label="t('experts.addTag')" @click="addTag" /></div></label><div class="editable-tags full"><el-tag v-for="(item, index) in form.expertise_tags" :key="item" closable type="info" @close="form.expertise_tags.splice(index, 1)">{{ item }}</el-tag></div></div></section>
      <section class="editor-section"><div><span>02</span><h2>{{ t('experts.members') }}</h2><p>{{ t('experts.membersHint') }}</p></div><div><div class="member-picker"><select v-model="selectedExpertID"><option value="">{{ t('experts.chooseExpert') }}</option><option v-for="item in candidates" :key="item.id" :value="item.id">{{ item.name }}</option></select><el-button :icon="Plus" :disabled="!selectedExpertID" @click="addMember">{{ t('experts.add') }}</el-button></div><ol class="ordered-members"><li v-for="(member, index) in members" :key="member.id" draggable="true" @dragstart="draggedIndex = index" @dragend="draggedIndex = undefined" @dragover.prevent @drop="dropMember(index)"><GripVertical :size="17" /><span class="member-order">{{ index + 1 }}</span><div><strong>{{ member.name }}</strong><small>{{ member.capability_introduction }}</small></div><el-button text circle :disabled="index === 0" :aria-label="t('experts.moveUp', { name: member.name })" @click="move(index, -1)"><ArrowUp :size="16" /></el-button><el-button text circle :disabled="index === members.length - 1" :aria-label="t('experts.moveDown', { name: member.name })" @click="move(index, 1)"><ArrowDown :size="16" /></el-button><el-button text circle type="danger" :aria-label="t('experts.removeMember', { name: member.name })" @click="form.expert_ids.splice(index, 1)"><X :size="16" /></el-button></li></ol><el-alert type="info" :closable="false" :title="`${t('experts.perRound', { count: members.length })} · ${t('experts.sequential')}`" /></div></section>
    </form>
  </section>
  <ConfirmDialog :open="confirmDelete" :title="t('experts.deleteExpertTitle', { name: team?.name })" :message="t('experts.deleteTeamHint')" :confirm-label="t('experts.confirmDelete')" :cancel-label="t('common.cancel')" danger @cancel="confirmDelete = false" @confirm="remove" />
</template>
