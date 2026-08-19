<script setup lang="ts">
import { computed, inject, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  ApiError, platformApiKey, type ConfiguredModel, type RepositoryBinding,
  type RepositoryBindingInput, type RuntimeImage, type SourceControlProvider,
} from "../api/client";
import { authContextKey } from "../auth/session";

const injectedApi = inject(platformApiKey);
const auth = inject(authContextKey);
if (!injectedApi || !auth) throw new Error("Repository Binding dependencies are required");
const api = injectedApi;
const route = useRoute();
const { t } = useI18n();

const providers = ref<SourceControlProvider[]>([]);
const bindings = ref<RepositoryBinding[]>([]);
const runtimes = ref<RuntimeImage[]>([]);
const models = ref<ConfiguredModel[]>([]);
const loading = ref(true);
const saving = ref(false);
const error = ref<ApiError>();
const notice = ref("");
const providerModal = ref(false);
const bindingModal = ref(false);
const editing = ref<RepositoryBinding>();
const providerForm = reactive({ name: "", kind: "github_com", baseURL: "https://github.com" });
type QualityCommandForm = { name: string; kind: string; executable: string; arguments: string[]; timeoutSeconds: number };
const runtimeCapabilities = ["streaming", "structured_final", "native_resume", "subagents", "usage"];
const bindingForm = reactive({
  providerID: "", name: "", repositorySSHURL: "", defaultBranch: "main", sshCredentialID: "",
  buildCredentialIDs: "", gitAuthorName: "Agent Platform", gitAuthorEmail: "", allowedRuntimeIDs: [] as string[], requiredRuntimeCapabilities: [] as string[],
  defaultRuntimeID: "", defaultModelID: "", maxInputTokens: 100000, maxOutputTokens: 20000,
  maxCostAmount: "50.00", instructions: "", qualityCommands: [newQualityCommand()] as QualityCommandForm[],
});
const intents = new Map<string, { fingerprint: string; key: string }>();
let refreshSequence = 0;
const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const teamID = computed(() => typeof route.query.team === "string" ? route.query.team : "");
const canAdminister = computed(() => (currentUser.value?.role_grants ?? []).some((grant) => grant.role === "platform_administrator" && !grant.team_id));
const enabledProviders = computed(() => providers.value.filter((provider) => provider.enabled));
const selectedRuntimes = computed(() => runtimes.value.filter((runtime) => runtime.id && bindingForm.allowedRuntimeIDs.includes(runtime.id)));

watch(teamID, () => void refresh(), { immediate: true });

async function refresh() {
  const requestedTeamID = teamID.value;
  if (!requestedTeamID) return;
  const sequence = ++refreshSequence;
  loading.value = true;
  error.value = undefined;
  try {
    const [providerValues, bindingValues, runtimeValues, modelValues] = await Promise.all([
      api.listSourceControlProviders(), api.listRepositoryBindings(requestedTeamID), loadAllRuntimes(), api.listConfiguredModels(),
    ]);
    if (sequence !== refreshSequence || teamID.value !== requestedTeamID) return;
    providers.value = providerValues;
    bindings.value = bindingValues;
    runtimes.value = runtimeValues;
    models.value = modelValues;
    if (editing.value?.id) {
      editing.value = bindingValues.find((binding) => binding.id === editing.value?.id) ?? editing.value;
    }
  } catch (reason) {
    if (sequence === refreshSequence) error.value = asApiError(reason);
  } finally {
    if (sequence === refreshSequence) loading.value = false;
  }
}

async function loadAllRuntimes() {
  const items: RuntimeImage[] = [];
  let pageToken = "";
  do {
    const page = await api.listRuntimeImages(pageToken, 100);
    items.push(...page.items);
    pageToken = page.nextPageToken;
  } while (pageToken);
  return items;
}

function intent(scope: string, input: unknown) {
  const fingerprint = JSON.stringify(input);
  const current = intents.get(scope);
  if (current?.fingerprint === fingerprint) return current.key;
  const key = crypto.randomUUID();
  intents.set(scope, { fingerprint, key });
  return key;
}

