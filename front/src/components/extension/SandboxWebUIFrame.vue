<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, computed } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import { apiClient } from "@/composables/useApi";
import ExtensionRenderState from "./ExtensionRenderState.vue";

const props = defineProps<{
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
}>();

const emit = defineEmits<{
  (e: "ready", session: string): void;
  (e: "error", error: string): void;
  (e: "action", action: string, input: unknown): void;
}>();

const iframeRef = ref<HTMLIFrameElement | null>(null);
const sessionId = ref<string>("");
const sessionNonce = ref<string>("");
const sessionToken = ref<string>("");
const sessionOrigin = ref<string>("");
const sessionCSP = ref<string>("");
const resourceUrl = ref<string>("");
const loading = ref(true);
const error = ref<string | null>(null);
const iframeLoaded = ref(false);
const ready = ref(false);
const preferredHeight = ref<number | null>(null);
let bridgePort: MessagePort | null = null;

const PROTOCOL_VERSION = "amitia-webui-bridge-v1";

const uiContext = computed(() => props.context ?? {});
const surfaceRole = computed(() => String((uiContext.value.surface as Record<string, unknown> | undefined)?.role ?? "main"));
const iframeStyle = computed(() => {
  const height = preferredHeight.value;
  if (!height || ["sidebar", "main"].includes(surfaceRole.value)) return undefined;
  return { height: `${height}px` };
});

async function createSession() {
  loading.value = true;
  error.value = null;
  ready.value = false;
  iframeLoaded.value = false;
  try {
    const surfaceData = (uiContext.value.surface as Record<string, unknown> | undefined) ?? {};
    const surfaceRole = String(surfaceData.role ?? "main");
    const themeData = (uiContext.value.theme as Record<string, unknown> | undefined) ?? {};
    const res = await apiClient.post<{
      sessionId: string;
      nonce: string;
      token: string;
      origin: string;
      csp: string;
      resourceUrl?: string;
      entryUrl?: string;
    }>("/api/extension/webui/session", {
      contributionId: props.contribution.contributionId,
      extensionId: props.contribution.extensionId,
      moduleId: props.contribution.moduleId,
      slotId: props.slotId,
      generation: props.contribution.generation,
      surface: surfaceRole,
      characterId: (uiContext.value.characterId as string) || "",
      conversationId: (uiContext.value.conversationId as string) || "",
      theme: {
        mode: themeData.mode || (uiContext.value.hostTheme as string) || "light",
        density: themeData.density || "default",
        tokens: themeData.tokens as Record<string, string> || buildThemeTokens(),
      },
      locale: (uiContext.value.locale as string) || navigator.language || "en",
      uiContext: uiContext.value,
      sandbox: props.contribution.sandbox ?? "web_restricted",
      entryPath: props.contribution.entryPath ?? "index.html",
      allowedActions: (props.contribution.actions ?? []).map((a) => a.actionId),
    });
    const data = res.data;
    sessionId.value = data.sessionId;
    sessionNonce.value = data.nonce;
    sessionToken.value = data.token;
    sessionOrigin.value = data.origin;
    sessionCSP.value = data.csp;
    resourceUrl.value = data.resourceUrl || data.entryUrl || "";
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
    emit("error", error.value);
  } finally {
    loading.value = false;
  }
}

async function destroySession() {
	bridgePort?.close();
	bridgePort = null;
  if (!sessionId.value) return;
  try {
    await apiClient.delete(`/api/extension/webui/session/${sessionId.value}`);
  } catch {
  }
  sessionId.value = "";
  sessionNonce.value = "";
  sessionToken.value = "";
  ready.value = false;
}

function onMessage(event: MessageEvent) {
  if (event.source !== iframeRef.value?.contentWindow) return;
  const data = event.data;
  if (!data || typeof data !== "object") return;
  if (data.type !== "amitia.extension.ready") return;
  if (data.protocolVersion !== PROTOCOL_VERSION) return;
  if (data.session !== sessionId.value) return;
  if (data.nonce !== sessionNonce.value) return;
  if (data.generation !== props.contribution.generation) return;
  if (bridgePort) return;
  const channel = new MessageChannel();
  bridgePort = channel.port1;
  bridgePort.onmessage = (portEvent) => {
    const message = portEvent.data;
    if (!message || typeof message !== "object") return;
    void handleBridgeMessage(message as Record<string, unknown>);
  };
  bridgePort.start();
  iframeRef.value?.contentWindow?.postMessage(
    {
      type: "amitia.extension.init",
      session: sessionId.value,
      nonce: sessionNonce.value,
      token: sessionToken.value,
      generation: props.contribution.generation,
      uiContext: uiContext.value,
      theme: buildThemeTokens(),
    },
    sessionOrigin.value || "*",
    [channel.port2],
  );
}

