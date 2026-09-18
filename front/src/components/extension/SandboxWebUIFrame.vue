<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, computed } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import { apiClient } from "@/composables/useApi";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import ExtensionRenderState from "./ExtensionRenderState.vue";
import {
  buildSandboxThemeSnapshot,
  buildSandboxThemeTokens,
  claimSandboxSession,
  getOrCreateSandboxSession,
  isSandboxSessionMissingError,
  putCachedSandboxSession,
  releaseSandboxSessionClaim,
  sandboxSessionKey,
  takeCachedSandboxSession,
  type SandboxSessionRecord,
} from "./sandboxSessionCache";

const props = defineProps<{
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
  surfaceState?: Record<string, unknown>;
  hostActions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
}>();

const emit = defineEmits<{
  (e: "ready", session: string): void;
  (e: "error", error: string): void;
  (e: "action", action: string, input: unknown): void;
}>();

const iframeRef = ref<HTMLIFrameElement | null>(null);
const sessionId = ref<string>("");
const sessionGeneration = ref<number>(0);
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
const preferredWidth = ref<number | null>(null);
const dismissCounter = ref(0);
let bridgePort: MessagePort | null = null;
let composerResizeObserver: ResizeObserver | null = null;

const PROTOCOL_VERSION = "amitia-webui-bridge-v1";

const hostContext = computed<Record<string, unknown>>(() => props.context ?? {});
const uiContext = computed<Record<string, unknown>>(() => ({
  ...hostContext.value,
  surfaceState: props.surfaceState ?? {},
}));
const surfaceRole = computed(() => String((uiContext.value.surface as Record<string, unknown> | undefined)?.role ?? "main"));
const overlayMode = computed(() => surfaceRole.value === "composer" || surfaceRole.value === "overlay");
const composerExpanded = computed(() => overlayMode.value && ((preferredWidth.value ?? 32) > 32 || (preferredHeight.value ?? 32) > 32));
const surfaceStateWithDismiss = computed<Record<string, unknown>>(() => ({
  ...(props.surfaceState ?? {}),
  dismissToken: dismissCounter.value,
}));

function onDocumentPointerDown(event: PointerEvent) {
  if (!composerExpanded.value) return;
  const iframe = iframeRef.value;
  if (!iframe) return;
  const target = event.target as Node | null;
  if (!target) return;
  if (target === iframe || iframe.contains(target)) return;
  dismissCounter.value += 1;
}
const iframeStyle = computed(() => {
  if (overlayMode.value) {
    return {
      width: `${preferredWidth.value || 32}px`,
      height: `${preferredHeight.value || 32}px`,
    };
  }
  const height = preferredHeight.value;
  if (!height || ["sidebar", "main"].includes(surfaceRole.value)) return undefined;
  return { height: `${height}px` };
});

const sessionCacheKey = computed(() =>
  sandboxSessionKey({
    contribution: props.contribution,
    context: uiContext.value,
    slotId: props.slotId,
  })
);

let serverCapabilities: string[] = [];
let serverGrantedPerms: string[] = [];
let serverGrantedScopes: string[] = [];
let activeSessionKey = "";

let restartToken = 0;
let restartPromise: Promise<void> | null = null;

function applySession(data: SandboxSessionRecord, cacheKey: string) {
  sessionId.value = data.sessionId;
  sessionGeneration.value = data.generation;
  sessionNonce.value = data.nonce;
  sessionToken.value = data.token;
  sessionOrigin.value = data.origin;
  sessionCSP.value = data.csp;
  resourceUrl.value = data.resourceUrl;
  serverCapabilities = data.capabilities;
  serverGrantedPerms = data.grantedPerms;
  serverGrantedScopes = data.grantedScopes;
  activeSessionKey = cacheKey;
}

