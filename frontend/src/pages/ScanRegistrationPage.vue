<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { localeStorageKey } from "../i18n";

const { t, locale } = useI18n();
const id = new URLSearchParams(window.location.search).get("id") ?? "";
const qrURL = ref("");
const loading = ref(true);
const status = ref<"waiting" | "completing" | "expired" | "completionFailed">("waiting");
const controller = new AbortController();
let timer: ReturnType<typeof setTimeout> | undefined;
let deadline = Date.now() + 300_000;
const endpoint = "/api/v1/registration/wechat_official/";
function setLanguage() { locale.value = locale.value === "en-US" ? "zh-CN" : "en-US"; localStorage.setItem(localeStorageKey, locale.value); document.documentElement.lang = locale.value; }
async function poll() {
  if (controller.signal.aborted) return;
  if (!/^[A-Za-z0-9_-]{43}$/.test(id) || Date.now() >= deadline) { loading.value = false; status.value = "expired"; return; }
  try {
    const response = await fetch(`${endpoint}status?id=${encodeURIComponent(id)}`, { signal: controller.signal, cache: "no-store", credentials: "same-origin" });
    if (!response.ok) { loading.value = false; status.value = response.status === 410 ? "expired" : "completionFailed"; return; }
    const result = await response.json() as { status: string; qr_url: string; expires_at: string };
    const qr = new URL(result.qr_url);
    if (qr.origin !== "https://mp.weixin.qq.com" || qr.pathname !== "/cgi-bin/showqrcode") throw new Error("Invalid QR");
    const expires = Date.parse(result.expires_at);
    if (!Number.isFinite(expires)) throw new Error("Invalid expiry");
    qrURL.value = qr.href; deadline = expires; loading.value = false;
    if (result.status === "verified") {
      status.value = "completing";
      const completed = await fetch(`${endpoint}complete?id=${encodeURIComponent(id)}`, { method: "POST", signal: controller.signal, credentials: "same-origin", headers: { "X-Registration-Request": "1" } });
      if (!completed.ok) throw new Error("Completion failed");
      const destination = await completed.json() as { redirect: string };
      // Redirects are generated from the server's exact Keycloak callback.
      const redirect = new URL(destination.redirect);
      if (redirect.protocol !== "https:") throw new Error("Invalid redirect");
      window.location.replace(redirect.href); return;
    }
    if (result.status !== "waiting") throw new Error("Consumed registration");
    timer = setTimeout(poll, 2000);
  } catch { if (!controller.signal.aborted) { loading.value = false; status.value = "completionFailed"; } }
}
onMounted(poll);
onUnmounted(() => { controller.abort(); clearTimeout(timer); });
</script>

<template>
  <main class="auth-screen">
    <el-card class="auth-card registration-card" shadow="never">
      <div class="auth-brand"><span class="logo-mark" aria-hidden="true">AW</span><strong>{{ t('product') }}</strong></div>
      <h1>{{ t('registration.scanTitle') }}</h1><p>{{ t('registration.scanHint') }}</p>
      <el-skeleton v-if="loading" :rows="4" animated />
      <img v-if="qrURL && status === 'waiting'" class="registration-qr" :src="qrURL" :alt="t('registration.qrAlt')" referrerpolicy="no-referrer">
      <p role="status" aria-live="polite">{{ t(`registration.${status}`) }}</p>
      <div class="registration-actions"><a href="/">{{ t('registration.back') }}</a><el-button text @click="setLanguage">{{ t('registration.language') }}</el-button></div>
    </el-card>
  </main>
</template>

<style scoped>
.registration-card :deep(.el-card__body) { gap: var(--aw-space-4); }
.registration-card h1 { font-size: var(--aw-font-size-title); margin: 0; }
.registration-card p { margin: 0; color: var(--aw-n7); line-height: 1.6; overflow-wrap: anywhere; }
.registration-qr { display: block; width: 240px; max-width: 100%; aspect-ratio: 1; margin: var(--aw-space-4) auto; }
.registration-actions { display: flex; justify-content: space-between; align-items: center; gap: var(--aw-space-3); flex-wrap: wrap; }
</style>
