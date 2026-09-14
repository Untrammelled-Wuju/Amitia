import { apiClient } from "@/composables/useApi";
import type {
  AgentSkillDetail,
  AgentSkillDefinition,
  AgentSkillPage,
  AgentSkillPreview,
  LocalExtensionPackage,
  PackageImportPreview,
  PackageOperation,
  PackageOperationResult,
  PackageSigner,
  PackageVersion,
  ExportedPackage,
  PackageDependency,
  TaskListFilters,
  TaskProgress,
  TaskResult,
  TaskRun,
  TaskRunPage,
} from "./types";

export type { PackageImportPreview, PackageOperationResult } from "./types";

const agentSkillPath = (id = "") =>
  `/api/extensions/agent-skills${id ? `/${encodeURIComponent(id)}` : ""}`;
export async function previewAgentSkillZIP(file: File) {
  const data = new FormData();
  data.append("source", "zip");
  data.append("file", file);
  const response = await apiClient.post(
    `${agentSkillPath()}/import/preview`,
    data,
  );
  return response.data as AgentSkillPreview;
}
export async function previewAgentSkillDirectory(
  rootName: string,
  files: Array<{ path: string; file: File }>,
) {
  const data = new FormData();
  data.append("source", "directory");
  data.append("rootName", rootName);
  data.append("paths", JSON.stringify(files.map((item) => item.path)));
  files.forEach((item) => data.append("files", item.file, item.file.name));
  const response = await apiClient.post(
    `${agentSkillPath()}/import/preview`,
    data,
  );
  return response.data as AgentSkillPreview;
}
export async function fetchAgentSkills(
  params: Record<string, unknown> = {},
) {
  const response = await apiClient.get(agentSkillPath(), { params });
  return response.data as AgentSkillPage;
}
export async function fetchAgentSkill(id: string) {
  const response = await apiClient.get(agentSkillPath(id));
  return response.data as AgentSkillDetail;
}
export async function installAgentSkill(previewId: string) {
  const response = await apiClient.post(`${agentSkillPath()}/import/install`, {
    previewId,
    enable: false,
  });
  return response.data as AgentSkillDefinition;
}
export async function setAgentSkillEnabled(id: string, enabled: boolean) {
  await apiClient.post(
    `${agentSkillPath(id)}/${enabled ? "enable" : "disable"}`,
  );
}
export async function removeAgentSkill(id: string) {
  await apiClient.delete(agentSkillPath(id));
}
export async function previewAgentSkillMCPDependencies(
  agentSkillExtensionId: string,
  dependencies: unknown[],
) {
  const response = await apiClient.post(
    "/api/mcp/agent-skills/dependencies/preview",
    { agentSkillExtensionId, dependencies },
  );
  return response.data?.data ?? response.data;
}
export async function installAgentSkillMCPDependencies(
  plan: unknown,
  options: {
    installOptional: boolean;
    confirmHTTP: boolean;
    confirmStdio: boolean;
  },
) {
  const response = await apiClient.post(
    "/api/mcp/agent-skills/dependencies/install",
    { plan, ...options, enableServers: true },
  );
  return response.data?.data ?? response.data;
}
export async function removeAgentSkillMCPDependencies(id: string) {
  const response = await apiClient.delete(
    `/api/mcp/agent-skills/${encodeURIComponent(id)}/dependencies`,
  );
  return response.data?.data ?? response.data;
}

export async function previewExtensionPackage(
  file: File,
  scopeType: "global" | "character",
  scopeId: string,
  extensionId = "",
  onProgress?: (percent: number) => void,
  managementTarget?: "game-center" | "pet-center",
) {
  const data = new FormData();
  data.append("file", file);
  data.append("scopeType", scopeType);
  data.append("scopeId", scopeId);
  if (extensionId) data.append("expectedExtensionId", extensionId);
  const response = await apiClient.post("/api/extensions/packages/artifacts", data, {
    headers: managementTarget ? { "X-Amitia-Management-Target": managementTarget } : undefined,
    onUploadProgress: (event) =>
      onProgress?.(
        event.total ? Math.round((event.loaded * 100) / event.total) : 0,
      ),
  });
  return response.data.preview as PackageImportPreview;
}

