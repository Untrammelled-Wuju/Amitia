<template>
  <section class="device-pairing-panel" aria-labelledby="pairing-title">
    <h3 id="pairing-title">连接设备</h3>
    <p class="pairing-description">扫描其他 Core 的配对码加入，或生成当前 Core 的二维码供其他设备扫码添加。</p>
    <div class="pairing-actions">
      <el-button type="primary" :disabled="!localIdentity || joinBusy || offerBusy" @click="scannerOpen = true">扫描配对码</el-button>
      <el-button v-if="localMeshBound || desktopAvailable" :loading="offerBusy" :disabled="joinBusy" @click="generatePairingOffer">生成配对二维码</el-button>
      <el-button :loading="joinBusy" :disabled="!localIdentity || offerBusy" @click="joinCurrentDevice()">手动配对</el-button>
    </div>
    <el-alert v-if="localError" :title="localError" type="warning" show-icon :closable="false" />
    <DevicePairScanner v-model="scannerOpen" @scanned="joinCurrentDevice" />
    <div v-if="pairingOffer" class="pairing-offer">
      <h4>一次性设备配对 Offer</h4>
      <p class="pairing-description">其他设备可扫描下方二维码加入当前 Core。配对信息使用一次或过期后失效。</p>
      <img v-if="pairingImage" :src="pairingImage" width="320" height="320" alt="一次性设备配对二维码" class="pairing-qr" />
      <el-button size="small" @click="copyPairingOffer">复制配对信息</el-button>
      <details class="pairing-details">
        <summary>查看原始配对信息</summary>
        <el-input :model-value="pairingOffer" readonly type="textarea" :rows="3" />
      </details>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import DevicePairScanner from "@/components/DevicePairScanner.vue";
import { useApi } from "@/composables/useApi";
import { useDeviceManagementIntent, deviceManagementRequestConfig } from "@/composables/useDeviceManagementIntent";
import {
  createCurrentDevicePairingOffer,
  getCurrentDevicePairingStatus,
  getDeploymentConfig,
  getRuntimeConnection,
  provisionCurrentDeviceMesh,
  saveDeploymentConfig,
} from "@/runtime/runtime-adapter";
import { initializeRuntimeCapabilities } from "@/runtime/runtime-capabilities";
import { getWebDeviceCredential, getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";
import type { PairingInput } from "@/runtime/web-device-mesh";

const props = defineProps<{ serverURL: string }>();
const emit = defineEmits<{ paired: [baseURL: string] }>();
const api = useApi();
const localIdentity = ref<{ deviceId?: string } | null>(null);
const localMeshBound = ref(false);
const localError = ref("");
const scannerOpen = ref(false);
const joinBusy = ref(false);
const offerBusy = ref(false);
const pairingOffer = ref("");
const pairingImage = ref("");
let disposed = false;
const desktopAvailable = computed(() => Boolean(window.amitiaDesktop));
const management = useDeviceManagementIntent(() => {
  pairingOffer.value = "";
  pairingImage.value = "";
});

async function loadCurrentDevice() {
  localError.value = "";
  try {
    if (desktopAvailable.value) {
      const [identity, status] = await Promise.all([
        window.amitiaDesktop?.getMeshIdentity?.(),
        window.amitiaDesktop?.getMeshStatus?.(),
      ]);
      localIdentity.value = identity || null;
      const state = String(status?.state || "unknown").toLowerCase();
      localMeshBound.value = !["", "unknown", "unprovisioned"].includes(state);
    } else {
      const connection = await getRuntimeConnection();
      localIdentity.value = getWebDeviceMeshIdentity();
      localMeshBound.value = Boolean(getWebDeviceCredential(connection.apiBaseURL));
    }
  } catch (err: any) {
    localIdentity.value = null;
    localMeshBound.value = false;
    localError.value = err?.message || "无法读取当前设备的 Device Mesh 身份";
  }
}

function pairingURI(raw: string) {
  const uri = new URL(raw);
  if (uri.protocol !== "amitia:" || uri.hostname !== "pair" || (uri.pathname !== "" && uri.pathname !== "/")) throw new Error("配对二维码格式无效");
  return uri;
}

function coreURL(raw: string) {
  const url = new URL(raw);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) throw new Error("请输入有效的 Core 服务器地址");
  return raw.trim().replace(/\/+$/, "");
}

