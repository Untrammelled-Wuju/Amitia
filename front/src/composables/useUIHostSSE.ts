import { ref, onUnmounted, type Ref } from "vue";
import { useRouter } from "vue-router";
import { ElNotification, ElMessageBox } from "element-plus";
import { resolveApiUrl, createAuthorizedRequestInit } from "../runtime/runtime-adapter";
import { isNavigationAllowed } from "../navigation/nav-whitelist";
import { useExtensionUIStore } from "../stores/extensionUI";

interface SSEEventEnvelope {
  eventType: string;
  requestId: string;
  sessionId: string;
  extensionId: string;
  payload: Record<string, unknown>;
  expiresAt?: string;
  timestamp: string;
}

const severityMap: Record<string, "success" | "warning" | "info" | "error"> = {
  info: "info",
  success: "success",
  warning: "warning",
  error: "error",
  critical: "error",
};

const MAX_RETRIES = 10;
const BASE_RECONNECT_MS = 2000;
const MAX_RECONNECT_MS = 30000;
const DEDUP_MAX_SIZE = 200;

export function useUIHostSSE(connected?: Ref<boolean>) {
  const router = useRouter();
  let eventSource: EventSource | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let reconnectAttempts = 0;
  let dedupCleanupTimer: ReturnType<typeof setInterval> | null = null;
  const processedRequestIds = new Set<string>();
  const isConnected = connected ?? ref(false);
  let extensionRefreshTimer: ReturnType<typeof setTimeout> | null = null;

  function isExpired(expiresAt?: string): boolean {
    if (!expiresAt) return false;
    try {
      return new Date(expiresAt).getTime() < Date.now();
    } catch {
      return false;
    }
  }

  function trackRequestId(requestId: string): boolean {
    if (processedRequestIds.has(requestId)) return false;
    processedRequestIds.add(requestId);
    if (processedRequestIds.size > DEDUP_MAX_SIZE) {
      const first = processedRequestIds.values().next().value;
      if (first) processedRequestIds.delete(first);
    }
    return true;
  }

  async function sendDialogResponse(dialogId: string, result: string): Promise<void> {
    try {
      const url = await resolveApiUrl("/api/extensions/ui/dialog-response");
      const init = await createAuthorizedRequestInit({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dialogId, result }),
      });
      await fetch(url, init);
    } catch {
      // ignore
    }
  }

  function parseEnvelope(event: MessageEvent): SSEEventEnvelope | null {
    try {
      return JSON.parse(event.data) as SSEEventEnvelope;
    } catch {
      return null;
    }
  }

  function shouldProcess(envelope: SSEEventEnvelope): boolean {
    if (!envelope.requestId) return false;
    if (isExpired(envelope.expiresAt)) return false;
    return trackRequestId(envelope.requestId);
  }

  function handleNotify(event: MessageEvent) {
    const envelope = parseEnvelope(event);
    if (!envelope || !envelope.payload) return;
    if (!shouldProcess(envelope)) return;
    const payload = envelope.payload;
    const type = severityMap[payload.severity as string] || "info";
    ElNotification({
      title: (payload.title as string) || "通知",
      message: (payload.body as string) || "",
      type,
      duration: 5000,
    });
  }

  function handleNavigate(event: MessageEvent) {
    const envelope = parseEnvelope(event);
    if (!envelope || !envelope.payload) return;
    if (!shouldProcess(envelope)) return;
    const target = envelope.payload.target as string;
    if (target && isNavigationAllowed(target)) {
      router.push(target).catch(() => {
        // ignore navigation errors
      });
    }
  }

  function handleDialog(event: MessageEvent) {
    const envelope = parseEnvelope(event);
    if (!envelope || !envelope.payload) return;
    if (!shouldProcess(envelope)) return;
    const payload = envelope.payload;
    const dialogId = payload.dialogId as string;
    const buttons = payload.buttons && (payload.buttons as string[]).length > 0
      ? (payload.buttons as string[])
      : ["确定"];

    ElMessageBox.confirm((payload.message as string) || "", "对话框", {
      confirmButtonText: buttons[0],
      cancelButtonText: buttons.length > 1 ? buttons[1] : "取消",
      distinguishCancelAndClose: true,
      type: "info",
    })
      .then(() => {
        void sendDialogResponse(dialogId, buttons[0]);
      })
      .catch((action: string) => {
        if (action === "cancel" && buttons.length > 1) {
          void sendDialogResponse(dialogId, buttons[1]);
        } else {
          void sendDialogResponse(dialogId, "closed");
        }
      });
  }

  const EXTENSION_CHANGE_EVENTS = new Set([
    "extension_installed",
    "extension_uninstalled",
    "extension_enabled",
    "extension_disabled",
    "extension_paused",
    "extension_resumed",
    "extension_updated",
    "extension_rolled_back",
    "extension_generation_changed",
    "extension_contributions_changed",
  ]);

  function handleExtensionChange(event: MessageEvent) {
    const envelope = parseEnvelope(event);
    if (!envelope) return;
    if (!EXTENSION_CHANGE_EVENTS.has(envelope.eventType)) return;
    if (extensionRefreshTimer) {
      clearTimeout(extensionRefreshTimer);
    }
    extensionRefreshTimer = setTimeout(() => {
      extensionRefreshTimer = null;
      const store = useExtensionUIStore();
      store.refreshSnapshot(true).catch(() => {});
    }, 300);
  }

  function getReconnectDelay(): number {
    const delay = Math.min(
      BASE_RECONNECT_MS * Math.pow(2, reconnectAttempts),
      MAX_RECONNECT_MS,
    );
    return delay + Math.random() * 500;
  }

  function scheduleReconnect() {
    if (reconnectAttempts >= MAX_RETRIES) {
      reconnectAttempts = 0;
      reconnectTimer = setTimeout(() => {
        void connect();
      }, MAX_RECONNECT_MS);
      return;
    }
    const delay = getReconnectDelay();
    reconnectAttempts++;
    reconnectTimer = setTimeout(() => {
      void connect();
    }, delay);
  }

  async function connect() {
    disconnect();
    try {
      const url = await resolveApiUrl(`/api/proactive-sse?clientId=ui-host`);
      eventSource = new EventSource(url);
      eventSource.addEventListener("ui_notify", handleNotify);
      eventSource.addEventListener("ui_navigate", handleNavigate);
      eventSource.addEventListener("ui_dialog", handleDialog);
      eventSource.addEventListener("extension_installed", handleExtensionChange);
      eventSource.addEventListener("extension_uninstalled", handleExtensionChange);
      eventSource.addEventListener("extension_enabled", handleExtensionChange);
      eventSource.addEventListener("extension_disabled", handleExtensionChange);
      eventSource.addEventListener("extension_paused", handleExtensionChange);
      eventSource.addEventListener("extension_resumed", handleExtensionChange);
      eventSource.addEventListener("extension_updated", handleExtensionChange);
      eventSource.addEventListener("extension_rolled_back", handleExtensionChange);
      eventSource.addEventListener("extension_generation_changed", handleExtensionChange);
      eventSource.addEventListener("extension_contributions_changed", handleExtensionChange);
      eventSource.onopen = () => {
        isConnected.value = true;
        reconnectAttempts = 0;
        const store = useExtensionUIStore();
        store.refreshSnapshot(true).catch(() => {});
      };
      eventSource.onerror = () => {
        isConnected.value = false;
        if (eventSource && eventSource.readyState === EventSource.CLOSED) {
          disconnect();
          scheduleReconnect();
        } else {
          disconnect();
          scheduleReconnect();
        }
      };
    } catch {
      scheduleReconnect();
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (extensionRefreshTimer) {
      clearTimeout(extensionRefreshTimer);
      extensionRefreshTimer = null;
    }
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    isConnected.value = false;
  }

  function resetDedup() {
    processedRequestIds.clear();
  }

  dedupCleanupTimer = setInterval(() => {
    if (processedRequestIds.size > DEDUP_MAX_SIZE / 2) {
      resetDedup();
    }
  }, 60000);

  onUnmounted(() => {
    disconnect();
    if (dedupCleanupTimer) {
      clearInterval(dedupCleanupTimer);
      dedupCleanupTimer = null;
    }
  });

  return {
    isConnected,
    connect,
    disconnect,
  };
}