async function registerProvider() {
  if (!canAdminister.value || saving.value) return;
  const input = { name: providerForm.name, kind: providerForm.kind, base_url: providerForm.baseURL };
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.registerSourceControlProvider(input, intent("provider.register", input));
    intents.delete("provider.register"); providerModal.value = false;
    Object.assign(providerForm, { name: "", kind: "github_com", baseURL: "https://github.com" });
    await refresh(); notice.value = t("repositoryCatalog.notice.providerRegistered");
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function toggleProvider(provider: SourceControlProvider) {
  if (!canAdminister.value || !provider.id || !provider.version || saving.value) return;
  const enabled = !provider.enabled;
  const scope = `provider.status:${provider.id}`;
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.changeSourceControlProviderStatus(provider.id, enabled, provider.version, intent(scope, { enabled }));
    intents.delete(scope); await refresh(); notice.value = t("repositoryCatalog.notice.providerChanged");
  } catch (reason) {
    const failure = asApiError(reason);
    if (failure.kind === "conflict") await refresh().catch(() => undefined);
    error.value = failure;
  } finally { saving.value = false; }
}

function beginBinding(binding?: RepositoryBinding) {
  editing.value = binding;
  Object.assign(bindingForm, {
    providerID: binding?.source_control_provider_id ?? enabledProviders.value[0]?.id ?? "",
    name: binding?.name ?? "", repositorySSHURL: binding?.repository_ssh_url ?? "", defaultBranch: binding?.default_branch ?? "main",
    sshCredentialID: binding?.ssh_credential_profile_id ?? "", buildCredentialIDs: (binding?.build_credential_profile_ids ?? []).join(", "),
    gitAuthorName: binding?.git_author_name ?? "Agent Platform", gitAuthorEmail: binding?.git_author_email ?? "",
    allowedRuntimeIDs: [...(binding?.allowed_runtime_image_ids ?? [])], defaultRuntimeID: binding?.default_runtime_image_id ?? "",
    requiredRuntimeCapabilities: [...(binding?.required_runtime_capabilities ?? [])],
    defaultModelID: binding?.default_model_id ?? models.value.find((model) => model.enabled)?.id ?? "",
    maxInputTokens: binding?.model_budget?.max_input_tokens ?? 100000, maxOutputTokens: binding?.model_budget?.max_output_tokens ?? 20000,
    maxCostAmount: binding?.model_budget?.max_cost_amount ?? "50.00", instructions: binding?.instructions ?? "",
    qualityCommands: binding?.quality_commands?.length ? binding.quality_commands.map((command) => ({
      name: command.name ?? "", kind: command.kind ?? "test", executable: command.executable ?? "",
      arguments: [...(command.arguments ?? [])], timeoutSeconds: command.timeout_seconds ?? 900,
    })) : [newQualityCommand()],
  });
  bindingModal.value = true; error.value = undefined; notice.value = "";
}

function bindingInput(): RepositoryBindingInput {
  return {
    team_id: teamID.value, source_control_provider_id: bindingForm.providerID, name: bindingForm.name,
    repository_ssh_url: bindingForm.repositorySSHURL, default_branch: bindingForm.defaultBranch,
    ssh_credential_profile_id: bindingForm.sshCredentialID,
    build_credential_profile_ids: bindingForm.buildCredentialIDs.split(",").map((value) => value.trim()).filter(Boolean),
    git_author_name: bindingForm.gitAuthorName, git_author_email: bindingForm.gitAuthorEmail,
    allowed_runtime_image_ids: [...bindingForm.allowedRuntimeIDs], default_runtime_image_id: bindingForm.defaultRuntimeID,
    required_runtime_capabilities: [...bindingForm.requiredRuntimeCapabilities],
    default_model_id: bindingForm.defaultModelID,
    model_budget: { max_input_tokens: bindingForm.maxInputTokens, max_output_tokens: bindingForm.maxOutputTokens, max_cost_amount: bindingForm.maxCostAmount },
    instructions: bindingForm.instructions,
    quality_commands: bindingForm.qualityCommands.map((command) => ({
      name: command.name, kind: command.kind, executable: command.executable,
      arguments: [...command.arguments], timeout_seconds: command.timeoutSeconds,
    })),
    egress_policy: { mode: "public" },
  };
}

async function saveBinding() {
  if (!canAdminister.value || saving.value) return;
  const input = bindingInput();
  const scope = editing.value?.id ? `binding.update:${editing.value.id}` : "binding.register";
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    if (editing.value?.id && editing.value.version) await api.updateRepositoryBinding(editing.value.id, input, editing.value.version, intent(scope, input));
    else await api.registerRepositoryBinding(input, intent(scope, input));
    intents.delete(scope); bindingModal.value = false; editing.value = undefined;
    await refresh(); notice.value = t("repositoryCatalog.notice.bindingSaved");
  } catch (reason) {
    const failure = asApiError(reason);
    if (failure.kind === "conflict") await refresh().catch(() => undefined);
    error.value = failure;
  } finally { saving.value = false; }
}

