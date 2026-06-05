<template>
  <div class="logs-page">
    <h2 class="page-title">聊天记录</h2>

    <!-- Privacy note -->
    <el-alert type="info" :closable="true" show-icon style="margin-bottom:14px">
      <template #title>聊天记录保存在你自己的设备或服务器上，你可随时删除。导出文件可能包含隐私信息，请妥善保管。</template>
    </el-alert>

    <div class="logs-layout">
      <!-- LEFT: Conversation list -->
      <aside class="conv-sidebar">
        <div class="sidebar-toolbar">
          <el-input v-model="convKeyword" placeholder="搜索..." size="small" clearable @clear="fetchConvs" @keyup.enter="fetchConvs">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
        </div>
        <div class="sidebar-filters">
          <el-select v-model="channelFilter" placeholder="频道" size="small" clearable @change="fetchConvs">
            <el-option v-for="ch in CHANNELS" :key="ch.value" :label="ch.label" :value="ch.value" />
          </el-select>
        </div>

        <div class="conv-list" v-if="convs.length > 0">
          <div
            v-for="c in convs"
            :key="c.id"
            class="conv-item"
            :class="{ active: selectedConvId === c.id }"
            @click="selectConv(c)"
          >
            <div class="ci-title">{{ c.title || (c.channel === "qq" ? "QQ聊天" : c.channel === "wechat" ? "微信聊天" : "新对话") }}</div>
            <div class="ci-meta">
              <el-tag size="small" type="info">{{ channelLabel(c.channel) }}</el-tag>
              <span>{{ c.messageCount || 0 }}条</span>
              <span class="ci-time">{{ fmtShort(c.updatedAt || c.createdAt) }}</span>
            </div>
            <div class="ci-preview" v-if="c.lastMessage">{{ c.lastMessage }}</div>
          </div>
        </div>
        <el-empty v-else description="暂无会话" :image-size="50" />

        <el-pagination
          v-if="convTotal > 20"
          v-model:current-page="convPage"
          :page-size="20"
          :total="convTotal"
          layout="prev,next"
          size="small"
          @current-change="fetchConvs"
          style="margin-top:8px;justify-content:center"
        />
      </aside>

      <!-- RIGHT: Message detail -->
      <main class="msg-detail" v-if="selectedConv">
        <div class="detail-header">
          <div class="dh-info">
            <span class="dh-title">{{ selectedConv.title || (selectedConv.channel === "qq" ? "QQ聊天" : selectedConv.channel === "wechat" ? "微信聊天" : "新对话") }}</span>
            <span class="dh-meta">{{ channelLabel(selectedConv.channel) }} · {{ selectedConv.messageCount || 0 }}条</span>
          </div>
          <div class="dh-actions">
            <el-dropdown trigger="click">
              <el-button size="small">导出<el-icon style="margin-left:4px"><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="exportConv('markdown')">Markdown</el-dropdown-item>
                  <el-dropdown-item @click="exportConv('json')">JSON</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" @click="clearConv" :disabled="!messages.length">清空</el-button>
            <el-button size="small" @click="fetchContextPreview">上下文预览</el-button>
            <el-dropdown trigger="click" style="margin-left:4px">
              <el-button size="small">切换角色<el-icon style="margin-left:4px"><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-for="c in characters"
                    :key="c.id"
                    @click="switchCharacter(c.id)"
                    :class="{ 'is-active': selectedConv?.characterId === c.id }"
                  >
                    {{ c.name }}
                    <el-tag size="small" type="success" v-if="c.isActive" style="margin-left:6px">当前</el-tag>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" @click="genSummary">Summary</el-button>
            <el-select v-model="continueCharId" placeholder="角色" size="small" style="width:120px" v-if="selectedConv?.source?.startsWith('import')" clearable>
              <el-option v-for="c in characters" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
            <el-button size="small" type="primary" @click="continueChat" v-if="selectedConv?.source?.startsWith('import')">Continue Chat</el-button>
            <el-button size="small" type="danger" @click="delConv">删除会话</el-button>
          </div>
        </div>

        <!-- Summary display -->
        <div class="detail-summary" v-if="currentSummary">
          <el-alert type="success" :closable="false">
            <template #title>
              会话摘要
              <el-button text size="small" style="margin-left:8px" @click="viewSummary">查看详情</el-button>
              <el-button text size="small" type="danger" @click="delSummary">删除</el-button>
            </template>
            {{ currentSummary.summaryText?.slice(0, 150) }}{{ currentSummary.summaryText?.length > 150 ? '...' : '' }}
          </el-alert>
        </div>

        <!-- Detail filters -->
        <div class="detail-filters" v-if="messages.length > 0">
          <el-select v-model="roleFilter" placeholder="角色" size="small" clearable style="width:90px">
            <el-option label="用户" value="user" /><el-option label="AI" value="assistant" />
          </el-select>
        </div>

        <!-- Messages -->
        <div class="msg-list" ref="msgListRef">
          <div v-for="m in filteredMessages" :key="m.id" class="msg-item" :class="m.role">
            <div class="mi-header">
              <span class="mi-role">{{ m.role === "user" ? "用户" : "AI" }}</span>
              <span class="mi-time">{{ fmtTime(m.createdAt) }}</span>
              <span class="mi-source" v-if="m.source">{{ m.source }}</span>
              <span class="mi-model" v-if="m.modelName">{{ m.modelName }}</span>
              <el-tag v-if="moodMap[m.id]" size="small" type="warning" class="mi-mood">{{ moodEmoji(moodMap[m.id]) }} {{ moodMap[m.id] }}</el-tag>
              <el-tag v-if="feedbackMap[m.id]?.length" size="small" type="success" class="mi-feedback">{{ feedbackMap[m.id][0].feedbackType }} ({{ feedbackMap[m.id].length }})</el-tag>
              <el-button text size="small" type="danger" class="mi-delete" @click="delMsg(m.id)">删除</el-button>
            </div>
            <div class="mi-content">{{ m.content }}</div>
            <div class="mi-metadata" v-if="devMode && m.metadata">
              <pre>{{ JSON.stringify(m.metadata, null, 2) }}</pre>
            </div>
          </div>
        </div>

        <el-pagination
          v-if="msgTotal > 50"
          v-model:current-page="msgPage"
          :page-size="50"
          :total="msgTotal"
          layout="prev,next"
          size="small"
          @current-change="fetchMessages"
          style="margin-top:8px;justify-content:center"
        />
      </main>

      <!-- No conversation selected -->
      <main class="msg-detail empty" v-else>
        <el-empty description="选择左侧会话查看详情" :image-size="60" />
      </main>
    </div>
  </div>

    <!-- Context Preview Dialog -->
    <el-dialog v-model="ctxPreviewVisible" title="上下文预览" width="700px" top="5vh" :close-on-click-modal="false">
      <div v-if="ctxPreviewLoading" style="text-align:center;padding:40px">
        <el-icon class="is-loading" :size="32"><Loading /></el-icon>
        <p style="margin-top:12px">加载中...</p>
      </div>
      <div v-else-if="ctxPreview" class="ctx-preview">
        <div class="ctxp-section">
          <div class="ctxp-label">角色</div>
          <div class="ctxp-value">{{ ctxPreview.character?.name }} ({{ ctxPreview.character?.identity }})</div>
        </div>
        <div class="ctxp-section">
          <div class="ctxp-label">最近消息</div>
          <div class="ctxp-value">{{ ctxPreview.recentMessageCount }} 条 | 估算总字符: {{ ctxPreview.estimatedChars }}</div>
        </div>
        <div class="ctxp-section" v-if="ctxPreview.usedMemories?.length">
          <div class="ctxp-label">使用记忆 ({{ ctxPreview.usedMemories.length }}条)</div>
          <div class="ctxp-memories">
            <div v-for="(m,i) in ctxPreview.usedMemories" :key="i" class="ctxp-memory-item">
              <el-tag size="small" type="info">{{ m.memoryType }}</el-tag>
              <span>{{ m.key }}: {{ m.value }}</span>
            </div>
          </div>
        </div>
        <div class="ctxp-section" v-if="ctxPreview.usedSummary">
          <div class="ctxp-label">会话摘要</div>
          <div class="ctxp-value ctxp-pre">{{ ctxPreview.usedSummary }}</div>
        </div>
        <div class="ctxp-section" v-if="ctxPreview.usedImportContext">
          <div class="ctxp-label">导入背景</div>
          <div class="ctxp-value ctxp-pre">{{ ctxPreview.usedImportContext }}</div>
        </div>
        <div class="ctxp-section">
          <div class="ctxp-label">System Prompt 预览</div>
          <div class="ctxp-value ctxp-pre ctxp-prompt">{{ ctxPreview.promptPreview }}</div>
        </div>
        <div class="ctxp-section">
          <div class="ctxp-label">最近消息内容</div>
          <div class="ctxp-messages">
            <div v-for="(m,i) in ctxPreview.recentMessages" :key="i" class="ctxp-msg-item" :class="m.role">
              <span class="ctxp-msg-role">{{ m.role === 'user' ? '用户' : 'AI' }}</span>
              <span class="ctxp-msg-content">{{ m.content }}</span>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="ctxPreviewVisible = false">关闭</el-button>
      </template>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Search, ArrowDown, Loading } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const { get, post, put, del } = useApi()