export async function fetchLocalExtensionPackageStatus() {
  const response = await apiClient.get("/api/extensions/packages/status");
  return response.data as { ready: boolean; installed: number };
}

export async function fetchLocalExtensionPackages() {
  const response = await apiClient.get("/api/extensions/packages");
  return response.data as LocalExtensionPackage[];
}

export async function installLocalExtensionPackage(
  file: File,
  onProgress?: (percent: number) => void,
) {
  return previewExtensionPackage(file, "global", "", "", onProgress);
}
export async function previewExtensionDirectory(
  rootName: string,
  files: Array<{ path: string; base64: string }>,
  scopeType: "global" | "character",
  scopeId: string,
  onProgress?: (percent: number) => void,
) {
  void rootName;
  void files;
  void scopeType;
  void scopeId;
  void onProgress;
  throw new Error("扩展目录直装已退役，请先构建插件系统 v1 的 .amitiax 扩展包");
}
export async function installExtensionPackage(
  preview: PackageImportPreview,
  confirmations: {
    unsigned: boolean;
    scripts: boolean;
    capabilities: string[];
    versionChange: boolean;
    signerChange: boolean;
    configMigration: boolean;
  },
  upgradeId = "",
  managementTarget?: "game-center" | "pet-center",
) {
  const targetExtensionId = upgradeId || (preview.currentVersion ? preview.id : "");
  const operationType = targetExtensionId ? "update" : "install";
  const operationNonce =
    typeof globalThis.crypto?.randomUUID === "function"
      ? globalThis.crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const idempotencyKey = `package:${operationType}:${preview.sessionId}:${operationNonce}`;
  const confirmationMap: Record<string, boolean> = {};
  for (const key of preview.capabilityConfirmations ?? []) {
    confirmationMap[key] = true;
  }
  confirmationMap["confirm.unsigned_dev"] = confirmations.unsigned;
  confirmationMap["confirm.scripts"] = confirmations.scripts;
  confirmationMap["confirm.version_change"] = confirmations.versionChange;
  confirmationMap["confirm.signer_change"] = confirmations.signerChange;
  confirmationMap["confirm.config_migration"] = confirmations.configMigration;
  confirmationMap["confirm.permission_escalation"] =
    confirmations.capabilities.length > 0;
  const confirmed = await apiClient.post(
    `/api/extensions/packages/previews/${encodeURIComponent(preview.sessionId)}/confirm`,
    {
      scopeType: preview.scopeType,
      scopeId: preview.scopeId,
      confirmations: confirmationMap,
    },
    managementTarget ? { headers: { "X-Amitia-Management-Target": managementTarget } } : undefined,
  );
  const response = await apiClient.post(
    targetExtensionId
      ? "/api/extensions/packages/operations/update"
      : "/api/extensions/packages/operations/install",
    {
      sessionId: preview.sessionId,
      scopeType: preview.scopeType,
      scopeId: preview.scopeId,
      confirmationToken: confirmed.data.confirmationToken,
      expectedExtensionId: targetExtensionId || undefined,
      idempotencyKey,
    },
    {
      timeout: 300000,
      headers: {
        ...(managementTarget ? { "X-Amitia-Management-Target": managementTarget } : {}),
        "Idempotency-Key": idempotencyKey,
      },
    },
  );
  return response.data as PackageOperationResult;
}
export async function setGameCenterExtensionEnabled(extensionId: string, enabled: boolean) {
  const response = await apiClient.post(
    `/api/game-center/extensions/${enabled ? "enable" : "disable"}`,
    { extensionId },
  );
  return response.data;
}
export async function fetchPackageVersions(
  id: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.get(
    `/api/extensions/${encodeURIComponent(id)}/versions`,
    { params: { scopeType, scopeId } },
  );
  return response.data as PackageVersion[];
}
export async function comparePackageVersions(
  id: string,
  from: string,
  to: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.get(
    `/api/extensions/${encodeURIComponent(id)}/versions/compare`,
    { params: { from, to, scopeType, scopeId } },
  );
  return response.data as Record<string, unknown>;
}
export async function exportExtensionPackage(
  id: string,
  format: "amitiax" | "agentskills-zip",
  version: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.post(
    `/api/extensions/${encodeURIComponent(id)}/export`,
    { format, version, scopeType, scopeId },
  );
  return response.data as ExportedPackage;
}
export async function downloadExtensionPackage(
  id: string,
  exported: ExportedPackage,
) {
  const response = await apiClient.get(
    `/api/extensions/${encodeURIComponent(id)}/exports/${encodeURIComponent(exported.exportId)}`,
    { responseType: "blob" },
  );
  if (window.amitiaDesktop?.saveExtensionPackage) {
    const base64 = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result).split(",")[1] || "");
      reader.onerror = reject;
      reader.readAsDataURL(response.data);
    });
    await window.amitiaDesktop.saveExtensionPackage({
      suggestedName: exported.fileName,
      base64,
    });
    return;
  }
  const url = URL.createObjectURL(response.data);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = exported.fileName;
  anchor.click();
  URL.revokeObjectURL(url);
}
export async function rollbackExtensionPackage(
  id: string,
  version: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.post(
    `/api/extensions/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}/rollback`,
    { scopeType, scopeId },
  );
  return response.data as PackageOperationResult;
}
export async function fetchPackageDependencies(
  id: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.get(
    `/api/extensions/${encodeURIComponent(id)}/dependencies`,
    { params: { scopeType, scopeId } },
  );
  return response.data as {
    dependencies: PackageDependency[];
    dependents: PackageDependency[];
  };
}
export async function previewPackageUninstall(
  id: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.get(
    `/api/extensions/${encodeURIComponent(id)}/uninstall/preview`,
    { params: { scopeType, scopeId } },
  );
  return response.data as import("./types").PackageUninstallPreview;
}
export async function uninstallExtensionPackage(
  id: string,
  scopeType: string,
  scopeId: string,
) {
  const response = await apiClient.delete(
    `/api/extensions/${encodeURIComponent(id)}`,
    { params: { scopeType, scopeId } },
  );
  return response.data as PackageOperationResult;
}
export async function fetchPackageOperations() {
  const response = await apiClient.get("/api/extensions/package-operations");
  return response.data as PackageOperation[];
}
export async function fetchPackageSigners() {
  const response = await apiClient.get("/api/extensions/signers");
  return response.data as PackageSigner[];
}
export async function setPackageSignerTrust(
  fingerprint: string,
  trusted: boolean,
) {
  await apiClient.post(
    `/api/extensions/signers/${encodeURIComponent(fingerprint)}/${trusted ? "trust" : "untrust"}`,
  );
}

