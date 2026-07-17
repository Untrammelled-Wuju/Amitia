import { apiClient } from "@/composables/useApi"
import type { Character } from "@/types"
import type {
  CapabilityDefinition,
  PermissionGrant,
  RunPage,
  SkillDetail,
  SkillResult,
  SkillTrigger,
  SkillView,
  PluginDetail,
  PluginEventPage,
  PluginHealth,
  PluginPage,
  PluginSchedule,
  PluginState,
  SurfaceDocument,
  WorkshopPage,
  WorkshopRevision,
  WorkshopSession,
  WorkshopSessionDetail,
  WorkshopTestReport,
  WorkshopValidation,
  AgentSkillDetail,
  AgentSkillDefinition,
  AgentSkillPage,
  AgentSkillPreview,
  AgentSkillScope,
  PackageImportPreview,
  PackageOperation,
  PackageOperationResult,
  PackageSigner,
  PackageVersion,
  ExportedPackage,
  PackageDependency,
} from "./types"

export async function fetchCharacterOptions() {
  const response = await apiClient.get("/api/characters")
  const characters = response.data?.data || response.data
  return Array.isArray(characters) ? characters as Character[] : []
}

export async function resolveCharacterId(availableCharacters?: Character[]) {
  const characters = availableCharacters || await fetchCharacterOptions()
  const cached = localStorage.getItem("uai-default-char")
  if (cached) {
    try {
      const parsed = JSON.parse(cached)
      if (parsed?.id && characters.some(item => String(item.id) === String(parsed.id))) return String(parsed.id)
    } catch {}
  }
  const selected = characters.find(item => item.isDefault) || characters.find(item => item.isActive) || characters.find(item => item.status !== "disabled")
  return selected?.id ? String(selected.id) : ""
}

export async function fetchSkills(characterId: string, filters: { enabled?: boolean; trigger?: SkillTrigger | ""; source?: string }) {
  const response = await apiClient.get("/api/extensions/skills", { params: { characterId, ...filters } })
  const skills = Array.isArray(response.data) ? response.data : []
  return skills.map((skill: SkillView) => normalizeSkill(skill))
}

export async function fetchSkill(characterId: string, id: string) {
  const response = await apiClient.get(`/api/extensions/skills/${encodeURIComponent(id)}`, { params: { characterId } })
  return normalizeSkill(response.data as SkillDetail)
}

function normalizeSkill<T extends SkillView>(skill: T): T {
  return {
    ...skill,
    capabilities: Array.isArray(skill.capabilities) ? skill.capabilities : [],
    triggers: Array.isArray(skill.triggers) ? skill.triggers : [],
  }
}

export async function setSkillEnabled(id: string, characterId: string, enabled: boolean) {
	await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`, null, { params: { characterId } })
}

export async function fetchCapabilities() {
  const response = await apiClient.get("/api/extensions/capabilities")
  return response.data as CapabilityDefinition[]
}

export async function updatePermissions(id: string, characterId: string, grants: PermissionGrant[]) {
  await apiClient.put(`/api/extensions/skills/${encodeURIComponent(id)}/permissions`, {
    characterId,
    grants: grants.map(({ capability, decision, scopeType, scopeId, expiresAt }) => ({ capability, decision, scopeType, scopeId, expiresAt })),
  })
}

export async function updateConfig(id: string, characterId: string, config: unknown) {
	await apiClient.put(`/api/extensions/skills/${encodeURIComponent(id)}/config`, config, { params: { characterId } })
}

export async function resetConfig(id: string, characterId: string) {
	await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/config/reset`, null, { params: { characterId } })
}

