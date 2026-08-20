<template>
  <div class="devices-page">
    <div class="page-head">
      <div>
        <h2>我的设备</h2>
        <p>查看云端绑定设备、Runtime 在线状态和同步进度。</p>
      </div>
      <el-button :loading="loading" @click="refresh">刷新</el-button>
    </div>

    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />

    <el-empty v-if="!loading && devices.length === 0" description="暂无已绑定设备" />
    <div v-else class="device-grid" v-loading="loading">
      <el-card v-for="device in devices" :key="device.deviceId" shadow="never" class="device-card">
        <template #header>
          <div class="device-head">
            <div>
              <strong>{{ device.label || device.deviceId }}</strong>
              <div class="muted">{{ device.platform }} · {{ device.trustState || "unknown" }}</div>
            </div>
            <el-tag :type="device.presence === 'online' ? 'success' : 'info'">{{ device.presence === "online" ? "在线" : "离线" }}</el-tag>
          </div>
        </template>

        <div class="detail-row"><span>Device ID</span><code>{{ device.deviceId }}</code></div>
        <div class="detail-row"><span>最后心跳</span><span>{{ formatTime(device.lastHeartbeat) }}</span></div>
        <div class="detail-row"><span>Runtime</span><span>{{ device.runtimes?.length || 0 }}</span></div>
        <div class="detail-row"><span>同步</span><span>{{ syncLabel(device.deviceId) }}</span></div>

        <div v-if="device.runtimes?.length" class="runtime-list">
          <div v-for="runtime in device.runtimes" :key="runtime.runtimeId" class="runtime-row">
            <div>
              <div>{{ runtime.runtimeId }}</div>
              <small>{{ runtime.presence }}</small>
            </div>
            <el-button size="small" @click="probe(device.deviceId, runtime.runtimeId)">探测</el-button>
          </div>
        </div>

        <div class="card-actions">
          <el-button size="small" @click="loadSync(device.deviceId)">刷新同步状态</el-button>
          <el-button size="small" type="danger" plain :loading="busy === device.deviceId" @click="revoke(device)">移除设备</el-button>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "@/composables/useApi";

type Runtime = { runtimeId: string; presence: string; runtimeSessionId?: string };
type Device = { deviceId: string; platform: string; label: string; trustState: string; presence: string; lastHeartbeat?: string; runtimes?: Runtime[] };

const api = useApi();
const loading = ref(false);
const busy = ref("");
const error = ref("");
const devices = ref<Device[]>([]);
const syncStatus = ref<Record<string, Record<string, any>>>({});

async function refresh() {
  loading.value = true;
  error.value = "";
  try {
    const result = await api.get<{ devices?: Device[] }>("/api/device-mesh/v1/devices");
    devices.value = result?.devices ?? [];
    await Promise.all(devices.value.map((device) => loadSync(device.deviceId, false)));
  } catch (err: any) {
    error.value = err?.message || "设备列表加载失败";
  } finally {
    loading.value = false;
  }
}

async function loadSync(deviceId: string, notify = true) {
  try {
    const result = await api.get<Record<string, any>>("/api/v1/sync/status", { deviceId });
    syncStatus.value = { ...syncStatus.value, [deviceId]: result || {} };
    if (notify) ElMessage.success("同步状态已刷新");
  } catch (err: any) {
    syncStatus.value = { ...syncStatus.value, [deviceId]: { error: err?.message || "不可用" } };
    if (notify) ElMessage.warning("同步状态暂不可用");
  }
}

function syncLabel(deviceId: string) {
  const status = syncStatus.value[deviceId];
  if (!status) return "读取中";
  if (status.error) return status.error;
  const lastApplied = status.lastApplied ?? status.lastAppliedSequence ?? status.cursor;
  const latest = status.latest ?? status.latestSequence ?? status.head;
  if (lastApplied != null && latest != null) return `${lastApplied} / ${latest}`;
  if (lastApplied != null) return `已应用 ${lastApplied}`;
  return status.status || "正常";
}

async function probe(deviceId: string, runtimeId: string) {
  try {
    const result = await api.post<Record<string, any>>(`/api/device-mesh/v1/devices/${encodeURIComponent(deviceId)}/runtimes/${encodeURIComponent(runtimeId)}/probe`);
    ElMessage.success(result?.ok === false ? "探测请求已返回" : "Runtime 探测成功");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "Runtime 探测失败");
  }
}

async function revoke(device: Device) {
  await ElMessageBox.confirm(`确定移除「${device.label || device.deviceId}」吗？该设备凭据会立即失效。`, "移除设备", { type: "warning", confirmButtonText: "移除", cancelButtonText: "取消" });
  busy.value = device.deviceId;
  try {
    await api.del(`/api/device-mesh/v1/devices/${encodeURIComponent(device.deviceId)}`);
    ElMessage.success("设备已移除");
    await refresh();
  } finally {
    busy.value = "";
  }
}

function formatTime(value?: string) {
  if (!value) return "暂无";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

onMounted(refresh);
</script>

<style scoped>
.devices-page { padding: 24px; display: flex; flex-direction: column; gap: 18px; }
.page-head, .device-head, .detail-row, .runtime-row, .card-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.page-head h2 { margin: 0 0 6px; font-size: 22px; }
.page-head p { margin: 0; color: var(--el-text-color-secondary); }
.device-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 14px; }
.device-card { min-width: 0; }
.muted, small { color: var(--el-text-color-secondary); font-size: 12px; margin-top: 4px; }
.detail-row { padding: 7px 0; font-size: 13px; }
.detail-row > span:first-child { color: var(--el-text-color-secondary); }
code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 65%; }
.runtime-list { margin-top: 12px; border-top: 1px solid var(--el-border-color-lighter); }
.runtime-row { padding: 9px 0; border-bottom: 1px solid var(--el-border-color-lighter); font-size: 12px; }
.runtime-row > div { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.card-actions { justify-content: flex-end; margin-top: 14px; }
@media (max-width: 700px) { .devices-page { padding: 16px; } .device-grid { grid-template-columns: 1fr; } }
</style>
