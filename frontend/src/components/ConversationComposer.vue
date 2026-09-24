<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ArrowUp, Check, ChevronLeft, ChevronRight, FilePlus2, FileText, Folder, Link, Plus, Search, Sparkles, Square, UserRound, Users, X } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { platformApiKey, type Attachment, type CLIConnectorAuthorization, type CLIConnectorAuthorizationFlow, type CLIConnectorDefinition, type CLIConnectorEnablement, type ConversationFile, type ConversationInput, type ConversationScope, type ConversationSelection, type Expert, type ExpertTeam, type MCPServer, type SelectionInput, type Skill } from "../api/client";
import { authContextKey } from "../auth/session";
import { conversationDraftKey, draftText, loadConversationDraft, saveConversationDraft, type DraftPart, type ComposerSubmission } from "../conversationDraft";
import type { CLIAuthorizationRequest } from "../cliAuthorization";
import ConnectorIcon from "./ConnectorIcon.vue";
import { clearSessionApproval, placeSessionApproval } from "../commandApprovalPlacement";

import ProfileIcon from "./ProfileIcon.vue";

const props = defineProps<{ scope: ConversationScope; disabled?: boolean; sendDisabled?: boolean; active?: boolean; stopping?: boolean; initialSkillId?: string; initialPrompt?: string; authorizationRequest?: CLIAuthorizationRequest; approvalExecutionId?: number; submit: (message: ComposerSubmission) => Promise<void> }>();
const emit = defineEmits<{ stop: []; launchConsumed: []; selectionChanged: [selection: ConversationSelection] }>();
const api = inject(platformApiKey)!;
const auth = inject(authContextKey, undefined);
const { t } = useI18n();
const router = useRouter();
const editor = ref<HTMLDivElement>();
const fileInput = ref<HTMLInputElement>();
const root = ref<HTMLElement>();
const selection = ref<ConversationSelection>();
const parts = ref<DraftPart[]>([]);
const uploaded = ref<Attachment[]>([]);
const pending = ref<File[]>([]);
const missingFiles = ref<string[]>([]);
const experts = ref<Expert[]>([]), teams = ref<ExpertTeam[]>([]), skills = ref<Skill[]>([]), mcp = ref<MCPServer[]>([]), cli = ref<CLIConnectorDefinition[]>([]), enablements = ref<CLIConnectorEnablement[]>([]);
const files = ref<ConversationFile[]>([]);
const workspacePath = ref("");
const menu = ref<"main" | "experts" | "skills" | "connectors" | "files" | "">("");
const query = ref("");
const highlighted = ref(0);
const loading = ref(true), updating = ref(false), sending = ref(false);
const error = ref("");
const cliAuthorizationPrompt = ref<{ definition: CLIConnectorDefinition; enablement: CLIConnectorEnablement; scopes: string[]; flow?: CLIConnectorAuthorizationFlow; completed?: boolean; failed?: boolean; activation?: boolean }>();
const cliAuthorizationBusy = ref(false);
const cliActivationBusy = ref<string[]>([]);
const pendingCLIActivation = ref<{ definition: CLIConnectorDefinition; popup: Window | null; selectAfter: boolean }>();
const owner = computed(() => auth?.session.state.value.kind === "authenticated" ? auth.session.state.value.currentUser.id : "");
const storageKey = computed(() => owner.value ? conversationDraftKey(owner.value, props.scope) : "");
const editorLocked = computed(() => props.disabled || sending.value || loading.value);
const locked = computed(() => editorLocked.value || updating.value);
const canSend = computed(() => !locked.value && !props.sendDisabled && !props.active && !!selection.value && Boolean(draftText(parts.value) || pending.value.length || uploaded.value.length));
const referencedFiles = computed(() => parts.value.flatMap((part) => part.kind === "file" ? [part.file] : []));
let caret: Range | undefined;
let trigger: { node: Text; start: number; end: number } | undefined;
let disposed = false;
let cliAuthorizationPoll: ReturnType<typeof setTimeout> | undefined;
let cliActivationPoll: ReturnType<typeof setTimeout> | undefined;
const tokens = new WeakMap<Node, Exclude<DraftPart, { kind: "text" }>>();