async function createSession(expectedToken: number) {
  if (expectedToken !== restartToken) return;
  loading.value = true;
  error.value = null;
  ready.value = false;
  iframeLoaded.value = false;
  serverCapabilities = [];
  serverGrantedPerms = [];
  serverGrantedScopes = [];
  try {
    const cacheKey = sessionCacheKey.value;
    claimSandboxSession(cacheKey);
    const cached = takeCachedSandboxSession(cacheKey);
    if (cached) {
      try {
        const info = await apiClient.get<{ generation?: number }>(`/api/extension/webui/session/${cached.sessionId}`);
        if (expectedToken !== restartToken) {
          releaseSandboxSessionClaim(cacheKey);
          return;
        }
        applySession({
          ...cached,
          generation: typeof info.data?.generation === "number" ? info.data.generation : cached.generation,
        }, cacheKey);
        loading.value = false;
        return;
      } catch (e) {
        if (!isSandboxSessionMissingError(e)) {
          applySession(cached, cacheKey);
          loading.value = false;
          return;
        }
      }
    }
    const data = await getOrCreateSandboxSession({
      contribution: props.contribution,
      context: uiContext.value,
      slotId: props.slotId,
    });
    if (expectedToken !== restartToken) {
      const staleSid = data?.sessionId ?? "";
      if (staleSid) {
        apiClient.delete(`/api/extension/webui/session/${staleSid}`).catch(() => {});
      }
      releaseSandboxSessionClaim(cacheKey);
      return;
    }
    applySession(data, cacheKey);
  } catch (e) {
    if (expectedToken !== restartToken) {
      releaseSandboxSessionClaim(sessionCacheKey.value);
      return;
    }
    releaseSandboxSessionClaim(sessionCacheKey.value);
    error.value = e instanceof Error ? e.message : String(e);
    emit("error", error.value);
  } finally {
    if (expectedToken === restartToken) {
      loading.value = false;
    }
  }
}

async function destroySession() {
  bridgePort?.close();
  bridgePort = null;
  const key = activeSessionKey || sessionCacheKey.value;
  if (!sessionId.value) {
    releaseSandboxSessionClaim(key);
    return;
  }
  try {
    await apiClient.delete(`/api/extension/webui/session/${sessionId.value}`);
  } catch {
  }
  releaseSandboxSessionClaim(key);
  sessionId.value = "";
  sessionGeneration.value = 0;
  sessionNonce.value = "";
  sessionToken.value = "";
  serverCapabilities = [];
  serverGrantedPerms = [];
  serverGrantedScopes = [];
  activeSessionKey = "";
  ready.value = false;
}

function stashSession() {
  bridgePort?.close();
  bridgePort = null;
  if (!sessionId.value) return;
  const entry: SandboxSessionRecord = {
    sessionId: sessionId.value,
    generation: sessionGeneration.value,
    nonce: sessionNonce.value,
    token: sessionToken.value,
    origin: sessionOrigin.value,
    csp: sessionCSP.value,
    resourceUrl: resourceUrl.value,
    capabilities: [...serverCapabilities],
    grantedPerms: [...serverGrantedPerms],
    grantedScopes: [...serverGrantedScopes],
  };
  const key = activeSessionKey || sessionCacheKey.value;
  putCachedSandboxSession(key, entry);
  releaseSandboxSessionClaim(key);
  sessionId.value = "";
  sessionGeneration.value = 0;
  sessionNonce.value = "";
  sessionToken.value = "";
  sessionOrigin.value = "";
  sessionCSP.value = "";
  resourceUrl.value = "";
  serverCapabilities = [];
  serverGrantedPerms = [];
  serverGrantedScopes = [];
  activeSessionKey = "";
  ready.value = false;
}

async function restartSession() {
  if (restartPromise) return restartPromise;
  const token = ++restartToken;
  restartPromise = (async () => {
    await destroySession();
    if (token !== restartToken) return;
    await createSession(token);
  })().finally(() => {
    restartPromise = null;
  });
  return restartPromise;
}

async function handleBridgeFailure(msg: Record<string, unknown>, error: unknown, requestSessionId: string) {
  if (requestSessionId !== sessionId.value) return;
  if (isSandboxSessionMissingError(error)) {
    await restartSession();
    return;
  }
  sendBridgeResponse(msg, {
    ok: false,
    error: error instanceof Error ? error.message : String(error),
  });
}

function onMessage(event: MessageEvent) {
  if (event.source !== iframeRef.value?.contentWindow) return;
  const data = event.data;
  if (!data || typeof data !== "object") return;
  if (data.type !== "amitia.extension.ready") return;
  if (data.protocolVersion !== PROTOCOL_VERSION) return;
  if (data.session !== sessionId.value) return;
  if (data.nonce !== sessionNonce.value) return;
  if (data.generation !== sessionGeneration.value) return;
  if (bridgePort) return;
  const channel = new MessageChannel();
  bridgePort = channel.port1;
  bridgePort.onmessage = (portEvent) => {
    const message = portEvent.data;
    if (!message || typeof message !== "object") return;
    void handleBridgeMessage(message as Record<string, unknown>, sessionId.value);
  };
  bridgePort.start();
  const env = resolveHostEnvironment();
  iframeRef.value?.contentWindow?.postMessage(
    {
      type: "amitia.extension.init",
      session: sessionId.value,
      nonce: sessionNonce.value,
      token: sessionToken.value,
      generation: sessionGeneration.value,
      uiContext: {
        theme: buildThemeTokens(),
        locale: (uiContext.value.locale as string) || navigator.language || "en",
        platform: env.platform,
        host: env.host,
        os: env.os,
        surface: (uiContext.value.surface as Record<string, unknown> | undefined)?.role ?? "main",
        slotId: props.slotId,
        surfaceState: surfaceStateWithDismiss.value,
        surfaceMetrics: buildSurfaceMetrics(),
      },
      capabilities: serverCapabilities,
      grantedPerms: serverGrantedPerms,
      grantedScopes: serverGrantedScopes,
      theme: buildThemeTokens(),
    },
    "*",
    [channel.port2],
  );
}

