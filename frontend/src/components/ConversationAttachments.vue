<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from "vue";
import { ChevronLeft, ChevronRight, Download, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import type { Attachment } from "../api/client";

const props = defineProps<{
  attachments: Attachment[];
  loadAttachment: (id: string) => Promise<Blob>;
}>();
const emit = defineEmits<{ error: [] }>();
const { t } = useI18n();
const imageURLs = ref<Record<string, string>>({});
const activeImageIndex = ref<number>();
const previewLayer = ref<HTMLElement>();
const previewTitleID = useId();
const imageAttachments = computed(() => props.attachments.filter((attachment) => attachment.image));
const activeImage = computed(() => activeImageIndex.value === undefined ? undefined : imageAttachments.value[activeImageIndex.value]);
const activeImageURL = computed(() => activeImage.value ? imageURLs.value[activeImage.value.id] : undefined);
const pendingImages = new Map<string, Promise<string | undefined>>();
let disposed = false;
let returnFocus: HTMLElement | undefined;

watch(
  () => props.attachments.map((attachment) => `${attachment.id}:${attachment.image}`).join("|"),
  () => {
    const currentIDs = new Set(props.attachments.map((attachment) => attachment.id));
    for (const [id, url] of Object.entries(imageURLs.value)) {
      if (currentIDs.has(id)) continue;
      URL.revokeObjectURL(url);
      delete imageURLs.value[id];
    }
    for (const attachment of imageAttachments.value) void ensureImageURL(attachment);
    if (activeImageIndex.value !== undefined && !activeImage.value) closePreview();
  },
  { immediate: true },
);

async function ensureImageURL(attachment: Attachment): Promise<string | undefined> {
  if (imageURLs.value[attachment.id]) return imageURLs.value[attachment.id];
  const pending = pendingImages.get(attachment.id);
  if (pending) return pending;
  const request = props.loadAttachment(attachment.id)
    .then((blob) => {
      if (disposed || !props.attachments.some((item) => item.id === attachment.id && item.image)) return undefined;
      const url = URL.createObjectURL(blob);
      imageURLs.value[attachment.id] = url;
      return url;
    })
    .catch(() => {
      emit("error");
      return undefined;
    })
    .finally(() => pendingImages.delete(attachment.id));
  pendingImages.set(attachment.id, request);
  return request;
}

async function openAttachment(attachment: Attachment, event: MouseEvent) {
  if (!attachment.image) {
    await downloadAttachment(attachment);
    return;
  }
  const index = imageAttachments.value.findIndex((item) => item.id === attachment.id);
  if (index < 0 || !await ensureImageURL(attachment)) return;
  returnFocus = event.currentTarget instanceof HTMLElement ? event.currentTarget : undefined;
  activeImageIndex.value = index;
  await nextTick();
  previewLayer.value?.focus();
}

async function moveImage(offset: number) {
  const count = imageAttachments.value.length;
  if (activeImageIndex.value === undefined || count < 2) return;
  const index = (activeImageIndex.value + offset + count) % count;
  if (!await ensureImageURL(imageAttachments.value[index]!)) return;
  activeImageIndex.value = index;
}

function handlePreviewKey(event: KeyboardEvent) {
  if (event.key === "Escape") closePreview();
  else if (event.key === "ArrowLeft" || event.key === "ArrowUp") void moveImage(-1);
  else if (event.key === "ArrowRight" || event.key === "ArrowDown") void moveImage(1);
  else return;
  event.preventDefault();
}

function closePreview() {
  activeImageIndex.value = undefined;
  void nextTick(() => returnFocus?.focus());
}

async function downloadAttachment(attachment: Attachment) {
  try {
    const cachedURL = attachment.image ? imageURLs.value[attachment.id] : undefined;
    const url = cachedURL ?? URL.createObjectURL(await props.loadAttachment(attachment.id));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = attachment.name;
    anchor.rel = "noopener noreferrer";
    anchor.click();
    if (!cachedURL) window.setTimeout(() => URL.revokeObjectURL(url), 0);
  } catch {
    emit("error");
  }
}

function clearImageURL(attachment: Attachment) {
  const url = imageURLs.value[attachment.id];
  if (!url) return;
  URL.revokeObjectURL(url);
  delete imageURLs.value[attachment.id];
  if (activeImage.value?.id === attachment.id) closePreview();
}

onBeforeUnmount(() => {
  disposed = true;
  for (const url of Object.values(imageURLs.value)) URL.revokeObjectURL(url);
});
</script>

<template>
  <div class="turn-attachments">
    <button
      v-for="attachment in attachments"
      :key="attachment.id"
      type="button"
      class="turn-attachment"
      :aria-label="t(attachment.image ? 'attachments.preview' : 'attachments.download', { name: attachment.name })"
      @click="openAttachment(attachment, $event)"
    >
      <img v-if="attachment.image && imageURLs[attachment.id]" :src="imageURLs[attachment.id]" :alt="attachment.name" @error="clearImageURL(attachment)">
      <span v-else class="attachment-file-mark">{{ attachment.image ? 'IMG' : 'FILE' }}</span>
      <span><strong>{{ attachment.name }}</strong><small>{{ (attachment.size / 1024).toFixed(1) }} KB</small></span>
    </button>
  </div>

  <Teleport to="body">
    <Transition name="image-preview">
      <div
        v-if="activeImage && activeImageURL"
        ref="previewLayer"
        class="image-preview-layer"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="previewTitleID"
        tabindex="-1"
        @click.self="closePreview"
        @keydown="handlePreviewKey"
      >
        <header class="image-preview-head">
          <div><strong :id="previewTitleID">{{ activeImage.name }}</strong><small>{{ t('attachments.position', { current: activeImageIndex! + 1, total: imageAttachments.length }) }}</small></div>
          <div>
            <button type="button" :aria-label="t('attachments.download', { name: activeImage.name })" @click="downloadAttachment(activeImage)"><Download aria-hidden="true" /></button>
            <button type="button" :aria-label="t('common.close')" @click="closePreview"><X aria-hidden="true" /></button>
          </div>
        </header>
        <div class="image-preview-canvas" @click.self="closePreview">
          <button type="button" class="image-preview-nav previous" :disabled="imageAttachments.length < 2" :aria-label="t('attachments.previous')" @click="moveImage(-1)"><ChevronLeft aria-hidden="true" /></button>
          <img :src="activeImageURL" :alt="activeImage.name" @error="clearImageURL(activeImage)">
          <button type="button" class="image-preview-nav next" :disabled="imageAttachments.length < 2" :aria-label="t('attachments.next')" @click="moveImage(1)"><ChevronRight aria-hidden="true" /></button>
        </div>
        <p class="image-preview-hint">{{ t('attachments.navigationHint') }}</p>
      </div>
    </Transition>
  </Teleport>
</template>
