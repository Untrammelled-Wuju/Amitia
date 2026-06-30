<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
﻿<template>
  <div class="mem-page">
    <h2 class="page-title">记忆管理</h2>

    <!-- Privacy note -->
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
      <template #title>记忆保存在你自己的设备或服务器上，可随时编辑或删除。候选记忆需确认后才保存。</template>
    </el-alert>

    <div class="pipeline-bar" v-if="pipelineStatus">
      <span class="pl-label">管线状态:</span>
      <template v-for="l in pipelineStatus.layers" :key="l.layer">
        <el-tooltip :content="l.name + ': ' + l.status + ' (' + l.durationMs + 'ms)'" placement="top">
          <span class="pl-dot" :class="'pl-' + l.status" :style="{backgroundColor: l.status === 'completed' ? '#67c23a' : l.status === 'skipped' ? '#c0c4cc' : '#f56c6c'}"></span>
        </el-tooltip>
      </template>
      <span class="pl-time" v-if="pipelineStatus.endedAt">{{ fmtDate(pipelineStatus.endedAt) }}</span>
    </div>

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
        {{ showGlobalResults ? '隐藏结果' : '显示结果(' + globalResultCount + ')' }}
      </el-button>
    </div>
    <div v-if="showGlobalResults && globalSearched" class="global-results">
      <div v-if="globalResults.memories.length" class="gr-section">
        <h4><el-tooltip content="按角色独立，切换角色后数据不同" placement="top"><span class="gr-label">结构化记忆</span></el-tooltip> ({{ globalResults.memories.length }})</h4>
        <div v-for="m in globalResults.memories" :key="m.id" class="gr-item">
          <el-tag size="small">{{ typeLabel(m.memoryType) }}</el-tag>
          <span>{{ m.key }}: {{ m.value }}</span>
          <span class="gr-score" v-if="m.score">({{ (m.score*100).toFixed(0) }}%)</span>
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
    <el-table ref="tableRef" :data="memories" stripe size="small" style="margin-top:10px" @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="36" />
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
            <el-progress :percentage="row.confidence ?? 50" :stroke-width="6" :show-text="false"
              :color="(row.confidence ?? 50) >= 80 ? '#67c23a' : (row.confidence ?? 50) >= 50 ? '#e6a23c' : '#f56c6c'" />
            <span style="font-size:11px">{{ row.confidence ?? 50 }}%</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="核实状态" width="90" sortable prop="verifiedStatus">
        <template #default="{ row }">
          <el-tag v-if="isExpired(row.expiresAt)" type="info" size="small">已过期</el-tag>
          <el-tag v-else-if="row.verifiedStatus === 'user_verified'" type="success" size="small">已确认</el-tag>
          <el-tag v-else-if="row.verifiedStatus === 'auto_confirmed'" type="warning" size="small">自动确认</el-tag>
          <el-tag v-else-if="row.verifiedStatus === 'contradicted'" type="danger" size="small">有矛盾</el-tag>
          <el-tag v-else type="info" size="small">未核实</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="范围" width="220">
        <template #default="{ row }">
          <el-tag size="small" :type="row.scope==='user'?'success':'info'">{{ row.scope==='user'?'共享':'独有' }}</el-tag>
          <span v-if="row.scope==='character' && row.characterId" class="scope-char-name">{{ charName(row.characterId) }}</span>
          <el-button v-if="row.scope==='character'" text size="small" type="warning" class="scope-toggle-btn" @click="toggleScope(row)">升级为共享</el-button>
          <el-button v-if="row.scope==='user'" text size="small" type="info" class="scope-toggle-btn" @click="toggleScope(row)">降级为独享</el-button>
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

      </el-tab-pane>

      <el-tab-pane label="检索分析" name="analysis">
        <div class="analysis-panel">
          <h3 class="ap-title">检索质量分析</h3>

          <div class="ap-stats-row">
            <el-card shadow="hover" class="ap-stat-card">
              <div class="ap-stat-num">{{ retrievalStats.totalCount }}</div>
              <div class="ap-stat-label">总检索次数</div>
            </el-card>
            <el-card shadow="hover" class="ap-stat-card">
              <div class="ap-stat-num" v-if="retrievalLogs.length > 0">{{ (retrievalLogs.length / (retrievalStats.totalCount || 1) * 100).toFixed(1) }}%</div>
              <div class="ap-stat-num" v-else>--</div>
              <div class="ap-stat-label">最近50条占比</div>
            </el-card>
          </div>

          <h4 class="ap-subtitle">半衰期参数（天）</h4>
          <div class="ap-sliders">
            <div class="ap-slider-item">
              <span class="ap-slider-label">情景记忆</span>
              <el-slider v-model="halflifeEpisodic" :min="7" :max="90" :step="1" show-input disabled />
            </div>
            <div class="ap-slider-item">
              <span class="ap-slider-label">用户画像</span>
              <el-slider v-model="halflifeProfile" :min="30" :max="180" :step="1" show-input disabled />
            </div>
            <div class="ap-slider-item">
              <span class="ap-slider-label">结构化事实</span>
              <el-slider v-model="halflifeFact" :min="60" :max="365" :step="1" show-input disabled />
            </div>
            <div class="ap-slider-item">
              <span class="ap-slider-label">世界书</span>
              <el-slider v-model="halflifeWorldbook" :min="180" :max="730" :step="1" show-input disabled />
            </div>
          </div>

          <h4 class="ap-subtitle">最近检索日志</h4>
          <el-table :data="retrievalLogs" size="small" max-height="300" style="width:100%">
            <el-table-column prop="queryText" label="查询文本" min-width="180" show-overflow-tooltip />
            <el-table-column label="检索记忆数" width="100">
              <template #default="{ row }">
                {{ parseMemIDs(row.retrievedMemoryIDs).length }}
              </template>
            </el-table-column>
            <el-table-column label="最高分" width="80">
              <template #default="{ row }">
                {{ maxScore(row.scoringDetails) }}
              </template>
            </el-table-column>
            <el-table-column prop="createdAt" label="时间" width="160">
              <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>

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
        <el-form-item label="范围">
          <el-select v-model="form.scope" style="width:100%">
            <el-option label="仅当前角色" value="character" />
            <el-option label="跨角色共享" value="user" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible=false">取消</el-button>
        <el-button type="primary" @click="saveMem" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
      <!-- Edit Candidate Dialog -->
    <el-dialog v-model="editCandidateVisible" title="编辑候选记忆" width="480px" destroy-on-close>
      <el-form :model="editForm" label-position="top">
        <el-form-item label="内容"><el-input v-model="editForm.content" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="editForm.memoryType" style="width:100%"><el-option v-for="t in TYPES" :key="t.value" :label="t.label" :value="t.value" /></el-select>
        </el-form-item>
        <el-form-item label="重要度"><el-slider v-model="editForm.importance" :max="10" show-input /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editCandidateVisible=false">取消</el-button>
        <el-button type="primary" @click="saveEditCandidate" :loading="saving">保存并确认</el-button>
      </template>
    </el-dialog>

        <!-- Conflict Resolution Dialog -->
    <el-dialog v-model="conflictVisible" title="记忆冲突检测" width="550px" destroy-on-close :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px">
        新记忆可能与现有记忆冲突，请选择处理方式:
      </el-alert>
      <div class="conflict-new">
        <strong>New:</strong> [{{ conflictNewType }}] {{ conflictNewContent }}
      </div>
      <div v-for="c in conflictList" :key="c.id" class="conflict-old">
        <strong>Existing:</strong> [{{ c.memoryType }}] {{ c.value }}
        <div class="conflict-reason">{{ c.reason }}</div>
      </div>
      <el-radio-group v-model="resolveAction" style="margin-top:12px;display:flex;flex-direction:column;gap:6px">
        <el-radio value="keep_old">保留旧记忆，丢弃新记忆</el-radio>
        <el-radio value="replace_old">用新记忆替换旧记忆</el-radio>
        <el-radio value="keep_both">同时保留</el-radio>
        <el-radio value="merge">合并到现有记忆</el-radio>
      </el-radio-group>
      <template #footer>
        <el-button @click="conflictVisible=false; dialogVisible=true">取消</el-button>
        <el-button type="primary" @click="doResolveConflict">解决</el-button>
      </template>
    </el-dialog>

    <!-- 生成候选 Dialog -->
    <el-dialog v-model="showGenerateDialog" title="生成候选" width="500px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="选择会话">
          <el-select v-model="generateConvId" placeholder="选择会话" filterable style="width:100%">
            <el-option v-for="c in conversationList" :key="c.id" :label="c.title" :value="c.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showGenerateDialog=false">取消</el-button>
        <el-button type="primary" @click="generateCandidates" :loading="generating">生成</el-button>
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

