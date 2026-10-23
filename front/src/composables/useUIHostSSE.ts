import { ref, onUnmounted, watch, type Ref } from "vue";
import { useRouter } from "vue-router";
import { ElNotification, ElMessageBox } from "element-plus";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import { isNavigationAllowed } from "../navigation/nav-whitelist";
import { useExtensionUIStore } from "../stores/extensionUI";
import { browserClientPluginRuntime, type BrowserDeclarativeClientPackage } from "../ui-runtime/clientPluginRuntime";
import { resolveUIHostDeviceId } from "../ui-runtime/deviceIdentity";
import { executeStyleCommand } from "../ui-runtime/styleCommand";
import { resolveHostEnvironment } from "./useHostEnvironment";
import { deliverScheduledBrowserNotification, registerScheduledBrowserEndpoint, type ScheduledBrowserEnvelope } from '../runtime/scheduled-notifications';
import { reportNotificationPresence } from "../runtime/device-notifications";

interface SSEEventEnvelope {
  eventType: string;
  requestId: string;
  sessionId: string;
  extensionId: string;
  payload: Record<string, unknown>;
  expiresAt?: string;
  timestamp: string;
  hostClientId?: string;
  hostSessionId?: string;
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
  let hostHeartbeatTimer: ReturnType<typeof setInterval> | null = null;
  let hostClientId = "";
  let hostSessionId = "";

  const disposeMainStyleCommand = window.amitiaDesktop?.onUIStyleCommand?.((data) => {
    void handleStyleCommand(new MessageEvent("ui_style_command", { data }));
  });

