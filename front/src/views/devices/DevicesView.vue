<template>
  <div class="devices-page">
    <section id="device-pairing" class="device-hero">
      <div class="device-hero-icon"><Connection class="device-hero-svg" /></div>
      <div class="device-hero-copy">
        <h2>设备互联</h2>
        <p>管理当前设备、可信设备与 Core 之间的连接和权限。</p>
      </div>
      <span class="device-hero-status" :data-state="localMeshState">
        <i></i>{{ localMeshStateLabel }}
      </span>
      <el-button :loading="loading" @click="refreshAll">刷新状态</el-button>
    </section>
    <section class="device-overview" aria-label="设备概览">
      <div>
        <div class="overview-heading"><span class="overview-label">当前设备</span><span class="overview-index">01</span></div>
        <strong>{{ desktopAvailable ? "桌面端" : "Web 端" }}</strong>
        <span class="overview-subtitle">{{ localIdentity?.platform || "正在读取设备信息" }}</span>
      </div>
      <div>
        <div class="overview-heading"><span class="overview-label">已绑定设备</span><span class="overview-index">02</span></div>
        <strong>{{ devices.length }}</strong>
        <span class="overview-subtitle">台可信设备</span>
      </div>
      <div>
        <div class="overview-heading"><span class="overview-label">在线设备</span><span class="overview-index">03</span></div>
        <strong>{{ onlineDeviceCount }}</strong>
        <span class="overview-subtitle">台设备在线</span>
      </div>
    </section>
    <section class="pairing-control" aria-labelledby="pairing-title">
      <div class="panel-heading">
        <div>
          <h3 id="pairing-title">连接设备</h3>
          <p>通过扫码或一次性配对码连接新设备。</p>
        </div>
      </div>
      <div class="pairing-actions">
        <el-button type="primary" :disabled="!localIdentity || joinBusy" @click="scannerOpen = true">扫描配对码</el-button>
        <el-button v-if="localMeshBound || desktopAvailable" :loading="offerBusy" @click="generatePairingOffer">生成配对二维码</el-button>
        <el-button v-if="!localMeshBound" :loading="joinBusy" :disabled="deploymentMode !== 'cloud' || !localIdentity" @click="joinCurrentDevice()">手动配对</el-button>
      </div>
    </section>
    <DevicePairScanner v-model="scannerOpen" @scanned="pairScanned" />
    <DeviceCapabilityGrants v-if="grantDevice" v-model:visible="grantsOpen" :device-id="grantDevice.deviceId" :label="grantDevice.label" :devices="devices" @changed="refresh" />
    <DeviceCoreCall v-if="callDevice" v-model:visible="callOpen" :device-id="callDevice.deviceId" :label="callDevice.label" />

    <p v-if="desktopAvailable && deploymentMode !== 'cloud'" class="pairing-hint">当前电脑可以提供局域网 Core 服务，也可以扫码连接其他设备。</p>
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
      <div class="card-actions"><el-button size="small" @click="copyPairingOffer">复制配对信息</el-button></div>
      <details class="device-detail-toggle">
        <summary>查看原始配对信息</summary>
        <el-input :model-value="pairingOffer" readonly type="textarea" :rows="3" />
      </details>
    </el-card>

    <section id="current-device" class="current-device-card" aria-labelledby="current-device-title">
      <div class="panel-heading">
        <div>
          <h3 id="current-device-title">当前设备</h3>
          <p>{{ localIdentity?.platform || (desktopAvailable ? "桌面端" : "浏览器") }} · {{ localMeshStateLabel }}</p>
        </div>
        <el-tag size="small" :type="localMeshState === 'connected' ? 'success' : 'info'" effect="plain">{{ localMeshStateLabel }}</el-tag>
      </div>
      <details v-if="localIdentity" class="device-detail-toggle">
        <summary>设备信息与管理</summary>
        <div class="device-detail-body">
          <div class="detail-row"><span>Device ID</span><code>{{ localIdentity.deviceId || "—" }}</code></div>
          <div class="detail-row"><span>Runtime ID</span><code>{{ localIdentity.runtimeId || "—" }}</code></div>
          <div v-if="currentPolicy" class="device-policy">
            <div class="detail-row">
              <span>运行模式</span>
              <el-tag size="small" :type="deploymentMode === 'cloud' ? 'warning' : 'success'">
                {{ deploymentMode === "cloud" ? "云端统筹" : "本地模式" }}
              </el-tag>
            </div>
            <p class="muted">{{ deploymentMode === 'cloud' ? '角色、对话、记忆和跨设备任务由 Cloud Core 统一管理。' : '项目服务、角色、对话和记忆全部在当前设备运行。' }}</p>
            <div class="detail-row"><span>云端管理员</span><el-tag :type="currentPolicy.administrator ? 'success' : 'info'">{{ currentPolicy.administrator ? '已授权' : '未授权' }}</el-tag></div>
          </div>
          <div v-if="localMeshBound" class="card-actions">
            <el-button size="small" type="danger" plain :loading="leaveBusy" @click="leaveCurrentDeviceMesh">解除本机云端绑定</el-button>
          </div>
        </div>
      </details>
      <el-empty v-else description="本机 Runtime 身份暂不可用" :image-size="56" />
    </section>

    <section id="bound-devices" class="bound-devices-panel">
      <div class="panel-heading">
        <div>
          <h3>已绑定设备 <span class="device-count">{{ devices.length }}</span></h3>
          <p>按设备查看状态，展开后可管理权限和同步。</p>
        </div>
      </div>
      <el-empty v-if="!loading && devices.length === 0" description="暂无已绑定设备" :image-size="70" />
      <div v-else class="device-list" v-loading="loading">
        <el-card v-for="device in devices" :key="device.deviceId" shadow="never" class="device-card">
          <div class="device-head">
            <div class="device-title">
              <strong>{{ device.label || device.deviceId }}</strong>
              <div class="muted">{{ device.platform }} · {{ formatTime(device.lastHeartbeat) }}</div>
            </div>
            <el-tag size="small" :type="device.trustState === 'revoked' ? 'danger' : device.presence === 'online' ? 'success' : 'info'" effect="plain">{{ device.trustState === 'revoked' ? "已撤销" : device.presence === "online" ? "在线" : "离线" }}</el-tag>
          </div>
          <details class="device-detail-toggle">
            <summary>查看详情与管理</summary>
            <div class="device-detail-body">
              <div class="detail-row"><span>Device ID</span><code>{{ device.deviceId }}</code></div>
              <div class="detail-row"><span>信任状态</span><span>{{ device.trustState || "未知" }}</span></div>
              <div class="detail-row"><span>Runtime</span><span>{{ device.runtimes?.length || 0 }}</span></div>
              <div class="detail-row"><span>同步</span><span>{{ syncLabel(device.deviceId) }}</span></div>
              <div v-if="device.coordination" class="detail-row"><span>运行模式</span><span>云端统筹</span></div>
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
                <el-button v-if="canAdminister && device.coordination" size="small" :disabled="device.trustState !== 'trusted'" :loading="busy === device.deviceId" @click="changeAdministrator(device)">{{ device.coordination.administrator ? '撤销管理员' : '授予管理员' }}</el-button>
                <el-button size="small" :disabled="device.trustState !== 'trusted' || (!canAdminister && device.deviceId !== managementIntent?.state.policy.deviceId)" @click="loadSync(device.deviceId)">刷新同步状态</el-button>
                <el-button v-if="(canAdminister || device.deviceId === localIdentity?.deviceId) && device.deviceId !== coreConsoleDeviceId" size="small" type="danger" plain :loading="busy === device.deviceId" @click="revoke(device)">移除设备</el-button>
              </div>
            </div>
          </details>
        </el-card>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Connection } from "@element-plus/icons-vue";
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
import { useDeviceManagementIntent, deviceManagementRequestConfig, type DeviceManagementIntent } from "@/composables/useDeviceManagementIntent";

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
const coreConsoleDeviceId = ref("");
const coordinationAvailable = ref(false);
const desktopAvailable = computed(() => Boolean(window.amitiaDesktop));
let managementIntent: DeviceManagementIntent | null = null;
const management = useDeviceManagementIntent(() => {
  managementIntent = null;
  approvals.value = [];
  devices.value = [];
  syncStatus.value = {};
  canAdminister.value = false;
  coreConsoleDeviceId.value = "";
  error.value = "Core或原设备管理权限已变化，请刷新";
});

