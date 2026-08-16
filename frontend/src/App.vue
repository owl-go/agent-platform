<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { getHealth } from "./api/client";

type Surface = "studio" | "workspace" | "operations";

const activeSurface = ref<Surface>("workspace");
const health = ref<"checking" | "online" | "offline">("checking");
const mobileNavOpen = ref(false);
const selectedRun = ref("run-1842");
const now = ref(new Date());

const navigation: Array<{ id: Surface; label: string; short: string }> = [
  { id: "studio", label: "Agent Studio", short: "AS" },
  { id: "workspace", label: "Workspace", short: "CW" },
  { id: "operations", label: "Operations", short: "OC" },
];

const agents = [
  { name: "Code Steward", runtime: "Codex CLI", version: "v12", status: "Released", tone: "coral" },
  { name: "Migration Pilot", runtime: "Claude Code", version: "v7", status: "Draft", tone: "yellow" },
  { name: "Incident Cartographer", runtime: "Hermes", version: "v4", status: "Released", tone: "blue" },
];

const timeline = [
  { time: "10:42:03", type: "PLAN", title: "Repository context mapped", detail: "Identified the collaboration aggregate and three transaction boundaries." },
  { time: "10:42:18", type: "COMMAND", title: "go test ./internal/collaboration/...", detail: "12 packages passed in 4.8s", state: "passed" },
  { time: "10:43:09", type: "CHANGE", title: "5 files changed", detail: "+286  -41  · service, repository, integration tests" },
  { time: "10:44:27", type: "DECISION", title: "Your review is needed", detail: "The migration adds a unique active-lease constraint.", state: "attention" },
];

const runs = [
  { id: "run-1842", task: "Harden workspace lease", agent: "Code Steward", runtime: "Codex", state: "Waiting", age: "2m", cost: "$0.84" },
  { id: "run-1841", task: "Trace checkout latency", agent: "Incident Cartographer", runtime: "Hermes", state: "Running", age: "8m", cost: "$1.12" },
  { id: "run-1839", task: "Upgrade GORM models", agent: "Migration Pilot", runtime: "Claude", state: "Passed", age: "14m", cost: "$0.62" },
  { id: "run-1837", task: "Remove stale artifacts", agent: "Code Steward", runtime: "Codex", state: "Failed", age: "31m", cost: "$0.28" },
];

