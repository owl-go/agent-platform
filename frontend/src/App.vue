<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { getHealth } from "./api/client";
import { authContextKey } from "./auth/session";
import { localeStorageKey, type SupportedLocale } from "./i18n";
import type { Surface } from "./router";
import { canAccessSurface, selectAccessibleTeam } from "./team/access";

const auth = inject(authContextKey);
if (!auth) throw new Error("Authentication context is required");

const router = useRouter();
const route = useRoute();
const { locale, t, d } = useI18n();
const authState = auth.session.state;
const currentUser = computed(() => authState.value.kind === "authenticated" ? authState.value.currentUser : undefined);
const activeTeam = computed(() => currentUser.value ? selectAccessibleTeam(currentUser.value, route.query.team) : undefined);
const currentSurface = computed<Surface>(() => {
  const name = String(route.name ?? "workspace");
  return name === "studio" || name === "operations" ? name : "workspace";
});
const authorized = computed(() => Boolean(currentUser.value && activeTeam.value && canAccessSurface(currentUser.value, activeTeam.value.id ?? "", currentSurface.value)));
const health = ref<"checking" | "online" | "offline">("checking");
const mobileNavOpen = ref(false);
const now = ref(new Date());
const userInitials = computed(() => {
  const displayName = currentUser.value?.display_name?.trim() ?? "";
  return displayName.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join("") || "U";
});
const navigation: Array<{ id: Surface; short: string }> = [
  { id: "studio", short: "AS" },
  { id: "workspace", short: "CW" },
  { id: "operations", short: "OC" },
];
let clock: number | undefined;

watch([currentUser, activeTeam], async ([user, team]) => {
  if (!user || !team?.id || route.query.team === team.id) return;
  await router.replace({ name: currentSurface.value, query: { ...route.query, team: team.id } });
}, { immediate: true });

onMounted(() => {
  void auth.session.initialize(auth.isCallback);
  const controller = new AbortController();
  getHealth(controller.signal).then(() => { health.value = "online"; }).catch(() => { health.value = "offline"; });
  clock = window.setInterval(() => { now.value = new Date(); }, 30_000);
});

onUnmounted(() => {
  auth.session.dispose();
  if (clock !== undefined) window.clearInterval(clock);
});

async function changeTeam(event: Event) {
  const teamID = (event.target as HTMLSelectElement).value;
  await router.push({ name: currentSurface.value, query: { team: teamID } });
}

function changeLocale(event: Event) {
  const selected = (event.target as HTMLSelectElement).value as SupportedLocale;
  locale.value = selected;
  localStorage.setItem(localeStorageKey, selected);
  document.documentElement.lang = selected;
}

function routeFor(surface: Surface) {
  return { name: surface, query: activeTeam.value?.id ? { team: activeTeam.value.id } : {} };
}
</script>

<template>
  <section v-if="authState.kind === 'checking'" class="auth-screen" aria-live="polite">
    <div class="auth-card"><span class="auth-code">{{ t('auth.checkingCode') }}</span><h1>{{ t('auth.checkingTitle') }}</h1><p>{{ t('auth.checkingBody') }}</p></div>
  </section>
  <section v-else-if="authState.kind === 'unauthenticated'" class="auth-screen" data-testid="sign-in">
    <div class="auth-card"><span class="auth-code">{{ t('auth.requiredCode') }}</span><h1>{{ t('auth.requiredTitle') }}</h1><p>{{ t(authState.reason === 'expired' ? 'auth.expiredBody' : 'auth.requiredBody') }}</p><button class="primary-action auth-action" data-testid="sign-in-button" @click="auth.session.signIn()">{{ t('auth.signIn') }} <span>↗</span></button></div>
  </section>
  <section v-else-if="authState.kind === 'error'" class="auth-screen" role="alert">
    <div class="auth-card auth-error"><span class="auth-code">{{ t('auth.unavailableCode') }}</span><h1>{{ t('auth.unavailableTitle') }}</h1><p>{{ authState.message }}</p></div>
  </section>

  <div v-else class="shell">
    <aside class="rail" :class="{ open: mobileNavOpen }">
      <RouterLink class="brand" :to="routeFor('workspace')" aria-label="Agent Platform home"><span class="brand-mark"><i></i><i></i><i></i></span><span class="brand-copy"><strong>AP</strong><small>CONTROL</small></span></RouterLink>
      <nav :aria-label="t('navigation.label')">
        <RouterLink v-for="item in navigation" :key="item.id" :to="routeFor(item.id)" :data-testid="`nav-${item.id}`" :class="{ unavailable: currentUser && activeTeam && !canAccessSurface(currentUser, activeTeam.id ?? '', item.id) }" @click="mobileNavOpen = false"><span class="nav-code">{{ item.short }}</span><span>{{ t(`navigation.${item.id}`) }}</span></RouterLink>
      </nav>
      <div class="rail-foot"><span class="environment">{{ currentUser?.organization?.slug ?? '—' }}</span><button class="settings-button">{{ t('navigation.settings') }}</button></div>
    </aside>
    <button v-if="mobileNavOpen" class="nav-scrim" aria-label="Close navigation" @click="mobileNavOpen = false"></button>

    <main>
      <header class="topbar">
        <button class="menu-button" :aria-label="t('navigation.open')" @click="mobileNavOpen = true"><span></span><span></span></button>
        <div class="breadcrumb"><span>{{ t('shell.product') }}</span><b>/</b><strong>{{ t(`navigation.${currentSurface}`) }}</strong></div>
        <div class="context-controls">
          <label><span>{{ t('shell.team') }}</span><select data-testid="team-select" :value="activeTeam?.id" :disabled="!currentUser?.teams?.length" @change="changeTeam"><option v-for="team in currentUser?.teams ?? []" :key="team.id" :value="team.id">{{ team.name }}</option></select></label>
          <label><span>{{ t('shell.language') }}</span><select data-testid="locale-select" :value="locale" @change="changeLocale"><option value="zh-CN">{{ t('locale.zh') }}</option><option value="en-US">{{ t('locale.en') }}</option></select></label>
        </div>
        <div class="platform-state"><span class="health-dot" :class="health"></span><span>{{ t('shell.api', { status: t(`health.${health}`) }) }}</span><time>{{ d(now, { hour: '2-digit', minute: '2-digit' }) }}</time><div class="current-user" data-testid="current-user"><strong>{{ currentUser?.display_name }}</strong><small>{{ t('shell.userScope', { organization: currentUser?.organization?.name, team: activeTeam?.name ?? '—' }) }}</small></div><button class="avatar" data-testid="sign-out" :aria-label="t('auth.signOut', { name: currentUser?.display_name })" @click="auth.session.signOut()">{{ userInitials }}</button></div>
      </header>

      <section v-if="!activeTeam" class="surface access-state" data-testid="no-team"><h1>{{ t('access.noTeamTitle') }}</h1><p>{{ t('access.noTeamBody') }}</p></section>
      <section v-else-if="!authorized" class="surface access-state" data-testid="access-denied"><h1>{{ t('access.deniedTitle') }}</h1><p>{{ t('access.deniedBody') }}</p></section>
      <RouterView v-else :key="`${currentUser?.user_id}:${activeTeam.id}:${currentSurface}`" />
    </main>
  </div>
</template>
