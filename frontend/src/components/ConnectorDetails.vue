<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { Lightbulb, MessageSquare } from "@lucide/vue";
import type { CLIConnectorDefinition, CLIConnectorEnablement, ConnectorInstallation, ConnectorPublication, MCPServer } from "../api/client";
import ConnectorIcon from "./ConnectorIcon.vue";

const props = withDefaults(defineProps<{ mcp?: MCPServer; cli?: CLIConnectorDefinition; enablement?: CLIConnectorEnablement; canEdit?: boolean; canConnect?: boolean; installation?: ConnectorInstallation; publication?: ConnectorPublication; busy?: boolean; canUninstall?: boolean; canDisconnect?: boolean; connectedState?: boolean }>(), { connectedState: undefined });
const emit = defineEmits<{ close: []; use: [prompt: string]; connect: []; disconnect: []; uninstall: []; "edit-mcp": [item: MCPServer]; "edit-cli": [item: CLIConnectorDefinition] }>();
const { t, locale } = useI18n();
const open = computed(() => Boolean(props.mcp || props.cli || props.installation || props.publication));
const title = computed(() => props.mcp?.name || props.cli?.name || props.installation?.name || props.publication?.revision.name || "");

// Installed examples belong to the active revision, even when the catalog offers an upgrade.
const metadata = computed(() => props.installation ?? props.publication?.revision);
const examples = computed(() => {
  const primary = locale.value.startsWith("zh") ? metadata.value?.examples_zh : metadata.value?.examples_en;
  const fallback = locale.value.startsWith("zh") ? metadata.value?.examples_en : metadata.value?.examples_zh;
  const values = (primary?.length ? primary : fallback) ?? [];
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))];
});
const prompts = computed(() => examples.value.length ? examples.value : [t("resources.connectorStarter", { name: title.value })]);
const connected = computed(() => props.connectedState ?? (props.installation ? props.installation.state === "active" && props.installation.authorized : props.cli ? props.enablement?.state === "enabled" : Boolean(props.mcp?.tested && !props.mcp.test_error)));
const available = computed(() => !props.busy && (props.installation ? (props.installation.state === "active" || props.installation.state === "disabled" && props.publication?.state === "available") : props.publication ? props.publication.state === "available" && props.publication.revision.conformance_available : props.cli ? props.cli.state === "available" : Boolean(props.mcp?.tested && !props.mcp.test_error)));

function edit() {
  if (props.mcp) emit("edit-mcp", props.mcp);
  if (props.cli) emit("edit-cli", props.cli);
  emit("close");
}
</script>

<template>
  <el-dialog :model-value="open" class="connector-details" :title="title" width="min(720px, calc(100vw - 32px))" align-center append-to-body destroy-on-close :close-on-click-modal="!busy" :close-on-press-escape="!busy" @close="emit('close')">
    <template v-if="metadata">
      <div class="catalog-detail-intro">
        <ConnectorIcon :icon="metadata.icon || 'plug'" :size="56" />
        <div><el-tag :type="connected ? 'success' : 'info'" size="small">{{ t(connected ? 'resources.connected' : 'resources.setupRequired') }}</el-tag><p>{{ metadata.description }}</p></div>
      </div>
    </template>
    <template v-else-if="mcp">
      <div class="catalog-detail-intro">
        <ConnectorIcon :icon="mcp.icon" :size="42" />
        <div><el-tag :type="mcp.tested ? 'success' : 'warning'" size="small">{{ mcp.test_pending ? t('settings.testPending') : mcp.tested ? t('settings.tested') : t('settings.testRequired') }}</el-tag><p>{{ t('resources.mcpDetailDescription') }}</p></div>
      </div>
      <dl class="connector-detail-fields">
        <div><dt>{{ t('resources.connectorType') }}</dt><dd>MCP · {{ mcp.transport === 'streamable_http' ? 'Streamable HTTP' : 'stdio' }}</dd></div>
        <div><dt>{{ t('resources.connectionAddress') }}</dt><dd>{{ mcp.url || `${mcp.runner} ${mcp.package}@${mcp.package_version}` }}</dd></div>
        <div v-if="mcp.arguments.length"><dt>{{ t('settings.arguments') }}</dt><dd>{{ mcp.arguments.join(' ') }}</dd></div>
      </dl>
      <el-alert v-if="mcp.test_error" :title="mcp.test_error" type="error" :closable="false" />
    </template>
    <template v-else-if="cli">
      <div class="catalog-detail-intro">
        <ConnectorIcon :icon="cli.icon || 'terminal'" :size="42" />
        <div><el-tag size="small">{{ t(`resources.state.${cli.state}`) }}</el-tag><el-tag v-if="cli.state === 'available'" :type="connected ? 'success' : 'info'" size="small">{{ t(connected ? 'resources.connected' : 'resources.setupRequired') }}</el-tag><p>{{ cli.description || (cli.npm_package === '@larksuite/cli' ? t('resources.feishuCapability') : t('resources.noCapabilityDescription')) }}</p></div>
      </div>
      <dl class="connector-detail-fields">
        <div><dt>{{ t('resources.connectorType') }}</dt><dd>{{ t('resources.cliConnector') }}</dd></div>
        <div><dt>{{ t('resources.installationType') }}</dt><dd>{{ cli.installation_type === 'upload' ? t('resources.zipUpload') : `${t('resources.npmInstall')} · ${cli.npm_package}@${cli.npm_version}` }}</dd></div>
        <div v-if="enablement?.provider_name"><dt>{{ t('resources.connectedApplication') }}</dt><dd>{{ enablement.provider_name }}</dd></div>
      </dl>
      <el-alert v-if="cli.failure_reason" :title="cli.failure_reason" type="error" :closable="false" />
    </template>
    <slot name="details" />
    <section class="connector-usage">
      <h3><Lightbulb :size="20" />{{ t('resources.connectorExamples') }}</h3>
      <p>{{ t('resources.connectorDraftHint') }}</p>
      <button v-for="prompt in prompts" :key="prompt" type="button" class="connector-usage-prompt" :disabled="!available" @click="emit('use', prompt)"><span>{{ prompt }}</span><MessageSquare :size="20" aria-hidden="true" /></button>
    </section>
    <template #footer>
      <div class="connector-detail-actions">
        <el-button v-if="canEdit" :disabled="busy" @click="edit">{{ t('common.edit') }}</el-button>
        <el-button v-if="canUninstall" type="danger" plain :disabled="busy" @click="emit('uninstall')">{{ t('resources.uninstall') }}</el-button>
        <el-button v-if="connected && canDisconnect" :disabled="busy" @click="emit('disconnect')">{{ t('resources.disconnectConnector') }}</el-button>
        <el-button v-else-if="!connected && (canConnect || !installation && publication)" type="primary" :loading="busy" :disabled="busy || publication && !installation && (publication.state !== 'available' || !publication.revision.conformance_available) || cli && cli.state !== 'available'" @click="emit('connect')">{{ t('resources.connectConnector') }}</el-button>
      </div>
    </template>
  </el-dialog>
</template>
