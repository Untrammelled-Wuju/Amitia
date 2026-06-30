<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="worldbook-page">
    <div class="page-header">
      <h2>世界书</h2>
      <div class="header-actions">
        <el-button size="small" :type="testPanelOpen ? 'warning' : 'success'" @click="testPanelOpen = !testPanelOpen">
          {{ testPanelOpen ? '关闭测试' : '在线测试' }}
        </el-button>
        <el-button size="small" @click="triggerImport">JSON导入</el-button>
        <el-button size="small" @click="exportRules">JSON导出</el-button>
        <el-button size="small" type="primary" @click="showAddForm = true">新增规则</el-button>
      </div>
    </div>

    <div class="filter-bar">
      <el-select v-model="filterType" placeholder="全部类型" clearable size="small" style="width:150px" @change="onFilterChange">
        <el-option label="全部类型" value="" />
        <el-option label="正则匹配" value="regex" />
        <el-option label="精确匹配" value="exact" />
        <el-option label="关键词匹配" value="keyword" />
      </el-select>
    </div>

    <div v-if="testPanelOpen" class="test-panel">
      <h3>在线测试</h3>
      <el-input v-model="testText" type="textarea" placeholder="输入测试文本，查看哪些规则命中..." :rows="4" />
      <el-button size="small" type="primary" @click="runTest" :disabled="!testText.trim()" style="margin-top:8px">测试匹配</el-button>
      <div v-if="testResults.length > 0" class="test-results">
        <h4>命中规则 ({{ testResults.length }})</h4>
        <div v-for="(r, idx) in testResults" :key="idx" class="test-match-item">
          <div class="match-header">
            <span class="match-type-badge" :class="'badge-' + r.entry.matchType">{{ matchTypeLabel(r.entry.matchType) }}</span>
            <span class="match-pattern">{{ r.entry.matchPattern }}</span>
            <span class="match-priority">优先级: {{ r.entry.priority }}</span>
          </div>
          <div class="match-content">{{ r.entry.injectContent }}</div>
          <div class="match-hit-text" v-html="highlightMatch(r.hitText, r.entry.matchPattern)"></div>
        </div>
      </div>
      <div v-if="testText && tested && testResults.length === 0" class="no-match">无规则命中</div>
    </div>

    <el-dialog v-model="showAddForm" title="新增规则" width="500px" align-center destroy-on-close @closed="showAddForm = false">
      <el-form label-width="80px">
        <el-form-item label="匹配类型">
          <el-select v-model="form.matchType" style="width:100%">
            <el-option label="正则匹配" value="regex" />
            <el-option label="精确匹配" value="exact" />
            <el-option label="关键词匹配" value="keyword" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配模式">
          <el-input v-model="form.matchPattern" placeholder="正则表达式/精确文本/关键词(逗号分隔)" />
        </el-form-item>
        <el-form-item label="匹配范围">
          <el-select v-model="form.matchScope" style="width:100%">
            <el-option label="全部上下文" value="full_context" />
            <el-option label="仅用户消息" value="user_message" />
            <el-option label="仅AI回复" value="assistant_reply" />
          </el-select>
        </el-form-item>
        <el-form-item label="注入内容">
          <el-input v-model="form.injectContent" type="textarea" placeholder="匹配命中后注入到上下文的记忆内容" :rows="3" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="form.priority" :min="0" :max="10" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddForm = false">取消</el-button>
        <el-button type="primary" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editVisible" title="编辑规则" width="500px" align-center destroy-on-close @closed="editingEntry = null">
      <el-form label-width="80px">
        <el-form-item label="匹配类型">
          <el-select v-model="editForm.matchType" style="width:100%">
            <el-option label="正则匹配" value="regex" />
            <el-option label="精确匹配" value="exact" />
            <el-option label="关键词匹配" value="keyword" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配模式">
          <el-input v-model="editForm.matchPattern" />
        </el-form-item>
        <el-form-item label="匹配范围">
          <el-select v-model="editForm.matchScope" style="width:100%">
            <el-option label="全部上下文" value="full_context" />
            <el-option label="仅用户消息" value="user_message" />
            <el-option label="仅AI回复" value="assistant_reply" />
          </el-select>
        </el-form-item>
        <el-form-item label="注入内容">
          <el-input v-model="editForm.injectContent" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="editForm.priority" :min="0" :max="10" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false; editingEntry = null">取消</el-button>
        <el-button type="primary" @click="handleUpdate">保存</el-button>
      </template>
    </el-dialog>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="rules-list">
      <div v-for="rule in rules" :key="rule.id" class="rule-card">
        <div class="rule-meta">
          <span class="match-type-badge" :class="'badge-' + rule.matchType">{{ matchTypeLabel(rule.matchType) }}</span>
          <span class="match-scope">范围: {{ scopeLabel(rule.matchScope) }}</span>
          <span class="priority">优先级: {{ rule.priority }}</span>
          <span class="hit-count">命中: {{ rule.hitCount }}</span>
        </div>
        <div class="rule-pattern">匹配: {{ rule.matchPattern }}</div>
        <div class="rule-content">注入: {{ rule.injectContent }}</div>
        <div class="rule-actions">
          <el-button size="small" @click="startEdit(rule)">编辑</el-button>
          <el-button size="small" type="danger" @click="handleDelete(rule.id)">删除</el-button>
        </div>
      </div>
      <div v-if="rules.length === 0" class="empty">暂无世界书规则</div>
    </div>

    <div v-if="totalPages > 1" class="pagination">
      <el-pagination v-model:current-page="page" :page-size="20" :total="total" layout="prev, pager, next" size="small" @current-change="changePage" />
    </div>

    <input ref="importInput" type="file" accept=".json" style="display:none" @change="handleImport" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useWorldBook } from "@/composables/useWorldBook"

