<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref } from "vue";
import { RouterLink, RouterView, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { getHealth } from "./api/client";
import { authContextKey } from "./auth/session";
import { localeStorageKey, type SupportedLocale } from "./i18n";

const auth = inject(authContextKey);
if (!auth) throw new Error("Authentication context is required");
const route = useRoute();
const { locale, t } = useI18n();
const authState = auth.session.state;
const currentUser = computed(() => authState.value.kind === "authenticated" ? authState.value.currentUser : undefined);
const online = ref<boolean | undefined>();
const mobileOpen = ref(false);
const userMenuOpen = ref(false);
const initials = computed(() => (currentUser.value?.display_name || currentUser.value?.username || "U").split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase()).join(""));
const nav = [
  { id: "sessions", icon: "◌" }, { id: "workflows", icon: "⌁" }, { id: "experts", icon: "✦" }, { id: "settings", icon: "⚙" },
] as const;
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
</script>

<template>
  <section v-if="authState.kind === 'checking'" class="auth-screen">
    <div class="auth-orbit"><i></i><i></i><i></i></div><h1>{{ t('auth.checking') }}</h1>
  </section>
  <section v-else-if="authState.kind === 'unauthenticated'" class="auth-screen">
    <div class="auth-card"><div class="logo-mark">AW</div><p class="eyebrow">{{ t('product') }}</p><h1>{{ t('auth.required') }}</h1><p>{{ t('auth.body') }}</p><button class="button primary wide" @click="auth.session.signIn()">{{ t('auth.signIn') }} <span>→</span></button></div>
  </section>
  <section v-else-if="authState.kind === 'error'" class="auth-screen"><div class="auth-card error-card"><div class="logo-mark">!</div><h1>{{ t('auth.unavailable') }}</h1><p>{{ authState.message }}</p></div></section>
  <div v-else class="app-shell">
    <aside class="sidebar" :class="{ open: mobileOpen }">
      <RouterLink to="/sessions" class="product-lockup" @click="mobileOpen = false"><span class="logo-mark">AW</span><span><strong>Agent</strong><small>Workspace</small></span></RouterLink>
      <button class="new-session" @click="$router.push('/sessions?new=1'); mobileOpen = false"><span>＋</span>{{ t('sessions.new') }}</button>
      <nav>
        <RouterLink v-for="item in nav" :key="item.id" :to="`/${item.id}`" @click="mobileOpen = false"><span class="nav-icon">{{ item.icon }}</span>{{ t(`nav.${item.id}`) }}</RouterLink>
      </nav>
      <div class="sidebar-spacer"></div>
      <div class="connection-state"><i :class="online === true ? 'online' : online === false ? 'offline' : ''"></i><span>{{ online === true ? t('auth.online') : online === false ? t('auth.offline') : t('auth.checkingApi') }}</span></div>
      <div class="user-zone">
        <button class="user-button" @click="userMenuOpen = !userMenuOpen"><span class="avatar">{{ initials }}</span><span><strong>{{ currentUser?.display_name }}</strong><small>@{{ currentUser?.username }}</small></span><b>•••</b></button>
        <div v-if="userMenuOpen" class="user-menu">
          <RouterLink v-if="currentUser?.administrator" to="/admin/users" @click="userMenuOpen = false">{{ t('nav.users') }}</RouterLink>
          <button @click="setLocale(locale === 'zh-CN' ? 'en-US' : 'zh-CN')">{{ locale === 'zh-CN' ? 'English' : '中文' }}</button>
          <button class="danger-text" @click="auth.session.signOut()">{{ t('auth.signOut') }}</button>
        </div>
      </div>
    </aside>
    <button v-if="mobileOpen" class="scrim" @click="mobileOpen = false"></button>
    <main class="main-stage">
      <header class="mobile-header"><button @click="mobileOpen = true">☰</button><strong>{{ t('product') }}</strong><span class="avatar">{{ initials }}</span></header>
      <RouterView :key="String(route.name)" />
    </main>
  </div>
</template>
