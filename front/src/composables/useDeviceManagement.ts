import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "@/composables/useApi";
import {
  getDeploymentConfig,
  getRuntimeConnection,
} from "@/runtime/runtime-adapter";
import { getWebDeviceCredential, getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";
import { useDeviceManagementIntent, deviceManagementRequestConfig, type DeviceManagementIntent } from "@/composables/useDeviceManagementIntent";

type Runtime = { runtimeId: string; presence: string; runtimeSessionId?: string };
type Policy = { deviceId: string; coordinated: boolean; administrator: boolean; modeRevision: number; permissionRevision: number; providerEpoch: number; selectedRole: string };
type Device = { deviceId: string; platform: string; label: string; trustState: string; presence: string; lastHeartbeat?: string; runtimes?: Runtime[]; coordination?: Policy };
type LocalIdentity = { deviceId?: string; runtimeId?: string; platform?: string; [key: string]: any };

export function useDeviceManagement() {
  const api = useApi();
  const loading = ref(false);
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
  let disposed = false;
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
      if (notify) ElMessage.warning(String(err?.message || err));
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
    } catch (error) {
  console.error("[original-error]", error);
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
    } catch (error) {
  console.error("[original-error]", error);
return; }
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
    if (disposed) return;
    let refreshing = false;
    approvalTimer = setInterval(async () => {
      if (disposed || refreshing || !canAdminister.value) return;
      refreshing = true;
      try { await refreshPairingApprovals(); } catch (error) {
  console.error("[original-error]", error);
}
      finally { refreshing = false; }
    }, 2000);
  });
  onUnmounted(() => { disposed = true; if (approvalTimer) clearInterval(approvalTimer); });

  return {
    loading,
    busy,
    error,
    localError,
    devices,
    grantDevice,
    grantsOpen,
    callDevice,
    callOpen,
    approvals,
    localIdentity,
    deploymentMode,
    currentPolicy,
    canAdminister,
    coreConsoleDeviceId,
    coordinationAvailable,
    desktopAvailable,
    localMeshState,
    localMeshBound,
    onlineDeviceCount,
    refresh,
    refreshAll,
    refreshPairingApprovals,
    decidePairing,
    loadSync,
    syncLabel,
    probe,
    revoke,
    formatTime,
    changeAdministrator,
  };
}