async function handleBridgeMessage(msg: Record<string, unknown>, requestSessionId: string) {
  const method = msg.method as string;
  if (method === "ui_ready" || method === "ui.ready") {
    try {
      const res = await apiClient.post(`/api/extension/webui/bridge/${sessionId.value}`, msg);
      ready.value = true;
      emit("ready", sessionId.value);
      sendBridgeResponse(msg, res.data as Record<string, unknown>);
    } catch (e) {
      await handleBridgeFailure(msg, e, requestSessionId);
    }
    return;
  }
  if (method === "ui.context.get") {
    try {
      const res = await apiClient.post(`/api/extension/webui/bridge/${sessionId.value}`, msg);
      const data = res.data as Record<string, unknown>;
      const output = data.output;
      if (output && typeof output === "object") {
        (output as Record<string, unknown>).surfaceState = surfaceStateWithDismiss.value;
        (output as Record<string, unknown>).surfaceMetrics = buildSurfaceMetrics();
      }
      sendBridgeResponse(msg, data);
    } catch (e) {
      await handleBridgeFailure(msg, e, requestSessionId);
    }
    return;
  }
  if (method === "ui.content.resize" || method === "ui.resize.request") {
    const input = msg.input as Record<string, unknown> | undefined;
    const requested = Number(input?.preferredHeight ?? input?.height);
    const requestedWidth = Number(input?.preferredWidth ?? input?.width);
    if (Number.isFinite(requestedWidth) && requestedWidth > 0) {
      const maximumWidth = surfaceRole.value === "composer" ? 520 : 1200;
      preferredWidth.value = Math.max(32, Math.min(Math.round(requestedWidth), maximumWidth));
    }
    if (Number.isFinite(requested) && requested > 0) {
      const maximum = surfaceRole.value === "composer" ? 480 : surfaceRole.value === "message" ? 480 : 720;
      const minimum = surfaceRole.value === "composer" ? 32 : 44;
      preferredHeight.value = Math.max(minimum, Math.min(Math.round(requested), maximum));
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
    const actionId = String(input?.actionId ?? input?.action_id ?? "");
    const localAction = props.hostActions?.[actionId];
    if (localAction) {
      try {
        const result = await localAction(input?.input);
        sendBridgeResponse(msg, { ok: true, result });
      } catch (e) {
        sendBridgeResponse(msg, { ok: false, error: e instanceof Error ? e.message : String(e) });
      }
      return;
    }
    emit("action", actionId, input?.input);
  }
  try {
    const res = await apiClient.post(`/api/extension/webui/bridge/${sessionId.value}`, msg);
    const data = res.data as Record<string, unknown>;
    sendBridgeResponse(msg, data);
  } catch (e) {
    await handleBridgeFailure(msg, e, requestSessionId);
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
  observeComposerSurface();
}

function observeComposerSurface() {
  if (surfaceRole.value !== "composer" || !iframeRef.value || typeof ResizeObserver === "undefined") return;
  const wrapper = iframeRef.value.closest(".input-wrapper");
  if (!(wrapper instanceof HTMLElement)) return;
  composerResizeObserver?.disconnect();
  composerResizeObserver = new ResizeObserver(() => {
    if (ready.value) postUIContext();
  });
  composerResizeObserver.observe(wrapper);
}

function buildSurfaceMetrics(): Record<string, number> {
  if (surfaceRole.value !== "composer" || !iframeRef.value) return {};
  const wrapper = iframeRef.value.closest(".input-wrapper");
  if (!(wrapper instanceof HTMLElement)) return {};
  const frameRect = iframeRef.value.getBoundingClientRect();
  const wrapperRect = wrapper.getBoundingClientRect();
  return {
    panelBottom: Math.max(40, Math.round(frameRect.bottom - wrapperRect.top + 8)),
  };
}

function postUIContext() {
  if (!bridgePort || !ready.value) return;
  const surface = (uiContext.value.surface as Record<string, unknown> | undefined) ?? {};
  const surfaceRole = String(surface.role ?? "main");
  const themeSnapshot = buildThemeSnapshot();
  const env = resolveHostEnvironment();
  const contextPayload = {
    theme: themeSnapshot,
    locale: (uiContext.value.locale as string) || navigator.language || "en",
    platform: env.platform,
    host: env.host,
    os: env.os,
    surface: surfaceRole,
    slotId: props.slotId,
    messageId: (uiContext.value.messageId as string) || "",
    characterId: (uiContext.value.characterId as string) || "",
    conversationId: (uiContext.value.conversationId as string) || "",
    capabilities: serverCapabilities,
    grantedPerms: serverGrantedPerms,
    grantedScopes: serverGrantedScopes,
    scope: {
      extensionId: props.contribution.extensionId,
      moduleId: props.contribution.moduleId,
    },
    generation: sessionGeneration.value,
    surfaceState: surfaceStateWithDismiss.value,
    surfaceMetrics: buildSurfaceMetrics(),
  };
  bridgePort.postMessage({ type: "host.event", method: "ui.host.context", payload: contextPayload });
  bridgePort.postMessage({ type: "host.event", method: "ui.host.theme", payload: themeSnapshot });
  bridgePort.postMessage({ type: "host.event", method: "ui.host.resize", payload: { width: surface.width ?? 0, height: surface.height ?? 0, breakpoint: surface.breakpoint ?? "xs", surfaceRole } });
}

function buildThemeSnapshot() {
  return buildSandboxThemeSnapshot(uiContext.value);
}

function buildThemeTokens() {
  return buildSandboxThemeTokens();
}

onMounted(async () => {
  window.addEventListener("message", onMessage);
  document.addEventListener("pointerdown", onDocumentPointerDown, true);
  await restartSession();
});

onBeforeUnmount(() => {
  window.removeEventListener("message", onMessage);
  document.removeEventListener("pointerdown", onDocumentPointerDown, true);
  composerResizeObserver?.disconnect();
  composerResizeObserver = null;
  ++restartToken;
  stashSession();
});

watch(sessionCacheKey, async () => {
  await restartSession();
});

watch(() => buildThemeSnapshot(), () => {
  if (!bridgePort || !ready.value) return;
  bridgePort.postMessage({ type: "host.event", method: "ui.host.theme", payload: buildThemeSnapshot() });
});

watch(() => {
  const surface = (uiContext.value.surface as Record<string, unknown> | undefined) ?? {};
  return { width: surface.width ?? 0, height: surface.height ?? 0, breakpoint: surface.breakpoint ?? "xs" };
}, () => {
  if (!bridgePort || !ready.value) return;
  const surface = (uiContext.value.surface as Record<string, unknown> | undefined) ?? {};
  const surfaceRole = String(surface.role ?? "main");
  bridgePort.postMessage({ type: "host.event", method: "ui.host.resize", payload: { width: surface.width ?? 0, height: surface.height ?? 0, breakpoint: surface.breakpoint ?? "xs", surfaceRole } });
}, { deep: true });

watch(ready, (value) => {
  if (!value) return;
  observeComposerSurface();
  postUIContext();
});

watch(() => uiContext.value.locale, () => {
  if (!bridgePort || !ready.value) return;
  postUIContext();
});

watch(() => props.surfaceState, () => {
  if (!bridgePort || !ready.value) return;
  postUIContext();
}, { deep: true });

watch(dismissCounter, () => {
  if (!bridgePort || !ready.value) return;
  postUIContext();
});
</script>

<template>
  <div
    class="sandbox-webui-frame"
    :class="{ 'sandbox-webui-frame--overlay': overlayMode }"
    :data-contribution-id="contribution.contributionId"
  >
    <template v-if="loading">
      <ExtensionRenderState state="loading" />
    </template>
    <template v-else-if="error">
      <ExtensionRenderState state="error" :detail="error" @retry="restartSession" />
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
  height: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.sandbox-webui-frame--overlay {
  position: relative;
  width: 32px;
  height: 32px;
  min-width: 32px;
  flex: 0 0 32px;
  overflow: visible;
}
.sandbox-webui-frame--overlay .sandbox-webui-frame__container {
  position: relative;
  width: 32px;
  height: 32px;
  overflow: visible;
}
.sandbox-webui-frame--overlay .sandbox-webui-frame__iframe {
  position: absolute;
  left: 0;
  bottom: 0;
  z-index: 120;
  min-height: 0;
  max-height: none;
  overflow: visible;
}
.sandbox-webui-frame--overlay .sandbox-webui-frame__connecting,
.sandbox-webui-frame--overlay .sandbox-webui-frame__loading {
  display: none;
}
.sandbox-webui-frame--overlay :deep(.extension-render-state) {
  display: none;
}
.sandbox-webui-frame__container {
  position: relative;
  width: 100%;
  height: 100%;
  flex: 1;
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
.sandbox-webui-frame__iframe--composer { min-height: 44px; max-height: 480px; }
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
