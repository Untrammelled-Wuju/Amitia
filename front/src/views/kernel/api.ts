import { apiClient } from "@/composables/useApi";

const BASE = "/api/extensions/kernel";

export interface KernelStatus {
  ready: boolean;
  root?: string;
  count?: number;
  time?: string;
}

export interface KernelExtension {
  name?: string;
  extensionId: string;
  version: string;
  installationId: string;
  state: string;
  enablement: string;
  systemManaged?: boolean;
  installedAt: string;
  updatedAt: string;
  generation: number;
}

export interface KernelExtensionDetail extends KernelExtension {
  publisher?: string;
  signatureStatus?: string;
  trustLevel?: string;
  runtimeStatus?: string;
  permissions?: KernelPermission[];
  scopes?: KernelScope[];
  dependencies?: KernelDependency[];
  circuitState?: string;
  quarantineState?: string;
  modules: KernelModule[];
  contributions: KernelContribution[];
}

export interface KernelPermission {
  name: string;
  scope?: string;
  reason?: string;
  required?: boolean;
  granted?: boolean;
}

export interface KernelScope {
  name: string;
  granted?: boolean;
}

export interface KernelDependency {
  type: string;
  id: string;
  version?: string;
  optional?: boolean;
  satisfied?: boolean;
}

export interface KernelModule {
  id: string;
  type: string;
  runtime: string;
  entryPoint: string;
  contributionCount: number;
}

export interface KernelContribution {
  id: string;
  kind: string;
  moduleId: string;
  name: string;
}

export interface InstallPreview {
  extensionId: string;
  name: string;
  version: string;
  publisher: string;
  installable: boolean;
  category: string;
  archiveHash: string;
  contentTreeHash: string;
  securityPassed: boolean;
  issues: PreviewIssue[];
  modules: PreviewModule[];
  missingDependencies: PreviewDependency[];
  requiredPermissions: PreviewPermission[];
}

export interface PreviewIssue {
  category: string;
  code: string;
  message: string;
  path?: string;
}

export interface PreviewModule {
  id: string;
  name: string;
  type: string;
  runtime?: string;
  supported: boolean;
}

export interface PreviewDependency {
  type: string;
  id: string;
  version?: string;
  optional: boolean;
  reason?: string;
  missing: boolean;
}

export interface PreviewPermission {
  name: string;
  reason?: string;
  required: boolean;
  scope?: string;
}

export interface InstallResult {
  extensionId: string;
  version: string;
  installationId: string;
  packageHash: string;
  contentTreeHash: string;
  artifactPath: string;
  installPath: string;
  definitionHash: string;
  installedAt: string;
}

export async function getKernelStatus(): Promise<KernelStatus> {
  const res = await apiClient.get(`${BASE}/status`);
  return res.data;
}

export async function listExtensions(): Promise<{ extensions: KernelExtension[]; total: number }> {
  const res = await apiClient.get(`${BASE}/extensions`);
  return res.data;
}

export async function getExtension(id: string): Promise<KernelExtensionDetail> {
  const res = await apiClient.get(`${BASE}/extension`, { params: { id } });
  return res.data;
}

export async function setExtensionPermission(
  extensionId: string,
  permission: string,
  granted: boolean,
): Promise<KernelPermission> {
  const res = await apiClient.post(`${BASE}/extensions/permissions`, {
    extensionId,
    permission,
    granted,
  });
  return res.data;
}

export async function previewInstall(file: File): Promise<InstallPreview> {
  const formData = new FormData();
  formData.append("package", file);
  const res = await apiClient.post(`${BASE}/extensions/preview`, formData);
  return res.data;
}

export async function installExtension(file: File): Promise<InstallResult> {
  const formData = new FormData();
  formData.append("package", file);
  const res = await apiClient.post(`${BASE}/extensions/install`, formData);
  return res.data;
}

export async function enableExtension(id: string): Promise<{ extensionId: string; enablement: string }> {
  const res = await apiClient.post(`${BASE}/extensions/enable`, { id });
  return res.data;
}

export async function disableExtension(id: string): Promise<{ extensionId: string; enablement: string }> {
  const res = await apiClient.post(`${BASE}/extensions/disable`, { id });
  return res.data;
}

export async function uninstallExtension(id: string): Promise<Record<string, unknown>> {
  const scopeType = "global";
  const scopeId = "";
  let previewResponse = await apiClient.post(`${BASE}/extensions/uninstall/preview`, {
    extensionId: id,
    scopeType,
    scopeId,
  });
  let preview = previewResponse.data as {
    uninstallable?: boolean;
    enabled?: boolean;
    dependents?: string[];
    requiredConfirmations?: string[];
  };
  let disabledForUninstall = false;
  if (!preview.uninstallable && preview.enabled && !(preview.dependents || []).length) {
    await disableExtension(id);
    disabledForUninstall = true;
    previewResponse = await apiClient.post(`${BASE}/extensions/uninstall/preview`, {
      extensionId: id,
      scopeType,
      scopeId,
    });
    preview = previewResponse.data;
  }
  if (preview.uninstallable === false) {
    const dependents = (preview.dependents || []).filter(Boolean);
    throw new Error(dependents.length > 0
      ? `存在依赖此扩展的插件：${dependents.join("、")}`
      : preview.enabled
        ? "请先停用该扩展后再卸载"
        : "当前扩展不可卸载");
  }
  const confirmations: Record<string, boolean> = {};
  for (const key of preview.requiredConfirmations || []) {
    if (key) confirmations[key] = true;
  }
  let confirmResponse: { data?: { confirmationToken?: string } };
  try {
    confirmResponse = await apiClient.post(`${BASE}/extensions/uninstall/confirm`, {
      extensionId: id,
      scopeType,
      scopeId,
      confirmations,
    });
  } catch (error) {
    if (disabledForUninstall) await enableExtension(id).catch(() => {});
    throw error;
  }
  const confirmationToken = String(confirmResponse.data?.confirmationToken || "");
  if (!confirmationToken) throw new Error("卸载确认令牌缺失");
  const idempotencyKey =
    typeof globalThis.crypto?.randomUUID === "function"
      ? `extension-uninstall:${id}:${globalThis.crypto.randomUUID()}`
      : `extension-uninstall:${id}:${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const res = await apiClient.post(
    `${BASE}/extensions/uninstall`,
    {
      extensionId: id,
      scopeType,
      scopeId,
      confirmationToken,
      idempotencyKey,
    },
    {
      headers: { "Idempotency-Key": idempotencyKey },
    },
  );
  return res.data;
}

export async function pauseExtension(id: string): Promise<{ extensionId: string; state: string }> {
  const res = await apiClient.post(`${BASE}/extensions/pause`, { id });
  return res.data;
}

export async function rollbackExtension(id: string): Promise<{ extensionId: string; rolledBack: boolean }> {
  const res = await apiClient.post(`${BASE}/extensions/rollback`, { id });
  return res.data;
}
