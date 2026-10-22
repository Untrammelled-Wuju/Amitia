import { app, BrowserWindow } from "electron";

export interface DesktopClientError {
  message: string;
  stack: string;
  source: string;
  at: string;
}

const MAX_CLIENT_ERRORS = 20;

function text(value: unknown): string {
  if (value instanceof Error) return value.message || String(value);
  if (typeof value === "string") return value;
  if (value == null) return "";
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function shorten(value: unknown, limit: number): string {
  const normalized = text(value).trim();
  return normalized.length > limit ? `${normalized.slice(0, limit)}...` : normalized;
}

export class ClientDiagnosticsBuffer {
  private readonly errors: DesktopClientError[] = [];

  constructor(private readonly limit = MAX_CLIENT_ERRORS) {}

  record(input: { message?: unknown; stack?: unknown; source?: unknown; at?: unknown }): void {
    const message = shorten(input.message, 2000);
    const stack = shorten(input.stack, 4000);
    if (!message && !stack) return;
    this.errors.push({
      message,
      stack,
      source: shorten(input.source, 500),
      at: shorten(input.at, 64) || new Date().toISOString(),
    });
    if (this.errors.length > this.limit) {
      this.errors.splice(0, this.errors.length - this.limit);
    }
  }

  list(): DesktopClientError[] {
    return this.errors.map((item) => ({ ...item }));
  }

  clear(): void {
    this.errors.length = 0;
  }
}

const diagnostics = new ClientDiagnosticsBuffer();
let installed = false;

export function recordDesktopClientError(input: {
  message?: unknown;
  stack?: unknown;
  source?: unknown;
}): void {
  diagnostics.record(input);
}

export function getDesktopClientErrors(): DesktopClientError[] {
  return diagnostics.list();
}

export function clearDesktopClientErrors(): void {
  diagnostics.clear();
}

function attachWindowDiagnostics(win: BrowserWindow): void {
  win.webContents.on("render-process-gone", (_event, details) => {
    diagnostics.record({
      message: `Electron renderer exited: ${details.reason} (exitCode=${details.exitCode})`,
      source: "electron:render-process-gone",
    });
  });
  win.webContents.on("unresponsive", () => {
    diagnostics.record({
      message: "Electron renderer became unresponsive",
      source: "electron:unresponsive",
    });
  });
  win.webContents.on("did-fail-load", (_event, errorCode, errorDescription, validatedURL, isMainFrame) => {
    if (!isMainFrame || errorCode === -3) return;
    diagnostics.record({
      message: `Electron main frame failed to load: ${errorDescription} (${errorCode}) ${validatedURL}`,
      source: "electron:did-fail-load",
    });
  });
}

export function installDesktopClientDiagnostics(): void {
  if (installed) return;
  installed = true;

  process.on("uncaughtExceptionMonitor", (error, origin) => {
    diagnostics.record({
      message: error.message || String(error),
      stack: error.stack,
      source: `electron:main:${origin}`,
    });
  });
  process.on("unhandledRejection", (reason) => {
    diagnostics.record({
      message: reason instanceof Error ? reason.message : reason,
      stack: reason instanceof Error ? reason.stack : "",
      source: "electron:main:unhandledRejection",
    });
  });
  app.on("child-process-gone", (_event, details) => {
    diagnostics.record({
      message: `Electron child process exited: type=${details.type} reason=${details.reason} exitCode=${details.exitCode}`,
      source: "electron:child-process-gone",
    });
  });
  app.on("browser-window-created", (_event, win) => {
    attachWindowDiagnostics(win);
  });
  for (const win of BrowserWindow.getAllWindows()) {
    attachWindowDiagnostics(win);
  }
}
