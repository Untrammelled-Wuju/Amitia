import { createHash, randomUUID } from "node:crypto";
import {
  app,
  BrowserWindow,
  ipcMain,
  session,
  type Session,
  type WebContents,
} from "electron";
import { IPC_CHANNELS } from "../shared/ipc";
import { getCoreEndpoint } from "./core-endpoint";
import { getLocalAdminHeaders, requestJSON } from "./backend-session-client";

export interface ExternalWebUIRequest {
  providerId: string;
  extensionId: string;
  url: string;
  allowedOrigins: string[];
  storageKey: string;
  persistent: boolean;
}

interface ProviderEntry {
  type?: string;
  url?: string;
  allowedOrigins?: string[];
}

interface ProviderDefinition {
  providerId?: string;
  extensionId?: string;
  capability?: string;
  enabled?: boolean;
  trustLevel?: string;
  entries?: Record<string, ProviderEntry>;
}

interface ExternalWebUIReservation {
  lease: string;
  partition: string;
  target: URL;
  allowedOrigins: Set<string>;
}

interface ExternalWebUILease {
  owner: WebContents;
  reservation: ExternalWebUIReservation;
  attached: boolean;
  guest?: WebContents;
}

let hostWindow: BrowserWindow | null = null;
let handlersRegistered = false;
const leases = new Map<string, ExternalWebUILease>();
const configuredPartitions = new Set<string>();

