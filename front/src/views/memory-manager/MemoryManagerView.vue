<template>
  <div class="mem-page">
    <h2 class="page-title">记忆管理</h2>

    <div class="pipeline-bar" v-if="pipelineStatus">
      <span class="pl-label">管线状态:</span>
      <template v-for="l in pipelineStatus.layers" :key="l.layer">
        <el-tooltip :content="l.name + ': ' + l.status + ' (' + l.durationMs + 'ms)'" placement="top">
          <span class="pl-dot" :class="'pl-' + l.status" :style="{ backgroundColor: l.status === 'completed' ? '#67c23a' : l.status === 'skipped' ? '#c0c4cc' : '#f56c6c' }"></span>
        </el-tooltip>
      </template>
      <span class="pl-time" v-if="pipelineStatus.endedAt">{{ fmtDate(pipelineStatus.endedAt) }}</span>
    </div>

    <div class="mem-toolbar">
      <el-input v-model="keyword" placeholder="搜索关键词..." size="small" style="width:180px" clearable @clear="fetchList" @keyup.enter="fetchList">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-select v-model="typeFilter" placeholder="类型" size="small" style="width:110px" clearable @change="fetchList">
        <el-option v-for="t in TYPES" :key="t.value" :label="t.label" :value="t.value" />
      </el-select>
      <el-select v-model="sourceFilter" placeholder="来源" size="small" style="width:110px" clearable @change="fetchList">
        <el-option v-for="s in SOURCES" :key="s.value" :label="s.label" :value="s.value" />
      </el-select>
      <el-select v-model="scopeTypeFilter" placeholder="范围" size="small" style="width:140px" clearable @change="fetchList">
        <el-option v-for="s in SCOPE_TYPES" :key="s.value" :label="s.label" :value="s.value" />
      </el-select>
      <el-select v-model="characterFilter" placeholder="角色" size="small" style="width:130px" clearable @change="fetchList">
        <el-option label="全部角色" value="" />
        <el-option v-for="ch in characters" :key="ch.id" :label="ch.name" :value="ch.id" />
      </el-select>
      <el-select v-model="sortBy" size="small" style="width:120px" @change="fetchList">
        <el-option label="重要度降序" value="importance_desc" />
        <el-option label="重要度升序" value="importance_asc" />
        <el-option label="时间降序" value="time_desc" />
        <el-option label="时间升序" value="time_asc" />
      </el-select>
      <el-button size="small" @click="showGenerateDialog = true">生成候选</el-button>
      <div class="toolbar-spacer"></div>
      <el-button size="small" type="primary" :icon="Plus" @click="showCreate">新建</el-button>
      <el-button size="small" @click="handleExport">导出</el-button>
      <el-button size="small" type="success" @click="batchVerify" :disabled="selectedIds.length===0">批量确认</el-button>
      <el-button size="small" type="warning" @click="batchSetImportant" :disabled="selectedIds.length===0">标为重要</el-button>
      <el-button size="small" type="danger" @click="batchDelete" :disabled="selectedIds.length===0">批量删除</el-button>
      <el-button size="small" type="danger" plain @click="handleClearAll" :disabled="total === 0">清空全部</el-button>
      <router-link to="/graph"><el-button size="small" type="info" plain>图谱</el-button></router-link>
    </div>

    <div class="global-search-bar">
      <el-input v-model="globalQuery" placeholder="全局搜索所有记忆类型..." size="small" clearable @clear="clearGlobalSearch" @keyup.enter="doGlobalSearch">
        <template #prefix><el-icon><Search /></el-icon></template>
        <template #append>
          <el-button size="small" @click="doGlobalSearch" :loading="globalSearching">搜索</el-button>
        </template>
      </el-input>
      <el-button size="small" @click="showGlobalResults = !showGlobalResults" v-if="globalSearched">
        {{ showGlobalResults ? "隐藏结果" : "显示结果(" + globalResultCount + ")" }}
      </el-button>
    </div>

    <div v-if="showGlobalResults && globalSearched" class="global-results">
      <div v-if="globalResults.memories.length" class="gr-section">
        <h4><el-tooltip content="按角色独立，切换角色后数据不同" placement="top"><span class="gr-label">结构化记忆</span></el-tooltip> ({{ globalResults.memories.length }})</h4>
        <div v-for="m in globalResults.memories" :key="m.id" class="gr-item">
          <el-tag size="small">{{ typeLabel(m.memoryType) }}</el-tag>
          <span>{{ m.key }}: {{ m.value }}</span>
          <span class="gr-score" v-if="m.score">({{ (m.score * 100).toFixed(0) }}%)</span>
        </div>
      </div>
      <div v-if="globalResults.profiles.length" class="gr-section">
        <h4><el-tooltip content="按用户共享，所有角色共用同一份画像" placement="top"><span class="gr-label">用户画像</span></el-tooltip> ({{ globalResults.profiles.length }})</h4>
        <div v-for="p in globalResults.profiles" :key="p.id" class="gr-item">{{ p.attributeName }}: {{ p.attributeValue }}</div>
      </div>
      <div v-if="globalResults.episodics.length" class="gr-section">
        <h4><el-tooltip content="按用户共享，跨角色的对话日记" placement="top"><span class="gr-label">情景记忆</span></el-tooltip> ({{ globalResults.episodics.length }})</h4>
        <div v-for="e in globalResults.episodics" :key="e.id" class="gr-item">{{ e.title }}</div>
      </div>
      <div v-if="globalResults.worldBooks.length" class="gr-section">
        <h4><el-tooltip content="全局共享，所有角色通用知识规则" placement="top"><span class="gr-label">世界书</span></el-tooltip> ({{ globalResults.worldBooks.length }})</h4>
        <div v-for="w in globalResults.worldBooks" :key="w.id" class="gr-item">{{ w.matchPattern }}</div>
      </div>
      <el-empty v-if="globalResultCount === 0" description="未找到相关结果" :image-size="40" />
    </div>

    <el-tabs v-model="activeTab" class="mem-tabs">
      <el-tab-pane label="全部记忆" name="list">
        <MemoryStatusPanel />
        <el-alert v-if="candidates.length > 0" type="warning" :closable="false" show-icon style="margin:10px 0">
          <template #title>
            有 {{ candidates.length }} 条候选记忆等待确认
            <el-button type="warning" size="small" link @click="showCandidates = !showCandidates">{{ showCandidates ? "收起" : "查看" }}</el-button>
          </template>
        </el-alert>
        <CandidateMemoryPanel :candidates="candidates" :show-candidates="showCandidates" @confirm="confirmCandidate" @edit="editCandidate" @delete-item="deleteCandidateItem" @toggle-show="showCandidates = !showCandidates" />
        <MemoryTable :memories="memories" :selected-ids="selectedIds" :page="page" :page-size="pageSize" :total="total" :characters="characters" @selection-change="handleSelectionChange" @edit="showEdit" @delete="delMem" @toggle-scope="toggleScope" @page-change="page = $event; fetchList()" />
      </el-tab-pane>

      <el-tab-pane label="检索分析" name="analysis">
        <RetrievalAnalysisPanel :retrieval-stats="retrievalStats" :retrieval-logs="retrievalLogs" :halflife-episodic="halflifeEpisodic" :halflife-profile="halflifeProfile" :halflife-fact="halflifeFact" :halflife-worldbook="halflifeWorldbook" />
      </el-tab-pane>
    </el-tabs>

    <MemorySearchDialog v-model="searchDialogVisible" />
    <MemoryEditorDialog v-model="dialogVisible" :editing="editing" :editing-id="editingId" :character-id="injectedCharacterId || ''" @memory-saved="fetchList" />
    <CandidateEditorDialog v-model="editCandidateVisible" @candidate-updated="loadCandidates" />
    <ConflictResolverDialog v-model="conflictVisible" @conflict-resolved="fetchList" />
    <CandidateGenerateDialog v-model="showGenerateDialog" :conversation-list="conversationList" :candidates="candidates" @update:candidates="candidates = $event" @show-candidates="showCandidates = true" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, inject, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Search, Plus } from "@element-plus/icons-vue"
