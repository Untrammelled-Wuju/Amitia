// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive, onUnmounted } from "vue";
import { useApi, apiClient } from "./useApi";
import { resolveApiUrl } from "@/runtime/runtime-adapter";
import { ElMessage } from "element-plus";

export type ProcessingTaskStatus =
  | "queued"
  | "processing"
  | "succeeded"
  | "partially_succeeded"
  | "failed"
  | "cancelled"
  | "pending";

export type ProcessingStage =
  | "queued"
  | "validating_sources"
  | "background_removal"
  | "compositing"
  | "finalizing"
  | "completed"
  | "failed"
  | "cancelled"
  | string;

export type ActionQualityLevel = "normal" | "warning" | "failed";

export type ProcessingActionStatus =
  | "pending"
  | "processing"
  | "succeeded"
  | "failed"
  | "cancelled";

const TERMINAL_STATUSES: string[] = [
  "succeeded",
  "partially_succeeded",
  "failed",
  "cancelled",
];

export function isProcessingTerminalStatus(
  status?: string | null,
): boolean {
  return !!status && TERMINAL_STATUSES.includes(status);
}

export interface ProcessingTask {
  id: string;
  generationTaskId: string;
  processingVersion: number;
  status: ProcessingTaskStatus | string;
  currentStage: ProcessingStage;
  progress: number;
  outputWidth: number;
  outputHeight: number;
  targetCharacterHeightRatio?: number;
  anchorMode?: string;
  backgroundMode?: string;
  outputFormat?: string;
  defaultFps?: number;
  executionId?: string;
  workerId?: string;
  leaseExpiresAt?: string;
  lastHeartbeatAt?: string;
  cancelRequestedAt?: string;
  errorCode?: string;
  errorMessage?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface ProcessingActionInfo {
  actionKey: string;
  actionName: string;
  status: ProcessingActionStatus | string;
  progress: number;
  qualityLevel: ActionQualityLevel | string;
  qualityFlags: string[];
  sourceAttempt: number;
  excluded: boolean;
}

export interface ProcessingQualitySummary {
  totalActions: number;
  succeededActions: number;
  failedActions: number;
  warningActions: number;
}

export interface GetProcessingTaskResponse {
  processingTask: ProcessingTask;
  actions: ProcessingActionInfo[];
  qualitySummary: ProcessingQualitySummary;
  previewPaths: Record<string, string>;
}

export interface CreateProcessingTaskParams {
  outputWidth: number;
  outputHeight: number;
  targetCharacterHeightRatio?: number;
  anchorMode?: string;
  backgroundMode?: string;
  outputFormat?: string;
  defaultFps?: number;
}

export interface CreatePackageParams {
  defaultAction: string;
  includedActions: string[];
  userDefaultAction?: string;
  petId?: string;
  version?: string;
}

export interface CreatePackageResponse {
  releaseId: string;
  petId: string;
  version: string;
  status: string;
  packageHash: string;
  packageId: string;
}

export interface ProcessingEventCallbacks {
  onProgress?: (payload: { progress: number; stage?: string }) => void;
  onAction?: (payload: { actionKey: string; status?: string }) => void;
  onActionProgress?: (payload: {
    actionKey: string;
    progress?: number;
    stage?: string;
  }) => void;
  onCompleted?: (payload: {
    status?: string;
    succeeded?: number;
    failed?: number;
    total?: number;
  }) => void;
  onConnected?: (payload: { taskId?: string }) => void;
  onError?: (err: Event) => void;
}

interface UseProcessingTaskOptions {
  activeIntervalMs?: number;
  idleIntervalMs?: number;
}

export function useProcessingTask(options: UseProcessingTaskOptions = {}) {
  const { post, get } = useApi();
  const {
    activeIntervalMs = 2000,
    idleIntervalMs = 5000,
  } = options;

  const processingTask = ref<ProcessingTask | null>(null);
  const actions = ref<ProcessingActionInfo[]>([]);
  const qualitySummary = ref<ProcessingQualitySummary>({
    totalActions: 0,
    succeededActions: 0,
    failedActions: 0,
    warningActions: 0,
  });
  const previewPaths = ref<Record<string, string>>({});
  const isConnected = ref(false);
  const loading = ref(false);
  const submitting = ref(false);

  const objectUrls = reactive<Set<string>>(new Set());

  let eventSource: EventSource | null = null;
  let pollTimer: ReturnType<typeof setTimeout> | null = null;
  let stopped = true;
  let refreshing = false;
  let sseFailed = false;
  let currentCallbacks: ProcessingEventCallbacks | null = null;
  let currentProcessingTaskId: string | number | null = null;

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
      await refreshState();
    } finally {
      refreshing = false;
    }
  }

  function checkTerminalAndMaybeStop() {
    if (stopped) return;
    const status = processingTask.value?.status;
    if (isProcessingTerminalStatus(status)) {
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
    isConnected.value = false;
  }

  async function connectSSE(processingTaskId: string | number) {
    disconnectSSE();
    try {
      const url = await resolveApiUrl(
        `/api/desktop-pets/processing-tasks/${processingTaskId}/events`,
      );
      const source = new EventSource(url);
      eventSource = source;
      source.onopen = () => {
        if (stopped) {
          disconnectSSE();
          return;
        }
        isConnected.value = true;
        sseFailed = false;
      };
      source.addEventListener("connected", (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data);
          currentCallbacks?.onConnected?.({ taskId: data?.taskId });
        } catch {
          currentCallbacks?.onConnected?.({});
        }
      });
      source.addEventListener("ping", () => {
        // heartbeat, no-op
      });
      source.addEventListener("processing.progress", (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data);
          currentCallbacks?.onProgress?.({
            progress: data?.progress,
            stage: data?.stage,
          });
        } catch {}
      });
      source.addEventListener("processing.action", (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data);
          currentCallbacks?.onAction?.({
            actionKey: data?.actionKey,
            status: data?.status,
          });
        } catch {}
      });
      source.addEventListener(
        "processing.action.progress",
        (event: MessageEvent) => {
          try {
            const data = JSON.parse(event.data);
            currentCallbacks?.onActionProgress?.({
              actionKey: data?.actionKey,
              progress: data?.progress,
              stage: data?.stage,
            });
          } catch {}
        },
      );
      source.addEventListener(
        "processing.completed",
        (event: MessageEvent) => {
          try {
            const data = JSON.parse(event.data);
            currentCallbacks?.onCompleted?.({
              status: data?.status,
              succeeded: data?.succeeded,
              failed: data?.failed,
              total: data?.total,
            });
          } catch {}
          void safeRefresh().then(() => {
            checkTerminalAndMaybeStop();
          });
        },
      );
      source.onerror = (err: Event) => {
        sseFailed = true;
        disconnectSSE();
        currentCallbacks?.onError?.(err);
        if (!stopped && !pollTimer) {
          schedulePoll();
        }
      };
    } catch {
      sseFailed = true;
      disconnectSSE();
    }
  }

  async function refreshState() {
    if (!currentProcessingTaskId) return;
    try {
      const data = await get<GetProcessingTaskResponse>(
        `/api/desktop-pets/processing-tasks/${currentProcessingTaskId}`,
      );
      applyState(data);
    } catch (err: any) {
      if (!sseFailed) {
        // 静默处理，避免与全局错误提示重复
      }
    }
  }

  function applyState(data?: GetProcessingTaskResponse | null) {
    if (!data) return;
    processingTask.value = data.processingTask || null;
    actions.value = data.actions || [];
    qualitySummary.value = data.qualitySummary || {
      totalActions: 0,
      succeededActions: 0,
      failedActions: 0,
      warningActions: 0,
    };
    previewPaths.value = data.previewPaths || {};
  }

  async function loadProcessingTask(processingTaskId: string | number) {
    currentProcessingTaskId = processingTaskId;
    loading.value = true;
    try {
      const data = await get<GetProcessingTaskResponse>(
        `/api/desktop-pets/processing-tasks/${processingTaskId}`,
      );
      applyState(data);
    } finally {
      loading.value = false;
    }
  }

  async function createProcessingTask(
    taskId: string | number,
    params: CreateProcessingTaskParams,
  ): Promise<ProcessingTask> {
    submitting.value = true;
    try {
      const fd = new FormData();
      fd.append("outputWidth", String(params.outputWidth ?? 0));
      fd.append("outputHeight", String(params.outputHeight ?? 0));
      if (params.targetCharacterHeightRatio !== undefined) {
        fd.append(
          "targetCharacterHeightRatio",
          String(params.targetCharacterHeightRatio),
        );
      }
      if (params.anchorMode) fd.append("anchorMode", params.anchorMode);
      if (params.backgroundMode) fd.append("backgroundMode", params.backgroundMode);
      if (params.outputFormat) fd.append("outputFormat", params.outputFormat);
      if (params.defaultFps !== undefined) {
        fd.append("defaultFps", String(params.defaultFps));
      }
      const data = await post<ProcessingTask>(
        `/api/desktop-pets/generation-tasks/${taskId}/process`,
        fd,
        { timeout: 60000 },
      );
      return data;
    } finally {
      submitting.value = false;
    }
  }

  async function cancelProcessingTask(
    processingTaskId: string | number,
  ): Promise<void> {
    await post(
      `/api/desktop-pets/processing-tasks/${processingTaskId}/cancel`,
    );
  }

  async function retryProcessingAction(
    processingTaskId: string | number,
    actionKey: string,
  ): Promise<void> {
    await post(
      `/api/desktop-pets/processing-tasks/${processingTaskId}/actions/${actionKey}/retry`,
    );
  }

  async function createPackage(
    processingTaskId: string | number,
    params: CreatePackageParams,
  ): Promise<CreatePackageResponse> {
    submitting.value = true;
    try {
      const idempotencyKey =
        typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
          ? crypto.randomUUID()
          : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
      const data = await post<{ release: any; validation: any }>(
        `/api/desktop-pets/releases/build`,
        {
          processingTaskId: String(processingTaskId),
          petId: params.petId || "",
          version: params.version || "",
          defaultAction: params.defaultAction || "",
          includedActions: params.includedActions || [],
          idempotencyKey,
        },
      );
      const release = data?.release || {};
      return {
        releaseId: release.id || "",
        petId: release.petId || "",
        version: release.version || "",
        status: release.status || "",
        packageHash: release.contentRootHash || release.manifestHash || "",
        packageId: release.id || "",
      };
    } finally {
      submitting.value = false;
    }
  }

  async function switchAttempt(
    processingTaskId: string | number,
    actionKey: string,
    attemptNumber: number,
  ): Promise<void> {
    await post(
      `/api/desktop-pets/processing-tasks/${processingTaskId}/actions/${actionKey}/switch-attempt`,
      { attemptNumber },
    );
  }

  async function excludeAction(
    processingTaskId: string | number,
    actionKey: string,
  ): Promise<void> {
    await post(
      `/api/desktop-pets/processing-tasks/${processingTaskId}/actions/${actionKey}/exclude`,
    );
  }

  function subscribeProcessingEvents(
    processingTaskId: string | number,
    callbacks?: ProcessingEventCallbacks,
  ) {
    stop();
    currentProcessingTaskId = processingTaskId;
    currentCallbacks = callbacks || null;
    stopped = false;
    void safeRefresh().then(() => {
      checkTerminalAndMaybeStop();
      if (stopped) return;
      void connectSSE(processingTaskId);
      if (!stopped && !pollTimer) {
        schedulePoll();
      }
    });
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
    revokeObjectUrls();
  });

  function revokeObjectUrls() {
    objectUrls.forEach((url) => {
      if (url) {
        try {
          URL.revokeObjectURL(url);
        } catch {}
      }
    });
    objectUrls.clear();
  }

  async function fetchBlob(url: string): Promise<string> {
    const res = await apiClient.get(url, { responseType: "blob" });
    const blob = res.data as Blob;
    if (!blob || blob.size === 0) return "";
    const objectUrl = URL.createObjectURL(blob);
    objectUrls.add(objectUrl);
    return objectUrl;
  }

  async function fetchActionPreview(
    processingTaskId: string | number,
    actionKey: string,
  ): Promise<string> {
    try {
      return await fetchBlob(
        `/api/desktop-pets/processing-tasks/${processingTaskId}/actions/${actionKey}/preview`,
      );
    } catch {
      return "";
    }
  }

  async function fetchProcessedFrame(
    processingTaskId: string | number,
    actionKey: string,
    frameIndex: number,
  ): Promise<string> {
    try {
      return await fetchBlob(
        `/api/desktop-pets/processing-tasks/${processingTaskId}/actions/${actionKey}/frames/${frameIndex}/processed-image`,
      );
    } catch {
      return "";
    }
  }

  async function fetchSourceFrame(
    processingTaskId: string | number,
    actionKey: string,
    frameIndex: number,
  ): Promise<string> {
    try {
      return await fetchBlob(
        `/api/desktop-pets/processing-tasks/${processingTaskId}/actions/${actionKey}/frames/${frameIndex}/source-image`,
      );
    } catch {
      return "";
    }
  }

  function notifyError(err: any, fallback: string) {
    const msg = err?.message || fallback;
    ElMessage.error(msg);
  }

  return {
    processingTask,
    actions,
    qualitySummary,
    previewPaths,
    isConnected,
    loading,
    submitting,
    createProcessingTask,
    getProcessingTask: loadProcessingTask,
    cancelProcessingTask,
    retryProcessingAction,
    createPackage,
    switchAttempt,
    excludeAction,
    subscribeProcessingEvents,
    stop,
    refresh: refreshState,
    fetchActionPreview,
    fetchProcessedFrame,
    fetchSourceFrame,
    revokeObjectUrls,
    notifyError,
  };
}
