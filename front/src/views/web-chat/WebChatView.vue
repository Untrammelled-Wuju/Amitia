<template>
  <div class="webchat-page">
    <ChatBanners
      :model-missing="modelMissing"
      :is-offline="isOffline"
      :model-error="modelError"
      :import-context="importContext"
      :show-import-detail="showImportDetail"
      :conv-summary="convSummary"
      :show-summary="showSummary"
      @close-error="modelError = ''"
      @close-import="importContext = null"
      @toggle-summary="showSummary = !showSummary"
    />

    <ChatHeaderBar
      :char-name="charName"
      :char-identity="charIdentity"
      :conv-title="convTitle"
      :reply-style="replyStyle"
      :can-regenerate="canRegenerate"
      :messages-count="messages.length"
      :conv-id="convId"
      @toggle-drawer="showDrawer = true"
      @update:reply-style="replyStyle = $event"
      @regenerate="handleRegenerate"
      @clear="handleClear"
      @view-memories="handleViewMemories"
      @toggle-char-picker="showCharPicker = true"
    />
    <button class="profile-toggle-btn" @click="showProfiles = !showProfiles" :title="showProfiles ? '隐藏画像' : '显示画像'">👤</button>

    <MessagesArea
      ref="msgAreaRef"
      :messages="messages"
      :char-name="charName"
      :char-avatar="charAvatar"
      :sending="sending"
      :show-scroll-btn="showScrollBtn"
      :is-pulling="isPulling"
      :pull-ready="pullReady"
      :pull-loading="pullLoading"
      :pull-text="pullText"
      @scroll="onScroll"
      @wheel="onWheel"
      @touch-start="onMsgTouchStart"
      @touch-move="onMsgTouchMove"
      @touch-end="onMsgTouchEnd"
      @retry="handleRetry"
      @scroll-to-bottom="scrollToBottom(true)"
    />

    <ChatInput
      ref="inputRef"
      :disabled="modelMissing"
      :sending="sending"
      @send="handleSend"
      @image="onImageAttached"
      @removeImage="onImageRemoved"
      @stop="handleStop"
      @voiceAudio="handleVoiceAudio"
      @voiceText="handleVoiceText"
      @video="onVideoAttached"
      @removeVideo="onVideoRemoved"
    />

    <ConversationDrawer
      v-model:visible="showDrawer"
      :characters="characters"
      :import-batches="importBatches"
      :active-char-id="characterId"
      :wechat-msg-count="wechatMsgCount"
      :is-wechat-active="isWechatActive"
      :wechat-online="wechatOnline"
      :qq-msg-count="qqMsgCount"
      :isQQActive="isQQActive"
      :qqOnline="qqOnline"
      @select-char="handleSwitchChar"
      @select-wechat="handleSelectWechat"
      @select-q-q="handleSelectQQ"
      @continue-import="handleContinueImport"
    />


    <div v-if="showProfiles" class="profile-summary-panel">
      <div class="profile-panel-header">
        <h4>用户画像摘要</h4>
        <button class="profile-close-btn" @click="showProfiles = false">✕</button>
      </div>
      <div v-if="profileLoading" class="profile-loading">加载中...</div>
      <div v-else-if="profileItems.length === 0" class="profile-empty">暂无画像</div>
      <div v-else class="profile-items">
        <div v-for="p in profileItems" :key="p.id" class="profile-item">
          <span class="profile-cat">{{ profileCatLabel(p.category) }}</span>
          <span class="profile-name">{{ p.attributeName }}</span>
          <span class="profile-val">{{ p.attributeValue }}</span>
          <span class="profile-conf" :class="profileConfClass(p.confidence)">{{ p.confidence }}%</span>
        </div>
      </div>
    </div>

    <CharacterPickerDialog
      v-model:visible="showCharPicker"
      :characters="characters"
      :character-id="characterId"
      @select="handleSwitchChar"
    />

    <MemoryPanel
      v-model:visible="showMemories"
      :memories="memories"
    />
  </div>
    <button class="mem-inject-toggle-btn" @click="showMemInject = !showMemInject" :title="showMemInject ? '隐藏记忆注入' : '记忆注入'">🧠</button>
    <div v-if="showMemInject" class="mem-inject-panel">
      <div class="mi-header">
        <h4>记忆注入</h4>
        <button class="mi-close-btn" @click="showMemInject = false">✕</button>
      </div>
      <div v-if="miLoading" class="mi-loading">加载中...</div>
      <div v-else>
        <div class="mi-section" v-if="miMemories.length">
          <h5>当前检索记忆 ({{ miMemories.length }})</h5>
          <div v-for="m in miMemories" :key="m.id" class="mi-item">
            <el-tag size="small" :type="m.matchType === 'vector' ? 'success' : 'info'">{{ m.matchType }}</el-tag>
            <span class="mi-layer">{{ m.memoryLayer || '事实记忆' }}</span>
            <span class="mi-score">{{ (m.score * 100).toFixed(0) }}%</span>
            <div class="mi-content">{{ m.memory?.value || m.value }}</div>
            <el-button size="small" text type="danger" @click="feedbackMemory(m, 'irrelevant')">不相关</el-button>
            <el-button size="small" text type="warning" @click="feedbackMemory(m, 'wrong')">错误</el-button>
          </div>
        </div>
        <div class="mi-section" v-if="miProfiles.length">
          <h5>用户画像 ({{ miProfiles.length }})</h5>
          <div v-for="p in miProfiles" :key="p.id" class="mi-profile-card">
            <span>{{ p.attributeName }}: {{ p.attributeValue }}</span>
            <el-tag :type="p.confidence >= 80 ? 'success' : 'warning'" size="small">{{ p.confidence }}%</el-tag>
          </div>
        </div>
        <div class="mi-section" v-if="miWorldbookHits.length">
          <h5>世界书命中 ({{ miWorldbookHits.length }})</h5>
          <div v-for="w in miWorldbookHits" :key="w.entry?.id || w.id" class="mi-wb-hit">
            <el-tag size="small" type="danger">命中</el-tag>
            <span>{{ w.entry?.matchPattern || w.matchPattern }}</span>
          </div>
        </div>
        <div class="mi-section">
          <h5>压缩状态</h5>
          <div class="mi-compress">
            <span>已压缩 {{ miCompression.compressedRounds || 0 }} / {{ miCompression.totalRounds || 0 }} 轮</span>
            <span v-if="miCompression.lastCompressedAt">上次: {{ miCompression.lastCompressedAt }}</span>
          </div>
        </div>
        <div class="mi-section">
          <h5>管线状态</h5>
          <div class="mi-pipeline">
            <template v-for="l in (miPipeline?.layers || [])" :key="l.layer">
              <el-tooltip :content="l.name + ': ' + l.status + ' (' + l.durationMs + 'ms)'" placement="top">
                <span class="mi-pl-dot" :style="{backgroundColor: l.status === 'completed' ? '#67c23a' : l.status === 'skipped' ? '#c0c4cc' : '#f56c6c'}"></span>
              </el-tooltip>
            </template>
          </div>
        </div>
        <div class="mi-empty" v-if="!miMemories.length && !miProfiles.length && !miWorldbookHits.length">暂无记忆注入数据</div>
      </div>
    </div>
