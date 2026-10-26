<template>
  <section class="channel-plugin-settings" data-amitia-ui="channels.plugin-settings">
    <header class="settings-heading">
      <div>
        <h2>{{ title }}</h2>
        <p>控制该渠道插件与其触发的 Agent 能力运行范围。</p>
      </div>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </header>

    <el-alert v-if="error" :title="error" type="warning" show-icon :closable="false" />

    <div class="settings-card">
      <div class="settings-row">
        <div class="settings-label">
          <strong>沙箱隔离</strong>
          <span>开启后插件进程只能写入沙箱目录，电脑上其他用户文件（家目录、外接磁盘、临时目录）不可读写；Agent 的读写与命令执行也固定在沙箱内。</span>
        </div>
        <el-switch :model-value="enabled" :loading="saving" @change="toggleEnabled" />
      </div>

      <div class="settings-row">
        <div class="settings-label">
          <strong>允许联网</strong>
          <span>关闭后沙箱内只能访问本机回环地址（如本机服务端口），外部网络请求会被拒绝。</span>
        </div>
        <el-switch :model-value="allowNetwork" :disabled="!enabled || saving" @change="toggleNetwork" />
      </div>

      <dl class="settings-facts">
        <div><dt>沙箱目录</dt><dd>{{ policy.sandboxRoot || "未创建" }}</dd></div>
        <div><dt>工作区标识</dt><dd>{{ policy.workspaceId || "—" }}</dd></div>
        <div><dt>实际生效</dt><dd>{{ enabled ? "已隔离（插件重启后完全生效）" : "未隔离，可访问本机文件" }}</dd></div>
      </dl>

      <p class="settings-hint">
        开启后，此后从该渠道新进来的会话都会把工作目录固定到沙箱目录内；Agent 的读写、执行命令都只能落在沙箱里。
        已经存在的会话保持原状，插件进程本身需要重启该插件才会切换到沙箱启动。
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { apiClient } from "@/composables/useApi";

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

const policy = ref<Policy>({ channelId: "", enabled: false, allowNetwork: true, workspaceId: "", sandboxRoot: "", revision: 0 });
const loading = ref(false);
const saving = ref(false);
const error = ref("");

const enabled = computed(() => policy.value.enabled === true);
const allowNetwork = computed(() => policy.value.allowNetwork !== false);

function unwrap(response: { data?: unknown }): Policy | null {
  const body = response?.data as { code?: number; msg?: string; data?: Policy } | undefined;
  if (!body || body.code !== 200 || !body.data) {
    error.value = String(body?.msg || "读取沙箱设置失败");
    return null;
  }
  return body.data;
}

async function load() {
  if (!props.channelId) return;
  loading.value = true;
  error.value = "";
  try {
    const response = await apiClient.get("/api/channels/sandbox-policies", { params: { channelId: props.channelId } });
    const next = unwrap(response as { data?: unknown });
    if (next) policy.value = next;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取沙箱设置失败";
  } finally {
    loading.value = false;
  }
}

async function save(next: Partial<Policy>) {
  if (!props.channelId || saving.value) return;
  saving.value = true;
  error.value = "";
  try {
    const response = await apiClient.put("/api/channels/sandbox-policies", {
      channelId: props.channelId,
      enabled: next.enabled ?? enabled.value,
      allowNetwork: next.allowNetwork ?? allowNetwork.value,
    });
    const saved = unwrap(response as { data?: unknown });
    if (saved) policy.value = saved;
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "保存沙箱设置失败";
  } finally {
    saving.value = false;
  }
}

function toggleEnabled(value: string | number | boolean) {
  void save({ enabled: value === true });
}

function toggleNetwork(value: string | number | boolean) {
  void save({ allowNetwork: value === true });
}

onMounted(load);
watch(() => props.channelId, load);
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
.settings-facts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin: 0; }
.settings-facts div { padding: 10px 12px; border-radius: var(--ac-radius-sm); background: var(--workbench-bg); }
.settings-facts dt { color: var(--text-muted); font-size: var(--ac-font-size-xs); }
.settings-facts dd { margin: 4px 0 0; overflow-wrap: anywhere; font-size: var(--ac-font-size-sm); }
.settings-hint { margin: 0; color: var(--text-muted); font-size: var(--ac-font-size-xs); line-height: 1.7; }
@media (max-width: 720px) {
  .settings-facts { grid-template-columns: 1fr; }
}
</style>