const matches = (name: string) => name.toLocaleLowerCase().includes(query.value.toLocaleLowerCase());
const filteredSkills = computed(() => skills.value.filter((item) => matches(item.name)));
const filteredExperts = computed(() => experts.value.filter((item) => matches(`${item.name} ${item.introduction}`)));
const filteredTeams = computed(() => teams.value.filter((item) => matches(`${item.name} ${item.introduction}`)));
const filteredFiles = computed(() => files.value.filter((item) => matches(`${item.name} ${item.path}`)).sort((a, b) => a.kind.localeCompare(b.kind) || a.name.localeCompare(b.name)));
const connectorRows = computed(() => [
  ...mcp.value.map((item) => ({ id: item.id, key: `mcp:${item.id}`, kind: "mcp" as const, name: item.name, icon: item.icon, available: item.tested && !item.test_error, active: item.tested && !item.test_error })),
  ...cli.value.map((item) => ({ id: item.id, key: `cli:${item.id}`, kind: "cli" as const, name: item.name, icon: item.icon, available: item.state === "available", active: cliActivationIsOn(item.id), definition: item })),
]);
const visibleConnectors = computed(() => {
  const value = selection.value; if (!value) return [];
  const rows = [...value.inherited_mcp_servers, ...value.mcp_servers].map((item) => ({ ...item, kind: "mcp" as const, key: `mcp:${item.id}` }));
  const all = [...rows, ...[...value.inherited_cli_connectors, ...value.cli_connectors].map((item) => ({ ...item, kind: "cli" as const, key: `cli:${item.id}` }))];
  return all.filter((item, index) => all.findIndex((other) => other.key === item.key) === index);
});
function isWorkspaceFile(file?: ConversationFile): boolean { return file?.kind === "workspace" || file?.kind === "directory"; }
function connectorEnabled(key: string): boolean { return !!selection.value && visibleConnectors.value.some((item) => item.key === key) && !selection.value.disabled_connectors.includes(key); }

function persist() {
  if (storageKey.value) saveConversationDraft(storageKey.value, { parts: parts.value, selection: selection.value, attachments: uploaded.value, pendingFileNames: [...missingFiles.value, ...pending.value.map((file) => file.name)] });
}
function rememberCaret() {
  const range = window.getSelection()?.rangeCount ? window.getSelection()!.getRangeAt(0) : undefined;
  if (range && editor.value?.contains(range.commonAncestorContainer)) caret = range.cloneRange();
}
function placeCaret(range: Range) { const current = window.getSelection(); current?.removeAllRanges(); current?.addRange(range); caret = range.cloneRange(); }
function tokenNode(part: Exclude<DraftPart, { kind: "text" }>): HTMLSpanElement {
  const node = document.createElement("span"); node.contentEditable = "false"; node.className = "composer-token"; tokens.set(node, part);
  const label = document.createElement("span"); label.textContent = part.kind === "skill" ? part.name : `@${part.file.name}`; node.append(label);
  const button = document.createElement("button"); button.type = "button"; button.textContent = "×"; button.setAttribute("aria-label", t("composer.removeToken", { name: part.kind === "skill" ? part.name : part.file.name }));
  button.addEventListener("click", () => { if (locked.value) return; node.remove(); void readEditor(); }); node.append(button); return node;
}
function renderEditor() {
  if (!editor.value) return;
  editor.value.replaceChildren(...parts.value.map((part) => part.kind === "text" ? document.createTextNode(part.text) : tokenNode(part)));
}
async function readEditor() {
  if (!editor.value) return;
  const result: DraftPart[] = [];
  function visit(node: Node) {
    if (node.nodeType === Node.TEXT_NODE) { result.push({ kind: "text", text: node.textContent ?? "" }); return; }
    if (!(node instanceof HTMLElement)) return;
    const token = tokens.get(node);
    if (token) { result.push(token); return; }
    if (node.tagName === "BR") { result.push({ kind: "text", text: "\n" }); return; }
    if ((node.tagName === "DIV" || node.tagName === "P") && result.length) result.push({ kind: "text", text: "\n" });
    node.childNodes.forEach(visit);
  }
  editor.value.childNodes.forEach(visit); parts.value = result; rememberCaret(); persist();
  const wanted = result.flatMap((part) => part.kind === "skill" ? [part.id] : []);
  if (selection.value && selection.value.skills.some((item) => !wanted.includes(item.id))) await changeSelection({ skill_ids: wanted });
}
function inputSelection(patch: Partial<SelectionInput>): SelectionInput {
  const value = selection.value;
  return { previous_id: value?.id, skill_ids: value?.skills.map((item) => item.id) ?? [], mcp_server_ids: value?.mcp_servers.map((item) => item.id) ?? [], cli_connector_ids: value?.cli_connectors.map((item) => item.id) ?? [], disabled_connectors: value?.disabled_connectors ?? [], ...patch };
}
async function changeSelection(patch: Partial<SelectionInput>): Promise<boolean> {
  if (updating.value) return false;
  updating.value = true; error.value = "";
  try { const value = await api.resolveConversationSelection(props.scope, inputSelection(patch)); if (disposed) return false; selection.value = value; persist(); return true; }
  catch { error.value = t("composer.selectionFailed"); return false; }
  finally { updating.value = false; }
}
function insertPart(part: DraftPart) {
  const target = editor.value; if (!target) return;
  target.focus(); let range = caret?.cloneRange();
  if (!range || !target.contains(range.commonAncestorContainer)) { range = document.createRange(); range.selectNodeContents(target); range.collapse(false); }
  if (trigger?.node.isConnected) { range.setStart(trigger.node, trigger.start); range.setEnd(trigger.node, Math.min(trigger.end, trigger.node.length)); }
  range.deleteContents(); const node = part.kind === "text" ? document.createTextNode(part.text) : tokenNode(part); range.insertNode(node); range.setStartAfter(node); range.collapse(true);
  if (part.kind !== "text") { const space = document.createTextNode(" "); range.insertNode(space); range.setStartAfter(space); range.collapse(true); }
  placeCaret(range); trigger = undefined; menu.value = ""; query.value = ""; void readEditor();
}
async function chooseSkill(item: Pick<Skill, "id" | "name">) {
  if (parts.value.some((part) => part.kind === "skill" && part.id === item.id)) { menu.value = ""; return; }
  if (await changeSelection({ skill_ids: [...parts.value.flatMap((part) => part.kind === "skill" ? [part.id] : []), item.id], refresh_ids: [`skill:${item.id}`] })) insertPart({ kind: "skill", id: item.id, name: item.name });
}
async function chooseExpert(kind: "expert" | "team" | "none", id = "") {
  if (await changeSelection({ change_expert: true, expert_id: kind === "expert" ? id : "", expert_team_id: kind === "team" ? id : "" })) menu.value = "";
}
async function toggleConnector(kind: "mcp" | "cli", id: string) {
  if (!selection.value) return;
  const key = `${kind}:${id}`, enabled = connectorEnabled(key), field = kind === "mcp" ? "mcp_server_ids" : "cli_connector_ids";
  const ids = (kind === "mcp" ? selection.value.mcp_servers : selection.value.cli_connectors).map((item) => item.id).filter((value) => value !== id);
  if (await changeSelection({ [field]: enabled ? ids : [...ids, id], disabled_connectors: enabled ? [...selection.value.disabled_connectors.filter((item) => item !== key), key] : selection.value.disabled_connectors.filter((item) => item !== key), refresh_ids: enabled ? [] : [key] })) await refreshRequestedCLIAuthorization();
}