import CandidateGenerateDialog from "./components/CandidateGenerateDialog.vue"
import MemorySearchDialog from "./components/MemorySearchDialog.vue"
import CandidateEditorDialog from "./components/CandidateEditorDialog.vue"
import MemoryEditorDialog from "./components/MemoryEditorDialog.vue"
import ConflictResolverDialog from "./components/ConflictResolverDialog.vue"
import MemoryStatusPanel from "./components/MemoryStatusPanel.vue"
import CandidateMemoryPanel from "./components/CandidateMemoryPanel.vue"
import MemoryTable from "./components/MemoryTable.vue"
import RetrievalAnalysisPanel from "./components/RetrievalAnalysisPanel.vue"
import { useMemoryDiagnostics } from "./composables/useMemoryDiagnostics"
import { useMemoryCandidates } from "./composables/useMemoryCandidates"
import { useMemoryList } from "./composables/useMemoryList"
import { useApi } from "../../composables/useApi"

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))

const { get, post, put, del } = useApi()

// Vector memory state
const vectorStatus = ref<any>(null)
const pipelineStatus = ref<any>(null)
const rebuilding = ref(false)
const selectedIds = ref<string[]>([])
const tableRef = ref<any>(null)
const searchDialogVisible = ref(false)
const searchQuery = ref("")
const searchResults = ref<any[]>([])
const searched = ref(false)