async function joinCurrentDevice(scanned?: string) {
  if (!localIdentity.value || joinBusy.value || offerBusy.value) return;
  joinBusy.value = true;
  try {
    const uri = scanned?.startsWith("amitia://") ? pairingURI(scanned) : null;
    const target = uri?.searchParams.get("endpoint") || props.serverURL.trim();
    if (!target) throw new Error("请先填写 Core 服务器地址，或扫描包含地址的配对二维码");
    const baseURL = coreURL(target);
    const fingerprint = uri?.searchParams.get("fingerprint") || "";
    let pairing: PairingInput;
    if (fingerprint) {
      if (!desktopAvailable.value) throw new Error("局域网身份验证需要桌面或手机客户端");
      const coreId = uri?.searchParams.get("core") || "";
      const offerToken = uri?.searchParams.get("offer") || "";
      if (!/^[0-9a-f]{64}$/.test(fingerprint) || !coreId || !offerToken) throw new Error("配对二维码身份信息不完整");
      pairing = { offerToken, fingerprint, coreId };
    } else {
      const status = await getCurrentDevicePairingStatus(baseURL);
      const first = status.firstDeviceSetupRequired === true;
      const prompt = scanned ? { value: scanned } : await ElMessageBox.prompt(
        first ? "这是 Cloud Core 的第一台可信设备。请输入 Cloud Core 主机本地生成的一次性设置码。" : "粘贴已信任设备生成的 amitia://pair?... 内容或 Offer Token。",
        first ? "首设备配对" : "设备配对",
        {
          confirmButtonText: "配对",
          cancelButtonText: "取消",
          inputPlaceholder: first ? "首设备设置码" : "amitia://pair?endpoint=...&offer=...",
          inputValidator: (value) => String(value || "").trim().length > 0 || "请输入配对信息",
        },
      );
      const raw = String(prompt.value || "").trim();
      if (first) {
        if (uri) throw new Error("该 Core 需要首设备设置码，请使用手动配对");
        pairing = { setupCode: raw };
      } else if (raw.startsWith("amitia://")) {
        const offer = pairingURI(raw);
        const endpoint = offer.searchParams.get("endpoint");
        if (endpoint && new URL(endpoint).origin !== new URL(baseURL).origin) throw new Error("配对 Offer 属于另一个 Cloud Core");
        pairing = { offerToken: offer.searchParams.get("offer") || "" };
      } else {
        pairing = { offerToken: raw };
      }
      if (!first && !pairing.offerToken) throw new Error("配对 Offer 无效");
    }
    if (scanned) await ElMessageBox.confirm(`确认连接 Cloud Core（${baseURL}）完成配对？`, "确认服务提供者", { confirmButtonText: "配对", cancelButtonText: "取消" });
    if (disposed) return;
    await provisionCurrentDeviceMesh(baseURL, pairing);
    if (disposed) return;
    await saveDeploymentConfig({ mode: "cloud", serverURL: baseURL });
    await initializeRuntimeCapabilities(true);
    pairingOffer.value = "";
    pairingImage.value = "";
    await loadCurrentDevice();
    emit("paired", baseURL);
    ElMessage.success("已配对并使用该设备的 Core 服务");
  } catch (err: any) {
    if (err !== "cancel" && err !== "close") ElMessage.error(err?.message || "当前设备加入 Device Mesh 失败");
  } finally {
    joinBusy.value = false;
  }
}

async function generatePairingOffer() {
  if (offerBusy.value || joinBusy.value) return;
  offerBusy.value = true;
  let intent: Awaited<ReturnType<typeof management.capture>> | null = null;
  try {
    const deployment = await getDeploymentConfig();
    if (deployment.mode === "local" && desktopAvailable.value) {
      intent = await management.capture();
      const result = await api.get<{ devices?: { deviceId: string; trustState: string }[] }>("/api/device-mesh/v1/devices", undefined, deviceManagementRequestConfig(intent));
      await management.validate(intent);
      const current = result.devices?.find((device) => device.deviceId === localIdentity.value?.deviceId);
      if (intent.state.coreConsoleDeviceId === localIdentity.value?.deviceId && current && current.trustState !== "trusted") {
        await ElMessageBox.confirm("当前 Core 主机的配对权限已失效。是否使用本机所有者身份恢复权限并生成配对二维码？其他设备的撤销状态不会改变。", "恢复本机配对权限", { type: "warning", confirmButtonText: "恢复并生成", cancelButtonText: "取消" });
        await management.validate(intent);
        await api.post("/api/device-mesh/v1/pairing/recover-local-device", { coreId: intent.state.coreId, deviceId: intent.state.coreConsoleDeviceId }, deviceManagementRequestConfig(intent));
      }
    }
    const connection = await getRuntimeConnection();
    const offer = await createCurrentDevicePairingOffer(connection.apiBaseURL, 600);
    pairingOffer.value = String(offer.qrPayload || offer.offerToken || "").trim();
    pairingImage.value = String(offer.qrImage || "");
    if (!pairingOffer.value) throw new Error("Cloud Core 未返回配对 Offer");
    ElMessage.success("一次性配对 Offer 已生成");
  } catch (err: any) {
    if (err !== "cancel" && err !== "close") ElMessage.error(err?.message || "生成配对 Offer 失败");
  } finally {
    management.release(intent);
    offerBusy.value = false;
  }
}

async function copyPairingOffer() {
  if (!pairingOffer.value) return;
  if (window.amitiaDesktop?.writeClipboardText) await window.amitiaDesktop.writeClipboardText(pairingOffer.value);
  else await navigator.clipboard.writeText(pairingOffer.value);
  ElMessage.success("配对 Offer 已复制");
}

onMounted(loadCurrentDevice);
onBeforeUnmount(() => { disposed = true; });
</script>

<style scoped>
.device-pairing-panel { margin-top: 20px; min-width: 0; }
.device-pairing-panel h3 { margin: 0; font-size: 16px; font-weight: 600; }
.pairing-description { color: var(--text-secondary); font-size: 12px; line-height: 1.6; }
.pairing-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.pairing-actions .el-button { margin-left: 0; }
.pairing-offer { margin-top: 20px; }
.pairing-offer h4 { margin: 0; font-size: 14px; }
.pairing-qr { display: block; max-width: 100%; height: auto; margin: 12px auto; }
.pairing-details { margin-top: 14px; }
.pairing-details summary { margin-bottom: 10px; cursor: pointer; color: var(--text-secondary); font-size: 12px; }
</style>
