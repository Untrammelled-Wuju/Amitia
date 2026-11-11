<template>
  <section class="channel-plugin-settings" data-amitia-ui="channels.plugin-settings">
    <header class="settings-heading">
      <div>
        <h2>{{ title }}</h2>
        <p>控制该渠道插件与其触发的 Agent 能力运行范围。</p>
      </div>
      <span class="settings-control" :tabindex="saving ? 0 : undefined" :role="saving ? 'button' : undefined" :aria-label="saving ? '正在保存渠道设置，请稍后重试。' : undefined" @click.capture="explainUnavailable($event, saving ? '正在保存渠道设置，请稍后重试。' : '')" @keydown.enter="explainUnavailable($event, saving ? '正在保存渠道设置，请稍后重试。' : '')" @keydown.space="explainUnavailable($event, saving ? '正在保存渠道设置，请稍后重试。' : '')"><el-button :loading="loading" :disabled="saving" @click="load">刷新</el-button></span>
    </header>

    <el-alert v-if="error" :title="error" type="warning" show-icon :closable="false" />
    <details v-if="errorDetail"><summary>查看原始错误</summary><pre>{{ errorDetail }}</pre></details>

    <div class="settings-card">
      <div class="settings-row">
        <div class="settings-label">
          <strong>沙箱隔离</strong>
          <span>开启后插件进程只能写入沙箱目录，电脑上其他用户文件（家目录、外接磁盘、临时目录）不可读写；Agent 的读写与命令执行也固定在沙箱内。</span>
        </div>
        <span class="settings-control" :tabindex="controlReason ? 0 : undefined" :role="controlReason ? 'button' : undefined" :aria-label="controlReason || undefined" @click.capture="explainUnavailable($event, controlReason)" @keydown.enter="explainUnavailable($event, controlReason)" @keydown.space="explainUnavailable($event, controlReason)"><el-switch :model-value="enabled" :loading="saving" :disabled="!!controlReason" @change="toggleEnabled" /></span>
      </div>

      <div class="settings-row">
        <div class="settings-label">
          <strong>允许联网</strong>
          <span>关闭后沙箱内只能访问本机回环地址（如本机服务端口），外部网络请求会被拒绝。</span>
        </div>
        <span class="settings-control" :tabindex="networkReason ? 0 : undefined" :role="networkReason ? 'button' : undefined" :aria-label="networkReason || undefined" @click.capture="explainUnavailable($event, networkReason)" @keydown.enter="explainUnavailable($event, networkReason)" @keydown.space="explainUnavailable($event, networkReason)"><el-switch :model-value="allowNetwork" :disabled="!!networkReason" @change="toggleNetwork" /></span>
      </div>

      <dl class="settings-facts">
        <div><dt>沙箱目录</dt><dd>{{ policy?.sandboxRoot || "未创建" }}</dd></div>
        <div><dt>工作区标识</dt><dd>{{ policy?.workspaceId || "—" }}</dd></div>
        <div><dt>实际生效</dt><dd>{{ !policy || loadFailed ? "状态不可用" : enabled ? "已隔离（插件重启后完全生效）" : "未隔离，可访问本机文件" }}</dd></div>
      </dl>

      <p class="settings-hint">
        开启后，此后从该渠道新进来的会话都会把工作目录固定到沙箱目录内；Agent 的读写、执行命令都只能落在沙箱里。
        已经存在的会话保持原状，插件进程本身需要重启该插件才会切换到沙箱启动。
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage, ElNotification } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { originalErrorText } from "@/runtime/original-error";

const props = defineProps<{
  channelId: string;
  title?: string;
}>();

interface Policy {
  channelId: string;
  enabled: boolean;
  allowNetwork: boolean;
  workspaceId: string;
  sandboxRoot: string;
  revision: number;
}

const policy = ref<Policy | null>(null);
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const errorDetail = ref("");
const loadFailed = ref(false);
let channelGeneration = 0;
let loadGeneration = 0;

const enabled = computed(() => policy.value?.enabled === true);
const allowNetwork = computed(() => policy.value?.allowNetwork !== false);
const controlReason = computed(() => loading.value ? "正在读取渠道设置，请稍后重试。" : saving.value ? "正在保存渠道设置，请稍后重试。" : loadFailed.value ? "无法读取渠道设置，请查看原始错误并重试。" : !policy.value ? "渠道设置尚未准备好，请先刷新。" : "");
const networkReason = computed(() => controlReason.value || (!enabled.value ? "请先开启沙箱隔离。" : ""));
function explainUnavailable(event: Event, reason: string) {
  if (!reason) return;
  event.preventDefault();
  event.stopPropagation();
  ElMessage.warning(reason);
}
function reportOldFailure(title: string, cause: unknown) {
  ElNotification({ title, type: "error", duration: 0, message: h("pre", { style: "white-space: pre-wrap; overflow-wrap: anywhere; max-height: 240px; overflow: auto; user-select: text" }, originalErrorText(cause)) });
}

