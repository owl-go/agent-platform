<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ElNotification } from "element-plus";
import { Box, ChatDotRound, Connection, Folder, Loading, Menu, MoreFilled, Picture, Plus, Setting, SwitchButton, User, UserFilled } from "@element-plus/icons-vue";
import en from "element-plus/es/locale/lang/en";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import { getHealth, platformApiKey, type CreditBalance } from "./api/client";
import { authContextKey } from "./auth/session";
import { localeStorageKey, type SupportedLocale } from "./i18n";
import CreditPanel from "./components/CreditPanel.vue";
import ApprovalInbox from "./components/ApprovalInbox.vue";

const auth = inject(authContextKey)!;
const api = inject(platformApiKey)!;
const route = useRoute();
const router = useRouter();
const { locale, t } = useI18n();
const authState = auth.session.state;
const currentUser = computed(() => authState.value.kind === "authenticated" ? authState.value.currentUser : undefined);
const online = ref<boolean | undefined>();
const mobileOpen = ref(false);
const creditPanelOpen = ref(false);
const creditBalance = ref<CreditBalance>();
const aiCreationUnread = ref(localStorage.getItem("ai-creation-unread") === "1");
const initials = computed(() => (currentUser.value?.display_name || currentUser.value?.username || "U").split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase()).join(""));
const navGroups = [
  {
    id: "workspace",
    items: [
      { id: "sessions", icon: ChatDotRound, path: "/sessions" },
      { id: "workflows", icon: Connection, path: "/workflows" },
      { id: "ai-creation", icon: Picture, path: "/ai-creation/image-generation" },
      { id: "knowledge-bases", icon: Folder, path: "/knowledge-bases" },
    ],
  },
  {
    id: "resources",
    items: [{ id: "resources", icon: Box, path: "/resources" }],
  },
  {
    id: "system",
    items: [{ id: "settings", icon: Setting, path: "/settings" }],
  },
] as const;
const elementLocale = computed(() => locale.value === "zh-CN" ? zhCn : en);
let controller: AbortController | undefined;
let imageMonitorTimer: number | undefined;
let imageMonitorRun = 0;
let monitoredImageRecord = "";
const formatCredits = (hundredths: number | undefined) => (Number(hundredths ?? 0) / 100).toFixed(2);

watch(currentUser, (user) => {
  if (user?.credit_balance) creditBalance.value = user.credit_balance;
  void monitorImageGeneration();
}, { immediate: true });
watch(() => route.meta.surface, (surface) => {
  if (surface === "ai-creation") {
    aiCreationUnread.value = false;
    localStorage.removeItem("ai-creation-unread");
  }
}, { immediate: true });

async function refreshCredits() {
  if (!currentUser.value) return;
  try { creditBalance.value = await api.getCreditBalance(); } catch { /* Keep the last known projection. */ }
}

function handleCreditsUpdated() {
  void refreshCredits();
  void monitorImageGeneration();
}

onMounted(() => {
  void auth.session.initialize(auth.isCallback);
  controller = new AbortController();
  getHealth(controller.signal).then(() => { online.value = true; }).catch(() => { online.value = false; });
  window.addEventListener("credits-updated", handleCreditsUpdated);
});
onUnmounted(() => { controller?.abort(); imageMonitorRun++; window.clearTimeout(imageMonitorTimer); auth.session.dispose(); window.removeEventListener("credits-updated", handleCreditsUpdated); });

async function monitorImageGeneration() {
  const run = ++imageMonitorRun;
  window.clearTimeout(imageMonitorTimer);
  if (!currentUser.value) return;
  let continueMonitoring = false;
  try {
    const records = await api.listImageGenerations();
    if (run !== imageMonitorRun) return;
    const running = records.find((record) => record.state === "pending" || record.state === "running");
    if (running) {
      monitoredImageRecord = running.id;
      continueMonitoring = true;
    }
    else if (monitoredImageRecord) {
      const completed = records.find((record) => record.id === monitoredImageRecord);
      if (completed) {
        ElNotification({ title: t("imageGeneration.title"), message: t(`imageGeneration.${completed.state}`), type: completed.state === "succeeded" ? "success" : "warning" });
        if (route.meta.surface !== "ai-creation") {
          aiCreationUnread.value = true;
          localStorage.setItem("ai-creation-unread", "1");
        }
      }
      monitoredImageRecord = "";
    }
  } catch { continueMonitoring = monitoredImageRecord !== ""; }
  if (run === imageMonitorRun && continueMonitoring) imageMonitorTimer = window.setTimeout(monitorImageGeneration, 3000);
}

