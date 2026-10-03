<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
const code = ref("");
const appliedCode = ref("");
const revision = ref(0);
const error = ref(false);
const origin = window.location.origin;
const previewDocument = computed(() => `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html,body{margin:0;width:100%;height:100%}body{overflow:auto}</style></head><body>${appliedCode.value}</body></html>`);
function run() {
  if (!code.value.trim()) { error.value = true; return; }
  error.value = false;
  appliedCode.value = code.value;
  revision.value++;
}
function clear() {
  code.value = "";
  appliedCode.value = "";
  revision.value++;
  error.value = false;
}
</script>

<template>
  <main class="embed-preview-page">
    <header>
      <h1>{{ t('embedPreview.title') }}</h1>
      <p>{{ t('embedPreview.introduction') }}</p>
    </header>
    <section class="embed-preview-editor">
      <el-form label-position="top" @submit.prevent="run">
        <el-form-item :label="t('embedPreview.code')">
          <el-input v-model="code" data-testid="embed-code" type="textarea" :rows="9" :placeholder="t('embedPreview.placeholder')" :aria-label="t('embedPreview.code')" />
        </el-form-item>
        <el-alert v-if="error" type="error" :title="t('embedPreview.empty')" :closable="false" />
        <div class="embed-preview-actions">
          <el-button data-testid="run-preview" type="primary" @click="run">{{ t('embedPreview.run') }}</el-button>
          <el-button @click="clear">{{ t('embedPreview.clear') }}</el-button>
          <span>{{ t('embedPreview.origin', { origin }) }}</span>
        </div>
      </el-form>
    </section>
    <section class="embed-preview-result">
      <h2>{{ t('embedPreview.result') }}</h2>
      <div class="embed-preview-stage">
        <iframe v-if="appliedCode" :key="revision" data-testid="embed-result" :srcdoc="previewDocument" :title="t('embedPreview.result')" sandbox="allow-scripts allow-same-origin allow-forms allow-popups" />
        <p v-else>{{ t('embedPreview.pending') }}</p>
      </div>
    </section>
  </main>
</template>

<style>
body { margin: 0; background: var(--aw-n2); font-family: var(--aw-font-sans); color: var(--aw-n10); }
</style>
<style scoped>
.embed-preview-page { max-width: 1200px; margin: 0 auto; padding: var(--aw-space-6); }
h1 { margin: 0; font-size: var(--el-font-size-extra-large); }
header p { margin: var(--aw-space-2) 0 var(--aw-space-5); color: var(--aw-n7); }
.embed-preview-editor { background: var(--aw-n0); border: 1px solid var(--aw-n4); border-radius: var(--aw-radius-card); padding: var(--aw-space-5); }
.embed-preview-actions { display: flex; align-items: center; flex-wrap: wrap; gap: var(--aw-space-3); margin-top: var(--aw-space-3); }
.embed-preview-actions .el-button + .el-button { margin-left: 0; }
.embed-preview-actions span { color: var(--aw-n7); font-size: var(--el-font-size-small); overflow-wrap: anywhere; }
.embed-preview-editor :deep(textarea) { font-family: var(--aw-font-mono); }
h2 { font-size: var(--el-font-size-large); margin: var(--aw-space-5) 0 var(--aw-space-3); }
.embed-preview-stage { height: 700px; min-height: 600px; background: var(--aw-n0); border: 1px solid var(--aw-n4); border-radius: var(--aw-radius-card); overflow: hidden; }
.embed-preview-stage iframe { display: block; width: 100%; height: 100%; border: 0; }
.embed-preview-stage > p { text-align: center; margin: var(--aw-space-8) var(--aw-space-4); color: var(--aw-n7); }
@media (max-width: 600px) {
  .embed-preview-page { padding: var(--aw-space-3); }
  .embed-preview-editor { padding: var(--aw-space-4); }
  .embed-preview-stage { height: 640px; }
}
</style>
