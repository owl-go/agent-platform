<script setup lang="ts">
import { computed, inject, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  ApiError, platformApiKey, type Agent, type AgentDraft, type ConfiguredModel,
  type AgentRelease, type DraftInput, type ReleaseApproval, type RepositoryBinding, type RuntimeImage,
} from "../api/client";
import { authContextKey } from "../auth/session";

const injectedApi = inject(platformApiKey);
const injectedAuth = inject(authContextKey);
if (!injectedApi || !injectedAuth) throw new Error("Agent Draft dependencies are required");
const api = injectedApi;
const auth = injectedAuth;
const route = useRoute();
const { t } = useI18n();
const agents = ref<Agent[]>([]);
const drafts = ref<AgentDraft[]>([]);
const approvals = ref<Record<string, ReleaseApproval>>({});
const releases = ref<AgentRelease[]>([]);
const bindings = ref<RepositoryBinding[]>([]);
const runtimes = ref<RuntimeImage[]>([]);
const models = ref<ConfiguredModel[]>([]);
const selectedAgentID = ref("");
const loading = ref(true);
const saving = ref(false);
const validatingDraftID = ref("");
const error = ref<ApiError>();
const notice = ref("");
const agentModal = ref(false);
const draftModal = ref(false);
const approvalModal = ref<AgentDraft>();
const decisionModal = ref<{ draft: AgentDraft; approval: ReleaseApproval; approved: boolean }>();
const blockModal = ref<AgentRelease>();
const editingDraft = ref<AgentDraft>();
const agentForm = reactive({ name: "", description: "" });
const approvalForm = reactive({ riskReason: "" });
const decisionForm = reactive({ reason: "" });
const blockForm = reactive({ reason: "" });
const draftForm = reactive({
  instructions: "", bindingID: "", runtimeID: "", modelID: "",
  maxInputTokens: 100000, maxOutputTokens: 20000, maxCostAmount: "50.00",
  timeoutSeconds: 1800, cpus: 2, memoryBytes: 4294967296, pids: 256, tempBytes: 10737418240,
  nativeSubagents: false, releaseRisk: "low",
});
const intents = new Map<string, { fingerprint: string; key: string }>();
let refreshSequence = 0;
const teamID = computed(() => typeof route.query.team === "string" ? route.query.team : "");
const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const canBuild = computed(() => (currentUser.value?.role_grants ?? []).some((grant) =>
  (grant.role === "agent_builder" && (!grant.team_id || grant.team_id === teamID.value)) || (grant.role === "platform_administrator" && !grant.team_id),
));
const canBlock = computed(() => (currentUser.value?.role_grants ?? []).some((grant) => grant.role === "platform_administrator" && !grant.team_id));
const selectedAgent = computed(() => agents.value.find((agent) => agent.id === selectedAgentID.value));
const releasedDraftIDs = computed(() => new Set(releases.value.map((release) => release.source_draft_id)));

watch(teamID, () => {
  selectedAgentID.value = "";
  agents.value = [];
  drafts.value = [];
  approvals.value = {};
  releases.value = [];
  bindings.value = [];
  runtimes.value = [];
  models.value = [];
  void refresh();
}, { immediate: true });
watch(selectedAgentID, () => void loadLifecycle());

async function refresh() {
  const requestedTeam = teamID.value;
  if (!requestedTeam) return;
  const sequence = ++refreshSequence;
  loading.value = true; error.value = undefined;
  try {
    const [agentValues, bindingValues, runtimeValues, modelValues] = await Promise.all([
      api.listAgents(requestedTeam), api.listRepositoryBindings(requestedTeam), loadAllRuntimes(), api.listConfiguredModels(),
    ]);
    if (sequence !== refreshSequence || teamID.value !== requestedTeam) return;
    agents.value = agentValues; bindings.value = bindingValues; runtimes.value = runtimeValues; models.value = modelValues;
    if (!agentValues.some((agent) => agent.id === selectedAgentID.value)) selectedAgentID.value = agentValues[0]?.id ?? "";
    if (!selectedAgentID.value) drafts.value = [];
  } catch (reason) { if (sequence === refreshSequence) error.value = asApiError(reason); }
  finally { if (sequence === refreshSequence) loading.value = false; }
}

async function loadAllRuntimes() {
  const result: RuntimeImage[] = [];
  let token = "";
  do { const page = await api.listRuntimeImages(token, 100); result.push(...page.items); token = page.nextPageToken; } while (token);
  return result;
}

