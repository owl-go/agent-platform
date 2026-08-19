<script setup lang="ts">
import { computed, inject, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type ConfiguredModel, type CredentialProfile } from "../api/client";
import { authContextKey } from "../auth/session";

const injectedApi = inject(platformApiKey);
const auth = inject(authContextKey);
if (!injectedApi || !auth) throw new Error("Model Catalog dependencies are required");
const api = injectedApi;
const { t } = useI18n();

const credentials = ref<CredentialProfile[]>([]);
const models = ref<ConfiguredModel[]>([]);
const loading = ref(true);
const saving = ref(false);
const error = ref<ApiError>();
const notice = ref("");
const credentialModal = ref(false);
const modelModal = ref(false);
const credentialForm = reactive({ name: "", secretRef: "" });
const modelForm = reactive({ name: "", modelID: "", endpoint: "", credentialProfileID: "" });
const intents = new Map<string, { fingerprint: string; key: string }>();
const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const canAdminister = computed(() => (currentUser.value?.role_grants ?? []).some((grant) => grant.role === "platform_administrator" && !grant.team_id));
const eligibleCredentials = computed(() => credentials.value.filter((profile) => profile.kind === "model" && profile.enabled && !profile.team_id));

onMounted(() => void refresh());

async function refresh() {
  loading.value = true;
  error.value = undefined;
  try {
    [credentials.value, models.value] = await Promise.all([api.listCredentialProfiles(), api.listConfiguredModels()]);
  } catch (reason) {
    error.value = asApiError(reason);
  } finally {
    loading.value = false;
  }
}

function intent(scope: string, input: unknown) {
  const fingerprint = JSON.stringify(input);
  const current = intents.get(scope);
  if (current?.fingerprint === fingerprint) return current.key;
  const key = crypto.randomUUID();
  intents.set(scope, { fingerprint, key });
  return key;
}

async function registerCredential() {
  if (!canAdminister.value || saving.value) return;
  const input = { name: credentialForm.name, kind: "model", secret_ref: credentialForm.secretRef };
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.registerCredentialProfile(input, intent("credential.register", input));
    intents.delete("credential.register");
    credentialForm.name = ""; credentialForm.secretRef = ""; credentialModal.value = false;
    notice.value = t("modelCatalog.notice.credentialRegistered");
    await refresh();
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function registerModel() {
  if (!canAdminister.value || saving.value) return;
  const input = { name: modelForm.name, model_id: modelForm.modelID, endpoint: modelForm.endpoint, credential_profile_id: modelForm.credentialProfileID };
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.registerConfiguredModel(input, intent("model.register", input));
    intents.delete("model.register");
    Object.assign(modelForm, { name: "", modelID: "", endpoint: "", credentialProfileID: "" });
    modelModal.value = false; notice.value = t("modelCatalog.notice.modelRegistered");
    await refresh();
  } catch (reason) { error.value = asApiError(reason); }
  finally { saving.value = false; }
}

async function toggleCredential(profile: CredentialProfile) {
  if (!canAdminister.value || !profile.id || !profile.version || saving.value) return;
  const enabled = !profile.enabled;
  const scope = `credential.status:${profile.id}`;
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.changeCredentialProfileStatus(profile.id, enabled, profile.version, intent(scope, { enabled }));
    intents.delete(scope); notice.value = t("modelCatalog.notice.credentialChanged"); await refresh();
  } catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict") await refresh().catch(() => undefined);
  } finally { saving.value = false; }
}

async function toggleModel(model: ConfiguredModel) {
  if (!canAdminister.value || !model.id || !model.version || saving.value) return;
  const enabled = !model.enabled;
  const scope = `model.status:${model.id}`;
  saving.value = true; error.value = undefined; notice.value = "";
  try {
    await api.changeConfiguredModelStatus(model.id, enabled, model.version, intent(scope, { enabled }));
    intents.delete(scope); notice.value = t("modelCatalog.notice.modelChanged"); await refresh();
  } catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict") await refresh().catch(() => undefined);
  } finally { saving.value = false; }
}

function credentialName(id?: string) {
  return credentials.value.find((profile) => profile.id === id)?.name ?? t("modelCatalog.missingCredential");
}
function asApiError(reason: unknown) { return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "unknown", ""); }
function errorLabel(value: ApiError) {
  if (value.kind === "forbidden") return t("errors.forbidden");
  if (value.kind === "conflict") return t("errors.conflict");
  if (value.kind === "validation") return t("errors.validation");
  return t("errors.server");
}
</script>

