<template>
  <div class="webchat-page">
    <!-- Model not configured banner -->
    <div v-if="modelMissing" class="config-banner">
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>
          模型未配置 &mdash;
          <router-link to="/model" class="banner-link">去配置模型</router-link>
        </template>
      </el-alert>
    </div>

    <!-- Offline banner -->
    <div v-if="isOffline" class="offline-banner">
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>当前网络不可用，消息暂未发送</template>
      </el-alert>
    </div>

    <!-- Model error banner -->
    <div v-if="modelError" class="error-banner">
      <el-alert type="error" closable show-icon @close="modelError = ''">
        <template #title>{{ modelError }}</template>
      </el-alert>
    </div>

    <!-- Import context banner -->
    <div v-if="importContext" class="import-banner">
      <el-alert type="success" :closable="true" show-icon @close="importContext = null">
        <template #title>
          Chatting based on imported records
          <span class="import-badge">import</span>
        </template>
        <template #default v-if="showImportDetail">
          <div class="import-detail">
            <p v-if="importContext.summary" class="import-summary">{{ importContext.summary }}</p>
            <p v-if="importContext.memoryCount" class="import-memories">
              {{ importContext.memoryCount }} confirmed memories available
            </p>
          </div>
        </template>
      </el-alert>
    </div>

    <!-- Summary banner -->
    <div v-if="convSummary" class="summary-banner">
      <el-alert type="info" :closable="false">
        <template #title>
          会话摘要
          <el-button text size="small" style="margin-left:8px" @click="showSummary = !showSummary">
            {{ showSummary ? '收起' : '展开' }}
          </el-button>
        </template>
        <template #default v-if="showSummary">
          <div class="summary-text">{{ convSummary }}</div>
        </template>
      </el-alert>
    </div>

    <!-- Chat header -->
    <header class="chat-header">
      <el-button :icon="Menu" text circle size="small" class="menu-btn" @click="showDrawer = true" />
      <div class="header-info">
        <span class="header-char-name">{{ charName || "选择角色" }}</span>
        <span class="header-char-desc" v-if="charName">{{ charIdentity || '暂无角色描述' }}</span>
        <span class="header-conv-title" v-if="convTitle">{{ convTitle }}</span>
      </div>
      <div class="header-style-select">
        <el-dropdown trigger="click" @command="(v: string) => replyStyle = v">
          <span class="style-trigger">
            {{ styleLabel(replyStyle) }}
            <el-icon class="style-arrow"><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="s in REPLY_STYLES"
                :key="s.value"
                :command="s.value"
                :class="{ 'is-active': replyStyle === s.value }"
              >
                <span>{{ s.label }}</span>
                <el-icon v-if="replyStyle === s.value" class="style-check"><Check /></el-icon>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
      <div class="header-actions">
        <el-dropdown trigger="click">
          <el-button text circle size="small" :icon="MoreFilled" />
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="handleRegenerate" :disabled="!canRegenerate">
                <el-icon><Refresh /></el-icon> 重新生成回复
              </el-dropdown-item>
              <el-dropdown-item @click="handleClear" :disabled="messages.length === 0">
                <el-icon><Delete /></el-icon> 清空会话
              </el-dropdown-item>
              <el-dropdown-item divided @click="handleViewMemories" v-if="convId">
                <el-icon><Collection /></el-icon> 查看相关记忆
              </el-dropdown-item>
              <el-dropdown-item @click="showCharPicker = true">
                <el-icon><Switch /></el-icon> 切换角色
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <!-- Messages area -->
    <div class="messages-area" ref="msgArea" @scroll="onScroll" @wheel="onWheel" @touchstart="onMsgTouchStart" @touchmove="onMsgTouchMove" @touchend="onMsgTouchEnd">
      <!-- Pull-down to load history indicator (mobile) -->
      <div class="pull-indicator" :class="{ pulling: isPulling, ready: pullReady }">
        <el-icon :size="18" class="pull-icon" :class="{ spin: pullLoading }">
          <Loading v-if="pullLoading" />
          <ArrowDown v-else />
        </el-icon>
        <span>{{ pullText }}</span>
      </div>

      <!-- Empty state -->
      <div v-if="messages.length === 0 && !sending" class="empty-chat">
        <div class="empty-icon">
          <el-icon :size="48"><ChatDotRound /></el-icon>
        </div>
        <p class="empty-text">你好，我是 {{ charName || "AI 陪伴角色" }}</p>
        <p class="empty-hint">随时可以和我聊聊天，我在这里陪你。</p>
      </div>

      <!-- Message bubbles -->
      <ChatBubble
        v-for="msg in messages"
        :key="msg.id"
        :message="msg"
        :char-name="charName"
        :char-avatar="charAvatar"
        :is-streaming="msg.id === 'streaming'"
        :status="msg.status"
        @retry="handleRetry"
      />



      <!-- Scroll-to-bottom button -->
      <transition name="fade">
        <el-button
          v-if="showScrollBtn"
          :icon="ArrowDown"
          circle
          size="small"
          class="scroll-btn"
          @click="scrollToBottom(true)"
        />
      </transition>
    </div>

    <!-- Chat input -->
    <ChatInput
      ref="inputRef"
      :disabled="modelMissing"
      :sending="sending"
      @send="handleSend"
        @image="onImageAttached"
        @removeImage="onImageRemoved"
      @stop="handleStop"
    />

    <!-- Character drawer -->
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
    <!-- Character picker dialog -->
    <el-dialog v-model="showCharPicker" title="切换角色" width="400px">
      <div class="char-list">
        <div
          v-for="c in characters"
          :key="c.id"
          class="char-option"
          :class="{ active: c.id === characterId }"
          @click="handleSwitchChar(c)"
        >
          <el-avatar :size="36">{{ c.name?.charAt(0) }}</el-avatar>
          <div class="char-option-info">
            <div class="char-option-name">{{ c.name }}</div>
            <div class="char-option-desc">{{ c.identity || c.personality }}</div>
          </div>
          <el-tag v-if="!!c.isDefault" size="small" type="success" effect="plain">默认角色</el-tag>
        </div>
      </div>
    </el-dialog>

    <!-- Memory drawer -->
    <el-drawer v-model="showMemories" title="相关记忆" direction="rtl" size="360px">
      <div v-if="memories.length > 0">
        <div v-for="m in memories" :key="m.key" class="memory-card">
          <el-tag size="small" type="info">{{ typeLabel(m.memoryType) }}</el-tag>
          <div class="memory-key">{{ m.key }}</div>
          <div class="memory-value">{{ m.value }}</div>
        </div>
      </div>
      <el-empty v-else description="暂无相关记忆" :image-size="60" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch, inject } from "vue"
