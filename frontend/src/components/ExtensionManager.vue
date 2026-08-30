<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type EnvironmentVariable, type MCPServer, type Skill } from "../api/client";

type ExtensionTab = "mcp" | "skills" | "cli";
type MCPDraft = { name: string; transport: "streamable_http" | "stdio"; url: string; runner: "npx" | "uvx"; package: string; package_version: string; argumentsText: string; environment: EnvironmentVariable[]; bearerToken: string };

const props = withDefaults(defineProps<{ selectable?: boolean; mcpServerIds?: string[]; skillIds?: string[] }>(), {
  selectable: false,
  mcpServerIds: () => [],
  skillIds: () => [],
});
const emit = defineEmits<{
  "update:mcpServerIds": [value: string[]];
  "update:skillIds": [value: string[]];
  resources: [value: { mcp: MCPServer[]; skills: Skill[] }];
  error: [];
}>();
const api = inject(platformApiKey)!;
const { t } = useI18n();
const activeTab = ref<ExtensionTab>("mcp");
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const editingMCP = ref<MCPServer>();
const showMCP = ref(false);
const mcpForm = ref<MCPDraft>(emptyMCPDraft());
const editingSkill = ref<Skill>();
const showSkill = ref(false);
const skillForm = ref({ name: "", source: "git" as "git" | "upload", git_url: "", git_ref: "main", archive: "" });
let poll: number | undefined;

onMounted(() => {
  void refresh();
  poll = window.setInterval(() => { if (mcp.value.some((item) => item.test_pending)) void refreshMCP(); }, 1500);
});
onBeforeUnmount(() => { if (poll !== undefined) window.clearInterval(poll); });

