import { apiClient } from "@/composables/useApi"
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
} from "./types"

export async function resolveCharacterId() {
  const cached = localStorage.getItem("uai-default-char")
  if (cached) {
    try {
      const parsed = JSON.parse(cached)
      if (parsed?.id) return String(parsed.id)
    } catch {}
  }
  const response = await apiClient.get("/api/characters")
  const characters = response.data?.data || response.data
  if (!Array.isArray(characters)) return ""
  const selected = characters.find((item: any) => item.isDefault) || characters.find((item: any) => item.isActive) || characters.find((item: any) => item.status !== "disabled")
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

export async function setSkillEnabled(id: string, enabled: boolean) {
  await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`)
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

export async function updateConfig(id: string, config: unknown) {
  await apiClient.put(`/api/extensions/skills/${encodeURIComponent(id)}/config`, config)
}

export async function resetConfig(id: string) {
  await apiClient.post(`/api/extensions/skills/${encodeURIComponent(id)}/config/reset`)
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
