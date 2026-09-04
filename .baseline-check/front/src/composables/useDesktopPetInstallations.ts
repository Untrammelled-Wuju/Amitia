// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { useApi, apiClient } from "./useApi";

export type InstallationStatus =
  | "installing"
  | "installed"
  | "enabled"
  | "disabled"
  | "invalid"
  | "uninstalling"
  | "uninstalled"
  | string;

export type InstallationRuntimeStatus =
  | "installed"
  | "pending_runtime"
  | "enabled"
  | "running"
  | "offline"
  | "corrupted"
  | "recovery_required"
  | string;

export interface DesktopPetInstallation {
  id: string;
  userId?: string;
  characterId: string;
  packageId: string;
  packageVersion: string;
  name: string;
  status: InstallationStatus;
  isActive: number;
  installPath?: string;
  manifestPath?: string;
  previewPath?: string;
  defaultActionKey?: string;
  canvasWidth?: number;
  canvasHeight?: number;
  packageHash?: string;
  installedAt?: string;
  lastEnabledAt?: string;
  lastDisabledAt?: string;
  createdAt?: string;
  updatedAt?: string;
  deviceId?: string;
  currentReleaseId?: string;
  lifecycleState?: string;
  integrityStatus?: string;
  legacyPackageId?: string;
  previewArtifactPath?: string;
  defaultActionReleaseId?: string;
  installedContentHash?: string;
  runtimeSyncState?: string;
  stateRevision?: number;
}

export interface RuntimeSettings {
  id?: string;
  installationId?: string;
  alwaysOnTop: number;
  scale: number;
  positionX: number;
  positionY: number;
  screenId?: string;
  idleEnabled: number;
  idleIntervalMinSeconds: number;
  idleIntervalMaxSeconds: number;
  clickThroughMode: string;
  soundEnabled: number;
  restoreOnAppStart?: number;
  positionMode?: string;
  displayFingerprint?: string;
  relativeX?: number;
  relativeY?: number;
  lastWindowWidth?: number;
  lastWindowHeight?: number;
  settingsRevision?: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface ManifestActionInfo {
  key: string;
  name: string;
  config?: string;
  supportsDefaultIdle?: boolean;
}

export interface InstallationDetail extends DesktopPetInstallation {
  settings?: RuntimeSettings;
  manifest?: {
    schemaVersion?: number;
    manifestFormat?: string;
    petId?: string;
    releaseId?: string;
    version?: string;
    name?: string;
    binding?: { policy?: string; sourceCharacterId?: string };
    canvas?: { width?: number; height?: number; coordinateSystem?: string };
    defaultAction?: string;
    preview?: string;
    actions?: ManifestActionInfo[];
    capabilities?: {
      transparentBackground?: boolean;
      frameSequence?: boolean;
      perFrameDuration?: boolean;
      audio?: boolean;
    };
  };
  characterName?: string;
}

export interface InstallParams {
  petId: string;
  releaseId: string;
  characterId?: string;
}

export interface UpdateSettingsParams {
  alwaysOnTop?: number;
  restoreOnAppStart?: number;
  scale?: number;
  positionX?: number;
  positionY?: number;
  screenId?: string;
  idleEnabled?: number;
  idleIntervalMinSeconds?: number;
  idleIntervalMaxSeconds?: number;
  clickThroughMode?: string;
  soundEnabled?: number;
}

export interface ListInstallationsResponse {
  items: DesktopPetInstallation[];
  total: number;
}

export interface UpdateSettingsResponse {
  settings: RuntimeSettings;
  operationId?: string;
  status?: string;
  stage?: string;
}

export interface InstallationOperationResult {
  operationId: string;
  installationId?: string;
  status: string;
  stage?: string;
}

interface InstallationOperation {
  id: string;
  installationId?: string;
  status: string;
  stage?: string;
  errorCode?: string;
  errorMessage?: string;
}

export function useDesktopPetInstallations() {
  const { get, post } = useApi();
  const loading = ref(false);
  const submitting = ref(false);
  const installations = ref<DesktopPetInstallation[]>([]);
  const total = ref(0);
  const currentInstallation = ref<InstallationDetail | null>(null);

  function buildInstallationsUrl(
    installationId: string,
    suffix?: string,
  ): string {
    const base = `/api/desktop-pets/installations/${encodeURIComponent(installationId)}`;
    return suffix ? `${base}${suffix}` : base;
  }

  function createIdempotencyKey(): string {
    return typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  }

  async function trackOperation(
    submission: InstallationOperationResult,
  ): Promise<InstallationOperationResult> {
    let current = submission;
    for (let attempt = 0; attempt < 12; attempt += 1) {
      if (current?.status === "failed_terminal" || current?.status === "cancelled") {
        throw new Error("桌宠操作失败");
      }
      if (!current?.operationId || !["created", "queued", "running", "failed_retryable", "cancel_requested"].includes(current.status)) {
        break;
      }
      await new Promise((resolve) => window.setTimeout(resolve, 150));
      const data = await get<{ operation: InstallationOperation }>(
        `/api/desktop-pets/operations/${encodeURIComponent(current.operationId)}`,
      );
      const op = data?.operation;
      if (!op) break;
      current = {
        operationId: op.id,
        installationId: op.installationId || current.installationId,
        status: op.status,
        stage: op.stage,
      };
      if (op.status === "failed_terminal" || op.status === "cancelled") {
        throw new Error(op.errorMessage || op.errorCode || "桌宠操作失败");
      }
    }
    return current;
  }

  async function submitOperation(
    url: string,
    data: Record<string, any> = {},
  ): Promise<InstallationOperationResult> {
    const idempotencyKey = createIdempotencyKey();
    const result = await post<InstallationOperationResult>(url, data, {
      headers: { "Idempotency-Key": idempotencyKey },
    });
    return trackOperation(result);
  }

  function showOperationMessage(action: string, result: InstallationOperationResult): void {
    if (result.status === "completed") {
      ElMessage.success(`${action}完成`);
      return;
    }
    if (result.status === "waiting_runtime_ack") {
      ElMessage.info(`${action}已提交，等待桌宠运行时同步`);
      return;
    }
    ElMessage.info(`${action}请求已提交`);
  }

  async function install(
    petId: string,
    releaseId: string,
    characterId?: string,
  ): Promise<InstallationOperationResult> {
    submitting.value = true;
    try {
      const idempotencyKey = createIdempotencyKey();
      const data = await post<InstallationOperationResult>(
        `/api/desktop-pets/pets/${encodeURIComponent(petId)}/releases/${encodeURIComponent(releaseId)}/install`,
        { characterId: characterId || "", idempotencyKey },
        { headers: { "Idempotency-Key": idempotencyKey } },
      );
      const tracked = await trackOperation(data);
      showOperationMessage("安装", tracked);
      return tracked;
    } finally {
      submitting.value = false;
    }
  }

  async function listInstallations(): Promise<DesktopPetInstallation[]> {
    loading.value = true;
    try {
      const data = await get<ListInstallationsResponse>(
        `/api/desktop-pets/installations`,
      );
      installations.value = data?.items || [];
      total.value = data?.total || 0;
      return installations.value;
    } catch (err: any) {
      ElMessage.error(err?.message || "加载安装列表失败");
      installations.value = [];
      total.value = 0;
      return [];
    } finally {
      loading.value = false;
    }
  }

  async function getInstallation(
    installationId: string,
  ): Promise<InstallationDetail> {
    loading.value = true;
    try {
      const data = await get<InstallationDetail>(
        buildInstallationsUrl(installationId),
      );
      currentInstallation.value = data || null;
      return data;
    } finally {
      loading.value = false;
    }
  }

  async function enable(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(buildInstallationsUrl(installationId, "/enable"));
      showOperationMessage("启用", result);
    } finally {
      submitting.value = false;
    }
  }

