<template>
  <div class="devices-page">
    <div id="device-pairing" class="page-head">
      <div>
        <h2>我的设备</h2>
        <p>查看云端绑定设备、Runtime 在线状态和同步进度。</p>
      </div>
      <div class="head-actions">
        <el-button :disabled="!localIdentity || joinBusy" @click="scannerOpen = true">扫描配对码</el-button>
        <el-button
          v-if="!localMeshBound"
          type="primary"
          :loading="joinBusy"
          :disabled="deploymentMode !== 'cloud' || !localIdentity"
          @click="joinCurrentDevice()"
        >
          配对当前设备
        </el-button>
        <el-button
          v-if="localMeshBound || desktopAvailable"
          type="primary"
          :loading="offerBusy"
          @click="generatePairingOffer"
        >
          生成设备配对二维码
        </el-button>
        <el-button :loading="loading" @click="refreshAll">刷新</el-button>
      </div>
    </div>
    <DevicePairScanner v-model="scannerOpen" @scanned="pairScanned" />
    <DeviceCapabilityGrants v-if="grantDevice" v-model:visible="grantsOpen" :device-id="grantDevice.deviceId" :label="grantDevice.label" :devices="devices" @changed="refresh" />
    <DeviceCoreCall v-if="callDevice" v-model:visible="callOpen" :device-id="callDevice.deviceId" :label="callDevice.label" />

    <el-alert
      v-if="desktopAvailable && deploymentMode !== 'cloud'"
      title="当前设备可提供局域网服务，也可以扫码使用另一台设备的 Core 服务。"
      type="info"
      show-icon
      :closable="false"
    />
    <el-alert v-if="localError" :title="localError" type="warning" show-icon :closable="false" />
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
    <el-card v-if="approvals.length" shadow="never">
      <template #header><strong>待批准的配对请求</strong></template>
      <div v-for="request in approvals" :key="request.requestId" class="detail-row">
        <span>{{ request.label || request.deviceId }} · {{ request.platform }}</span>
        <div>
          <el-button :loading="busy === request.requestId" @click="decidePairing(request, false)">拒绝</el-button>
          <el-button type="primary" :loading="busy === request.requestId" @click="decidePairing(request, true)">批准配对</el-button>
        </div>
      </div>
    </el-card>
    <el-card v-if="pairingOffer" shadow="never" class="pairing-offer-card">
      <template #header><strong>一次性设备配对 Offer</strong></template>
      <p class="muted">新设备扫描二维码时使用下面的 URI；当前版本也可以直接复制粘贴。Offer 使用一次或过期后失效。</p>
      <img v-if="pairingImage" :src="pairingImage" width="320" height="320" alt="一次性设备配对二维码" class="pairing-qr" />
      <el-input :model-value="pairingOffer" readonly type="textarea" :rows="3" />
      <div class="card-actions"><el-button size="small" @click="copyPairingOffer">复制 Offer</el-button></div>
    </el-card>

    <el-card id="current-device" shadow="never" class="current-device-card">
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
        <template v-if="currentPolicy">
          <div class="detail-row">
            <span>统筹模式</span>
            <el-switch :model-value="currentPolicy.coordinated" :disabled="!coordinationAvailable" :loading="policyBusy" @change="changeMode" />
          </div>
          <p class="muted">{{ !coordinationAvailable ? '统筹模式暂不可用，当前数据仍由现有服务管理。' : currentPolicy.coordinated ? '新对话、记忆及持续事项由 Core 保存和管理，使用 Core 角色。' : '使用本机角色，新对话、记忆及持续事项由本机保存和管理。' }} 历史数据保持原归属；AI 服务始终由 Core 提供。</p>
          <div class="detail-row"><span>云端管理员</span><el-tag :type="currentPolicy.administrator ? 'success' : 'info'">{{ currentPolicy.administrator ? '已授权' : '未授权' }}</el-tag></div>
        </template>
      </template>
      <el-empty v-else description="本机 Runtime 身份暂不可用" :image-size="56" />
    </el-card>

    <section id="bound-devices">
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
          <div v-if="device.coordination" class="detail-row"><span>统筹模式</span><span>{{ device.coordination.coordinated ? '开启' : '关闭' }}</span></div>
          <div v-if="device.coordination" class="detail-row"><span>管理员</span><span>{{ device.coordination.administrator ? '已授权' : '未授权' }}</span></div>

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
            <el-button v-if="device.trustState === 'trusted'" size="small" :disabled="!coordinationAvailable" @click="callDevice = device; callOpen = true">通过 Core 调用</el-button>
            <el-button v-if="device.trustState === 'trusted' && (canAdminister || device.deviceId === localIdentity?.deviceId)" size="small" @click="grantDevice = device; grantsOpen = true">能力授权</el-button>
            <el-button v-if="canAdminister && device.coordination" size="small" :disabled="!device.coordination.coordinated || device.trustState !== 'trusted'" :loading="busy === device.deviceId" @click="changeAdministrator(device)">{{ device.coordination.administrator ? '撤销管理员' : '授予管理员' }}</el-button>
            <el-button size="small" @click="loadSync(device.deviceId)">刷新同步状态</el-button>
            <el-button v-if="canAdminister || device.deviceId === localIdentity?.deviceId" size="small" type="danger" plain :loading="busy === device.deviceId" @click="revoke(device)">移除设备</el-button>
          </div>
        </el-card>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "@/composables/useApi";
