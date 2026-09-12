<script setup lang="ts">
import { Check } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import type { CLIConnectorDefinition, CLIConnectorEnablement, MCPServer, Skill } from "../api/client";
import ConnectorIcon from "./ConnectorIcon.vue";
import ProfileIcon from "./ProfileIcon.vue";

const props = withDefaults(defineProps<{
  mcpServers?: MCPServer[];
  skills?: Skill[];
  cliConnectors?: CLIConnectorDefinition[];
  cliEnablements?: CLIConnectorEnablement[];
  mcpServerIds?: string[];
  skillIds?: string[];
  cliConnectorDefinitionIds?: string[];
}>(), {
  mcpServers: () => [],
  skills: () => [],
  cliConnectors: () => [],
  cliEnablements: () => [],
  mcpServerIds: () => [],
  skillIds: () => [],
  cliConnectorDefinitionIds: () => [],
});

const emit = defineEmits<{
  "update:mcpServerIds": [ids: string[]];
  "update:skillIds": [ids: string[]];
  "update:cliConnectorDefinitionIds": [ids: string[]];
}>();

const { t } = useI18n();

function toggle(ids: string[], id: string) {
  return ids.includes(id) ? ids.filter((item) => item !== id) : [...ids, id];
}

function toggleMCP(item: MCPServer) {
  if (!item.tested && !props.mcpServerIds.includes(item.id)) return;
  emit("update:mcpServerIds", toggle(props.mcpServerIds, item.id));
}

function toggleSkill(item: Skill) {
  emit("update:skillIds", toggle(props.skillIds, item.id));
}

function cliEnabled(id: string) {
  return props.cliEnablements.some((item) => item.definition_id === id && item.state === "enabled");
}

function toggleCLI(item: CLIConnectorDefinition) {
  if (!cliEnabled(item.id) && !props.cliConnectorDefinitionIds.includes(item.id)) return;
  emit("update:cliConnectorDefinitionIds", toggle(props.cliConnectorDefinitionIds, item.id));
}
</script>

<template>
  <div class="expert-resource-selector">
    <section class="expert-resource-group">
      <header>
        <div><h3>{{ t('experts.skills') }}</h3><p>{{ t('experts.selectSkillsHint') }}</p></div>
        <span>{{ t('experts.selectedCount', { count: skillIds.length }) }}</span>
      </header>
      <div v-if="skills.length" class="expert-resource-options">
        <button v-for="item in skills" :key="item.id" type="button" class="expert-resource-option" :class="{ selected: skillIds.includes(item.id) }" :aria-pressed="skillIds.includes(item.id)" @click="toggleSkill(item)">
          <ProfileIcon :icon="item.icon || 'sparkles'" />
          <span class="expert-resource-copy"><strong>{{ item.name }}</strong><small>{{ item.platform ? t('resources.platformSkills') : t('resources.mySkills') }}</small></span>
          <span class="expert-resource-check" aria-hidden="true"><Check v-if="skillIds.includes(item.id)" /></span>
        </button>
      </div>
      <p v-else class="expert-resource-empty">{{ t('common.empty') }}</p>
    </section>

    <section class="expert-resource-group">
      <header>
        <div><h3>{{ t('resources.connectors') }}</h3><p>{{ t('experts.selectConnectorsHint') }}</p></div>
        <span>{{ t('experts.selectedCount', { count: mcpServerIds.length + cliConnectorDefinitionIds.length }) }}</span>
      </header>
      <div v-if="mcpServers.length || cliConnectors.length" class="expert-resource-options">
        <button v-for="item in mcpServers" :key="`mcp-${item.id}`" type="button" class="expert-resource-option" :class="{ selected: mcpServerIds.includes(item.id) }" :disabled="!item.tested && !mcpServerIds.includes(item.id)" :aria-pressed="mcpServerIds.includes(item.id)" @click="toggleMCP(item)">
          <ConnectorIcon :icon="item.icon" :size="38" />
          <span class="expert-resource-copy"><strong>{{ item.name }}</strong><small>MCP · {{ item.tested ? t('experts.tested') : t('experts.testRequired') }}</small></span>
          <span class="expert-resource-check" aria-hidden="true"><Check v-if="mcpServerIds.includes(item.id)" /></span>
        </button>
        <button v-for="item in cliConnectors" :key="`cli-${item.id}`" type="button" class="expert-resource-option" :class="{ selected: cliConnectorDefinitionIds.includes(item.id) }" :disabled="!cliEnabled(item.id) && !cliConnectorDefinitionIds.includes(item.id)" :aria-pressed="cliConnectorDefinitionIds.includes(item.id)" @click="toggleCLI(item)">
          <ConnectorIcon :icon="item.icon" :size="38" />
          <span class="expert-resource-copy"><strong>{{ item.name }}</strong><small>{{ t('resources.cli') }} · {{ cliEnabled(item.id) ? t('common.enabled') : t('settings.unavailable') }}</small></span>
          <span class="expert-resource-check" aria-hidden="true"><Check v-if="cliConnectorDefinitionIds.includes(item.id)" /></span>
        </button>
      </div>
      <p v-else class="expert-resource-empty">{{ t('common.empty') }}</p>
    </section>
  </div>
</template>
