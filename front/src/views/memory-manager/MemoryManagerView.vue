<template>
  <div class="mem-page">
    <h2 class="page-title">记忆管理</h2>

    <!-- Privacy note -->
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
      <template #title>记忆保存在你自己的设备或服务器上，可随时编辑或删除。候选记忆需确认后才保存。</template>
    </el-alert>

    <!-- Toolbar -->
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
      <el-select v-model="sortBy" size="small" style="width:120px" @change="fetchList">
        <el-option label="重要度降序" value="importance_desc" />
        <el-option label="重要度升序" value="importance_asc" />
        <el-option label="时间降序" value="time_desc" />
        <el-option label="时间升序" value="time_asc" />
      </el-select>
      <el-button size="small" @click="showGenerateDialog = true">Generate Candidates</el-button>
      <div class="toolbar-spacer"></div>
      <el-button size="small" type="primary" :icon="Plus" @click="showCreate">新建</el-button>
      <el-button size="small" @click="handleExport">导出</el-button>
      <el-button size="small" type="success" @click="batchVerify" :disabled="selectedIds.length===0">批量确认</el-button>
      <el-button size="small" type="warning" @click="batchSetImportant" :disabled="selectedIds.length===0">标为重要</el-button>
      <el-button size="small" type="danger" plain @click="handleClearAll" :disabled="total === 0">清空全部</el-button>
    </div>

        <!-- Vector Memory Index -->
    <div class="vector-index-bar" v-if="vectorStatus">
      <div class="vib-info">
        <span class="vib-label">向量索引:</span>
        <el-tag :type="vectorStatus.enabled ? 'success' : 'info'" size="small">
          {{ vectorStatus.enabled ? '已启用' : '已禁用' }}
        </el-tag>
        <span class="vib-provider" v-if="vectorStatus.enabled">
          Provider: {{ vectorStatus.providerName }} | 总向量: {{ vectorStatus.totalEmbeddings || vectorStatus.totalEmbedded || 0 }}
        </span>
        <span class="vib-time" v-if="vectorStatus.lastRebuildAt">
          最近重建: {{ fmtDate(vectorStatus.lastRebuildAt) }}
        </span>
      </div>
      <div class="vib-actions">
        <el-button size="small" @click="rebuildIndex" :loading="rebuilding">
          {{ rebuilding ? '重建中...' : '重建索引' }}
        </el-button>
        <el-button size="small" @click="searchMemory" :disabled="!vectorStatus.enabled">
          语义搜索
        </el-button>
      </div>
      <el-table
        v-if="vectorStatus.collections && vectorStatus.collections.length"
        :data="vectorStatus.collections"
        size="small"
        class="vector-collection-table"
      >
        <el-table-column prop="label" label="层级" min-width="100" />
        <el-table-column prop="name" label="Collection" min-width="160" show-overflow-tooltip />
        <el-table-column label="向量数" width="90">
          <template #default="{ row }">{{ row.totalEmbeddings || 0 }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'ready' ? 'success' : row.status === 'error' ? 'danger' : 'info'">
              {{ row.status === 'ready' ? '正常' : row.status === 'error' ? '异常' : '未启用' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- Memory Search Dialog -->
    <el-dialog v-model="searchDialogVisible" title="语义搜索" width="500px">
      <el-input v-model="searchQuery" placeholder="输入搜索词..." @keyup.enter="doSearch" />
      <div style="margin-top:12px;max-height:300px;overflow-y:auto">
        <div v-for="r in searchResults" :key="r.id" class="search-result-item">
          <div class="sri-header">
            <el-tag size="small">{{ typeLabel(r.memoryType) }}</el-tag>
            <span class="sri-score">Score: {{ (r.score * 100).toFixed(1) }}%</span>
          </div>
          <div class="sri-key">{{ r.key }}</div>
          <div class="sri-value">{{ r.value }}</div>
        </div>
        <el-empty v-if="searchResults.length === 0 && searched" description="无结果" />
        <div v-if="!searched" style="color:var(--ac-color-text-muted);text-align:center;padding:20px">
          输入关键词进行语义搜索
        </div>
      </div>
    </el-dialog>

<!-- Candidate memories banner -->
    <el-alert v-if="candidates.length > 0" type="warning" :closable="false" show-icon style="margin:10px 0">
      <template #title>
        有 {{ candidates.length }} 条候选记忆等待确认
        <el-button type="warning" size="small" link @click="showCandidates = !showCandidates">{{ showCandidates ? "收起" : "查看" }}</el-button>
      </template>
    </el-alert>

    <!-- Candidate list -->
    <div v-if="showCandidates && candidates.length > 0" class="candidate-list">
      <div v-for="c in candidates" :key="c.id" class="candidate-card">
        <div class="cc-header">
          <el-tag size="small" :type="c.importance > 7 ? 'danger' : 'info'">{{ typeLabel(c.memoryType) }}</el-tag>
          <span class="cc-importance">重要: {{ c.importance }}/10</span>
        </div>
        <div class="cc-key">{{ c.key }}</div>
        <div class="cc-value">{{ c.value }}</div>
        <div class="cc-source">来源: {{ c.sourceText || "提取" }}</div>
        <div class="cc-actions">
          <el-button size="small" type="primary" @click="confirmCandidate(c)">确认保存</el-button>
          <el-button size="small" @click="editCandidate(c)">编辑</el-button>
          <el-button size="small" type="danger" @click="deleteCandidateItem(c)">删除</el-button>
        </div>
      </div>
    </div>

    <!-- Memory list -->
    <el-table :data="memories" stripe size="small" style="margin-top:10px">
      <el-table-column prop="key" label="关键词" width="140" show-overflow-tooltip />
      <el-table-column prop="value" label="内容" show-overflow-tooltip />
      <el-table-column label="类型" width="90">
        <template #default="{row}">
          <el-tag size="small" type="info">{{ typeLabel(row.memoryType) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="来源" width="80">
        <template #default="{row}">
          <span class="source-badge" :class="row.source">{{ sourceLabel(row.source) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="重要度" width="90" sortable prop="importance">
        <template #default="{row}">
          <el-progress :percentage="row.importance * 10" :stroke-width="6" :show-text="false" :color="importanceColor(row.importance)" />
          <span style="font-size:11px;margin-left:4px">{{ row.importance }}/10</span>
        </template>
      </el-table-column>
      <el-table-column label="置信度" width="100" sortable prop="confidence">
        <template #default="{ row }">
          <div style="display:flex;align-items:center;gap:4px">
            <el-progress :percentage="row.confidence || 50" :stroke-width="6" :show-text="false"
              :color="row.confidence >= 80 ? '#67c23a' : row.confidence >= 50 ? '#e6a23c' : '#f56c6c'" />
            <span style="font-size:11px">{{ row.confidence || 50 }}%</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="核实状态" width="90" sortable prop="verifiedStatus">
        <template #default="{ row }">
          <el-tag v-if="row.verifiedStatus === 'user_verified'" type="success" size="small">已确认</el-tag>
          <el-tag v-else-if="row.verifiedStatus === 'auto_confirmed'" type="warning" size="small">自动确认</el-tag>
          <el-tag v-else-if="row.verifiedStatus === 'contradicted'" type="danger" size="small">有矛盾</el-tag>
          <el-tag v-else type="info" size="small">未核实</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="{row}">
          <el-button text size="small" @click="showEdit(row)">编辑</el-button>
          <el-button text size="small" type="danger" @click="delMem(row.id)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-if="total > pageSize"
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="prev,pager,next"
      @current-change="fetchList"
      style="margin-top:12px;justify-content:center"
    />

    <!-- Create/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑记忆' : '新建记忆'" width="480px" destroy-on-close>
      <el-form :model="form" label-position="top">
        <el-form-item label="关键词"><el-input v-model="form.key" placeholder="例如: 喜欢的音乐" /></el-form-item>
        <el-form-item label="内容"><el-input v-model="form.value" type="textarea" :rows="3" placeholder="例如: 喜欢星期六下午听轻音乐" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.memoryType" style="width:100%"><el-option v-for="t in TYPES" :key="t.value" :label="t.label" :value="t.value" /></el-select>
        </el-form-item>
        <el-form-item label="重要度">
          <el-slider v-model="form.importance" :max="10" show-input :marks="{1:'低',5:'中',10:'高'}" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible=false">取消</el-button>
        <el-button type="primary" @click="saveMem" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
      <!-- Edit Candidate Dialog -->
    <el-dialog v-model="editCandidateVisible" title="Edit Candidate" width="480px" destroy-on-close>
      <el-form :model="editForm" label-position="top">
        <el-form-item label="Content"><el-input v-model="editForm.content" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="Type">
          <el-select v-model="editForm.memoryType" style="width:100%"><el-option v-for="t in TYPES" :key="t.value" :label="t.label" :value="t.value" /></el-select>
        </el-form-item>
        <el-form-item label="Importance"><el-slider v-model="editForm.importance" :max="10" show-input /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editCandidateVisible=false">Cancel</el-button>
        <el-button type="primary" @click="saveEditCandidate" :loading="saving">Save & Accept</el-button>
      </template>
    </el-dialog>

        <!-- Conflict Resolution Dialog -->
    <el-dialog v-model="conflictVisible" title="Memory Conflict Detected" width="550px" destroy-on-close :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px">
        The new memory may conflict with existing ones. Choose how to resolve:
      </el-alert>
      <div class="conflict-new">
        <strong>New:</strong> [{{ conflictNewType }}] {{ conflictNewContent }}
      </div>
      <div v-for="c in conflictList" :key="c.id" class="conflict-old">
        <strong>Existing:</strong> [{{ c.memoryType }}] {{ c.value }}
        <div class="conflict-reason">{{ c.reason }}</div>
      </div>
      <el-radio-group v-model="resolveAction" style="margin-top:12px;display:flex;flex-direction:column;gap:6px">
        <el-radio value="keep_old">Keep old, discard new</el-radio>
        <el-radio value="replace_old">Replace old with new</el-radio>
        <el-radio value="keep_both">Keep both</el-radio>
        <el-radio value="merge">Merge into existing</el-radio>
      </el-radio-group>
      <template #footer>
        <el-button @click="conflictVisible=false">Cancel</el-button>
        <el-button type="primary" @click="doResolveConflict">Resolve</el-button>
      </template>
    </el-dialog>

    <!-- Generate Candidates Dialog -->
    <el-dialog v-model="showGenerateDialog" title="Generate Candidates" width="500px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="Source">
          <el-radio-group v-model="generateSource">
            <el-radio value="conversation">Recent Chat</el-radio>
            <el-radio value="import">Import Batch</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="选择会话" v-if="generateSource === 'conversation'">
          <el-select v-model="generateConvId" placeholder="选择会话" filterable style="width:100%">
            <el-option v-for="c in conversationList" :key="c.id" :label="c.title" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Import Batch ID" v-if="generateSource === 'import'">
          <el-input v-model="generateBatchId" placeholder="Enter import batch ID" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showGenerateDialog=false">Cancel</el-button>
        <el-button type="primary" @click="generateCandidates" :loading="generating">Generate</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, inject, type Ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { Search, Plus } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))

const { get, post, put, del } = useApi()

// Vector memory state
const vectorStatus = ref<any>(null)
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

const memories = ref<any[]>([])
const candidates = ref<any[]>([])
const keyword = ref("")
const typeFilter = ref("")
const sourceFilter = ref("")
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
const form = reactive({ key:"", value:"", memoryType:"custom", importance:5, characterId:"" })
const editCandidateVisible = ref(false)
const conflictVisible = ref(false)
const showGenerateDialog = ref(false)
const editForm = reactive({ key: "", value: "", content: "", memoryType: "custom", importance: 5, candidateId: "" })
const conflictNewType = ref("")
const conflictNewContent = ref("")
const conflictList = ref<any[]>([])
const resolveAction = ref("")
const generating = ref(false)
const generateSource = ref("conversation")
const generateConvId = ref("")
const generateBatchId = ref("")

async function doResolveConflict() {
  if (!resolveAction.value) { ElMessage.warning("请选择处理方式"); return }
  try {
    await post("/api/memories/resolve-conflict", {
      action: resolveAction.value,
      newKey: conflictNewType.value ? "" : "",
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
  if (generateSource.value === "import") {
    if (!generateBatchId.value) { ElMessage.warning("请输入Import Batch ID"); return }
    ElMessage.info("导入批次提取暂不支持，请使用会话方式")
    return
  }
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
function sourceLabel(s: string) { return SOURCES.find(x=>x.value===s)?.label || s }
function importanceColor(v: number) { return v>=8?'#c85a5a':v>=5?'#c8924a':'#5b7fa5' }

async function fetchList() {
  const params: any = { page:page.value, pageSize:pageSize.value }
  if (injectedCharacterId?.value) params.characterId = injectedCharacterId.value
  if (keyword.value) params.keyword = keyword.value
  if (typeFilter.value) params.memoryType = typeFilter.value
  if (sourceFilter.value) params.source = sourceFilter.value
  if (sortBy.value) params.sortBy = sortBy.value
  try {
    const r = await get<any>("/api/memories", params)
    memories.value = r?.items || []
    total.value = r?.total || 0
  } catch {}
}

function showCreate() {
  editing.value = false; editingId.value = ""
  form.key=""; form.value=""; form.memoryType="custom"; form.importance=5; form.characterId=injectedCharacterId?.value||""
  dialogVisible.value = true
}

function showEdit(row: any) {
  editing.value = true; editingId.value = row.id
  form.key=row.key; form.value=row.value; form.memoryType=row.memoryType; form.importance=row.importance; form.characterId=row.characterId||""
  dialogVisible.value = true
}

async function saveMem() {
  saving.value = true
  try {
    if (editing.value) await put(`/api/memories/${editingId.value}`,{...form})
    else await post("/api/memories",{...form})
    dialogVisible.value = false
    ElMessage.success(editing.value?"保存成功":"新建成功")
    fetchList()
  } catch {}
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

async function handleClearAll() {
  await ElMessageBox.confirm(`确定清空当前角色全部 ${total.value} 条记忆？此操作不可撤销。`,"警告",{type:"warning",confirmButtonText:"确定清空",confirmButtonClass:"el-button--danger"})
  const cid = injectedCharacterId?.value
  if (cid) await del(`/api/memories?characterId=${cid}`)
  else await del("/api/memories")
  ElMessage.success("已清空")
  fetchList()
}

async function handleExport() {
  // Build JSON export
  const data = memories.value.map(m=>({key:m.key,value:m.value,type:m.memoryType,importance:m.importance,source:m.source}))
  const blob = new Blob([JSON.stringify(data,null,2)],{type:"application/json"})
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a"); a.href=url; a.download="memories-"+new Date().toISOString().slice(0,10)+".json"; a.click()
  URL.revokeObjectURL(url)
  ElMessage.success("已导出")
}

async function confirmCandidate(c: any) {
  try {
    await post("/api/memory-candidates/" + c.id + "/accept", {})
    ElMessage.success("已保存")
  } catch {
    await post("/api/memories",{key:c.key,value:c.value,memoryType:c.memoryType||"custom",importance:c.importance||5})
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
    ElMessage.success("已删除")
    candidates.value = candidates.value.filter(x=>x.id!==c.id)
  } catch {
    candidates.value = candidates.value.filter(x=>x.id!==c.id)
    ElMessage.success("已删除")
  }
}

async function loadVectorStatus() {
  try {
    vectorStatus.value = await get<any>("/api/memories/vector-status")
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

onMounted(async () => {
  await loadVectorStatus()
  await fetchList()
  await loadCandidates()
  await loadConversations()
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
</style>