</template>
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch, inject } from "vue"
import { useRoute, useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi, isLoggedIn, getToken } from "../../composables/useApi"
import { useCachedApi } from "../../composables/useCachedApi"
import ChatBanners from "../../components/ChatBanners.vue"
import ChatHeaderBar from "../../components/ChatHeaderBar.vue"
import MessagesArea from "../../components/MessagesArea.vue"
import ChatInput from "../../components/ChatInput.vue"
import ConversationDrawer from "../../components/ConversationDrawer.vue"
import CharacterPickerDialog from "../../components/CharacterPickerDialog.vue"
import MemoryPanel from "../../components/MemoryPanel.vue"
import { useProfile } from "@/composables/useProfile"

const route = useRoute()
const router = useRouter()
const { get, post, put, del } = useApi()
const { cachedGet, cachedPost, invalidateCache, saveCache } = useCachedApi()
const currentCharName = inject<any>("currentCharName", null)

// ==========================================================
// State
// ==========================================================
const characters = ref<any[]>([])
const messages = ref<any[]>([])
const importBatches = ref<any[]>([])
const memories = ref<any[]>([])

const characterId = ref("")
const cachedDef = (() => { try { const v = localStorage.getItem("uai-default-char"); return v ? JSON.parse(v) : null } catch { return null } })()
const charName = ref(cachedDef?.name || "")
const charIdentity = ref(cachedDef?.identity || "")
const charAvatar = ref("")
const convId = ref("")
const convTitle = ref("")
const conversations = ref<any[]>([])
const wechatMsgCount = ref(0)
const isWechatActive = ref(false)
const wechatOnline = ref(false)
const qqMsgCount = ref(0)
const isQQActive = ref(false)
const qqOnline = ref(false)
const callActive = ref(false)
const currentImageBase64 = ref<string | null>(null)
const currentImageFile = ref<File | null>(null)
const pendingImageBase64 = ref<string | null>(null)
const pendingAudioUrl = ref<string | null>(null)
const pendingVideoUrl = ref<string | null>(null)
const sending = ref(false)

const modelMissing = ref(false)

const showDrawer = ref(false)
const showCharPicker = ref(false)
const showMemories = ref(false)
const showScrollBtn = ref(false)
const showProfiles = ref(false)
const showMemInject = ref(false)
const miLoading = ref(false)
const miMemories = ref<any[]>([])
const miProfiles = ref<any[]>([])
const miWorldbookHits = ref<any[]>([])
const miCompression = ref<any>({})
const miPipeline = ref<any>(null)

const profileLoading = ref(false)
const profileItems = ref<any[]>([])
const { profiles: profData, fetchProfiles, categoryLabel: profileCatLabel } = useProfile()


const profileCatMap: Record<string, string> = {
  personal_info: '个人信息', preference: '偏好', habit: '习惯',
  fear: '恐惧', relationship: '关系', health: '健康', plan: '计划',
}
function profileConfClass(c: number): string { if (c >= 80) return "conf-high"; if (c >= 50) return "conf-mid"; return "conf-low" }

