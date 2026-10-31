// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import type { App } from "vue";

export interface TrackedClientError {
  message: string;
  stack: string;
  source: string;
  at: string;
}

const MAX_TRACKED_ERRORS = 15;
const tracked: TrackedClientError[] = [];
let installed = false;

function shorten(value: unknown, limit: number): string {
  const text = typeof value === "string" ? value : value == null ? "" : String(value);
  const trimmed = text.trim();
  return trimmed.length > limit ? `${trimmed.slice(0, limit)}...` : trimmed;
}

export function recordClientError(input: { message?: unknown; stack?: unknown; source?: unknown }): void {
  const message = shorten(input.message, 2000);
  const stack = shorten(input.stack, 4000);
  if (!message && !stack) return;
  tracked.push({
    message,
    stack,
    source: shorten(input.source, 500),
    at: new Date().toISOString(),
  });
  while (tracked.length > MAX_TRACKED_ERRORS) tracked.shift();
}

export function collectClientErrors(): TrackedClientError[] {
  return tracked.map((item) => ({ ...item }));
}

export function clearClientErrors(): void {
  tracked.length = 0;
  if (typeof window !== "undefined") {
    void window.amitiaDesktop?.clearClientDiagnostics?.().catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
  }
}

export async function collectAllClientErrors(): Promise<TrackedClientError[]> {
  const rendererErrors = collectClientErrors();
  const desktopErrors = typeof window !== "undefined"
    ? await window.amitiaDesktop?.getClientDiagnostics?.().catch((error) => {
  console.error("[original-error]", error);
  return [];
}) ?? []
    : [];
  const merged = [...desktopErrors, ...rendererErrors];
  const seen = new Set<string>();
  const timestamp = (value: string): number => {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : 0;
  };
  return merged
    .filter((item) => {
      const key = `${item.at}\u0000${item.source}\u0000${item.message}\u0000${item.stack}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .sort((left, right) => timestamp(left.at) - timestamp(right.at))
    .slice(-20);
}

function describeReason(reason: unknown): { message: string; stack: string } {
  if (reason instanceof Error) {
    return { message: reason.message || String(reason), stack: reason.stack || "" };
  }
  if (reason && typeof reason === "object") {
    const candidate = reason as { message?: unknown; stack?: unknown };
    return {
      message: shorten(candidate.message ?? JSON.stringify(reason), 2000),
      stack: shorten(candidate.stack, 4000),
    };
  }
  return { message: shorten(reason, 2000), stack: "" };
}

export function installWindowErrorTracking(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;

  window.addEventListener("error", (event) => {
    const target = event as ErrorEvent;
    const described = describeReason(target.error);
    recordClientError({
      message: described.message || target.message,
      stack: described.stack,
      source: `window.onerror ${target.filename || ""}:${target.lineno || 0}:${target.colno || 0}`,
    });
  });

  window.addEventListener("unhandledrejection", (event) => {
    const described = describeReason((event as PromiseRejectionEvent).reason);
    recordClientError({
      message: described.message,
      stack: described.stack,
      source: "unhandledrejection",
    });
  });

}

export function installClientErrorTracking(app: App): void {
  installWindowErrorTracking();
  const previousHandler = app.config.errorHandler;
  app.config.errorHandler = (error, instance, info) => {
    const described = describeReason(error);
    recordClientError({
      message: described.message,
      stack: described.stack,
      source: `vue:${info || "unknown"}`,
    });
    if (previousHandler) {
      previousHandler(error, instance, info);
    } else {
      console.error(error);
    }
  };
}