async function originalManagement(target?: string, administrator = false) {
  const intent = managementIntent;
  if (!intent) throw new Error("请先重新加载原设备管理页面");
  if (administrator && !intent.state.canAdminister) throw new Error("当前设备没有Core管理员权限");
  if (target && !intent.state.canAdminister && intent.state.policy.deviceId !== target) throw new Error("只能管理本设备或使用当前Core管理员权限");
  await management.validate(intent);
  return intent;
}
const localMeshState = computed(() => String(localMeshStatus.value?.state || "unknown").toLowerCase());
const localMeshBound = computed(() => !["", "unknown", "unprovisioned"].includes(localMeshState.value));
const onlineDeviceCount = computed(() => devices.value.filter((device) => device.trustState !== "revoked" && device.presence === "online").length);
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
    management.release(managementIntent);
    managementIntent = null;
    const intent = await management.capture();
    const result = await api.get<{ devices?: Device[] }>("/api/device-mesh/v1/devices", undefined, deviceManagementRequestConfig(intent));
    await management.validate(intent);
    managementIntent = intent;
    devices.value = result?.devices ?? [];
    syncStatus.value = {};
    const state = intent.state;
    currentPolicy.value = { ...state.policy, selectedRole: state.policy.selectedRole || "" };
    canAdminister.value = state.canAdminister;
    coreConsoleDeviceId.value = state.coreConsoleDeviceId || "";
    coordinationAvailable.value = state.coordinationAvailable === true;
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
  const intent = await originalManagement(undefined, true);
  const result = await api.get<{ requests: PairingApproval[] }>("/api/device-mesh/v1/pairing/approvals", undefined, deviceManagementRequestConfig(intent));
  await management.validate(intent);
  if (intent !== managementIntent) return;
  approvals.value = result.requests || [];
}

