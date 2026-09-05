<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { platformApiKey, type CLIConnectorDefinition, type CLIConnectorDefinitionInput, type CLIConnectorEnablement, type EnvironmentVariable, type MCPServer, type ResourceDeletionImpact, type Skill } from "../api/client";
import { authContextKey } from "../auth/session";
import ConfirmDialog from "./ConfirmDialog.vue";

type ResourceTab = "mcp" | "skills";
type MCPDraft = { name: string; transport: "streamable_http" | "stdio"; url: string; runner: "npx" | "uvx"; package: string; package_version: string; argumentsText: string; environment: EnvironmentVariable[]; bearerToken: string };

const props = withDefaults(defineProps<{ selectable?: boolean; initialTab?: ResourceTab; mcpServerIds?: string[]; skillIds?: string[] }>(), {
  selectable: false,
  initialTab: "mcp",
  mcpServerIds: () => [],
  skillIds: () => [],
});
const emit = defineEmits<{
  "update:mcpServerIds": [value: string[]];
  "update:skillIds": [value: string[]];
  resources: [value: { mcp: MCPServer[]; skills: Skill[] }];
  tabChange: [value: ResourceTab];
  error: [];
}>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const { t } = useI18n();
const canManageCLI = computed(() => auth?.session.state.value.kind === "authenticated" && auth.session.state.value.currentUser.administrator);
const activeTab = ref<ResourceTab>(props.initialTab);
const mcp = ref<MCPServer[]>([]);
const skills = ref<Skill[]>([]);
const cliDefinitions = ref<CLIConnectorDefinition[]>([]);
const cliEnablements = ref<CLIConnectorEnablement[]>([]);
const showCLI = ref(false);
const cliForm = ref<CLIConnectorDefinitionInput>({ name: t("resources.feishuCLI"), npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-QARcHz96pfEzzRZdjXene5h9fJ46lCu5q2TWx+blLyOIXEPuJwi6bT+RT9hPOsKFW+bbGYvamU8LpD6FsIa5ew==", executable: "lark-cli", authentication_driver: "feishu", supported_architectures: ["linux-amd64"], recommended_skill_ids: [], capabilities: [{ id: "identity", argv_prefix: ["auth", "status"], risk: "low", identities: ["user"], scopes: [], egress_hosts: ["open.feishu.cn"], timeout_seconds: 60 }] });
const editingMCP = ref<MCPServer>();
const showMCP = ref(false);
const mcpForm = ref<MCPDraft>(emptyMCPDraft());
const editingSkill = ref<Skill>();
const showSkill = ref(false);
const skillForm = ref({ name: "", source: "git" as "git" | "upload", git_url: "", git_ref: "main", archive: "" });
const pendingDelete = ref<({ kind: "mcp"; item: MCPServer } | { kind: "skill"; item: Skill }) & { impact: ResourceDeletionImpact }>();
let poll: number | undefined;

onMounted(() => {
  void refresh();
  poll = window.setInterval(() => { if (mcp.value.some((item) => item.test_pending)) void refreshMCP(); }, 1500);
});
onBeforeUnmount(() => { if (poll !== undefined) window.clearInterval(poll); });

function emptyMCPDraft(): MCPDraft { return { name: "", transport: "streamable_http", url: "", runner: "npx", package: "", package_version: "", argumentsText: "", environment: [], bearerToken: "" }; }
function notifyResources() { emit("resources", { mcp: mcp.value, skills: skills.value }); }
function selectTab(value: ResourceTab) { activeTab.value = value; emit("tabChange", value); }
async function refresh() {
  try { [mcp.value, skills.value, cliDefinitions.value, cliEnablements.value] = await Promise.all([api.listMCPServers(), api.listSkills(), api.listCLIConnectorDefinitions?.() ?? Promise.resolve([]), api.listCLIConnectorEnablements?.() ?? Promise.resolve([])]); notifyResources(); } catch { emit("error"); }
}
async function enableCLI(item: CLIConnectorDefinition) { try { const value = await api.enableCLIConnector(item.id); cliEnablements.value = [...cliEnablements.value.filter((entry) => entry.definition_id !== item.id), value]; } catch { emit("error"); } }
function enablementFor(id: string) { return cliEnablements.value.find((item) => item.definition_id === id); }
function addCLICapability() { cliForm.value.capabilities.push({ id: "", argv_prefix: [], risk: "low", identities: ["user"], scopes: [], egress_hosts: [], timeout_seconds: 60 }); }
function removeCLICapability(index: number) { cliForm.value.capabilities.splice(index, 1); }
async function saveCLI() { try { await api.createCLIConnectorDefinition(cliForm.value); showCLI.value = false; cliDefinitions.value = await api.listCLIConnectorDefinitions(); } catch { emit("error"); } }
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
  try { await api.deleteMCPServer(item.id, pendingDelete.value?.impact.confirmation_token ?? ""); emit("update:mcpServerIds", props.mcpServerIds.filter((id) => id !== item.id)); await refresh(); } catch { emit("error"); }
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
  try { await api.deleteSkill(item.id, pendingDelete.value?.impact.confirmation_token ?? ""); emit("update:skillIds", props.skillIds.filter((id) => id !== item.id)); await refresh(); } catch { emit("error"); }
}
function toggleSkill(item: Skill, checked: boolean) { emit("update:skillIds", checked ? [...new Set([...props.skillIds, item.id])] : props.skillIds.filter((id) => id !== item.id)); }
async function confirmRemove() {
	if (!pendingDelete.value) return;
	const target = pendingDelete.value;
	if (target.kind === "mcp") await removeMCP(target.item);
	else await removeSkill(target.item);
	pendingDelete.value = undefined;
}
async function requestDelete(target: { kind: "mcp"; item: MCPServer } | { kind: "skill"; item: Skill }) {
  try {
    const impact = target.kind === "mcp" ? await api.getMCPConnectorDeletionImpact(target.item.id) : await api.getSkillDeletionImpact(target.item.id);
    pendingDelete.value = { ...target, impact };
  } catch { emit("error"); }
}
async function fileToBase64(file: File): Promise<string> {
  const bytes = new Uint8Array(await file.arrayBuffer()); let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  return btoa(binary);
}
</script>

<template>
  <div class="extension-manager" :class="{ selectable }">
    <nav class="subtabs" :aria-label="t('resources.title')"><el-button text :class="{ active: activeTab === 'skills' }" @click="selectTab('skills')">{{ t("resources.skills") }}</el-button><el-button text :class="{ active: activeTab === 'mcp' }" @click="selectTab('mcp')">{{ t("resources.connectors") }}</el-button></nav>
    <div v-if="activeTab === 'mcp'"><div class="section-heading"><strong>MCP</strong><el-button type="primary" class="compact-action" @click="openNewMCP">＋ MCP</el-button></div><div class="resource-list"><article v-for="item in mcp" :key="item.id" class="el-card"><label v-if="selectable" class="extension-choice" :title="item.tested ? '' : t('experts.testRequired')"><el-checkbox :model-value="mcpServerIds.includes(item.id)" :disabled="!item.tested" @change="toggleMCP(item, Boolean($event))" /></label><span class="resource-mark">MCP</span><div><strong>{{ item.name }}</strong><p>{{ item.transport }} · {{ item.url || `${item.runner} ${item.package}@${item.package_version}` }}<template v-if="item.test_error"> · {{ item.test_error }}</template></p></div><el-tag :type="item.tested ? 'success' : 'warning'" size="small">{{ item.test_pending ? t("settings.testPending") : item.tested ? t("settings.tested") : t("settings.testRequired") }}</el-tag><el-button circle :aria-label="t('common.retry')" :loading="item.test_pending" @click="testMCP(item)">↻</el-button><el-button circle :aria-label="t('common.edit')" @click="openMCP(item)">✎</el-button><el-button circle type="danger" plain :aria-label="t('common.delete')" @click="requestDelete({ kind: 'mcp', item })">×</el-button></article></div><div class="section-heading"><strong>{{ t('resources.cli') }}</strong><el-button v-if="canManageCLI" @click="showCLI = true">＋ {{ t('resources.cliDefinition') }}</el-button></div><div class="resource-list"><article v-for="item in cliDefinitions" :key="item.id" class="el-card"><span class="resource-mark">CLI</span><div><strong>{{ item.name }}</strong><p>{{ item.npm_package }}@{{ item.npm_version }}</p></div><el-tag>{{ t(`resources.state.${item.state}`) }}</el-tag><template v-if="enablementFor(item.id)?.state === 'waiting_for_user'"><a :href="enablementFor(item.id)?.action_url" target="_blank" rel="noreferrer">{{ t('resources.continueSetup') }}</a></template><el-button v-else-if="item.state === 'available'" @click="enableCLI(item)">{{ t('resources.enable') }}</el-button></article></div></div>
    <div v-if="activeTab === 'skills'"><el-button type="primary" class="compact-action" @click="openNewSkill">＋ Skill</el-button><div class="resource-list"><article v-for="item in skills" :key="item.id" class="el-card"><label v-if="selectable" class="extension-choice"><el-checkbox :model-value="skillIds.includes(item.id)" @change="toggleSkill(item, Boolean($event))" /></label><span class="resource-mark">SK</span><div><strong>{{ item.name }}</strong><p>{{ item.git_url || item.source }} · {{ item.sha256.slice(0, 12) }}</p></div><el-button circle :aria-label="t('common.edit')" @click="openSkill(item)">✎</el-button><el-button circle type="danger" plain :aria-label="t('common.delete')" @click="requestDelete({ kind: 'skill', item })">×</el-button></article></div></div>
  </div>
  <Teleport to="body">
    <div v-if="showMCP" class="modal-layer" @click.self="showMCP = false"><form class="modal-card el-card" @submit.prevent="saveMCP"><h2>{{ editingMCP ? t("common.edit") : t("common.new") }} MCP</h2><label>{{ t("common.name") }}<input v-model="mcpForm.name" required></label><label>{{ t("settings.transport") }}<select v-model="mcpForm.transport"><option value="streamable_http">Streamable HTTP</option><option value="stdio">stdio</option></select></label><template v-if="mcpForm.transport === 'streamable_http'"><label>URL<input v-model="mcpForm.url" type="url" required></label><label>{{ t("settings.bearerToken") }}<input v-model="mcpForm.bearerToken" type="password" :placeholder="editingMCP ? t('settings.keepSecret') : t('settings.optional')"></label></template><template v-else><label>Runner<select v-model="mcpForm.runner"><option value="npx">npx</option><option value="uvx">uvx</option></select></label><label>Package<input v-model="mcpForm.package" required></label><label>{{ t("settings.fixedVersion") }}<input v-model="mcpForm.package_version" required placeholder="1.2.3"></label><label>{{ t("settings.arguments") }}<textarea v-model="mcpForm.argumentsText" rows="4" :placeholder="t('settings.onePerLine')"></textarea></label></template><div><div v-for="(variable, index) in mcpForm.environment" :key="index" class="inline-fields"><input v-model="variable.name" placeholder="VARIABLE_NAME"><input v-model="variable.value" :type="variable.secret ? 'password' : 'text'" :placeholder="variable.configured && variable.secret ? t('settings.keepSecret') : t('settings.value')"><label><input v-model="variable.secret" type="checkbox"> Secret</label><el-button text type="danger" @click="removeMCPEnvironment(index)">×</el-button></div><el-button @click="addMCPEnvironment">＋ {{ t("settings.environment") }}</el-button></div><div class="modal-actions"><el-button @click="showMCP = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ t("common.save") }}</el-button></div></form></div>
    <div v-if="showSkill" class="modal-layer" @click.self="showSkill = false"><form class="modal-card el-card" @submit.prevent="saveSkill"><h2>{{ editingSkill ? t("common.edit") : t("common.new") }} Skill</h2><label>{{ t("common.name") }}<input v-model="skillForm.name" :disabled="Boolean(editingSkill)" required></label><label>{{ t("settings.source") }}<select v-model="skillForm.source" :disabled="Boolean(editingSkill)"><option value="git">Git</option><option value="upload">ZIP</option></select></label><template v-if="skillForm.source === 'git'"><label>Git URL<input v-model="skillForm.git_url" type="url" :disabled="Boolean(editingSkill)" required placeholder="https://github.com/…"></label><label>Git ref<input v-model="skillForm.git_ref" required></label></template><label v-else>ZIP<input type="file" accept=".zip,application/zip" :required="Boolean(editingSkill) || !skillForm.archive" @change="selectSkillArchive"></label><p class="muted">{{ t("settings.skillHint") }}</p><div class="modal-actions"><el-button @click="showSkill = false">{{ t("common.cancel") }}</el-button><el-button native-type="submit" type="primary">{{ t("common.save") }}</el-button></div></form></div>
    <div v-if="showCLI" class="modal-layer" @click.self="showCLI = false"><form class="modal-card el-card" @submit.prevent="saveCLI"><h2>{{ t('resources.cliDefinition') }}</h2><label>{{ t('common.name') }}<input v-model="cliForm.name" required></label><label>{{ t('resources.npmPackage') }}<input v-model="cliForm.npm_package" required></label><label>{{ t('resources.exactVersion') }}<input v-model="cliForm.npm_version" required placeholder="1.2.3"></label><label>{{ t('resources.npmIntegrity') }}<input v-model="cliForm.npm_integrity" required></label><label>{{ t('resources.executable') }}<input v-model="cliForm.executable" required></label><label>{{ t('resources.architectures') }}<el-select v-model="cliForm.supported_architectures" multiple><el-option value="linux-amd64" label="Linux AMD64" /><el-option value="linux-arm64" label="Linux ARM64" /></el-select></label><label>{{ t('resources.recommendedSkills') }}<el-select v-model="cliForm.recommended_skill_ids" multiple><el-option v-for="skill in skills" :key="skill.id" :value="skill.id" :label="skill.name" /></el-select></label><section class="cli-capabilities"><div class="section-heading"><strong>{{ t('resources.capabilities') }}</strong><el-button @click="addCLICapability">＋ {{ t('experts.add') }}</el-button></div><article v-for="(capability, index) in cliForm.capabilities" :key="index" class="el-card"><label>{{ t('resources.capabilityId') }}<el-input v-model="capability.id" required /></label><label>{{ t('resources.argvPrefix') }}<el-select v-model="capability.argv_prefix" multiple allow-create filterable default-first-option /></label><label>{{ t('resources.risk') }}<el-select v-model="capability.risk"><el-option value="low" :label="t('resources.lowRisk')" /><el-option value="high" :label="t('resources.highRisk')" /></el-select></label><label>{{ t('resources.identities') }}<el-select v-model="capability.identities" multiple><el-option value="user" :label="t('approvals.user')" /><el-option value="bot" :label="t('approvals.bot')" /></el-select></label><label>{{ t('resources.scopes') }}<el-select v-model="capability.scopes" multiple allow-create filterable default-first-option /></label><label>{{ t('resources.egressHosts') }}<el-select v-model="capability.egress_hosts" multiple allow-create filterable default-first-option /></label><label>{{ t('resources.timeoutSeconds') }}<el-input-number v-model="capability.timeout_seconds" :min="1" :max="900" /></label><el-button type="danger" text :disabled="cliForm.capabilities.length === 1" @click="removeCLICapability(index)">{{ t('common.delete') }}</el-button></article></section><div class="modal-actions"><el-button @click="showCLI = false">{{ t('common.cancel') }}</el-button><el-button native-type="submit" type="primary">{{ t('common.save') }}</el-button></div></form></div>
  </Teleport>
  <ConfirmDialog :open="Boolean(pendingDelete)" :title="t('common.delete')" :message="pendingDelete ? pendingDelete.impact.affected_experts.length ? t('resources.deleteAffected', { resource: pendingDelete.item.name, experts: pendingDelete.impact.affected_experts.map((expert) => expert.name).join('、') }) : t('resources.deleteUnaffected', { resource: pendingDelete.item.name }) : ''" :confirm-label="t('common.delete')" :cancel-label="t('common.cancel')" danger @cancel="pendingDelete = undefined" @confirm="confirmRemove" />
</template>
