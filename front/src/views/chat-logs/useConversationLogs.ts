// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, computed, nextTick } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import {
  fetchConvsApi,
  fetchMessagesApi,
  deleteMessageApi,
  clearConversationApi,
  deleteConversationApi,
  exportConversationApi,
  fetchFeedbackApi,
  fetchMoodsApi,
  fetchSummaryApi,
  generateSummaryApi,
  deleteSummaryApi,
  switchCharacterApi,
  fetchContextPreviewApi,
  continueChatApi,
  loadCharactersApi,
} from "./api"

export function useConversationLogs() {
  const characters = ref<any[]>([])

  const convs = ref<any[]>([])
  const convKeyword = ref("")
  const continueCharId = ref("")
  const channelFilter = ref("")
  const convPage = ref(1)
  const convTotal = ref(0)
  const selectedConv = ref<any>(null)
  const selectedConvId = ref("")

  const messages = ref<any[]>([])
  const msgPage = ref(1)
  const msgTotal = ref(0)
  const roleFilter = ref("")
  const msgListRef = ref<HTMLElement>()

  const filteredMessages = computed(() => {
    if (!roleFilter.value) return messages.value
    return messages.value.filter(m => m.role === roleFilter.value)
  })

  async function fetchConvs() {
    const params: any = { page: convPage.value, pageSize: 20 }
    if (convKeyword.value) params.keyword = convKeyword.value
    if (channelFilter.value) params.channel = channelFilter.value
    try {
      const r = await fetchConvsApi(params)
      let items: any[] = Array.isArray(r) ? r : (r?.items || [])
      const wechatItems = items.filter((c: any) => c.channel === 'wechat' || c.source === 'wechat')
      const otherItems = items.filter((c: any) => c.channel !== 'wechat' && c.source !== 'wechat')
      convs.value = [...wechatItems, ...otherItems]
      convTotal.value = r?.total || (Array.isArray(r) ? r.length : 0)
    } catch {}
  }

  async function selectConv(c: any) {
    selectedConv.value = c
    selectedConvId.value = c.id
    msgPage.value = 1
    await fetchMessages()
    await fetchSummary()
    await fetchMoods()
    await fetchFeedback()
  }

  async function fetchMessages() {
    if (!selectedConvId.value) return
    try {
      const r = await fetchMessagesApi(selectedConvId.value, { page: msgPage.value, pageSize: 50 })
      messages.value = Array.isArray(r) ? r : (r?.items || [])
      msgTotal.value = r?.total || (Array.isArray(r) ? r.length : 0)
      nextTick(() => { if (msgListRef.value) msgListRef.value.scrollTop = 0 })
    } catch {}
  }

  async function delMsg(id: string) {
    await ElMessageBox.confirm("确定删除这条消息？", "提示", { type: "warning" })
    await deleteMessageApi(id)
    ElMessage.success("已删除")
    fetchMessages()
    fetchConvs()
  }

  const moodMap = ref<Record<string, string>>({})
  const feedbackMap = ref<Record<string, any[]>>({})

  async function fetchFeedback() {
    if (!selectedConvId.value) return
    const map: Record<string, any[]> = {}
    try {
      const res = await fetchFeedbackApi()
      const items = res?.items || res || []
      for (const f of items) {
        if (!map[f.messageId]) map[f.messageId] = []
        map[f.messageId].push(f)
      }
      feedbackMap.value = map
    } catch { feedbackMap.value = {} }
  }

  async function fetchMoods() {
    if (!selectedConvId.value) return
    try {
      const r = await fetchMoodsApi(selectedConvId.value)
      const items = r?.items || []
      const map: Record<string, string> = {}
      for (const m of items) {
        if (m.messageId) map[m.messageId] = m.moodLabel
      }
      moodMap.value = map
    } catch { moodMap.value = {} }
  }

  async function clearConv() {
    await ElMessageBox.confirm("确定清空本会话所有消息？", "确认", { type: "warning" })
    await clearConversationApi(selectedConvId.value)
    messages.value = []
    ElMessage.success("已清空")
    fetchConvs()
  }

  async function delConv() {
    await ElMessageBox.confirm(
      "确定删除整个会话及其所有消息？此操作不可撤销。",
      "警告",
      { type: "warning", confirmButtonText: "删除", confirmButtonClass: "el-button--danger" }
    )
    await deleteConversationApi(selectedConvId.value)
    selectedConv.value = null
    selectedConvId.value = ""
    messages.value = []
    ElMessage.success("已删除")
    fetchConvs()
  }

  async function exportConv(format: string) {
    try {
      await exportConversationApi(format, [selectedConvId.value])
      ElMessage.success("已导出到 data/exports 目录")
    } catch {}
  }

  const currentSummary = ref<any>(null)
  const summaryVisible = ref(false)
  const genSummaryLoading = ref(false)

  async function fetchSummary() {
    if (!selectedConvId.value) return
    try {
      const r = await fetchSummaryApi(selectedConvId.value)
      currentSummary.value = r?.summaryText ? r : null
    } catch { currentSummary.value = null }
  }

  async function genSummary() {
    if (!selectedConvId.value) return
    genSummaryLoading.value = true
    try {
      await generateSummaryApi(selectedConvId.value)
      ElMessage.success("摘要已生成")
      await fetchSummary()
    } catch (err: any) {
    }
    genSummaryLoading.value = false
  }

  function viewSummary() {
    summaryVisible.value = true
  }

  async function delSummary() {
    await ElMessageBox.confirm("确定删除此会话的摘要?", "确认", { type: "warning" })
    if (!selectedConvId.value) return
    await deleteSummaryApi(selectedConvId.value)
    currentSummary.value = null
    ElMessage.success("已删除")
  }

  const devMode = ref(false)

  const ctxPreviewVisible = ref(false)
  const ctxPreviewLoading = ref(false)
  const ctxPreview = ref<any>(null)

  async function fetchContextPreview() {
    if (!selectedConvId.value) return
    ctxPreviewVisible.value = true
    ctxPreviewLoading.value = true
    ctxPreview.value = null
    try {
      ctxPreview.value = await fetchContextPreviewApi(selectedConvId.value)
    } catch (err: any) {
      ElMessage.error(err?.message || 'Failed to load context preview')
      ctxPreviewVisible.value = false
    } finally {
      ctxPreviewLoading.value = false
    }
  }

  async function switchCharacter(charId: string) {
    if (!selectedConvId.value) return
    try {
      await ElMessageBox.confirm(
        "切换角色后，该会话的后续回复将按新角色风格生成，历史消息保持不变。",
        "切换角色",
        { confirmButtonText: "确认切换", cancelButtonText: "取消", type: "warning" }
      )
    } catch { return }

    try {
      await switchCharacterApi(selectedConvId.value, charId)
      ElMessage.success("角色已切换")
      selectedConv.value.characterId = charId
      const char = characters.value.find((c: any) => c.id === charId)
      if (char) selectedConv.value.characterName = char.name
    } catch (e: any) {
      ElMessage.error("切换失败: " + (e?.response?.data?.message || e?.message || ""))
    }
  }

  async function continueChat() {
    if (!selectedConv.value) return
    try {
      const result = await continueChatApi({
        importBatchId: selectedConv.value.importBatchId || selectedConv.value.id,
        characterId: continueCharId.value || undefined,
      })
      if (result?.id) {
        ElMessage.success("Conversation created! Redirecting...")
        window.open(`/chat/${result.id}`, "_self")
      }
    } catch (err: any) {
      ElMessage.error(err?.message || "Failed to create conversation")
    }
  }

  async function loadCharacters() {
    try { characters.value = await loadCharactersApi() || [] } catch {}
  }

  return {
    characters,
    convs,
    convKeyword,
    continueCharId,
    channelFilter,
    convPage,
    convTotal,
    selectedConv,
    selectedConvId,
    messages,
    msgPage,
    msgTotal,
    roleFilter,
    msgListRef,
    filteredMessages,
    fetchConvs,
    selectConv,
    fetchMessages,
    delMsg,
    moodMap,
    feedbackMap,
    fetchFeedback,
    fetchMoods,
    clearConv,
    delConv,
    exportConv,
    currentSummary,
    summaryVisible,
    genSummaryLoading,
    fetchSummary,
    genSummary,
    viewSummary,
    delSummary,
    devMode,
    ctxPreviewVisible,
    ctxPreviewLoading,
    ctxPreview,
    fetchContextPreview,
    switchCharacter,
    continueChat,
    loadCharacters,
  }
}
