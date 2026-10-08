import { apiClient } from "./useApi";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

export interface OwnedMemoryManagementResource { id: string; kind: string; ownerId: string; roleId: string; revision: number; deleted?: boolean; body: Record<string, any>; executionScope: OwnedExecutionScope }
const unwrap = (response: any) => response?.data?.data ?? response?.data;
export const ownedMemoryAuthority = (scope: OwnedExecutionScope) => JSON.stringify(Object.entries(scope).filter(([key]) => !["requestId", "turnId", "executionId"].includes(key)).sort(([left], [right]) => left.localeCompare(right)));

export function useOwnedMemoryManagement() {
  function resources(result: any, expected: OwnedExecutionScope): OwnedMemoryManagementResource[] {
    if (!expected.authorizationRealm || !result?.executionScope || ownedMemoryAuthority(result.executionScope) !== ownedMemoryAuthority(expected) || !Array.isArray(result.resources) || result.resources.length > 512) throw new Error("记忆管理的数据归属已变化，请重新加载");
    return result.resources.map((resource: any) => {
      if (!resource.id || resource.ownerId !== expected.resourceOwnerId || resource.roleId !== expected.roleId || !Number.isSafeInteger(resource.revision) || resource.revision < 1 || !resource.body || typeof resource.body !== "object") throw new Error("记忆管理记录所属设备或版本无效");
      return { ...resource, executionScope: { ...expected } };
    });
  }
  async function read(expected: OwnedExecutionScope, query: Record<string, unknown> = {}, candidates = false) {
    const result = unwrap(await apiClient.get(`/api/device-mesh/v1/business/${candidates ? "memory-candidates" : "memories"}`, { params: { ...query, characterId: expected.roleId, targetDeviceId: expected.targetDeviceId } }));
    const rows = resources(result, expected);
    if (rows.some((row) => candidates ? row.kind !== "checkpoint" || !row.id.startsWith("memory-candidate/") || row.body.type !== "owned-memory-candidate" || row.body.state !== "pending" : row.kind !== "memory")) throw new Error("记忆管理返回了其他资源种类");
    return { resources: rows, nextCursor: result.nextCursor || "", scores: result.scores || {} };
  }
  async function write(expected: OwnedExecutionScope, requestId: string, input: Record<string, unknown>, candidates = false) {
    const result = unwrap(await apiClient.post(`/api/device-mesh/v1/business/${candidates ? "memory-candidates" : "memories"}/manage`, { ...input, requestId, characterId: expected.roleId, targetDeviceId: expected.targetDeviceId, expectedExecutionScope: expected }));
    const rows = resources(result, expected);
    const generated = candidates && input.action === "generate";
    const proof = `checkpoint/${candidates ? "memory-candidate-operation" : "memory-management"}/${requestId}`;
    if (result.saved !== true || result.executionScope.requestId !== requestId || result.acknowledgement?.requestId !== `${requestId}${generated ? "|candidate-result" : ""}` || result.acknowledgement?.ownerId !== expected.resourceOwnerId || result.acknowledgement?.versions?.[proof] !== (generated ? 2 : 1) || rows.some((row) => result.acknowledgement?.versions?.[`${row.kind}/${row.id}`] !== row.revision)) throw new Error("记忆操作尚未获得数据所有者保存确认");
    if (!candidates && (!rows.some((row) => row.kind === "memory") || !rows.some((row) => row.kind === "fact")) || candidates && !generated && !rows.some((row) => row.kind === "checkpoint" && row.id === input.id && row.revision === Number(input.expectedRevision) + 1) || generated && rows.some((row) => row.kind !== "checkpoint" || !row.id.startsWith(`memory-candidate/${requestId}/`) || row.body.type !== "owned-memory-candidate" || row.body.state !== "pending")) throw new Error("记忆操作缺少原记录的保存版本");
    if (candidates && input.action === "accept" && (!rows.some((row) => row.kind === "memory") || !rows.some((row) => ["fact", "profile", "episodic"].includes(row.kind)) || !rows.some((row) => row.kind === "checkpoint" && row.id === input.id && row.body.state === "accepted"))) throw new Error("候选记忆尚未确认写入正式记忆");
    return rows;
  }
  function conflicts(error: any, expected: OwnedExecutionScope) {
    const payload = error?.response?.data;
    const rows = payload?.conflicts || payload?.data?.conflicts;
    if (!Array.isArray(rows) || rows.length < 1 || rows.length > 32) return [];
    return resources({ executionScope: expected, resources: rows }, expected).filter((row) => row.kind === "memory" && !row.deleted);
  }
  return { read, write, conflicts };
}