import { useRoute, useRouter } from "vue-router"
import {
  Menu, MoreFilled, Refresh, Delete, Collection, Switch,
  ChatDotRound, ArrowDown, Check, Promotion, Loading,
} from "@element-plus/icons-vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useApi, isLoggedIn, getToken } from "../../composables/useApi"
import { useCachedApi } from "../../composables/useCachedApi"
import ChatBubble from "../../components/ChatBubble.vue"
import ChatInput from "../../components/ChatInput.vue"
import ConversationDrawer from "../../components/ConversationDrawer.vue"

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
const sending = ref(false)

const modelMissing = ref(false)

const showDrawer = ref(false)
const showCharPicker = ref(false)
const showMemories = ref(false)
const showScrollBtn = ref(false)
const importContext = ref<any>(null)
const showImportDetail = ref(false)

const msgArea = ref<HTMLElement>()
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
const REPLY_STYLES = [
  { value: "natural", label: "默认自然", icon: "ChatDotRound" },
  { value: "shorter", label: "更简短", icon: "Minus" },
  { value: "gentler", label: "更温柔", icon: "Sunny" },
  { value: "humorous", label: "更幽默", icon: "Lollipop" },
  { value: "rational", label: "更理性", icon: "Document" },
  { value: "quiet_listening", label: "安静倾听", icon: "Headset" },
  { value: "encouraging", label: "鼓励一点", icon: "TrendCharts" },
]
const styleLabel = (v: string) => REPLY_STYLES.find(s => s.value === v)?.label || "Natural"

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
    })
    proactiveSSE.onerror = () => { proactiveSSE?.close(); setTimeout(connectProactiveSSE, 5000) }
  } catch { setTimeout(connectProactiveSSE, 5000) }
}

async function fetchWechatMsgCount() {
  try {
    const r = await get<any>("/api/wechat/status")
    const status = r?.data || r
    wechatMsgCount.value = status?.messageCount || 0
    wechatOnline.value = status?.status === "connected" || status?.accountId != null
  } catch {}
}

async function fetchQQStatus() {
  try {
    const r = await get<any>("/api/qq/status")
    const data = r?.data || r
    qqMsgCount.value = data?.messageCount || 0
    qqOnline.value = data?.qqOnline || data?.status === "online"
  } catch {}
}

