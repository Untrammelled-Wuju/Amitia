import { apiClient } from "@/composables/useApi";

export type DataPortabilityScope = "all" | "character" | "settings" | "memory";
export type DataPortabilityProfile = "portable" | "full";
export type DataPortabilityCollisionPolicy = "duplicate" | "skip" | "replace";

export interface DataPortabilityComponent {
  id: string;
  logicalName: string;
  kind: string;
  itemCount: number;
  required?: boolean;
  sensitive?: boolean;
  rebuildable?: boolean;
  estimatedSize?: number;
}

export interface DataPortabilityExportPreview {
  scope: DataPortabilityScope;
  profile: DataPortabilityProfile;
  characterId: string;
  encrypted: boolean;
  components: DataPortabilityComponent[];
  estimatedSize: number;
}

export interface DataPortabilityExportResult {
  operationId: string;
  file: string;
  size: number;
  checksum: string;
  scope: DataPortabilityScope;
  profile: DataPortabilityProfile;
  characterId: string;
  encrypted: boolean;
  components: DataPortabilityComponent[];
  createdAt: string;
  appVersion: string;
}

export interface DataPortabilityImportPreview {
  ticketId: string;
  backupId: string;
  createdAt: string;
  expiresAt: string;
  preview: {
    manifest: {
      backupId: string;
      appVersion: string;
      schemaFingerprint: string;
      profile: DataPortabilityProfile;
      scope: DataPortabilityScope;
      characterId?: string;
      encrypted: boolean;
      sourcePlatform: string;
      requiresReindex: boolean;
    };
    components: Array<DataPortabilityComponent & { collisions?: Array<Record<string, unknown>>; warnings?: string[] }>;
    canImport: boolean;
    requiresReindex: boolean;
  };
}

export interface DataPortabilityImportResult {
  ticketId: string;
  operationId: string;
  status: string;
  preImportBackupId: string;
  stats: Record<string, number>;
  totalImported: number;
  componentCount: number;
  requiresReindex: boolean;
}

export interface DataPortabilityBackup {
  name: string;
  size: number;
  createdAt: string;
  encrypted: boolean;
}

export interface DataPortabilityRestorePreview {
  operationId: string;
  backupId: string;
  createdAt: string;
  encrypted: boolean;
  manifest: {
    appVersion: string;
    schemaFingerprint: string;
    profile: string;
    scope: string;
    characterId?: string;
    createdAt: string;
    components: DataPortabilityComponent[];
  };
}

export async function previewDataPortabilityExport(payload: {
  scope: DataPortabilityScope;
  profile: DataPortabilityProfile;
  characterId?: string;
  passphrase?: string;
}) {
  const response = await apiClient.post<DataPortabilityExportPreview>("/api/data-portability/exports/preview", payload);
  return response.data;
}

export async function exportDataPortability(payload: {
  scope: DataPortabilityScope;
  profile: DataPortabilityProfile;
  characterId?: string;
  passphrase?: string;
}) {
  const response = await apiClient.post<DataPortabilityExportResult>("/api/data-portability/exports", payload);
  return response.data;
}

export async function downloadDataPortabilityExport(file: string) {
  const response = await apiClient.get(`/api/data-portability/exports/${encodeURIComponent(file)}`, { responseType: "blob" });
  return response.data instanceof Blob ? response.data : new Blob([response.data]);
}

export async function inspectDataPortabilityImport(file: File, passphrase: string) {
  const form = new FormData();
  form.append("file", file);
  if (passphrase) form.append("passphrase", passphrase);
  const response = await apiClient.post<DataPortabilityImportPreview>("/api/data-portability/imports/inspect", form);
  return response.data;
}

export async function commitDataPortabilityImport(ticketId: string, policy: DataPortabilityCollisionPolicy, activate: boolean) {
  const response = await apiClient.post<DataPortabilityImportResult>(
    `/api/data-portability/imports/${encodeURIComponent(ticketId)}/commit`,
    { policy, activate },
    { timeout: 10 * 60 * 1000 },
  );
  return response.data;
}

export async function cancelDataPortabilityImport(ticketId: string) {
  await apiClient.delete(`/api/data-portability/imports/${encodeURIComponent(ticketId)}`);
}

export async function listDataPortabilityBackups() {
  const response = await apiClient.get<{ backups: DataPortabilityBackup[] }>("/api/data-portability/backups");
  return response.data.backups ?? [];
}

export async function createDataPortabilityBackup(passphrase: string) {
  const response = await apiClient.post<{ backupId: string; name: string }>(
    "/api/data-portability/backups",
    { passphrase },
    { timeout: 10 * 60 * 1000 },
  );
  return response.data;
}

export async function deleteDataPortabilityBackup(name: string) {
  await apiClient.delete(`/api/data-portability/backups/${encodeURIComponent(name)}`);
}

export async function previewDataPortabilityRestore(name: string, passphrase: string) {
  const response = await apiClient.post<DataPortabilityRestorePreview>(
    `/api/data-portability/backups/${encodeURIComponent(name)}/restore/preview`,
    { passphrase },
    { timeout: 10 * 60 * 1000 },
  );
  return response.data;
}

export async function executeDataPortabilityRestore(operationId: string) {
  const response = await apiClient.post<{
    operationId: string;
    status: string;
    preRestoreBackupId: string;
    requiresRestart: boolean;
    stats: Record<string, number>;
  }>(
    `/api/data-portability/restores/${encodeURIComponent(operationId)}`,
    {},
    { timeout: 10 * 60 * 1000 },
  );
  return response.data;
}

export async function cancelDataPortabilityRestore() {
  await apiClient.delete("/api/data-portability/restores/pending");
}