const TYPES = [
  { label:"偏好",value:"preference"},{ label:"事件",value:"event"},{ label:"习惯",value:"habit"},
  { label:"昵称",value:"nickname"},{ label:"关系",value:"relationship"},{ label:"其他",value:"custom"},
]
const SOURCES = [
  { label:"手动",value:"manual"},{ label:"摘要",value:"summary"},{ label:"提取",value:"extracted"},{ label:"导入",value:"import"},
]
const SCOPE_TYPES = [
  { label:"用户全局", value:"user_global" },
  { label:"用户-角色", value:"user_character" },
  { label:"角色自身", value:"character_self" },
  { label:"世界", value:"world" },
]
const SENSITIVITY_OPTIONS = [
  { label:"普通", value:"normal" },
  { label:"敏感", value:"sensitive" },
  { label:"高敏感", value:"high" },
]

const memories = ref<any[]>([])
const candidates = ref<any[]>([])
const keyword = ref("")
const typeFilter = ref("")
const sourceFilter = ref("")
const scopeTypeFilter = ref("")
const characterFilter = ref(injectedCharacterId?.value || "")
const characters = ref<any[]>([])
const sortBy = ref("importance_desc")
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dialogVisible = ref(false)
const editing = ref(false)
const editingId = ref("")
const saving = ref(false)
const showCandidates = ref(false)
const conversationList = ref<any[]>([])
const form = reactive({
  key:"",
  value:"",
  memoryType:"custom",
  importance:5,
  characterId:"",
  scope:"character",
  scopeType:"user_character",
  source:"manual",
  sensitivity:"normal",
  allowContextUse:true,
  allowProactiveMention:false,
  requiresConfirmation:false,
})
const editCandidateVisible = ref(false)
const conflictVisible = ref(false)
const showGenerateDialog = ref(false)
const editForm = reactive({ key: "", value: "", content: "", memoryType: "custom", importance: 5, candidateId: "", scope: "character" })
const conflictNewType = ref("")
const conflictNewContent = ref("")
const conflictList = ref<any[]>([])
const resolveAction = ref("")
const generating = ref(false)
const generateConvId = ref("")
const activeTab = ref("list")
const retrievalStats = ref({ totalCount: 0 })
const retrievalLogs = ref<any[]>([])
const halflifeEpisodic = ref(30)
const halflifeProfile = ref(90)
const halflifeFact = ref(180)
const halflifeWorldbook = ref(365)
const globalQuery = ref("")
const globalSearching = ref(false)
const globalSearched = ref(false)
const showGlobalResults = ref(false)
const globalResults = ref({ memories: [] as any[], profiles: [] as any[], episodics: [] as any[], worldBooks: [] as any[] })
const globalResultCount = ref(0)

async function doResolveConflict() {
  if (!resolveAction.value) { ElMessage.warning("请选择处理方式"); return }
  try {
    await post("/api/memories/resolve-conflict", {
      action: resolveAction.value,
      newKey: "", characterId: injectedCharacterId?.value || "",
      newValue: conflictNewContent.value,
      newType: conflictNewType.value,
      importance: 5,
      conflictId: conflictList.value[0]?.id || "",
    })
    ElMessage.success("冲突已解决")
    conflictVisible.value = false
    fetchList()
  } catch (err: any) {
    ElMessage.error(err?.message || "处理失败")
  }
}