const taskRunPath = (taskRunId: string) =>
  `/api/extensions/tasks/${encodeURIComponent(taskRunId)}`;

export async function fetchTasks(filters: TaskListFilters = {}) {
  const response = await apiClient.get("/api/extensions/tasks", {
    params: {
      extensionId: filters.extensionId || undefined,
      status: filters.status || undefined,
      page: filters.page,
      pageSize: filters.pageSize,
    },
  });
  return response.data as TaskRunPage;
}

export async function fetchTask(taskRunId: string) {
  const response = await apiClient.get(taskRunPath(taskRunId));
  return response.data as TaskRun;
}

export async function cancelTask(taskRunId: string) {
  await apiClient.post(`${taskRunPath(taskRunId)}/cancel`);
}

export async function retryTask(taskRunId: string) {
  const response = await apiClient.post(`${taskRunPath(taskRunId)}/retry`);
  return response.data as TaskRun;
}

export async function recoverTask(taskRunId: string) {
  const response = await apiClient.post(`${taskRunPath(taskRunId)}/recover`);
  return response.data as TaskRun;
}

export async function fetchTaskProgress(taskRunId: string) {
  const response = await apiClient.get(`${taskRunPath(taskRunId)}/progress`);
  return response.data as TaskProgress;
}

export async function fetchTaskResult(taskRunId: string) {
  const response = await apiClient.get(`${taskRunPath(taskRunId)}/result`);
  return response.data as TaskResult;
}