onMounted(async () => {
  await fetchProfiles({ pageSize: 10 })
  await loadMemInject()
  profileItems.value = profData.value
})
const importContext = ref<any>(null)
const showImportDetail = ref(false)

const msgAreaRef = ref<InstanceType<typeof MessagesArea>>()
const inputRef = ref<InstanceType<typeof ChatInput>>()

let abortController: AbortController | null = null

function attachLocalImages(_msgs: any[]) {}
let messagesVersion = 0
let lastPolledMsgId: string | null = null
let eventSource: EventSource | null = null


function connectSSE() {
  disconnectSSE()
  if (!convId.value) return
  const apiBase = (import.meta as any).env?.VITE_API_URL || ""
  const url = apiBase + "/api/messages/stream?conversationId=" + encodeURIComponent(convId.value) + (lastPolledMsgId ? "&since=" + encodeURIComponent(lastPolledMsgId) : "")
  eventSource = new EventSource(url)
  eventSource.onmessage = function(event) {
    try {
      const msg = JSON.parse(event.data)
      if (!msg.role || msg.role === "tool") return
      if ((msg as any).tool_calls_json) return
      if (!messages.value.some((m: any) => m.id === msg.id)) {
        if (msg.role === "user") {
          const now = Date.now()
          const dup = messages.value.some((m: any) =>
            m.role === "user" && m.content === msg.content &&
            String(m.id).startsWith("user-") &&
            (now - new Date(m.createdAt).getTime()) < 15000
          )
          if (dup) return
        }
        lastPolledMsgId = msg.id || lastPolledMsgId
        messages.value.push(msg)
        if (msg.source === "proactive" && "Notification" in window && (Notification as any).permission === "granted") {
          new Notification("日程提醒", { body: msg.content.slice(0, 200), tag: "reminder-" + msg.id })
        }
        scrollToBottom()
        fetchWechatMsgCount()
        fetchQQStatus()
      }
    } catch (_e) { /* skip */ }
  }
  eventSource.onerror = () => {
    disconnectSSE()
    setTimeout(() => { if (convId.value) connectSSE() }, 3000)
  }
}

function disconnectSSE() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

// Summary
const convSummary = ref("")
const showSummary = ref(false)

async function fetchConvSummary() {}

async function loadMemInject() {
  miLoading.value = true
  try {
    if (convId.value) {
      const { useApi } = await import("../../composables/useApi")
      const { get } = useApi()
      try { const compR: any = await get("/api/chats/" + convId.value + "/compression-status"); miCompression.value = compR || {} } catch {}
      try { const pipeR: any = await get("/api/memory/pipeline/status"); miPipeline.value = pipeR } catch {}
      try { const profR: any = await get("/api/profiles", { pageSize: 5 }); miProfiles.value = profR?.items || [] } catch {}
    }
  } catch {}
  miLoading.value = false
}

function feedbackMemory(m: any, type: string) {
  ElMessage.info("已标记为" + (type === "irrelevant" ? "不相关" : "错误") + "，将优化后续检索")
}

async function checkWorldbookHits(userMsg: string) {
  try {
    const { useApi } = await import("../../composables/useApi")
    const { get } = useApi()
    const r: any = await get("/api/world-book/match", { text: userMsg })
    miWorldbookHits.value = r?.matches || []
  } catch { miWorldbookHits.value = [] }
}


// Network & error states
const isOffline = ref(!navigator.onLine)


// Watch network recovery while sending (Step 70)
watch(isOffline, (offline) => {
  if (!offline && sending.value && messages.value.some(m => m.status === 'sending')) {
    ElMessage.info("网络已恢复，可重新发送消息")
  }
})
// Refresh characters when drawer opens (to pick up default changes)
// Also listen for default-char-changed events from other components
watch(showDrawer, (open) => {
  if (open) refreshCharacters()
})

const handleOnline = () => {
  isOffline.value = false
  ElMessage.success("网络已恢复")
}
const handleOffline = () => {
  isOffline.value = true
  ElMessage.warning("网络已断开")
}
const handleDefaultCharChanged = () => refreshCharacters()

async function refreshCharacters() {
  try {
    const chars = await get<any[]>("/api/characters")
    if (Array.isArray(chars)) {
      characters.value = chars
      // Also update localStorage cache
      saveCache("/api/characters", chars)
    }
  } catch {}
}
const modelError = ref("")

// Reply style
const replyStyle = ref<string>("natural")

// Scroll-aware state
const userScrolledUp = ref(false)

// Pull-to-refresh states
const isPulling = ref(false)
const pullReady = ref(false)
const pullLoading = ref(false)
const pullText = ref("Pull down to load earlier messages")
const pullStartY = ref(0)

// Pagination for history loading
const isLoadingHistory = ref(false)
const hasMoreHistory = ref(true)
const msgPage = ref(1)
const HISTORY_PAGE_SIZE = 50


