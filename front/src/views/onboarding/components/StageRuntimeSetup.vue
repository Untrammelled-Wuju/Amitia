<template>
  <div class="ob-stage-inner ob-setup-stage-inner ob-flow-stage-inner">
    <div class="ob-boot-panel ob-setup-step-panel">
      <h2 class="ob-boot-title">
        {{ deployMode === "local" ? "正在检查运行环境" : "正在连接 Cloud Core" }}
      </h2>
      <div class="ob-boot-copy">
        {{
          deployMode === "local"
            ? "系统将检查本地核心服务、运行时就绪状态和能力接口。"
            : "Cloud Core 使用设备凭证认证。检查完成后，未绑定的当前设备必须先通过一次性配对再继续。"
        }}
      </div>
      <div class="ob-boot-list">
        <div v-for="(row, idx) in bootRows" :key="idx" class="ob-boot-row" :class="row.state">
          <span class="ob-boot-dot"></span>
          <span class="ob-boot-name">{{ row.name }}</span>
          <span class="ob-boot-state">{{ row.stateText }}</span>
        </div>
      </div>

      <div v-if="pairingRequired" class="ob-pairing-panel">
        <div class="ob-input-label">
          {{ firstDeviceSetupRequired ? "首设备设置码" : "一次性配对 Offer" }}
        </div>
        <div class="ob-boot-copy pairing-copy">
          {{
            firstDeviceSetupRequired
              ? "这是该 Space 的第一台可信设备。设置码只在 Cloud Core 主机本地生成，不会通过网络公开。"
              : "请从任意已信任设备生成 amitia://pair?...，也可以直接粘贴 Offer Token。"
          }}
        </div>
        <input
          v-model="pairingValue"
          class="ob-field"
          :placeholder="firstDeviceSetupRequired ? '输入首设备设置码' : 'amitia://pair?endpoint=...&offer=...'"
          :disabled="pairingBusy"
          @keyup.enter="pairCurrentDevice"
        />
        <div v-if="pairingError" class="ob-pairing-error">{{ pairingError }}</div>
        <button class="ob-stage-action ob-pairing-action" type="button" :disabled="pairingBusy || !pairingValue.trim()" @click="pairCurrentDevice">
          {{ pairingBusy ? "正在配对" : "配对当前设备并继续" }}
        </button>
      </div>

      <div v-else-if="webAccessSetupRequired" class="ob-pairing-panel">
        <div class="ob-input-label">设置 Web 访问密码</div>
        <div class="ob-boot-copy pairing-copy">
          该密码只用于浏览器访问 Cloud Core。Flutter、Electron、Device Agent 和插件不需要输入。
        </div>
        <input
          v-model="webAccessPassword"
          class="ob-field"
          type="password"
          autocomplete="new-password"
          maxlength="128"
          placeholder="至少 8 个字符"
          :disabled="webAccessBusy"
        />
        <input
          v-model="webAccessPasswordConfirm"
          class="ob-field"
          type="password"
          autocomplete="new-password"
          maxlength="128"
          placeholder="再次输入访问密码"
          :disabled="webAccessBusy"
          @keyup.enter="setupBrowserAccess"
        />
        <div v-if="webAccessError" class="ob-pairing-error">{{ webAccessError }}</div>
        <button
          class="ob-stage-action ob-pairing-action"
          type="button"
          :disabled="webAccessBusy || !webAccessPassword || !webAccessPasswordConfirm"
          @click="setupBrowserAccess"
        >
          {{ webAccessBusy ? "正在保存" : "设置密码并继续" }}
        </button>
      </div>

      <div v-else-if="checksComplete && pairingVerified" class="ob-pairing-ready">
        当前设备已通过 DeviceCredential 验证，正在进入下一步。
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { getWebAccessStatus, setupWebAccess } from "@/runtime/web-access";
import {
  getApiBaseURL,
  getBackendAuthHeaders,
  getCurrentDevicePairingStatus,
  isCurrentDevicePaired,
  provisionCurrentDeviceMesh,
} from "@/runtime/runtime-adapter";

const props = defineProps<{ deployMode: string; serverURL: string }>();
const emit = defineEmits<{ healthCheckDone: [] }>();

