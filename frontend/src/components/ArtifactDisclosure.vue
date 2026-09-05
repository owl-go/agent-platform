<script setup lang="ts">
import { computed, ref } from "vue";
import { ChevronRight, Download, FileText } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import type { Artifact } from "../api/client";

const props = defineProps<{ artifacts: Artifact[] }>();
const emit = defineEmits<{ download: [artifact: Artifact] }>();
const { t } = useI18n();
const showArtifacts = ref(false);
const showChanges = ref(false);
const files = computed(() => props.artifacts.filter((artifact) => artifact.kind === "file"));

function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function toggleArtifacts() {
  showArtifacts.value = !showArtifacts.value;
  if (showArtifacts.value) showChanges.value = false;
}

function toggleChanges() {
  showChanges.value = !showChanges.value;
  if (showChanges.value) showArtifacts.value = false;
}
</script>

<template>
  <div v-if="files.length" class="artifact-disclosure">
    <div class="artifact-disclosure-links">
      <button type="button" :aria-expanded="showArtifacts" @click="toggleArtifacts">
        {{ t('artifactDisclosure.viewAllArtifacts', { count: files.length }) }}<ChevronRight aria-hidden="true" />
      </button>
      <button type="button" :aria-expanded="showChanges" @click="toggleChanges">
        {{ t('artifactDisclosure.viewAllChanges', { count: files.length }) }}<ChevronRight aria-hidden="true" />
      </button>
    </div>
    <div v-if="showArtifacts" class="generated-artifacts">
      <button v-for="artifact in files" :key="artifact.id" type="button" class="generated-artifact" :disabled="artifact.expired" @click="emit('download', artifact)">
        <span class="generated-artifact-icon" aria-hidden="true"><FileText /></span>
        <span><strong>{{ artifact.name }}</strong><small>{{ artifact.expired ? t('workflows.expired') : formatFileSize(artifact.size) }}</small></span>
        <Download aria-hidden="true" />
      </button>
    </div>
    <div v-if="showChanges" class="artifact-changes">
      <article v-for="artifact in files" :key="artifact.id">
        <header><span class="generated-artifact-icon" aria-hidden="true"><FileText /></span><span><strong>{{ artifact.name }}</strong><small>{{ formatFileSize(artifact.size) }}</small></span></header>
      </article>
    </div>
  </div>
</template>