// ==========================================================
// Computed
// ==========================================================
const canRegenerate = computed(() => {
  if (!convId.value || messages.value.length === 0) return false
  const last = messages.value[messages.value.length - 1]
  return last?.role === "assistant"
})

// ==========================================================
// Init
// ==========================================================
let proactiveSSE: EventSource | null = null
function connectProactiveSSE() {
  try {
    proactiveSSE = new EventSource("/api/proactive-sse")
    proactiveSSE.addEventListener("proactive_message", (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (msg.conversationId === convId.value) {
          messages.value.push({ id: msg.messageId, conversationId: msg.conversationId, role: msg.role, content: msg.content, source: msg.source, createdAt: new Date().toISOString() })
          nextTick(() => scrollToBottom())
        }
        } catch {}
        fetchWechatMsgCount()
        fetchQQStatus()
    })
    proactiveSSE.onerror = () => { proactiveSSE?.close(); setTimeout(connectProactiveSSE, 5000) }
  } catch { setTimeout(connectProactiveSSE, 5000) }
}

let __fsLast = 0
let __wcfLast = 0
async function fetchWechatMsgCount() {
  if (Date.now() - __wcfLast < 8000) return
  __wcfLast = Date.now()
  try {
    const r = await get<any>("/api/wechat/status")
    const status = r?.data || r
    wechatOnline.value = status?.status === "connected" || status?.accountId != null
  } catch {}
}

async function fetchStatus() {
  if (Date.now() - __fsLast < 8000) return
  __fsLast = Date.now()
  try {
    const r = await get<any>("/api/qq/status")
    const data = r?.data || r
    qqOnline.value = data?.qqOnline || data?.status === "online"
  } catch {}
}

async function fetchQQStatus() { fetchStatus() }
onMounted(async () => {
  fetchWechatMsgCount()
  fetchQQStatus()
  setInterval(fetchWechatMsgCount, 30000)
  setInterval(fetchStatus, 15000)
  connectProactiveSSE()
  history.scrollRestoration = 'manual'
  // Network listeners
  window.addEventListener("online", () => {
    isOffline.value = false
    ElMessage.success("网络已恢复")
  })
  window.addEventListener("offline", () => {
    isOffline.value = true
    ElMessage.warning("网络已断开")
  })

  // Check login for cloud mode
  const h = await get<any>("/api/health").catch(() => null)
  if (h?.deployMode === "cloud-web" && !isLoggedIn()) {
    router.push("/login")
    return
  }

  // Check model
  if (h?.model === "not_configured") {
    modelMissing.value = true
  }

  // Load characters (cache-first)
  // Clear potentially stale character cache
  const CACHE_VERSION = 2
  const storedVersion = localStorage.getItem("char_cache_version")
  if (String(storedVersion) !== String(CACHE_VERSION)) {
    invalidateCache("_api_characters")
    localStorage.setItem("char_cache_version", String(CACHE_VERSION))
  }

  const { data: cachedChars, refresh: refreshChars } = await cachedGet<any[]>("/api/characters")
  if (cachedChars.value?.length) {
    characters.value = cachedChars.value
    const lastConv = localStorage.getItem("webchat-last-conv")
    if (lastConv === "wechat") {
      await handleSelectWechat(true)
      return
    }
    const savedId = localStorage.getItem("webchat-char-id")
    const preferred = savedId ? characters.value.find((c: any) => c.id === savedId) : null
    if (preferred) { selectCharacter(preferred) }
    else {
      const active = characters.value.find((c: any) => c.isActive)
      if (active) selectCharacter(active)
      else if (characters.value.length > 0) selectCharacter(characters.value[0])
    }
    // Sync default character to localStorage
    const def = characters.value.find((c: any) => c.isDefault)
    if (def) {
      localStorage.setItem("uai-default-char", JSON.stringify({
        id: def.id, name: def.name,
        identity: def.identity || def.personality || "",
        updatedAt: Date.now(),
      }))
    }
  }
  // Background refresh - sync default char to localStorage
  refreshChars().then(() => {
    if (cachedChars.value?.length) {
      characters.value = cachedChars.value
      const active = characters.value.find((c: any) => c.isActive)
      if (active && !characterId.value) selectCharacter(active)
      // Sync default character to localStorage
      const def = characters.value.find((c: any) => c.isDefault)
      if (def) {
        localStorage.setItem("uai-default-char", JSON.stringify({
          id: def.id, name: def.name,
          identity: def.identity || def.personality || "",
          updatedAt: Date.now(),
        }))
      }
    }
  })
  // Auto-load character conversation
  await loadCharacterConversation()
  await fetchConversations()


  // Load import batches
  try {
    const r = await get<any>("/api/imports/batches")
    importBatches.value = r?.items || []
  } catch {}

  // Focus input
  nextTick(() => inputRef.value?.focus())
})

onUnmounted(() => {
  proactiveSSE?.close()
  disconnectSSE()
  window.removeEventListener("online", () => { isOffline.value = false })
  window.removeEventListener("offline", () => { isOffline.value = true })
})

