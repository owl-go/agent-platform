<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref, watch } from "vue";
import { isNavigationFailure, NavigationFailureType, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { platformApiKey, type Expert, type ExpertTeam, type Skill } from "../api/client";
import { authContextKey } from "../auth/session";
import { renderMarkdown } from "../markdown";
import { saveErrorMessage } from "../api/saveErrors";
import ExpertBindingsEditor from "./ExpertBindingsEditor.vue";
import ExpertTeamSettings from "./ExpertTeamSettings.vue";

const props = defineProps<{ skill?: Skill; expert?: Expert; team?: ExpertTeam; createTeam?: boolean }>();
const emit = defineEmits<{ close: []; editSkill: [skill: Skill]; saved: [] }>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const router = useRouter();
const { t } = useI18n();
const content = ref("");
const loading = ref(false), error = ref("");
const editing = ref(false);
const sendingPrompt = ref("");
let taskSession: { id: string; accepted: boolean } | undefined;
let disposed = false;
const item = computed(() => props.skill ?? props.expert ?? props.team);
const open = computed(() => Boolean(item.value || props.createTeam));
const specialistUnavailable = computed(() => Boolean(props.expert && !props.expert.available || props.team && !props.team.available));
const administrator = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const canEdit = computed(() => {
  if (props.team) return administrator.value && props.team.mutable === true && !props.team.immutable;
  if (props.skill?.immutable || props.expert?.immutable) return false;
  return (!props.skill?.platform && !props.expert?.platform) || administrator.value;
});
let generation = 0;
watch(item, async (value) => {
  const current = ++generation; content.value = ""; error.value = ""; editing.value = false;
  taskSession = undefined; sendingPrompt.value = "";
  if (!value) return;
  loading.value = true;
  try {
    if (props.skill) {
      const document = await api.getSkillDocument(props.skill.id);
      if (current === generation) content.value = document.content;
    }
  } catch { if (current === generation) error.value = t("errors.generic"); }
  finally { if (current === generation) loading.value = false; }
}, { immediate: true });
function launch() {
  if (!item.value) return;
  const field = props.skill ? "skill_id" : props.team ? "expert_team_id" : "expert_id";
  void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), [field]: item.value.id } }); emit("close");
}
async function sendPrompt(prompt: string) {
  const specialist = props.expert ?? props.team;
  if (!specialist || specialistUnavailable.value || sendingPrompt.value) return;
  const current = generation;
  const selectionInput = props.team ? { expert_team_id: specialist.id } : { expert_id: specialist.id };
  sendingPrompt.value = prompt; error.value = "";
  try {
    if (!taskSession) {
      const session = await api.createSession(selectionInput);
      if (disposed || current !== generation) return;
      taskSession = { id: session.id, accepted: false };
    }
    const session = taskSession;
    if (!session.accepted) {
      const selection = await api.getConversationSelection({ session_id: session.id });
      if (disposed || current !== generation) return;
      if (props.team ? selection.expert_team_id !== specialist.id : selection.expert_id !== specialist.id) throw new Error("specialist_not_selected");
      await api.sendSessionMessage(session.id, prompt, [], undefined, { selection_id: selection.id, file_references: [] });
      session.accepted = true;
    }
    if (disposed || current !== generation) return;
    const failure = await router.push({ path: "/sessions", query: { open: session.id } });
    if (isNavigationFailure(failure, NavigationFailureType.aborted | NavigationFailureType.cancelled)) throw new Error("conversation_not_opened");
    emit("close");
  } catch (cause) {
    if (!disposed && current === generation) error.value = saveErrorMessage(cause, t, "experts.taskSendFailed");
  } finally {
    if (current === generation) sendingPrompt.value = "";
  }
}
onBeforeUnmount(() => { disposed = true; });
async function exportPackage() {
  const value = props.expert ?? props.team; if (!value) return;
  try {
    const archive = await api.exportExpertPackage(props.team ? "expert_team" : "expert", value.id);
    const url = URL.createObjectURL(archive); const link = document.createElement("a"); link.href = url; link.download = "expert-package.zip"; link.click(); window.setTimeout(() => URL.revokeObjectURL(url), 0);
  } catch { error.value = t("experts.exportFailed"); }
}

function edit() {
  if (props.skill) emit("editSkill", props.skill);
  else { editing.value = true; return; }
  emit("close");
}
</script>
<template>
  <el-drawer v-if="skill" :model-value="open" class="catalog-details" :title="skill.name" size="min(680px, 100vw)" destroy-on-close @close="emit('close')">
    <el-skeleton v-if="loading" :rows="5" animated />
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <p class="catalog-source">{{ skill.source === 'git' ? skill.git_url : t('composer.localSkill') }} · {{ t('composer.version', { version: skill.version }) }}</p>
    <div class="markdown-body skill-document" v-html="renderMarkdown(content)"></div>
    <template #footer><el-button v-if="canEdit" @click="edit">{{ t('common.edit') }}</el-button><el-button type="primary" @click="launch">{{ t('composer.useSkill') }}</el-button></template>
  </el-drawer>
  <el-dialog v-else :model-value="open" class="catalog-details expert-details-dialog" :title="createTeam ? t('experts.createTeam') : item?.name" width="min(560px, calc(100vw - 32px))" :close-on-click-modal="!sendingPrompt" :close-on-press-escape="!sendingPrompt" :show-close="!sendingPrompt" destroy-on-close @close="emit('close')">
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <ExpertTeamSettings v-if="createTeam || editing && team" :key="team?.id || 'new-team'" :team="team" @cancel="createTeam ? emit('close') : editing = false" @saved="emit('saved'); emit('close')" />
    <ExpertBindingsEditor v-else-if="editing" :key="item?.id" :expert="expert" @cancel="editing = false" @saved="emit('saved'); emit('close')" />
    <template v-else>
      <p class="expert-capability-description">{{ expert?.introduction || team?.introduction }}</p>
      <section v-if="(expert?.starter_prompts || team?.starter_prompts)?.length" class="expert-common-tasks" :aria-busy="Boolean(sendingPrompt)"><h3>{{ t('experts.commonTasks') }}</h3><ul><li v-for="prompt in (expert?.starter_prompts || team?.starter_prompts)" :key="prompt"><button type="button" class="expert-task-prompt" :disabled="specialistUnavailable || Boolean(sendingPrompt)" :aria-label="t('experts.sendTask', { name: item?.name, prompt })" @click="sendPrompt(prompt)">{{ prompt }}</button></li></ul><p v-if="sendingPrompt" role="status" class="muted">{{ t('experts.taskSending') }}</p></section>
    </template>
    <template v-if="!editing && !createTeam" #footer><el-button :disabled="Boolean(sendingPrompt)" @click="exportPackage">{{ t('experts.exportPackage') }}</el-button><el-button v-if="canEdit" :disabled="Boolean(sendingPrompt)" @click="edit">{{ t('common.edit') }}</el-button><el-button type="primary" :disabled="specialistUnavailable || Boolean(sendingPrompt)" @click="launch">{{ t('composer.summon') }}</el-button></template>
  </el-dialog>
</template>