async function loadLifecycle() {
  const agentID = selectedAgentID.value;
  const requestedTeam = teamID.value;
  if (!agentID || !requestedTeam) { drafts.value = []; approvals.value = {}; releases.value = []; return; }
  try {
    const [draftValues, releaseValues] = await Promise.all([api.listAgentDrafts(agentID, requestedTeam), api.listAgentReleases(agentID, requestedTeam)]);
    const approvalEntries = await Promise.all(draftValues.filter((draft) => draft.release_risk === "high" && draft.id).map(async (draft) => {
      try { return [draft.id!, await api.getAgentDraftApproval(agentID, draft.id!, requestedTeam)] as const; }
      catch (reason) { if (reason instanceof ApiError && reason.kind === "not_found") return undefined; throw reason; }
    }));
    if (selectedAgentID.value === agentID && teamID.value === requestedTeam) {
      drafts.value = draftValues; releases.value = releaseValues;
      approvals.value = Object.fromEntries(approvalEntries.filter((entry): entry is readonly [string, ReleaseApproval] => Boolean(entry)));
    }
  } catch (reason) { if (selectedAgentID.value === agentID && teamID.value === requestedTeam) error.value = asApiError(reason); }
}

function intent(scope: string, input: unknown) {
  const fingerprint = JSON.stringify(input);
  const current = intents.get(scope);
  if (current?.fingerprint === fingerprint) return current.key;
  const key = crypto.randomUUID(); intents.set(scope, { fingerprint, key }); return key;
}

async function createAgent() {
  if (!canBuild.value || saving.value) return;
  const input = { name: agentForm.name, description: agentForm.description };
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    const scope = `agent.create:${teamID.value}`;
    const created = await api.createAgent(teamID.value, input, intent(scope, input));
    intents.delete(scope); agentModal.value = false; Object.assign(agentForm, { name: "", description: "" });
    await refresh(); selectedAgentID.value = created.id ?? ""; notice.value = t("agentCatalog.notice.agentCreated");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

function beginDraft(draft?: AgentDraft) {
  editingDraft.value = draft;
  const configuration = draft?.configuration;
  Object.assign(draftForm, {
    instructions: configuration?.instructions ?? "", bindingID: configuration?.repository_binding_id ?? bindings.value.find((binding) => binding.validation_report?.valid)?.id ?? "",
    runtimeID: configuration?.runtime_image_id ?? runtimes.value.find((runtime) => runtime.status === "production")?.id ?? "",
    modelID: configuration?.configured_model_id ?? models.value.find((model) => model.enabled)?.id ?? "",
    maxInputTokens: configuration?.model_budget?.max_input_tokens ?? 100000, maxOutputTokens: configuration?.model_budget?.max_output_tokens ?? 20000,
    maxCostAmount: configuration?.model_budget?.max_cost_amount ?? "50.00", timeoutSeconds: configuration?.execution_limits?.timeout_seconds ?? 1800,
    cpus: configuration?.execution_limits?.cpus ?? 2, memoryBytes: configuration?.execution_limits?.memory_bytes ?? 4294967296,
    pids: configuration?.execution_limits?.pids ?? 256, tempBytes: configuration?.execution_limits?.temp_bytes ?? 10737418240,
    nativeSubagents: configuration?.native_subagents ?? false, releaseRisk: draft?.release_risk ?? "low",
  });
  draftModal.value = true; error.value = undefined; notice.value = "";
}

function draftInput(): DraftInput {
  return { release_risk: draftForm.releaseRisk, configuration: {
    instructions: draftForm.instructions, repository_binding_id: draftForm.bindingID, runtime_image_id: draftForm.runtimeID,
    configured_model_id: draftForm.modelID, model_budget: { max_input_tokens: draftForm.maxInputTokens, max_output_tokens: draftForm.maxOutputTokens, max_cost_amount: draftForm.maxCostAmount },
    execution_limits: { timeout_seconds: draftForm.timeoutSeconds, cpus: draftForm.cpus, memory_bytes: draftForm.memoryBytes, pids: draftForm.pids, temp_bytes: draftForm.tempBytes, egress: "public" },
    native_subagents: draftForm.nativeSubagents,
  } };
}

async function saveDraft() {
  const agentID = selectedAgentID.value;
  if (!canBuild.value || !agentID || saving.value) return;
  const input = draftInput();
  const scope = editingDraft.value?.id ? `draft.update:${teamID.value}:${editingDraft.value.id}` : `draft.create:${teamID.value}:${agentID}`;
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    if (editingDraft.value?.id && editingDraft.value.version) await api.updateAgentDraft(agentID, editingDraft.value.id, teamID.value, input, editingDraft.value.version, intent(scope, input));
    else await api.createAgentDraft(agentID, teamID.value, input, intent(scope, input));
    intents.delete(scope); draftModal.value = false; editingDraft.value = undefined; await loadLifecycle(); notice.value = t("agentCatalog.notice.draftSaved");
  } catch (reason) {
    const failure = asApiError(reason);
    if (failure.kind === "conflict" && editingDraft.value?.id) {
      const current = await api.getAgentDraft(agentID, editingDraft.value.id, teamID.value).catch(() => undefined);
      if (current) editingDraft.value = current;
    }
    error.value = failure;
  } finally { saving.value = false; }
}