const characters = ref<any[]>([])

const CHANNELS = [
  { label:"Web",value:"web"},{ label:"微信",value:"wechat"},{ label:"QQ",value:"qq"},{ label:"导入",value:"import"},{ label:"测试",value:"test"},
]

// Conversation
const convs = ref<any[]>([])
const convKeyword = ref("")
const continueCharId = ref("")
const channelFilter = ref("")
const convPage = ref(1)
const convTotal = ref(0)
const selectedConv = ref<any>(null)
const selectedConvId = ref("")

// Messages
const messages = ref<any[]>([])
const msgPage = ref(1)
const msgTotal = ref(0)
const roleFilter = ref("")
const msgListRef = ref<HTMLElement>()

const filteredMessages = computed(() => {
  if (!roleFilter.value) return messages.value
  return messages.value.filter(m => m.role === roleFilter.value)
})

function channelLabel(ch: string) { return CHANNELS.find(x=>x.value===ch)?.label || ch }
function fmtShort(d: string) { if(!d)return ""; try{ return new Date(d).toLocaleDateString("zh-CN") } catch{ return d } }
function fmtTime(d: string) { if(!d)return ""; try{ return new Date(d).toLocaleString("zh-CN") } catch{ return d } }

async function fetchConvs() {
  const params: any = { page:convPage.value, pageSize:20 }
  if (convKeyword.value) params.keyword = convKeyword.value
  if (channelFilter.value) params.channel = channelFilter.value
  try {
    const r = await get<any>("/api/chats/conversations", params)
    let items: any[] = r?.items || []
    // Pin wechat conversations at the top
    const wechatItems = items.filter((c: any) => c.channel === 'wechat' || c.source === 'wechat')
    const otherItems = items.filter((c: any) => c.channel !== 'wechat' && c.source !== 'wechat')
    convs.value = [...wechatItems, ...otherItems]
    convTotal.value = r?.total || 0
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
    const r = await get<any>(`/api/chats/conversations/${selectedConvId.value}/messages`, { page:msgPage.value, pageSize:50 })
    messages.value = r?.items || []
    msgTotal.value = r?.total || 0
    nextTick(() => { if (msgListRef.value) msgListRef.value.scrollTop = 0 })
  } catch {}
}

