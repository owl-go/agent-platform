<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { ArrowRight, Connection, Refresh, Warning } from "@element-plus/icons-vue";
import { platformApiKey, type HomeAction, type HomeOverview, type HomeTask } from "../api/client";

const api = inject(platformApiKey)!;
const router = useRouter();
const { locale, t } = useI18n();
const overview = ref<HomeOverview>({ recent_tasks: [], common_workflows: [], action_items: [] });
const loading = ref(true);
const failed = ref(false);
const hasContent = computed(() => overview.value.action_items.length + overview.value.recent_tasks.length + overview.value.common_workflows.length > 0);
let controller: AbortController | undefined;

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(value));
}

function taskLocation(task: Pick<HomeTask, "kind" | "id" | "parent_id"> | Pick<HomeAction, "execution_kind" | "execution_id" | "parent_id">) {
	const kind = "execution_kind" in task ? task.execution_kind : task.kind;
	const id = "execution_id" in task ? task.execution_id : task.id;
  if (kind === "session") return { path: "/sessions", query: { open: "parent_id" in task && task.parent_id ? task.parent_id : id } };
  return { path: `/workflows/${task.parent_id}`, query: { tab: "history", open_run: id } };
}

function stateLabel(state: string) {
  const known = new Set(["idle", "queued", "running", "waiting_for_user", "succeeded", "completed", "failed", "cancelled"]);
  return known.has(state) ? t(`home.taskState.${state}`) : state;
}

function openTask(task: HomeTask | HomeAction) {
  void router.push(taskLocation(task));
}

async function load() {
  controller?.abort();
  controller = new AbortController();
  loading.value = true;
  failed.value = false;
  try {
    overview.value = await api.getHomeOverview(controller.signal);
  } catch (error) {
    if ((error as DOMException).name !== "AbortError") failed.value = true;
  } finally {
    loading.value = false;
  }
}

function refreshWhenVisible() {
  if (document.visibilityState === "visible") void load();
}

onMounted(() => {
  void load();
  document.addEventListener("visibilitychange", refreshWhenVisible);
});
onUnmounted(() => {
  controller?.abort();
  document.removeEventListener("visibilitychange", refreshWhenVisible);
});
</script>

<template>
  <main class="page-surface home-page">
    <header class="page-header home-header">
      <div><p class="eyebrow">{{ t('product') }}</p><h1>{{ t('home.title') }}</h1><p>{{ t('home.subtitle') }}</p></div>
      <el-button :icon="Refresh" :loading="loading" @click="load">{{ t('home.refresh') }}</el-button>
    </header>

    <el-alert v-if="failed" type="error" :closable="false" :title="t('home.loadFailed')"><el-button link type="primary" @click="load">{{ t('common.retry') }}</el-button></el-alert>
    <div v-else-if="loading && !hasContent" class="home-loading"><el-skeleton :rows="6" animated /></div>
    <el-empty v-else-if="!hasContent" :description="t('home.empty')" />

    <div v-else class="home-layout">
      <section class="home-section home-actions" aria-labelledby="home-actions-heading">
        <div class="home-section-heading"><div><h2 id="home-actions-heading">{{ t('home.actions') }}</h2><p>{{ t('home.actionsHint') }}</p></div><el-tag v-if="overview.action_items.length" type="warning" round>{{ overview.action_items.length }}</el-tag></div>
        <div v-if="overview.action_items.length" class="home-list">
          <button v-for="action in overview.action_items" :key="`${action.execution_kind}:${action.execution_id}`" class="home-row home-action-row" type="button" @click="openTask(action)">
            <span class="home-row-icon attention"><el-icon><Warning /></el-icon></span>
            <span class="home-row-copy"><strong>{{ action.title }}</strong><small>{{ t(`home.actionKind.${action.kind}`) }} · {{ t(`home.taskKind.${action.execution_kind}`) }} · {{ formatDate(action.created_at) }}</small></span>
            <el-icon class="home-row-arrow"><ArrowRight /></el-icon>
          </button>
        </div>
        <p v-else class="home-section-empty">{{ t('home.noActions') }}</p>
      </section>

      <section class="home-section" aria-labelledby="home-recent-heading">
        <div class="home-section-heading"><div><h2 id="home-recent-heading">{{ t('home.recent') }}</h2><p>{{ t('home.recentHint') }}</p></div></div>
        <div v-if="overview.recent_tasks.length" class="home-list">
          <button v-for="task in overview.recent_tasks" :key="`${task.kind}:${task.id}`" class="home-row" type="button" @click="openTask(task)">
            <span class="home-row-icon"><el-icon><Connection /></el-icon></span>
            <span class="home-row-copy"><strong>{{ task.title }}</strong><small>{{ t(`home.taskKind.${task.kind}`) }} · {{ stateLabel(task.state) }} · {{ formatDate(task.updated_at) }}</small></span>
            <el-icon class="home-row-arrow"><ArrowRight /></el-icon>
          </button>
        </div>
        <p v-else class="home-section-empty">{{ t('home.noRecent') }}</p>
      </section>

      <section class="home-section home-workflows" aria-labelledby="home-workflows-heading">
        <div class="home-section-heading"><div><h2 id="home-workflows-heading">{{ t('home.commonWorkflows') }}</h2><p>{{ t('home.commonWorkflowsHint') }}</p></div></div>
        <div v-if="overview.common_workflows.length" class="home-workflow-grid">
          <button v-for="workflow in overview.common_workflows" :key="workflow.id" type="button" @click="router.push(`/workflows/${workflow.id}`)">
            <span class="home-row-icon"><el-icon><Connection /></el-icon></span><strong>{{ workflow.name }}</strong><small>{{ t('home.runCount', { count: workflow.run_count }) }} · {{ formatDate(workflow.updated_at) }}</small>
          </button>
        </div>
        <p v-else class="home-section-empty">{{ t('home.noWorkflows') }}</p>
      </section>
    </div>
  </main>
</template>
