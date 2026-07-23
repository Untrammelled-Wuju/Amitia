import { reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../../composables/useApi";

export function useMemoryCandidates(
  fetchList?: () => Promise<void> | void,
  characterId?: () => string | null | undefined,
) {
  const { get, post, put, del } = useApi();
  const candidates = ref<any[]>([]);
  const showCandidates = ref(false);
  const conversationList = ref<any[]>([]);
  const showGenerateDialog = ref(false);
  const generating = ref(false);
  const generateConvId = ref("");
  const editCandidateVisible = ref(false);
  const editForm = reactive({
    key: "",
    value: "",
    content: "",
    memoryType: "custom",
    importance: 5,
    candidateId: "",
    scope: "character",
  });
  const conflictVisible = ref(false);
  const conflictNewType = ref("");
  const conflictNewContent = ref("");
  const conflictList = ref<any[]>([]);
  const resolveAction = ref("");

  async function loadCandidates() {
    try {
      const r: any = await get("/api/memory-candidates");
      candidates.value = r?.candidates || [];
    } catch {}
  }

  async function confirmCandidate(c: any) {
    try {
      await post("/api/memory-candidates/" + c.id + "/accept", {});
      ElMessage.success("已保存");
    } catch {
      await post("/api/memories", {
        key: c.key,
        value: c.value,
        memoryType: c.memoryType || "custom",
        importance: c.importance || 5,
        source: "manual",
        scope: "character",
        scopeType: "user_character",
        characterId: characterId?.() || "",
        sensitivity: "normal",
        allowContextUse: true,
        allowProactiveMention: false,
        requiresConfirmation: false,
      });
      ElMessage.success("已保存");
    }
    candidates.value = candidates.value.filter((x) => x.id !== c.id);
    await fetchList?.();
  }

  async function deleteCandidateItem(c: any) {
    try {
      await del("/api/memory-candidates/" + c.id);
      candidates.value = candidates.value.filter((x) => x.id !== c.id);
      ElMessage.success("已删除");
    } catch {
      ElMessage.error("删除失败");
    }
  }

  function toggleCandidates() {
    showCandidates.value = !showCandidates.value;
  }

  async function loadConversations() {
    try {
      const result: any = await get("/api/chats/conversations", {
        pageSize: 100,
      });
      conversationList.value = result?.items || result?.data || [];
    } catch {}
  }
  async function generateCandidates() {
    if (!generateConvId.value) {
      ElMessage.warning("请选择会话");
      return;
    }
    generating.value = true;
    try {
      const result: any = await post("/api/memory-candidates/generate", {
        conversationId: generateConvId.value,
      });
      candidates.value = result?.candidates || [];
      if (candidates.value.length > 0) {
        showGenerateDialog.value = false;
        showCandidates.value = true;
        ElMessage.success("已提取 " + candidates.value.length + " 条候选记忆");
      } else ElMessage.info("未提取到候选记忆");
    } catch (error: any) {
      ElMessage.error(error?.message || "提取失败");
    }
    generating.value = false;
  }
  function editCandidate(candidate: any) {
    Object.assign(editForm, {
      key: candidate.key,
      value: candidate.value,
      content: candidate.value,
      memoryType: candidate.memoryType,
      importance: candidate.importance,
      candidateId: candidate.id,
    });
    editCandidateVisible.value = true;
  }
  async function saveEditCandidate() {
    if (!editForm.candidateId) return;
    try {
      await put("/api/memory-candidates/" + editForm.candidateId, {
        key: editForm.key,
        value: editForm.content,
        memoryType: editForm.memoryType,
        importance: editForm.importance,
      });
      ElMessage.success("已更新");
      editCandidateVisible.value = false;
      await loadCandidates();
    } catch (error: any) {
      ElMessage.error(error?.message || "更新失败");
    }
  }
  async function doResolveConflict() {
    if (!resolveAction.value) {
      ElMessage.warning("请选择处理方式");
      return;
    }
    try {
      await post("/api/memories/resolve-conflict", {
        action: resolveAction.value,
        newKey: "",
        characterId: characterId?.() || "",
        newValue: conflictNewContent.value,
        newType: conflictNewType.value,
        importance: 5,
        conflictId: conflictList.value[0]?.id || "",
      });
      ElMessage.success("冲突已解决");
      conflictVisible.value = false;
      await fetchList?.();
    } catch (error: any) {
      ElMessage.error(error?.message || "处理失败");
    }
  }

  return {
    candidates,
    showCandidates,
    conversationList,
    showGenerateDialog,
    generating,
    generateConvId,
    editCandidateVisible,
    editForm,
    conflictVisible,
    conflictNewType,
    conflictNewContent,
    conflictList,
    resolveAction,
    loadCandidates,
    confirmCandidate,
    deleteCandidateItem,
    toggleCandidates,
    loadConversations,
    generateCandidates,
    editCandidate,
    saveEditCandidate,
    doResolveConflict,
  };
}