// ==========================================================
// Character selection
// ==========================================================
function selectCharacter(c: any) {
  characterId.value = c.id
  charName.value = c.name
  charIdentity.value = c.identity || c.personality || ""
  charAvatar.value = c.avatar || ""
  localStorage.setItem("webchat-char-id", c.id)
  if (currentCharName) currentCharName.value = c.name
}

async function handleSwitchChar(c: any) {
  // Confirm before switching
  try {
    await ElMessageBox.confirm(
      "切换角色后，将加载新角色的对话记录。",
      "切换角色",
      { confirmButtonText: "确认切换", cancelButtonText: "取消", type: "warning" }
    )
  } catch {
    return // User cancelled
  }

  isWechatActive.value = false
  isQQActive.value = false
  localStorage.setItem("webchat-last-conv", "char")
  selectCharacter(c)
  showCharPicker.value = false
  ElMessage.success("已切换角色: " + c.name)
  // Load the new character's conversation
  await loadCharacterConversation()
  if (!convId.value) messages.value = []
  fetchConversations()
}

// ==========================================================
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
    lastPolledMsgId = null
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
      attachLocalImages(messages.value)
      msgPage.value = 1; hasMoreHistory.value = items.length >= HISTORY_PAGE_SIZE
      await nextTick(); scrollToBottom()
    }
    else { messages.value = [] }
    lastPolledMsgId = messages.value[messages.value.length - 1]?.id || null
    connectSSE()
    fetchConvSummary()
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
async function handleSelectWechat(skipConfirm = false) {
  if (!skipConfirm) {
    try {
      await ElMessageBox.confirm(
        "将切换到微信对话。",
        "切换对话",
        { confirmButtonText: "确认切换", cancelButtonText: "取消", type: "info" }
      )
    } catch {
      return
    }
  }
  showDrawer.value = false
  try {
    const convs = await get<any>("/api/web-chat/conversations", { pageSize: 50 })
    const items = convs?.conversations || convs?.items || []
    const wc = items.find((x: any) => x.id === "channel-wechat") || items.find((x: any) => x.channel === "wechat")
      // 清理重复的微信对话
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
      await ElMessageBox.confirm(
        "将切换到QQ对话。",
        "切换对话",
        { confirmButtonText: "确认切换", cancelButtonText: "取消", type: "info" }
      )
    } catch {
      return
    }
  }
  showDrawer.value = false
  try {
    if (!qqOnline.value) {
      ElMessage.warning("QQ未连接")
      return
    }
    const convs = await get<any>("/api/web-chat/conversations", { pageSize: 50 })
    const items = convs?.conversations || convs?.items || []
    const qc = items.find((x: any) => x.id === "channel-qq") || items.find((x: any) => x.channel === "qq")
      // 清理重复的QQ对话
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

async function handleSelectConv(conv: any) {
  showDrawer.value = false
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
      await nextTick()
      scrollToBottom()
    } else {
      messages.value = []
    }
    lastPolledMsgId = messages.value[messages.value.length - 1]?.id || null
    connectSSE()
  } catch (e: any) {
    if (version !== messagesVersion) return
    console.error("[handleSelectConv] error:", e.message)
    messages.value = []
  }
  fetchConvSummary()
}