async function decidePairing(request: PairingApproval, allow: boolean) {
  busy.value = request.requestId;
  try {
    const intent = await originalManagement(undefined, true);
    await api.put(`/api/device-mesh/v1/pairing/approvals/${encodeURIComponent(request.requestId)}`, { allow, expectedRevision: request.revision }, deviceManagementRequestConfig(intent));
    await management.validate(intent);
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
    ElMessage.warning("云端统筹模式必须先配置 Cloud Core");
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
    const intent = managementIntent;
    const currentDevice = devices.value.find((device) => device.deviceId === localIdentity.value?.deviceId);
    if (deploymentMode.value === "local" && intent && intent.state.coreConsoleDeviceId === localIdentity.value?.deviceId && currentDevice && currentDevice.trustState !== "trusted") {
      await ElMessageBox.confirm("当前 Core 主机的配对权限已失效。是否使用本机所有者身份恢复权限并生成配对二维码？其他设备的撤销状态不会改变。", "恢复本机配对权限", { type: "warning", confirmButtonText: "恢复并生成", cancelButtonText: "取消" });
      if (intent !== managementIntent || intent.controller.signal.aborted) throw new Error("本机 Core 已变化，请刷新后重试");
      await management.validate(intent);
      await api.post("/api/device-mesh/v1/pairing/recover-local-device", { coreId: intent.state.coreId, deviceId: intent.state.coreConsoleDeviceId }, deviceManagementRequestConfig(intent));
      await refresh();
    }
    const connection = await getRuntimeConnection();
    const offer = await createCurrentDevicePairingOffer(connection.apiBaseURL, 600);
    pairingOffer.value = String(offer.qrPayload || offer.offerToken || "").trim();
    pairingImage.value = String(offer.qrImage || "");
    if (!pairingOffer.value) throw new Error("Cloud Core 未返回配对 Offer");
    ElMessage.success("一次性配对 Offer 已生成");
  } catch (err: any) {
    if (err === "cancel" || err === "close") return;
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
  const original = managementIntent;
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
    if (!original || original !== managementIntent) throw new Error("原设备管理页面已变化，请重新确认");
    await management.validate(original);
    await deprovisionCurrentDeviceMesh(original.apiBaseURL, original);
    ElMessage.success("当前设备的 Device Mesh 凭据已清除");
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "解除本机 Device Mesh 绑定失败");
  } finally {
    leaveBusy.value = false;
  }
}