async function validateBinding(binding: RepositoryBinding) {
  if (!canAdminister.value || !binding.id || !binding.version || saving.value) return;
  const scope = `binding.validate:${binding.id}`;
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.validateRepositoryBinding(binding.id, teamID.value, binding.version, intent(scope, { team_id: teamID.value }));
    intents.delete(scope); await refresh(); notice.value = t("repositoryCatalog.notice.bindingValidated");
  } catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict") await refresh().catch(() => undefined);
  } finally { saving.value = false; }
}

function toggleAllowedRuntime(id: string) {
  const index = bindingForm.allowedRuntimeIDs.indexOf(id);
  if (index >= 0) bindingForm.allowedRuntimeIDs.splice(index, 1);
  else bindingForm.allowedRuntimeIDs.push(id);
  if (!bindingForm.allowedRuntimeIDs.includes(bindingForm.defaultRuntimeID)) bindingForm.defaultRuntimeID = bindingForm.allowedRuntimeIDs[0] ?? "";
}
function toggleRequiredCapability(capability: string) {
  const index = bindingForm.requiredRuntimeCapabilities.indexOf(capability);
  if (index >= 0) bindingForm.requiredRuntimeCapabilities.splice(index, 1);
  else bindingForm.requiredRuntimeCapabilities.push(capability);
}
function newQualityCommand(): QualityCommandForm { return { name: "test", kind: "test", executable: "", arguments: [], timeoutSeconds: 900 }; }
function addQualityCommand() { if (bindingForm.qualityCommands.length < 20) bindingForm.qualityCommands.push(newQualityCommand()); }
function removeQualityCommand(index: number) { if (bindingForm.qualityCommands.length > 1) bindingForm.qualityCommands.splice(index, 1); }
function addQualityArgument(command: QualityCommandForm) { if (command.arguments.length < 100) command.arguments.push(""); }
function removeQualityArgument(command: QualityCommandForm, index: number) { command.arguments.splice(index, 1); }
function providerName(id?: string) { return providers.value.find((provider) => provider.id === id)?.name ?? t("repositoryCatalog.missingDependency"); }
function runtimeName(id?: string) { const value = runtimes.value.find((runtime) => runtime.id === id); return value ? `${value.runtime} / ${value.cli_version}` : t("repositoryCatalog.missingDependency"); }
function modelName(id?: string) { return models.value.find((model) => model.id === id)?.name ?? t("repositoryCatalog.missingDependency"); }
function asApiError(reason: unknown) { return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "unknown", ""); }
function errorLabel(value: ApiError) {
  if (value.kind === "forbidden") return t("errors.forbidden");
  if (value.kind === "conflict") return t("errors.conflict");
  if (value.kind === "validation") return t("errors.validation");
  return t("errors.server");
}
</script>

