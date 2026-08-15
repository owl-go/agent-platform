<script setup lang="ts">
const surfaces = [
  { name: "Agent Studio", detail: "Draft, validate, release" },
  { name: "Conversation", detail: "Task, run, inspect" },
  { name: "Operations", detail: "Recover, audit, control" },
];
</script>

<template>
  <main>
    <p class="eyebrow">CONTROL PLANE / PHASE 0</p>
    <section class="hero">
      <div>
        <h1>Agent work,<br />under control.</h1>
        <p class="lede">
          A governed runtime for coding agents, built around observable runs,
          isolated workspaces, and reviewable branches.
        </p>
      </div>
      <div class="status-card">
        <span class="pulse" aria-hidden="true"></span>
        <strong>Foundation ready</strong>
        <small>Runtime adapters are the first delivery seam.</small>
      </div>
    </section>
    <section class="surface-grid" aria-label="Product surfaces">
      <article v-for="(surface, index) in surfaces" :key="surface.name">
        <span>0{{ index + 1 }}</span>
        <h2>{{ surface.name }}</h2>
        <p>{{ surface.detail }}</p>
      </article>
    </section>
  </main>
</template>