async function generateCandidates() {
  if (!generateConvId.value) { ElMessage.warning("请选择会话"); return }
  generating.value = true
  try {
    const res: any = await post("/api/memory-candidates/generate", { conversationId: generateConvId.value })
    candidates.value = res?.candidates || []
    if (candidates.value.length > 0) {
      showGenerateDialog.value = false
      showCandidates.value = true
      ElMessage.success("已提取 " + candidates.value.length + " 条候选记忆")
    } else {
      ElMessage.info("未提取到候选记忆")
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "提取失败")
  }
  generating.value = false
}

function typeLabel(t: string) { return TYPES.find(x=>x.value===t)?.label || t }

function charName(cid: string) {
  const ch = characters.value.find((c: any) => String(c.id) === String(cid))
  return ch ? "[" + ch.name + "]" : ""
}
function sourceLabel(s: string) { return SOURCES.find(x=>x.value===s)?.label || s }
function importanceColor(v: number) { return v>=8?'#c85a5a':v>=5?'#c8924a':'#5b7fa5' }
function isExpired(expiresAt?: string) { return !!expiresAt && new Date(expiresAt).getTime() < Date.now() }
function legacyScopeToScopeType(scope: string) { return scope === "user" ? "user_global" : "user_character" }
function rowScopeType(row: any) { return row.scopeType || row.scope_type || legacyScopeToScopeType(row.scope || "character") }
function scopeTypeToScope(scopeType: string) { return scopeType === "user_global" ? "user" : scopeType === "world" ? "world" : "character" }
function scopeTypeLabel(row: any) { return SCOPE_TYPES.find(x => x.value === rowScopeType(row))?.label || rowScopeType(row) }
function rowSensitivity(row: any) { return row.sensitivity || row.sensitivityLevel || row.sensitivity_level || "normal" }
function sensitivityLabel(value: string) { return SENSITIVITY_OPTIONS.find(x => x.value === value)?.label || value }
function sensitivityTagType(value: string) { return value === "high" ? "danger" : value === "sensitive" ? "warning" : "info" }
function readBooleanFlag(row: any, keys: string[], defaultValue: boolean) {
  for (const key of keys) {
    const value = row?.[key]
    if (typeof value === "boolean") return value
    if (typeof value === "number") return value !== 0
    if (typeof value === "string") {
      const normalized = value.trim().toLowerCase()
      if (["true", "1", "yes", "y", "on"].includes(normalized)) return true
      if (["false", "0", "no", "n", "off"].includes(normalized)) return false
    }
  }
  return defaultValue
}
function rowAllowContextUse(row: any) { return readBooleanFlag(row, ["allowContextUse", "allow_context_use"], true) }
function rowAllowProactiveMention(row: any) { return readBooleanFlag(row, ["allowProactiveMention", "allow_proactive_mention"], false) }
function rowRequiresConfirmation(row: any) { return readBooleanFlag(row, ["requiresConfirmation", "requires_confirmation"], false) }
function scopeTypeTagType(row: any) {
  const scopeType = rowScopeType(row)
  return scopeType === "user_global" || scopeType === "world" ? "success" : scopeType === "character_self" ? "warning" : "info"
}

async function fetchList() {
  const params: any = { page:page.value, pageSize: pageSize.value }
  if (characterFilter.value) params.characterId = characterFilter.value
  if (keyword.value) params.keyword = keyword.value
  if (typeFilter.value) params.memoryType = typeFilter.value
  if (sourceFilter.value) params.source = sourceFilter.value
  if (scopeTypeFilter.value) params.scopeType = scopeTypeFilter.value
  if (sortBy.value) params.sortBy = sortBy.value
  try {
    const r = await get<any>("/api/memories", params)
    memories.value = r?.items || []
    total.value = r?.total || 0
  } catch {}
}

function showCreate() {
  editing.value = false; editingId.value = ""
  form.key=""
  form.value=""
  form.memoryType="custom"
  form.importance=5
  form.characterId=injectedCharacterId?.value||""
  form.source="manual"
  form.scope="character"
  form.scopeType="user_character"
  form.sensitivity="normal"
  form.allowContextUse=true
  form.allowProactiveMention=false
  form.requiresConfirmation=false
  dialogVisible.value = true
}