<template>
  <section class="model-catalog" data-testid="model-catalog">
    <header class="catalog-header model-catalog-header">
      <div><p class="kicker">{{ t('modelCatalog.kicker') }}</p><h2>{{ t('modelCatalog.title') }}</h2><p>{{ t('modelCatalog.body') }}</p></div>
      <span v-if="!canAdminister" class="read-only-badge">{{ t('modelCatalog.readOnly') }}</span>
    </header>
    <p class="policy-note"><strong>{{ t('modelCatalog.policyTitle') }}</strong> {{ t('modelCatalog.policyBody') }}</p>
    <p v-if="notice" class="catalog-notice" role="status" data-testid="model-notice">{{ notice }}</p>
    <div v-if="error" class="catalog-error" role="alert"><strong>{{ errorLabel(error) }}</strong><span>{{ t('modelCatalog.errorBody') }}</span><small v-if="error.requestID">{{ t('runtimeCatalog.requestId', { id: error.requestID }) }}</small><button @click="refresh">{{ t('runtimeCatalog.retry') }}</button></div>
    <div v-if="loading" class="catalog-loading"><i></i><span>{{ t('modelCatalog.loading') }}</span></div>
    <div v-else class="model-catalog-grid">
      <section class="catalog-column" data-testid="credential-catalog">
        <header><div><span>{{ t('modelCatalog.credentialSection') }}</span><h3>{{ t('modelCatalog.credentials') }}</h3></div><button v-if="canAdminister" class="secondary-action" data-testid="register-credential" @click="credentialModal = true">{{ t('modelCatalog.addCredential') }}</button></header>
        <div v-if="credentials.length === 0" class="mini-empty"><strong>{{ t('modelCatalog.noCredentials') }}</strong><p>{{ t('modelCatalog.noCredentialsBody') }}</p></div>
        <article v-for="profile in credentials" :key="profile.id" class="catalog-record" :data-testid="`credential-${profile.id}`">
          <div class="record-heading"><div><span>{{ profile.kind }}</span><h4>{{ profile.name }}</h4></div><em :class="profile.enabled ? 'state-production' : 'state-blocked'">{{ t(profile.enabled ? 'modelCatalog.enabled' : 'modelCatalog.disabled') }}</em></div>
          <dl><div><dt>{{ t('modelCatalog.scope') }}</dt><dd>{{ profile.team_id ? t('modelCatalog.teamScope') : t('modelCatalog.organizationScope') }}</dd></div><div><dt>{{ t('modelCatalog.secret') }}</dt><dd>{{ profile.secret_configured ? t('modelCatalog.secretConfigured') : t('modelCatalog.secretMissing') }}</dd></div></dl>
          <button v-if="canAdminister" class="record-action" :disabled="saving" :data-testid="`toggle-credential-${profile.id}`" @click="toggleCredential(profile)">{{ t(profile.enabled ? 'modelCatalog.disable' : 'modelCatalog.enable') }}</button>
        </article>
      </section>
      <section class="catalog-column" data-testid="configured-model-catalog">
        <header><div><span>{{ t('modelCatalog.modelSection') }}</span><h3>{{ t('modelCatalog.models') }}</h3></div><button v-if="canAdminister" class="secondary-action" :disabled="eligibleCredentials.length === 0" data-testid="register-model" @click="modelModal = true">{{ t('modelCatalog.addModel') }}</button></header>
        <div v-if="models.length === 0" class="mini-empty"><strong>{{ t('modelCatalog.noModels') }}</strong><p>{{ t('modelCatalog.noModelsBody') }}</p></div>
        <article v-for="model in models" :key="model.id" class="catalog-record" :data-testid="`model-${model.id}`">
          <div class="record-heading"><div><span>{{ model.model_id }}</span><h4>{{ model.name }}</h4></div><em :class="model.enabled ? 'state-production' : 'state-blocked'">{{ t(model.enabled ? 'modelCatalog.enabled' : 'modelCatalog.disabled') }}</em></div>
          <dl><div class="wide"><dt>{{ t('modelCatalog.endpoint') }}</dt><dd><code>{{ model.endpoint }}</code></dd></div><div><dt>{{ t('modelCatalog.credential') }}</dt><dd>{{ credentialName(model.credential_profile_id) }}</dd></div></dl>
          <button v-if="canAdminister" class="record-action" :disabled="saving" :data-testid="`toggle-model-${model.id}`" @click="toggleModel(model)">{{ t(model.enabled ? 'modelCatalog.disable' : 'modelCatalog.enable') }}</button>
        </article>
      </section>
    </div>

    <div v-if="credentialModal" class="modal-backdrop" @click.self="credentialModal = false"><form class="catalog-modal compact-modal" data-testid="credential-form" @submit.prevent="registerCredential"><header><span>{{ t('modelCatalog.newCredential') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="credentialModal = false">×</button></header><h3>{{ t('modelCatalog.addCredential') }}</h3><p>{{ t('modelCatalog.credentialHint') }}</p><div class="form-grid"><label><span>{{ t('modelCatalog.name') }}</span><input v-model.trim="credentialForm.name" required maxlength="100" data-testid="credential-name"></label><label class="wide"><span>{{ t('modelCatalog.secretRef') }}</span><input v-model.trim="credentialForm.secretRef" required maxlength="500" pattern="[a-z][a-z0-9+.-]*://[^\s]+" autocomplete="off" data-testid="credential-secret-ref"></label></div><footer><button type="button" @click="credentialModal = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-credential">{{ t('modelCatalog.register') }}</button></footer></form></div>
    <div v-if="modelModal" class="modal-backdrop" @click.self="modelModal = false"><form class="catalog-modal compact-modal" data-testid="model-form" @submit.prevent="registerModel"><header><span>{{ t('modelCatalog.newModel') }}</span><button type="button" :aria-label="t('modelCatalog.close')" @click="modelModal = false">×</button></header><h3>{{ t('modelCatalog.addModel') }}</h3><div class="form-grid"><label><span>{{ t('modelCatalog.name') }}</span><input v-model.trim="modelForm.name" required maxlength="100" data-testid="model-name"></label><label><span>{{ t('modelCatalog.modelID') }}</span><input v-model.trim="modelForm.modelID" required maxlength="200" data-testid="model-id"></label><label class="wide"><span>{{ t('modelCatalog.endpoint') }}</span><input v-model.trim="modelForm.endpoint" type="url" required pattern="https://.*" data-testid="model-endpoint"></label><label class="wide"><span>{{ t('modelCatalog.credential') }}</span><select v-model="modelForm.credentialProfileID" required data-testid="model-credential"><option value="" disabled>{{ t('modelCatalog.chooseCredential') }}</option><option v-for="profile in eligibleCredentials" :key="profile.id" :value="profile.id">{{ profile.name }}</option></select></label></div><footer><button type="button" @click="modelModal = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-model">{{ t('modelCatalog.register') }}</button></footer></form></div>
  </section>
</template>