  function getStableHostClientId(): string {
    if (hostClientId) return hostClientId;
    const key = "amitia.ui.host-client-id.v1";
    try {
      const existing = window.sessionStorage.getItem(key)?.trim();
      if (existing) {
        hostClientId = existing;
        return existing;
      }
      const suffix = typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
        ? crypto.randomUUID()
        : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 14)}`;
      hostClientId = `ui-host-${suffix}`;
      window.sessionStorage.setItem(key, hostClientId);
    } catch {
      hostClientId = `ui-host-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 14)}`;
    }
    return hostClientId;
  }

  async function postHostSession(path: string, body: Record<string, unknown>): Promise<Record<string, unknown>> {
    const url = await resolveApiUrl(path);
    const init = await createAuthenticatedFetchInit(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const response = await fetch(url, init);
    if (!response.ok) throw new Error(`UI host session request failed: ${response.status}`);
    return await response.json() as Record<string, unknown>;
  }

  async function registerHostSession(): Promise<string> {
    const clientId = getStableHostClientId();
    const deviceId = await resolveUIHostDeviceId();
    const environment = resolveHostEnvironment();
    const desktopStyleHost = typeof window !== "undefined" && Boolean(window.amitiaDesktop?.onUIStyleCommand);
    const result = await postHostSession("/api/extensions/ui/host-session", {
      hostClientId: clientId,
      deviceId,
      platform: environment.platform === "macos" ? "darwin" : environment.platform,
      windowId: "main",
      features: [
        "ui.notify",
        "ui.dialog",
        "ui.navigate",
        ...(desktopStyleHost ? [] : ["ui.style"]),
      ],
    });
    hostSessionId = String(result.hostSessionId ?? "").trim();
    if (!hostSessionId) throw new Error("UI host registration returned no hostSessionId");
    if (hostHeartbeatTimer) clearInterval(hostHeartbeatTimer);
    const intervalSeconds = Math.max(20, Number(result.heartbeatIntervalSeconds ?? 60) || 60);
    hostHeartbeatTimer = setInterval(() => {
      if (!hostSessionId) return;
      void postHostSession("/api/extensions/ui/host-session/heartbeat", {
        hostClientId: clientId,
        hostSessionId,
      }).catch(() => {});
    }, intervalSeconds * 1000);
    return clientId;
  }

  async function unregisterHostSession(): Promise<void> {
    if (hostHeartbeatTimer) {
      clearInterval(hostHeartbeatTimer);
      hostHeartbeatTimer = null;
    }
    const clientId = hostClientId;
    const sessionId = hostSessionId;
    hostSessionId = "";
    if (!clientId || !sessionId) return;
    try {
      await postHostSession("/api/extensions/ui/host-session/disconnect", {
        hostClientId: clientId,
        hostSessionId: sessionId,
      });
    } catch {
      // Expiration/heartbeat cleanup remains authoritative if the client exits abruptly.
    }
  }

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

  async function sendDialogResponse(
    dialogId: string,
    result: string,
    responseHostClientId = hostClientId,
    responseHostSessionId = hostSessionId,
  ): Promise<void> {
    try {
      const url = await resolveApiUrl("/api/extensions/ui/dialog-response");
      const init = await createAuthenticatedFetchInit("/api/extensions/ui/dialog-response", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          dialogId,
          result,
          hostClientId: responseHostClientId,
          hostSessionId: responseHostSessionId,
        }),
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

  async function handleStyleCommand(event: MessageEvent) {
    const envelope = parseEnvelope(event);
    if (!envelope || !envelope.payload || !shouldProcess(envelope)) return;
    const commandId = String(envelope.payload.commandId ?? "").trim();
    if (!commandId) return;
    const action = String(envelope.payload.action ?? "").trim();
    const expectedRevision = Number(envelope.payload.expectedRevision ?? 0) || 0;
    const changes = envelope.payload.changes && typeof envelope.payload.changes === "object" && !Array.isArray(envelope.payload.changes)
      ? envelope.payload.changes as Record<string, unknown>
      : {};
    for (const key of [
      "scope",
      "target",
      "selector",
      "component",
      "targets",
      "rules",
      "states",
      "state",
      "properties",
      "property",
      "transactionId",
      "source",
      "limit",
      "includeComputed",
      "page",
      "platform",
      "surface",
    ]) {
      if (changes[key] === undefined && envelope.payload[key] !== undefined) {
        changes[key] = envelope.payload[key];
      }
    }
    const payloadHostClientId = String(envelope.payload.hostClientId ?? envelope.hostClientId ?? hostClientId).trim();
    const payloadHostSessionId = String(envelope.payload.hostSessionId ?? envelope.hostSessionId ?? hostSessionId).trim();
    try {
      const result = await executeStyleCommand(action, expectedRevision, changes);
      await sendClientRuntimeResponse(
        commandId,
        payloadHostClientId,
        payloadHostSessionId,
        result,
      );
    } catch (error) {
      await sendClientRuntimeResponse(
        commandId,
        payloadHostClientId,
        payloadHostSessionId,
        {},
        error instanceof Error ? error.message : String(error),
      );
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
    const responseHostClientId = String(envelope.hostClientId ?? hostClientId).trim();
    const responseHostSessionId = String(envelope.hostSessionId ?? hostSessionId).trim();
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
        void sendDialogResponse(dialogId, buttons[0], responseHostClientId, responseHostSessionId);
      })
      .catch((action: string) => {
        if (action === "cancel" && buttons.length > 1) {
          void sendDialogResponse(dialogId, buttons[1], responseHostClientId, responseHostSessionId);
        } else {
          void sendDialogResponse(dialogId, "closed", responseHostClientId, responseHostSessionId);
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

  function activeNotificationConversationId(): string {
    if (router.currentRoute.value.path !== "/chat") return "";
    const value = router.currentRoute.value.query.conversationId;
    return typeof value === "string" ? value : "";
  }

  async function reportPresence(): Promise<void> {
    try {
      const deviceId = await resolveUIHostDeviceId();
      const foreground = document.visibilityState === "visible" && document.hasFocus();
      await reportNotificationPresence(deviceId, foreground, foreground ? activeNotificationConversationId() : "");
    } catch {}
  }

  async function clearPresence(): Promise<void> {
    try {
      const deviceId = await resolveUIHostDeviceId();
      await reportNotificationPresence(deviceId, false, "");
    } catch {}
  }

  function dispatchEvent(eventName: string, data: string) {
    const event = new MessageEvent(eventName, { data });
    switch (eventName) {
      case "scheduled_notification": {
        if (window.amitiaDesktop) break;
        const envelope = parseEnvelope(event);
        if (!envelope || !envelope.payload || !shouldProcess(envelope)) break;
        void deliverScheduledBrowserNotification(envelope as ScheduledBrowserEnvelope, {
          acknowledge: async (state) => {
            const path = '/api/notifications/scheduled/' + encodeURIComponent(envelope.requestId) + '/ack';
            const response = await fetch(await resolveApiUrl(path), await createAuthenticatedFetchInit(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ state }) }));
            if (!response.ok) throw new Error('通知回执未确认');
          },
          navigate: (target) => { if (isNavigationAllowed(target)) void router.push(target); },
        }).catch(() => {});
        break;
      }
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
      case "ui_style_command":
        void handleStyleCommand(event);
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
      const clientId = getStableHostClientId();
      const url = await resolveApiUrl(`/api/proactive-sse?clientId=${encodeURIComponent(clientId)}`);
      const init = await createAuthenticatedFetchInit("/api/proactive-sse", {
        headers: { Accept: "text/event-stream" },
        signal: controller.signal,
      });
      const response = await fetch(url, init);
      if (!response.ok || !response.headers.get("content-type")?.includes("text/event-stream")) {
        throw new Error("SSE 连接未建立");
      }
      if (version !== connectionVersion || controller.signal.aborted) return;
      await registerHostSession();
      if (!window.amitiaDesktop) {
        const deviceId = await resolveUIHostDeviceId();
        const path = '/api/notifications/devices/register';
        const permitted = typeof Notification !== 'undefined' && Notification.permission === 'granted';
        await registerScheduledBrowserEndpoint(deviceId, permitted, async (input) => {
          const response = await fetch(await resolveApiUrl(path), await createAuthenticatedFetchInit(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) }));
          if (!response.ok) throw new Error('浏览器通知通道未确认注册');
          return response.json();
        });
      }
      await reportPresence();
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
    if (hostHeartbeatTimer) {
      clearInterval(hostHeartbeatTimer);
      hostHeartbeatTimer = null;
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
    disposeMainStyleCommand?.();
    void unregisterHostSession();
    disposePresence();
    void clearPresence();
    disconnect();
    if (dedupCleanupTimer) {
      clearInterval(dedupCleanupTimer);
      dedupCleanupTimer = null;
    }
  });

  const stopPresenceRouteWatch = watch(
    () => router.currentRoute.value.fullPath,
    () => { void reportPresence(); },
  );
  window.addEventListener("focus", reportPresence);
  window.addEventListener("blur", reportPresence);
  document.addEventListener("visibilitychange", reportPresence);

  function disposePresence() {
    stopPresenceRouteWatch();
    window.removeEventListener("focus", reportPresence);
    window.removeEventListener("blur", reportPresence);
    document.removeEventListener("visibilitychange", reportPresence);
  }

  return {
    isConnected,
    connect,
    disconnect,
  };
}