async function handleContinueImport(batch: any) {
  showDrawer.value = false
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
function uuidLike(obj: any): boolean { return typeof obj.id === "string" && obj.id.length > 20 }

function onImageAttached(file: File, base64: string) {
  currentImageFile.value = file
  currentImageBase64.value = base64
}

function onImageRemoved() {
  currentImageFile.value = null
  currentImageBase64.value = null
}

function onVideoAttached(_file: File, videoUrl: string) {
  pendingVideoUrl.value = videoUrl
}

function onVideoRemoved() {
  pendingVideoUrl.value = null
}

async function handleVoiceAudio(blob: Blob, transcript?: string, duration?: number) {
  try {
    const formData = new FormData()
    formData.append("audio", blob, "voice.webm")
    const token = localStorage.getItem("ai-companion-token") || ""
    const res = await fetch("/api/voice/upload", {
      method: "POST",
      headers: { Authorization: "Bearer " + token },
      body: formData,
    })
    if (!res.ok) throw new Error("Voice upload failed")
    const data = await res.json()
    const audioUrl = data?.data?.audioUrl || data?.audioUrl || ""
    if (!audioUrl) throw new Error("No audioUrl returned")
    pendingAudioUrl.value = audioUrl
    const sendText = transcript || "[语音]"
    await doActualSend(sendText, audioUrl, true)
  } catch (err: any) {
    console.error("[Voice] upload failed:", err)
    ElMessage.error("语音发送失败")
  }
}

function handleVoiceText(text: string) {
  if (text) {
    inputRef.value?.setText?.(text)
  }
}





async function handleImageSend(text: string, imageBase64: string) {
  if (sending.value) return
  currentImageBase64.value = null
  currentImageFile.value = null
  const hasUserText = !!(text && text.trim())
  const sendText = hasUserText ? text : "[图片]"
  pendingImageBase64.value = imageBase64
  await doActualSend(sendText)
}

async function handleSend(text: string, imageBase64?: string, videoBase64?: string) {
  if (videoBase64 || pendingVideoUrl.value) {
    pendingVideoUrl.value = videoBase64 || pendingVideoUrl.value || ""
    const sendText = text.trim() || "[视频]"
    doActualSend(sendText, undefined, undefined, pendingVideoUrl.value)
    pendingVideoUrl.value = null
    return
  }
  if (imageBase64 || currentImageBase64.value) {
    handleImageSend(text, imageBase64 || currentImageBase64.value || "")
    return
  }
  if (sending.value) return
  doActualSend(text)
}

// doActualSend 是原来的 handleSend 逻辑
async function doActualSend(text: string, audioUrl?: string, voiceMessage?: boolean, videoUrl?: string) {
  if (sending.value) return

  // 断开 SSE 轮询，防止与本 SSE 流重复推送消息
  disconnectSSE()

  // 立即插入用户消息，不等后端返回
  const userMsgLocalId = "user-" + Date.now()
  const imgUrl = pendingImageBase64.value
  const finalAudioUrl = audioUrl || pendingAudioUrl.value
  const finalVideoUrl = videoUrl || pendingVideoUrl.value
  pendingImageBase64.value = null
  pendingAudioUrl.value = null
  pendingVideoUrl.value = null
  const hasImage = !!(imgUrl)
  const hasVoice = !!(finalAudioUrl)
  const hasVideo = !!(finalVideoUrl)
  const displayContent = (hasVoice && !text.trim()) ? "[语音]" : (hasVideo && !text.trim()) ? "" : (hasImage && text === "[图片]") ? "" : text
  const sendContent = (hasVoice && !text.trim()) ? "[语音]" : (hasVideo && !text.trim()) ? "[视频]" : (hasImage && !text.trim()) ? "[图片]" : text
  messages.value.push({ id: userMsgLocalId, role: "user", content: displayContent, imageUrl: imgUrl || undefined, audioUrl: finalAudioUrl || undefined, audioDuration: 0, videoUrl: finalVideoUrl || undefined, status: "sent", conversationId: convId.value, createdAt: new Date().toISOString() })
  
  scrollToBottom(true)

  checkWorldbookHits(text)
  sending.value = true
  modelError.value = ""

  try {
    const token = localStorage.getItem("ai-companion-token") || ""
    const res = await fetch("/api/web-chat/send-stream", {
      method: "POST",
      headers: { "Content-Type": "application/json", "Authorization": `Bearer ${token}` },
      body: JSON.stringify({ conversationId: convId.value || undefined, characterId: characterId.value || undefined, message: sendContent, imageUrl: imgUrl || "", audioUrl: finalAudioUrl || "", voiceMessage: !!finalAudioUrl, videoUrl: finalVideoUrl || "" }),
    })

    if (!res.ok) throw new Error(`HTTP ${res.status}`)

    const reader = res.body?.getReader()
    if (!reader) throw new Error("No response stream")
    const decoder = new TextDecoder()
    let buffer = ""
    let eventType = ""

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split("\n")
      buffer = lines.pop() || ""
      for (const line of lines) {
        if (line.startsWith("event:")) {
          eventType = line.slice(6).trim()
          continue
        }
        if (!line.startsWith("data:")) continue
        try {
          const data = JSON.parse(line.slice(5).trim())
          if (data.conversationId && !convId.value) convId.value = data.conversationId

          if (eventType === "message_start") {
            // 用后端返回的 DB ID 替换本地临时用户消息 ID，避免重复
            const uIdx = messages.value.findIndex((m: any) => m.id === userMsgLocalId)
            if (uIdx >= 0 && data.userMessageId) {
              messages.value[uIdx].id = data.userMessageId
            }
            if (data.conversationId && !convId.value) convId.value = data.conversationId
            continue
          }

            if (eventType === "token" && data.content) {
              messages.value.push({
                id: data.id || ("msg-" + Date.now()), role: "assistant", content: data.content,
                status: "streaming", conversationId: data.conversationId || convId.value,
                createdAt: data.createdAt || new Date().toISOString()
              })
              scrollToBottom(true)
            }

            if (eventType === "voice_audio" && data.audioUrl) {
              let targetMsg = [...messages.value].reverse().find((m: any) => m.id === data.messageId || m.status === "streaming")
              if (!targetMsg && data.content) {
                messages.value.push({
                  id: data.messageId || ("msg-" + Date.now()), role: "assistant", content: data.content,
                  status: "streaming", conversationId: data.conversationId || convId.value,
                  createdAt: data.createdAt || new Date().toISOString(), audioUrl: data.audioUrl, audioDuration: data.duration || 0,
                })
                scrollToBottom(true)
                return
              }
              if (targetMsg) {
                targetMsg.audioUrl = data.audioUrl
                targetMsg.audioDuration = data.duration
              }
            }

                    if (eventType === "done") {
              const lastStreaming = [...messages.value].reverse().find((m: any) => m.status === "streaming" && m.id !== "streaming")
              if (lastStreaming?.id) lastPolledMsgId = lastStreaming.id
              messages.value.forEach((m: any) => {
                if (m.status === "streaming") m.status = "sent"
              })
            }
        } catch { /* skip malformed */ }
      }
    }
    // SSE already streaming, no need for DB polling
  } catch (err: any) {
    if (err?.name === "AbortError") {
      const streaming = messages.value.filter((m: any) => m.status === "streaming")
      for (const sm of streaming) {
        sm.status = "interrupted"
      }
    } else {
      console.error("[Stream] Failed:", err)
      const errMsg = err?.message || "连接失败"
      modelError.value = errMsg
      ElMessage.error(errMsg)
      const tIdx = messages.value.findIndex((m: any) => m.id === userMsgLocalId) 
      if (tIdx >= 0) {
        messages.value[tIdx] = { ...messages.value[tIdx], id: "failed-" + Date.now(), status: "failed" }
      }
      const sIdx = messages.value.findIndex(m => m.id === "streaming")
      if (sIdx >= 0) messages.value.splice(sIdx, 1)
    }
  } finally {
    sending.value = false
    abortController = null
    const lastMsg = messages.value[messages.value.length - 1]
    if (lastMsg?.id && lastMsg.id !== "streaming") lastPolledMsgId = lastMsg.id
    fetchWechatMsgCount()
    fetchQQStatus()

  }
}