async function loadSync(deviceId: string, notify = true) {
  const intent = managementIntent;
  if (!intent) return;
  if (!intent.state.canAdminister && intent.state.policy.deviceId !== deviceId) return;
  if (devices.value.find((device) => device.deviceId === deviceId)?.trustState !== "trusted") return;
  try {
    const result = await api.get<Record<string, any>>("/api/v1/sync/status", { deviceId }, deviceManagementRequestConfig(intent));
    await management.validate(intent);
    if (intent !== managementIntent) return;
    syncStatus.value = { ...syncStatus.value, [deviceId]: result || {} };
    if (notify) ElMessage.success("同步状态已刷新");
  } catch (err: any) {
    if (intent.controller.signal.aborted || intent !== managementIntent) return;
    syncStatus.value = { ...syncStatus.value, [deviceId]: { error: err?.message || "不可用" } };
    if (notify) ElMessage.warning("同步状态暂不可用");
  }
}

function syncLabel(deviceId: string) {
  const device = devices.value.find((device) => device.deviceId === deviceId);
  if (device?.trustState === "revoked") return "已撤销，不再同步";
  if (device?.trustState !== "trusted") return "未受信任，暂不可同步";
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
    const intent = await originalManagement(deviceId);
    const result = await api.post<Record<string, any>>(`/api/device-mesh/v1/devices/${encodeURIComponent(deviceId)}/runtimes/${encodeURIComponent(runtimeId)}/probe`, undefined, deviceManagementRequestConfig(intent));
    await management.validate(intent);
    ElMessage.success(result?.ok === false ? "探测请求已返回" : "Runtime 探测成功");
    await refresh();
  } catch (err: any) {
    ElMessage.error(err?.message || "Runtime 探测失败");
  }
}

