import { apiClient } from "./useApi";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

export async function transferMemoryScope(record: { kind: string; id: string; revision: number; ownerId: string; executionScope: OwnedExecutionScope }, targetScope: "global" | "character", targetRoleId: string) {
  const requestId = crypto.randomUUID();
  const scope = record.executionScope;
  const kind = record.kind;
  const response = await apiClient.post("/api/device-mesh/v1/business/memory-scope/transfer", {
    requestId, characterId: scope.roleId, targetDeviceId: scope.targetDeviceId, expectedExecutionScope: scope,
    kind, id: record.id, expectedRevision: record.revision, targetScope, targetRoleId: targetScope === "character" ? targetRoleId : undefined,
  });
  const ack = response.data?.data ?? response.data;
  if (ack.ownerId !== record.ownerId || ack.requestId !== requestId || ack.versions?.[`${kind}/${record.id}`] !== record.revision + 1) throw new Error("数据所有者尚未确认分区迁移，请重新加载核对");
  window.dispatchEvent(new CustomEvent("amitia:memory-scope-changed", { detail: { ownerId: record.ownerId } }));
}