function handleStop() {
  if (abortController) {
    abortController.abort()
    abortController = null
  }
  messages.value.filter((m: any) => m.status === "streaming").forEach((m: any) => m.status = "interrupted")
  sending.value = false
}

// ==========================================================
// Regenerate
// ==========================================================

// ==========================================================
// Retry failed message (Step 70)
// ==========================================================
async function handleRetry(msg: any) {
  if (sending.value) return
  
  // Remove the failed user message and its partial assistant reply
  messages.value = messages.value.filter(m => m.id !== msg.id)
  // Also remove any partial assistant message that follows it
  const lastAsst = [...messages.value].reverse().find(m => m.role === 'assistant' && (m.status === 'interrupted' || m.status === 'failed'))
  if (lastAsst) {
    messages.value = messages.value.filter(m => m.id !== lastAsst.id)
  }
  
  // Try to clean up on backend too
  try {
    await post("/api/web-chat/retry", { messageId: msg.id })
  } catch { /* best effort */ }
  
  // Resend the message
  const text = msg.content
  if (text) {
    await handleSend(text)
  }
}

async function handleRegenerate() {
  if (!canRegenerate.value || !convId.value) return
  sending.value = true
  try {
    const res = await post<any>(`/api/web-chat/conversations/${convId.value}/regenerate`)
    if (res) {
      // Replace last assistant message
      if (res.assistantMessage) {
        const lastIdx = messages.value.length - 1
        if (messages.value[lastIdx]?.role === "assistant") {
          messages.value[lastIdx] = res.assistantMessage
        } else {
          messages.value.push(res.assistantMessage)
        }
      }
      scrollToBottom(true)
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "重新生成失败")
  } finally {
    sending.value = false
  }
}

// ==========================================================
// Clear conversation
// ==========================================================
async function handleClear() {
  try {
    await ElMessageBox.confirm("确定清空当前会话的所有消息？", "提示", {
      type: "warning",
      confirmButtonText: "清空",
    })
    if (convId.value) {
      await del(`/api/web-chat/conversations/${convId.value}/messages`)
    }
    messages.value = []
    ElMessage.success("已清空")
  } catch { /* cancelled */ }
}

// ==========================================================
// View memories
// ==========================================================
async function handleViewMemories() {
  showMemories.value = true
  try {
    const r = await get<any>("/api/memories", { page: 1, pageSize: 10 })
    memories.value = r?.items || []
  } catch {}
}

// ==========================================================
// Scroll helpers
// ==========================================================
function scrollToBottom(smooth = false) {
  // If user is scrolling up during streaming, don't auto-scroll
  if (!smooth && userScrolledUp.value) return
  userScrolledUp.value = false
  nextTick(() => {
    requestAnimationFrame(() => {
      const el = msgAreaRef.value?.rootEl
      if (!el) return
      el.scrollTo({
        top: el.scrollHeight,
        behavior: smooth ? "smooth" : "auto",
      })
    })
  })
}

function onScroll() {
  const el = msgAreaRef.value?.rootEl
  if (!el) return
  const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  const threshold = 200
  showScrollBtn.value = distFromBottom > threshold
  // User is scrolling up if not near bottom
  userScrolledUp.value = distFromBottom > 100

  // Load older messages when scrolled to top
  if (el.scrollTop <= 50 && hasMoreHistory.value && !isLoadingHistory.value && convId.value) {
    loadOlderMessages()
  }
}



