import { apiClient } from "@/composables/useApi"
import type { TemporalAnchor, TemporalDiagnostics, TemporalProfile, TemporalSnapshot } from "./types"

export async function getTemporalProfile() { return (await apiClient.get<TemporalProfile>("/api/temporal/profile")).data }
export async function updateTemporalProfile(profile: TemporalProfile) { return (await apiClient.put<TemporalProfile>("/api/temporal/profile",profile)).data }
export async function getCharacterTemporalProfile(characterId:string) { return (await apiClient.get<TemporalProfile>(`/api/temporal/characters/${encodeURIComponent(characterId)}/profile`)).data }
export async function updateCharacterTemporalProfile(characterId:string,profile:TemporalProfile) { return (await apiClient.put<TemporalProfile>(`/api/temporal/characters/${encodeURIComponent(characterId)}/profile`,profile)).data }
export async function getTemporalSnapshot(characterId="",channel="web") { return (await apiClient.get<TemporalSnapshot>("/api/temporal/snapshot",{params:{characterId,channel}})).data }
export async function getTemporalDiagnostics(characterId="",channel="web") { return (await apiClient.get<TemporalDiagnostics>("/api/temporal/diagnostics",{params:{characterId,channel}})).data }
export async function listTemporalAnchors(characterId="",status="") { return (await apiClient.get<TemporalAnchor[]>("/api/temporal/anchors",{params:{characterId,status}})).data }
export async function createTemporalAnchor(anchor:Partial<TemporalAnchor>) { return (await apiClient.post<TemporalAnchor>("/api/temporal/anchors",anchor)).data }
export async function updateTemporalAnchor(id:string,anchor:Partial<TemporalAnchor>) { return (await apiClient.put<TemporalAnchor>(`/api/temporal/anchors/${encodeURIComponent(id)}`,anchor)).data }
export async function deleteTemporalAnchor(id:string,characterId="") { await apiClient.delete(`/api/temporal/anchors/${encodeURIComponent(id)}`,{params:{characterId}}) }
export async function confirmTemporalAnchor(id:string,characterId="") { return (await apiClient.post<TemporalAnchor>(`/api/temporal/anchors/${encodeURIComponent(id)}/confirm`,undefined,{params:{characterId}})).data }
export async function suggestTemporalTimezone(timezone:string) { return (await apiClient.post<TemporalProfile>("/api/temporal/timezone-suggestion",{timezone})).data }
export async function acceptTemporalTimezoneSuggestion() { return (await apiClient.post<TemporalProfile>("/api/temporal/timezone-suggestion/accept")).data }
export async function rejectTemporalTimezoneSuggestion() { return (await apiClient.post<TemporalProfile>("/api/temporal/timezone-suggestion/reject")).data }

export type * from "./types"