function cliEnablement(definitionID: string) { return enablements.value.find((item) => item.definition_id === definitionID); }
function cliActivationIsOn(definitionID: string) { return ["enabled", "waiting_for_user"].includes(cliEnablement(definitionID)?.state ?? ""); }
function cliUserScopes(definition: CLIConnectorDefinition) {
  return [...new Set((definition.capabilities ?? []).filter((capability) => capability.identities?.includes("user")).flatMap((capability) => capability.scopes ?? []))];
}
function replaceCLIEnablement(value: CLIConnectorEnablement) {
  enablements.value = [...enablements.value.filter((item) => item.definition_id !== value.definition_id), value];
}
async function selectActivatedCLI(definitionID: string) {
  if (!selection.value || connectorEnabled(`cli:${definitionID}`)) return true;
  const ids = selection.value.cli_connectors.map((item) => item.id).filter((id) => id !== definitionID);
  return changeSelection({ cli_connector_ids: [...ids, definitionID], disabled_connectors: selection.value.disabled_connectors.filter((item) => item !== `cli:${definitionID}`), refresh_ids: [`cli:${definitionID}`] });
}
async function authorizeActivatedCLI(definition: CLIConnectorDefinition, enablement: CLIConnectorEnablement, popup: Window | null) {
  const scopes = cliUserScopes(definition);
  if (definition.authentication_driver !== "feishu" || !scopes.length) { closeBlankCLIWindow(popup); return; }
  const authorizations = await api.listCLIConnectorAuthorizations(enablement.id);
  if (authorizations.some((item) => item.state === "active" && scopes.every((scope) => (item.scopes ?? []).includes(scope)))) { closeBlankCLIWindow(popup); return; }
  const flow = await api.beginCLIConnectorAuthorization(enablement.id, "user", scopes);
  cliAuthorizationPrompt.value = { definition, enablement, scopes, flow, activation: true };
  if (popup && !popup.closed && flow.action_url) popup.location.href = flow.action_url;
  scheduleCLIAuthorizationPoll();
}
async function finishCLIActivation(definition: CLIConnectorDefinition, enablement: CLIConnectorEnablement, popup: Window | null, selectAfter: boolean) {
  replaceCLIEnablement(enablement);
  if (selectAfter && !await selectActivatedCLI(definition.id)) { closeBlankCLIWindow(popup); return; }
  await authorizeActivatedCLI(definition, enablement, popup);
}
function scheduleCLIActivationPoll() {
  if (cliActivationPoll) clearTimeout(cliActivationPoll);
  cliActivationPoll = setTimeout(() => void completeCLIActivation(), 3000);
}
async function completeCLIActivation() {
  const pending = pendingCLIActivation.value;
  const enablement = pending && cliEnablement(pending.definition.id);
  if (!pending || !enablement || enablement.state !== "waiting_for_user" || disposed) return;
  let completed: CLIConnectorEnablement;
  try {
    completed = await api.completeCLIConnectorEnablement(enablement.id);
  } catch { scheduleCLIActivationPoll(); return; }
  replaceCLIEnablement(completed);
  if (completed.state === "waiting_for_user") { scheduleCLIActivationPoll(); return; }
  pendingCLIActivation.value = undefined;
  if (completed.state !== "enabled") { closeBlankCLIWindow(pending.popup); return; }
  try { await finishCLIActivation(pending.definition, completed, pending.popup, pending.selectAfter); }
  catch { closeBlankCLIWindow(pending.popup); error.value = t("composer.selectionFailed"); }
}
async function setCLIActivation(definition: CLIConnectorDefinition, active: boolean, selectAfter = false) {
  if (cliActivationBusy.value.includes(definition.id)) return;
  cliActivationBusy.value.push(definition.id); error.value = "";
  const popup = active && definition.authentication_driver === "feishu" ? openCLIWindow() : null;
  try {
    if (active) {
      const enablement = await api.enableCLIConnector(definition.id);
      replaceCLIEnablement(enablement);
      if (enablement.state === "waiting_for_user") {
        pendingCLIActivation.value = { definition, popup, selectAfter };
        if (popup && !popup.closed && enablement.action_url) popup.location.href = enablement.action_url;
        scheduleCLIActivationPoll();
      } else if (enablement.state === "enabled") await finishCLIActivation(definition, enablement, popup, selectAfter);
      else closeBlankCLIWindow(popup);
    } else {
      const enablement = cliEnablement(definition.id);
      if (!enablement) return;
      replaceCLIEnablement(await api.disableCLIConnector(definition.id, enablement.version));
      if (pendingCLIActivation.value?.definition.id === definition.id) {
        pendingCLIActivation.value = undefined;
        if (cliActivationPoll) clearTimeout(cliActivationPoll);
      }
      if (connectorEnabled(`cli:${definition.id}`)) await toggleConnector("cli", definition.id);
      if (cliAuthorizationPrompt.value?.definition.id === definition.id) cliAuthorizationPrompt.value = undefined;
    }
  } catch { closeBlankCLIWindow(popup); error.value = t("composer.selectionFailed"); }
  finally { cliActivationBusy.value = cliActivationBusy.value.filter((id) => id !== definition.id); }
}
async function chooseConnector(item: (typeof connectorRows.value)[number]) {
  if (item.kind === "cli" && !item.active) { await setCLIActivation(item.definition, true, true); return; }
  await toggleConnector(item.kind, item.id);
}
async function setVisibleConnectorActive(kind: "mcp" | "cli", id: string, active: boolean) {
  if (kind === "mcp") {
    if (connectorEnabled(`mcp:${id}`) !== active) await toggleConnector(kind, id);
    return;
  }
  const definition = cli.value.find((item) => item.id === id);
  if (definition) await setCLIActivation(definition, active, active);
}