type Row = { name: string; state: "" | "running" | "done" | "error"; stateText: string };
const bootRows = ref<Row[]>([]);
const checksComplete = ref(false);
const pairingRequired = ref(false);
const pairingVerified = ref(false);
const firstDeviceSetupRequired = ref(false);
const pairingValue = ref("");
const pairingBusy = ref(false);
const pairingError = ref("");
const webAccessSetupRequired = ref(false);
const webAccessPassword = ref("");
const webAccessPasswordConfirm = ref("");
const webAccessBusy = ref(false);
const webAccessError = ref("");
let currentBaseURL = "";
let abortController: AbortController | null = null;

function resetRows() {
  const names = props.deployMode === "local"
    ? ["检查本地服务", "检查运行时就绪状态", "检查运行时能力"]
    : ["检查 Cloud Core", "读取 Space 信息", "检查设备配对服务", "确认运行时兼容性"];
  bootRows.value = names.map((name) => ({ name, state: "", stateText: "等待" }));
  checksComplete.value = false;
  pairingRequired.value = false;
  pairingVerified.value = false;
  pairingError.value = "";
  pairingValue.value = "";
  webAccessSetupRequired.value = false;
  webAccessPassword.value = "";
  webAccessPasswordConfirm.value = "";
  webAccessBusy.value = false;
  webAccessError.value = "";
}

function delay(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => {
      clearTimeout(timer);
      reject(new DOMException("Aborted", "AbortError"));
    }, { once: true });
  });
}

async function fetchWithTimeout(url: string, signal: AbortSignal, timeoutMs = 8000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  signal.addEventListener("abort", () => controller.abort(), { once: true });
  try {
    return await fetch(url, {
      signal: controller.signal,
      headers: {
        Accept: "application/json",
        "X-Amitia-Client-Type": window.amitiaDesktop ? "desktop" : "web",
      },
      credentials: window.amitiaDesktop ? "same-origin" : "include",
    });
  } finally {
    clearTimeout(timer);
  }
}

async function runCheck(row: Row, url: string, validate?: (data: any) => boolean) {
  row.state = "running";
  row.stateText = "处理中";
  try {
    const res = await fetchWithTimeout(url, abortController!.signal);
    if (!res.ok) throw new Error(String(res.status));
    if (validate) {
      const data = await res.json();
      if (!validate(data?.data ?? data)) throw new Error("incompatible response");
    }
    row.state = "done";
    row.stateText = "已完成";
    return true;
  } catch {
    row.state = "error";
    row.stateText = "失败";
    return false;
  }
}

async function verifyPrivateDeviceAuth(baseURL: string): Promise<boolean> {
  const headers = await getBackendAuthHeaders("business");
  if (!headers.Authorization) return false;
  try {
    const res = await fetch(`${baseURL}/api/space/profile`, {
      headers: { ...headers, Accept: "application/json" },
      credentials: window.amitiaDesktop ? "same-origin" : "include",
    });
    return res.ok;
  } catch {
    return false;
  }
}

function normalizeOrigin(raw: string): string {
  return new URL(raw).origin;
}

function parseOffer(raw: string, expectedBaseURL: string): string {
  const value = raw.trim();
  if (!value.startsWith("amitia://")) return value;
  const uri = new URL(value);
  const endpoint = uri.searchParams.get("endpoint")?.trim() || "";
  if (endpoint && normalizeOrigin(endpoint) !== normalizeOrigin(expectedBaseURL)) {
    throw new Error("配对 Offer 属于另一个 Cloud Core，请检查服务地址");
  }
  const offer = uri.searchParams.get("offer")?.trim() || "";
  if (!offer) throw new Error("配对 URI 中没有 Offer Token");
  return offer;
}

function needsBrowserAccessPassword(): boolean {
  return props.deployMode !== "local" && !window.amitiaDesktop;
}

async function finishPairedDevice(baseURL: string): Promise<void> {
  pairingVerified.value = true;
  pairingRequired.value = false;

  if (needsBrowserAccessPassword()) {
    const access = await getWebAccessStatus(baseURL);
    if (!access.configured) {
      webAccessSetupRequired.value = true;
      return;
    }
    if (!access.authenticated) {
      webAccessError.value = "请先通过 Web 访问密码登录";
      return;
    }
  }

  if (!(await verifyPrivateDeviceAuth(baseURL))) {
    throw new Error("当前设备凭证验证失败");
  }
  emit("healthCheckDone");
}

async function finishIfAlreadyPaired(baseURL: string): Promise<boolean> {
  const paired = await isCurrentDevicePaired(baseURL);
  if (!paired) return false;
  await finishPairedDevice(baseURL);
  return true;
}