async function delMsg(id: string) {
  await ElMessageBox.confirm("确定删除这条消息？","提示",{type:"warning"})
  await del(`/api/chats/messages/${id}`)
  ElMessage.success("已删除")
  fetchMessages()
  fetchConvs()
}

async function fetchFeedback() {
  if (!selectedConvId.value) return
  const map: Record<string, any[]> = {}
  try {
    // Fetch recent feedback
    const res = await get<any>("/api/messages/feedback/recent", { limit: 200 })
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
    const r = await get<any>(`/api/moods/conversations/${selectedConvId.value}`)
    const items = r?.items || []
    const map: Record<string, string> = {}
    for (const m of items) {
      if (m.messageId) map[m.messageId] = m.moodLabel
    }
    moodMap.value = map
  } catch { moodMap.value = {} }
}

function moodEmoji(label: string): string {
  const map: Record<string, string> = { tired: '馃槨', happy: '馃槈', stressed: '馃槶', sad: '馃槻', angry: '馃槻', confused: '馃槳' }
  return map[label] || ''
}

async function clearConv() {
  await ElMessageBox.confirm("确定清空本会话所有消息？","确认",{type:"warning"})
  await del(`/api/chats/conversations/${selectedConvId.value}/messages`)
  messages.value = []
  ElMessage.success("已清空")
  fetchConvs()
}