function showEdit(row: any) {
  editing.value = true; editingId.value = row.id
  form.key=row.key
  form.value=row.value
  form.memoryType=row.memoryType
  form.importance=row.importance
  form.characterId=row.characterId||""
  form.scope=row.scope||"character"
  form.scopeType=rowScopeType(row)
  form.source=row.source||"manual"
  form.sensitivity=rowSensitivity(row)
  form.allowContextUse=rowAllowContextUse(row)
  form.allowProactiveMention=rowAllowProactiveMention(row)
  form.requiresConfirmation=rowRequiresConfirmation(row)
  dialogVisible.value = true
}

async function toggleScope(row: any) {
  const newScopeType = rowScopeType(row) === "user_global" ? "user_character" : "user_global"
  const newScope = newScopeType === "user_global" ? "user" : "character"
  try {
    await put(`/api/memories/${row.id}`, { scope: newScope, scopeType: newScopeType })
    row.scope = newScope
    row.scopeType = newScopeType
    ElMessage.success(newScopeType === "user_global" ? "已升级为全局记忆" : "已降级为角色记忆")
  } catch {}
}

async function saveMem() {
  saving.value = true
  try {
    const payload = {
      ...form,
      source: form.source || "manual",
      scope: scopeTypeToScope(form.scopeType),
      allowContextUse: !!form.allowContextUse,
      allowProactiveMention: !!form.allowProactiveMention,
      requiresConfirmation: !!form.requiresConfirmation,
    }
    if (editing.value) await put(`/api/memories/${editingId.value}`, payload)
    else await post("/api/memories", payload)
    dialogVisible.value = false
    ElMessage.success(editing.value?"保存成功":"新建成功")
    fetchList()
  } catch (err: any) { ElMessage.error(err?.message || "保存失败") }
  saving.value = false
}

async function delMem(id: string) {
  await ElMessageBox.confirm("确定删除？","提示",{type:"warning"})
  await del(`/api/memories/${id}`)
  ElMessage.success("已删除")
  fetchList()
}

function handleSelectionChange(rows: any[]) { selectedIds.value = rows.map(r => r.id) }

async function batchVerify() {
  if (selectedIds.value.length === 0) return
  try { await post("/api/memories/batch-verify", { ids: selectedIds.value, status: "user_verified" }); ElMessage.success("批量确认成功"); selectedIds.value = []; fetchList() } catch { ElMessage.error("操作失败") }
}

async function batchSetImportant() {
  if (selectedIds.value.length === 0) return
  try { await post("/api/memories/batch-importance", { ids: selectedIds.value, importance: 10 }); ElMessage.success("已标为重要"); selectedIds.value = []; fetchList() } catch { ElMessage.error("操作失败") }
}

async function batchDelete() {
  if (selectedIds.value.length === 0) return
  await ElMessageBox.confirm(`确定删除选中的 ${selectedIds.value.length} 条记忆？此操作不可撤销。`,"提示",{type:"warning"})
  try {
    await Promise.all(selectedIds.value.map(id => del(`/api/memories/${id}`)))
    ElMessage.success("批量删除成功")
    selectedIds.value = []
    tableRef.value?.clearSelection?.()
    fetchList()
  } catch {
    ElMessage.error("批量删除失败")
  }
}

async function handleClearAll() {
  await ElMessageBox.confirm(`确定清空当前角色全部 ${total.value} 条记忆？此操作不可撤销。`,"警告",{type:"warning",confirmButtonText:"确定清空",confirmButtonClass:"el-button--danger"})
  const cid = characterFilter.value || injectedCharacterId?.value
  if (!cid) { ElMessage.warning("请先选择角色再清空"); return }
  await del(`/api/memories?characterId=${cid}`)
  ElMessage.success("已清空")
  fetchList()
}