function setLocale(value: SupportedLocale) {
  locale.value = value;
  localStorage.setItem(localeStorageKey, value);
  document.documentElement.lang = value;
}

function handleUserCommand(command: "credits" | "users" | "locale" | "signout") {
  if (command === "credits") creditPanelOpen.value = true;
  if (command === "users") void router.push("/admin/users");
  if (command === "locale") setLocale(locale.value === "zh-CN" ? "en-US" : "zh-CN");
  if (command === "signout") void auth.session.signOut();
}
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <section v-if="authState.kind === 'checking'" class="auth-screen">
      <el-icon class="auth-loading" :size="42"><Loading /></el-icon><h1>{{ t('auth.checking') }}</h1>
    </section>
    <section v-else-if="authState.kind === 'unauthenticated'" class="auth-screen">
      <el-card class="auth-card" shadow="always"><div class="logo-mark">AW</div><p class="eyebrow">{{ t('product') }}</p><h1>{{ t('auth.required') }}</h1><p>{{ t('auth.body') }}</p><el-button type="primary" size="large" class="wide" @click="auth.session.signIn()">{{ t('auth.signIn') }} <span>→</span></el-button></el-card>
    </section>
    <section v-else-if="authState.kind === 'error'" class="auth-screen"><el-result icon="error" :title="t('auth.unavailable')" :sub-title="authState.message" /></section>
    <el-container v-else class="app-shell">
      <el-aside class="sidebar" :class="{ open: mobileOpen }" width="248px">
        <RouterLink to="/sessions" class="product-lockup" @click="mobileOpen = false"><span class="logo-mark">AW</span><span><strong>Agent</strong><small>Workspace</small></span></RouterLink>
        <el-button class="new-session" @click="$router.push('/sessions?new=1'); mobileOpen = false"><el-icon><Plus /></el-icon>{{ t('sessions.new') }}</el-button>
        <nav :aria-label="t('nav.label')">
          <section v-for="group in navGroups" :key="group.id" class="nav-group">
            <h2>{{ t(`nav.groups.${group.id}`) }}</h2>
            <RouterLink v-for="item in group.items" :key="item.id" :to="item.path" :class="{ 'router-link-active': route.meta.surface === item.id }" :aria-current="route.meta.surface === item.id ? 'page' : undefined" @click="mobileOpen = false"><el-icon class="nav-icon"><component :is="item.icon" /></el-icon>{{ t(`nav.${item.id}`) }}<span v-if="item.id === 'ai-creation' && aiCreationUnread" class="nav-unread" aria-label="Unread completion"></span></RouterLink>
          </section>
        </nav>
        <div class="sidebar-spacer"></div>
        <div class="connection-state"><el-badge is-dot :type="online === true ? 'success' : online === false ? 'danger' : 'info'" /><span>{{ online === true ? t('auth.online') : online === false ? t('auth.offline') : t('auth.checkingApi') }}</span></div>
        <div class="user-zone">
          <el-dropdown placement="top-start" trigger="click" @command="handleUserCommand">
            <button class="user-button"><el-avatar :size="34">{{ initials }}</el-avatar><span><strong>{{ currentUser?.display_name }}</strong><small>@{{ currentUser?.username }}</small></span><el-icon><MoreFilled /></el-icon></button>
            <template #dropdown><el-dropdown-menu>
              <el-dropdown-item command="credits"><span class="credit-menu-row"><b>✧</b><span>{{ t('credits.balance') }}</span><strong>{{ formatCredits(creditBalance?.total_hundredths) }} ›</strong></span></el-dropdown-item>
              <el-dropdown-item v-if="currentUser?.administrator" command="users" :icon="User">{{ t('nav.users') }}</el-dropdown-item>
              <el-dropdown-item command="locale" :icon="UserFilled">{{ locale === 'zh-CN' ? 'English' : '中文' }}</el-dropdown-item>
              <el-dropdown-item command="signout" :icon="SwitchButton" divided>{{ t('auth.signOut') }}</el-dropdown-item>
            </el-dropdown-menu></template>
          </el-dropdown>
        </div>
      </el-aside>
      <button v-if="mobileOpen" class="scrim" @click="mobileOpen = false"></button>
      <el-main class="main-stage">
        <header class="mobile-header"><el-button text :icon="Menu" @click="mobileOpen = true" /><strong>{{ t('product') }}</strong><el-avatar :size="32">{{ initials }}</el-avatar></header>
        <ApprovalInbox v-if="currentUser" />
        <RouterView :key="String(route.name)" />
      </el-main>
      <CreditPanel :open="creditPanelOpen" @close="creditPanelOpen = false" @updated="creditBalance = $event" />
    </el-container>
  </el-config-provider>
</template>
