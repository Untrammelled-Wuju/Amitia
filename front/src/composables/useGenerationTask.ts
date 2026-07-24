// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, onUnmounted } from "vue";
import { resolveApiUrl } from "@/runtime/runtime-adapter";

export type GenerationTaskStatus =
  | "pending"
  | "queued"
  | "processing"
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
  let eventSource: EventSource | null = null;
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
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    connected.value = false;
  }

  async function connectSSE(taskId: string | number) {
    disconnectSSE();
    try {
      const url = await resolveApiUrl(
        `/api/desktop-pets/generation-tasks/${taskId}/events`,
      );
      const source = new EventSource(url);
      eventSource = source;
      source.onopen = () => {
        if (stopped) {
          disconnectSSE();
          return;
        }
        connected.value = true;
        sseFailed = false;
      };
      const eventTypes = [
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
      ];
      const handleEvent = () => {
        if (stopped) return;
        void safeRefresh().then(() => {
          checkTerminalAndMaybeStop();
        });
      };
      eventTypes.forEach((type) => {
        source.addEventListener(type, handleEvent);
      });
      source.onerror = () => {
        sseFailed = true;
        disconnectSSE();
        if (!stopped && !pollTimer) {
          schedulePoll();
        }
      };
    } catch {
      sseFailed = true;
      disconnectSSE();
    }
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
