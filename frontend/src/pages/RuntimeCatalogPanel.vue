<script setup lang="ts">
import { computed, inject, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ApiError, platformApiKey, type RuntimeImage } from "../api/client";
import { authContextKey } from "../auth/session";

const runtimes = ["claude", "codex", "hermes", "openclaw"] as const;
const capabilities = ["streaming", "structured_final", "native_resume", "subagents", "usage"] as const;
const statuses = ["experimental", "production", "blocked", "deprecated"] as const;
const injectedApi = inject(platformApiKey);
const auth = inject(authContextKey);
if (!injectedApi || !auth) throw new Error("Runtime Catalog dependencies are required");
const api = injectedApi;

const { t, d } = useI18n();
const items = ref<RuntimeImage[]>([]);
const selected = ref<RuntimeImage>();
const loading = ref(true);
const saving = ref(false);
const error = ref<ApiError>();
const notice = ref("");
const currentToken = ref("");
const nextToken = ref("");
const previousTokens = ref<string[]>([]);
const registerOpen = ref(false);
const registerIntent = ref("");
const registerIntentFingerprint = ref("");
const statusIntent = ref("");
const statusIntentFingerprint = ref("");
const registerForm = reactive({ runtime: "claude", cliVersion: "", adapterVersion: "", imageDigest: "", capabilities: new Set<string>() });
const statusForm = reactive({ status: "experimental", blockedReason: "", conformanceEvidenceKey: "" });
const failedNavigation = ref<{ token: string; remember: boolean; direction: "next" | "previous" | "reload" }>();
const currentUser = computed(() => auth.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser : undefined);
const canAdminister = computed(() => (currentUser.value?.role_grants ?? []).some((grant) => grant.role === "platform_administrator" && !grant.team_id));

onMounted(() => void loadPage("", false));

async function loadPage(token: string, remember: boolean, direction: "next" | "previous" | "reload" = "reload") {
  loading.value = true;
  error.value = undefined;
  try {
    const page = await api.listRuntimeImages(token, 12);
    if (remember) previousTokens.value.push(currentToken.value);
    if (direction === "previous") previousTokens.value.pop();
    currentToken.value = token;
    items.value = page.items;
    nextToken.value = page.nextPageToken;
    if (!selected.value || !page.items.some((item) => item.id === selected.value?.id)) selected.value = page.items[0];
    syncStatusForm();
    failedNavigation.value = undefined;
  } catch (reason) {
    error.value = asApiError(reason);
    failedNavigation.value = { token, remember, direction };
  } finally {
    loading.value = false;
  }
}

async function previousPage() {
  const token = previousTokens.value.at(-1);
  if (token !== undefined) await loadPage(token, false, "previous");
}

function selectImage(image: RuntimeImage) {
  selected.value = image;
  statusIntent.value = "";
  notice.value = "";
  syncStatusForm();
}

function syncStatusForm() {
  statusForm.status = selected.value?.status ?? "experimental";
  statusForm.blockedReason = selected.value?.blocked_reason ?? "";
  statusForm.conformanceEvidenceKey = selected.value?.conformance_evidence_key ?? "";
}

async function registerImage() {
  if (!canAdminister.value || saving.value) return;
  const input = {
    runtime: registerForm.runtime, cli_version: registerForm.cliVersion,
    adapter_version: registerForm.adapterVersion, image_digest: registerForm.imageDigest,
    capabilities: Object.fromEntries(capabilities.map((capability) => [capability, registerForm.capabilities.has(capability)])),
  };
  const fingerprint = JSON.stringify(input);
  if (!registerIntent.value || registerIntentFingerprint.value !== fingerprint) {
    registerIntent.value = crypto.randomUUID();
    registerIntentFingerprint.value = fingerprint;
  }
  saving.value = true;
  error.value = undefined;
  notice.value = "";
  try {
    const image = await api.registerRuntimeImage(input, registerIntent.value);
    registerIntent.value = "";
    registerIntentFingerprint.value = "";
    registerOpen.value = false;
    notice.value = t("runtimeCatalog.notice.registered");
    await loadPage("", false);
    selected.value = items.value.find((item) => item.id === image.id) ?? image;
    syncStatusForm();
  } catch (reason) {
    error.value = asApiError(reason);
  } finally {
    saving.value = false;
  }
}

