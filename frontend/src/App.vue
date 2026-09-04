<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ChatDotRound, Connection, Loading, MagicStick, Menu, MoreFilled, Plus, Setting, SwitchButton, User, UserFilled } from "@element-plus/icons-vue";
import en from "element-plus/es/locale/lang/en";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import { getHealth } from "./api/client";
import { authContextKey } from "./auth/session";
import { localeStorageKey, type SupportedLocale } from "./i18n";

const auth = inject(authContextKey)!;
const route = useRoute();
const router = useRouter();
const { locale, t } = useI18n();
const authState = auth.session.state;
const currentUser = computed(() => authState.value.kind === "authenticated" ? authState.value.currentUser : undefined);
const online = ref<boolean | undefined>();
const mobileOpen = ref(false);
const initials = computed(() => (currentUser.value?.display_name || currentUser.value?.username || "U").split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase()).join(""));
const nav = [
  { id: "sessions", icon: ChatDotRound }, { id: "workflows", icon: Connection }, { id: "experts", icon: MagicStick }, { id: "settings", icon: Setting },
] as const;
const elementLocale = computed(() => locale.value === "zh-CN" ? zhCn : en);
let controller: AbortController | undefined;

onMounted(() => {
  void auth.session.initialize(auth.isCallback);
  controller = new AbortController();
  getHealth(controller.signal).then(() => { online.value = true; }).catch(() => { online.value = false; });
});
onUnmounted(() => { controller?.abort(); auth.session.dispose(); });

function setLocale(value: SupportedLocale) {
  locale.value = value;
  localStorage.setItem(localeStorageKey, value);
  document.documentElement.lang = value;
}

function handleUserCommand(command: "users" | "locale" | "signout") {
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
        <nav>
          <RouterLink v-for="item in nav" :key="item.id" :to="`/${item.id}`" :class="{ 'router-link-active': route.meta.surface === item.id }" :aria-current="route.meta.surface === item.id ? 'page' : undefined" @click="mobileOpen = false"><el-icon class="nav-icon"><component :is="item.icon" /></el-icon>{{ t(`nav.${item.id}`) }}</RouterLink>
        </nav>
        <div class="sidebar-spacer"></div>
        <div class="connection-state"><el-badge is-dot :type="online === true ? 'success' : online === false ? 'danger' : 'info'" /><span>{{ online === true ? t('auth.online') : online === false ? t('auth.offline') : t('auth.checkingApi') }}</span></div>
        <div class="user-zone">
          <el-dropdown placement="top-start" trigger="click" @command="handleUserCommand">
            <button class="user-button"><el-avatar :size="34">{{ initials }}</el-avatar><span><strong>{{ currentUser?.display_name }}</strong><small>@{{ currentUser?.username }}</small></span><el-icon><MoreFilled /></el-icon></button>
            <template #dropdown><el-dropdown-menu>
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
        <RouterView :key="String(route.name)" />
      </el-main>
    </el-container>
  </el-config-provider>
</template>