  async function disable(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(buildInstallationsUrl(installationId, "/disable"));
      showOperationMessage("停用", result);
    } finally {
      submitting.value = false;
    }
  }

  async function updateDefaultAction(
    installationId: string,
    actionKey: string,
  ): Promise<void> {
    submitting.value = true;
    try {
      const idempotencyKey = createIdempotencyKey();
      const res = await apiClient.patch(
        buildInstallationsUrl(installationId, "/default-action"),
        { actionKey },
        { headers: { "Idempotency-Key": idempotencyKey } },
      );
      const result = await trackOperation(res.data as InstallationOperationResult);
      showOperationMessage("默认动作更新", result);
    } finally {
      submitting.value = false;
    }
  }

  async function updateSettings(
    installationId: string,
    settings: UpdateSettingsParams,
  ): Promise<RuntimeSettings | null> {
    submitting.value = true;
    try {
      const idempotencyKey = createIdempotencyKey();
      const res = await apiClient.patch(
        buildInstallationsUrl(installationId, "/settings"),
        settings,
        { headers: { "Idempotency-Key": idempotencyKey } },
      );
      const payload = (res.data as UpdateSettingsResponse) || null;
      if (payload?.operationId) {
        const tracked = await trackOperation({
          operationId: payload.operationId,
          status: payload.status || "",
          stage: payload.stage,
        });
        showOperationMessage("运行配置更新", tracked);
      } else {
        ElMessage.success("运行配置已更新");
      }
      return payload?.settings || null;
    } finally {
      submitting.value = false;
    }
  }

  async function recenter(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(buildInstallationsUrl(installationId, "/recenter"));
      showOperationMessage("位置重置", result);
    } finally {
      submitting.value = false;
    }
  }

  async function playAction(
    installationId: string,
    actionKey: string,
  ): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(
        buildInstallationsUrl(installationId, `/actions/${encodeURIComponent(actionKey)}/play`),
      );
      showOperationMessage("动作播放", result);
    } finally {
      submitting.value = false;
    }
  }

  async function uninstall(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      const idempotencyKey = createIdempotencyKey();
      const res = await apiClient.delete(buildInstallationsUrl(installationId), {
        headers: { "Idempotency-Key": idempotencyKey },
      });
      const result = await trackOperation(res.data as InstallationOperationResult);
      showOperationMessage("卸载", result);
    } finally {
      submitting.value = false;
    }
  }

  async function upgrade(
    installationId: string,
    targetReleaseId: string,
  ): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(
        buildInstallationsUrl(installationId, "/upgrade"),
        { targetReleaseId },
      );
      showOperationMessage("升级", result);
    } finally {
      submitting.value = false;
    }
  }

  async function switchInstallation(
    installationId: string,
  ): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(buildInstallationsUrl(installationId, "/enable"));
      showOperationMessage("切换当前桌宠", result);
    } finally {
      submitting.value = false;
    }
  }

  async function repair(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      const result = await submitOperation(buildInstallationsUrl(installationId, "/repair"));
      showOperationMessage("修复", result);
    } finally {
      submitting.value = false;
    }
  }

  async function refresh(): Promise<DesktopPetInstallation[]> {
    return listInstallations();
  }

  return {
    loading,
    submitting,
    installations,
    total,
    currentInstallation,
    install,
    listInstallations,
    getInstallation,
    enable,
    disable,
    updateDefaultAction,
    updateSettings,
    recenter,
    playAction,
    uninstall,
    upgrade,
    switchInstallation,
    repair,
    refresh,
  };
}