async function preparePairing(baseURL: string) {
  const status = await getCurrentDevicePairingStatus(baseURL);
  firstDeviceSetupRequired.value = status.firstDeviceSetupRequired === true;
  if (await finishIfAlreadyPaired(baseURL)) return;
  pairingRequired.value = true;
}

async function pairCurrentDevice() {
  if (pairingBusy.value || !pairingValue.value.trim() || !currentBaseURL) return;
  pairingBusy.value = true;
  pairingError.value = "";
  try {
    const pairing = firstDeviceSetupRequired.value
      ? { setupCode: pairingValue.value.trim() }
      : { offerToken: parseOffer(pairingValue.value, currentBaseURL) };
    await provisionCurrentDeviceMesh(currentBaseURL, pairing);
    await finishPairedDevice(currentBaseURL);
  } catch (error: any) {
    pairingError.value = error?.message || "设备配对失败";
  } finally {
    pairingBusy.value = false;
  }
}

async function setupBrowserAccess() {
  if (webAccessBusy.value || !currentBaseURL) return;
  webAccessError.value = "";
  if (webAccessPassword.value.length < 8) {
    webAccessError.value = "访问密码至少需要 8 个字符";
    return;
  }
  if (webAccessPassword.value.length > 128) {
    webAccessError.value = "访问密码不能超过 128 个字符";
    return;
  }
  if (webAccessPassword.value !== webAccessPasswordConfirm.value) {
    webAccessError.value = "两次输入的密码不一致";
    return;
  }

  webAccessBusy.value = true;
  try {
    await setupWebAccess(currentBaseURL, webAccessPassword.value);
    webAccessSetupRequired.value = false;
    webAccessPassword.value = "";
    webAccessPasswordConfirm.value = "";
    if (!(await verifyPrivateDeviceAuth(currentBaseURL))) {
      throw new Error("Web 密码已设置，但设备认证验证失败");
    }
    emit("healthCheckDone");
  } catch (error: any) {
    webAccessError.value = error?.message || "Web 访问密码设置失败";
  } finally {
    webAccessBusy.value = false;
  }
}

async function startChecks() {
  abortController?.abort();
  abortController = new AbortController();
  resetRows();
  const base = props.deployMode === "local"
    ? await getApiBaseURL()
    : props.serverURL.trim().replace(/\/+$/, "");
  if (!base) return;
  currentBaseURL = base;

  const checks = props.deployMode === "local"
    ? [
        ["/api/public/health", null],
        ["/readyz", null],
        ["/api/public/runtime/capabilities", null],
      ] as const
    : [
        ["/api/public/health", null],
        ["/api/public/core/info", (data: any) => Boolean(data?.spaceId && data?.instanceId)],
        ["/api/public/device-mesh/v1/pairing/status", (data: any) => typeof data?.firstDeviceSetupRequired === "boolean"],
        ["/api/public/runtime/capabilities", null],
      ] as const;

  for (let i = 0; i < checks.length; i++) {
    if (abortController.signal.aborted) return;
    if (i) await delay(220, abortController.signal);
    const [path, validate] = checks[i];
    const ok = await runCheck(bootRows.value[i], `${base}${path}`, validate || undefined);
    if (!ok) return;
  }
  if (abortController.signal.aborted) return;
  checksComplete.value = true;

  // Electron local mode authenticates with the local Desktop Session. Every
  // Cloud connection, and every browser UI, is an explicit Device Mesh member.
  if (props.deployMode === "local" && window.amitiaDesktop) {
    emit("healthCheckDone");
    return;
  }

  try {
    await preparePairing(base);
  } catch (error: any) {
    pairingRequired.value = true;
    pairingError.value = error?.message || "无法读取设备配对状态";
  }
}

watch(() => [props.deployMode, props.serverURL], () => void startChecks());
onMounted(() => void startChecks());
onUnmounted(() => abortController?.abort());
</script>

<style scoped>
.ob-pairing-panel {
  margin-top: 20px;
  display: grid;
  gap: 10px;
}
.pairing-copy {
  margin: 0;
}
.ob-pairing-error {
  font-size: 12px;
  color: #ff8f8f;
}
.ob-pairing-action {
  justify-self: start;
  margin-top: 4px;
}
.ob-pairing-ready {
  margin-top: 18px;
  font-size: 13px;
  opacity: 0.72;
}
</style>