import DevicePairScanner from '@/components/DevicePairScanner.vue';
import DeviceCapabilityGrants from '@/components/DeviceCapabilityGrants.vue';
import DeviceCoreCall from '@/components/DeviceCoreCall.vue';
import {
  createCurrentDevicePairingOffer,
  deprovisionCurrentDeviceMesh,
  getCurrentDevicePairingStatus,
  getDeploymentConfig,
  getRuntimeConnection,
  provisionCurrentDeviceMesh,
  saveDeploymentConfig,
} from "@/runtime/runtime-adapter";
import { getWebDeviceCredential, getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";

type Runtime = { runtimeId: string; presence: string; runtimeSessionId?: string };
type Policy = { deviceId: string; coordinated: boolean; administrator: boolean; modeRevision: number; permissionRevision: number; providerEpoch: number; selectedRole: string };
type Device = { deviceId: string; platform: string; label: string; trustState: string; presence: string; lastHeartbeat?: string; runtimes?: Runtime[]; coordination?: Policy };
type LocalIdentity = { deviceId?: string; runtimeId?: string; platform?: string; [key: string]: any };

const api = useApi();
const loading = ref(false);
const joinBusy = ref(false);
const scannerOpen = ref(false);
const offerBusy = ref(false);
const pairingOffer = ref("");
const pairingImage = ref("");
const leaveBusy = ref(false);
const busy = ref("");
const error = ref("");
const localError = ref("");
const devices = ref<Device[]>([]);
const grantDevice = ref<Device | null>(null);
const grantsOpen = ref(false);
const callDevice = ref<Device | null>(null);
const callOpen = ref(false);
type PairingApproval = { requestId: string; deviceId: string; platform: string; label: string; revision: number };
const approvals = ref<PairingApproval[]>([]);
let approvalTimer: ReturnType<typeof setInterval> | undefined;
const syncStatus = ref<Record<string, Record<string, any>>>({});
const localIdentity = ref<LocalIdentity | null>(null);
const localMeshStatus = ref<Record<string, any>>({});
const deploymentMode = ref("local");
const currentPolicy = ref<Policy | null>(null);
const canAdminister = ref(false);
const coordinationAvailable = ref(false);
const policyBusy = ref(false);
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
    const state = await api.get<{ policy: Policy; canAdminister: boolean; coordinationAvailable: boolean }>("/api/device-mesh/v1/coordination/me");
    currentPolicy.value = state.policy;
    canAdminister.value = state.canAdminister;
    coordinationAvailable.value = state.coordinationAvailable;
    await refreshPairingApprovals();
    await Promise.all(devices.value.map((device) => loadSync(device.deviceId, false)));
  } catch (err: any) {
    error.value = err?.message || "设备列表加载失败";
  } finally {
    loading.value = false;
  }
}