async function delConv() {
  await ElMessageBox.confirm("确定删除整个会话及其所有消息？此操作不可撤销。","警告",{type:"warning",confirmButtonText:"删除",confirmButtonClass:"el-button--danger"})
  await del(`/api/chats/conversations/${selectedConvId.value}`)
  selectedConv.value = null
  selectedConvId.value = ""
  messages.value = []
  ElMessage.success("已删除")
  fetchConvs()
}

async function exportConv(format: string) {
  try {
    await post("/api/chats/export", { format, conversationIds: [selectedConvId.value] })
    ElMessage.success("已导出到 data/exports 目录")
  } catch {}
}

// ---- Summary ----
const currentSummary = ref<any>(null)
const moodMap = ref<Record<string, string>>({})
const feedbackMap = ref<Record<string, any[]>>({})
const summaryVisible = ref(false)
const genSummaryLoading = ref(false)

// ---- Context Preview ----
const ctxPreviewVisible = ref(false)
const ctxPreviewLoading = ref(false)
const ctxPreview = ref<any>(null)
const devMode = ref(false)

async function fetchContextPreview() {
  if (!selectedConvId.value) return
  ctxPreviewVisible.value = true
  ctxPreviewLoading.value = true
  ctxPreview.value = null
  try {
    ctxPreview.value = await get<any>(
      `/api/agent/context-preview?conversationId=${selectedConvId.value}`
    )
  } catch (err: any) {
    ElMessage.error(err?.message || 'Failed to load context preview')
    ctxPreviewVisible.value = false
  } finally {
    ctxPreviewLoading.value = false
  }
}

async function fetchSummary() {
  if (!selectedConvId.value) return
  try {
    const r = await get<any>(`/api/chats/conversations/${selectedConvId.value}/summary`)
    currentSummary.value = r?.summaryText ? r : null
  } catch { currentSummary.value = null }
}

async function genSummary() {
  if (!selectedConvId.value) return
  genSummaryLoading.value = true
  try {
    await post(`/api/chats/conversations/${selectedConvId.value}/summary/generate`)
    ElMessage.success("摘要已生成")
    await fetchSummary()
  } catch (err: any) {
    // handled by interceptor
  }
  genSummaryLoading.value = false
}

