<template>
  <div class="devices-page">
    <div class="page-head">
      <div>
        <h2>我的设备</h2>
        <p>查看云端绑定设备、Runtime 在线状态和同步进度。</p>
      </div>
      <div class="head-actions">
        <el-button
          v-if="!localMeshBound"
          type="primary"
          :loading="joinBusy"
          :disabled="deploymentMode !== 'cloud' || !localIdentity"
          @click="joinCurrentDevice"
        >
          配对当前设备
        </el-button>
        <el-button
          v-if="localMeshBound"
          type="primary"
          :loading="offerBusy"
          :disabled="deploymentMode !== 'cloud'"
          @click="generatePairingOffer"
        >
          生成新设备配对 Offer
        </el-button>
        <el-button :loading="loading" @click="refreshAll">刷新</el-button>
      </div>
    </div>

    <el-alert
      v-if="desktopAvailable && deploymentMode !== 'cloud'"
      title="当前不是云端模式。将桌面 Runtime 加入 Device Mesh 前，请先在部署设置中切换到 Cloud Core。"
      type="info"
      show-icon
      :closable="false"
    />
    <el-alert v-if="localError" :title="localError" type="warning" show-icon :closable="false" />
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
    <el-card v-if="pairingOffer" shadow="never" class="pairing-offer-card">
      <template #header><strong>一次性设备配对 Offer</strong></template>
      <p class="muted">新设备扫描二维码时使用下面的 URI；当前版本也可以直接复制粘贴。Offer 使用一次或过期后失效。</p>
      <el-input :model-value="pairingOffer" readonly type="textarea" :rows="3" />
      <div class="card-actions"><el-button size="small" @click="copyPairingOffer">复制 Offer</el-button></div>
    </el-card>

    <el-card shadow="never" class="current-device-card">
      <template #header>
        <div class="device-head">
          <div>
            <strong>{{ desktopAvailable ? "当前桌面 Runtime" : "当前 Web 设备" }}</strong>
            <div class="muted">{{ desktopAvailable ? "本机身份来自 Device Agent，不由云端猜测" : "浏览器作为独立 Device Mesh 成员，以 DeviceCredential 访问 Cloud Core" }}</div>
          </div>
          <div class="device-head-actions">
            <el-tag :type="localMeshState === 'connected' ? 'success' : 'info'">{{ localMeshStateLabel }}</el-tag>
            <el-button
              v-if="localMeshBound"
              size="small"
              type="danger"
              plain
              :loading="leaveBusy"
              @click="leaveCurrentDeviceMesh"
            >
              解除本机云端绑定
            </el-button>
          </div>
        </div>
      </template>
      <template v-if="localIdentity">
        <div class="detail-row"><span>Device ID</span><code>{{ localIdentity.deviceId || "—" }}</code></div>
        <div class="detail-row"><span>Runtime ID</span><code>{{ localIdentity.runtimeId || "—" }}</code></div>
        <div class="detail-row"><span>平台</span><span>{{ localIdentity.platform || "—" }}</span></div>
      </template>
      <el-empty v-else description="本机 Runtime 身份暂不可用" :image-size="56" />
    </el-card>

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
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "@/composables/useApi";
import {
  createCurrentDevicePairingOffer,
  deprovisionCurrentDeviceMesh,
  getCurrentDevicePairingStatus,
  getDeploymentConfig,
  getRuntimeConnection,
  provisionCurrentDeviceMesh,
} from "@/runtime/runtime-adapter";
import { getWebDeviceCredential, getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";

type Runtime = { runtimeId: string; presence: string; runtimeSessionId?: string };
type Device = { deviceId: string; platform: string; label: string; trustState: string; presence: string; lastHeartbeat?: string; runtimes?: Runtime[] };
type LocalIdentity = { deviceId?: string; runtimeId?: string; platform?: string; [key: string]: any };

const api = useApi();
const loading = ref(false);
const joinBusy = ref(false);
const offerBusy = ref(false);
const pairingOffer = ref("");
const leaveBusy = ref(false);
const busy = ref("");
const error = ref("");
const localError = ref("");
const devices = ref<Device[]>([]);
const syncStatus = ref<Record<string, Record<string, any>>>({});
const localIdentity = ref<LocalIdentity | null>(null);
const localMeshStatus = ref<Record<string, any>>({});
const deploymentMode = ref("local");
const desktopAvailable = computed(() => Boolean(window.amitiaDesktop));
const localMeshState = computed(() => String(localMeshStatus.value?.state || "unknown").toLowerCase());
const localMeshBound = computed(() => !["", "unknown", "unprovisioned"].includes(localMeshState.value));
const localMeshStateLabel = computed(() => {
  if (["connected", "ready"].includes(localMeshState.value)) return "已加入 Mesh";
  if (localMeshState.value === "connecting") return "连接中";
  if (localMeshState.value === "unprovisioned") return "未绑定";
  return localMeshState.value === "unknown" ? "未知" : localMeshState.value;
});

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

async function loadCurrentDevice() {
  localError.value = "";
  try {
    const deployment = await getDeploymentConfig();
    deploymentMode.value = deployment.mode || "local";
    if (desktopAvailable.value) {
      const [identity, status] = await Promise.all([
        window.amitiaDesktop?.getMeshIdentity?.(),
        window.amitiaDesktop?.getMeshStatus?.(),
      ]);
      localIdentity.value = identity || null;
      localMeshStatus.value = status || {};
      return;
    }

    const connection = await getRuntimeConnection();
    const identity = getWebDeviceMeshIdentity();
    const credential = getWebDeviceCredential(connection.apiBaseURL);
    localIdentity.value = identity;
    localMeshStatus.value = credential
      ? { state: "connected", cloudBaseUrl: credential.cloudBaseURL, deviceId: credential.deviceId, runtimeId: credential.runtimeId }
      : { state: "unprovisioned", deviceId: identity.deviceId, runtimeId: identity.runtimeId };
  } catch (err: any) {
    localIdentity.value = null;
    localMeshStatus.value = {};
    localError.value = err?.message || "无法读取当前设备的 Device Mesh 身份";
  }
}

async function refreshAll() {
  await loadCurrentDevice();
  if (deploymentMode.value === "cloud" && localMeshBound.value) {
    await refresh();
  } else {
    devices.value = [];
    error.value = "";
  }
}

async function joinCurrentDevice() {
  if (!localIdentity.value) return;
  if (deploymentMode.value !== "cloud") {
    ElMessage.warning("请先切换到云端部署模式");
    return;
  }
  joinBusy.value = true;
  try {
    const connection = await getRuntimeConnection();
    const status = await getCurrentDevicePairingStatus(connection.apiBaseURL);
    const first = status.firstDeviceSetupRequired === true;
    const prompt = await ElMessageBox.prompt(
      first
        ? "这是 Cloud Core 的第一台可信设备。请输入 Cloud Core 主机本地生成的一次性设置码。"
        : "粘贴已信任设备生成的 amitia://pair?... 内容或 Offer Token。",
      first ? "首设备配对" : "设备配对",
      {
        confirmButtonText: "配对",
        cancelButtonText: "取消",
        inputPlaceholder: first ? "首设备设置码" : "amitia://pair?endpoint=...&offer=...",
        inputValidator: (value) => String(value || "").trim().length > 0 || "请输入配对信息",
      },
    );
    const raw = String(prompt.value || "").trim();
    let offerToken = "";
    if (!first) {
      if (raw.startsWith("amitia://")) {
        const uri = new URL(raw);
        const endpoint = uri.searchParams.get("endpoint") || "";
        if (endpoint && new URL(endpoint).origin !== new URL(connection.apiBaseURL).origin) {
          throw new Error("配对 Offer 属于另一个 Cloud Core");
        }
        offerToken = uri.searchParams.get("offer") || "";
      } else {
        offerToken = raw;
      }
      if (!offerToken) throw new Error("配对 Offer 无效");
    }
    await provisionCurrentDeviceMesh(connection.apiBaseURL, first ? { setupCode: raw } : { offerToken });
    ElMessage.success("当前设备已加入 Device Mesh");
    await refreshAll();
  } catch (err: any) {
    if (err === "cancel" || err === "close") return;
    ElMessage.error(err?.message || "当前设备加入 Device Mesh 失败");
  } finally {
    joinBusy.value = false;
  }
}

async function generatePairingOffer() {
  offerBusy.value = true;
  try {
    const connection = await getRuntimeConnection();
    const offer = await createCurrentDevicePairingOffer(connection.apiBaseURL, 600);
    pairingOffer.value = String(offer.qrPayload || offer.offerToken || "").trim();
    if (!pairingOffer.value) throw new Error("Cloud Core 未返回配对 Offer");
    ElMessage.success("一次性配对 Offer 已生成");
  } catch (err: any) {
    ElMessage.error(err?.message || "生成配对 Offer 失败");
  } finally {
    offerBusy.value = false;
  }
}

async function copyPairingOffer() {
  if (!pairingOffer.value) return;
  if (window.amitiaDesktop?.writeClipboardText) {
    await window.amitiaDesktop.writeClipboardText(pairingOffer.value);
  } else {
    await navigator.clipboard.writeText(pairingOffer.value);
  }
  ElMessage.success("配对 Offer 已复制");
}

async function leaveCurrentDeviceMesh() {
  try {
    await ElMessageBox.confirm(
      "确定解除当前设备的云端绑定吗？Cloud Core 会立即撤销该设备的所有 DeviceCredential，然后清除本机凭据；个人 Space 数据不会被删除。",
      "解除当前设备云端绑定",
      { type: "warning", confirmButtonText: "解除绑定", cancelButtonText: "取消" },
    );
  } catch {
    return;
  }

  leaveBusy.value = true;
  try {
    const connection = await getRuntimeConnection();
    await deprovisionCurrentDeviceMesh(connection.apiBaseURL);
    ElMessage.success("当前设备的 Device Mesh 凭据已清除");
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "解除本机 Device Mesh 绑定失败");
  } finally {
    leaveBusy.value = false;
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
  try {
    await ElMessageBox.confirm(`确定移除「${device.label || device.deviceId}」吗？该设备凭据会立即失效。`, "移除设备", { type: "warning", confirmButtonText: "移除", cancelButtonText: "取消" });
  } catch {
    return;
  }
  busy.value = device.deviceId;
  try {
    await api.del(`/api/device-mesh/v1/devices/${encodeURIComponent(device.deviceId)}`);
    ElMessage.success("设备已移除");
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "设备移除失败");
  } finally {
    busy.value = "";
  }
}