async function refreshPairingApprovals() {
  if (!canAdminister.value) { approvals.value = []; return; }
  const result = await api.get<{ requests: PairingApproval[] }>("/api/device-mesh/v1/pairing/approvals");
  approvals.value = result.requests || [];
}

async function decidePairing(request: PairingApproval, allow: boolean) {
  busy.value = request.requestId;
  try {
    await api.put(`/api/device-mesh/v1/pairing/approvals/${encodeURIComponent(request.requestId)}`, { allow, expectedRevision: request.revision });
    await refreshPairingApprovals();
    ElMessage.success(allow ? "已批准该设备配对" : "已拒绝该设备配对");
  } catch (error: any) { ElMessage.error(error?.message || "配对审批失败"); }
  finally { busy.value = ""; }
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
  if ((deploymentMode.value === "cloud" && localMeshBound.value) || (desktopAvailable.value && deploymentMode.value === "local")) {
    await refresh();
  } else {
    devices.value = [];
    error.value = "";
  }
}

async function joinCurrentDevice(scanned?: string) {
  if (!localIdentity.value) return;
  if (deploymentMode.value !== "cloud" && !scanned) {
    ElMessage.warning("请先切换到云端部署模式");
    return;
  }
  joinBusy.value = true;
  try {
    if (scanned?.startsWith("amitia://")) {
      const uri = new URL(scanned);
      if (uri.hostname !== "pair" || (uri.pathname !== "" && uri.pathname !== "/")) throw new Error("配对二维码格式无效");
      const endpoint = uri.searchParams.get("endpoint") || "";
      const fingerprint = uri.searchParams.get("fingerprint") || "";
      const coreId = uri.searchParams.get("core") || "";
      const offerToken = uri.searchParams.get("offer") || "";
      if (fingerprint) {
        if (!desktopAvailable.value) throw new Error("局域网身份验证需要桌面或手机客户端");
        if (!/^[0-9a-f]{64}$/.test(fingerprint) || !coreId || !offerToken) throw new Error("配对二维码身份信息不完整");
        await provisionCurrentDeviceMesh(endpoint, { offerToken, fingerprint, coreId });
        await saveDeploymentConfig({ mode: "cloud", serverURL: endpoint });
        ElMessage.success("已配对并使用扫码设备的 Core 服务");
        await refreshAll();
        return;
      }
    }
    const connection = await getRuntimeConnection();
    const status = await getCurrentDevicePairingStatus(connection.apiBaseURL);
    const first = status.firstDeviceSetupRequired === true;
    const prompt = scanned ? { value: scanned } : await ElMessageBox.prompt(
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
    if (scanned) await ElMessageBox.confirm(`确认使用当前 Cloud Core（${connection.apiBaseURL}）完成配对？`, '确认服务提供者', { confirmButtonText: '配对', cancelButtonText: '取消' });
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

async function pairScanned(raw: string) { await joinCurrentDevice(raw); }

async function generatePairingOffer() {
  offerBusy.value = true;
  try {
    const connection = await getRuntimeConnection();
    const offer = await createCurrentDevicePairingOffer(connection.apiBaseURL, 600);
    pairingOffer.value = String(offer.qrPayload || offer.offerToken || "").trim();
    pairingImage.value = String(offer.qrImage || "");
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

async function changeMode(value: string | number | boolean) {
  if (!currentPolicy.value || policyBusy.value) return;
  try {
    await ElMessageBox.confirm('切换将中断正在生成的回复。已有聊天、记忆及持续事项保留原归属，新数据按切换后的模式保存。', '切换统筹模式', { confirmButtonText: '切换', cancelButtonText: '取消' });
  } catch { return; }
  policyBusy.value = true;
  try {
    const result = await api.put<{ policy: Policy }>("/api/device-mesh/v1/coordination/me", { coordinated: Boolean(value), expectedRevision: currentPolicy.value.modeRevision, selectedRole: currentPolicy.value.selectedRole });
    currentPolicy.value = result.policy;
    window.dispatchEvent(new CustomEvent('amitia:execution-scope-changed', { detail: { reason: '统筹模式已切换，当前回复已中断', policy: result.policy } }));
    ElMessage.success('统筹模式已更新');
    await refresh();
  } catch (err: any) { ElMessage.error(err?.message || '统筹模式更新失败'); await refresh(); }
  finally { policyBusy.value = false; }
}

async function changeAdministrator(device: Device) {
  if (!device.coordination || !canAdminister.value) return;
  const grant = !device.coordination.administrator;
  try {
    await ElMessageBox.confirm(grant ? `「${device.label || device.deviceId}」将可以完整配置当前 Core 并查询 Core 数据。` : '撤销后该设备将立即失去 Core 管理权限。', grant ? '授予云端管理员' : '撤销云端管理员', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' });
  } catch { return; }
  busy.value = device.deviceId;
  try {
    await api.put(`/api/device-mesh/v1/devices/${encodeURIComponent(device.deviceId)}/administrator`, { grant, expectedRevision: device.coordination.permissionRevision });
    await refresh();
    ElMessage.success(grant ? '管理员权限已授予' : '管理员权限已撤销');
  } catch (err: any) { ElMessage.error(err?.message || '授权失败'); }
  finally { busy.value = ''; }
}

onMounted(async () => {
  await refreshAll();
  let refreshing = false;
  approvalTimer = setInterval(async () => {
    if (refreshing || !canAdminister.value) return;
    refreshing = true;
    try { await refreshPairingApprovals(); } catch {}
    finally { refreshing = false; }
  }, 2000);
});
onUnmounted(() => { if (approvalTimer) clearInterval(approvalTimer); });
</script>

<style scoped>
.devices-page { padding: 24px; display: flex; flex-direction: column; gap: 18px; }
.page-head, .device-head, .detail-row, .runtime-row, .card-actions, .head-actions, .device-head-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.page-head h2 { margin: 0 0 6px; font-size: calc(var(--ac-font-size-base) * 11 / 7); }
.page-head p { margin: 0; color: var(--el-text-color-secondary); font-size: var(--ac-font-size-sm); }
.device-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(340px, 100%), 1fr)); gap: 14px; }
.device-card, .current-device-card { min-width: 0; }
.pairing-qr { display: block; max-width: 100%; height: auto; margin: 12px auto; }
.muted, small { color: var(--el-text-color-secondary); font-size: var(--ac-font-size-xs); margin-top: 4px; }
.detail-row { padding: 7px 0; font-size: var(--ac-font-size-sm); }
.detail-row > span:first-child { flex-shrink: 0; color: var(--el-text-color-secondary); }
code { min-width: 0; overflow-wrap: anywhere; max-width: 65%; text-align: right; }
.runtime-list { margin-top: 12px; border-top: 1px solid var(--el-border-color-lighter); }
.runtime-row { padding: 9px 0; border-bottom: 1px solid var(--el-border-color-lighter); font-size: var(--ac-font-size-xs); }
.runtime-row > div { min-width: 0; overflow-wrap: anywhere; }
.card-actions, .head-actions, .device-head-actions { flex-wrap: wrap; }
.card-actions { justify-content: flex-end; margin-top: 14px; }
@media (max-width: 700px) { .devices-page { padding: 16px; } .device-grid { grid-template-columns: 1fr; } .page-head { align-items: flex-start; flex-direction: column; } .head-actions { width: 100%; flex-wrap: wrap; } }
</style>