async function validateDraft(draft: AgentDraft) {
  if (!canBuild.value || !selectedAgentID.value || !draft.id || !draft.version || saving.value) return;
  const scope = `draft.validate:${teamID.value}:${draft.id}`;
  saving.value = true; validatingDraftID.value = draft.id; error.value = undefined; notice.value = "";
  try {
    await api.validateAgentDraft(selectedAgentID.value, draft.id, teamID.value, draft.version, intent(scope, { teamID: teamID.value, version: draft.version }));
    intents.delete(scope); await loadLifecycle(); notice.value = t("agentCatalog.notice.validated");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; validatingDraftID.value = ""; }
}

function approvalFor(draft: AgentDraft) { return draft.id ? approvals.value[draft.id] : undefined; }
function approvalIsCurrent(draft: AgentDraft, approval?: ReleaseApproval) { return Boolean(approval && approval.draft_version === draft.version); }

async function requestReleaseApproval() {
  const draft = approvalModal.value;
  if (!canBuild.value || !selectedAgentID.value || !draft?.id || !draft.version || saving.value) return;
  const input = { teamID: teamID.value, draftVersion: draft.version, riskReason: approvalForm.riskReason };
  const scope = `release-approval.request:${teamID.value}:${draft.id}`;
  saving.value = true; error.value = undefined;
  try {
    await api.requestAgentDraftApproval(selectedAgentID.value, draft.id, teamID.value, approvalForm.riskReason, intent(scope, input));
    intents.delete(scope); approvalModal.value = undefined; approvalForm.riskReason = ""; await loadLifecycle(); notice.value = t("agentCatalog.notice.approvalRequested");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function decideReleaseApproval() {
  const decision = decisionModal.value;
  if (!canBuild.value || !selectedAgentID.value || !decision?.draft.id || !decision.approval.version || saving.value) return;
  const input = { teamID: teamID.value, approvalVersion: decision.approval.version, approved: decision.approved, reason: decisionForm.reason };
  const scope = `release-approval.decide:${teamID.value}:${decision.approval.id}`;
  saving.value = true; error.value = undefined;
  try {
    await api.decideAgentDraftApproval(selectedAgentID.value, decision.draft.id, teamID.value, decision.approved, decisionForm.reason, decision.approval.version, intent(scope, input));
    intents.delete(scope); decisionModal.value = undefined; decisionForm.reason = ""; await loadLifecycle(); notice.value = t("agentCatalog.notice.approvalDecided");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function publishDraft(draft: AgentDraft) {
  if (!canBuild.value || !selectedAgentID.value || !draft.id || !draft.version || saving.value) return;
  const scope = `agent-release.publish:${teamID.value}:${draft.id}`;
  saving.value = true; error.value = undefined;
  try {
    await api.publishAgentDraft(selectedAgentID.value, draft.id, teamID.value, intent(scope, { teamID: teamID.value, draftVersion: draft.version }));
    intents.delete(scope); await loadLifecycle(); notice.value = t("agentCatalog.notice.published");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function deprecateRelease(release: AgentRelease) {
  if (!canBuild.value || !selectedAgentID.value || !release.id || !release.version || saving.value) return;
  const scope = `agent-release.deprecate:${teamID.value}:${release.id}`;
  saving.value = true; error.value = undefined;
  try {
    await api.deprecateAgentRelease(selectedAgentID.value, release.id, teamID.value, release.version, intent(scope, { teamID: teamID.value, version: release.version }));
    intents.delete(scope); await loadLifecycle(); notice.value = t("agentCatalog.notice.deprecated");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function blockRelease() {
  const release = blockModal.value;
  if (!canBlock.value || !selectedAgentID.value || !release?.id || !release.version || saving.value) return;
  const input = { teamID: teamID.value, version: release.version, reason: blockForm.reason };
  const scope = `agent-release.block:${teamID.value}:${release.id}`;
  saving.value = true; error.value = undefined;
  try {
    await api.blockAgentRelease(selectedAgentID.value, release.id, teamID.value, blockForm.reason, release.version, intent(scope, input));
    intents.delete(scope); blockModal.value = undefined; blockForm.reason = ""; await loadLifecycle(); notice.value = t("agentCatalog.notice.blocked");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

function asApiError(reason: unknown) { return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "unknown", ""); }
function errorLabel(value: ApiError) { return t(value.kind === "forbidden" ? "errors.forbidden" : value.kind === "conflict" ? "errors.conflict" : value.kind === "validation" ? "errors.validation" : "errors.server"); }
</script>

<template>
  <section class="agent-catalog" data-testid="agent-catalog">
    <header class="catalog-header"><div><p class="kicker">{{ t('agentCatalog.kicker') }}</p><h2>{{ t('agentCatalog.title') }}</h2><p>{{ t('agentCatalog.body') }}</p></div><button v-if="canBuild" class="primary-action" data-testid="create-agent" @click="agentModal = true">{{ t('agentCatalog.createAgent') }}</button><span v-else class="read-only-badge">{{ t('agentCatalog.readOnly') }}</span></header>
    <p v-if="notice" class="catalog-notice" role="status" data-testid="agent-notice">{{ notice }}</p>
    <div v-if="error" class="catalog-error" role="alert"><strong>{{ errorLabel(error) }}</strong><span>{{ t(error.kind === 'conflict' ? 'agentCatalog.conflictBody' : 'agentCatalog.errorBody') }}</span><button @click="refresh">{{ t('runtimeCatalog.retry') }}</button></div>
    <div v-if="loading" class="catalog-loading"><i></i><span>{{ t('agentCatalog.loading') }}</span></div>
    <div v-else class="agent-workbench">
      <aside class="agent-list"><header><span>{{ t('agentCatalog.agents') }}</span></header><p v-if="agents.length === 0">{{ t('agentCatalog.noAgents') }}</p><button v-for="agent in agents" :key="agent.id" :class="{ active: agent.id === selectedAgentID }" :data-testid="`agent-${agent.id}`" @click="selectedAgentID = agent.id ?? ''"><strong>{{ agent.name }}</strong><span>{{ agent.description }}</span></button></aside>
      <section class="draft-board">
        <header><div><span>{{ t('agentCatalog.drafts') }}</span><h3>{{ selectedAgent?.name ?? t('agentCatalog.selectAgent') }}</h3></div><button v-if="canBuild && selectedAgent" class="secondary-action" data-testid="create-draft" @click="beginDraft()">{{ t('agentCatalog.createDraft') }}</button></header>
        <p v-if="selectedAgent && drafts.length === 0">{{ t('agentCatalog.noDrafts') }}</p>
        <article v-for="draft in drafts" :key="draft.id" class="draft-card" :data-testid="`draft-${draft.id}`">
          <div class="record-heading"><div><span>R{{ draft.revision }} · V{{ draft.version }}</span><h4>{{ t(`agentCatalog.state.${validatingDraftID === draft.id ? 'validating' : draft.state ?? 'draft'}`) }}</h4></div><em :class="draft.state === 'ready' ? 'state-production' : 'state-blocked'">{{ t(`agentCatalog.risk.${draft.release_risk ?? 'low'}`) }}</em></div>
          <dl class="binding-facts"><div><dt>{{ t('agentCatalog.runtimeImage') }}</dt><dd>{{ draft.configuration?.runtime_image_id }}</dd></div><div><dt>{{ t('agentCatalog.configuredModel') }}</dt><dd>{{ draft.configuration?.configured_model_id }}</dd></div><div class="wide"><dt>{{ t('agentCatalog.repositoryBinding') }}</dt><dd>{{ draft.configuration?.repository_binding_id }}</dd></div></dl>
          <ul v-if="draft.validation_report && !draft.validation_report.valid" class="validation-errors"><li v-for="(message, field) in draft.validation_report.errors" :key="field"><code>{{ field }}</code><span>{{ message }}</span></li></ul>
          <section v-if="draft.release_risk === 'high'" class="release-approval" :data-testid="`release-approval-${draft.id}`">
            <strong>{{ t('agentCatalog.releaseApproval.title') }}</strong>
            <template v-if="approvalFor(draft)">
              <span>{{ t('agentCatalog.releaseApproval.state') }}: {{ t(`agentCatalog.releaseApproval.status.${approvalFor(draft)?.state ?? 'pending'}`) }}</span>
              <span>{{ t('agentCatalog.releaseApproval.requestedBy') }}: {{ approvalFor(draft)?.requested_by }}</span>
              <span>{{ t('agentCatalog.releaseApproval.draftVersion') }}: V{{ approvalFor(draft)?.draft_version }}</span>
              <p>{{ approvalFor(draft)?.risk_reason }}</p>
              <em v-if="!approvalIsCurrent(draft, approvalFor(draft))">{{ t('agentCatalog.releaseApproval.expired') }}</em>
            </template>
            <span v-else>{{ t('agentCatalog.releaseApproval.notRequested') }}</span>
          </section>
          <footer v-if="canBuild">
            <button class="record-action" :disabled="saving" :data-testid="`edit-draft-${draft.id}`" @click="beginDraft(draft)">{{ t('agentCatalog.edit') }}</button>
            <button class="record-action" :disabled="saving" :data-testid="`validate-draft-${draft.id}`" @click="validateDraft(draft)">{{ t('agentCatalog.validate') }}</button>
            <button v-if="draft.release_risk === 'high' && draft.state === 'ready' && !approvalIsCurrent(draft, approvalFor(draft))" class="record-action" :disabled="saving" :data-testid="`request-release-approval-${draft.id}`" @click="approvalModal = draft">{{ t('agentCatalog.releaseApproval.request') }}</button>
            <template v-if="approvalFor(draft)?.state === 'pending' && approvalIsCurrent(draft, approvalFor(draft)) && approvalFor(draft)?.requested_by !== currentUser?.user_id">
              <button class="record-action" :disabled="saving" :data-testid="`approve-release-${draft.id}`" @click="decisionModal = { draft, approval: approvalFor(draft)!, approved: true }">{{ t('agentCatalog.releaseApproval.approve') }}</button>
              <button class="record-action" :disabled="saving" :data-testid="`reject-release-${draft.id}`" @click="decisionModal = { draft, approval: approvalFor(draft)!, approved: false }">{{ t('agentCatalog.releaseApproval.reject') }}</button>
            </template>
            <button v-if="draft.state === 'ready' && !releasedDraftIDs.has(draft.id) && (draft.release_risk === 'low' || approvalIsCurrent(draft, approvalFor(draft)) && approvalFor(draft)?.state === 'approved')" class="primary-action compact-action" :disabled="saving" :data-testid="`publish-draft-${draft.id}`" @click="publishDraft(draft)">{{ t('agentCatalog.publish') }}</button>
          </footer>
        </article>

        <section v-if="selectedAgent" class="release-board" data-testid="release-board">
          <header><div><span>{{ t('agentCatalog.releases') }}</span><h3>{{ t('agentCatalog.releaseImmutable') }}</h3></div></header>
          <p v-if="releases.length === 0">{{ t('agentCatalog.noReleases') }}</p>
          <article v-for="release in releases" :key="release.id" class="release-card" :data-testid="`release-${release.id}`">
            <div class="record-heading"><div><span>#{{ release.release_number }} · V{{ release.version }}</span><h4>{{ t(`agentCatalog.releaseStatus.${release.status ?? 'released'}`) }}</h4></div><em>{{ t(`agentCatalog.risk.${release.release_risk ?? 'low'}`) }}</em></div>
            <dl class="release-snapshot">
              <div><dt>{{ t('agentCatalog.repositoryBinding') }}</dt><dd>{{ release.repository_binding_snapshot?.name }} · {{ release.repository_binding_snapshot?.default_branch }}</dd></div>
              <div><dt>{{ t('agentCatalog.runtimeImage') }}</dt><dd>{{ release.runtime_image_snapshot?.runtime }} · {{ release.runtime_image_snapshot?.image_digest }}</dd></div>
              <div><dt>{{ t('agentCatalog.configuredModel') }}</dt><dd>{{ release.configured_model_snapshot?.name }} · {{ release.configured_model_snapshot?.model_id }}</dd></div>
              <div><dt>{{ t('repositoryCatalog.budget') }}</dt><dd>{{ release.configuration?.model_budget?.max_input_tokens }} / {{ release.configuration?.model_budget?.max_output_tokens }} / {{ release.configuration?.model_budget?.max_cost_amount }}</dd></div>
              <div><dt>{{ t('agentCatalog.capabilities') }}</dt><dd>{{ Object.entries(release.runtime_image_snapshot?.capabilities ?? {}).filter(([, enabled]) => enabled).map(([name]) => name).join(', ') || t('agentCatalog.none') }}</dd></div>
              <div v-if="release.approval_evidence"><dt>{{ t('agentCatalog.releaseApproval.evidence') }}</dt><dd>V{{ release.approval_evidence.draft_version }} · {{ release.approval_evidence.requested_by }} → {{ release.approval_evidence.approved_by }} · {{ release.approval_evidence.risk_reason }}</dd></div>
              <div v-if="release.blocked_reason"><dt>{{ t('agentCatalog.blockReason') }}</dt><dd>{{ release.blocked_reason }}</dd></div>
            </dl>
            <footer v-if="release.status === 'released'"><button v-if="canBuild" class="record-action" :disabled="saving" :data-testid="`deprecate-release-${release.id}`" @click="deprecateRelease(release)">{{ t('agentCatalog.deprecate') }}</button><button v-if="canBlock" class="record-action danger-action" :disabled="saving" :data-testid="`block-release-${release.id}`" @click="blockModal = release">{{ t('agentCatalog.block') }}</button></footer>
          </article>
        </section>
      </section>
    </div>
    <div v-if="agentModal" class="modal-backdrop" @click.self="agentModal = false"><form class="catalog-modal compact-modal" data-testid="agent-form" @submit.prevent="createAgent"><header><span>{{ t('agentCatalog.newAgent') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="agentModal = false">×</button></header><h3>{{ t('agentCatalog.createAgent') }}</h3><div class="form-grid"><label><span>{{ t('modelCatalog.name') }}</span><input v-model.trim="agentForm.name" required data-testid="agent-name"></label><label class="wide"><span>{{ t('agentCatalog.description') }}</span><textarea v-model="agentForm.description" rows="3" data-testid="agent-description"></textarea></label></div><footer><button type="button" @click="agentModal = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-agent">{{ t('agentCatalog.create') }}</button></footer></form></div>
    <div v-if="draftModal" class="modal-backdrop binding-modal-backdrop" @click.self="draftModal = false"><form class="catalog-modal binding-modal" data-testid="draft-form" @submit.prevent="saveDraft"><header><span>{{ t(editingDraft ? 'agentCatalog.editDraft' : 'agentCatalog.newDraft') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="draftModal = false">×</button></header><h3>{{ t(editingDraft ? 'agentCatalog.edit' : 'agentCatalog.createDraft') }}</h3><div class="form-grid"><label class="wide"><span>{{ t('agentCatalog.instructions') }}</span><textarea v-model="draftForm.instructions" required rows="5" data-testid="draft-instructions"></textarea></label><label><span>{{ t('agentCatalog.repositoryBinding') }}</span><select v-model="draftForm.bindingID" required data-testid="draft-binding"><option v-for="binding in bindings" :key="binding.id" :value="binding.id">{{ binding.name }} · {{ t(binding.validation_report?.valid ? 'agentCatalog.validated' : 'agentCatalog.unvalidated') }}</option></select></label><label><span>{{ t('agentCatalog.runtimeImage') }}</span><select v-model="draftForm.runtimeID" required data-testid="draft-runtime"><option v-for="runtime in runtimes" :key="runtime.id" :value="runtime.id">{{ runtime.runtime }} · {{ t(`runtimeCatalog.status.${runtime.status ?? 'experimental'}`) }}</option></select></label><label><span>{{ t('agentCatalog.configuredModel') }}</span><select v-model="draftForm.modelID" required data-testid="draft-model"><option v-for="model in models" :key="model.id" :value="model.id">{{ model.name }} · {{ t(model.enabled ? 'agentCatalog.enabled' : 'agentCatalog.disabled') }}</option></select></label><label><span>{{ t('agentCatalog.releaseRisk') }}</span><select v-model="draftForm.releaseRisk" data-testid="draft-risk"><option value="low">{{ t('agentCatalog.risk.low') }}</option><option value="high">{{ t('agentCatalog.risk.high') }}</option></select></label><label><span>{{ t('repositoryCatalog.inputBudget') }}</span><input v-model.number="draftForm.maxInputTokens" type="number" min="1" required data-testid="draft-input-budget"></label><label><span>{{ t('repositoryCatalog.outputBudget') }}</span><input v-model.number="draftForm.maxOutputTokens" type="number" min="1" required data-testid="draft-output-budget"></label><label><span>{{ t('repositoryCatalog.costBudget') }}</span><input v-model.trim="draftForm.maxCostAmount" required data-testid="draft-cost-budget"></label><label><span>{{ t('agentCatalog.timeout') }}</span><input v-model.number="draftForm.timeoutSeconds" type="number" min="1" max="7200" required></label><label><span>{{ t('agentCatalog.cpu') }}</span><input v-model.number="draftForm.cpus" type="number" min="0.1" step="0.1" required></label><label><span>{{ t('agentCatalog.memoryBytes') }}</span><input v-model.number="draftForm.memoryBytes" type="number" min="1" required></label><label><span>{{ t('agentCatalog.pids') }}</span><input v-model.number="draftForm.pids" type="number" min="1" required></label><label><span>{{ t('agentCatalog.tempBytes') }}</span><input v-model.number="draftForm.tempBytes" type="number" min="1" required></label><label><span>{{ t('agentCatalog.egress') }}</span><input :value="t('agentCatalog.publicEgress')" disabled></label><label class="wide checkbox-label"><input v-model="draftForm.nativeSubagents" type="checkbox" data-testid="draft-subagents"><span>{{ t('agentCatalog.nativeSubagents') }}</span></label></div><footer><button type="button" @click="draftModal = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-draft">{{ t('agentCatalog.save') }}</button></footer></form></div>
    <div v-if="approvalModal" class="modal-backdrop" @click.self="approvalModal = undefined"><form class="catalog-modal compact-modal" data-testid="release-approval-form" @submit.prevent="requestReleaseApproval"><header><span>{{ t('agentCatalog.releaseApproval.title') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="approvalModal = undefined">×</button></header><h3>{{ t('agentCatalog.releaseApproval.request') }}</h3><label><span>{{ t('agentCatalog.releaseApproval.riskReason') }}</span><textarea v-model.trim="approvalForm.riskReason" required maxlength="2000" rows="4" data-testid="approval-risk-reason"></textarea></label><footer><button type="button" @click="approvalModal = undefined">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-release-approval">{{ t('agentCatalog.releaseApproval.request') }}</button></footer></form></div>
    <div v-if="decisionModal" class="modal-backdrop" @click.self="decisionModal = undefined"><form class="catalog-modal compact-modal" data-testid="release-decision-form" @submit.prevent="decideReleaseApproval"><header><span>{{ t('agentCatalog.releaseApproval.title') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="decisionModal = undefined">×</button></header><h3>{{ t(decisionModal.approved ? 'agentCatalog.releaseApproval.approve' : 'agentCatalog.releaseApproval.reject') }}</h3><p>{{ decisionModal.approval.risk_reason }}</p><label><span>{{ t('agentCatalog.releaseApproval.decisionReason') }}</span><textarea v-model.trim="decisionForm.reason" :required="!decisionModal.approved" maxlength="2000" rows="4" data-testid="approval-decision-reason"></textarea></label><footer><button type="button" @click="decisionModal = undefined">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-release-decision">{{ t(decisionModal.approved ? 'agentCatalog.releaseApproval.approve' : 'agentCatalog.releaseApproval.reject') }}</button></footer></form></div>
    <div v-if="blockModal" class="modal-backdrop" @click.self="blockModal = undefined"><form class="catalog-modal compact-modal" data-testid="block-release-form" @submit.prevent="blockRelease"><header><span>{{ t('agentCatalog.block') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="blockModal = undefined">×</button></header><h3>{{ t('agentCatalog.blockRelease') }}</h3><label><span>{{ t('agentCatalog.blockReason') }}</span><textarea v-model.trim="blockForm.reason" required maxlength="2000" rows="4" data-testid="block-release-reason"></textarea></label><footer><button type="button" @click="blockModal = undefined">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action danger-action" :disabled="saving" data-testid="submit-block-release">{{ t('agentCatalog.block') }}</button></footer></form></div>
  </section>
</template>
