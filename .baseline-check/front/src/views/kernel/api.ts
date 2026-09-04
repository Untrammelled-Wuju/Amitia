import { apiClient } from "@/composables/useApi";

const BASE = "/api/extensions/kernel";

export interface KernelStatus {
  ready: boolean;
  root?: string;
  count?: number;
  time?: string;
}

export interface KernelExtension {
  extensionId: string;
  version: string;
  installationId: string;
  state: string;
  enablement: string;
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

export async function uninstallExtension(id: string): Promise<{ extensionId: string; uninstalled: boolean }> {
  const res = await apiClient.post(`${BASE}/extensions/uninstall`, { id });
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
