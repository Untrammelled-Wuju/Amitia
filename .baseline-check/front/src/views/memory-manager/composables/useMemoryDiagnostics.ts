import { ref } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../../composables/useApi";

export function useMemoryDiagnostics() {
  const { get, post } = useApi();
  const vectorStatus = ref<any>(null);
  const pipelineStatus = ref<any>(null);
  const rebuilding = ref(false);
  const retrievalStats = ref({ totalCount: 0 });
  const retrievalLogs = ref<any[]>([]);
  const halflifeEpisodic = ref(30);
  const halflifeProfile = ref(90);
  const halflifeFact = ref(180);
  const halflifeWorldbook = ref(365);

  async function loadVectorStatus() {
    try {
      vectorStatus.value = await get<any>("/api/memories/vector-status");
    } catch {}
  }

  async function fetchPipelineStatus() {
    try {
      pipelineStatus.value = await get<any>("/api/memory/pipeline/status");
    } catch {}
  }

  async function rebuildIndex() {
    rebuilding.value = true;
    try {
      const result = await post<any>("/api/memories/rebuild-embeddings", {});
      vectorStatus.value = result;
      ElMessage.success(
        "索引重建完成：" +
          (result.embedded ?? result.totalEmbedded ?? 0) +
          " 条记忆已处理",
      );
      await loadVectorStatus();
    } catch (err: any) {
      ElMessage.error(err.message || "Rebuild failed");
    }
    rebuilding.value = false;
  }

  async function loadRetrievalStats() {
    try {
      const r: any = await get("/api/memory/retrieval/stats");
      retrievalStats.value = { totalCount: r?.totalCount || 0 };
      retrievalLogs.value = r?.recentLogs || [];
    } catch {}
  }

  function fmtDate(d: string) {
    if (!d) return "";
    try {
      return new Date(d).toLocaleString("zh-CN");
    } catch {
      return d;
    }
  }

  return {
    vectorStatus,
    pipelineStatus,
    rebuilding,
    retrievalStats,
    retrievalLogs,
    halflifeEpisodic,
    halflifeProfile,
    halflifeFact,
    halflifeWorldbook,
    loadVectorStatus,
    fetchPipelineStatus,
    rebuildIndex,
    loadRetrievalStats,
    fmtDate,
  };
}