function onWheel(e: WheelEvent) {
  const el = msgAreaRef.value?.rootEl
  if (!el) return
  if (e.deltaY >= 0) return
  const noOverflow = el.scrollHeight <= el.clientHeight
  const atTop = el.scrollTop <= 0
  if ((noOverflow || atTop) && hasMoreHistory.value && !isLoadingHistory.value && convId.value) {
    e.preventDefault()
    loadOlderMessages()
  }
}
// ==========================================================
// Load older messages (pagination)
// ==========================================================
async function loadOlderMessages() {
  if (isLoadingHistory.value || !hasMoreHistory.value || !convId.value) return
  isLoadingHistory.value = true
  try {
    const r = await get<any>(`/api/web-chat/conversations/${convId.value}/messages`, {
      page: msgPage.value + 1,
      pageSize: HISTORY_PAGE_SIZE,
    })
    const older = r?.items || []
    if (older.length === 0) {
      hasMoreHistory.value = false
    } else {
      // Preserve scroll position after prepending
      const el = msgAreaRef.value?.rootEl
      const prevHeight = el?.scrollHeight || 0
      attachLocalImages(older)
      messages.value = [...older, ...messages.value]
      msgPage.value++
      nextTick(() => {
        if (el) {
          el.scrollTop = el.scrollHeight - prevHeight
        }
      })
    }
  } catch {
    // Silently fail - history load is best-effort
  } finally {
    isLoadingHistory.value = false
  }
}

// ==========================================================
// Pull-to-refresh (mobile - load older messages)
// ==========================================================
function onMsgTouchStart(e: TouchEvent) {
  const el = msgAreaRef.value?.rootEl
  if (!el || el.scrollTop > 5) return
  pullStartY.value = e.touches[0].clientY
  isPulling.value = true
  pullText.value = "Pull down to load earlier messages"
  pullReady.value = false
}

function onMsgTouchMove(e: TouchEvent) {
  if (!isPulling.value) return
  const dy = e.touches[0].clientY - pullStartY.value
  if (dy > 60) {
    pullReady.value = true
    pullText.value = "Release to load"
  } else {
    pullReady.value = false
    pullText.value = "Pull down to load earlier messages"
  }
}

async function onMsgTouchEnd() {
  if (!isPulling.value) return
  if (pullReady.value && hasMoreHistory.value && !isLoadingHistory.value) {
    pullLoading.value = true
    pullText.value = "Loading..."
    await loadOlderMessages()
    pullLoading.value = false
  }
  isPulling.value = false
  pullReady.value = false
  pullText.value = "Pull down to load earlier messages"
}

// ==========================================================
// Helpers
// ==========================================================
</script>
<style scoped>
.webchat-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}
@media (max-width: 768px) {
  .webchat-page {
    max-width: 100%;
  }
}


.profile-toggle-btn {
  position: fixed; right: 16px; top: 40px; width: 36px; height: 36px;
  border-radius: 50%; border: 1px solid #ddd; background: #fff;
  cursor: pointer; font-size: 16px; z-index: 901;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
}
.mem-inject-toggle-btn {
  position: fixed; right: 16px; top: 90px; width: 36px; height: 36px;
  border-radius: 50%; border: 1px solid #ddd; background: #fff;
  cursor: pointer; font-size: 16px; z-index: 901;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
}
.mem-inject-panel {
  position: fixed; right: 16px; top: 130px; width: 320px; max-height: 60vh;
  background: #fff; border-radius: 12px; box-shadow: 0 4px 20px rgba(0,0,0,0.12);
  overflow-y: auto; z-index: 900;
}
.mi-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid #eee;
}
.mi-header h4 { margin: 0; font-size: 15px; }
.mi-close-btn { background: none; border: none; font-size: 16px; cursor: pointer; color: #999; }
.mi-loading { padding: 24px; text-align: center; color: #999; font-size: 13px; }
.mi-section { padding: 8px 16px; border-bottom: 1px solid #f0f0f0; }
.mi-section h5 { margin: 4px 0 8px; font-size: 13px; color: #666; }
.mi-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 0;
  font-size: 13px; border-bottom: 1px solid #fafafa; flex-wrap: wrap;
}
.mi-layer { color: #666; font-size: 12px; }
.mi-score { color: #999; font-size: 11px; }
.mi-content { flex: 1 1 100%; color: #333; font-size: 12px; margin-top: 4px; }
.mi-profile-card { padding: 8px 0; border-bottom: 1px solid #fafafa; font-size: 12px; }.profile-summary-panel {
  position: fixed; right: 16px; top: 80px; width: 280px; max-height: 60vh;
  background: #fff; border-radius: 12px; box-shadow: 0 4px 20px rgba(0,0,0,0.12);
  overflow-y: auto; z-index: 900;
}
.profile-panel-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid #eee;
}
.profile-panel-header h4 { margin: 0; font-size: 15px; }
.profile-close-btn { background: none; border: none; font-size: 16px; cursor: pointer; color: #999; }
.profile-loading, .profile-empty { padding: 24px; text-align: center; color: #999; font-size: 13px; }
.profile-items { padding: 8px 0; }
.profile-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 16px;
  font-size: 13px; border-bottom: 1px solid #f5f5f5;
}
.profile-cat { color: #999; font-size: 11px; min-width: 48px; }
.profile-name { color: #666; min-width: 48px; }
.profile-val { color: #333; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.profile-conf { font-size: 11px; font-weight: 600; min-width: 36px; text-align: right; }
.conf-high { color: #4caf50; }
.conf-mid { color: #ff9800; }
.conf-low { color: #f44336; }
</style>
