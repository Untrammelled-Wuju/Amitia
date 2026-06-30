// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref } from "vue"
import { ElMessage } from "element-plus"
import { ElMessageBox } from "element-plus"
import { useApi } from "./useApi"

export function useConversation(
  characterId: Ref<string>,
  charName: Ref<string>,
  convId: Ref<string>,
  convTitle: Ref<string>,
  messages: Ref<any[]>,
  conversations: Ref<any[]>,
  characters: Ref<any[]>,
  importBatches: Ref<any[]>,
  isWechatActive: Ref<boolean>,
  isQQActive: Ref<boolean>,
  wechatMsgCount: Ref<number>,
  qqMsgCount: Ref<number>,
  wechatOnline: Ref<boolean>,
  qqOnline: Ref<boolean>,
  msgPage: Ref<number>,
  hasMoreHistory: Ref<boolean>,
  isLoadingHistory: Ref<boolean>,
  disconnectSSE: () => void,
  connectSSE: () => void,
  scrollToBottom: (smooth?: boolean) => void,
) {
  const { get, post, del } = useApi()

  const HISTORY_PAGE_SIZE = 50
  let messagesVersion = 0

  function selectCharacter(c: any) {
    characterId.value = c.id
    charName.value = c.name
    localStorage.setItem("webchat-char-id", c.id)
  }

  async function loadCharacterConversation() {
    if (!characterId.value) return
    const c = characters.value.find((x: any) => x.id === characterId.value)
    let dedicatedConvId = c?.conversationId
    if (!dedicatedConvId) {
      try {
        const created = await post<any>("/api/web-chat/conversations", {
          characterId: characterId.value,
          title: "",
        })
        if (created?.id) dedicatedConvId = created.id
      } catch {}
    }
    if (!dedicatedConvId) {
      disconnectSSE()
      convId.value = ""
      convTitle.value = ""
      messages.value = []
      return
    }
    disconnectSSE()
    convId.value = dedicatedConvId
    convTitle.value = c?.name ? `${c.name} 的对话` : ""
    const version = ++messagesVersion
    try {
      const r = await get<any>(`/api/web-chat/conversations/${dedicatedConvId}/messages`)
      if (version !== messagesVersion) return
      const items = (r?.messages || r?.items || [])
      if (items.length) {
        if (items.length < 50 && (r?.totalPages || 1) <= 1) hasMoreHistory.value = false
        messages.value = items.map((m: any) => {
          if (m.imageUrl && m.content === "[图片]") return { ...m, content: "" }
          return m
        })
        msgPage.value = 1
        hasMoreHistory.value = items.length >= HISTORY_PAGE_SIZE
      } else {
        messages.value = []
      }
      scrollToBottom()
      connectSSE()
    } catch {
      if (version !== messagesVersion) return
      messages.value = []
    }
  }

  async function fetchConversations() {
    if (!characterId.value) { conversations.value = []; return }
    try {
      const r = await get<any>("/api/web-chat/conversations", { pageSize: 100 })
      const items = r?.conversations || r?.items || []
      conversations.value = items
      const wc = items.find((x: any) => x.channel === "wechat")
      wechatMsgCount.value = wc?.messageCount || wc?.msgCount || 0
      const qc = items.find((x: any) => x.channel === "qq")
      qqMsgCount.value = qc?.messageCount || qc?.msgCount || 0
    } catch { conversations.value = [] }
  }

  async function handleSelectConv(conv: any) {
    isWechatActive.value = conv?.channel === "wechat"
    isQQActive.value = conv?.channel === "qq"
    convId.value = conv.id
    convTitle.value = conv?.channel === "qq" ? "QQ聊天" : conv?.channel === "wechat" ? "微信聊天" : (conv.title || "")
    msgPage.value = 1
    hasMoreHistory.value = true
    const version = ++messagesVersion
    try {
      const url = `/api/web-chat/conversations/${encodeURIComponent(conv.id)}/messages`
      const r = await get<any>(url, { page: 1, pageSize: HISTORY_PAGE_SIZE })
      if (version !== messagesVersion) return
      const items = (r?.messages || r?.items || [])
      if (items.length) {
        messages.value = items.map((m: any) => {
          if (m.imageUrl && m.content === "[图片]") return { ...m, content: "" }
          return m
        })
        hasMoreHistory.value = items.length >= HISTORY_PAGE_SIZE
        const cid = conv.characterId || conv.character_id
        if (cid && cid !== characterId.value) {
          const c = characters.value.find((x: any) => x.id === cid)
          if (c) selectCharacter(c)
        } else if (!characterId.value || !charName.value) {
          const defaultChar = characters.value.find((c: any) => c.isDefault) || characters.value.find((c: any) => c.isActive) || characters.value[0]
          if (defaultChar) selectCharacter(defaultChar)
        }
      } else {
        messages.value = []
      }
      scrollToBottom()
      connectSSE()
    } catch {
      if (version !== messagesVersion) return
      messages.value = []
    }
  }

  async function handleSelectWechat(skipConfirm = false) {
    if (!skipConfirm) {
      try {
        await ElMessageBox.confirm("将切换到微信对话。", "切换对话", { confirmButtonText: "确认切换", cancelButtonText: "取消", type: "info" })
      } catch { return }
    }
    try {
      const convs = await get<any>("/api/web-chat/conversations", { pageSize: 50 })
      const items = convs?.conversations || convs?.items || []
      const wc = items.find((x: any) => x.id === "channel-wechat") || items.find((x: any) => x.channel === "wechat")
      const wechatDups = items.filter((x: any) => (x.id === "channel-wechat" || x.channel === "wechat") && x.id !== wc?.id)
      for (const d of wechatDups) {
        try { await del(`/api/web-chat/conversations/${encodeURIComponent(d.id)}`) } catch {}
      }
      if (wc) {
        localStorage.setItem("webchat-last-conv", "wechat")
        const cid = wc.characterId || wc.character_id
        if (cid) {
          const c = characters.value.find((x: any) => x.id === cid)
          if (c) selectCharacter(c)
        }
        if (!characterId.value || !charName.value) {
          const fallback = characters.value.find((x: any) => x.isDefault) || characters.value.find((x: any) => x.isActive) || characters.value[0]
          if (fallback) selectCharacter(fallback)
        }
        await handleSelectConv(wc)
        return
      }
      const defaultChar = characters.value.find((c: any) => c.isDefault || c.isActive)
      const created = await post<any>("/api/web-chat/conversations", {
        title: "微信对话", channel: "wechat", characterId: defaultChar?.id || characterId.value || ""
      })
      if (created?.id) {
        await handleSelectConv(created)
        return
      }
    } catch (e: any) {
      console.error("[handleSelectWechat]", e)
    }
    ElMessage.warning("未找到微信对话")
  }

  async function handleSelectQQ(skipConfirm = false) {
    if (!skipConfirm) {
      try {
        await ElMessageBox.confirm("将切换到QQ对话。", "切换对话", { confirmButtonText: "确认切换", cancelButtonText: "取消", type: "info" })
      } catch { return }
    }
    try {
      if (!qqOnline.value) {
        ElMessage.warning("QQ未连接")
        return
      }
      const convs = await get<any>("/api/web-chat/conversations", { pageSize: 50 })
      const items = convs?.conversations || convs?.items || []
      const qc = items.find((x: any) => x.id === "channel-qq") || items.find((x: any) => x.channel === "qq")
      const qqDups = items.filter((x: any) => (x.id === "channel-qq" || x.channel === "qq") && x.id !== qc?.id)
      for (const d of qqDups) {
        try { await del(`/api/web-chat/conversations/${encodeURIComponent(d.id)}`) } catch {}
      }
      if (qc) {
        localStorage.setItem("webchat-last-conv", "qq")
        const cid = qc.characterId || qc.character_id
        if (cid) {
          const c = characters.value.find((x: any) => x.id === cid)
          if (c) selectCharacter(c)
        }
        if (!characterId.value || !charName.value) {
          const fallback = characters.value.find((x: any) => x.isDefault) || characters.value.find((x: any) => x.isActive) || characters.value[0]
          if (fallback) selectCharacter(fallback)
        }
        await handleSelectConv(qc)
        return
      }
      const defaultChar = characters.value.find((c: any) => c.isDefault || c.isActive)
      const created = await post<any>("/api/web-chat/conversations", {
        title: "QQ对话", channel: "qq", characterId: defaultChar?.id || characterId.value || ""
      })
      if (created?.id) {
        await handleSelectConv(created)
        return
      }
    } catch (e: any) {
      console.error("[handleSelectQQ]", e)
    }
    ElMessage.warning("未找到QQ对话")
  }

  async function handleContinueImport(batch: any) {
    try {
      const r = await get<any>("/api/web-chat/conversations", { importBatchId: batch.id })
      const convs = r?.items || []
      if (convs.length > 0) {
        await handleSelectConv(convs[0])
      } else {
        const created = await post<any>("/api/web-chat/conversations", {
          characterId: characterId.value,
          title: `[导入] ${batch.title}`,
        })
        if (created?.id) {
          convId.value = created.id
          messages.value = []
        }
      }
      ElMessage.success("已切换到导入记录对话")
    } catch {}
  }

  return {
    selectCharacter,
    loadCharacterConversation,
    fetchConversations,
    handleSelectConv,
    handleSelectWechat,
    handleSelectQQ,
    handleContinueImport,
    getMessagesVersion: () => messagesVersion,
  }
}