const {
  rules, loading, total, page, totalPages,
  fetchRules, createRule, updateRule, deleteRule,
  testMatch, matchTypeLabel, scopeLabel,
} = useWorldBook()

const filterType = ref("")
const testPanelOpen = ref(false)
const testText = ref("")
const testResults = ref<any[]>([])
const tested = ref(false)
const showAddForm = ref(false)
const editVisible = ref(false)
const editingEntry = ref<any>(null)
const importInput = ref<HTMLInputElement | null>(null)

const form = reactive({ matchType: "keyword", matchPattern: "", matchScope: "full_context", injectContent: "", priority: 0 })
const editForm = reactive({ matchType: "", matchPattern: "", matchScope: "", injectContent: "", priority: 0 })

onMounted(() => { fetchRules() })

function onFilterChange() {
  fetchRules({ matchType: filterType.value || undefined })
}

async function runTest() {
  tested.value = true
  testResults.value = (await testMatch(testText.value))?.matches || []
}

async function handleCreate() {
  await createRule(form)
  showAddForm.value = false
  form.matchType = "keyword"; form.matchPattern = ""; form.matchScope = "full_context"; form.injectContent = ""; form.priority = 0
}

function startEdit(rule: any) {
  editingEntry.value = rule
  editForm.matchType = rule.matchType
  editForm.matchPattern = rule.matchPattern
  editForm.matchScope = rule.matchScope
  editForm.injectContent = rule.injectContent
  editForm.priority = rule.priority
  editVisible.value = true
}

async function handleUpdate() {
  await updateRule(editingEntry.value.id, { ...editForm })
  editVisible.value = false
  editingEntry.value = null
}

async function handleDelete(id: string) {
  try {
    await ElMessageBox.confirm("确定删除这条规则？", "删除确认", { confirmButtonText: "确定", cancelButtonText: "取消", type: "warning" })
    await deleteRule(id)
  } catch {}
}

function changePage(p: number) { fetchRules({ page: p, matchType: filterType.value || undefined }) }

function highlightMatch(text: string, pattern: string): string {
  if (!text || !pattern) return text
  return text.replace(new RegExp(pattern.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi'), '<mark>$&</mark>')
}

function triggerImport() { importInput.value?.click() }

async function handleImport(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const text = await file.text()
  let data: any[]
  try { data = JSON.parse(text) } catch { ElMessage.error("JSON格式错误"); return }
  if (!Array.isArray(data)) { ElMessage.error("JSON应为数组"); return }
  let success = 0
  for (const item of data) {
    try { await createRule(item); success++ } catch {}
  }
  ElMessage.success(`导入完成：成功 ${success} / ${data.length}`)
  fetchRules()
}

function exportRules() {
  const data = rules.value.map(r => ({ matchType: r.matchType, matchPattern: r.matchPattern, matchScope: r.matchScope, injectContent: r.injectContent, priority: r.priority }))
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url; a.download = "world_book.json"; a.click()
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.worldbook-page { padding: 24px; max-width: 900px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h2 { margin: 0; font-size: 24px; }
.header-actions { display: flex; gap: 8px; }

.filter-bar { margin-bottom: 16px; }

.test-panel { background: #f9f9f9; border: 1px solid #e0e0e0; border-radius: 10px; padding: 16px; margin-bottom: 20px; }
.test-panel h3 { margin: 0 0 12px; }

.test-results { margin-top: 12px; }
.test-results h4 { margin: 0 0 8px; }
.test-match-item { background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; padding: 10px; margin-bottom: 8px; }
.match-header { display: flex; gap: 8px; align-items: center; margin-bottom: 4px; font-size: 13px; }
.match-pattern { font-family: monospace; color: #333; }
.match-priority { color: #999; font-size: 12px; }
.match-content { color: #555; font-size: 14px; margin-top: 4px; }
.match-hit-text { font-size: 12px; color: #999; margin-top: 4px; font-family: monospace; background: #fffde7; padding: 4px 8px; border-radius: 4px; }
.match-hit-text :deep(mark) { background: #ffeb3b; padding: 0 2px; }
.no-match { color: #999; text-align: center; padding: 16px; }
.match-type-badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; color: #fff; }
.badge-regex { background: #7b1fa2; }
.badge-exact { background: #1976d2; }
.badge-keyword { background: #388e3c; }
.rules-list { display: flex; flex-direction: column; gap: 12px; }
.rule-card { background: var(--ac-color-bg-secondary); border: 1px solid #666; border-radius: 10px; padding: 14px; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
.rule-meta { display: flex; gap: 12px; align-items: center; margin-bottom: 8px; font-size: 12px; }
.match-scope { color: #999; }
.priority { color: #ff9800; }
.hit-count { color: #43a047; }
.rule-pattern { font-family: monospace; font-size: 14px; color: var(--ac-color-text-primary); margin-bottom: 4px; }
.rule-content { font-size: 14px; color: var(--ac-color-text-primary); margin-bottom: 8px; }
.rule-actions { display: flex; gap: 8px; }
.loading, .empty { text-align: center; padding: 48px; color: #999; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; margin-top: 20px; }

</style>
