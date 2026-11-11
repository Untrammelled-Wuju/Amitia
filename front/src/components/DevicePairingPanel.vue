<template>
  <section class="device-pairing-panel" aria-labelledby="pairing-title">
    <div class="pairing-heading">
      <div>
        <h3 id="pairing-title">设备操作</h3>
        <p class="pairing-description">{{ localCenter ? "由子设备扫描当前统筹中心生成的二维码。" : "使用中心设备生成的二维码加入统筹中心。" }}</p>
      </div>
      <slot name="actions" />
    </div>
    <div class="pairing-actions">
      <template v-if="localCenter">
        <span class="pairing-control" :tabindex="busyReason ? 0 : undefined" :role="busyReason ? 'button' : undefined" :aria-label="busyReason || undefined" @click.capture="explainUnavailable($event, busyReason)" @keydown.enter="explainUnavailable($event, busyReason)" @keydown.space="explainUnavailable($event, busyReason)"><el-button type="primary" data-amitia-ui="device-pairing.generate" :loading="offerBusy" :disabled="!!busyReason" @click="generatePairingOffer">生成二维码</el-button></span>
      </template>
      <template v-else>
        <span class="pairing-control" :tabindex="joinReason ? 0 : undefined" :role="joinReason ? 'button' : undefined" :aria-label="joinReason || undefined" @click.capture="explainUnavailable($event, joinReason)" @keydown.enter="explainUnavailable($event, joinReason)" @keydown.space="explainUnavailable($event, joinReason)"><el-button type="primary" data-amitia-ui="device-pairing.scan" :disabled="!!joinReason" @click="scannerOpen = true">扫码配对</el-button></span>
        <span class="pairing-control" :tabindex="joinReason ? 0 : undefined" :role="joinReason ? 'button' : undefined" :aria-label="joinReason || undefined" @click.capture="explainUnavailable($event, joinReason)" @keydown.enter="explainUnavailable($event, joinReason)" @keydown.space="explainUnavailable($event, joinReason)"><el-button data-amitia-ui="device-pairing.image" :disabled="!!joinReason" @click="choosePairingImage">使用二维码图片</el-button></span>
      </template>
      <span class="pairing-control" :tabindex="manualReason ? 0 : undefined" :role="manualReason ? 'button' : undefined" :aria-label="manualReason || undefined" @click.capture="explainUnavailable($event, manualReason)" @keydown.enter="explainUnavailable($event, manualReason)" @keydown.space="explainUnavailable($event, manualReason)"><el-button data-amitia-ui="device-pairing.manual" :loading="joinBusy || offerBusy" :disabled="!!manualReason" @click="manualPairing">手动配对</el-button></span>
    </div>
    <input ref="pairingImageInput" class="pairing-image-input" type="file" accept="image/*" aria-label="选择配对二维码图片" @change="readPairingImage" />
    <el-alert v-if="localError" :title="localError" type="warning" show-icon :closable="false" />
    <details v-if="localErrorDetail"><summary>查看设备身份原始错误</summary><pre>{{ localErrorDetail }}</pre></details>
    <el-button v-if="localError" size="small" :loading="identityLoading" @click="loadCurrentDevice">重试读取设备身份</el-button>
    <el-alert v-if="offerNotice" :title="offerNotice" type="warning" show-icon :closable="false" />
    <DevicePairScanner v-if="!localCenter" v-model="scannerOpen" @scanned="joinCurrentDevice" />
    <div v-if="pairingOffer" class="pairing-offer">
      <h4>设备配对二维码</h4>
      <p class="pairing-description">子设备可在有效期内多次扫描该二维码加入当前 Core；每台设备仍需单独确认审批。</p>
      <img v-if="pairingImage" :src="pairingImage" width="320" height="320" alt="设备配对二维码" class="pairing-qr" />
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
import { BrowserQRCodeReader } from "@zxing/browser";
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
import { getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";
import type { PairingInput } from "@/runtime/web-device-mesh";
import { startPairingLANMonitor } from "@/runtime/pairing-lan-monitor";
import { originalErrorText } from "@/runtime/original-error";

const props = defineProps<{ serverURL: string; mode?: "local" | "cloud" }>();
const emit = defineEmits<{ paired: [baseURL: string] }>();
const api = useApi();
const localCenter = computed(() => props.mode !== "cloud");
const localIdentity = ref<{ deviceId?: string } | null>(null);
const localError = ref("");
const localErrorDetail = ref("");
const identityLoading = ref(false);
const scannerOpen = ref(false);
const pairingImageInput = ref<HTMLInputElement>();
const joinBusy = ref(false);
const offerBusy = ref(false);
const busyReason = computed(() => joinBusy.value || offerBusy.value ? "正在处理设备配对，请稍后重试。" : "");
const joinReason = computed(() => busyReason.value || (identityLoading.value ? "正在读取设备身份，请稍后重试。" : !localIdentity.value ? "设备身份尚未就绪，请检查页面中的错误信息。" : ""));
const manualReason = computed(() => localCenter.value ? busyReason.value : joinReason.value);
function explainUnavailable(event: Event, reason: string) {
  if (!reason) return;
  event.preventDefault();
  event.stopPropagation();
  ElMessage.warning(reason);
}
const pairingOffer = ref("");
const pairingImage = ref("");
const offerNotice = ref("");
let offerGeneration = 0;
let offerTimer: ReturnType<typeof setTimeout> | undefined;
let lanMonitor: AbortController | undefined;
let disposed = false;
const desktopAvailable = computed(() => Boolean(window.amitiaDesktop));
const management = useDeviceManagementIntent(() => {
  invalidateOffer("统筹中心连接或权限已变化，请重新生成二维码");
});

function invalidateOffer(message: string) {
  offerGeneration++;
  lanMonitor?.abort();
  lanMonitor = undefined;
  if (offerTimer) clearTimeout(offerTimer);
  offerTimer = undefined;
  if (pairingOffer.value || offerBusy.value) offerNotice.value = message;
  pairingOffer.value = "";
  pairingImage.value = "";
}

async function loadCurrentDevice() {
  if (identityLoading.value) return;
  identityLoading.value = true;
  localError.value = "";
  localErrorDetail.value = "";
  try {
    if (desktopAvailable.value) {
      const identity = await window.amitiaDesktop?.getMeshIdentity?.();
      localIdentity.value = identity || null;
    } else {
      const connection = await getRuntimeConnection();
      localIdentity.value = getWebDeviceMeshIdentity();
      void connection;
    }
    if (!localIdentity.value?.deviceId) throw new Error("设备未返回有效的 Device Mesh 身份");
  } catch (err) {
    localIdentity.value = null;
    localError.value = "无法读取当前设备的 Device Mesh 身份";
    localErrorDetail.value = originalErrorText(err);
  } finally {
    identityLoading.value = false;
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
  if (localCenter.value) return;
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
        first ? "这是统筹中心的第一台可信设备。请输入中心设备本地生成的一次性设置码。" : "粘贴已信任设备生成的 amitia://pair?... 内容或 Offer Token。",
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
        if (endpoint && new URL(endpoint).origin !== new URL(baseURL).origin) throw new Error("配对 Offer 属于另一个统筹中心");
        pairing = { offerToken: offer.searchParams.get("offer") || "" };
      } else {
        pairing = { offerToken: raw };
      }
      if (!first && !pairing.offerToken) throw new Error("配对 Offer 无效");
    }
    if (scanned) await ElMessageBox.confirm(`确认连接统筹中心（${baseURL}）完成配对？`, "确认服务提供者", { confirmButtonText: "配对", cancelButtonText: "取消" });
    if (disposed) return;
    await provisionCurrentDeviceMesh(baseURL, pairing);
    if (disposed) return;
    await saveDeploymentConfig({ mode: "cloud", serverURL: baseURL });
    await initializeRuntimeCapabilities(true);
    pairingOffer.value = "";
    pairingImage.value = "";
    await loadCurrentDevice();
    emit("paired", baseURL);
    ElMessage.success("已配对并使用该统筹中心");
  } catch (err: any) {
    if (err !== "cancel" && err !== "close") ElMessage.error(err?.message || "当前设备加入 Device Mesh 失败");
  } finally {
    joinBusy.value = false;
  }
}

