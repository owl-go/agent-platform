<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { ArrowLeft, Code2, Eye, Plus } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { platformApiKey, type Skill } from "../api/client";
import { renderMarkdown } from "../markdown";
import { localizedSkillDescription, parseSkillDocument } from "../skillDocument";

const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { locale, t } = useI18n();
const skill = ref<Skill>();
const content = ref("");
const loading = ref(true);
const error = ref("");
const raw = ref(false);
const metadata = computed(() => parseSkillDocument(content.value));
const title = computed(() => metadata.value.displayName || skill.value?.name || t("resources.skillDetail"));
const description = computed(() => localizedSkillDescription(metadata.value, locale.value));

onMounted(async () => {
  try {
    const document = await api.getSkillDocument(String(route.params.skillId));
    skill.value = document.skill;
    content.value = document.content;
  } catch {
    error.value = t("resources.skillLoadFailed");
  } finally {
    loading.value = false;
  }
});

function back() {
  const returnTo = (router.options.history.state as { skillReturnTo?: string }).skillReturnTo;
  if (returnTo) router.back();
  else void router.push("/resources");
}

function useSkill() {
  if (skill.value) void router.push({ path: "/sessions", query: { new: crypto.randomUUID(), skill_id: skill.value.id } });
}
</script>

<template>
  <section class="page-surface skill-detail-page">
    <button class="skill-detail-back" type="button" @click="back"><ArrowLeft />{{ t('resources.backSkills') }}</button>
    <el-skeleton v-if="loading" class="skill-detail-loading" :rows="8" animated />
    <el-result v-else-if="error" icon="error" :title="error"><template #extra><el-button @click="back">{{ t('resources.backSkills') }}</el-button></template></el-result>
    <template v-else-if="skill">
      <header class="skill-detail-hero">
        <div><p class="eyebrow">SKILL</p><h1>{{ title }}</h1><p>{{ description || t('resources.noSkillDescription') }}</p></div>
        <el-button type="primary" size="large" @click="useSkill"><Plus />{{ t('composer.useSkill') }}</el-button>
      </header>
      <article class="skill-document-card el-card">
        <div class="skill-document-toolbar">
          <dl>
            <div><dt>{{ t('common.name') }}</dt><dd>{{ title }}</dd></div>
            <div><dt>{{ t('resources.documentVersion') }}</dt><dd>{{ metadata.version || skill.version }}</dd></div>
            <div><dt>{{ t('settings.source') }}</dt><dd>{{ skill.source === 'git' ? skill.git_url : t('composer.localSkill') }}</dd></div>
          </dl>
          <div class="skill-document-view-switch">
            <el-button circle :type="raw ? 'default' : 'primary'" :aria-label="t('resources.previewDocument')" @click="raw = false"><Eye /></el-button>
            <el-button circle :type="raw ? 'primary' : 'default'" :aria-label="t('resources.rawDocument')" @click="raw = true"><Code2 /></el-button>
          </div>
        </div>
        <pre v-if="raw" class="skill-document-raw">{{ content }}</pre>
        <div v-else class="markdown-body skill-document" v-html="renderMarkdown(metadata.body)"></div>
      </article>
    </template>
  </section>
</template>