async function refreshRequestedCLIAuthorization() {
  const request = props.authorizationRequest;
  if (!selection.value || !request || disposed || !connectorEnabled(`cli:${request.connectorID}`)) {
    cliAuthorizationPrompt.value = undefined;
    return;
  }
  const definition = cli.value.find((item) => item.id === request.connectorID);
  const enablement = enablements.value.find((item) => item.definition_id === request.connectorID && item.state === "enabled");
  const capability = definition?.capabilities?.find((item) => item.id === request.capabilityID && item.identities?.includes("user"));
  if (!definition || definition.authentication_driver !== "feishu" || !enablement || !capability) {
    cliAuthorizationPrompt.value = undefined;
    return;
  }
  const scopes = [...new Set(capability.scopes ?? [])];
  if (!scopes.length) {
    cliAuthorizationPrompt.value = undefined;
    return;
  }
  let authorizations: CLIConnectorAuthorization[];
  try { authorizations = await api.listCLIConnectorAuthorizations(enablement.id); }
  catch { return; }
  if (props.authorizationRequest !== request || disposed) return;
  const authorized = authorizations.some((item) => item.state === "active" && scopes.every((scope) => (item.scopes ?? []).includes(scope)));
  if (authorized) {
    cliAuthorizationPrompt.value = undefined;
    return;
  }
  const current = cliAuthorizationPrompt.value;
  cliAuthorizationPrompt.value = current?.definition.id === definition.id && current.scopes.join() === scopes.join() ? { ...current, definition, enablement, scopes } : { definition, enablement, scopes };
}
function openCLIWindow(): Window | null {
  try {
    const popup = window.open("about:blank", "_blank");
    if (popup) popup.opener = null;
    return popup;
  } catch { return null; }
}
function closeBlankCLIWindow(popup: Window | null) {
  try { if (popup && !popup.closed && popup.location.href === "about:blank") popup.close(); } catch { /* Keep external pages open. */ }
}
async function beginSelectedCLIAuthorization() {
  const prompt = cliAuthorizationPrompt.value;
  if (!prompt || cliAuthorizationBusy.value) return;
  const popup = openCLIWindow();
  cliAuthorizationBusy.value = true; prompt.failed = false;
  try {
    const flow = await api.beginCLIConnectorAuthorization(prompt.enablement.id, "user", prompt.scopes);
    if (disposed) { closeBlankCLIWindow(popup); return; }
    prompt.flow = flow;
    if (popup && !popup.closed && flow.action_url) popup.location.href = flow.action_url;
    scheduleCLIAuthorizationPoll();
  } catch {
    closeBlankCLIWindow(popup); prompt.failed = true;
  } finally { cliAuthorizationBusy.value = false; }
}
function scheduleCLIAuthorizationPoll() {
  if (cliAuthorizationPoll) clearTimeout(cliAuthorizationPoll);
  cliAuthorizationPoll = setTimeout(() => void completeSelectedCLIAuthorization(), 3000);
}
async function completeSelectedCLIAuthorization() {
  const prompt = cliAuthorizationPrompt.value;
  if (!prompt?.flow || prompt.flow.state !== "waiting_for_user" || cliAuthorizationBusy.value || disposed) return;
  cliAuthorizationBusy.value = true;
  try {
    const flow = await api.completeCLIConnectorAuthorization(prompt.flow.id);
    prompt.flow = flow;
    if (flow.state === "completed") prompt.completed = true;
    else if (flow.state === "invalid") prompt.failed = true;
    else scheduleCLIAuthorizationPoll();
  } catch { scheduleCLIAuthorizationPoll(); }
  finally { cliAuthorizationBusy.value = false; }
}
function handleAuthorizationReturn() {
  if (document.visibilityState === "visible") { void completeCLIActivation(); void completeSelectedCLIAuthorization(); }
}
async function loadFiles(path = "") {
  try { files.value = await api.listConversationFiles(props.scope, path); workspacePath.value = path; }
  catch { error.value = t("composer.filesFailed"); }
}
async function openMenu(value: typeof menu.value) { rememberCaret(); trigger = undefined; menu.value = menu.value === value ? "" : value; query.value = ""; highlighted.value = 0; if (value === "files") await loadFiles(); }
function onInput(event: Event) {
  void readEditor(); if ((event as InputEvent).isComposing) return;
  const range = caret; const node = range?.startContainer;
  if (!(node instanceof Text) || !range?.collapsed) { trigger = undefined; return; }
  const before = node.data.slice(0, range.startOffset), match = /(?:^|\s)([/@])([^\s/@]*)$/.exec(before);
  if (!match) { if (trigger) menu.value = ""; trigger = undefined; return; }
  trigger = { node, start: range.startOffset - match[2].length - 1, end: range.startOffset };
  menu.value = match[1] === "/" ? "skills" : "files"; query.value = match[2]; highlighted.value = 0;
  if (menu.value === "files") void loadFiles(workspacePath.value);
}
async function chooseFile(file: ConversationFile) {
  if (file.kind === "directory") { await loadFiles(file.path); query.value = ""; return; }
  const count = new Set(referencedFiles.value.map((item) => `${item.kind}:${item.id}:${item.path}`)).size + pending.value.length + uploaded.value.length;
  const repeated = referencedFiles.value.some((item) => item.kind === file.kind && item.id === file.id && item.path === file.path);
  if (!file.available || file.size > 100 * 1024 * 1024 || (!repeated && count >= 10)) { error.value = t("sessions.attachmentLimits"); return; }
  insertPart({ kind: "file", file });
}
function chooseLocal(event: Event) {
  const input = event.target as HTMLInputElement, chosen = [...(input.files ?? [])];
  queueLocalFiles(chosen);
  input.value = "";
}
function queueLocalFiles(chosen: File[]) {
  if (!chosen.length || locked.value) return;
  if (chosen.some((file) => file.size > 100 * 1024 * 1024) || chosen.length + pending.value.length + uploaded.value.length + new Set(referencedFiles.value.map((file) => `${file.kind}:${file.id}:${file.path}`)).size > 10) error.value = t("sessions.attachmentLimits");
  else pending.value.push(...chosen);
  menu.value = ""; persist();
}
function clipboardFiles(data: DataTransfer | null): File[] {
  if (!data) return [];
  const files = [...data.files];
  if (files.length) return files;
  return [...data.items].flatMap((item) => {
    const file = item.kind === "file" ? item.getAsFile() : null;
    return file ? [file] : [];
  });
}
function keydown(event: KeyboardEvent) {
  if (event.isComposing) return;
  if (event.key === "Escape") { menu.value = ""; trigger = undefined; return; }
  if ((menu.value === "skills" || menu.value === "files") && ["ArrowDown", "ArrowUp", "Enter"].includes(event.key)) {
    const items = menu.value === "skills" ? filteredSkills.value : filteredFiles.value;
    event.preventDefault();
    if (event.key === "Enter") { const item = items[highlighted.value]; if (item) { if (menu.value === "skills") void chooseSkill(item); else void chooseFile(item as ConversationFile); } }
    else highlighted.value = (highlighted.value + (event.key === "ArrowDown" ? 1 : -1) + Math.max(items.length, 1)) % Math.max(items.length, 1);
    return;
  }
  if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); if (!menu.value) void send(); }
}
function paste(event: ClipboardEvent) {
  event.preventDefault(); rememberCaret();
  const files = clipboardFiles(event.clipboardData);
  if (files.length) { queueLocalFiles(files); return; }
  insertPart({ kind: "text", text: event.clipboardData?.getData("text/plain") ?? "" });
}
async function send() {
  if (!canSend.value || !selection.value) return;
  sending.value = true; error.value = "";
  try {
    const skillIDs = parts.value.flatMap((part) => part.kind === "skill" ? [part.id] : []);
    if (selection.value.skills.map((item) => item.id).sort().join() !== [...skillIDs].sort().join()) {
      if (!await changeSelection({ skill_ids: skillIDs })) throw new Error("selection_not_saved");
    }
    while (pending.value.length) { const file = pending.value[0]; const attachment = await api.uploadAttachment(file); uploaded.value.push(attachment); pending.value.shift(); persist(); }
    await props.submit({ content: draftText(parts.value), attachmentIDs: uploaded.value.map((item) => item.id), input: { selection_id: selection.value.id, file_references: referencedFiles.value.map((file) => ({ kind: file.kind as "attachment" | "artifact" | "workspace", id: file.id, path: file.path })) } });
    if (cliAuthorizationPrompt.value?.completed) cliAuthorizationPrompt.value = undefined;
    parts.value = []; uploaded.value = []; missingFiles.value = []; renderEditor();
    // The accepted selection is immutable. Resolve an empty explicit Skill set
    // for the next draft without modifying the historical message.
    await changeSelection({ skill_ids: [] }); persist();
  } catch { error.value = t("composer.sendFailed"); persist(); }
  finally { sending.value = false; }
}
function outside(event: PointerEvent) { if (!root.value?.contains(event.target as Node)) { menu.value = ""; trigger = undefined; } }
async function initialize() {
  loading.value = true; error.value = "";
  try {
    const results = await Promise.all([api.listExperts(), api.listExpertTeams(), api.listSkills(), api.listMCPServers(), api.listCLIConnectorDefinitions(), api.listCLIConnectorEnablements()]);
    if (disposed) return;
    [experts.value, teams.value, skills.value, mcp.value, cli.value, enablements.value] = results;
    if (!selection.value) selection.value = await api.getConversationSelection(props.scope);
    if (props.initialSkillId) {
      const skill = skills.value.find((item) => item.id === props.initialSkillId);
      if (selection.value.skills.some((item) => item.id === props.initialSkillId)) emit("launchConsumed");
      else if (skill) { await chooseSkill(skill); if (!error.value) emit("launchConsumed"); }
      else error.value = t("composer.selectionFailed");
    }
    await refreshRequestedCLIAuthorization(); persist();
  } catch { error.value = t("composer.selectionFailed"); }
  finally { loading.value = false; }
}
onMounted(async () => {
  placeSessionApproval(props.approvalExecutionId ? String(props.approvalExecutionId) : undefined);
  document.addEventListener("pointerdown", outside);
  document.addEventListener("visibilitychange", handleAuthorizationReturn);
  const saved = storageKey.value ? loadConversationDraft(storageKey.value) : undefined;
  if (saved) { parts.value = saved.parts; selection.value = saved.selection; uploaded.value = saved.attachments; missingFiles.value = saved.pendingFileNames; }
  else if (props.initialPrompt) parts.value = [{ kind: "text", text: props.initialPrompt }];
  await nextTick(); renderEditor();
  await initialize();
});
watch(selection, (value) => { if (value) emit("selectionChanged", value); });
watch(() => props.authorizationRequest, () => void refreshRequestedCLIAuthorization(), { deep: true });
watch(() => props.approvalExecutionId, (current, previous) => { if (previous) clearSessionApproval(String(previous)); placeSessionApproval(current ? String(current) : undefined); });
watch([parts, uploaded, pending, missingFiles, selection], persist, { deep: true });
onBeforeUnmount(() => { disposed = true; clearSessionApproval(props.approvalExecutionId ? String(props.approvalExecutionId) : undefined); if (cliAuthorizationPoll) clearTimeout(cliAuthorizationPoll); if (cliActivationPoll) clearTimeout(cliActivationPoll); persist(); document.removeEventListener("pointerdown", outside); document.removeEventListener("visibilitychange", handleAuthorizationReturn); });
</script>

