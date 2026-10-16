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

export function installClientErrorTracking(app: App): void {
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
