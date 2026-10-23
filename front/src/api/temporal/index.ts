import { apiClient } from "@/composables/useApi";
import type {
  RelationshipTimeContext,
  RelationshipTimeDiagnostics,
  RelationshipTimeSettings,
  ReunionEpisode,
  TemporalAnchor,
  TemporalDiagnostics,
  TemporalProfile,
  TemporalSnapshot,
} from "./types";

export async function getTemporalProfile() {
  return (await apiClient.get<TemporalProfile>("/api/temporal/profile")).data;
}
export async function updateTemporalProfile(profile: TemporalProfile) {
  return (
    await apiClient.put<TemporalProfile>("/api/temporal/profile", profile)
  ).data;
}
export async function getCharacterTemporalProfile(characterId: string) {
  return (
    await apiClient.get<TemporalProfile>(
      `/api/temporal/characters/${encodeURIComponent(characterId)}/profile`,
    )
  ).data;
}
export async function updateCharacterTemporalProfile(
  characterId: string,
  profile: TemporalProfile,
) {
  return (
    await apiClient.put<TemporalProfile>(
      `/api/temporal/characters/${encodeURIComponent(characterId)}/profile`,
      profile,
    )
  ).data;
}
export async function getTemporalSnapshot(characterId = "", channel = "web") {
  return (
    await apiClient.get<TemporalSnapshot>("/api/temporal/snapshot", {
      params: { characterId, channel },
    })
  ).data;
}
export async function getTemporalDiagnostics(
  characterId = "",
  channel = "web",
) {
  return (
    await apiClient.get<TemporalDiagnostics>("/api/temporal/diagnostics", {
      params: { characterId, channel },
    })
  ).data;
}
export async function listTemporalAnchors(characterId = "", status = "") {
  return (
    await apiClient.get<TemporalAnchor[]>("/api/temporal/anchors", {
      params: { characterId, status },
    })
  ).data;
}
export async function createTemporalAnchor(anchor: Partial<TemporalAnchor>) {
  return (await apiClient.post<TemporalAnchor>("/api/temporal/anchors", anchor))
    .data;
}
export async function updateTemporalAnchor(
  id: string,
  anchor: Partial<TemporalAnchor>,
) {
  return (
    await apiClient.put<TemporalAnchor>(
      `/api/temporal/anchors/${encodeURIComponent(id)}`,
      anchor,
    )
  ).data;
}
export async function deleteTemporalAnchor(id: string, characterId = "") {
  await apiClient.delete(`/api/temporal/anchors/${encodeURIComponent(id)}`, {
    params: { characterId },
  });
}
export async function confirmTemporalAnchor(id: string, characterId = "") {
  return (
    await apiClient.post<TemporalAnchor>(
      `/api/temporal/anchors/${encodeURIComponent(id)}/confirm`,
      undefined,
      { params: { characterId } },
    )
  ).data;
}
export async function suggestTemporalTimezone(timezone: string) {
  return (
    await apiClient.post<TemporalProfile>("/api/temporal/timezone-suggestion", {
      timezone,
    })
  ).data;
}
export async function acceptTemporalTimezoneSuggestion() {
  return (
    await apiClient.post<TemporalProfile>(
      "/api/temporal/timezone-suggestion/accept",
    )
  ).data;
}
export async function rejectTemporalTimezoneSuggestion() {
  return (
    await apiClient.post<TemporalProfile>(
      "/api/temporal/timezone-suggestion/reject",
    )
  ).data;
}
interface RelationshipTimeView {
  ownerId: string;
  roleId: string;
  executionScope: Record<string, unknown>;
  settings: RelationshipTimeSettings;
  state: RelationshipTimeContext;
  episodes: ReunionEpisode[];
  episode?: ReunionEpisode;
}

const relationshipAuthority = Symbol("relationshipTimeAuthority");

async function relationshipView(characterId: string, episodeId = "") {
  const result = (await apiClient.get<RelationshipTimeView>("/api/device-mesh/v1/business/relationship-time", { params: { characterId, ...(episodeId ? { episodeId } : {}) } })).data;
  if (!result?.ownerId || result.roleId !== characterId || result.executionScope?.resourceOwnerId !== result.ownerId || result.executionScope?.roleId !== characterId) throw new Error("关系时间数据归属无法确认");
  return result;
}

export async function getRelationshipTimeSettings(characterId: string) {
  const result = await relationshipView(characterId);
  Object.assign(result.settings, { [relationshipAuthority]: result.executionScope });
  return result.settings;
}

export async function updateRelationshipTimeSettings(characterId: string, settings: RelationshipTimeSettings) {
  const expectedExecutionScope = (settings as RelationshipTimeSettings & { [relationshipAuthority]?: Record<string, unknown> })[relationshipAuthority];
  if (!expectedExecutionScope) throw new Error("请重新加载关系时间设置");
  const result = (await apiClient.put<RelationshipTimeView>("/api/device-mesh/v1/business/relationship-time", { settings, expectedExecutionScope }, { params: { characterId } })).data;
  if (result?.roleId !== characterId || result.ownerId !== expectedExecutionScope.resourceOwnerId) throw new Error("关系时间保存归属无法确认");
  Object.assign(result.settings, { [relationshipAuthority]: result.executionScope });
  return result.settings;
}

export async function getRelationshipTimeState(characterId: string) {
  return (await relationshipView(characterId)).state;
}

export async function listReunionEpisodes(characterId: string) {
  return (await relationshipView(characterId)).episodes;
}

export async function getReunionEpisode(characterId: string, episodeId: string) {
  const episode = (await relationshipView(characterId, episodeId)).episode;
  if (!episode) throw new Error("重逢记录不存在或不属于当前所有者");
  return episode;
}

export type * from "./types";