onMounted(async () => {
  fetchWechatMsgCount()
  fetchQQStatus()
  setInterval(fetchWechatMsgCount, 15000)
  setInterval(fetchQQStatus, 15000)
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
      const convs = await get<any>("/api/web-chat/conversations", { pageSize: 100 })
      const all = convs?.conversations || convs?.items || []
      const charConvs = all.filter((x: any) => (x.characterId || x.character_id) === characterId.value)
      if (charConvs.length > 0) {
        charConvs.sort((a: any, b: any) => new Date(b.updatedAt || b.updated_at || 0).getTime() - new Date(a.updatedAt || a.updated_at || 0).getTime())
        dedicatedConvId = charConvs[0].id
      }
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
    conversations.value = r?.conversations || r?.items || []
  } catch { conversations.value = [] }
}

async function fetchConvSummary() {
  if (!convId.value) return
  try {
    const r = await get<any>(`/api/chats/conversations/${convId.value}/summary`)
    convSummary.value = r?.summaryText || ""
  } catch { convSummary.value = "" }
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
    const r = await get<any>(url)
    if (version !== messagesVersion) return
    const items = (r?.messages || r?.items || [])
    if (items.length) {
      messages.value = items.map((m: any) => {
        if (m.imageUrl && m.content === "[图片]") return { ...m, content: "" }
        return m
      })
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





async function handleImageSend(text: string, imageBase64: string) {
  if (sending.value) return
  currentImageBase64.value = null
  currentImageFile.value = null
  const hasUserText = !!(text && text.trim())
  const sendText = hasUserText ? text : "[图片]"
  pendingImageBase64.value = imageBase64
  await doActualSend(sendText)
}

async function handleSend(text: string, imageBase64?: string) {
  if (imageBase64 || currentImageBase64.value) {
    handleImageSend(text, imageBase64 || currentImageBase64.value || "")
    return
  }
  if (sending.value) return
  doActualSend(text)
}

// doActualSend 是原来的 handleSend 逻辑
async function doActualSend(text: string) {
  if (sending.value) return

  // 断开 SSE 轮询，防止与本 SSE 流重复推送消息
  disconnectSSE()

  // 立即插入用户消息，不等后端返回
  const userMsgLocalId = "user-" + Date.now()
  const imgUrl = pendingImageBase64.value
  pendingImageBase64.value = null
  const displayContent = (imgUrl && text === "[图片]") ? "" : text
  const sendContent = (imgUrl && !text.trim()) ? "[图片]" : text
  messages.value.push({ id: userMsgLocalId, role: "user", content: displayContent, imageUrl: imgUrl || undefined, status: "sent", conversationId: convId.value, createdAt: new Date().toISOString() })
  
  scrollToBottom(true)

  sending.value = true
  modelError.value = ""

  try {
    const token = localStorage.getItem("ai-companion-token") || ""
    const res = await fetch("/api/web-chat/send-stream", {
      method: "POST",
      headers: { "Content-Type": "application/json", "Authorization": `Bearer ${token}` },
      body: JSON.stringify({ conversationId: convId.value || undefined, characterId: characterId.value || undefined, message: sendContent, imageUrl: imgUrl || "" }),
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
              const targetMsg = [...messages.value].reverse().find((m: any) => m.id === data.messageId || m.status === "streaming")
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
      const tIdx = -1
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
      const el = msgArea.value
      if (!el) return
      el.scrollTo({
        top: el.scrollHeight,
        behavior: smooth ? "smooth" : "auto",
      })
    })
  })
}

function onScroll() {
  const el = msgArea.value
  if (!el) return
  const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  const threshold = 200
  showScrollBtn.value = distFromBottom > threshold
  // User is scrolling up if not near bottom
  userScrolledUp.value = distFromBottom > 100

  // Load older messages when scrolled to top
  if (el.scrollTop <= 20 && hasMoreHistory.value && !isLoadingHistory.value && convId.value) {
    loadOlderMessages()
  }
}



function onWheel(e: WheelEvent) {
  const el = msgArea.value
  if (!el) return
  if (e.deltaY >= 0) return
  const noOverflow = el.scrollHeight <= el.clientHeight
  const atTop = el.scrollTop <= 0
  if ((noOverflow || atTop) && hasMoreHistory.value && !isLoadingHistory.value && convId.value) {
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
      const el = msgArea.value
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
  const el = msgArea.value
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
function typeLabel(type: string): string {
  const labels: Record<string, string> = {
    preference: "偏好",
    event: "事件",
    habit: "习惯",
    nickname: "昵称",
    relationship: "关系",
    custom: "其他",
  }
  return labels[type] || type
}
</script>

<style scoped>
.webchat-page { height: 100%; display: flex; flex-direction: column;
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}

/* Config banner */
.config-banner {
  flex-shrink: 0;
  margin: 0 0 8px;
}

/* Offline & error banners */
.offline-banner,
.error-banner {
  flex-shrink: 0;
  margin: 0 0 8px;
}

.banner-link {
  font-weight: 600;
  text-decoration: underline;
  color: var(--el-color-warning-dark-2);
}

/* Chat header */
.chat-header {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0 8px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.menu-btn {
  flex-shrink: 0;
}
.header-info {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  flex: 1;
  min-width: 0;
  position: relative;
}

.header-char-name {
  display: block;
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  color: var(--ac-color-text);
}

.header-char-desc {
  display: block;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-conv-title {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  white-space: nowrap;
}

.

.header-actions {
  flex-shrink: 0;
}

/* Messages area */
.messages-area {
  align-self: center;
  width: 100%;
  margin: 0 auto;
  flex: 1 1 0;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 16px;
  position: relative;
}



/* Empty state */
.empty-chat {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  padding: 40px 20px;
}

.empty-icon {
  color: var(--ac-color-text-muted);
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-text {
  font-size: var(--ac-font-size-lg);
  color: var(--ac-color-text-secondary);
  margin-bottom: 8px;
}

.empty-hint {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  max-width: 280px;
}

/* Typing indicator */
.typing-bubble {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 0;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

.typing-dots {
  display: flex;
  gap: 4px;
}

.typing-dots span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ac-color-text-muted);
  animation: dotPulse 1.4s ease-in-out infinite both;
}

.typing-dots span:nth-child(2) { animation-delay: 0.2s; }
.typing-dots span:nth-child(3) { animation-delay: 0.4s; }

@keyframes dotPulse {
  0%, 80%, 100% { opacity: 0.3; transform: scale(0.8); }
  40% { opacity: 1; transform: scale(1); }
}

.typing-text {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}

/* Scroll-to-bottom button */
.scroll-btn {
  position: sticky;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  box-shadow: var(--ac-shadow-md);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--ac-transition-fast);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Character picker */
.char-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 400px;
  overflow-y: auto;
}

.char-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
}

.char-option:hover {
  background: var(--ac-color-surface-hover);
}

.char-option.active {
  background: var(--ac-color-primary-bg);
}

.char-option-info {
  flex: 1;
  min-width: 0;
}

.char-option-name {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
}

.char-option-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.char-active-icon {
  color: var(--ac-color-success);
}

/* Memory cards */
.memory-card {
  padding: 10px;
  margin-bottom: 8px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
}

.memory-key {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  margin: 4px 0;
}

.memory-value {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
}

/* Summary banner */
.summary-banner {
  margin-bottom: 6px;
  flex-shrink: 0;
}

.summary-text {
  white-space: pre-wrap;
  font-size: var(--ac-font-size-xs);
  line-height: 1.6;
  color: var(--ac-color-text-secondary);
  margin-top: 4px;
}

.header-style-select {
  flex-shrink: 0;
  margin-right: 4px;
}

.style-trigger {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-secondary);
  cursor: pointer;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  border: 1px solid var(--ac-color-border-light);
  transition: all var(--ac-transition-fast);
}

.style-trigger:hover {
  color: var(--ac-color-primary);
  border-color: var(--ac-color-primary);
}

.style-arrow {
  font-size: 10px;
  transition: transform var(--ac-transition-fast);
}

.style-check {
  margin-left: 8px;
  color: var(--ac-color-primary);
}

/* Mobile */
/* Pull-down indicator */
.pull-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 0;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  transition: opacity var(--ac-transition-fast);
  opacity: 0;
  height: 36px;
}

.pull-indicator.pulling { opacity: 0.6; }
.pull-indicator.ready { opacity: 1; color: var(--ac-color-primary); }

.pull-icon.spin { animation: spin 0.8s linear infinite; }

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .webchat-page { height: 100%; display: flex; flex-direction: column;
    max-width: 100%;
  }

  .chat-header {
    padding: 4px 0 6px;
  }

  .messages-area {
  align-self: center;
  width: 100%;
    padding: 12px 12px;
  }
}
</style>