function emptyMCPDraft(): MCPDraft { return { name: "", transport: "streamable_http", url: "", runner: "npx", package: "", package_version: "", argumentsText: "", environment: [], bearerToken: "" }; }
function notifyResources() { emit("resources", { mcp: mcp.value, skills: skills.value }); }
async function refresh() {
  try { [mcp.value, skills.value] = await Promise.all([api.listMCPServers(), api.listSkills()]); notifyResources(); } catch { emit("error"); }
}
async function refreshMCP() { try { mcp.value = await api.listMCPServers(); notifyResources(); } catch { /* Preserve the last usable projection while polling. */ } }
function openNewMCP() { editingMCP.value = undefined; mcpForm.value = emptyMCPDraft(); showMCP.value = true; }
function openMCP(item: MCPServer) {
  editingMCP.value = item;
  mcpForm.value = { name: item.name, transport: item.transport, url: item.url ?? "", runner: item.runner ?? "npx", package: item.package ?? "", package_version: item.package_version ?? "", argumentsText: item.arguments.join("\n"), environment: item.environment.filter((entry) => entry.name !== "MCP_BEARER_TOKEN").map((entry) => ({ ...entry, value: "" })), bearerToken: "" };
  showMCP.value = true;
}
function addMCPEnvironment() { mcpForm.value.environment.push({ name: "", value: "", secret: false, configured: false }); }
function removeMCPEnvironment(index: number) { mcpForm.value.environment.splice(index, 1); }
function mcpPayload(): Record<string, unknown> {
  const argumentsList = mcpForm.value.argumentsText.split("\n").map((value) => value.trim()).filter(Boolean);
  const environment = mcpForm.value.environment.map((entry) => ({ ...entry, value: entry.value || undefined }));
  if (mcpForm.value.transport === "streamable_http") {
    const existingBearer = editingMCP.value?.environment.find((entry) => entry.name === "MCP_BEARER_TOKEN");
    if (mcpForm.value.bearerToken || existingBearer?.configured) environment.push({ name: "MCP_BEARER_TOKEN", value: mcpForm.value.bearerToken || undefined, secret: true, configured: Boolean(existingBearer?.configured) });
    return { name: mcpForm.value.name, transport: "streamable_http", url: mcpForm.value.url, arguments: [], environment };
  }
  return { name: mcpForm.value.name, transport: "stdio", runner: mcpForm.value.runner, package: mcpForm.value.package, package_version: mcpForm.value.package_version, arguments: argumentsList, environment };
}
async function saveMCP() {
  try {
    if (editingMCP.value) await api.updateMCPServer(editingMCP.value.id, mcpPayload(), editingMCP.value.version);
    else await api.createMCPServer(mcpPayload());
    showMCP.value = false;
    await refresh();
  } catch { emit("error"); }
}
async function removeMCP(item: MCPServer) {
  if (!confirm(`${t("common.delete")} ${item.name}?`)) return;
  try { await api.deleteMCPServer(item.id); emit("update:mcpServerIds", props.mcpServerIds.filter((id) => id !== item.id)); await refresh(); } catch { emit("error"); }
}
async function testMCP(item: MCPServer) {
  try { const updated = await api.testMCPServer(item.id); mcp.value = mcp.value.map((entry) => entry.id === updated.id ? updated : entry); notifyResources(); } catch { emit("error"); }
}
function toggleMCP(item: MCPServer, checked: boolean) {
  if (!item.tested) return;
  emit("update:mcpServerIds", checked ? [...new Set([...props.mcpServerIds, item.id])] : props.mcpServerIds.filter((id) => id !== item.id));
}
function openNewSkill() { editingSkill.value = undefined; skillForm.value = { name: "", source: "git", git_url: "", git_ref: "main", archive: "" }; showSkill.value = true; }
function openSkill(item: Skill) { editingSkill.value = item; skillForm.value = { name: item.name, source: item.source, git_url: item.git_url ?? "", git_ref: item.git_ref ?? "main", archive: "" }; showSkill.value = true; }
async function selectSkillArchive(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file || file.size > 10 * 1024 * 1024) { if (file) emit("error"); return; }
  skillForm.value.archive = await fileToBase64(file);
}
async function saveSkill() {
  try {
    let saved: Skill;
    if (editingSkill.value) {
      const input = editingSkill.value.source === "git" ? { git_ref: skillForm.value.git_ref } : { archive: skillForm.value.archive };
      saved = await api.updateSkill(editingSkill.value.id, input, editingSkill.value.version);
    } else if (skillForm.value.source === "git") saved = await api.createGitSkill({ name: skillForm.value.name, git_url: skillForm.value.git_url, git_ref: skillForm.value.git_ref });
    else saved = await api.createUploadSkill({ name: skillForm.value.name, archive: skillForm.value.archive });
    if (props.selectable && !editingSkill.value) emit("update:skillIds", [...new Set([...props.skillIds, saved.id])]);
    showSkill.value = false;
    await refresh();
  } catch { emit("error"); }
}
async function removeSkill(item: Skill) {
  if (!confirm(`${t("common.delete")} ${item.name}?`)) return;
  try { await api.deleteSkill(item.id); emit("update:skillIds", props.skillIds.filter((id) => id !== item.id)); await refresh(); } catch { emit("error"); }
}
function toggleSkill(item: Skill, checked: boolean) { emit("update:skillIds", checked ? [...new Set([...props.skillIds, item.id])] : props.skillIds.filter((id) => id !== item.id)); }
async function fileToBase64(file: File): Promise<string> {
  const bytes = new Uint8Array(await file.arrayBuffer()); let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  return btoa(binary);
}
</script>

