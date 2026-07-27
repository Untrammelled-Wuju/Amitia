<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";

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
const sessionOrigin = ref<string>("");
const sessionCSP = ref<string>("");
const resourceUrl = ref<string>("");
const loading = ref(true);
const error = ref<string | null>(null);
const iframeLoaded = ref(false);
const ready = ref(false);

async function createSession() {
  loading.value = true;
  error.value = null;
  ready.value = false;
  iframeLoaded.value = false;
  try {
    const response = await fetch(`/api/extension/webui/session`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        contributionId: props.contribution.contributionId,
        extensionId: props.contribution.extensionId,
        moduleId: props.contribution.moduleId,
        slotId: props.slotId,
        generation: props.contribution.generation,
        sandbox: props.contribution.sandbox ?? "web_restricted",
        entryPath: props.contribution.entryPath ?? "index.html",
        allowedActions: (props.contribution.actions ?? []).map((a) => a.actionId),
      }),
    });
    if (!response.ok) throw new Error(`session create failed: ${response.status}`);
    const data = await response.json();
    sessionId.value = data.sessionId;
    sessionNonce.value = data.nonce;
    sessionOrigin.value = data.origin;
    sessionCSP.value = data.csp;
    resourceUrl.value = data.resourceUrl || data.entryUrl;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
    emit("error", error.value);
  } finally {
    loading.value = false;
  }
}

async function destroySession() {
  if (!sessionId.value) return;
  try {
    await fetch(`/api/extension/webui/session/${sessionId.value}`, {
      method: "DELETE",
    });
  } catch {
  }
  sessionId.value = "";
  sessionNonce.value = "";
  ready.value = false;
}

function onMessage(event: MessageEvent) {
  const expectedOrigin = window.location.origin;
  if (event.origin !== expectedOrigin) return;
  const data = event.data;
  if (!data || typeof data !== "object") return;
  if (data.session !== sessionId.value) return;
  if (data.nonce !== sessionNonce.value) return;
  void handleBridgeMessage(data);
}

async function handleBridgeMessage(msg: Record<string, unknown>) {
  const method = msg.method as string;
  if (method === "ui.ready") {
    ready.value = true;
    emit("ready", sessionId.value);
    sendBridgeResponse(msg, { ok: true, sessionId: sessionId.value });
    return;
  }
  if (method === "ui.action.invoke") {
    const input = msg.input as Record<string, unknown> | null;
    const actionId = input?.actionId as string;
    emit("action", actionId, input?.input);
  }
  try {
    const response = await fetch(`/api/extension/webui/bridge/${sessionId.value}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(msg),
    });
    const result = await response.json();
    sendBridgeResponse(msg, result);
  } catch (e) {
    sendBridgeResponse(msg, {
      ok: false,
      error: e instanceof Error ? e.message : String(e),
    });
  }
}

function sendBridgeResponse(originalMsg: Record<string, unknown>, response: Record<string, unknown>) {
  if (!iframeRef.value?.contentWindow) return;
  iframeRef.value.contentWindow.postMessage(
    {
      type: "bridge.response",
      method: originalMsg.method,
      id: originalMsg.id,
      ...response,
    },
    window.location.origin,
  );
}

function sendHostWelcome() {
  if (!iframeRef.value?.contentWindow) return;
  iframeRef.value.contentWindow.postMessage(
    {
      type: "host.welcome",
      hostOrigin: window.location.origin,
      sessionId: sessionId.value,
    },
    window.location.origin,
  );
}

function onIframeLoad() {
  iframeLoaded.value = true;
  nextTick(() => {
    sendHostWelcome();
    setTimeout(() => {
      if (!ready.value) {
        sendHostWelcome();
      }
    }, 500);
  });
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
</script>

<template>
  <div class="sandbox-webui-frame" :data-contribution-id="contribution.contributionId">
    <template v-if="loading">
      <div class="sandbox-webui-frame__loading">
        <div class="sandbox-webui-frame__spinner" />
        <span>加载扩展界面...</span>
      </div>
    </template>
    <template v-else-if="error">
      <div class="sandbox-webui-frame__error">
        <div class="sandbox-webui-frame__error-icon">!</div>
        <div class="sandbox-webui-frame__error-text">
          <div class="sandbox-webui-frame__error-title">加载失败</div>
          <div class="sandbox-webui-frame__error-detail">{{ error }}</div>
        </div>
        <button class="sandbox-webui-frame__retry" @click="createSession">重试</button>
      </div>
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
}
.sandbox-webui-frame__iframe {
  width: 100%;
  min-height: 240px;
  border: none;
  border-radius: 6px;
  background: transparent;
}
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
