<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { platformApiKey, type ConnectorInstallation } from "../api/client";

const api = inject(platformApiKey)!;
const items = ref<ConnectorInstallation[]>([]);
const busy = ref(false);
const error = ref("");
const showGuided = ref(false);
const form = ref({ source: "", version: "1.0.0", type: "mcp" as "mcp" | "cli", name: "", description: "", auth_mode: "none" as "none" | "oauth" | "cli", mcp_json: "", cli_json: "", skill_name: "main", skill_markdown: "" });

async function refresh() {
  try { items.value = await api.listConnectorInstallations(); error.value = ""; }
  catch { error.value = "连接器安装状态暂时不可用"; }
}
async function upload(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  busy.value = true;
  try { await api.uploadConnectorPackage(file); await refresh(); }
  catch { error.value = "连接器包校验失败，请检查 ZIP 结构和清单"; }
  finally { busy.value = false; (event.target as HTMLInputElement).value = ""; }
}
async function disable(item: ConnectorInstallation) {
  busy.value = true;
  try { await api.disableConnectorInstallation(item.id, item.version); await refresh(); }
  catch { error.value = "连接器状态已变化，请刷新后重试"; }
  finally { busy.value = false; }
}
async function uninstall(item: ConnectorInstallation) {
  busy.value = true;
  try { await api.uninstallConnector(item.id, item.version); await refresh(); }
  catch { error.value = "卸载失败，请刷新后重试"; }
  finally { busy.value = false; }
}
async function createGuided() {
  busy.value = true;
  try { await api.createConnectorPackage(form.value); showGuided.value = false; await refresh(); }
  catch { error.value = "引导创建失败，请检查清单 JSON 和 Skill 内容"; }
  finally { busy.value = false; }
}
onMounted(() => { void refresh(); });
</script>

<template>
  <section class="connector-package-panel" aria-label="连接器包">
    <div class="resource-toolbar">
      <strong>连接器包</strong>
      <label class="el-button el-button--primary compact-action" :class="{ 'is-disabled': busy }">
        上传 ZIP
        <input type="file" accept=".zip,application/zip" :disabled="busy" hidden @change="upload">
      </label>
      <el-button :disabled="busy" @click="showGuided = true">引导创建</el-button>
    </div>
    <p v-if="error" class="form-error">{{ error }}</p>
    <div v-if="items.length" class="connector-package-list">
      <article v-for="item in items" :key="item.id" class="el-card connector-package-item">
        <div><strong>{{ item.source }}</strong><small>{{ item.state }} · {{ item.authorized ? '已连接' : '未连接' }}</small></div>
        <div class="connector-package-actions"><el-button size="small" :disabled="busy || item.state !== 'active'" @click="disable(item)">禁用</el-button><el-button size="small" type="danger" plain :disabled="busy" @click="uninstall(item)">卸载</el-button></div>
      </article>
    </div>
    <p v-else class="muted">暂无统一连接器包。</p>
    <div v-if="showGuided" class="modal-layer" @click.self="showGuided = false"><form class="modal-card el-card" @submit.prevent="createGuided"><h2>引导创建连接器</h2><label>Source<input v-model="form.source" required pattern="[a-z0-9]+(-[a-z0-9]+)*"></label><label>版本<input v-model="form.version" required></label><label>模式<select v-model="form.type"><option value="mcp">MCP</option><option value="cli">CLI</option></select></label><label>名称<input v-model="form.name" required></label><label>描述<textarea v-model="form.description" required rows="3"></textarea></label><label>Skill 名称<input v-model="form.skill_name" required></label><label>Skill 内容<textarea v-model="form.skill_markdown" required rows="6"></textarea></label><label>模式清单 JSON<textarea v-if="form.type === 'mcp'" v-model="form.mcp_json" required rows="5" placeholder='{"transport":"streamable_http",...}'></textarea><textarea v-else v-model="form.cli_json" required rows="5" placeholder='{"runtime":...}'></textarea></label><div class="modal-actions"><el-button @click="showGuided = false">取消</el-button><el-button native-type="submit" type="primary" :loading="busy">创建</el-button></div></form></div>
  </section>
</template>