const memories = ref<any[]>([])
const candidates = ref<any[]>([])
const keyword = ref("")
const typeFilter = ref("")
const sourceFilter = ref("")
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
const form = reactive({ key:"", value:"", memoryType:"custom", importance:5, characterId:"", scope:"character", source:"manual" })
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

async function fetchList() {
  const params: any = { page:page.value, pageSize:pageSize.value }
  if (characterFilter.value) params.characterId = characterFilter.value
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
  form.key=""; form.value=""; form.memoryType="custom"; form.importance=5; form.characterId=injectedCharacterId?.value||""; form.source="manual"; form.scope="character"
  dialogVisible.value = true
}

function showEdit(row: any) {
  editing.value = true; editingId.value = row.id
  form.key=row.key; form.value=row.value; form.memoryType=row.memoryType; form.importance=row.importance; form.characterId=row.characterId||""; form.scope=row.scope||"character"; form.source=row.source||"manual"
  dialogVisible.value = true
}

async function toggleScope(row: any) { const newScope = row.scope === "user" ? "character" : "user"; try { await put(`/api/memories/${row.id}`, { scope: newScope }); row.scope = newScope; ElMessage.success(newScope === "user" ? "已升级为共享记忆" : "已降级为独享记忆") } catch {} }

async function saveMem() {
  saving.value = true
  try {
    const payload = {...form, source: form.source || "manual"}
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
    const all = await get<any>("/api/memories", params)
    const items = all?.items || []
    const data = items.map((m:any)=>({key:m.key,value:m.value,type:m.memoryType,importance:m.importance,source:m.source,scope:m.scope}))
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
    await post("/api/memories",{key:c.key,value:c.value,memoryType:c.memoryType||"custom",importance:c.importance||5,source:"manual",scope:"character",characterId:characterFilter.value||injectedCharacterId?.value||""})
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
</style>



