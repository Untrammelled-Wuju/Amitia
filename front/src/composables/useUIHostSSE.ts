import { ref, onUnmounted, type Ref } from "vue";
import { useRouter } from "vue-router";
import { ElNotification, ElMessageBox } from "element-plus";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import { isNavigationAllowed } from "../navigation/nav-whitelist";
import { useExtensionUIStore } from "../stores/extensionUI";
import { browserClientPluginRuntime, type BrowserDeclarativeClientPackage } from "../ui-runtime/clientPluginRuntime";

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
  let abortController: AbortController | null = null;
  let connectionVersion = 0;
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
      const init = await createAuthenticatedFetchInit("/api/extensions/ui/dialog-response", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dialogId, result }),
      });
      await fetch(url, init);
    } catch {
      // ignore
    }
  }

  async function sendClientRuntimeResponse(
    commandId: string,
    hostClientId: string,
    hostSessionId: string,
    result: Record<string, unknown> = {},
    error = "",
  ): Promise<void> {
    try {
      const url = await resolveApiUrl("/api/extensions/ui/client-runtime-response");
      const init = await createAuthenticatedFetchInit("/api/extensions/ui/client-runtime-response", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ commandId, hostClientId, hostSessionId, result, error }),
      });
      await fetch(url, init);
    } catch {
      // The backend command has its own timeout. A disconnected response path must
      // not crash the UI host or leave the runtime half-applied.
    }
  }

  function readRuntimeCommandId(payload: Record<string, unknown>): string {
    return typeof payload.commandId === "string" ? payload.commandId.trim() : "";
  }

  async function executeClientRuntimeCommand(
    action: string,
    payload: Record<string, unknown>,
    conversationId: string,
  ): Promise<Record<string, unknown>> {
    switch (action) {
      case "inspect":
        return browserClientPluginRuntime.inspect() as unknown as Record<string, unknown>;
      case "define": {
        const pkg = payload.package as BrowserDeclarativeClientPackage | undefined;
        if (!pkg || typeof pkg !== "object") throw new Error("define requires a declarative package");
        browserClientPluginRuntime.defineScopedDeclarativePackage(conversationId, pkg);
        return { ok: true, id: pkg.id, version: pkg.version, state: "defined" };
      }
      case "run": {
        const id = String(payload.id ?? "").trim();
        const version = String(payload.version ?? "").trim() || undefined;
        if (!id) throw new Error("run requires id");
        const scopedId = browserClientPluginRuntime.scopedPackageId(conversationId, id);
        await browserClientPluginRuntime.runPackage(scopedId, version);
        return { ok: true, id, version: version ?? null, state: "running" };
      }
      case "stop": {
        const id = String(payload.id ?? "").trim();
        if (!id) throw new Error("stop requires id");
        const scopedId = browserClientPluginRuntime.scopedPackageId(conversationId, id);
        await browserClientPluginRuntime.stopPackage(scopedId);
        return { ok: true, id, state: "stopped" };
      }
      case "rollback": {
        const id = String(payload.id ?? "").trim();
        if (!id) throw new Error("rollback requires id");
        const scopedId = browserClientPluginRuntime.scopedPackageId(conversationId, id);
        const version = await browserClientPluginRuntime.rollbackPackage(scopedId);
        return { ok: true, id, version, state: "running" };
      }
      case "undefine": {
        const id = String(payload.id ?? "").trim();
        if (!id) throw new Error("undefine requires id");
        const scopedId = browserClientPluginRuntime.scopedPackageId(conversationId, id);
        let version = String(payload.version ?? "").trim();
        if (!version) {
          const inspection = browserClientPluginRuntime.inspect();
          const pkg = inspection.packages.find((item) => item.id === scopedId);
          version = pkg?.activeVersion || pkg?.versions.at(-1) || "";
        }
        if (!version) throw new Error(`client package ${id} has no defined version`);
        await browserClientPluginRuntime.undefinePackage(scopedId, version);
        return { ok: true, id, version, state: "undefined" };
      }
      default:
        throw new Error(`unsupported client runtime action: ${action}`);
    }
  }

  async function handleClientRuntimeCommand(event: MessageEvent) {
    const envelope = parseEnvelope(event);
    if (!envelope || !envelope.payload) return;
    if (!shouldProcess(envelope)) return;
    const commandId = readRuntimeCommandId(envelope.payload);
    if (!commandId) return;
    const action = typeof envelope.payload.action === "string" ? envelope.payload.action.trim() : "";
    const payload = envelope.payload.payload && typeof envelope.payload.payload === "object"
      ? envelope.payload.payload as Record<string, unknown>
      : {};
    const conversationId = String(envelope.payload.conversationId ?? "").trim();
    const hostClientId = String(envelope.payload.hostClientId ?? "").trim();
    const hostSessionId = String(envelope.payload.hostSessionId ?? "").trim();
    const expectResponse = envelope.payload.expectResponse !== false;
    const respond = async (result: Record<string, unknown>, error = "") => {
      if (!expectResponse) return;
      await sendClientRuntimeResponse(commandId, hostClientId, hostSessionId, result, error);
    };
    try {
      const sessionState = envelope.payload.sessionState;
      if (conversationId && !browserClientPluginRuntime.isActiveConversationScope(conversationId)) {
        const deferredRevision = sessionState && typeof sessionState === "object"
          ? Number((sessionState as Record<string, unknown>).revision ?? 0)
          : 0;
        await respond({
          ok: true,
          state: "deferred",
          revision: Number.isFinite(deferredRevision) ? deferredRevision : 0,
        });
        return;
      }
      const reconcileOnly = envelope.payload.reconcileOnly === true;
      let applied = false;
      if (sessionState && typeof sessionState === "object") {
        applied = await browserClientPluginRuntime.synchronizeSession(sessionState as any);
        if (!applied && conversationId && !browserClientPluginRuntime.isActiveConversationScope(conversationId)) {
          const deferredRevision = Number((sessionState as Record<string, unknown>).revision ?? 0);
          await respond({
            ok: true,
            state: "deferred",
            revision: Number.isFinite(deferredRevision) ? deferredRevision : 0,
          });
          return;
        }
      }
      const result = reconcileOnly
        ? {
            ok: true,
            state: "reconciled",
            applied,
            revision: browserClientPluginRuntime.getSessionRevision(conversationId) ?? 0,
            inspection: action === "inspect" ? browserClientPluginRuntime.inspect() : undefined,
          }
        : await executeClientRuntimeCommand(action, payload, conversationId);
      await respond(result);
    } catch (error) {
      await respond({}, error instanceof Error ? error.message : String(error));
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
		"extension_rollback_failed",
		"extension_generation_changed",
		"extension_contributions_changed",
		"ui_provider_changed",
		"ui_profile_changed",
		"ui_slot_changed",
	]);

	function handleExtensionChange(event: MessageEvent) {
		const envelope = parseEnvelope(event);
		if (!envelope) return;
		if (!EXTENSION_CHANGE_EVENTS.has(envelope.eventType)) return
		if (envelope.eventType === "extension_rollback_failed") return
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

  function dispatchEvent(eventName: string, data: string) {
    const event = new MessageEvent(eventName, { data });
    switch (eventName) {
      case "ui_notify":
        handleNotify(event);
        break;
      case "ui_navigate":
        handleNavigate(event);
        break;
      case "ui_dialog":
        handleDialog(event);
        break;
      case "ui_client_runtime_command":
        void handleClientRuntimeCommand(event);
        break;
      case "proactive_message":
        window.dispatchEvent(
          new CustomEvent("amitia:proactive-message", { detail: data }),
        );
        break;
      case "extension_installed":
      case "extension_uninstalled":
      case "extension_enabled":
      case "extension_disabled":
      case "extension_paused":
      case "extension_resumed":
      case "extension_updated":
      case "extension_rolled_back":
      case "extension_rollback_failed":
      case "extension_generation_changed":
      case "extension_contributions_changed":
      case "ui_provider_changed":
      case "ui_profile_changed":
      case "ui_slot_changed":
        handleExtensionChange(event);
        break;
    }
  }

  function processEvents(chunk: string) {
    for (const block of chunk.replace(/\r/g, "").split("\n\n")) {
      if (!block) continue;
      let eventName = "message";
      const data: string[] = [];
      for (const line of block.split("\n")) {
        if (line.startsWith("event:")) {
          eventName = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
          data.push(line.slice(5).trimStart());
        }
      }
      if (data.length > 0) dispatchEvent(eventName, data.join("\n"));
    }
  }

  async function consumeStream(response: Response, version: number) {
    const reader = response.body?.getReader();
    if (!reader) throw new Error("SSE 响应缺少数据流");
    const decoder = new TextDecoder();
    let buffer = "";
    while (version === connectionVersion) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true }).replace(/\r/g, "");
      const boundary = buffer.lastIndexOf("\n\n");
      if (boundary < 0) continue;
      processEvents(buffer.slice(0, boundary));
      buffer = buffer.slice(boundary + 2);
    }
  }

  async function connect() {
    disconnect();
    const version = ++connectionVersion;
    const controller = new AbortController();
    abortController = controller;
    try {
      const url = await resolveApiUrl(`/api/proactive-sse?clientId=ui-host`);
      const init = await createAuthenticatedFetchInit("/api/proactive-sse", {
        headers: { Accept: "text/event-stream" },
        signal: controller.signal,
      });
      const response = await fetch(url, init);
      if (!response.ok || !response.headers.get("content-type")?.includes("text/event-stream")) {
        throw new Error("SSE 连接未建立");
      }
      if (version !== connectionVersion || controller.signal.aborted) return;
      isConnected.value = true;
      reconnectAttempts = 0;
      useExtensionUIStore().refreshSnapshot(true).catch(() => {});
      await consumeStream(response, version);
      if (version === connectionVersion && !controller.signal.aborted) {
        isConnected.value = false;
        scheduleReconnect();
      }
    } catch {
      if (version === connectionVersion && !controller.signal.aborted) {
        isConnected.value = false;
        scheduleReconnect();
      }
    }
  }

  function disconnect() {
    connectionVersion++;
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (extensionRefreshTimer) {
      clearTimeout(extensionRefreshTimer);
      extensionRefreshTimer = null;
    }
    if (abortController) {
      abortController.abort();
      abortController = null;
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
