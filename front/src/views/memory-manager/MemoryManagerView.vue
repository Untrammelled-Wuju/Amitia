<template>
  <div class="mem-page">
    <h2 class="page-title">记忆管理</h2>

    <div class="pipeline-bar" v-if="pipelineStatus">
      <span class="pl-label">管线状态:</span>
      <template v-for="l in pipelineStatus.layers" :key="l.layer">
        <el-tooltip :content="l.name + ': ' + l.status + ' (' + l.durationMs + 'ms)'" placement="top">
        <span class="pl-dot" :class="'pl-' + l.status" :style="{ backgroundColor: l.status === 'completed' ? 'var(--ac-color-success)' : l.status === 'skipped' ? 'var(--ac-color-text-muted)' : 'var(--ac-color-danger)' }"></span>
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
import { ref, onMounted, inject, type Ref } from "vue"
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
import { useMemorySearch } from "./composables/useMemorySearch"
import { useMemoryEditor } from "./composables/useMemoryEditor"
import { TYPES, SOURCES, SCOPE_TYPES, SENSITIVITY_OPTIONS, typeLabel, sourceLabel, importanceColor, isExpired, legacyScopeToScopeType, rowScopeType, scopeTypeToScope, scopeTypeLabel, rowSensitivity, sensitivityLabel, sensitivityTagType, readBooleanFlag, rowAllowContextUse, rowAllowProactiveMention, rowRequiresConfirmation, scopeTypeTagType, fmtDate, parseMemIDs, maxScore } from "./memoryFormatters"

const injectedCharacterId = inject<Ref<string | null>>('currentCharacterId', ref(null))

const { memories, keyword, typeFilter, sourceFilter, scopeTypeFilter, characterFilter, characters, sortBy, page, pageSize, total, selectedIds, tableRef, fetchList, handleSelectionChange, delMem, toggleScope, batchVerify, batchSetImportant, batchDelete, handleClearAll, handleExport, loadCharacters } = useMemoryList(injectedCharacterId)
const { dialogVisible, editing, editingId, saving, form, showCreate, showEdit, saveMem } = useMemoryEditor(fetchList, () => injectedCharacterId?.value)
const { vectorStatus, pipelineStatus, rebuilding, retrievalStats, retrievalLogs, halflifeEpisodic, halflifeProfile, halflifeFact, halflifeWorldbook, loadVectorStatus, fetchPipelineStatus, rebuildIndex, loadRetrievalStats } = useMemoryDiagnostics()
const { candidates, showCandidates, conversationList, showGenerateDialog, generating, generateConvId, editCandidateVisible, editForm, conflictVisible, conflictNewType, conflictNewContent, conflictList, resolveAction, loadCandidates, confirmCandidate, deleteCandidateItem, loadConversations, generateCandidates, editCandidate, saveEditCandidate, doResolveConflict } = useMemoryCandidates(fetchList, () => characterFilter.value || injectedCharacterId?.value)
const { searchDialogVisible, searchQuery, searchResults, searched, globalQuery, globalSearching, globalSearched, showGlobalResults, globalResults, globalResultCount, searchMemory, doSearch, clearGlobalSearch, doGlobalSearch } = useMemorySearch()
const activeTab = ref("list")

function charName(cid: string) {
  const ch = characters.value.find((c: any) => String(c.id) === String(cid))
  return ch ? "[" + ch.name + "]" : ""
}
onMounted(async () => {
  await loadCharacters()
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
.page-title { font-size:24px; font-weight:600; margin-bottom:12px; }
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
  background: var(--ac-color-bg-secondary); border: 1px solid var(--ac-color-border); border-radius: 6px;
  margin-bottom: 12px; font-size: 13px;
}
.pl-label { color: var(--ac-color-text-secondary); font-weight: 600; margin-right: 4px; }
.pl-dot {
  display: inline-block; width: 14px; height: 14px; border-radius: 50%;
  cursor: pointer; transition: transform 0.15s;
}
.pl-dot:hover { transform: scale(1.3); }
.pl-time { color: var(--ac-color-text-muted); font-size: 12px; margin-left: auto; }
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
  color: var(--ac-color-text-muted);
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