export async function executeSkill(id: string, payload: { characterId: string; conversationId?: string; channel?: string; sessionId?: string; idempotencyKey?: string; input: unknown }) {
  const response = await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/execute`, payload)
  return response.data as SkillResult
}

export async function fetchRuns(characterId: string, filters: Record<string, unknown>) {
  const response = await apiClient.get("/api/extensions/runs", { params: { characterId, ...filters } })
  return response.data as RunPage
}

export async function fetchRun(characterId: string, runId: string) {
  const response = await apiClient.get(`/api/extensions/runs/${encodeURIComponent(runId)}`, { params: { characterId } })
  return response.data
}

const pluginPath = (id: string) => `/api/extensions/plugins/${encodeURIComponent(id)}`

export async function fetchPlugins(page = 1, pageSize = 20) {
  const response = await apiClient.get("/api/extensions/plugins", { params: { page, pageSize } })
  return response.data as PluginPage
}

export async function fetchPlugin(characterId: string, id: string) {
  const response = await apiClient.get(pluginPath(id), { params: { characterId } })
  return response.data as PluginDetail
}

export async function setPluginEnabled(id: string, enabled: boolean, characterId: string) {
  await apiClient.post(`${pluginPath(id)}/${enabled ? "enable" : "disable"}`, null, { params: { characterId } })
}

export async function reloadPlugin(id: string) { await apiClient.post(`${pluginPath(id)}/reload`) }

export async function updatePluginConfig(id: string, characterId: string, config: unknown) {
  await apiClient.put(`${pluginPath(id)}/config`, config, { params: { characterId } })
}

export async function resetPluginConfig(id: string, characterId: string) {
  await apiClient.post(`${pluginPath(id)}/config/reset`, null, { params: { characterId } })
}

export async function updatePluginPermissions(id: string, characterId: string, grants: PermissionGrant[]) {
  await apiClient.put(`${pluginPath(id)}/permissions`, { characterId, grants: grants.map(({ capability, decision, scopeType, scopeId, expiresAt }) => ({ capability, decision, scopeType, scopeId, expiresAt })) })
}

export async function fetchPluginHealth(id: string) {
  const response = await apiClient.get(`${pluginPath(id)}/health`)
  return response.data as PluginHealth
}

export async function resetPluginCircuit(id: string) { await apiClient.post(`${pluginPath(id)}/circuit/reset`) }

export async function fetchPluginState(id: string, characterId: string) {
  const response = await apiClient.get(`${pluginPath(id)}/state`, { params: { characterId } })
  return response.data as PluginState[]
}

export async function fetchPluginSurface(id: string) {
  const response = await apiClient.get(`${pluginPath(id)}/surface`)
  return response.data as SurfaceDocument
}

export async function fetchPluginSchedules(id: string, characterId: string) {
  const response = await apiClient.get(`${pluginPath(id)}/schedules`, { params: { characterId } })
  return response.data as PluginSchedule[]
}

export async function setPluginScheduleEnabled(id: string, scheduleId: string, enabled: boolean, characterId: string) {
  await apiClient.post(`${pluginPath(id)}/schedules/${encodeURIComponent(scheduleId)}/${enabled ? "resume" : "pause"}`, null, { params: { characterId } })
}

export async function fetchPluginEvents(id: string, characterId: string, status = "", page = 1, pageSize = 20) {
  const response = await apiClient.get(`${pluginPath(id)}/events`, { params: { characterId, status, page, pageSize } })
  return response.data as PluginEventPage
}

export async function retryPluginEvent(id: string, eventId: string, characterId: string) { await apiClient.post(`${pluginPath(id)}/events/${encodeURIComponent(eventId)}/retry`, null, { params: { characterId } }) }

export async function executePluginAction(id: string, actionId: string, characterId: string, input: unknown) {
  const response = await apiClient.post(`${pluginPath(id)}/surface/actions/${encodeURIComponent(actionId)}`, { characterId, input })
  return response.data as SkillResult
}

const workshopPath = (id = "") => `/api/extensions/workshop/sessions${id ? `/${encodeURIComponent(id)}` : ""}`
export async function fetchWorkshopSessions(characterId: string, page = 1, pageSize = 20) { const response = await apiClient.get(workshopPath(), { params: { characterId, page, pageSize } }); return response.data as WorkshopPage }
export async function createWorkshopSession(requirement: string, characterId: string) { const response = await apiClient.post(workshopPath(), { requirement, characterId }, { params: { characterId } }); return response.data as WorkshopSession }
export async function fetchWorkshopSession(id: string, characterId: string) { const response = await apiClient.get(workshopPath(id), { params: { characterId } }); return response.data as WorkshopSessionDetail }
export async function archiveWorkshopSession(id: string, characterId: string) { await apiClient.post(`${workshopPath(id)}/archive`, null, { params: { characterId } }) }
export async function generateWorkshopDraft(id: string, characterId: string, payload: { requirement?: string; draft?: unknown } = {}) { const response = await apiClient.post(`${workshopPath(id)}/generate`, payload, { params: { characterId } }); return response.data as WorkshopRevision }
export async function validateWorkshopDraft(id: string, revision: number, characterId: string) { const response = await apiClient.post(`${workshopPath(id)}/revisions/${revision}/validate`, null, { params: { characterId } }); return response.data as WorkshopValidation }
export async function confirmWorkshopPermissions(id: string, revision: number, characterId: string, payload: { workflowChecksum: string; capabilities: string[]; confirmedHighRisk: string[]; production: boolean }) { await apiClient.post(`${workshopPath(id)}/revisions/${revision}/permissions/confirm`, payload, { params: { characterId } }) }
export async function testWorkshopDraft(id: string, revision: number, characterId: string, payload: { mode: string; testCases?: unknown[]; controlledLiveConfirmed?: boolean }) { const response = await apiClient.post(`${workshopPath(id)}/revisions/${revision}/test`, payload, { params: { characterId } }); return response.data as WorkshopTestReport }
export async function installWorkshopDraft(id: string, revision: number, characterId: string) { const response = await apiClient.post(`${workshopPath(id)}/revisions/${revision}/install`, null, { params: { characterId } }); return response.data as { sessionId: string; skillId: string; version: string; artifactId: string; enabled: boolean } }
export async function forkWorkflowSkill(id: string, characterId: string) { const response = await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/workshop/fork`, null, { params: { characterId } }); return response.data as WorkshopSessionDetail }
export async function rollbackWorkflowSkill(id: string, version: string, characterId: string) { const response = await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}/rollback`, null, { params: { characterId } }); return response.data as { skillId: string; version: string; artifactId: string; enabled: boolean } }
export async function generateWorkshopInstruction(requirement: string, characterId: string) { const response = await apiClient.post("/api/extensions/workshop/instructions/generate", { requirement }, { params: { characterId } }); return response.data as AgentSkillPreview }

const agentSkillPath = (id = "") => `/api/extensions/agent-skills${id ? `/${encodeURIComponent(id)}` : ""}`
export async function previewAgentSkillZIP(file: File) { const data = new FormData(); data.append("source", "zip"); data.append("file", file); const response = await apiClient.post(`${agentSkillPath()}/import/preview`, data); return response.data as AgentSkillPreview }
export async function previewAgentSkillDirectory(rootName: string, files: Array<{ path: string; file: File }>) { const data = new FormData(); data.append("source", "directory"); data.append("rootName", rootName); data.append("paths", JSON.stringify(files.map(item => item.path))); files.forEach(item => data.append("files", item.file, item.file.name)); const response = await apiClient.post(`${agentSkillPath()}/import/preview`, data); return response.data as AgentSkillPreview }
export async function fetchAgentSkills(characterId: string, params: Record<string, unknown> = {}) { const response = await apiClient.get(agentSkillPath(), { params: { characterId, ...params } }); return response.data as AgentSkillPage }
export async function fetchAgentSkill(id: string, characterId: string) { const response = await apiClient.get(agentSkillPath(id), { params: { characterId } }); return response.data as AgentSkillDetail }
export async function installAgentSkill(previewId: string, scope: AgentSkillScope, characterId: string) { const response = await apiClient.post(`${agentSkillPath()}/import/install`, { previewId, scope, characterId, enable: false }); return response.data as AgentSkillDefinition }
export async function setAgentSkillEnabled(id: string, enabled: boolean, characterId: string) { await apiClient.post(`${agentSkillPath(id)}/${enabled ? "enable" : "disable"}`, null, { params: { characterId } }) }
export async function removeAgentSkill(id: string, characterId: string) { await apiClient.delete(agentSkillPath(id), { params: { characterId } }) }

export async function previewExtensionPackage(file: File, scopeType: "global" | "character", scopeId: string, extensionId = "", onProgress?: (percent: number) => void) {
  const data = new FormData()
  data.append("file", file)
  data.append("scopeType", scopeType)
  data.append("scopeId", scopeId)
  const url = extensionId ? `/api/extensions/${encodeURIComponent(extensionId)}/upgrade/preview` : "/api/extensions/packages/import/preview"
  const response = await apiClient.post(url, data, { onUploadProgress: event => onProgress?.(event.total ? Math.round(event.loaded * 100 / event.total) : 0) })
  return response.data as PackageImportPreview
}
export async function previewExtensionDirectory(rootName: string, files: Array<{ path: string; base64: string }>, scopeType: "global" | "character", scopeId: string, onProgress?: (percent: number) => void) {
  const response = await apiClient.post("/api/extensions/packages/import/preview", { rootName, files, scopeType, scopeId }, { onUploadProgress: event => onProgress?.(event.total ? Math.round(event.loaded * 100 / event.total) : 0) })
  return response.data as PackageImportPreview
}
export async function installExtensionPackage(preview: PackageImportPreview, confirmations: { unsigned: boolean; scripts: boolean; capabilities: string[]; versionChange: boolean; signerChange: boolean; configMigration: boolean }, upgradeId = "") {
  const payload = { sessionId: preview.sessionId, scopeType: preview.scopeType, scopeId: preview.scopeId, confirmUnsigned: confirmations.unsigned, confirmScripts: confirmations.scripts, confirmedCapabilities: confirmations.capabilities, confirmVersionChange: confirmations.versionChange, confirmSignerChange: confirmations.signerChange, confirmConfigMigration: confirmations.configMigration }
  const url = upgradeId ? `/api/extensions/${encodeURIComponent(upgradeId)}/upgrade` : "/api/extensions/packages/import/install"
  const response = await apiClient.post(url, payload)
  return response.data as PackageOperationResult
}
export async function fetchPackageVersions(id: string, scopeType: string, scopeId: string) { const response = await apiClient.get(`/api/extensions/${encodeURIComponent(id)}/versions`, { params: { scopeType, scopeId } }); return response.data as PackageVersion[] }
export async function comparePackageVersions(id: string, from: string, to: string, scopeType: string, scopeId: string) { const response = await apiClient.get(`/api/extensions/${encodeURIComponent(id)}/versions/compare`, { params: { from, to, scopeType, scopeId } }); return response.data as Record<string, unknown> }
export async function exportExtensionPackage(id: string, format: "amitiax" | "agentskills-zip", version: string, scopeType: string, scopeId: string) { const response = await apiClient.post(`/api/extensions/${encodeURIComponent(id)}/export`, { format, version, scopeType, scopeId }); return response.data as ExportedPackage }
export async function downloadExtensionPackage(id: string, exported: ExportedPackage) { const response = await apiClient.get(`/api/extensions/${encodeURIComponent(id)}/exports/${encodeURIComponent(exported.exportId)}`, { responseType: "blob" }); if (window.amitiaDesktop?.saveExtensionPackage) { const base64 = await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result).split(",")[1] || ""); reader.onerror = reject; reader.readAsDataURL(response.data) }); await window.amitiaDesktop.saveExtensionPackage({ suggestedName: exported.fileName, base64 }); return } const url = URL.createObjectURL(response.data); const anchor = document.createElement("a"); anchor.href = url; anchor.download = exported.fileName; anchor.click(); URL.revokeObjectURL(url) }
export async function rollbackExtensionPackage(id: string, version: string, scopeType: string, scopeId: string) { const response = await apiClient.post(`/api/extensions/${encodeURIComponent(id)}/versions/${encodeURIComponent(version)}/rollback`, { scopeType, scopeId }); return response.data as PackageOperationResult }
export async function fetchPackageDependencies(id: string, scopeType: string, scopeId: string) { const response = await apiClient.get(`/api/extensions/${encodeURIComponent(id)}/dependencies`, { params: { scopeType, scopeId } }); return response.data as { dependencies: PackageDependency[]; dependents: PackageDependency[] } }
export async function previewPackageUninstall(id: string, scopeType: string, scopeId: string) { const response = await apiClient.get(`/api/extensions/${encodeURIComponent(id)}/uninstall/preview`, { params: { scopeType, scopeId } }); return response.data as import("./types").PackageUninstallPreview }
export async function uninstallExtensionPackage(id: string, scopeType: string, scopeId: string) { const response = await apiClient.delete(`/api/extensions/${encodeURIComponent(id)}`, { params: { scopeType, scopeId } }); return response.data as PackageOperationResult }
export async function fetchPackageOperations() { const response = await apiClient.get("/api/extensions/package-operations"); return response.data as PackageOperation[] }
export async function fetchPackageSigners() { const response = await apiClient.get("/api/extensions/signers"); return response.data as PackageSigner[] }
export async function setPackageSignerTrust(fingerprint: string, trusted: boolean) { await apiClient.post(`/api/extensions/signers/${encodeURIComponent(fingerprint)}/${trusted ? "trust" : "untrust"}`) }