function formatTime(value?: string) {
  if (!value) return "暂无";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

onMounted(refreshAll);
</script>

<style scoped>
.devices-page { padding: 24px; display: flex; flex-direction: column; gap: 18px; }
.page-head, .device-head, .detail-row, .runtime-row, .card-actions, .head-actions, .device-head-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.page-head h2 { margin: 0 0 6px; font-size: 22px; }
.page-head p { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.device-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 14px; }
.device-card, .current-device-card { min-width: 0; }
.muted, small { color: var(--el-text-color-secondary); font-size: 12px; margin-top: 4px; }
.detail-row { padding: 7px 0; font-size: 13px; }
.detail-row > span:first-child { color: var(--el-text-color-secondary); }
code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 65%; }
.runtime-list { margin-top: 12px; border-top: 1px solid var(--el-border-color-lighter); }
.runtime-row { padding: 9px 0; border-bottom: 1px solid var(--el-border-color-lighter); font-size: 12px; }
.runtime-row > div { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.card-actions { justify-content: flex-end; margin-top: 14px; }
@media (max-width: 700px) { .devices-page { padding: 16px; } .device-grid { grid-template-columns: 1fr; } .page-head { align-items: flex-start; flex-direction: column; } .head-actions { width: 100%; flex-wrap: wrap; } }
</style>