async function handleBridgeMessage(msg: Record<string, unknown>) {
  const method = msg.method as string;
  if (method === "ui.ready") {
    ready.value = true;
    emit("ready", sessionId.value);
    sendBridgeResponse(msg, { ok: true, sessionId: sessionId.value });
    return;
  }
  if (method === "ui.content.resize" || method === "ui.resize.request") {
    const input = msg.input as Record<string, unknown> | undefined;
    const requested = Number(input?.preferredHeight ?? input?.height);
    if (Number.isFinite(requested) && requested > 0) {
      const maximum = surfaceRole.value === "composer" ? 160 : surfaceRole.value === "message" ? 480 : 720;
      preferredHeight.value = Math.max(44, Math.min(Math.round(requested), maximum));
    }
    sendBridgeResponse(msg, { ok: true });
    return;
  }
  if (msg.type === "host.event") {
    sendBridgeResponse(msg, { ok: true });
    return;
  }
  if (method === "ui.action.invoke") {
    const input = msg.input as Record<string, unknown> | null;
    const actionId = input?.actionId as string;
    emit("action", actionId, input?.input);
  }
  try {
    const res = await apiClient.post(`/api/extension/webui/bridge/${sessionId.value}`, msg);
    sendBridgeResponse(msg, res.data as Record<string, unknown>);
  } catch (e) {
    sendBridgeResponse(msg, {
      ok: false,
      error: e instanceof Error ? e.message : String(e),
    });
  }
}

function sendBridgeResponse(originalMsg: Record<string, unknown>, response: Record<string, unknown>) {
  bridgePort?.postMessage({
    type: "bridge.response",
    method: originalMsg.method,
    id: originalMsg.id,
    ...response,
  });
}

function onIframeLoad() {
  iframeLoaded.value = true;
}

function postUIContext() {
  if (!bridgePort || !ready.value) return;
  const surface = (uiContext.value.surface as Record<string, unknown> | undefined) ?? {};
  const themeData = buildThemeTokens();
  bridgePort.postMessage({ type: "host.event", method: "ui.host.context", payload: uiContext.value });
  bridgePort.postMessage({ type: "host.event", method: "ui.host.theme", payload: themeData });
  bridgePort.postMessage({ type: "host.event", method: "ui.host.resize", payload: { width: surface.width ?? 0, height: surface.height ?? 0, breakpoint: surface.breakpoint ?? "xs", surfaceRole: surface.role ?? "main" } });
}

function buildThemeTokens() {
  const themeData = (uiContext.value.theme as Record<string, unknown> | undefined) ?? {};
  const mode = themeData.mode || (uiContext.value.hostTheme as string) || "light";
  const cs = getComputedStyle(document.documentElement);
  const surface = cs.getPropertyValue("--amitia-bg-surface").trim() || cs.getPropertyValue("--surface-bg").trim() || "transparent";
  const text = cs.getPropertyValue("--amitia-text-primary").trim() || cs.getPropertyValue("--text-primary").trim() || "inherit";
  const textSecondary = cs.getPropertyValue("--amitia-text-secondary").trim() || cs.getPropertyValue("--text-secondary").trim() || "inherit";
  const border = cs.getPropertyValue("--amitia-border").trim() || cs.getPropertyValue("--surface-border").trim() || "transparent";
  const control = cs.getPropertyValue("--amitia-control-hover").trim() || cs.getPropertyValue("--control-hover-bg").trim() || "transparent";
  const radius = cs.getPropertyValue("--amitia-radius-sm").trim() || cs.getPropertyValue("--radius-sm").trim() || "8px";
  const font = cs.getPropertyValue("--amitia-font-ui").trim() || cs.getPropertyValue("--ac-font-family").trim() || "system-ui";
  const fontSize = cs.getPropertyValue("--amitia-font-size-sm").trim() || cs.getPropertyValue("--ac-font-size-sm").trim() || "13px";
  const accent = cs.getPropertyValue("--ac-color-primary").trim() || cs.getPropertyValue("--tp-primary").trim() || "#c99557";
  const success = cs.getPropertyValue("--ac-color-success").trim() || cs.getPropertyValue("--tp-success").trim() || "#75a184";
  const warning = cs.getPropertyValue("--ac-color-warning").trim() || cs.getPropertyValue("--tp-warning").trim() || "#c99a56";
  const danger = cs.getPropertyValue("--ac-color-danger").trim() || cs.getPropertyValue("--tp-danger").trim() || "#c96e6a";
  return {
    mode,
    surface,
    text,
    textSecondary,
    border,
    control,
    radius,
    font,
    fontSize,
    accent,
    success,
    warning,
    danger,
  };
}

