import type { AgentUIEvent } from "@/conversation/runtime/agentEventReducer";
import type { WorkbenchOpenCommand, WorkbenchViewKind } from "@/stores/workbench";
import { resolveUIHostDeviceId } from "@/ui-runtime/deviceIdentity";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";

const kinds: readonly WorkbenchViewKind[] = ["files", "file", "git", "diff", "browser", "terminal", "tasks"];
const deviceIdentifier = /^[a-zA-Z0-9_:.-]{1,140}$/;

// A browser-event origin is observational metadata. A remote target is selected
// only in cloud mode and is always authorized again by Cloud Core/Device Mesh.
// Never silently interpret another device's browser reference as local.
export function resolveWorkbenchBrowserTarget(command: WorkbenchOpenCommand, origin: string, local: string, mode: string): WorkbenchOpenCommand | null {
  if (command.kind !== "browser" || !command.browserRef || command.targetDeviceId || !origin) return command;
  if (!deviceIdentifier.test(origin)) return null;
  if (origin === local) return command;
  // Local mode has only one bound Runtime. The backend still validates the
  // observation reference against that Runtime and conversation scope.
  if (mode !== "cloud") return command;
  return { ...command, targetDeviceId: origin };
}


export function workbenchCommandFromEvent(event: AgentUIEvent, currentConversationId: string): WorkbenchOpenCommand | null {
  if (!event || event.type !== "workbench.open" || event.version !== 1) return null;
  const targetConversationId = String(event.conversationId || currentConversationId || "").trim();
  if (!/^[a-zA-Z0-9_:.-]{1,160}$/.test(targetConversationId)) return null;
  const payload = event.payload;
  if (!payload || typeof payload !== "object" || !kinds.includes(payload.kind as WorkbenchViewKind)) return null;
  const resourceUri = String(payload.resourceUri || "").trim();
  const workspaceUri = String(payload.workspaceUri || "").trim();
  const targetDeviceId = String(payload.targetDeviceId || "").trim();
  const originDeviceId = typeof payload.originDeviceId === "string" ? payload.originDeviceId.trim() : "";
  if ((targetDeviceId && !deviceIdentifier.test(targetDeviceId)) || (originDeviceId && !deviceIdentifier.test(originDeviceId))) return null;
  const browserRef = payload.kind === "browser" && typeof payload.browserRef === "string" ? payload.browserRef : "";
  if (browserRef && !/^[A-Za-z0-9_-]{32}$/.test(browserRef)) return null;
  if (resourceUri.length > 4096 || workspaceUri.length > 4096 || targetDeviceId.length > 140) return null;
  if (payload.kind === "tasks" && (resourceUri || workspaceUri || targetDeviceId || originDeviceId || payload.browserRef || payload.taskId)) return null;
  if (payload.kind === "browser") {
    try {
      const url = new URL(resourceUri);
      if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password) return null;
    } catch { return null; }
  }
  if (payload.kind === "file" || payload.kind === "diff") {
    if (!resourceUri || !workspaceUri) return null;
  }
  return {
    protocolVersion: 1,
    conversationId: targetConversationId,
    kind: payload.kind as WorkbenchViewKind,
    title: String(payload.title || "").slice(0, 140),
    resourceUri,
    workspaceUri,
    targetDeviceId,
    browserRef,
    focus: payload.focus === "activate" ? "activate" : "background",
    forceNew: payload.forceNew === true,
    operationId: String(payload.operationId || event.eventId || "").slice(0, 140),
  };
}

let dispatchQueue: Promise<void> = Promise.resolve();

export function publishWorkbenchEvent(event: AgentUIEvent, currentConversationId: string): boolean {
  const command = workbenchCommandFromEvent(event, currentConversationId);
  if (!command || typeof window === "undefined") return false;
  const origin = typeof event.payload?.originDeviceId === "string" ? event.payload.originDeviceId.trim() : "";
  // Preserve event ordering even when Desktop identity resolution is async.
  dispatchQueue = dispatchQueue.catch(() => {}).then(async () => {
    let resolved: WorkbenchOpenCommand | null = command;
    if (origin && command.browserRef && !command.targetDeviceId) {
      try {
        const [local, deployment] = await Promise.all([resolveUIHostDeviceId(), getDeploymentConfig()]);
        resolved = resolveWorkbenchBrowserTarget(command, origin, local, deployment.mode);
      } catch {
        // An unidentified remote Agent tab must never be opened against the
        // local runtime. Keep the event unconsumed until identity is known.
        resolved = null;
      }
    }
    if (resolved) window.dispatchEvent(new CustomEvent("amitia:workbench:open", { detail: resolved }));
  });
  return true;
}