<template>
  <section class="repository-catalog" data-testid="repository-catalog">
    <header class="catalog-header"><div><p class="kicker">{{ t('repositoryCatalog.kicker') }}</p><h2>{{ t('repositoryCatalog.title') }}</h2><p>{{ t('repositoryCatalog.body') }}</p></div><span v-if="!canAdminister" class="read-only-badge">{{ t('repositoryCatalog.readOnly') }}</span></header>
    <p class="policy-note"><strong>{{ t('repositoryCatalog.secretBoundary') }}</strong> {{ t('repositoryCatalog.secretBoundaryBody') }}</p>
    <p v-if="notice" class="catalog-notice" role="status" data-testid="repository-notice">{{ notice }}</p>
    <div v-if="error" class="catalog-error" role="alert"><strong>{{ errorLabel(error) }}</strong><span>{{ t(error.kind === 'conflict' ? 'repositoryCatalog.conflictBody' : 'repositoryCatalog.errorBody') }}</span><small v-if="error.requestID">{{ t('runtimeCatalog.requestId', { id: error.requestID }) }}</small><button @click="refresh">{{ t('runtimeCatalog.retry') }}</button></div>
    <div v-if="loading" class="catalog-loading"><i></i><span>{{ t('repositoryCatalog.loading') }}</span></div>
    <div v-else class="repository-grid">
      <section class="catalog-column" data-testid="provider-catalog"><header><div><span>{{ t('repositoryCatalog.providerSection') }}</span><h3>{{ t('repositoryCatalog.providers') }}</h3></div><button v-if="canAdminister" class="secondary-action" data-testid="register-provider" @click="providerModal = true">{{ t('repositoryCatalog.addProvider') }}</button></header><div v-if="providers.length === 0" class="mini-empty"><strong>{{ t('repositoryCatalog.noProviders') }}</strong></div><article v-for="provider in providers" :key="provider.id" class="catalog-record" :data-testid="`provider-${provider.id}`"><div class="record-heading"><div><span>{{ provider.kind }}</span><h4>{{ provider.name }}</h4></div><em :class="provider.enabled ? 'state-production' : 'state-blocked'">{{ t(provider.enabled ? 'modelCatalog.enabled' : 'modelCatalog.disabled') }}</em></div><dl><div class="wide"><dt>{{ t('repositoryCatalog.baseURL') }}</dt><dd><code>{{ provider.base_url }}</code></dd></div></dl><button v-if="canAdminister" class="record-action" :disabled="saving" :data-testid="`toggle-provider-${provider.id}`" @click="toggleProvider(provider)">{{ t(provider.enabled ? 'modelCatalog.disable' : 'modelCatalog.enable') }}</button></article></section>
      <section class="binding-column" data-testid="binding-catalog"><header><div><span>{{ t('repositoryCatalog.bindingSection') }}</span><h3>{{ t('repositoryCatalog.bindings') }}</h3></div><button v-if="canAdminister" class="secondary-action" :disabled="enabledProviders.length === 0" data-testid="register-binding" @click="beginBinding()">{{ t('repositoryCatalog.addBinding') }}</button></header><div v-if="bindings.length === 0" class="mini-empty"><strong>{{ t('repositoryCatalog.noBindings') }}</strong></div><article v-for="binding in bindings" :key="binding.id" class="binding-record" :data-testid="`binding-${binding.id}`"><div class="record-heading"><div><span>{{ binding.default_branch }}</span><h4>{{ binding.name }}</h4></div><em :class="binding.validation_report?.valid ? 'state-production' : 'state-blocked'">{{ t(binding.validation_report?.valid ? 'repositoryCatalog.valid' : binding.validation_report ? 'repositoryCatalog.invalid' : 'repositoryCatalog.unvalidated') }}</em></div><dl class="binding-facts"><div class="wide"><dt>{{ t('repositoryCatalog.repository') }}</dt><dd><code>{{ binding.repository_ssh_url }}</code></dd></div><div><dt>{{ t('repositoryCatalog.provider') }}</dt><dd>{{ providerName(binding.source_control_provider_id) }}</dd></div><div><dt>{{ t('repositoryCatalog.defaults') }}</dt><dd>{{ runtimeName(binding.default_runtime_image_id) }} · {{ modelName(binding.default_model_id) }}</dd></div><div><dt>{{ t('repositoryCatalog.requiredCapabilities') }}</dt><dd>{{ binding.required_runtime_capabilities?.join(', ') || '—' }}</dd></div><div><dt>{{ t('repositoryCatalog.sshCredential') }}</dt><dd><code>{{ binding.ssh_credential_profile_id }}</code></dd></div><div><dt>{{ t('repositoryCatalog.buildCredentials') }}</dt><dd>{{ binding.build_credential_profile_ids?.join(', ') || '—' }}</dd></div><div><dt>{{ t('repositoryCatalog.budget') }}</dt><dd>{{ binding.model_budget?.max_input_tokens }} / {{ binding.model_budget?.max_output_tokens }} / {{ binding.model_budget?.max_cost_amount }}</dd></div><div class="wide"><dt>{{ t('repositoryCatalog.egress') }}</dt><dd>{{ binding.egress_policy?.mode }} · {{ t('repositoryCatalog.publicOnly') }}</dd></div></dl><ul v-if="binding.validation_report && !binding.validation_report.valid" class="validation-errors" :aria-label="t('repositoryCatalog.validationErrors')"><li v-for="(message, field) in binding.validation_report.errors" :key="field"><code>{{ field }}</code><span>{{ message }}</span></li></ul><footer v-if="canAdminister"><button class="record-action" :disabled="saving" :data-testid="`edit-binding-${binding.id}`" @click="beginBinding(binding)">{{ t('repositoryCatalog.edit') }}</button><button class="record-action" :disabled="saving" :data-testid="`validate-binding-${binding.id}`" @click="validateBinding(binding)">{{ t('repositoryCatalog.validate') }}</button></footer></article></section>
    </div>

    <div v-if="providerModal" class="modal-backdrop" @click.self="providerModal = false"><form class="catalog-modal compact-modal" data-testid="provider-form" @submit.prevent="registerProvider"><header><span>{{ t('repositoryCatalog.newProvider') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="providerModal = false">×</button></header><h3>{{ t('repositoryCatalog.addProvider') }}</h3><div class="form-grid"><label><span>{{ t('modelCatalog.name') }}</span><input v-model.trim="providerForm.name" required data-testid="provider-name"></label><label><span>{{ t('repositoryCatalog.kind') }}</span><select v-model="providerForm.kind" data-testid="provider-kind" @change="providerForm.baseURL = providerForm.kind === 'github_com' ? 'https://github.com' : ''"><option value="github_com">GitHub.com</option><option value="gitlab_self_managed">GitLab Self-Managed</option></select></label><label class="wide"><span>{{ t('repositoryCatalog.baseURL') }}</span><input v-model.trim="providerForm.baseURL" type="url" required pattern="https://.*" data-testid="provider-base-url"></label></div><footer><button type="button" @click="providerModal = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-provider">{{ t('modelCatalog.register') }}</button></footer></form></div>

    <div v-if="bindingModal" class="modal-backdrop binding-modal-backdrop" @click.self="bindingModal = false"><form class="catalog-modal binding-modal" data-testid="binding-form" @submit.prevent="saveBinding"><header><span>{{ t(editing ? 'repositoryCatalog.editBinding' : 'repositoryCatalog.newBinding') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="bindingModal = false">×</button></header><h3>{{ t(editing ? 'repositoryCatalog.edit' : 'repositoryCatalog.addBinding') }}</h3><div class="form-grid binding-form-grid"><label><span>{{ t('modelCatalog.name') }}</span><input v-model.trim="bindingForm.name" required data-testid="binding-name"></label><label><span>{{ t('repositoryCatalog.provider') }}</span><select v-model="bindingForm.providerID" required data-testid="binding-provider"><option v-for="provider in enabledProviders" :key="provider.id" :value="provider.id">{{ provider.name }}</option></select></label><label class="wide"><span>{{ t('repositoryCatalog.repositorySSHURL') }}</span><input v-model.trim="bindingForm.repositorySSHURL" required placeholder="git@github.com:org/repo.git" data-testid="binding-repository-url"></label><label><span>{{ t('repositoryCatalog.defaultBranch') }}</span><input v-model.trim="bindingForm.defaultBranch" required data-testid="binding-default-branch"></label><label><span>{{ t('repositoryCatalog.sshCredential') }}</span><input v-model.trim="bindingForm.sshCredentialID" required data-testid="binding-ssh-credential"></label><label class="wide"><span>{{ t('repositoryCatalog.buildCredentialsHint') }}</span><input v-model.trim="bindingForm.buildCredentialIDs" data-testid="binding-build-credentials"></label><label><span>{{ t('repositoryCatalog.gitAuthorName') }}</span><input v-model.trim="bindingForm.gitAuthorName" required data-testid="binding-author-name"></label><label><span>{{ t('repositoryCatalog.gitAuthorEmail') }}</span><input v-model.trim="bindingForm.gitAuthorEmail" type="email" required data-testid="binding-author-email"></label><fieldset class="wide runtime-checks"><legend>{{ t('repositoryCatalog.allowedRuntimes') }}</legend><label v-for="runtime in runtimes" :key="runtime.id"><input type="checkbox" :checked="Boolean(runtime.id && bindingForm.allowedRuntimeIDs.includes(runtime.id))" :data-testid="`binding-runtime-${runtime.id}`" @change="runtime.id && toggleAllowedRuntime(runtime.id)"><span>{{ runtime.runtime }} / {{ runtime.cli_version }} · {{ runtime.status }}</span></label></fieldset><label><span>{{ t('repositoryCatalog.defaultRuntime') }}</span><select v-model="bindingForm.defaultRuntimeID" required data-testid="binding-default-runtime"><option v-for="runtime in selectedRuntimes" :key="runtime.id" :value="runtime.id">{{ runtime.runtime }} / {{ runtime.cli_version }}</option></select></label><label><span>{{ t('repositoryCatalog.defaultModel') }}</span><select v-model="bindingForm.defaultModelID" required data-testid="binding-default-model"><option v-for="model in models" :key="model.id" :value="model.id">{{ model.name }} · {{ model.enabled ? t('modelCatalog.enabled') : t('modelCatalog.disabled') }}</option></select></label><label><span>{{ t('repositoryCatalog.inputBudget') }}</span><input v-model.number="bindingForm.maxInputTokens" type="number" min="1" required data-testid="binding-input-budget"></label><label><span>{{ t('repositoryCatalog.outputBudget') }}</span><input v-model.number="bindingForm.maxOutputTokens" type="number" min="1" required data-testid="binding-output-budget"></label><label><span>{{ t('repositoryCatalog.costBudget') }}</span><input v-model.trim="bindingForm.maxCostAmount" inputmode="decimal" required data-testid="binding-cost-budget"></label><label class="wide"><span>{{ t('repositoryCatalog.instructions') }}</span><textarea v-model="bindingForm.instructions" rows="3" data-testid="binding-instructions"></textarea></label><fieldset v-for="(command, commandIndex) in bindingForm.qualityCommands" :key="commandIndex" class="wide quality-command-editor"><legend>{{ t('repositoryCatalog.qualityCommand', { number: commandIndex + 1 }) }}</legend><div class="form-grid"><label><span>{{ t('repositoryCatalog.qualityKind') }}</span><select v-model="command.kind" :data-testid="commandIndex === 0 ? 'binding-quality-kind' : `binding-quality-kind-${commandIndex}`"><option value="build">build</option><option value="format">format</option><option value="lint">lint</option><option value="test">test</option></select></label><label><span>{{ t('repositoryCatalog.qualityName') }}</span><input v-model.trim="command.name" required :data-testid="commandIndex === 0 ? 'binding-quality-name' : `binding-quality-name-${commandIndex}`"></label><label><span>{{ t('repositoryCatalog.executable') }}</span><input v-model.trim="command.executable" required :data-testid="commandIndex === 0 ? 'binding-quality-executable' : `binding-quality-executable-${commandIndex}`"></label><label><span>{{ t('repositoryCatalog.timeout') }}</span><input v-model.number="command.timeoutSeconds" type="number" min="1" max="3600" required :data-testid="commandIndex === 0 ? 'binding-quality-timeout' : `binding-quality-timeout-${commandIndex}`"></label><div class="wide quality-arguments"><span>{{ t('repositoryCatalog.arguments') }}</span><label v-for="(_, argumentIndex) in command.arguments" :key="argumentIndex"><span>{{ t('repositoryCatalog.argument', { number: argumentIndex + 1 }) }}</span><input v-model="command.arguments[argumentIndex]" :data-testid="`binding-quality-argument-${commandIndex}-${argumentIndex}`"><button type="button" @click="removeQualityArgument(command, argumentIndex)">{{ t('repositoryCatalog.removeArgument') }}</button></label><button type="button" @click="addQualityArgument(command)">{{ t('repositoryCatalog.addArgument') }}</button></div><button type="button" :disabled="bindingForm.qualityCommands.length === 1" @click="removeQualityCommand(commandIndex)">{{ t('repositoryCatalog.removeQualityCommand') }}</button></div></fieldset><button class="wide" type="button" :disabled="bindingForm.qualityCommands.length >= 20" data-testid="add-quality-command" @click="addQualityCommand">{{ t('repositoryCatalog.addQualityCommand') }}</button><label><span>{{ t('repositoryCatalog.egress') }}</span><input value="public" disabled></label></div><footer><button type="button" @click="bindingModal = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-binding">{{ t(editing ? 'repositoryCatalog.save' : 'modelCatalog.register') }}</button></footer></form></div>
    <aside v-if="bindingModal" class="capability-drawer" aria-labelledby="required-capabilities-title"><strong id="required-capabilities-title">{{ t('repositoryCatalog.requiredCapabilities') }}</strong><label v-for="capability in runtimeCapabilities" :key="capability"><input type="checkbox" :checked="bindingForm.requiredRuntimeCapabilities.includes(capability)" :data-testid="`binding-capability-${capability}`" @change="toggleRequiredCapability(capability)"><span>{{ capability }}</span></label></aside>
  </section>
</template>
