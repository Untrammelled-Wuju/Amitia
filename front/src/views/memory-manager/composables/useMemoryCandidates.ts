import { ref } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "../../../composables/useApi"

export function useMemoryCandidates() {
  const { get, post, put, del } = useApi()
  const candidates = ref<any[]>([])
  const showCandidates = ref(false)

  async function loadCandidates() {
    try {
      const r: any = await get("/api/memory-candidates")
      candidates.value = r?.candidates || []
    } catch {}
  }

  async function confirmCandidate(c: any) {
    try {
      await post("/api/memory-candidates/" + c.id + "/accept", {})
      ElMessage.success("已保存")
    } catch {
      ElMessage.warning("自动保存失败，请手动创建")
    }
    candidates.value = candidates.value.filter(x => x.id !== c.id)
  }

  async function deleteCandidateItem(c: any) {
    try {
      await del("/api/memory-candidates/" + c.id)
      candidates.value = candidates.value.filter(x => x.id !== c.id)
      ElMessage.success("已删除")
    } catch { ElMessage.error("删除失败") }
  }

  function toggleCandidates() { showCandidates.value = !showCandidates.value }

  return { candidates, showCandidates, loadCandidates, confirmCandidate, deleteCandidateItem, toggleCandidates }
}
