import { get } from "@/composables/request";
import type { RuntimeDebugSnapshot } from "@/types";

export function fetchDecisionSnapshotApi() {
  return get<RuntimeDebugSnapshot>("/api/runtime/debug/snapshot");
}