async function changeStatus() {
  if (!canAdminister.value || !selected.value?.id || !selected.value.version || saving.value) return;
  const currentImage = selected.value;
  const input = {
    status: statusForm.status,
    blocked_reason: statusForm.status === "blocked" ? statusForm.blockedReason : undefined,
    conformance_evidence_key: statusForm.status === "production" ? statusForm.conformanceEvidenceKey : undefined,
  };
  const fingerprint = JSON.stringify(input);
  if (!statusIntent.value || statusIntentFingerprint.value !== fingerprint) {
    statusIntent.value = crypto.randomUUID();
    statusIntentFingerprint.value = fingerprint;
  }
  saving.value = true;
  error.value = undefined;
  notice.value = "";
  try {
    const updated = await api.changeRuntimeImageStatus(currentImage.id!, input, currentImage.version!, statusIntent.value);
    statusIntent.value = "";
    statusIntentFingerprint.value = "";
    selected.value = updated;
    items.value = items.value.map((item) => item.id === updated.id ? updated : item);
    notice.value = t("runtimeCatalog.notice.statusChanged");
    syncStatusForm();
  } catch (reason) {
    error.value = asApiError(reason);
    if (error.value.kind === "conflict") {
      statusIntent.value = "";
      statusIntentFingerprint.value = "";
      try {
        const selectedID = currentImage.id!;
        const refreshed = await api.getRuntimeImage(selectedID);
        selected.value = refreshed;
        items.value = items.value.map((item) => item.id === refreshed.id ? refreshed : item);
        syncStatusForm();
      } catch {
        // Preserve the conflict explanation if refreshing the record also fails.
      }
    }
  } finally {
    saving.value = false;
  }
}

function beginNewRegistration() {
  registerIntent.value = "";
  registerIntentFingerprint.value = "";
  error.value = undefined;
  notice.value = "";
  registerOpen.value = true;
}

function asApiError(reason: unknown): ApiError {
  return reason instanceof ApiError ? reason : new ApiError("unknown", 0, "unknown", "");
}

function errorLabel(value: ApiError) {
  if (value.kind === "forbidden") return t("errors.forbidden");
  if (value.kind === "conflict") return t("errors.conflict");
  if (value.kind === "validation") return t("errors.validation");
  return t("errors.server");
}
</script>

