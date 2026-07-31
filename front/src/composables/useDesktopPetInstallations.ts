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

export type RuntimeStateSnapshot = Record<string, any>;

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
  launchOnStartup: number;
  scale: number;
  positionX: number;
  positionY: number;
  screenId?: string;
  idleEnabled: number;
  idleIntervalMinSeconds: number;
  idleIntervalMaxSeconds: number;
  clickThroughMode: string;
  soundEnabled: number;
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
    packageId?: string;
    name?: string;
    characterId?: string;
    generationTaskId?: string;
    processingVersion?: number;
    createdAt?: string;
    canvas?: { width?: number; height?: number };
    defaultAction?: string;
    preview?: string;
    actions?: ManifestActionInfo[];
    capabilities?: {
      hasTransparentBackground?: boolean;
      supportsFrameSequence?: boolean;
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
  launchOnStartup?: number;
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
}

export function useDesktopPetInstallations() {
  const { get, post, del } = useApi();
  const loading = ref(false);
  const submitting = ref(false);
  const installations = ref<DesktopPetInstallation[]>([]);
  const total = ref(0);
  const currentInstallation = ref<InstallationDetail | null>(null);

  function buildInstallationsUrl(
    installationId: string,
    suffix?: string,
  ): string {
    const base = `/api/desktop-pets/installations/${installationId}`;
    return suffix ? `${base}${suffix}` : base;
  }

  async function install(
    petId: string,
    releaseId: string,
    characterId?: string,
  ): Promise<DesktopPetInstallation> {
    submitting.value = true;
    try {
      const idempotencyKey =
        typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
          ? crypto.randomUUID()
          : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
      const data = await post<DesktopPetInstallation>(
        `/api/desktop-pets/pets/${petId}/releases/${releaseId}/install`,
        { characterId: characterId || "", idempotencyKey },
      );
      ElMessage.success("桌宠已安装");
      return data;
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
      await post(buildInstallationsUrl(installationId, "/enable"));
      ElMessage.success("桌宠已启用");
    } finally {
      submitting.value = false;
    }
  }

  async function disable(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      await post(buildInstallationsUrl(installationId, "/disable"));
      ElMessage.success("桌宠已停用");
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
      await post(
        buildInstallationsUrl(installationId, "/default-action"),
        { action_key: actionKey },
      );
      ElMessage.success("默认动作已更新");
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
      const res = await apiClient.patch(
        buildInstallationsUrl(installationId, "/settings"),
        settings,
      );
      const updated = (res.data as RuntimeSettings) || null;
      ElMessage.success("运行配置已更新");
      return updated;
    } finally {
      submitting.value = false;
    }
  }

  async function recenter(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      await post(buildInstallationsUrl(installationId, "/recenter"));
      ElMessage.success("桌宠已重置位置");
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
      await post(
        buildInstallationsUrl(installationId, "/play-action"),
        { actionKey },
      );
      ElMessage.success("动作已触发");
    } finally {
      submitting.value = false;
    }
  }

  async function uninstall(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      await del(buildInstallationsUrl(installationId));
      ElMessage.success("桌宠已卸载");
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
      await post(buildInstallationsUrl(installationId, "/upgrade"), {
        targetReleaseId,
      });
      ElMessage.success("升级请求已提交");
    } finally {
      submitting.value = false;
    }
  }

  async function switchInstallation(
    installationId: string,
  ): Promise<void> {
    submitting.value = true;
    try {
      await post(buildInstallationsUrl(installationId, "/switch"));
      ElMessage.success("切换请求已提交");
    } finally {
      submitting.value = false;
    }
  }

  async function repair(installationId: string): Promise<void> {
    submitting.value = true;
    try {
      await post(buildInstallationsUrl(installationId, "/repair"));
      ElMessage.success("修复请求已提交");
    } finally {
      submitting.value = false;
    }
  }

  async function getDesiredState(): Promise<RuntimeStateSnapshot> {
    loading.value = true;
    try {
      return await get<RuntimeStateSnapshot>(
        "/api/desktop-pets/runtime/desired-state",
      );
    } finally {
      loading.value = false;
    }
  }

  async function getActualState(): Promise<RuntimeStateSnapshot> {
    loading.value = true;
    try {
      return await get<RuntimeStateSnapshot>(
        "/api/desktop-pets/runtime/actual-state",
      );
    } finally {
      loading.value = false;
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
    getDesiredState,
    getActualState,
    refresh,
  };
}