async function revoke(device: Device) {
  const original = managementIntent;
  try {
    await ElMessageBox.confirm(`确定移除「${device.label || device.deviceId}」吗？该设备凭据会立即失效。`, "移除设备", { type: "warning", confirmButtonText: "移除", cancelButtonText: "取消" });
  } catch {
    return;
  }
  busy.value = device.deviceId;
  try {
    if (!original || original !== managementIntent) throw new Error("原设备管理页面已变化，请重新确认");
    const intent = await originalManagement(device.deviceId);
    await api.del(`/api/device-mesh/v1/devices/${encodeURIComponent(device.deviceId)}`, deviceManagementRequestConfig(intent));
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

async function changeAdministrator(device: Device) {
  if (!device.coordination || !canAdminister.value) return;
  const original = managementIntent;
  const revision = device.coordination.permissionRevision;
  const grant = !device.coordination.administrator;
  try {
    await ElMessageBox.confirm(grant ? `「${device.label || device.deviceId}」将可以完整配置当前 Core 并查询 Core 数据。` : '撤销后该设备将立即失去 Core 管理权限。', grant ? '授予云端管理员' : '撤销云端管理员', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' });
  } catch { return; }
  busy.value = device.deviceId;
  try {
    if (!original || original !== managementIntent) throw new Error("原设备管理页面已变化，请重新确认");
    const intent = await originalManagement(undefined, true);
    await api.put(`/api/device-mesh/v1/devices/${encodeURIComponent(device.deviceId)}/administrator`, { grant, expectedRevision: revision }, deviceManagementRequestConfig(intent));
    if (device.deviceId !== intent.state.policy.deviceId) await management.validate(intent);
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
.devices-page { width: min(100%, 1040px); min-width: 0; display: flex; flex-direction: column; gap: 18px; }
.devices-page :deep(.el-card) { border: 0; background: var(--surface-bg-elevated); box-shadow: none; }
.devices-page :deep(.el-card__header) { border-bottom: 0; }
.device-hero { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; align-items: center; gap: 16px; padding: 28px; border-radius: 24px; background: #2C2C2E; }
.device-hero-icon { width: 56px; height: 56px; display: grid; place-items: center; border-radius: 16px; background: color-mix(in srgb, var(--ac-color-primary) 12%, var(--control-hover-bg)); color: var(--ac-color-primary); }
.device-hero-svg { width: 23px; height: 23px; }
.device-hero-copy { min-width: 0; }
.device-hero-copy h2 { margin: 0; font-size: 20px; line-height: 1.35; }
.device-hero-copy p { margin: 6px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.device-hero-status { display: inline-flex; align-items: center; gap: 7px; padding: 6px 10px; border-radius: 999px; background: color-mix(in srgb, var(--workbench-bg) 68%, transparent); color: var(--text-secondary); font-size: 11px; white-space: nowrap; }
.device-hero-status i { width: 7px; height: 7px; border-radius: 50%; background: var(--text-secondary); }
.device-hero-status[data-state="connected"] i, .device-hero-status[data-state="ready"] i { background: var(--ac-color-success); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ac-color-success) 14%, transparent); }
.device-head, .detail-row, .runtime-row, .card-actions, .head-actions, .device-head-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.device-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(340px, 100%), 1fr)); gap: 14px; }
.device-card, .current-device-card { min-width: 0; }
.pairing-qr { display: block; max-width: 100%; height: auto; margin: 12px auto; }
.muted, small { color: var(--el-text-color-secondary); font-size: var(--ac-font-size-xs); margin-top: 4px; }
.detail-row { padding: 7px 0; font-size: var(--ac-font-size-sm); }
.detail-row > span:first-child { flex-shrink: 0; color: var(--el-text-color-secondary); }
code { min-width: 0; overflow-wrap: anywhere; max-width: 65%; text-align: right; }
.runtime-list { margin-top: 14px; display: grid; gap: 8px; }
.runtime-row { padding: 10px 12px; border-radius: 10px; background: var(--control-hover-bg); font-size: var(--ac-font-size-xs); }
.runtime-row > div { min-width: 0; overflow-wrap: anywhere; }
.card-actions, .head-actions, .device-head-actions { flex-wrap: wrap; }
.card-actions { justify-content: flex-end; margin-top: 14px; }
@media (max-width: 700px) { .device-grid { grid-template-columns: 1fr; } .head-actions { width: 100%; flex-wrap: wrap; } }
.device-overview { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.device-overview > div { display: grid; gap: 7px; min-width: 0; padding: 20px; border-radius: 18px; background: var(--surface-bg-elevated); }
.overview-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.overview-label { font-size: 12px; color: var(--text-secondary); }
.overview-index { color: var(--text-muted); font-size: 10px; letter-spacing: 0.08em; }
.device-overview strong { font-size: 22px; font-weight: 680; letter-spacing: -0.03em; }
.overview-subtitle { font-size: 12px; color: var(--text-secondary); }
.pairing-control, .current-device-card, .bound-devices-panel { min-width: 0; border-radius: 20px; padding: 24px; background: var(--surface-bg-elevated); }
.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
.panel-heading h3 { margin: 0; font-size: 16px; font-weight: 600; }
.panel-heading p { margin: 6px 0 0; font-size: 12px; color: var(--text-secondary); line-height: 1.5; }
.pairing-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.device-detail-toggle { margin-top: 14px; }
.device-detail-toggle > summary { padding: 14px 0 2px; cursor: pointer; color: var(--text-secondary); font-size: 13px; }
.device-detail-body { padding-top: 14px; }
.device-policy { margin-top: 14px; padding: 14px; border-radius: 12px; background: color-mix(in srgb, var(--workbench-bg) 54%, transparent); }
.device-count { display: inline-flex; margin-left: 6px; font-size: 12px; font-weight: 500; color: var(--text-secondary); }
.device-list { display: grid; gap: 12px; }
.device-card { border: 0; border-radius: 14px; background: var(--workbench-bg); box-shadow: none; }
.device-card :deep(.el-card__body) { padding: 16px 18px; }
.device-title { min-width: 0; }
.device-title strong { font-size: 14px; overflow-wrap: anywhere; }
.device-head { align-items: flex-start; }
.detail-row { align-items: flex-start; }
.detail-row code { font-size: 12px; overflow-wrap: anywhere; }
.card-actions { padding-top: 16px; }
.pairing-offer-card { border-radius: 16px; }
.pairing-hint { margin: -7px 0 0; color: var(--text-secondary); font-size: 12px; line-height: 1.6; }
@media (max-width: 820px) { .device-hero { grid-template-columns: auto minmax(0, 1fr); } .device-hero-status, .device-hero > .el-button { grid-column: auto; justify-self: start; } .device-overview { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 700px) { .device-hero { padding: 20px; } .pairing-control, .current-device-card, .bound-devices-panel { padding: 18px; } .device-overview > div { padding: 16px; } .pairing-actions .el-button { margin-left: 0; } }
</style>
