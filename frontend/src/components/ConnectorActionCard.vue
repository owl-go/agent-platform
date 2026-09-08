<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { ConnectorActionRequirement } from "../api/client";

const props = defineProps<{ action: ConnectorActionRequirement; busy?: boolean }>();
defineEmits<{ start: []; check: []; copy: []; cancel: [] }>();
const { t, locale } = useI18n();
const supported = new Set(["open_url", "copy_value", "check_status"]);
const unsupported = computed(() => props.action.contract_version !== 1 || props.action.actions.some((item) => !supported.has(item)));
const operation = computed(() => props.action.operation_phrase[locale.value] || props.action.operation_phrase[locale.value.startsWith("zh") ? "zh-CN" : "en"] || props.action.capability_id);
const actionURL = computed(() => props.action.action_url ? new URL(props.action.action_url, window.location.origin).toString() : "");
</script>

<template>
  <section class="connector-action-card" aria-live="polite">
    <div>
      <strong>{{ t('connectorActions.title', { connector: action.connector_name }) }}</strong>
      <span>{{ t(`connectorActions.reason.${action.reason}`, { operation }) }}</span>
      <small v-if="action.permissions.length">{{ t('connectorActions.permissions', { permissions: action.permissions.join('、') }) }}</small>
      <small v-if="unsupported">{{ t('connectorActions.unsupported') }}</small>
      <a v-else-if="actionURL" class="connector-action-link" :href="actionURL" target="_blank" rel="noreferrer">{{ actionURL }}</a>
    </div>
    <div class="connector-action-buttons">
      <template v-if="!unsupported">
        <el-button v-if="action.actions.includes('open_url')" size="small" type="primary" :loading="busy" @click="$emit('start')">{{ t('connectorActions.open') }}</el-button>
        <el-button v-if="action.actions.includes('copy_value') && actionURL" size="small" :disabled="busy" @click="$emit('copy')">{{ t('connectorActions.copy') }}</el-button>
        <el-button v-if="action.actions.includes('check_status')" size="small" :disabled="busy" @click="$emit('check')">{{ t('connectorActions.check') }}</el-button>
      </template>
      <el-button size="small" text :disabled="busy" @click="$emit('cancel')">{{ t('common.cancel') }}</el-button>
    </div>
  </section>
</template>
