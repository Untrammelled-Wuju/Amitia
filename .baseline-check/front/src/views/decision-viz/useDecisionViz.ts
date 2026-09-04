import { ref, reactive } from "vue";
import { fetchDecisionSnapshotApi } from "./api";
import type { RuntimeDebugSnapshot } from "@/types";

export function useDecisionViz() {
  const loading = ref(false);
  const loadError = ref("");
  const snapshot = reactive<RuntimeDebugSnapshot>({
    meta: { generatedAt: "", degraded: false },
    summary: { activeInteractions: 0, queuedTasks: 0, reconciliationIssues: 0 },
    interactions: [],
    budgets: [],
    queues: [],
    deliveries: [],
    tools: [],
    circuits: [],
    reconciliation: [],
    behaviorPlan: undefined,
    expressionPlan: undefined,
  });
  async function load() {
    loading.value = true;
    loadError.value = "";
    try {
      const data = await fetchDecisionSnapshotApi();
      Object.assign(snapshot, data);
    } catch (err: any) {
      loadError.value = err?.message || "决策快照接口暂不可用";
    } finally {
      loading.value = false;
    }
  }
  return { loading, loadError, snapshot, load };
}