function unwrap(response: { data?: unknown }): Policy {
  const body = response?.data as { code?: number; msg?: string; data?: Policy } | undefined;
  if (!body || body.code !== 200 || !body.data) {
    throw response;
  }
  return body.data;
}

async function load() {
  if (saving.value) { ElMessage.warning("正在保存渠道设置，请稍后重试。"); return; }
  const channel = props.channelId;
  const generation = channelGeneration;
  const request = ++loadGeneration;
  if (!channel) { loading.value = false; policy.value = null; loadFailed.value = true; error.value = "渠道标识为空，无法读取渠道设置。"; errorDetail.value = ""; return; }
  loading.value = true;
  error.value = "";
  errorDetail.value = "";
  try {
    const response = await apiClient.get("/api/channels/sandbox-policies", { params: { channelId: channel } });
    const next = unwrap(response as { data?: unknown });
    if (next.channelId !== channel) throw response;
    if (generation !== channelGeneration || channel !== props.channelId || request !== loadGeneration) return;
    policy.value = next;
    loadFailed.value = false;
  } catch (cause) {
    if (generation !== channelGeneration || channel !== props.channelId || request !== loadGeneration) { reportOldFailure(`渠道 ${channel} 读取失败`, cause); return; }
    error.value = "读取沙箱设置失败，请重试";
    errorDetail.value = originalErrorText(cause);
    loadFailed.value = true;
  } finally {
    if (generation === channelGeneration && channel === props.channelId && request === loadGeneration) loading.value = false;
  }
}

async function save(next: Partial<Policy>) {
  if (controlReason.value || !props.channelId) { ElMessage.warning(controlReason.value || "渠道标识为空。"); return; }
  const channel = props.channelId;
  const generation = channelGeneration;
  saving.value = true;
  error.value = "";
  errorDetail.value = "";
  try {
    const response = await apiClient.put("/api/channels/sandbox-policies", {
      channelId: channel,
      enabled: next.enabled ?? enabled.value,
      allowNetwork: next.allowNetwork ?? allowNetwork.value,
    });
    const saved = unwrap(response as { data?: unknown });
    if (saved.channelId !== channel) throw response;
    if (generation === channelGeneration && channel === props.channelId) policy.value = saved;
  } catch (cause) {
    if (generation !== channelGeneration || channel !== props.channelId) { reportOldFailure(`渠道 ${channel} 保存失败`, cause); return; }
    error.value = "保存沙箱设置失败，请重试";
    errorDetail.value = originalErrorText(cause);
  } finally {
    if (generation === channelGeneration && channel === props.channelId) saving.value = false;
  }
}

function toggleEnabled(value: string | number | boolean) {
  void save({ enabled: value === true });
}

function toggleNetwork(value: string | number | boolean) {
  if (networkReason.value) { ElMessage.warning(networkReason.value); return; }
  void save({ allowNetwork: value === true });
}

onMounted(load);
watch(() => props.channelId, () => { channelGeneration++; saving.value = false; policy.value = null; void load(); });
onBeforeUnmount(() => { channelGeneration++; loadGeneration++; });
</script>

<style scoped>
.channel-plugin-settings { display: flex; flex-direction: column; min-height: 0; height: 100%; overflow: auto; padding: 16px 20px 24px; gap: 14px; }
.settings-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.settings-heading h2 { margin: 0; font-size: 18px; }
.settings-heading p { margin: 5px 0 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
.settings-card { display: flex; flex-direction: column; gap: 16px; padding: 18px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-md); background: var(--surface-bg); }
.settings-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; }
.settings-label { display: grid; gap: 5px; min-width: 0; }
.settings-label strong { font-size: var(--ac-font-size-sm); }
.settings-label span { color: var(--text-secondary); font-size: var(--ac-font-size-xs); line-height: 1.6; }
.settings-control { display: inline-flex; }
.settings-control[role="button"] :deep(.el-switch),
.settings-control[role="button"] :deep(.el-button) { pointer-events: none; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
.settings-facts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin: 0; }
.settings-facts div { padding: 10px 12px; border-radius: var(--ac-radius-sm); background: var(--workbench-bg); }
.settings-facts dt { color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.settings-facts dd { margin: 4px 0 0; overflow-wrap: anywhere; font-size: var(--ac-font-size-sm); }
.settings-hint { margin: 0; color: var(--text-muted); font-size: var(--ac-font-size-xs); line-height: 1.7; }
@media (max-width: 720px) {
  .settings-facts { grid-template-columns: 1fr; }
}
</style>