onMounted(async () => {
  window.addEventListener("message", onMessage);
  await createSession();
});

onBeforeUnmount(() => {
  window.removeEventListener("message", onMessage);
  void destroySession();
});

watch(() => props.contribution.contributionId, async () => {
  await destroySession();
  await createSession();
});

watch(() => props.contribution.generation, async () => {
  await destroySession();
  await createSession();
});

watch(uiContext, postUIContext, { deep: true });
watch(ready, (value) => { if (value) postUIContext(); });
</script>

<template>
  <div class="sandbox-webui-frame" :data-contribution-id="contribution.contributionId">
    <template v-if="loading">
      <ExtensionRenderState state="loading" />
    </template>
    <template v-else-if="error">
      <ExtensionRenderState state="error" :detail="error" @retry="createSession" />
    </template>
    <template v-else>
      <div class="sandbox-webui-frame__container">
        <div v-if="!ready && iframeLoaded" class="sandbox-webui-frame__connecting">
          正在建立连接...
        </div>
        <iframe
          ref="iframeRef"
          :src="resourceUrl"
          class="sandbox-webui-frame__iframe"
          :class="`sandbox-webui-frame__iframe--${surfaceRole}`"
          :style="iframeStyle"
          sandbox="allow-scripts"
          referrerpolicy="no-referrer"
          :data-session-id="sessionId"
          @load="onIframeLoad"
        ></iframe>
      </div>
    </template>
  </div>
</template>

<style scoped>
.sandbox-webui-frame {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.sandbox-webui-frame__container {
  position: relative;
  width: 100%;
  min-height: 0;
}
.sandbox-webui-frame__iframe {
  width: 100%;
  min-height: 0;
  height: 100%;
  border: none;
  border-radius: 6px;
  background: transparent;
}
.sandbox-webui-frame__iframe--composer { min-height: 44px; max-height: 160px; }
.sandbox-webui-frame__iframe--message { min-height: 44px; max-height: 480px; }
.sandbox-webui-frame__iframe--sidebar, .sandbox-webui-frame__iframe--main { display: block; min-height: 0; }
.sandbox-webui-frame__loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px;
  font-size: 13px;
  color: rgb(128, 128, 128);
}
.sandbox-webui-frame__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgb(200, 200, 200);
  border-top-color: rgb(74, 108, 247);
  border-radius: 50%;
  animation: sandbox-spin 0.8s linear infinite;
}
@keyframes sandbox-spin {
  to { transform: rotate(360deg); }
}
.sandbox-webui-frame__connecting {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 10px;
  font-size: 11px;
  color: rgb(128, 128, 128);
  background: rgba(255, 255, 255, 0.8);
  border-radius: 4px;
  pointer-events: none;
}
.sandbox-webui-frame__error {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  background: rgb(254, 242, 242);
  border: 1px solid rgb(252, 165, 165);
  border-radius: 6px;
}
.sandbox-webui-frame__error-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: rgb(220, 38, 38);
  color: white;
  font-size: 14px;
  font-weight: bold;
  flex-shrink: 0;
}
.sandbox-webui-frame__error-text {
  flex: 1;
  min-width: 0;
}
.sandbox-webui-frame__error-title {
  font-size: 13px;
  font-weight: 600;
  color: rgb(154, 26, 26);
}
.sandbox-webui-frame__error-detail {
  font-size: 12px;
  color: rgb(180, 40, 40);
  word-break: break-word;
}
.sandbox-webui-frame__retry {
  padding: 4px 12px;
  font-size: 12px;
  color: rgb(74, 108, 247);
  background: white;
  border: 1px solid rgb(200, 200, 200);
  border-radius: 4px;
  cursor: pointer;
  flex-shrink: 0;
}
.sandbox-webui-frame__retry:hover {
  background: rgb(248, 248, 248);
}
</style>