<template>
  <section class="catalog-shell" data-testid="runtime-catalog">
    <header class="catalog-header">
      <div><p class="kicker">{{ t('runtimeCatalog.kicker') }}</p><h2>{{ t('runtimeCatalog.title') }}</h2><p>{{ t('runtimeCatalog.body') }}</p></div>
      <button v-if="canAdminister" class="primary-action" data-testid="register-runtime" @click="beginNewRegistration">{{ t('runtimeCatalog.register') }}</button>
      <span v-else class="read-only-badge">{{ t('runtimeCatalog.readOnly') }}</span>
    </header>

    <p v-if="notice" class="catalog-notice" role="status" data-testid="runtime-notice">{{ notice }}</p>
    <div v-if="error" class="catalog-error" role="alert">
      <strong>{{ errorLabel(error) }}</strong><span>{{ t('runtimeCatalog.errorBody') }}</span>
      <small v-if="error.requestID">{{ t('runtimeCatalog.requestId', { id: error.requestID }) }}</small>
      <button v-if="failedNavigation" @click="loadPage(failedNavigation.token, failedNavigation.remember, failedNavigation.direction)">{{ t('runtimeCatalog.retry') }}</button>
    </div>

    <div v-if="loading" class="catalog-loading" aria-live="polite"><i></i><span>{{ t('runtimeCatalog.loading') }}</span></div>
    <div v-else-if="!error && items.length === 0" class="catalog-empty"><span>0×RI</span><div><h3>{{ t('runtimeCatalog.emptyTitle') }}</h3><p>{{ t('runtimeCatalog.emptyBody') }}</p></div></div>
    <div v-else-if="items.length" class="catalog-grid">
      <div class="catalog-list" :aria-label="t('runtimeCatalog.listLabel')">
        <button v-for="image in items" :key="image.id" :class="{ active: selected?.id === image.id }" :data-testid="`runtime-${image.id}`" @click="selectImage(image)">
          <span class="runtime-monogram">{{ image.runtime?.slice(0, 2).toUpperCase() }}</span>
          <span><strong>{{ image.runtime }}</strong><small>{{ image.cli_version }} · {{ image.adapter_version }}</small></span>
          <em :class="`state-${image.status}`">{{ t(`runtimeCatalog.status.${image.status}`) }}</em>
        </button>
        <nav class="catalog-pagination" :aria-label="t('runtimeCatalog.pagination')">
          <button :disabled="previousTokens.length === 0" @click="previousPage">{{ t('runtimeCatalog.previous') }}</button>
          <button :disabled="!nextToken" @click="loadPage(nextToken, true, 'next')">{{ t('runtimeCatalog.next') }}</button>
        </nav>
      </div>

      <article v-if="selected" class="runtime-detail" data-testid="runtime-detail">
        <div class="detail-title"><div><span>{{ t('runtimeCatalog.registered') }}</span><h3>{{ selected.runtime }}</h3></div><strong :class="`state-${selected.status}`">{{ t(`runtimeCatalog.status.${selected.status}`) }}</strong></div>
        <dl>
          <div><dt>{{ t('runtimeCatalog.cliVersion') }}</dt><dd>{{ selected.cli_version }}</dd></div>
          <div><dt>{{ t('runtimeCatalog.adapterVersion') }}</dt><dd>{{ selected.adapter_version }}</dd></div>
          <div class="digest"><dt>{{ t('runtimeCatalog.digest') }}</dt><dd>{{ selected.image_digest }}</dd></div>
          <div><dt>{{ t('runtimeCatalog.registeredAt') }}</dt><dd>{{ selected.created_at ? d(new Date(selected.created_at), 'long') : '—' }}</dd></div>
        </dl>
        <div class="capability-block"><h4>{{ t('runtimeCatalog.capabilities') }}</h4><p>{{ t('runtimeCatalog.capabilityCaveat') }}</p><ul><li v-for="(enabled, capability) in selected.capabilities" :key="capability" :class="{ enabled }"><span>{{ enabled ? '●' : '○' }}</span>{{ capability }}</li></ul></div>
        <div class="evidence-state" :class="{ verified: Boolean(selected.conformance_evidence_key) }">
          <strong>{{ t(selected.status === 'production' && selected.conformance_evidence_key ? 'runtimeCatalog.productionRuntime' : selected.conformance_evidence_key ? 'runtimeCatalog.evidenceRecorded' : 'runtimeCatalog.noEvidence') }}</strong>
          <p v-if="selected.conformance_evidence_key">{{ t('runtimeCatalog.evidenceKey') }} <code>{{ selected.conformance_evidence_key }}</code></p>
          <p v-if="selected.conformance_evidence_sha256">{{ t('runtimeCatalog.evidenceSHA256') }} <code>{{ selected.conformance_evidence_sha256 }}</code></p>
          <p v-else>{{ t('runtimeCatalog.noEvidenceBody') }}</p>
        </div>
        <div v-if="selected.status === 'blocked'" class="blocked-reason"><strong>{{ t('runtimeCatalog.blockedReason') }}</strong><p>{{ selected.blocked_reason }}</p></div>
        <form v-if="canAdminister && selected.status !== 'deprecated'" class="status-form" @submit.prevent="changeStatus">
          <label><span>{{ t('runtimeCatalog.changeStatus') }}</span><select v-model="statusForm.status" data-testid="runtime-status"><option v-for="status in statuses" :key="status" :value="status">{{ t(`runtimeCatalog.status.${status}`) }}</option></select></label>
          <label v-if="statusForm.status === 'blocked'"><span>{{ t('runtimeCatalog.blockedReason') }}</span><textarea v-model="statusForm.blockedReason" required maxlength="1000" data-testid="blocked-reason"></textarea></label>
          <label v-if="statusForm.status === 'production'"><span>{{ t('runtimeCatalog.evidenceKey') }}</span><input v-model.trim="statusForm.conformanceEvidenceKey" required maxlength="512" pattern="(?!/)(?!.*://)(?!.*\.\.).+" data-testid="conformance-evidence-key"></label>
          <button class="secondary-action" :disabled="saving || statusForm.status === selected.status && statusForm.blockedReason === (selected.blocked_reason ?? '') && statusForm.conformanceEvidenceKey === (selected.conformance_evidence_key ?? '')" data-testid="save-runtime-status">{{ saving ? t('runtimeCatalog.saving') : t('runtimeCatalog.saveStatus') }}</button>
        </form>
        <p v-else-if="selected.status === 'deprecated'" class="terminal-note">{{ t('runtimeCatalog.deprecatedFinal') }}</p>
      </article>
    </div>

    <div v-if="registerOpen" class="modal-backdrop" @click.self="registerOpen = false">
      <form class="catalog-modal" data-testid="register-runtime-form" @submit.prevent="registerImage">
        <header><span>NEW / RI</span><button type="button" :aria-label="t('runtimeCatalog.close')" @click="registerOpen = false">×</button></header>
        <h3>{{ t('runtimeCatalog.registerTitle') }}</h3><p>{{ t('runtimeCatalog.immutableHint') }}</p>
        <div class="form-grid">
          <label><span>{{ t('runtimeCatalog.runtime') }}</span><select v-model="registerForm.runtime" data-testid="runtime-kind"><option v-for="runtime in runtimes" :key="runtime">{{ runtime }}</option></select></label>
          <label><span>{{ t('runtimeCatalog.cliVersion') }}</span><input v-model.trim="registerForm.cliVersion" required maxlength="100" data-testid="runtime-cli-version"></label>
          <label><span>{{ t('runtimeCatalog.adapterVersion') }}</span><input v-model.trim="registerForm.adapterVersion" required maxlength="100" data-testid="runtime-adapter-version"></label>
          <label class="wide"><span>{{ t('runtimeCatalog.digest') }}</span><input v-model.trim="registerForm.imageDigest" required pattern="[^\s@]+@sha256:[a-f0-9]{64}" placeholder="registry.example/runtime@sha256:…" data-testid="runtime-digest"></label>
        </div>
        <fieldset><legend>{{ t('runtimeCatalog.declaredCapabilities') }}</legend><label v-for="capability in capabilities" :key="capability"><input type="checkbox" :checked="registerForm.capabilities.has(capability)" @change="($event.target as HTMLInputElement).checked ? registerForm.capabilities.add(capability) : registerForm.capabilities.delete(capability)">{{ capability }}</label></fieldset>
        <div class="modal-actions"><button type="button" @click="registerOpen = false">{{ t('runtimeCatalog.cancel') }}</button><button class="primary-action" :disabled="saving" data-testid="submit-runtime">{{ saving ? t('runtimeCatalog.saving') : t('runtimeCatalog.register') }}</button></div>
      </form>
    </div>
  </section>
</template>