function parseHttpURL(value: unknown): URL | null {
  if (typeof value !== "string") return null;
  try {
    const parsed = new URL(value.trim());
    if (!["http:", "https:"].includes(parsed.protocol) || parsed.username || parsed.password) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function validateExternalWebUIRequest(input: unknown): ExternalWebUIRequest {
  if (!input || typeof input !== "object") throw new Error("external web UI request must be an object");
  const row = input as Record<string, unknown>;
  const target = parseHttpURL(row.url);
  if (!target) throw new Error("external web UI url must be HTTP(S) without credentials");
  const providerId = typeof row.providerId === "string" ? row.providerId.trim() : "";
  const extensionId = typeof row.extensionId === "string" ? row.extensionId.trim() : "";
  if (!providerId || providerId.length > 256 || !extensionId || extensionId.length > 256) {
    throw new Error("external web UI provider identity is invalid");
  }
  if (!Array.isArray(row.allowedOrigins) || row.allowedOrigins.length === 0 || row.allowedOrigins.length > 16) {
    throw new Error("external web UI allowedOrigins must contain 1 to 16 origins");
  }
  const allowedOrigins = row.allowedOrigins.map((value) => {
    const origin = parseHttpURL(value);
    if (!origin || origin.pathname !== "/" || origin.search || origin.hash) {
      throw new Error("external web UI allowedOrigins contains an invalid origin");
    }
    return origin.origin;
  });
  if (!allowedOrigins.includes(target.origin)) {
    throw new Error("external web UI target origin must be allowed");
  }
  const storageKey = typeof row.storageKey === "string" ? row.storageKey.trim() : "";
  if (!storageKey || storageKey.length > 2048) throw new Error("external web UI storageKey is invalid");
  return {
    providerId,
    extensionId,
    url: target.href,
    allowedOrigins: [...new Set(allowedOrigins)],
    storageKey,
    persistent: row.persistent === true,
  };
}

function providerEntryKey(platform: NodeJS.Platform): string {
  if (platform === "darwin") return "electron_macos";
  if (platform === "win32") return "electron_windows";
  return "electron_linux";
}

export function externalWebUIProviderAuthorized(
  provider: ProviderDefinition,
  platform: NodeJS.Platform,
  request: ExternalWebUIRequest,
): boolean {
  if (
    provider.providerId !== request.providerId
    || provider.extensionId !== request.extensionId
    || provider.capability !== "workspace.tab"
    || provider.enabled !== true
    || !["system", "official", "trusted", "user_trusted", "verified"].includes(String(provider.trustLevel || "").toLowerCase())
  ) {
    return false;
  }
  const entry = provider.entries?.[providerEntryKey(platform)];
  if (entry?.type !== "external_web_ui" || entry.url !== request.url) return false;
  const expected = new Set((entry.allowedOrigins ?? []).map((value) => new URL(value).origin));
  const actual = new Set(request.allowedOrigins);
  return expected.size === actual.size && [...actual].every((origin) => expected.has(origin));
}

async function authorizeExternalWebUIRequest(request: ExternalWebUIRequest): Promise<void> {
  const payload = await requestJSON<{ providers?: ProviderDefinition[] }>(
    "GET",
    "/api/extensions/ui/providers",
    getLocalAdminHeaders(),
  );
  const provider = payload.providers?.find((item) => item.providerId === request.providerId);
  if (!provider || !externalWebUIProviderAuthorized(provider, process.platform, request)) {
    throw new Error("external web UI provider is not authorized");
  }
}

function partitionFor(request: ExternalWebUIRequest): string {
  if (!request.persistent) return `amitia-external-web-ui-${randomUUID()}`;
  const digest = createHash("sha256").update(request.storageKey).digest("hex").slice(0, 32);
  return `persist:amitia-external-web-ui-${digest}`;
}

function isApplicationRequest(url: URL): boolean {
  const core = getCoreEndpoint();
  if (core) {
    try {
      if (url.origin === new URL(core.httpBaseURL).origin) return true;
    } catch {
    }
  }
  if (!hostWindow || hostWindow.isDestroyed()) return false;
  try {
    const current = new URL(hostWindow.webContents.getURL());
    return current.origin !== "null" && url.origin === current.origin;
  } catch {
    return false;
  }
}

function configureGuestSession(browserSession: Session): void {
  browserSession.setPermissionRequestHandler((_contents, _permission, callback) => callback(false));
  browserSession.setPermissionCheckHandler(() => false);
  browserSession.setDevicePermissionHandler(() => false);
  browserSession.setDisplayMediaRequestHandler((_request, callback) => callback({}));
  browserSession.on("will-download", (event) => event.preventDefault());
  browserSession.webRequest.onBeforeRequest((details, callback) => {
    let url: URL;
    try {
      url = new URL(details.url);
    } catch {
      callback({ cancel: true });
      return;
    }
    if (["data:", "blob:", "about:"].includes(url.protocol)) {
      callback({ cancel: false });
      return;
    }
    if (!["http:", "https:", "ws:", "wss:"].includes(url.protocol)) {
      callback({ cancel: true });
      return;
    }
    callback({ cancel: url.username !== "" || url.password !== "" || isApplicationRequest(url) });
  });
}

function configureGuest(guest: WebContents, lease: ExternalWebUILease): void {
  lease.guest = guest;
  guest.setWindowOpenHandler(() => ({ action: "deny" }));
  guest.on("will-frame-navigate", (event) => {
    if (event.isMainFrame && (!event.url || !lease.reservation.allowedOrigins.has(new URL(event.url).origin))) {
      event.preventDefault();
    }
  });
  guest.on("will-redirect", (event, url, _isInPlace, isMainFrame) => {
    if (isMainFrame && !lease.reservation.allowedOrigins.has(new URL(url).origin)) event.preventDefault();
  });
  guest.on("will-attach-webview", (event) => event.preventDefault());
  guest.on("login", (event, _details, _authInfo, callback) => {
    event.preventDefault();
    callback();
  });
  guest.once("destroyed", () => leases.delete(lease.reservation.lease));
}

function releaseLease(owner: WebContents, leaseID: string): void {
  const lease = leases.get(leaseID);
  if (!lease || lease.owner !== owner) return;
  leases.delete(leaseID);
  const guest = lease.guest;
  if (guest && !guest.isDestroyed()) guest.close({ waitForBeforeUnload: false });
}

function releaseOwnerLeases(owner: WebContents): void {
  for (const [leaseID, lease] of leases) {
    if (lease.owner === owner) releaseLease(owner, leaseID);
  }
}

function registerHandlers(): void {
  if (handlersRegistered) return;
  handlersRegistered = true;
  ipcMain.handle(IPC_CHANNELS.externalWebUIAcquire, (event, input: unknown) => {
    if (!hostWindow || hostWindow.isDestroyed() || event.sender !== hostWindow.webContents) {
      throw new Error("external web UI is unavailable for this window");
    }
    const request = validateExternalWebUIRequest(input);
    return authorizeExternalWebUIRequest(request).then(() => {
      if (!hostWindow || hostWindow.isDestroyed() || event.sender !== hostWindow.webContents) {
        throw new Error("external web UI is unavailable for this window");
      }
      const partition = partitionFor(request);
      if (!configuredPartitions.has(partition)) {
        configureGuestSession(session.fromPartition(partition));
        configuredPartitions.add(partition);
      }
      const leaseID = randomUUID();
      leases.set(leaseID, {
        owner: event.sender,
        attached: false,
        reservation: {
          lease: leaseID,
          partition,
          target: new URL(request.url),
          allowedOrigins: new Set(request.allowedOrigins),
        },
      });
      return { lease: leaseID, partition };
    });
  });
  ipcMain.handle(IPC_CHANNELS.externalWebUIRelease, (event, leaseID: unknown) => {
    if (typeof leaseID === "string") releaseLease(event.sender, leaseID);
  });
  ipcMain.handle(IPC_CHANNELS.externalWebUIClear, async (event, input: unknown) => {
    if (!hostWindow || hostWindow.isDestroyed() || event.sender !== hostWindow.webContents) {
      throw new Error("external web UI is unavailable for this window");
    }
    const request = validateExternalWebUIRequest(input);
    await authorizeExternalWebUIRequest(request);
    const partition = partitionFor(request);
    const browserSession = session.fromPartition(partition);
    if (!configuredPartitions.has(partition)) {
      configureGuestSession(browserSession);
      configuredPartitions.add(partition);
    }
    await browserSession.clearStorageData();
    await browserSession.clearCache();
  });
}

export function attachExternalWebUIHost(window: BrowserWindow): void {
  registerHandlers();
  hostWindow = window;
  const owner = window.webContents;
  owner.on("will-attach-webview", (event, preferences, params) => {
    const leaseID = typeof params.src === "string" && params.src.startsWith("about:blank#")
      ? params.src.slice("about:blank#".length)
      : "";
    const lease = leases.get(leaseID);
    if (!lease || lease.owner !== owner || lease.attached || params.partition !== lease.reservation.partition) {
      event.preventDefault();
      return;
    }
    lease.attached = true;
    for (const key of Object.keys(preferences)) Reflect.deleteProperty(preferences, key);
    Object.assign(preferences, {
      nodeIntegration: false,
      nodeIntegrationInWorker: false,
      nodeIntegrationInSubFrames: false,
      contextIsolation: true,
      sandbox: true,
      webSecurity: true,
      allowRunningInsecureContent: false,
      webviewTag: false,
      plugins: false,
      navigateOnDragDrop: false,
      disableDialogs: true,
      devTools: !app.isPackaged,
    });
    params.preload = "";
    params.httpreferrer = "";
  });
  owner.on("did-attach-webview", (_event, guest) => {
    guest.once("dom-ready", () => {
      const current = guest.getURL();
      const leaseID = current.startsWith("about:blank#") ? current.slice("about:blank#".length) : "";
      const lease = leases.get(leaseID);
      if (!lease || lease.owner !== owner || lease.guest) {
        guest.close({ waitForBeforeUnload: false });
        return;
      }
      configureGuest(guest, lease);
    });
  });
  owner.on("did-start-navigation", (_event, _url, inPlace, isMainFrame) => {
    if (isMainFrame && !inPlace) releaseOwnerLeases(owner);
  });
  owner.on("render-process-gone", () => releaseOwnerLeases(owner));
  owner.once("destroyed", () => {
    releaseOwnerLeases(owner);
    if (hostWindow === window) hostWindow = null;
  });
}
