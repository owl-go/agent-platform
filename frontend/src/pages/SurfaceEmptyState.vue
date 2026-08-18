<script setup lang="ts">
defineProps<{ kicker: string; title: string; body: string; emptyTitle: string; emptyBody: string }>();
</script>

<template>
  <section class="surface route-surface">
    <header class="surface-heading reveal">
      <div><p class="kicker">{{ kicker }}</p><h1>{{ title }}</h1><p>{{ body }}</p></div>
    </header>
    <article class="empty-state reveal delay-1" aria-live="polite">
      <span class="empty-mark" aria-hidden="true">AP</span>
      <div><h2>{{ emptyTitle }}</h2><p>{{ emptyBody }}</p></div>
    </article>
  </section>
</template>
