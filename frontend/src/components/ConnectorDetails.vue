<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { CLIConnectorDefinition, CLIConnectorEnablement, MCPServer } from "../api/client";
import ProfileIcon from "./ProfileIcon.vue";

const props = defineProps<{ mcp?: MCPServer; cli?: CLIConnectorDefinition; enablement?: CLIConnectorEnablement; canEdit?: boolean; canPublish?: boolean }>();
const emit = defineEmits<{ close: []; "edit-mcp": [item: MCPServer]; "edit-cli": [item: CLIConnectorDefinition]; "publish-cli": [item: CLIConnectorDefinition] }>();
const { t } = useI18n();
const open = computed(() => Boolean(props.mcp || props.cli));
const title = computed(() => props.mcp?.name || props.cli?.name || "");

function edit() {
  if (props.mcp) emit("edit-mcp", props.mcp);
  if (props.cli) emit("edit-cli", props.cli);
  emit("close");
}
</script>

<template>
  <el-drawer :model-value="open" class="catalog-details connector-details" :title="title" size="min(620px, 100vw)" destroy-on-close @close="emit('close')">
    <template v-if="mcp">
      <div class="catalog-detail-intro">
        <span class="extension-card-mark">{{ mcp.name.slice(0, 1).toUpperCase() }}</span>
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
        <ProfileIcon :icon="cli.icon || 'terminal'" />
        <div><el-tag size="small">{{ t(`resources.state.${cli.state}`) }}</el-tag><p>{{ cli.description || (cli.npm_package === '@larksuite/cli' ? t('resources.feishuCapability') : t('resources.noCapabilityDescription')) }}</p></div>
      </div>
      <dl class="connector-detail-fields">
        <div><dt>{{ t('resources.connectorType') }}</dt><dd>{{ t('resources.cliConnector') }}</dd></div>
        <div><dt>{{ t('resources.installationType') }}</dt><dd>{{ cli.installation_type === 'upload' ? t('resources.zipUpload') : `${t('resources.npmInstall')} · ${cli.npm_package}@${cli.npm_version}` }}</dd></div>
        <div v-if="enablement?.provider_name"><dt>{{ t('resources.connectedApplication') }}</dt><dd>{{ enablement.provider_name }}</dd></div>
        <div><dt>Manifest</dt><dd>v{{ cli.manifest_version }} · {{ cli.bundle_sha256 || '—' }}</dd></div>
        <div><dt>{{ t('resources.capabilities') }}</dt><dd><article v-for="capability in cli.capabilities" :key="capability.id" class="connector-manifest-capability"><strong>{{ capability.display_name?.['zh-CN'] || capability.display_name?.en || capability.id }}</strong><small v-if="capability.display_name?.en">EN: {{ capability.display_name.en }}</small><small>{{ t('resources.operationPhraseZh') }}: {{ capability.operation_phrase?.['zh-CN'] || '—' }}</small><small>{{ t('resources.operationPhraseEn') }}: {{ capability.operation_phrase?.en || '—' }}</small><small>{{ capability.risk === 'high' ? t('resources.highRisk') : t('resources.lowRisk') }} · {{ capability.identities.join(', ') }}</small><code>{{ capability.argv_prefix.join(' ') }}</code><small>{{ t('resources.scopes') }}: {{ capability.scopes.join(', ') || '—' }}</small><small>{{ t('resources.egressHosts') }}: {{ capability.egress_hosts.join(', ') }}</small><small>{{ t('resources.timeoutSeconds') }}: {{ capability.timeout_seconds }} · {{ capability.idempotency }}</small></article></dd></div>
        <div><dt>{{ t('resources.usageGuide') }}</dt><dd class="connector-usage-guide">{{ cli.usage_guide || '—' }}</dd></div>
      </dl>
      <el-alert v-if="cli.failure_reason" :title="cli.failure_reason" type="error" :closable="false" />
    </template>
    <template #footer><el-button v-if="canEdit" @click="edit">{{ t('common.edit') }}</el-button><el-button v-if="canPublish && cli" type="primary" @click="emit('publish-cli', cli)">{{ t('resources.publish') }}</el-button></template>
  </el-drawer>
</template>