<template>
  <div class="extension-manager" :class="{ selectable }">
    <nav class="subtabs"><button type="button" :class="{ active: activeTab === 'mcp' }" @click="activeTab = 'mcp'">{{ t("settings.mcp") }}</button><button type="button" :class="{ active: activeTab === 'skills' }" @click="activeTab = 'skills'">{{ t("settings.skills") }}</button><button type="button" :class="{ active: activeTab === 'cli' }" @click="activeTab = 'cli'">{{ t("settings.cli") }}</button></nav>
    <div v-if="activeTab === 'mcp'"><button type="button" class="button primary compact-action" @click="openNewMCP">＋ MCP</button><div class="resource-list"><article v-for="item in mcp" :key="item.id"><label v-if="selectable" class="extension-choice" :title="item.tested ? '' : t('experts.testRequired')"><input type="checkbox" :checked="mcpServerIds.includes(item.id)" :disabled="!item.tested" @change="toggleMCP(item, ($event.target as HTMLInputElement).checked)"></label><span class="resource-mark">MCP</span><div><strong>{{ item.name }}</strong><p>{{ item.transport }} · {{ item.url || `${item.runner} ${item.package}@${item.package_version}` }}<template v-if="item.test_error"> · {{ item.test_error }}</template></p></div><span class="safe-chip" :class="{ unsafe: !item.tested }">{{ item.test_pending ? t("settings.testPending") : item.tested ? t("settings.tested") : t("settings.testRequired") }}</span><button type="button" :aria-label="t('common.retry')" :disabled="item.test_pending" @click="testMCP(item)">↻</button><button type="button" :aria-label="t('common.edit')" @click="openMCP(item)">✎</button><button type="button" :aria-label="t('common.delete')" @click="removeMCP(item)">×</button></article></div></div>
    <div v-if="activeTab === 'skills'"><button type="button" class="button primary compact-action" @click="openNewSkill">＋ Skill</button><div class="resource-list"><article v-for="item in skills" :key="item.id"><label v-if="selectable" class="extension-choice"><input type="checkbox" :checked="skillIds.includes(item.id)" @change="toggleSkill(item, ($event.target as HTMLInputElement).checked)"></label><span class="resource-mark">SK</span><div><strong>{{ item.name }}</strong><p>{{ item.git_url || item.source }} · {{ item.sha256.slice(0, 12) }}</p></div><button type="button" :aria-label="t('common.edit')" @click="openSkill(item)">✎</button><button type="button" :aria-label="t('common.delete')" @click="removeSkill(item)">×</button></article></div></div>
    <div v-if="activeTab === 'cli'" class="coming-soon"><span>⌘</span><p>{{ t("common.comingSoon") }}</p></div>
  </div>
  <Teleport to="body">
    <div v-if="showMCP" class="modal-layer" @click.self="showMCP = false"><form class="modal-card" @submit.prevent="saveMCP"><p class="eyebrow">MCP SERVER</p><h2>{{ editingMCP ? t("common.edit") : t("common.new") }} MCP</h2><label>{{ t("common.name") }}<input v-model="mcpForm.name" required></label><label>{{ t("settings.transport") }}<select v-model="mcpForm.transport"><option value="streamable_http">Streamable HTTP</option><option value="stdio">stdio</option></select></label><template v-if="mcpForm.transport === 'streamable_http'"><label>URL<input v-model="mcpForm.url" type="url" required></label><label>{{ t("settings.bearerToken") }}<input v-model="mcpForm.bearerToken" type="password" :placeholder="editingMCP ? t('settings.keepSecret') : t('settings.optional')"></label></template><template v-else><label>Runner<select v-model="mcpForm.runner"><option value="npx">npx</option><option value="uvx">uvx</option></select></label><label>Package<input v-model="mcpForm.package" required></label><label>{{ t("settings.fixedVersion") }}<input v-model="mcpForm.package_version" required placeholder="1.2.3"></label><label>{{ t("settings.arguments") }}<textarea v-model="mcpForm.argumentsText" rows="4" :placeholder="t('settings.onePerLine')"></textarea></label></template><div><div v-for="(variable, index) in mcpForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><button type="button" class="text-button" @click="removeMCPEnvironment(index)">×</button></div><button type="button" class="button ghost" @click="addMCPEnvironment">＋ {{ t("settings.environment") }}</button></div><div class="modal-actions"><button type="button" class="button ghost" @click="showMCP = false">{{ t("common.cancel") }}</button><button class="button primary">{{ t("common.save") }}</button></div></form></div>
    <div v-if="showSkill" class="modal-layer" @click.self="showSkill = false"><form class="modal-card" @submit.prevent="saveSkill"><p class="eyebrow">SKILL</p><h2>{{ editingSkill ? t("common.edit") : t("common.new") }} Skill</h2><label>{{ t("common.name") }}<input v-model="skillForm.name" :disabled="Boolean(editingSkill)" required></label><label>{{ t("settings.source") }}<select v-model="skillForm.source" :disabled="Boolean(editingSkill)"><option value="git">Git</option><option value="upload">ZIP</option></select></label><template v-if="skillForm.source === 'git'"><label>Git URL<input v-model="skillForm.git_url" type="url" :disabled="Boolean(editingSkill)" required placeholder="https://github.com/…"></label><label>Git ref<input v-model="skillForm.git_ref" required></label></template><label v-else>ZIP<input type="file" accept=".zip,application/zip" :required="Boolean(editingSkill) || !skillForm.archive" @change="selectSkillArchive"></label><p class="muted">{{ t("settings.skillHint") }}</p><div class="modal-actions"><button type="button" class="button ghost" @click="showSkill = false">{{ t("common.cancel") }}</button><button class="button primary">{{ t("common.save") }}</button></div></form></div>
  </Teleport>
</template>