const selectedRunData = computed(() => runs.find((run) => run.id === selectedRun.value) ?? runs[0]);
const activeLabel = computed(() => navigation.find((item) => item.id === activeSurface.value)?.label);
const formattedTime = computed(() => now.value.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" }));

let clock: number | undefined;

onMounted(() => {
  const controller = new AbortController();
  getHealth(controller.signal)
    .then(() => { health.value = "online"; })
    .catch(() => { health.value = "offline"; });
  clock = window.setInterval(() => { now.value = new Date(); }, 30_000);
});

onUnmounted(() => {
  if (clock !== undefined) window.clearInterval(clock);
});

function selectSurface(surface: Surface) {
  activeSurface.value = surface;
  mobileNavOpen.value = false;
}
</script>

<template>
  <div class="shell">
    <aside class="rail" :class="{ open: mobileNavOpen }">
      <button class="brand" aria-label="Agent Platform home" @click="selectSurface('workspace')">
        <span class="brand-mark"><i></i><i></i><i></i></span>
        <span class="brand-copy"><strong>AP</strong><small>CONTROL</small></span>
      </button>

      <nav aria-label="Product surfaces">
        <button
          v-for="item in navigation"
          :key="item.id"
          :class="{ active: activeSurface === item.id }"
          @click="selectSurface(item.id)"
        >
          <span class="nav-code">{{ item.short }}</span>
          <span>{{ item.label }}</span>
        </button>
      </nav>

      <div class="rail-foot">
        <span class="environment">SG / DEV</span>
        <button aria-label="Platform settings" class="settings-button">SETTINGS</button>
      </div>
    </aside>

    <div v-if="mobileNavOpen" class="nav-scrim" @click="mobileNavOpen = false"></div>

    <main>
      <header class="topbar">
        <button class="menu-button" aria-label="Open navigation" @click="mobileNavOpen = true">
          <span></span><span></span>
        </button>
        <div class="breadcrumb"><span>AGENT PLATFORM</span><b>/</b><strong>{{ activeLabel }}</strong></div>
        <div class="platform-state">
          <span class="health-dot" :class="health"></span>
          <span>API {{ health }}</span>
          <time>{{ formattedTime }}</time>
          <span class="avatar">FK</span>
        </div>
      </header>

      <section v-if="activeSurface === 'studio'" class="surface studio-surface">
        <div class="surface-heading reveal">
          <div>
            <p class="kicker">BUILD / VALIDATE / RELEASE</p>
            <h1>Agent Studio</h1>
            <p>Shape agents into governed, immutable releases.</p>
          </div>
          <button class="primary-action"><span>+</span> New agent draft</button>
        </div>

        <div class="metric-strip reveal delay-1">
          <article><span>RELEASED</span><strong>08</strong><small>2 updated this week</small></article>
          <article><span>DRAFTS</span><strong>03</strong><small>1 needs validation</small></article>
          <article><span>RUNTIMES</span><strong>04</strong><small>All adapters registered</small></article>
          <article class="config-metric"><span>EXTERNAL CONFIG</span><strong>DEFERRED</strong><small>Credentials intentionally skipped</small></article>
        </div>

        <div class="studio-layout reveal delay-2">
          <section class="panel agent-catalog">
            <div class="panel-title"><div><span>CATALOG</span><h2>Your agents</h2></div><button class="text-button">View all 11</button></div>
            <article v-for="agent in agents" :key="agent.name" class="agent-row">
              <span class="agent-monogram" :class="agent.tone">{{ agent.name.split(' ').map((word) => word[0]).join('') }}</span>
              <div><h3>{{ agent.name }}</h3><p>{{ agent.runtime }} · {{ agent.version }}</p></div>
              <span class="status-pill" :class="agent.status.toLowerCase()">{{ agent.status }}</span>
              <button class="row-arrow" :aria-label="`Open ${agent.name}`">↗</button>
            </article>
          </section>

          <aside class="panel release-path">
            <div class="panel-title"><div><span>RELEASE PATH</span><h2>Draft quality</h2></div><b>03 / 04</b></div>
            <div class="release-score"><span>75</span><small>%</small></div>
            <ol>
              <li class="done"><span>01</span><div><strong>Definition</strong><small>Instructions and limits</small></div></li>
              <li class="done"><span>02</span><div><strong>Bindings</strong><small>Runtime and repository</small></div></li>
              <li class="done"><span>03</span><div><strong>Validation</strong><small>Quality gates passed</small></div></li>
              <li><span>04</span><div><strong>Risk approval</strong><small>Builder review required</small></div></li>
            </ol>
          </aside>
        </div>
      </section>

      <section v-else-if="activeSurface === 'workspace'" class="surface workspace-surface">
        <div class="task-bar reveal">
          <div class="task-identity">
            <span class="task-number">TASK / 042</span>
            <div><h1>Harden workspace lease</h1><p>agent-platform · review/task-042</p></div>
          </div>
          <div class="task-controls">
            <span class="run-state"><i></i> Waiting for you</span>
            <button class="ghost-action">•••</button>
            <button class="primary-action">Continue task <span>↗</span></button>
          </div>
        </div>

        <div class="workspace-grid reveal delay-1">
          <aside class="context-panel">
            <div class="context-section">
              <span class="section-label">SESSION</span>
              <dl><div><dt>Agent</dt><dd>Code Steward v12</dd></div><div><dt>Runtime</dt><dd>Codex CLI</dd></div><div><dt>Model</dt><dd>Configured default</dd></div><div><dt>Runs</dt><dd>04 / 50</dd></div></dl>
            </div>
            <div class="context-section branch-block">
              <span class="section-label">WORKSPACE</span>
              <p class="branch-name"><i></i> review/task-042</p>
              <div class="diff-numbers"><strong>5</strong><span>files</span><b>+286</b><em>-41</em></div>
              <button class="wide-button">Inspect full diff <span>↗</span></button>
            </div>
            <div class="context-section spend-block">
              <span class="section-label">MODEL BUDGET</span>
              <div class="budget-line"><strong>$3.18</strong><span>of $8.00</span></div>
              <div class="meter"><i></i></div>
              <small>39.8% consumed across 4 runs</small>
            </div>
          </aside>

          <section class="conversation-panel">
            <div class="conversation-intro">
              <span class="speaker user">YOU</span>
              <div><p>Make workspace writes serial per session and prove the lease recovers after a worker disappears.</p><time>10:41</time></div>
            </div>
            <div class="agent-response">
              <div class="response-head"><span class="speaker agent">CS</span><div><strong>Code Steward</strong><small>Run 04 · 3m 12s</small></div><span class="runtime-tag">CODEX CLI</span></div>
              <p>I added a session-scoped workspace lease beside the existing run lease. Claim, renew, finish, and reconciliation now update both in one transaction.</p>
              <div class="timeline">
                <article v-for="event in timeline" :key="event.time" :class="event.state">
                  <time>{{ event.time }}</time><span class="event-mark"></span>
                  <div><small>{{ event.type }}</small><strong>{{ event.title }}</strong><p>{{ event.detail }}</p></div>
                </article>
              </div>
              <div class="decision-card">
                <div><span>DECISION / 01</span><h3>Accept the lease constraint?</h3><p>This migration prevents two active write leases for one session at the database boundary.</p></div>
                <div class="decision-actions"><button class="reject-action">Request change</button><button class="approve-action">Accept &amp; continue</button></div>
              </div>
            </div>
            <div class="composer">
              <button aria-label="Attach context">+</button><span>Reply with a decision or new direction...</span><kbd>⌘ ↵</kbd>
            </div>
          </section>
        </div>
      </section>

      <section v-else class="surface operations-surface">
        <div class="surface-heading reveal">
          <div><p class="kicker">OBSERVE / INTERVENE / RECOVER</p><h1>Operations Console</h1><p>Current execution truth, without the noise.</p></div>
          <div class="ops-summary"><span><i class="running-dot"></i> 1 RUNNING</span><span>1 NEEDS ACTION</span></div>
        </div>

        <div class="ops-toolbar reveal delay-1">
          <div class="filter-group"><button class="active">All runs <b>24</b></button><button>Active <b>02</b></button><button>Failed <b>01</b></button></div>
          <div class="toolbar-actions"><button>Last 24 hours⌄</button><button>Filter +</button></div>
        </div>

        <div class="operations-grid reveal delay-2">
          <section class="run-table" aria-label="Recent runs">
            <div class="table-head"><span>RUN / TASK</span><span>AGENT</span><span>RUNTIME</span><span>STATE</span><span>AGE</span><span>COST</span></div>
            <button v-for="run in runs" :key="run.id" :class="{ selected: selectedRun === run.id }" @click="selectedRun = run.id">
              <span><b>{{ run.id }}</b><small>{{ run.task }}</small></span><span>{{ run.agent }}</span><span>{{ run.runtime }}</span><span><i class="state-dot" :class="run.state.toLowerCase()"></i>{{ run.state }}</span><span>{{ run.age }}</span><span>{{ run.cost }}</span>
            </button>
          </section>

          <aside class="run-inspector">
            <div class="inspector-head"><span>RUN INSPECTOR</span><button>×</button></div>
            <div class="inspector-title"><small>{{ selectedRunData.id }}</small><h2>{{ selectedRunData.task }}</h2><span class="status-pill waiting">{{ selectedRunData.state }}</span></div>
            <div class="attempt-map"><span class="complete">CLAIMED</span><i></i><span class="complete">RUNNING</span><i></i><span class="current">WAITING</span></div>
            <dl class="inspector-data"><div><dt>Attempt</dt><dd>01</dd></div><div><dt>Worker</dt><dd>worker-sg-02</dd></div><div><dt>Image</dt><dd>codex@sha256:9c2…</dd></div><div><dt>Sandbox</dt><dd>gVisor / running</dd></div><div><dt>Heartbeat</dt><dd>8s ago</dd></div><div><dt>Model cost</dt><dd>{{ selectedRunData.cost }}</dd></div></dl>
            <div class="operator-note"><span>OPERATOR NOTE</span><p>Run is healthy. Waiting on a user decision; no intervention required.</p></div>
            <div class="operator-actions"><button>Interrupt</button><button class="danger-button">Kill run</button></div>
          </aside>
        </div>
      </section>

      <footer class="preview-notice">
        <span>INTERFACE PREVIEW</span>
        <p>External identity, Git, model, and object storage connections are intentionally deferred.</p>
      </footer>
    </main>
  </div>
</template>