async function handleExport() {
  try {
    const params: any = { pageSize: 10000 }
    if (characterFilter.value) params.characterId = characterFilter.value
    if (typeFilter.value) params.memoryType = typeFilter.value
    if (sourceFilter.value) params.source = sourceFilter.value
    if (scopeTypeFilter.value) params.scopeType = scopeTypeFilter.value
    const all = await get<any>("/api/memories", params)
    const items = all?.items || []
    const data = items.map((m:any)=>({
      key:m.key,
      value:m.value,
      type:m.memoryType,
      importance:m.importance,
      source:m.source,
      scope:m.scope,
      scopeType:rowScopeType(m),
      sensitivity:rowSensitivity(m),
      allowContextUse:rowAllowContextUse(m),
      allowProactiveMention:rowAllowProactiveMention(m),
      requiresConfirmation:rowRequiresConfirmation(m),
    }))
    const blob = new Blob([JSON.stringify(data,null,2)],{type:"application/json"})
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a"); a.href=url; a.download="memories-"+new Date().toISOString().slice(0,10)+".json"; a.click()
    URL.revokeObjectURL(url)
    ElMessage.success(`已导出 ${items.length} 条记忆`)
  } catch { ElMessage.error("导出失败") }
}

async function confirmCandidate(c: any) {
  try {
    await post("/api/memory-candidates/" + c.id + "/accept", {})
    ElMessage.success("已保存")
  } catch {
    await post("/api/memories",{
      key:c.key,
      value:c.value,
      memoryType:c.memoryType||"custom",
      importance:c.importance||5,
      source:"manual",
      scope:"character",
      scopeType:"user_character",
      characterId:characterFilter.value||injectedCharacterId?.value||"",
      sensitivity:"normal",
      allowContextUse:true,
      allowProactiveMention:false,
      requiresConfirmation:false,
    })
    ElMessage.success("已保存")
  }
  candidates.value = candidates.value.filter(x=>x.id!==c.id)
  fetchList()
}

async function editCandidate(c: any) {
  editForm.key = c.key
  editForm.value = c.value
  editForm.content = c.value
  editForm.memoryType = c.memoryType
  editForm.importance = c.importance
  editForm.candidateId = c.id
  editCandidateVisible.value = true
}

async function saveEditCandidate() {
  if (!editForm.candidateId) return
  saving.value = true
  try {
    await put("/api/memory-candidates/" + editForm.candidateId, {
      key: editForm.key,
      value: editForm.content,
      memoryType: editForm.memoryType,
      importance: editForm.importance,
    })
    ElMessage.success("已更新")
    editCandidateVisible.value = false
    await loadCandidates()
  } catch (err: any) {
    ElMessage.error(err?.message || "更新失败")
  }
  saving.value = false
}

async function deleteCandidateItem(c: any) {
  try {
    await del("/api/memory-candidates/" + c.id)
    candidates.value = candidates.value.filter(x=>x.id!==c.id)
    ElMessage.success("已删除")
  } catch { ElMessage.error("删除失败") }
}

async function loadVectorStatus() {
  try {
    vectorStatus.value = await get<any>("/api/memories/vector-status")
  } catch {}
}

async function fetchPipelineStatus() {
  try {
    const r = await get<any>("/api/memory/pipeline/status")
    pipelineStatus.value = r
  } catch {}
}

async function rebuildIndex() {
  rebuilding.value = true
  try {
    const result = await post<any>("/api/memories/rebuild-embeddings", {})
    ElMessage.success(`索引重建完成：${result.embedded ?? result.totalEmbedded ?? 0} 条记忆已处理`)
    await loadVectorStatus()
  } catch (err: any) {
    ElMessage.error(err.message || "Rebuild failed")
  }
  rebuilding.value = false
}

function searchMemory() {
  searchDialogVisible.value = true
  searched.value = false
  searchResults.value = []
  searchQuery.value = ""
}

async function doSearch() {
  if (!searchQuery.value.trim()) return
  try {
    const result = await post<any>("/api/memories/hybrid-search", {
      keyword: searchQuery.value.trim(),
      limit: 10,
    })
    const items = result?.items || []
    searchResults.value = items.map((r: any) => ({
      id: r.memory?.id || r.id,
      key: r.memory?.key || r.key,
      value: r.memory?.value || r.value,
      memoryType: r.memory?.memoryType || r.memoryType,
      score: r.score ?? 0,
      matchType: r.matchType || "hybrid",
      memoryLayer: r.memoryLayer || "",
    }))
    searched.value = true
  } catch {
    try {
      const result = await post<any>("/api/memories/search", {
        keyword: searchQuery.value.trim(),
        limit: 10,
      })
      searchResults.value = (result?.items || []).map((r: any) => ({ ...r, score: 0 }))
      searched.value = true
    } catch {
      searchResults.value = []
      searched.value = true
    }
  }
}

