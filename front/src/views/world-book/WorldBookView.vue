<template>
  <div class="worldbook-page">
    <div class="page-header">
      <h2>世界书</h2>
      <div class="header-actions">
        <button class="btn btn-test" @click="testPanelOpen = !testPanelOpen">
          {{ testPanelOpen ? '关闭测试' : '在线测试' }}
        </button>
        <button class="btn btn-import" @click="triggerImport">JSON导入</button>
        <button class="btn btn-export" @click="exportRules">JSON导出</button>
        <button class="btn btn-add" @click="showAddForm = true">新增规则</button>
      </div>
    </div>

    <div class="filter-bar">
      <select v-model="filterType" @change="onFilterChange">
        <option value="">全部类型</option>
        <option value="regex">正则匹配</option>
        <option value="exact">精确匹配</option>
        <option value="keyword">关键词匹配</option>
      </select>
    </div>

    <div v-if="testPanelOpen" class="test-panel">
      <h3>在线测试</h3>
      <textarea v-model="testText" placeholder="输入测试文本，查看哪些规则命中..." rows="4"></textarea>
      <button class="btn" @click="runTest" :disabled="!testText.trim()">测试匹配</button>
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

    <div v-if="showAddForm" class="modal-overlay" @click.self="showAddForm = false">
      <div class="modal">
        <h3>新增规则</h3>
        <form @submit.prevent="handleCreate">
          <label>匹配类型</label>
          <select v-model="form.matchType">
            <option value="regex">正则匹配</option>
            <option value="exact">精确匹配</option>
            <option value="keyword">关键词匹配</option>
          </select>
          <label>匹配模式</label>
          <input v-model="form.matchPattern" placeholder="正则表达式/精确文本/关键词(逗号分隔)" />
          <label>匹配范围</label>
          <select v-model="form.matchScope">
            <option value="full_context">全部上下文</option>
            <option value="user_message">仅用户消息</option>
            <option value="assistant_reply">仅AI回复</option>
          </select>
          <label>注入内容</label>
          <textarea v-model="form.injectContent" placeholder="匹配命中后注入到上下文的记忆内容" rows="3"></textarea>
          <label>优先级</label>
          <input v-model.number="form.priority" type="number" placeholder="数字越大越优先" />
          <div class="form-actions">
            <button type="submit" class="btn btn-primary">创建</button>
            <button type="button" class="btn" @click="showAddForm = false">取消</button>
          </div>
        </form>
      </div>
    </div>

    <div v-if="editingEntry" class="modal-overlay" @click.self="editingEntry = null">
      <div class="modal">
        <h3>编辑规则</h3>
        <form @submit.prevent="handleUpdate">
          <label>匹配类型</label>
          <select v-model="editForm.matchType">
            <option value="regex">正则匹配</option>
            <option value="exact">精确匹配</option>
            <option value="keyword">关键词匹配</option>
          </select>
          <label>匹配模式</label>
          <input v-model="editForm.matchPattern" />
          <label>匹配范围</label>
          <select v-model="editForm.matchScope">
            <option value="full_context">全部上下文</option>
            <option value="user_message">仅用户消息</option>
            <option value="assistant_reply">仅AI回复</option>
          </select>
          <label>注入内容</label>
          <textarea v-model="editForm.injectContent" rows="3"></textarea>
          <label>优先级</label>
          <input v-model.number="editForm.priority" type="number" />
          <div class="form-actions">
            <button type="submit" class="btn btn-primary">保存</button>
            <button type="button" class="btn" @click="editingEntry = null">取消</button>
          </div>
        </form>
      </div>
    </div>

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
          <button class="btn-sm" @click="startEdit(rule)">编辑</button>
          <button class="btn-sm btn-del" @click="handleDelete(rule.id)">删除</button>
        </div>
      </div>
      <div v-if="rules.length === 0" class="empty">暂无世界书规则</div>
    </div>

    <div class="pagination" v-if="totalPages > 1">
      <button :disabled="page <= 1" @click="changePage(page - 1)">上一页</button>
      <span>{{ page }} / {{ totalPages }}</span>
      <button :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</button>
    </div>

    <input ref="importInput" type="file" accept=".json" style="display:none" @change="handleImport" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from "vue"
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
}

async function handleUpdate() {
  await updateRule(editingEntry.value.id, { ...editForm })
  editingEntry.value = null
}

async function handleDelete(id: string) {
  if (confirm("确定删除？")) { await deleteRule(id) }
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
  try { data = JSON.parse(text) } catch { alert("JSON格式错误"); return }
  if (!Array.isArray(data)) { alert("JSON应为数组"); return }
  let success = 0
  for (const item of data) {
    try { await createRule(item); success++ } catch {}
  }
  alert(`导入完成：成功 ${success} / ${data.length}`)
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
.btn { padding: 8px 14px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; font-size: 13px; }
.btn:hover { background: #f5f5f5; }
.btn-primary { background: #1976d2; color: #fff; border-color: #1976d2; }
.btn-add { background: #1976d2; color: #fff; border: none; }
.btn-test { background: #43a047; color: #fff; border: none; }
.btn-export { background: #ff9800; color: #fff; border: none; }
.btn-import { background: #8e24aa; color: #fff; border: none; }
.btn-sm { padding: 4px 10px; border: 1px solid #ddd; border-radius: 4px; background: #fff; cursor: pointer; font-size: 12px; }
.btn-del { color: #f44336; border-color: #f44336; }
.filter-bar { margin-bottom: 16px; }
.filter-bar select { padding: 6px 12px; border: 1px solid #ddd; border-radius: 6px; }
.test-panel { background: #f9f9f9; border: 1px solid #e0e0e0; border-radius: 10px; padding: 16px; margin-bottom: 20px; }
.test-panel h3 { margin: 0 0 12px; }
.test-panel textarea { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 6px; resize: vertical; box-sizing: border-box; }
.test-panel .btn { margin-top: 8px; }
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
.rule-card { background: #fff; border: 1px solid #eee; border-radius: 10px; padding: 14px; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
.rule-meta { display: flex; gap: 12px; align-items: center; margin-bottom: 8px; font-size: 12px; }
.match-scope { color: #999; }
.priority { color: #ff9800; }
.hit-count { color: #43a047; }
.rule-pattern { font-family: monospace; font-size: 14px; color: #333; margin-bottom: 4px; }
.rule-content { font-size: 14px; color: #555; margin-bottom: 8px; }
.rule-actions { display: flex; gap: 8px; }
.loading, .empty { text-align: center; padding: 48px; color: #999; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; margin-top: 20px; }
.pagination button { padding: 6px 14px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; }
.pagination button:disabled { opacity: 0.4; cursor: not-allowed; }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.4); display: flex; justify-content: center; align-items: center; z-index: 1000; }
.modal { background: #fff; border-radius: 12px; padding: 24px; max-width: 540px; width: 90vw; max-height: 80vh; overflow-y: auto; }
.modal h3 { margin: 0 0 16px; }
.modal label { display: block; font-size: 13px; color: #666; margin: 10px 0 4px; }
.modal input, .modal select, .modal textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; box-sizing: border-box; }
.modal textarea { resize: vertical; }
.form-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
</style>