async function generatePairingOffer() {
  if (!localCenter.value) return;
  if (offerBusy.value || joinBusy.value) return;
  invalidateOffer("");
  offerNotice.value = "";
  const generation = offerGeneration;
  offerBusy.value = true;
  let intent: Awaited<ReturnType<typeof management.capture>> | null = null;
  try {
    const deployment = await getDeploymentConfig();
    if (deployment.mode === "local" && desktopAvailable.value) {
      const monitor = new AbortController();
      lanMonitor = monitor;
      await startPairingLANMonitor(monitor,
        () => invalidateOffer("局域网地址已变化，请重新生成二维码"),
        (error) => invalidateOffer(`局域网地址监测中断，请重新生成二维码：${error instanceof Error ? error.stack || error.message : String(error)}`),
      );
      if (disposed || generation !== offerGeneration) return;
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
    const offer = await createCurrentDevicePairingOffer(connection.apiBaseURL, 1800);
    if (disposed || generation !== offerGeneration) return;
    if (intent) await management.validate(intent);
    if (disposed || generation !== offerGeneration) return;
    const expiresAt = Date.parse(offer.expiresAt);
    if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) throw new Error(`配对二维码有效期无效：${offer.expiresAt}`);
    pairingOffer.value = String(offer.qrPayload || offer.offerToken || "").trim();
    pairingImage.value = String(offer.qrImage || "");
    if (!pairingOffer.value) throw new Error("统筹中心未返回配对 Offer");
    offerTimer = setTimeout(() => invalidateOffer("配对二维码已过期，请重新生成"), expiresAt - Date.now());
    ElMessage.success("30 分钟有效的设备配对二维码已生成");
  } catch (err: any) {
    if (generation === offerGeneration) {
      invalidateOffer("");
      if (err !== "cancel" && err !== "close") ElMessage.error(err instanceof Error ? err.stack || err.message : String(err));
    }
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

function choosePairingImage() {
  if (localCenter.value) return;
  pairingImageInput.value?.click();
}

async function readPairingImage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file || localCenter.value) return;
  const url = URL.createObjectURL(file);
  try {
    const result = await new BrowserQRCodeReader().decodeFromImageUrl(url);
    await joinCurrentDevice(result.getText());
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    URL.revokeObjectURL(url);
  }
}

async function manualPairing() {
  if (localCenter.value) {
    await generatePairingOffer();
    return;
  }
  await joinCurrentDevice();
}

onMounted(loadCurrentDevice);
onBeforeUnmount(() => { disposed = true; invalidateOffer(""); });
</script>

<style scoped>
.device-pairing-panel { margin-top: 20px; min-width: 0; }
.device-pairing-panel h3 { margin: 0; font-size: 16px; font-weight: 600; }
.pairing-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.pairing-description { color: var(--text-secondary); font-size: 12px; line-height: 1.6; }
.pairing-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.pairing-actions .el-button { margin-left: 0; }
.pairing-control { display: inline-flex; }
.pairing-control[role="button"] :deep(.el-button) { pointer-events: none; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
.pairing-image-input { display: none; }
.pairing-offer { margin-top: 20px; }
.pairing-offer h4 { margin: 0; font-size: 14px; }
.pairing-qr { display: block; max-width: 100%; height: auto; margin: 12px auto; }
.pairing-details { margin-top: 14px; }
.pairing-details summary { margin-bottom: 10px; cursor: pointer; color: var(--text-secondary); font-size: 12px; }
</style>
