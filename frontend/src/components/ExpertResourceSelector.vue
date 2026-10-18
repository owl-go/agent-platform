<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { CLIConnectorDefinition, CLIConnectorEnablement, MCPServer, Skill } from "../api/client";

const props = withDefaults(defineProps<{
  disabled?: boolean; mcpServers?: MCPServer[]; skills?: Skill[]; cliConnectors?: CLIConnectorDefinition[];
  cliEnablements?: CLIConnectorEnablement[]; mcpServerIds?: string[]; skillIds?: string[];
  cliConnectorDefinitionIds?: string[];
}>(), {
  disabled: false, mcpServers: () => [], skills: () => [], cliConnectors: () => [], cliEnablements: () => [],
  mcpServerIds: () => [], skillIds: () => [], cliConnectorDefinitionIds: () => [],
});
const emit = defineEmits<{
  "update:mcpServerIds": [ids: string[]]; "update:skillIds": [ids: string[]];
  "update:cliConnectorDefinitionIds": [ids: string[]];
}>();
const { t } = useI18n();
// Prefix values because MCP and CLI identifiers belong to independent catalogs.
const connectors = computed(() => [
  ...props.mcpServerIds.map(id => `mcp:${id}`),
  ...props.cliConnectorDefinitionIds.map(id => `cli:${id}`),
]);
function updateConnectors(values: string[]) {
  emit("update:mcpServerIds", values.filter(id => id.startsWith("mcp:")).map(id => id.slice(4)));
  emit("update:cliConnectorDefinitionIds", values.filter(id => id.startsWith("cli:")).map(id => id.slice(4)));
}
function cliEnabled(id: string) {
  return props.cliEnablements.some(item => item.definition_id === id && item.state === "enabled");
}
</script>

<template>
  <div class="expert-resource-selector">
    <label>{{ t('experts.skills') }}
      <el-select :disabled="disabled" :model-value="skillIds" multiple filterable collapse-tags collapse-tags-tooltip :max-collapse-tags="2" :aria-label="t('experts.skills')" :placeholder="t('experts.selectSkillsHint')" @update:model-value="emit('update:skillIds', $event)">
        <el-option v-for="item in skills" :key="item.id" :label="item.name" :value="item.id" />
        <el-option v-for="id in skillIds.filter(id => !skills.some(item => item.id === id))" :key="id" :label="t('composer.resourceUnavailable')" :value="id" disabled />
      </el-select>
    </label>
    <label>{{ t('resources.connectors') }}
      <el-select :disabled="disabled" :model-value="connectors" multiple filterable collapse-tags collapse-tags-tooltip :max-collapse-tags="2" :aria-label="t('resources.connectors')" :placeholder="t('experts.selectConnectorsHint')" @update:model-value="updateConnectors">
        <el-option v-for="item in mcpServers" :key="`mcp:${item.id}`" :label="item.name" :value="`mcp:${item.id}`" :disabled="!item.tested && !mcpServerIds.includes(item.id)"><span>{{ item.name }}</span><small v-if="!item.tested"> · {{ t('experts.testRequired') }}</small></el-option>
        <el-option v-for="item in cliConnectors" :key="`cli:${item.id}`" :label="item.name" :value="`cli:${item.id}`" :disabled="!cliEnabled(item.id) && !cliConnectorDefinitionIds.includes(item.id)"><span>{{ item.name }}</span><small v-if="!cliEnabled(item.id)"> · {{ t('settings.unavailable') }}</small></el-option>
        <el-option v-for="id in mcpServerIds.filter(id => !mcpServers.some(item => item.id === id))" :key="`mcp:${id}`" :label="t('composer.resourceUnavailable')" :value="`mcp:${id}`" disabled />
        <el-option v-for="id in cliConnectorDefinitionIds.filter(id => !cliConnectors.some(item => item.id === id))" :key="`cli:${id}`" :label="t('composer.resourceUnavailable')" :value="`cli:${id}`" disabled />
      </el-select>
    </label>
  </div>
</template>
