<script setup lang="ts">
import { saveErrorMessage } from "../api/saveErrors";
import { computed, inject, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { ArrowLeft, ArrowRight, Download, MagicStick, Picture, Plus, Setting, VideoPause } from "@element-plus/icons-vue";
import { platformApiKey, type ImageGenerationInput, type ImageGenerationRecord, type ImageModel, type PromptOptimizationCandidate, type ReferenceImageUpload } from "../api/client";
import { authContextKey } from "../auth/session";

const api = inject(platformApiKey)!;
const auth = inject(authContextKey)!;
const { t } = useI18n();
const models = ref<ImageModel[]>([]);
const history = ref<ImageGenerationRecord[]>([]);
const active = ref<ImageGenerationRecord>();
const imageURLs = ref<Record<number, string>>({});
const imageURLCache = new Map<string, Record<number, string>>();
const imageLoads = new Map<string, Promise<string>>();
const discardedImageRecords = new Set<string>();
const maxCachedImageRecords = 6;
const loading = ref(true);
const submitting = ref(false);
const error = ref("");
const adminOpen = ref(false);
const adminModels = ref<ImageModel[]>([]);
const editingModel = ref<ImageModel>();
const references = ref<Array<ReferenceImageUpload & { name: string }>>([]);
const promptModels = ref<PromptOptimizationCandidate[]>([]);
const promptModelID = ref("");
const previousPrompt = ref("");
const originalPrompt = ref("");
const customSize = ref("");
const customSizeMode = ref(false);
const optimizing = ref(false);
const previewPosition = ref<number>();
let streamAbort: AbortController | undefined;
let streamReconnect: number | undefined;
let lastEventID = 0;
let submitRequestID = "";
let regenerationRequestID = "";
let disposed = false;

const form = reactive<ImageGenerationInput>({ image_model_id: "", mode: "generate", prompt: "", size: "", quality: "", format: "png", background: "opaque", count: 1 });
const selectedModel = computed(() => models.value.find((model) => model.id === form.image_model_id));
const locked = computed(() => active.value && ["pending", "running"].includes(active.value.state));
const validSize = computed(() => isValidImageSize(form.size));
const estimate = computed(() => imageRate(selectedModel.value, form.size, form.quality) ?? 0);
const administrator = computed(() => auth.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const preferenceKey = computed(() => auth.session.state.value.kind === "authenticated" ? `image-generation-options:${auth.session.state.value.currentUser.id}` : "");

const adminForm = reactive({ endpoint: "", api_key: "", provider_model_id: "gpt-image-1" });
const promptAdminForm = reactive({ provider_model_id: "", endpoint: "", api_key: "", instruction: "请将用户输入扩展为清晰、具体、适合图片生成模型理解的提示词，保留原始意图，只返回优化后的提示词。" });

function normalizeRecord(record: ImageGenerationRecord): ImageGenerationRecord {
  return {
    ...record,
    images: record.images ?? [],
    requested_count: record.requested_count ?? 0,
    validated_count: record.validated_count ?? 0,
    reservation_hundredths: record.reservation_hundredths ?? 0,
    consumption_hundredths: record.consumption_hundredths ?? 0,
    version: record.version ?? 0,
  };
}

watch(selectedModel, (model) => {
  if (!model) return;
  if (!model.modes.includes(form.mode)) form.mode = model.modes[0] ?? "generate";
  if (!model.sizes.includes(form.size) && !isValidImageSize(form.size)) form.size = model.default_size;
  customSizeMode.value = !model.sizes.includes(form.size);
  if (customSizeMode.value) customSize.value = form.size;
  if (!model.qualities.includes(form.quality)) form.quality = model.default_quality;
  if (!model.formats.includes(form.format)) form.format = model.default_format;
  if (!model.backgrounds.includes(form.background)) form.background = model.default_background;
});
watch(() => form.size, (size) => {
  const model = selectedModel.value;
  if (model && size && !model.sizes.includes(size)) {
    customSizeMode.value = true;
    customSize.value = size;
  }
});
watch(customSize, (size) => { if (customSizeMode.value) form.size = normalizeImageSize(size); });
watch(() => ({ image_model_id: form.image_model_id, mode: form.mode, size: form.size, quality: form.quality, format: form.format, background: form.background, count: form.count }), (options) => {
  if (preferenceKey.value) localStorage.setItem(preferenceKey.value, JSON.stringify(options));
}, { deep: true });

async function load() {
  loading.value = true;
  try {
    const [options, records] = await Promise.all([api.getImageGenerationOptions(), api.listImageGenerations()]);
    models.value = options.image_models ?? []; promptModels.value = options.prompt_optimization_models ?? []; history.value = (records ?? []).map(normalizeRecord);
    active.value = history.value.find((record) => ["pending", "running"].includes(record.state)) ?? history.value[0];
    if (!form.image_model_id && preferenceKey.value) {
      try { Object.assign(form, JSON.parse(localStorage.getItem(preferenceKey.value) ?? "{}")); } catch { /* Ignore obsolete local preferences. */ }
    }
    if (!models.value.some((model) => model.id === form.image_model_id) && models.value[0]) form.image_model_id = models.value[0].id;
    if (!promptModelID.value && promptModels.value[0]) promptModelID.value = promptModels.value[0].provider_model_id;
    await loadImages(active.value);
    startStream();
  } catch { error.value = t("imageGeneration.requestFailed"); }
  finally { loading.value = false; }
}

async function optimizePrompt() {
  if (!promptModelID.value || !form.prompt.trim() || locked.value) return;
  optimizing.value = true; error.value = "";
  try { const original = form.prompt; const result = await api.optimizeImagePrompt(promptModelID.value, original, document.documentElement.lang); originalPrompt.value ||= original; previousPrompt.value = original; form.prompt = result.prompt; window.dispatchEvent(new Event("credits-updated")); }
  catch { error.value = t("imageGeneration.requestFailed"); }
  finally { optimizing.value = false; }
}

function undoOptimization() { if (previousPrompt.value) { form.prompt = previousPrompt.value; previousPrompt.value = ""; originalPrompt.value = ""; } }

async function refreshActive(recordID: string) {
  if (active.value?.id !== recordID) return;
  active.value = normalizeRecord(await api.getImageGeneration(recordID));
  const index = history.value.findIndex((item) => item.id === recordID);
  if (index >= 0) history.value[index] = active.value;
  await loadImages(active.value);
}

function startStream() {
  if (streamReconnect !== undefined) window.clearTimeout(streamReconnect);
  streamReconnect = undefined;
  streamAbort?.abort();
  if (!locked.value || !active.value) return;
  const recordID = active.value.id;
  const controller = new AbortController();
  streamAbort = controller;
  void api.streamImageGeneration(recordID, (event) => {
    lastEventID = event.sequence;
    void refreshActive(recordID);
  }, controller.signal, lastEventID).then(async () => {
    await refreshActive(recordID);
    if (!controller.signal.aborted && active.value?.id === recordID && locked.value) streamReconnect = window.setTimeout(startStream, 1500);
  }).catch(() => {
    if (!controller.signal.aborted && active.value?.id === recordID) streamReconnect = window.setTimeout(startStream, 1500);
  });
}

async function submit() {
  if (locked.value || !form.prompt.trim() || !validSize.value) return;
  submitting.value = true; error.value = "";
  try {
    submitRequestID ||= crypto.randomUUID();
    const submittedReferences = [...references.value];
    active.value = normalizeRecord(await api.submitImageGeneration({ ...form, request_id: submitRequestID, original_prompt: originalPrompt.value || form.prompt, reference_upload_ids: submittedReferences.map((item) => item.id) }));
    submitRequestID = "";
    history.value.unshift(active.value);
    form.prompt = ""; previousPrompt.value = ""; originalPrompt.value = ""; references.value = [];
    for (const reference of submittedReferences) void api.deleteReferenceImage(reference.id).catch(() => undefined);
    window.dispatchEvent(new Event("credits-updated"));
    lastEventID = 0; startStream();
  } catch { error.value = t("imageGeneration.requestFailed"); }
  finally { submitting.value = false; }
}

async function uploadReferences(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = [...(input.files ?? [])].slice(0, 10 - references.value.length);
  for (const file of files) {
    try { references.value.push({ ...(await api.uploadReferenceImage(file)), name: file.name }); }
    catch (cause) { error.value = saveErrorMessage(cause, t, "imageGeneration.requestFailed"); }
  }
  input.value = "";
}

async function removeReference(index: number) {
  const [reference] = references.value.splice(index, 1);
  if (reference) await api.deleteReferenceImage(reference.id).catch(() => undefined);
}

function moveReference(index: number, offset: number) {
  const target = index + offset;
  if (target < 0 || target >= references.value.length) return;
  [references.value[index], references.value[target]] = [references.value[target], references.value[index]];
}

async function stop() {
  if (!active.value) return;
  try { active.value = normalizeRecord(await api.stopImageGeneration(active.value.id)); window.dispatchEvent(new Event("credits-updated")); }
  catch { error.value = t("imageGeneration.requestFailed"); }
}

async function regenerate() {
  if (!active.value || locked.value) return;
  const currentModel = models.value.find((model) => model.id === active.value?.image_model_id);
  const currentRate = imageRate(currentModel, active.value.size, active.value.quality);
  if (currentRate === undefined || !window.confirm(t("imageGeneration.regenerateEstimate", { value: ((currentRate * active.value.requested_count) / 100).toFixed(2) }))) return;
  try { regenerationRequestID ||= crypto.randomUUID(); active.value = normalizeRecord(await api.regenerateImageGeneration(active.value.id, regenerationRequestID)); regenerationRequestID = ""; history.value.unshift(active.value); window.dispatchEvent(new Event("credits-updated")); lastEventID = 0; startStream(); }
  catch { error.value = t("imageGeneration.requestFailed"); }
}

function reuseSettings() {
  if (!active.value || locked.value) return;
  const model = models.value.find((item) => item.id === active.value?.image_model_id);
  if (!model) return;
  form.image_model_id = model.id; form.mode = active.value.mode; form.prompt = active.value.prompt;
  form.size = isValidImageSize(active.value.size) ? active.value.size : model.default_size;
  customSizeMode.value = !model.sizes.includes(form.size);
  customSize.value = customSizeMode.value ? form.size : "";
  form.quality = model.qualities.includes(active.value.quality) ? active.value.quality : model.default_quality;
  form.format = model.formats.includes(active.value.format) ? active.value.format : model.default_format;
  form.background = model.backgrounds.includes(active.value.background) ? active.value.background : model.default_background;
  form.count = active.value.requested_count;
}

function selectSize(value: string) {
  if (value === "__custom__") {
    customSizeMode.value = true;
    form.size = normalizeImageSize(customSize.value);
    return;
  }
  customSizeMode.value = false;
  form.size = value;
}

function normalizeImageSize(size: string): string {
  return size.trim().toLowerCase().replaceAll("×", "x").replace(/\s+/g, "");
}

function isValidImageSize(size: string): boolean {
  const match = /^([1-9]\d*)x([1-9]\d*)$/.exec(size);
  if (!match) return false;
  const width = Number(match[1]);
  const height = Number(match[2]);
  return Number.isSafeInteger(width) && Number.isSafeInteger(height) && width <= 64_000_000 && height <= 64_000_000 && width * height <= 64_000_000;
}

function imageRate(model: ImageModel | undefined, size: string, quality: string): number | undefined {
  if (!model) return undefined;
  return model.rates.find((rate) => rate.size === size && rate.quality === quality)?.amount_hundredths
    ?? model.rates.find((rate) => rate.size === model.default_size && rate.quality === quality)?.amount_hundredths
    ?? undefined;
}

async function deleteRecord() {
  if (!active.value || locked.value || !window.confirm(`${t('common.delete')}?`)) return;
  const id = active.value.id;
  try { await api.deleteImageGeneration(id); discardedImageRecords.add(id); revokeCachedImages(id); history.value = history.value.filter((item) => item.id !== id); active.value = history.value[0]; await loadImages(active.value); }
  catch { error.value = t("imageGeneration.requestFailed"); }
}

async function downloadAll() {
  if (!active.value) return;
  const url = URL.createObjectURL(await api.getGeneratedImagesZIP(active.value.id));
  const link = document.createElement("a"); link.href = url; link.download = `${active.value.id}.zip`; link.click(); setTimeout(() => URL.revokeObjectURL(url), 0);
}

async function selectRecord(record: ImageGenerationRecord) {
  streamAbort?.abort();
  if (streamReconnect !== undefined) window.clearTimeout(streamReconnect);
  streamReconnect = undefined; lastEventID = 0;
  active.value = record;
  await loadImages(record);
  startStream();
}

async function loadImages(record?: ImageGenerationRecord) {
  if (!record) { imageURLs.value = {}; return; }
  imageURLs.value = touchCachedImages(record.id);
  await Promise.all((record.images ?? []).map(async (image) => {
    if (imageURLCache.get(record.id)?.[image.position]) return;
    const key = `${record.id}:${image.position}`;
    let pending = imageLoads.get(key);
    if (!pending) {
      pending = api.getGeneratedImage(record.id, image.position).then((blob) => URL.createObjectURL(blob));
      imageLoads.set(key, pending);
    }
    try {
      const url = await pending;
      if (disposed || discardedImageRecords.has(record.id)) { URL.revokeObjectURL(url); return; }
      const urls = { ...(imageURLCache.get(record.id) ?? {}), [image.position]: url };
      cacheImages(record.id, urls);
      if (active.value?.id === record.id) imageURLs.value = urls;
    } catch { /* Keep other valid results visible. */ }
    finally { if (imageLoads.get(key) === pending) imageLoads.delete(key); }
  }));
}

function touchCachedImages(recordID: string): Record<number, string> {
  const urls = imageURLCache.get(recordID) ?? {};
  if (imageURLCache.has(recordID)) {
    imageURLCache.delete(recordID);
    imageURLCache.set(recordID, urls);
  }
  return urls;
}

function cacheImages(recordID: string, urls: Record<number, string>) {
  imageURLCache.delete(recordID);
  imageURLCache.set(recordID, urls);
  while (imageURLCache.size > maxCachedImageRecords) {
    const oldest = imageURLCache.keys().next().value as string | undefined;
    if (!oldest) break;
    if (oldest === active.value?.id) {
      const current = imageURLCache.get(oldest)!;
      imageURLCache.delete(oldest);
      imageURLCache.set(oldest, current);
      continue;
    }
    revokeCachedImages(oldest);
  }
}

function revokeCachedImages(recordID: string) {
  for (const url of Object.values(imageURLCache.get(recordID) ?? {})) URL.revokeObjectURL(url);
  imageURLCache.delete(recordID);
}

function downloadImage(position: number) {
  const url = imageURLs.value[position];
  if (!url || !active.value) return;
  const link = document.createElement("a"); link.href = url; link.download = `${active.value.id}-${position}.${active.value.format}`; link.click();
}

async function reuseImageAsReference(position: number) {
  if (!active.value || locked.value || references.value.length >= 10) return;
  const editableModel = selectedModel.value?.modes.includes("edit") ? selectedModel.value : models.value.find((model) => model.modes.includes("edit"));
  if (!editableModel) { error.value = t("imageGeneration.noEditModel"); return; }
  try {
    const blob = await api.getGeneratedImage(active.value.id, position);
    const file = new File([blob], `generated-${position}.${active.value.format}`, { type: blob.type });
    references.value.push({ ...(await api.uploadReferenceImage(file)), name: file.name });
    form.image_model_id = editableModel.id;
    form.mode = "edit";
  } catch (cause) { error.value = saveErrorMessage(cause, t, "imageGeneration.requestFailed"); }
}

function movePreview(offset: number) {
  if (!active.value?.images.length || previewPosition.value === undefined) return;
  const positions = active.value.images.map((image) => image.position);
  const index = positions.indexOf(previewPosition.value);
  previewPosition.value = positions[(index + offset + positions.length) % positions.length];
}

async function openAdmin() {
  adminOpen.value = true;
  const [imageModels, promptSettings] = await Promise.all([api.listImageModels(), api.listPromptOptimizationCandidates()]);
  adminModels.value = imageModels;
  const configured = promptSettings[0];
  if (configured) Object.assign(promptAdminForm, { provider_model_id: configured.provider_model_id, endpoint: configured.endpoint ?? "", api_key: "", instruction: configured.instruction });
}

async function createModel() {
  try {
    const input = { endpoint: adminForm.endpoint, provider_model_id: adminForm.provider_model_id };
    const model = editingModel.value
      ? await api.reviseImageModel(editingModel.value.id, editingModel.value.version, { ...input, ...(adminForm.api_key ? { replacement_api_key: adminForm.api_key } : {}) })
      : await api.createImageModel({ ...input, api_key: adminForm.api_key });
    adminModels.value = [model, ...adminModels.value.filter((item) => item.id !== model.id)]; editingModel.value = undefined; adminForm.api_key = "";
  } catch (cause) { error.value = saveErrorMessage(cause, t, "imageGeneration.requestFailed"); }
}

async function verify(model: ImageModel) { error.value = ""; try { Object.assign(model, await api.verifyImageModel(model.id)); } catch { error.value = t("imageGeneration.verifyFailed"); } }
async function toggle(model: ImageModel) { error.value = ""; try { Object.assign(model, await api.setImageModelAvailability(model.id, model.state !== "available")); await load(); } catch (cause) { error.value = saveErrorMessage(cause, t, "imageGeneration.requestFailed"); } }
function editModel(model: ImageModel) { editingModel.value = model; Object.assign(adminForm, { endpoint: model.endpoint ?? "", api_key: "", provider_model_id: model.provider_model_id }); }
async function deleteModel(model: ImageModel) { if (!window.confirm(`${t('common.delete')}?`)) return; await api.deleteImageModel(model.id, model.version); adminModels.value = adminModels.value.filter((item) => item.id !== model.id); await load(); }
async function savePromptCandidates() {
  error.value = "";
  try {
    promptModels.value = await api.replacePromptOptimizationCandidates([{ ...promptAdminForm }]);
    promptAdminForm.api_key = "";
    promptModelID.value = promptModels.value[0]?.provider_model_id ?? "";
  } catch (cause) { error.value = saveErrorMessage(cause, t, "imageGeneration.requestFailed"); }
}

onMounted(load);
onBeforeUnmount(() => {
  disposed = true;
  streamAbort?.abort();
  if (streamReconnect !== undefined) window.clearTimeout(streamReconnect);
  for (const reference of references.value) void api.deleteReferenceImage(reference.id).catch(() => undefined);
  for (const recordID of [...imageURLCache.keys()]) revokeCachedImages(recordID);
});
</script>

<template>
  <main class="image-generation-page">
    <header class="image-generation-head">
      <div><p class="eyebrow">{{ t('nav.ai-applications') }}</p><h1>{{ t('imageGeneration.title') }}</h1></div>
      <el-button v-if="administrator" :icon="Setting" @click="openAdmin">{{ t('imageGeneration.configure') }}</el-button>
    </header>
    <div v-if="error" class="image-generation-error" role="alert">{{ error }}</div>
    <section v-loading="loading" class="image-workbench">
      <aside class="image-settings-card">
        <div class="image-card-title"><el-icon><MagicStick /></el-icon><h2>{{ t('imageGeneration.settings') }}</h2><el-button v-if="administrator" class="image-admin-trigger" :icon="Setting" @click="openAdmin">{{ t('imageGeneration.configure') }}</el-button></div>
        <el-segmented v-model="form.mode" :options="[{ label: t('imageGeneration.textToImage'), value: 'generate' }, { label: t('imageGeneration.imageToImage'), value: 'edit', disabled: !selectedModel?.modes.includes('edit') }]" :disabled="locked" />
        <div v-if="models.length === 0" class="image-no-model"><el-icon><Picture /></el-icon><p>{{ t('imageGeneration.noModel') }}</p><el-button v-if="administrator" text @click="openAdmin">{{ t('imageGeneration.configure') }}</el-button></div>
        <el-form v-else label-position="top">
          <el-form-item :label="t('imageGeneration.model')"><el-select v-model="form.image_model_id" :disabled="locked"><el-option v-for="model in models" :key="model.id" :label="model.display_name" :value="model.id" /></el-select></el-form-item>
          <div v-if="form.mode === 'edit'" class="reference-uploader"><label><input type="file" accept="image/png,image/jpeg,image/webp" multiple :disabled="locked || references.length >= 10" @change="uploadReferences"><span>{{ t('common.upload') }} Reference Images</span></label><div v-for="(reference,index) in references" :key="reference.id"><span>{{ index + 1 }} · {{ reference.name }} · {{ reference.width }}×{{ reference.height }}</span><span class="reference-actions"><button type="button" :disabled="locked || index === 0" aria-label="Move up" @click="moveReference(index,-1)">↑</button><button type="button" :disabled="locked || index === references.length - 1" aria-label="Move down" @click="moveReference(index,1)">↓</button><button type="button" :disabled="locked" aria-label="Remove" @click="removeReference(index)">×</button></span></div><small>PNG / JPEG / WebP · 20 MiB · metadata is sent to the selected provider</small></div>
          <el-form-item :label="t('imageGeneration.prompt')"><template #label><span class="prompt-label"><span>{{ t('imageGeneration.prompt') }}</span><span><el-button v-if="previousPrompt" text size="small" :disabled="locked" @click="undoOptimization">Undo</el-button><el-button text size="small" :loading="optimizing" :disabled="locked || !promptModelID || !form.prompt.trim()" @click="optimizePrompt">{{ t('imageGeneration.optimize') }}</el-button></span></span></template><el-input v-model="form.prompt" type="textarea" :rows="7" maxlength="10000" show-word-limit :placeholder="t('imageGeneration.promptPlaceholder')" :disabled="locked" /></el-form-item>
          <p v-if="promptModels.length" class="image-optimization-model">{{ t('imageGeneration.optimizationModel') }} · {{ promptModels[0]?.display_name }}</p>
          <div class="image-option-grid">
            <el-form-item :label="t('imageGeneration.size')"><el-select :model-value="customSizeMode ? '__custom__' : form.size" :disabled="locked" @update:model-value="selectSize"><el-option v-for="value in selectedModel?.sizes" :key="value" :label="value" :value="value" /><el-option :label="t('imageGeneration.customSize')" value="__custom__" /></el-select><el-input v-if="customSizeMode" v-model="customSize" class="custom-image-size" :disabled="locked" :placeholder="t('imageGeneration.customSizePlaceholder')" /><small v-if="customSizeMode" :class="{ 'image-size-invalid': customSize && !validSize }">{{ customSize && !validSize ? t('imageGeneration.customSizeInvalid') : t('imageGeneration.customSizeHelp') }}</small></el-form-item>
            <el-form-item :label="t('imageGeneration.count')"><el-input-number v-model="form.count" :min="1" :max="4" :disabled="locked" /></el-form-item>
            <el-form-item :label="t('imageGeneration.quality')"><el-select v-model="form.quality" :disabled="locked"><el-option v-for="value in selectedModel?.qualities" :key="value" :label="value" :value="value" /></el-select></el-form-item>
            <el-form-item :label="t('imageGeneration.format')"><el-select v-model="form.format" :disabled="locked"><el-option v-for="value in selectedModel?.formats" :key="value" :label="value.toUpperCase()" :value="value" /></el-select></el-form-item>
            <el-form-item :label="t('imageGeneration.background')"><el-select v-model="form.background" :disabled="locked"><el-option v-for="value in selectedModel?.backgrounds.filter((item) => form.format !== 'jpeg' || item !== 'transparent')" :key="value" :label="t(`imageGeneration.${value}`)" :value="value" /></el-select></el-form-item>
          </div>
          <p class="image-credit-estimate">{{ t('imageGeneration.estimate', { value: ((estimate * form.count) / 100).toFixed(2) }) }}</p>
          <el-button v-if="locked" class="image-primary-action" type="danger" :icon="VideoPause" @click="stop">{{ t('imageGeneration.stop') }}</el-button>
          <el-button v-else class="image-primary-action" type="primary" :icon="MagicStick" :loading="submitting" :disabled="!form.prompt.trim() || !validSize || (form.mode === 'edit' && references.length === 0)" @click="submit">{{ t('imageGeneration.generate') }}</el-button>
        </el-form>
      </aside>

      <section class="image-results-card">
        <div class="image-card-title"><el-icon><Picture /></el-icon><h2>{{ t('imageGeneration.results') }}</h2></div>
        <div v-if="active" class="image-result-meta"><el-tag>{{ t(`imageGeneration.${active.state}`) }}</el-tag><span>{{ active.image_model_name }} · {{ active.size }} · {{ (active.consumption_hundredths / 100).toFixed(2) }} Credits</span><span class="result-actions"><el-button v-if="active.images.length > 1" text @click="downloadAll">ZIP</el-button><el-button text :disabled="locked" @click="reuseSettings">Reuse settings</el-button><el-button text :disabled="locked" @click="regenerate">{{ t('common.retry') }}</el-button><el-button text type="danger" :disabled="locked" @click="deleteRecord">{{ t('common.delete') }}</el-button></span></div>
        <div v-if="active?.images?.length" class="generated-image-grid" :class="{ single: active.images.length === 1 }"><figure v-for="image in active.images" :key="image.position"><button v-if="imageURLs[image.position]" class="image-preview-trigger" :aria-label="`Preview result ${image.position}`" @click="previewPosition = image.position"><img class="generated-image-preview" :src="imageURLs[image.position]" :alt="`${active.image_model_name} result ${image.position}`"></button><div v-else class="image-loading-tile" /><figcaption><span>{{ image.width }} × {{ image.height }}</span><span><el-button text :disabled="locked || references.length >= 10" @click="reuseImageAsReference(image.position)">{{ t('imageGeneration.useAsReference') }}</el-button><el-button text :icon="Download" @click="downloadImage(image.position)">{{ t('imageGeneration.download') }}</el-button></span></figcaption></figure></div>
        <div v-else class="image-result-empty">
          <div v-if="locked" class="image-generation-motion" role="status" aria-live="polite">
            <span class="image-generation-orbit" aria-hidden="true"><i /><i /><i /></span>
            <p>{{ t(`imageGeneration.${active?.state}`) }}</p>
            <small>{{ active?.image_model_name }} · {{ active?.size }}</small>
          </div>
          <template v-else><el-icon><MagicStick /></el-icon><p>{{ active ? t(`imageGeneration.${active.state}`) : t('imageGeneration.empty') }}</p></template>
        </div>
        <details v-if="history.length" class="image-history"><summary>{{ t('imageGeneration.history') }} · {{ history.length }}</summary><button v-for="record in history" :key="record.id" :class="{ active: active?.id === record.id }" @click="selectRecord(record)"><span>{{ record.prompt }}</span><small>{{ t(`imageGeneration.${record.state}`) }} · {{ new Date(record.created_at).toLocaleString() }}</small></button></details>
      </section>
    </section>

    <div v-if="previewPosition !== undefined" class="image-lightbox" role="dialog" aria-modal="true" @click.self="previewPosition = undefined"><button class="lightbox-close" aria-label="Close preview" @click="previewPosition = undefined">×</button><button v-if="(active?.images.length ?? 0) > 1" aria-label="Previous image" @click="movePreview(-1)"><el-icon><ArrowLeft /></el-icon></button><img class="image-lightbox-image" :src="imageURLs[previewPosition]" alt="Generated image preview"><button v-if="(active?.images.length ?? 0) > 1" aria-label="Next image" @click="movePreview(1)"><el-icon><ArrowRight /></el-icon></button></div>

    <el-dialog v-model="adminOpen" :title="t('imageGeneration.adminTitle')" width="min(760px, 94vw)">
      <el-form label-position="top" class="image-admin-form">
        <el-form-item :label="t('imageGeneration.endpoint')"><el-input v-model="adminForm.endpoint" placeholder="https://api.openai.com/v1" /></el-form-item>
        <el-form-item :label="t('imageGeneration.apiKey')"><el-input v-model="adminForm.api_key" type="password" show-password autocomplete="new-password" /><small>{{ editingModel && editingModel.api_key_configured ? t('imageGeneration.keepApiKey') : t('imageGeneration.apiKeyRequired') }}</small></el-form-item>
        <el-form-item :label="t('imageGeneration.providerModelId')"><el-input v-model="adminForm.provider_model_id" /></el-form-item>
        <el-button type="primary" :icon="Plus" @click="createModel">{{ t('imageGeneration.addModel') }}</el-button>
      </el-form>
      <div class="image-admin-list"><div v-for="model in adminModels" :key="model.id"><div><strong>{{ model.display_name }}</strong><small>{{ model.provider_model_id }} · {{ model.state }} · {{ model.api_key_configured ? t('imageGeneration.secretSet') : t('imageGeneration.secretMissing') }}</small></div><el-button @click="editModel(model)">{{ t('common.edit') }}</el-button><el-button :disabled="model.state !== 'unverified' || !model.api_key_configured" @click="verify(model)">{{ t('imageGeneration.verify') }}</el-button><el-button :disabled="!model.verified_at" @click="toggle(model)">{{ t(model.state === 'available' ? 'imageGeneration.disable' : 'imageGeneration.enable') }}</el-button><el-button type="danger" text @click="deleteModel(model)">{{ t('common.delete') }}</el-button></div></div>
      <el-divider>{{ t('imageGeneration.optimizationModel') }}</el-divider>
      <el-form label-position="top" class="prompt-candidate-editor">
        <el-form-item :label="t('imageGeneration.providerModelId')"><el-input v-model="promptAdminForm.provider_model_id" /></el-form-item>
        <el-form-item :label="t('imageGeneration.endpoint')"><el-input v-model="promptAdminForm.endpoint" placeholder="https://api.openai.com/v1" /></el-form-item>
        <el-form-item :label="t('imageGeneration.apiKey')"><el-input v-model="promptAdminForm.api_key" type="password" show-password autocomplete="new-password" /><small>{{ promptModels[0]?.api_key_configured ? t('imageGeneration.keepApiKey') : t('imageGeneration.apiKeyRequired') }}</small></el-form-item>
        <el-form-item :label="t('imageGeneration.optimizationPrompt')"><el-input v-model="promptAdminForm.instruction" type="textarea" :rows="4" /></el-form-item>
        <el-button @click="savePromptCandidates">{{ t('common.save') }}</el-button>
      </el-form>
    </el-dialog>
  </main>
</template>
