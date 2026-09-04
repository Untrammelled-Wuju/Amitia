// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, onUnmounted } from "vue";
import { consumeAuthenticatedSSE } from "@/runtime/authenticated-sse";

export type GenerationTaskStatus =
  | "pending"
  | "queued"
  | "processing"
  | "cancelling"
  | "succeeded"
  | "partially_succeeded"
  | "failed"
  | "cancelled";

const TERMINAL_STATUSES: string[] = [
  "succeeded",
  "partially_succeeded",
  "failed",
  "cancelled",
];

export function isTerminalStatus(status?: string | null): boolean {
  return !!status && TERMINAL_STATUSES.includes(status);
}

interface UseGenerationTaskOptions {
  refresh: () => Promise<void> | void;
  getStatus: () => string | undefined | null;
  activeIntervalMs?: number;
  idleIntervalMs?: number;
}

export function useGenerationTask(options: UseGenerationTaskOptions) {
  const {
    refresh,
    getStatus,
    activeIntervalMs = 2000,
    idleIntervalMs = 5000,
  } = options;

  const connected = ref(false);
  let eventAbortController: AbortController | null = null;
  let pollTimer: ReturnType<typeof setTimeout> | null = null;
  let stopped = true;
  let refreshing = false;
  let sseFailed = false;

  function clearPollTimer() {
    if (pollTimer) {
      clearTimeout(pollTimer);
      pollTimer = null;
    }
  }

  function currentInterval(): number {
    if (typeof document !== "undefined" && document.hidden) {
      return idleIntervalMs;
    }
    return activeIntervalMs;
  }

  async function safeRefresh() {
    if (refreshing) return;
    refreshing = true;
    try {
      await refresh();
    } finally {
      refreshing = false;
    }
  }

  function checkTerminalAndMaybeStop() {
    if (stopped) return;
    if (isTerminalStatus(getStatus())) {
      stop();
    }
  }

  function schedulePoll() {
    clearPollTimer();
    if (stopped) return;
    pollTimer = setTimeout(async () => {
      if (stopped) return;
      await safeRefresh();
      if (stopped) return;
      checkTerminalAndMaybeStop();
      if (!stopped) {
        schedulePoll();
      }
    }, currentInterval());
  }

  function disconnectSSE() {
    eventAbortController?.abort();
    eventAbortController = null;
    connected.value = false;
  }

  function handleSSEFailure(controller: AbortController) {
    if (controller.signal.aborted || eventAbortController !== controller) return;
    sseFailed = true;
    eventAbortController = null;
    connected.value = false;
    if (!stopped && !pollTimer) {
      schedulePoll();
    }
  }

  async function connectSSE(taskId: string | number) {
    disconnectSSE();
    const controller = new AbortController();
    eventAbortController = controller;
    const eventTypes = new Set([
      "task.started",
      "task.claimed",
      "task.cancel_requested",
      "task.completed",
      "action.started",
      "action.completed",
      "action.retry",
      "frame.started",
      "frame.succeeded",
      "frame.failed",
    ]);
    const path = `/api/desktop-pets/generation-tasks/${taskId}/events`;

    void consumeAuthenticatedSSE(path, {
      signal: controller.signal,
      onOpen: () => {
        if (stopped || eventAbortController !== controller) {
          controller.abort();
          return;
        }
        connected.value = true;
        sseFailed = false;
      },
      onEvent: ({ event }) => {
        if (stopped || !eventTypes.has(event)) return;
        void safeRefresh().then(() => {
          checkTerminalAndMaybeStop();
        });
      },
    })
      .then(() => handleSSEFailure(controller))
      .catch(() => handleSSEFailure(controller));
  }

  async function start(taskId: string | number) {
    stop();
    stopped = false;
    await safeRefresh();
    checkTerminalAndMaybeStop();
    if (stopped) return;
    await connectSSE(taskId);
    if (!stopped && !pollTimer) {
      schedulePoll();
    }
  }

  function stop() {
    stopped = true;
    disconnectSSE();
    clearPollTimer();
  }

  function onVisibilityChange() {
    if (stopped) return;
    checkTerminalAndMaybeStop();
    if (stopped) return;
    schedulePoll();
  }

  if (typeof document !== "undefined") {
    document.addEventListener("visibilitychange", onVisibilityChange);
  }

  onUnmounted(() => {
    stop();
    if (typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", onVisibilityChange);
    }
  });

  return {
    connected,
    start,
    stop,
  };
}