function fmtDate(d: string) {
  if (!d) return ""
  try { return new Date(d).toLocaleString("zh-CN") } catch { return d }
}

async function loadConversations() {
  try {
    const res: any = await get("/api/chats/conversations", { pageSize: 100 })
    conversationList.value = res?.items || res?.data || []
  } catch {}
}

async function loadCandidates() {
  try {
    const r: any = await get("/api/memory-candidates")
    candidates.value = r?.candidates || []
  } catch {}
}

function parseMemIDs(raw: string): string[] {
  if (!raw) return []
  try { return JSON.parse(raw) } catch { return [] }
}

function maxScore(raw: string): string {
  if (!raw) return "--"
  try {
    const arr = JSON.parse(raw)
    if (!Array.isArray(arr) || arr.length === 0) return "--"
    const max = Math.max(...arr.map((x: any) => x.score || 0))
    return (max * 100).toFixed(1) + "%"
  } catch { return "--" }
}

function clearGlobalSearch() {
  globalQuery.value = ""
  globalSearched.value = false
  showGlobalResults.value = false
  globalResults.value = { memories: [], profiles: [], episodics: [], worldBooks: [] }
  globalResultCount.value = 0
}

async function doGlobalSearch() {
  if (!globalQuery.value.trim()) return
  globalSearching.value = true
  try {
    const q = globalQuery.value.trim()
    const hub = await import("../../composables/useMemoryHub")
    const { useMemoryHub } = hub
    const { globalSearch } = useMemoryHub()
    const results = await globalSearch(q)
    globalResults.value = results
    globalResultCount.value = results.memories.length + results.profiles.length + results.episodics.length + results.worldBooks.length
    globalSearched.value = true
    showGlobalResults.value = true
  } catch {
    globalResults.value = { memories: [], profiles: [], episodics: [], worldBooks: [] }
    globalResultCount.value = 0
    globalSearched.value = true
    showGlobalResults.value = true
  }
  globalSearching.value = false
}



async function loadRetrievalStats() {
  try {
    const r: any = await get("/api/memory/retrieval/stats")
    retrievalStats.value = { totalCount: r?.totalCount || 0 }
    retrievalLogs.value = r?.recentLogs || []
  } catch {}
}

onMounted(async () => {
  try { characters.value = await get<any[]>("/api/characters") || [] } catch {}
  await loadVectorStatus()
  fetchPipelineStatus()

  await fetchList()
  await loadCandidates()
  await loadConversations()
  loadRetrievalStats()
})
</script>

<style scoped>
.mem-page { }
.page-title { font-size:var(--ac-font-size-lg); font-weight:600; margin-bottom:12px; }
.mem-toolbar { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
.toolbar-spacer { flex:1; }
.candidate-list { display:flex; flex-direction:column; gap:8px; margin:10px 0; }
.candidate-card { padding:12px; border-radius:var(--ac-radius-md); background:var(--ac-color-warning-bg, rgba(200,146,74,0.08)); border:1px solid var(--ac-color-warning-border, rgba(200,146,74,0.2)); }
.cc-header { display:flex; align-items:center; gap:8px; margin-bottom:6px; }
.cc-importance { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); }
.cc-key { font-weight:600; font-size:var(--ac-font-size-sm); margin-bottom:2px; }
.cc-value { font-size:var(--ac-font-size-sm); color:var(--ac-color-text-secondary); margin-bottom:4px; }
.cc-source { font-size:var(--ac-font-size-xs); color:var(--ac-color-text-muted); margin-bottom:6px; }
.source-badge { font-size:var(--ac-font-size-xs); padding:1px 6px; border-radius:4px; background:var(--ac-color-bg-secondary); }

