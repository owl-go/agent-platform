<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ElNotification } from "element-plus";
import { ArrowRight, Box, ChatDotRound, CircleClose, Connection, HomeFilled, Loading, Menu, MoreFilled, Picture, Plus, Setting, SwitchButton, User, UserFilled } from "@element-plus/icons-vue";
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
const aiApplicationsUnread = ref(localStorage.getItem("ai-applications-unread") === "1");
const aiApplicationsExpanded = ref(localStorage.getItem("ai-applications-expanded") !== "0");
const initials = computed(() => (currentUser.value?.display_name || currentUser.value?.username || "U").split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase()).join(""));
type ResourceCenterTab = "experts" | "skills" | "connectors" | "knowledge";
type NavChild = { id: string; path: string; resourceTab?: ResourceCenterTab; routeNames?: string[] };
type NavItem = { id: string; icon: typeof Picture; path: string; children?: NavChild[] };
type NavGroup = { id: string; items: NavItem[] };
const navGroups = [
  {
    id: "workspace",
    items: [
      { id: "home", icon: HomeFilled, path: "/home" },
      { id: "sessions", icon: ChatDotRound, path: "/sessions" },
      { id: "workflows", icon: Connection, path: "/workflows" },
      {
        id: "ai-applications",
        icon: Picture,
        path: "/ai-apps",
        children: [
          { id: "ai-applications-assistants", path: "/ai-apps/assistants" },
          { id: "ai-applications-image-creation", path: "/ai-apps/image-creation" },
        ],
      },
    ],
  },
  {
    id: "resources",
    items: [{
      id: "resources",
      icon: Box,
      path: "/resources",
      children: [
        { id: "resources-experts", path: "/resources", resourceTab: "experts", routeNames: ["expert-new", "expert-edit", "expert-team-new", "expert-team-edit"] },
        { id: "resources-skills", path: "/resources?tab=skills", resourceTab: "skills", routeNames: ["skill-detail"] },
        { id: "resources-connectors", path: "/resources?tab=connectors", resourceTab: "connectors" },
        { id: "resources-knowledge", path: "/resources?tab=knowledge", resourceTab: "knowledge" },
      ],
    }],
  },
  {
    id: "system",
    items: [{ id: "settings", icon: Setting, path: "/settings" }],
  },
] satisfies NavGroup[];
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
  if (surface === "ai-applications") {
    aiApplicationsUnread.value = false;
    localStorage.removeItem("ai-applications-unread");
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
        if (route.meta.surface !== "ai-applications") {
          aiApplicationsUnread.value = true;
          localStorage.setItem("ai-applications-unread", "1");
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

function isNavItemActive(item: NavItem) {
  return route.meta.surface === item.id;
}

function isNavChildActive(child: NavChild) {
  if (child.routeNames?.includes(String(route.name))) return true;
  if (child.resourceTab) {
    if (route.name === "resources") {
      const tab = ["skills", "connectors", "knowledge"].includes(String(route.query.tab)) ? String(route.query.tab) : "experts";
      return child.resourceTab === tab;
    }
    return false;
  }
  return route.path === child.path || route.path.startsWith(`${child.path}/`);
}

const resourcesExpanded = ref(localStorage.getItem("resources-expanded") !== "0");

function isNavItemExpanded(item: NavItem) {
  return item.id === "ai-applications" ? aiApplicationsExpanded.value : resourcesExpanded.value;
}

function toggleNavItem(item: NavItem) {
  if (item.id === "ai-applications") {
    aiApplicationsExpanded.value = !aiApplicationsExpanded.value;
    localStorage.setItem("ai-applications-expanded", aiApplicationsExpanded.value ? "1" : "0");
    return;
  }
  resourcesExpanded.value = !resourcesExpanded.value;
  localStorage.setItem("resources-expanded", resourcesExpanded.value ? "1" : "0");
}
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <main v-if="authState.kind !== 'authenticated'" class="auth-screen" aria-labelledby="auth-title">
      <el-card class="auth-card" shadow="never">
        <div class="auth-brand"><span class="logo-mark" aria-hidden="true">AW</span><strong>{{ t('product') }}</strong></div>
        <div v-if="authState.kind === 'checking'" class="auth-content" role="status" aria-live="polite" aria-busy="true">
          <el-icon class="auth-loading" :size="24" aria-hidden="true"><Loading /></el-icon>
          <h1 id="auth-title">{{ t('auth.checking') }}</h1>
        </div>
        <div v-else-if="authState.kind === 'unauthenticated'" class="auth-content">
          <h1 id="auth-title">{{ t('auth.required') }}</h1>
          <p>{{ t('auth.body') }}</p>
          <el-button type="primary" size="large" class="auth-sign-in" @click="auth.session.signIn()">{{ t('auth.signIn') }}<el-icon aria-hidden="true"><ArrowRight /></el-icon></el-button>
        </div>
        <div v-else class="auth-content" role="alert">
          <el-icon class="auth-error-icon" :size="24" aria-hidden="true"><CircleClose /></el-icon>
          <h1 id="auth-title">{{ t('auth.unavailable') }}</h1>
          <p>{{ authState.message }}</p>
        </div>
      </el-card>
    </main>
    <el-container v-else class="app-shell">
      <el-aside class="sidebar" :class="{ open: mobileOpen }" width="240px">
        <RouterLink to="/home" class="product-lockup" @click="mobileOpen = false"><span class="logo-mark">AW</span><span><strong>Agent</strong><small>Workspace</small></span></RouterLink>
        <el-button class="new-session" @click="$router.push('/sessions?new=1'); mobileOpen = false"><el-icon><Plus /></el-icon>{{ t('sessions.new') }}</el-button>
        <nav :aria-label="t('nav.label')">
          <section v-for="group in navGroups" :key="group.id" class="nav-group">
            <h2>{{ t(`nav.groups.${group.id}`) }}</h2>
            <template v-for="item in group.items" :key="item.id">
              <template v-if="item.children">
                <button class="nav-parent" :class="{ 'router-link-active': isNavItemActive(item) }" type="button" :data-nav-id="item.id" :aria-current="isNavItemActive(item) ? 'page' : undefined" :aria-expanded="isNavItemExpanded(item)" :aria-controls="`${item.id}-submenu`" @click="toggleNavItem(item)"><el-icon class="nav-icon"><component :is="item.icon" /></el-icon><span>{{ t(`nav.${item.id}`) }}</span><span v-if="item.id === 'ai-applications' && aiApplicationsUnread" class="nav-unread" aria-label="Unread completion"></span><span class="nav-chevron" :class="{ expanded: isNavItemExpanded(item) }" aria-hidden="true">⌄</span></button>
                <div v-if="isNavItemExpanded(item)" :id="`${item.id}-submenu`" class="nav-submenu" :aria-label="t(`nav.${item.id}`)">
                  <RouterLink v-for="child in item.children" :key="child.id" :to="child.path" active-class="nav-route-active" exact-active-class="nav-route-exact-active" :class="{ 'router-link-active': isNavChildActive(child) }" :aria-current="isNavChildActive(child) ? 'page' : undefined" @click="mobileOpen = false">{{ t(`nav.${child.id}`) }}</RouterLink>
                </div>
              </template>
              <RouterLink v-else :to="item.path" :class="{ 'router-link-active': isNavItemActive(item) }" :aria-current="isNavItemActive(item) ? 'page' : undefined" @click="mobileOpen = false"><el-icon class="nav-icon"><component :is="item.icon" /></el-icon>{{ t(`nav.${item.id}`) }}</RouterLink>
            </template>
          </section>
        </nav>
        <div class="sidebar-spacer"></div>
        <div class="connection-state"><el-badge is-dot :type="online === true ? 'success' : online === false ? 'danger' : 'info'" /><span>{{ online === true ? t('auth.online') : online === false ? t('auth.offline') : t('auth.checkingApi') }}</span></div>
        <div class="user-zone">
          <el-dropdown placement="top-start" trigger="click" @command="handleUserCommand">
            <button class="user-button"><el-avatar :size="34">{{ initials }}</el-avatar><span><strong>{{ currentUser?.display_name }}</strong><small>@{{ currentUser?.username }}</small></span><el-icon><MoreFilled /></el-icon></button>
            <template #dropdown><el-dropdown-menu>
              <el-dropdown-item command="credits"><span class="credit-menu-row"><b>✧</b><span>{{ t('credits.available') }}</span><strong>{{ formatCredits(creditBalance?.available_hundredths) }} ›</strong></span></el-dropdown-item>
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