<template>
  <footer ref="root" class="composer resource-composer" :aria-busy="sending || updating || loading">
    <div id="session-command-approval-slot" class="composer-approval-slot"></div>
    <section v-if="pendingCLIActivation && cliEnablement(pendingCLIActivation.definition.id)?.state === 'waiting_for_user'" class="composer-authorization" role="status" aria-live="polite">
      <div><strong>{{ t('composer.activationRequired', { name: pendingCLIActivation.definition.name }) }}</strong><small>{{ t('composer.activationHint') }}</small></div>
      <a :href="cliEnablement(pendingCLIActivation.definition.id)?.action_url" target="_blank" rel="noopener noreferrer">{{ t('resources.continueSetup') }}</a>
    </section>
    <section v-if="cliAuthorizationPrompt" class="composer-authorization" role="status" aria-live="polite">
      <div><strong>{{ cliAuthorizationPrompt.completed ? t(cliAuthorizationPrompt.activation ? 'composer.activationAuthorizationCompleted' : 'composer.authorizationCompleted') : t('composer.authorizationRequired', { name: cliAuthorizationPrompt.definition.name }) }}</strong><small>{{ cliAuthorizationPrompt.completed ? t(cliAuthorizationPrompt.activation ? 'composer.activationAuthorizationReady' : 'composer.authorizationContinue') : t(cliAuthorizationPrompt.activation ? 'composer.activationAuthorizationHint' : 'composer.authorizationHint') }}</small></div>
      <el-button v-if="!cliAuthorizationPrompt.completed && !cliAuthorizationPrompt.flow?.action_url" type="primary" :loading="cliAuthorizationBusy" @click="beginSelectedCLIAuthorization">{{ t('resources.authorizeNow') }}</el-button>
      <a v-else-if="!cliAuthorizationPrompt.completed" :href="cliAuthorizationPrompt.flow?.action_url" target="_blank" rel="noopener noreferrer">{{ t('resources.authorizeNow') }}</a>
      <small v-if="cliAuthorizationPrompt.failed" class="authorization-error">{{ t('resources.authorizationInvalidInput') }}</small>
    </section>
    <div v-if="error" class="composer-error" role="alert">{{ error }}<el-button v-if="!selection" text :disabled="loading" @click="initialize">{{ t('common.retry') }}</el-button><el-button text @click="error = ''">{{ t('common.close') }}</el-button></div>
    <div v-if="missingFiles.length" class="composer-notice">{{ t('composer.reselectFiles', { names: missingFiles.join(', ') }) }}<el-button text @click="missingFiles = []; persist()">{{ t('common.close') }}</el-button></div>
    <div v-if="pending.length || uploaded.length" class="pending-attachments">
      <span v-for="(file, index) in pending" :key="`local-${index}`">{{ file.name }}<button type="button" :disabled="locked" :aria-label="t('sessions.removeAttachment', { name: file.name })" @click="pending.splice(index, 1); persist()"><X /></button></span>
      <span v-for="(file, index) in uploaded" :key="file.id">{{ file.name }}<button type="button" :disabled="locked" :aria-label="t('sessions.removeAttachment', { name: file.name })" @click="uploaded.splice(index, 1); persist()"><X /></button></span>
    </div>
    <div ref="editor" class="composer-editor" role="textbox" aria-multiline="true" :aria-label="t('sessions.placeholder')" :data-placeholder="t('composer.placeholder')" :contenteditable="!editorLocked" @input="onInput" @keydown="keydown" @keyup="rememberCaret" @mouseup="rememberCaret" @paste="paste" @drop.prevent></div>
    <div class="composer-toolbar">
      <el-button class="composer-plus" circle :disabled="locked" :aria-label="t('composer.add')" :aria-expanded="Boolean(menu)" @click="openMenu('main')"><Plus :size="21" /></el-button>
      <el-button v-if="selection?.name" class="composer-specialist" text :disabled="locked" @click="openMenu('experts')"><ProfileIcon :icon="selection.icon" :background="selection.icon_background" :team="selection.member_count > 1" /><span>{{ selection.name }}</span></el-button>
      <el-popover v-for="item in visibleConnectors" :key="item.key" trigger="click" :width="270" :disabled="locked">
        <template #reference><el-button circle class="composer-connector" :class="{ 'is-off': !connectorEnabled(item.key) }" :aria-label="item.name" :title="item.name"><ConnectorIcon :icon="item.icon" :size="22" /></el-button></template>
        <div class="connector-switch"><strong>{{ item.name }}</strong><el-switch :model-value="item.kind === 'cli' ? cliActivationIsOn(item.id) : connectorEnabled(item.key)" :loading="item.kind === 'cli' && cliActivationBusy.includes(item.id)" :disabled="locked" :aria-label="item.name" @change="setVisibleConnectorActive(item.kind, item.id, Boolean($event))" /></div>
        <el-button text @click="router.push('/resources?tab=connectors')">{{ t('composer.manageConnectors') }}<ChevronRight :size="15" /></el-button>
      </el-popover>
      <span class="composer-spacer"></span>
      <el-button v-if="active" class="stop-generation" circle :loading="stopping" :aria-label="t('sessions.stopGeneration')" @click="emit('stop')"><template #icon><Square :size="17" /></template></el-button>
      <el-button v-else type="primary" circle :loading="sending" :disabled="!canSend" :aria-label="t('composer.send')" @click="send"><template #icon><ArrowUp :size="19" /></template></el-button>
    </div>
    <input ref="fileInput" class="composer-file-input" type="file" multiple :disabled="locked" @change="chooseLocal">
    <section v-if="menu" class="composer-menu" :aria-label="t('composer.add')">
      <template v-if="menu === 'main'">
        <button type="button" @click="openMenu('files')"><FilePlus2 />{{ t('sessions.addAttachment') }}<ChevronRight /></button>
        <button type="button" @click="openMenu('experts')"><UserRound />{{ t('experts.title') }}<ChevronRight /></button>
        <button type="button" @click="openMenu('skills')"><Sparkles />{{ t('composer.skills') }}<ChevronRight /></button>
        <button type="button" @click="openMenu('connectors')"><Link />{{ t('composer.connectors') }}<ChevronRight /></button>
      </template>
      <template v-else>
        <div class="composer-menu-search"><button type="button" :aria-label="t('common.back')" @click="menu = 'main'; query = ''"><ChevronLeft /></button><Search :size="16" /><input v-model="query" :placeholder="t('composer.search')" :aria-label="t('composer.search')" @input="highlighted = 0" @keydown="keydown"></div>
        <div class="composer-options" role="listbox">
          <template v-if="menu === 'experts'">
            <button type="button" :disabled="locked" @click="chooseExpert('none')">{{ t('sessions.noExpert') }}<Check v-if="!selection?.name" /></button>
            <small>{{ t('experts.title') }}</small><button v-for="item in filteredExperts" :key="item.id" type="button" :disabled="locked || !item.available" @click="chooseExpert('expert', item.id)"><UserRound /><span>{{ item.name }}<small>{{ item.introduction }}</small></span><Check v-if="selection?.expert_id === item.id" /></button>
            <small>{{ t('experts.teams') }}</small><button v-for="item in filteredTeams" :key="item.id" type="button" :disabled="locked || !item.available" @click="chooseExpert('team', item.id)"><Users /><span>{{ item.name }}<small>{{ item.introduction }}</small></span><Check v-if="selection?.expert_team_id === item.id" /></button>
          </template>
          <template v-if="menu === 'skills'"><button v-for="(item, index) in filteredSkills" :key="item.id" type="button" role="option" :aria-selected="highlighted === index" :class="{ highlighted: highlighted === index }" :disabled="locked" @click="chooseSkill(item)"><Sparkles /><span>{{ item.name }}</span><Check v-if="parts.some((part) => part.kind === 'skill' && part.id === item.id)" /></button><p v-if="!filteredSkills.length">{{ t('composer.empty') }}</p></template>
          <template v-if="menu === 'connectors'"><div v-for="item in connectorRows.filter((row) => matches(row.name))" :key="item.key" class="composer-connector-option"><button type="button" :disabled="locked || !item.available" @click="chooseConnector(item)"><ConnectorIcon :icon="item.icon" :size="22" /><span>{{ item.name }}<small v-if="!item.available">{{ t('composer.connectorUnavailable') }}</small><small v-else-if="item.kind === 'cli' && !item.active">{{ t('composer.connectorInactive') }}</small></span><Check v-if="connectorEnabled(item.key)" /></button><el-switch v-if="item.kind === 'cli'" :model-value="item.active" :loading="cliActivationBusy.includes(item.id)" :disabled="locked || !item.available" :aria-label="t('composer.connectorActivation', { name: item.name })" @click.stop @change="setCLIActivation(item.definition, Boolean($event), Boolean($event))" /></div></template>
          <template v-if="menu === 'files'">
            <button type="button" @click="fileInput?.click()"><FilePlus2 />{{ t('composer.localFiles') }}</button>
            <button v-if="workspacePath" type="button" @click="loadFiles(workspacePath.split('/').slice(0, -1).join('/'))"><Folder />{{ t('composer.parentFolder') }}</button>
            <template v-for="(item, index) in filteredFiles" :key="`${item.kind}:${item.id}:${item.path}`"><small v-if="index === 0 || isWorkspaceFile(item) !== isWorkspaceFile(filteredFiles[index - 1])">{{ isWorkspaceFile(item) ? t('workflows.workspace') : t('composer.conversationFiles') }}</small><button type="button" role="option" :aria-selected="highlighted === index" :class="{ highlighted: highlighted === index }" :disabled="locked || !item.available" @click="chooseFile(item)"><Folder v-if="item.kind === 'directory'" /><FileText v-else /><span>{{ item.name }}<small>{{ item.kind === 'workspace' || item.kind === 'directory' ? t('workflows.workspace') : t('composer.conversationFiles') }}<template v-if="item.path"> · {{ item.path }}</template><template v-if="!item.available"> · {{ t('composer.fileUnavailable') }}</template></small></span></button></template>
          </template>
        </div>
        <button v-if="menu === 'skills'" type="button" class="composer-menu-footer" @click="router.push('/resources?tab=skills')">{{ t('composer.manageSkills') }}<ChevronRight /></button>
        <button v-if="menu === 'experts'" type="button" class="composer-menu-footer" @click="router.push('/experts')">{{ t('composer.moreExperts') }}<ChevronRight /></button>
        <button v-if="menu === 'connectors'" type="button" class="composer-menu-footer" @click="router.push('/resources?tab=connectors')">{{ t('composer.manageConnectors') }}<ChevronRight /></button>
      </template>
    </section>
  </footer>
</template>