/* Vector Index Bar */
.vector-index-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  margin: 10px 0;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  border: 1px solid var(--ac-color-border-light);
  flex-wrap: wrap;
  gap: 8px;
}
.vib-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.vib-label {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}
.vib-provider {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
.vib-time {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
.vib-actions {
  display: flex;
  gap: 6px;
}
.vector-collection-table {
  width: 100%;
  margin-top: 8px;
}

/* Search Results */
.search-result-item {
  padding: 8px 10px;
  border-bottom: 1px solid var(--ac-color-border-light);
}
.search-result-item:last-child {
  border-bottom: none;
}
.sri-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.sri-score {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-primary);
  font-weight: 600;
}
.sri-key {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}
.sri-value {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
}
.pipeline-bar {
  display: flex; align-items: center; gap: 10px; padding: 8px 12px;
  background: var(--ac-color-bg-secondary); border: 1px solid #e4e7ed; border-radius: 6px;
  margin-bottom: 12px; font-size: 13px;
}
.pl-label { color: #606266; font-weight: 600; margin-right: 4px; }
.pl-dot {
  display: inline-block; width: 14px; height: 14px; border-radius: 50%;
  cursor: pointer; transition: transform 0.15s;
}
.pl-dot:hover { transform: scale(1.3); }
.pl-time { color: #909399; font-size: 12px; margin-left: auto; }
.mem-tabs { margin-top: 8px; }
.analysis-panel { padding: 4px 0; }
.ap-title { font-size: 16px; font-weight: 600; margin-bottom: 12px; }
.ap-stats-row { display: flex; gap: 16px; margin-bottom: 20px; }
.ap-stat-card { flex: 1; text-align: center; }
.ap-stat-num { font-size: 28px; font-weight: 700; color: var(--ac-color-primary); }
.ap-stat-label { font-size: 13px; color: var(--ac-color-text-muted); margin-top: 4px; }
.ap-subtitle { font-size: 14px; font-weight: 600; margin: 16px 0 10px; }
.ap-sliders { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 16px; }
.ap-slider-item { flex: 1; min-width: 200px; display: flex; align-items: center; gap: 10px; }
.ap-slider-label { font-size: 13px; white-space: nowrap; min-width: 80px; }
.global-search-bar { display: flex; align-items: center; gap: 8px; margin: 10px 0; }
.global-search-bar .el-input { flex: 1; max-width: 400px; }
.global-results { background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-sm); padding: 12px; margin-bottom: 12px; }
.gr-section { margin-bottom: 10px; }
.gr-section h4 { font-size: 13px; margin: 0 0 6px; color: var(--ac-color-text-secondary); }
.gr-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px; }
.gr-score { font-size: 11px; color: var(--ac-color-primary); }
.sub-panel { padding: 8px 0; }
.sub-loading, .sub-empty { text-align: center; padding: 40px; color: var(--ac-color-text-muted); }
.profile-cards { display: flex; flex-direction: column; gap: 6px; }
.profile-card { display: flex; align-items: center; gap: 10px; padding: 8px 12px; background: var(--ac-color-bg-secondary); border-radius: var(--ac-radius-sm); }
.pc-attr { font-weight: 600; font-size: 13px; }
.pc-val { font-size: 13px; color: var(--ac-color-text-secondary); }
.pc-conf { font-size: 12px; color: var(--ac-color-text-muted); }
.episodic-cards { display: flex; flex-direction: column; gap: 8px; }
.episodic-card { padding: 10px 12px; background: var(--ac-color-bg-secondary); border-radius: var(--ac-radius-sm); }
.ec-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.ec-emoji { font-size: 16px; }
.ec-title { font-weight: 600; font-size: 13px; }
.ec-content { font-size: 13px; color: var(--ac-color-text-secondary); margin-bottom: 4px; }
.ec-time { font-size: 11px; color: var(--ac-color-text-muted); }
.wb-cards { display: flex; flex-direction: column; gap: 8px; }
.wb-card { padding: 10px 12px; background: var(--ac-color-bg-secondary); border-radius: var(--ac-radius-sm); }
.wb-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.wb-pattern { font-size: 12px; background: var(--ac-color-bg); padding: 1px 6px; border-radius: 3px; }
.wb-priority { font-size: 12px; color: var(--ac-color-text-muted); }
.wb-content { font-size: 13px; color: var(--ac-color-text-secondary); }
.graph-mini { padding: 8px 0; }
.graph-stat { font-size: 14px; margin-bottom: 6px; }

.scope-char-name {
  font-size: 11px;
  color: #909399;
  margin-left: 4px;
}

.scope-toggle-btn {
  margin-left: 4px !important;
  text-decoration: underline !important;
}

.permission-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.permission-switches {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
</style>