function viewSummary() {
  summaryVisible.value = true
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
    await put(`/api/chats/conversations/${selectedConvId.value}/character`, { characterId: charId })
    ElMessage.success("角色已切换")
    // Refresh conversation to show new character
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
    const result = await post<any>("/api/web-chat/conversations/from-import", {
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

async function delSummary() {
  await ElMessageBox.confirm("确定删除此会话的摘要?", "确认", { type: "warning" })
  if (!selectedConvId.value) return
  await del(`/api/chats/conversations/${selectedConvId.value}/summary`)
  currentSummary.value = null
  ElMessage.success("已删除")
}

async function loadCharacters() {
  try { characters.value = await get<any[]>("/api/characters") || [] } catch {}
}

onMounted(() => { fetchConvs(); loadCharacters() })
</script>

<style scoped>
.logs-page { display:flex; flex-direction:column; height:100%; }
.page-title { font-size:var(--ac-font-size-lg); font-weight:600; margin-bottom:12px; flex-shrink:0; }

.logs-layout { display:flex; gap:14px; flex:1; overflow:hidden; min-height:0; }

/* Sidebar */
.conv-sidebar { width:280px; flex-shrink:0; display:flex; flex-direction:column; overflow:hidden; }
.sidebar-toolbar { margin-bottom:6px; }
.sidebar-filters { margin-bottom:6px; }
.conv-list { flex:1; overflow-y:auto; }

.conv-item { padding:10px; border-radius:var(--ac-radius-sm); cursor:pointer; transition:background var(--ac-transition-fast); margin-bottom:2px; }
.conv-item:hover { background:var(--ac-color-surface-hover); }
.conv-item.active { background:var(--ac-color-primary-bg); border-left:3px solid var(--ac-color-primary); }
.ci-title { font-size:var(--ac-font-size-sm); font-weight:500; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.ci-meta { display:flex; gap:6px; align-items:center; margin-top:3px; font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); }
.ci-preview { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); margin-top:4px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.ci-time { margin-left:auto; }

/* Detail */
.msg-detail { flex:1; display:flex; flex-direction:column; overflow:hidden; min-width:0; }
.msg-detail.empty { align-items:center; justify-content:center; }

.detail-header { display:flex; align-items:center; justify-content:space-between; padding-bottom:8px; border-bottom:1px solid var(--ac-color-border-light); flex-shrink:0; }
.dh-title { font-weight:600; font-size:var(--ac-font-size-base); }
.dh-meta { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); margin-left:10px; }
.dh-actions { display:flex; gap:6px; }

.detail-filters { padding:6px 0; flex-shrink:0; }

.msg-list { flex:1; overflow-y:auto; }
.msg-item { padding:12px; border-bottom:1px solid var(--ac-color-border-light); }
.msg-item.assistant { background:var(--ac-color-bg-secondary); }
.mi-header { display:flex; align-items:center; gap:8px; margin-bottom:4px; }
.mi-role { font-weight:600; font-size:var(--ac-font-size-xs); }
.mi-time { font-size:10px; color:var(--ac-color-text-muted); }
.mi-source { font-size:10px; color:var(--ac-color-text-placeholder); background:var(--ac-color-surface); padding:0 4px; border-radius:3px; }
.mi-model { font-size:10px; color:var(--ac-color-text-placeholder); }
.mi-delete { margin-left:auto; opacity:0; transition:opacity var(--ac-transition-fast); }
.msg-item:hover .mi-delete { opacity:1; }
.mi-content { font-size:var(--ac-font-size-sm); line-height:1.6; white-space:pre-wrap; word-break:break-word; }
.mi-metadata { margin-top:8px; padding:8px; background:#1e1e1e; color:#d4d4d4; border-radius:4px; }
.mi-metadata pre { margin:0; font-size:11px; font-family:Consolas,monospace; white-space:pre-wrap; word-break:break-all; }

/* Summary */
.detail-summary { margin-bottom: 8px; }
.summary-content { white-space: pre-wrap; line-height: 1.7; font-size: var(--ac-font-size-sm); }
.summary-meta { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 10px; }

@media (max-width:768px) {
  .logs-page {
    max-width: 100%;
    height: 100%;
  }

  .logs-layout {
    flex-direction: column;
  }

  .conv-sidebar {
    width: 100%;
    max-height: 200px;
    flex-shrink: 0;
  }

  .msg-detail {
    flex: 1;
    overflow: hidden;
  }

  .detail-header {
    flex-wrap: wrap;
    gap: 6px;
  }

  .dh-actions {
    width: 100%;
    overflow-x: auto;
    flex-wrap: nowrap;
    gap: 4px;
  }

  .dh-actions .el-button {
    white-space: nowrap;
    font-size: var(--ac-font-size-xs);
  }

  .msg-item {
    padding: 10px;
  }

  .mi-header {
    flex-wrap: wrap;
    gap: 4px;
  }
}
/* Context Preview */
.ctx-preview { display:flex; flex-direction:column; gap:12px; max-height:60vh; overflow-y:auto; }
.ctxp-section {  }
.ctxp-label { font-weight:600; font-size:13px; color:var(--ac-color-text-secondary); margin-bottom:4px; }
.ctxp-value { font-size:13px; color:var(--ac-color-text); }
.ctxp-pre { white-space:pre-wrap; word-break:break-word; font-size:12px; background:var(--ac-color-bg-secondary); padding:8px 10px; border-radius:var(--ac-radius-sm); max-height:150px; overflow-y:auto; }
.ctxp-prompt { max-height:200px; font-family:monospace; font-size:11px; }
.ctxp-memories { display:flex; flex-direction:column; gap:4px; }
.ctxp-memory-item { display:flex; align-items:center; gap:8px; font-size:12px; }
.ctxp-messages { display:flex; flex-direction:column; gap:4px; max-height:200px; overflow-y:auto; }
.ctxp-msg-item { display:flex; gap:8px; padding:4px 8px; border-radius:var(--ac-radius-sm); font-size:12px; }
.ctxp-msg-item.user { background:#e8f4fd; }
.ctxp-msg-item.assistant { background:#f5f5f5; }
.ctxp-msg-role { font-weight:600; flex-shrink:0; min-width:30px; }
.ctxp-msg-content { word-break:break-word; }
</style>

