<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="worldbook-page">
    <ExtensionPageHeader
      title="世界书"
      description="维护世界观、背景设定和可供角色使用的资料。"
      parent-title="角色"
      parent-path="/characters"
      breadcrumb-label="角色页面层级"
    >
      <template #actions>
        <div v-if="loadedContext" class="header-actions">
        <el-button
          v-if="canWrite"
          size="small"
          :type="testPanelOpen ? 'warning' : 'success'"
          @click="testPanelOpen = !testPanelOpen"
        >
          {{ testPanelOpen ? "关闭测试" : "在线测试" }}
        </el-button>
        <el-button v-if="canWrite" size="small" @click="openImportDialog">JSON导入</el-button>
        <el-button size="small" @click="exportRules">JSON导出</el-button>
        <el-button v-if="canWrite" size="small" type="primary" :disabled="mutationLoading" @click="showAddForm = true"
          >新增规则</el-button
        >
        </div>
      </template>
    </ExtensionPageHeader>
    <pre v-if="operationError && !showAddForm && !editVisible && !importDialogVisible" class="worldbook-error" role="alert" aria-label="世界书原始错误" tabindex="0">{{ operationError }}</pre>
    <el-alert v-if="!canWrite" :title="coreData.access.value.reason || '当前设备只能查看统筹中心世界书'" type="info" :closable="false" show-icon />
    <template v-if="loadedContext">
    <div class="filter-bar">
      <el-select
        v-model="filterType"
        placeholder="全部类型"
        clearable
        size="small"
        style="width: 150px"
        @change="onFilterChange"
      >
        <el-option label="全部类型" value="" />
        <el-option label="正则匹配" value="regex" />
        <el-option label="精确匹配" value="exact" />
        <el-option label="关键词匹配" value="keyword" />
      </el-select>
    </div>

    <div v-if="testPanelOpen" class="test-panel">
      <h3>在线测试</h3>
      <el-input
        v-model="testText"
        type="textarea"
        placeholder="输入测试文本，查看哪些规则命中..."
        :rows="4"
      />
      <el-button
        size="small"
        type="primary"
        @click="runTest"
        :loading="testing"
        :disabled="!testText.trim()"
        style="margin-top: 8px"
        >测试匹配</el-button
      >
      <div v-if="testResults.length > 0" class="test-results">
        <h4>命中规则 ({{ testResults.length }})</h4>
        <div v-for="(r, idx) in testResults" :key="idx" class="test-match-item">
          <div class="match-header">
            <span
              class="match-type-badge"
              :class="'badge-' + r.entry.matchType"
              >{{ matchTypeLabel(r.entry.matchType) }}</span
            >
            <span class="match-pattern">{{ r.entry.matchPattern }}</span>
            <span class="match-priority">优先级: {{ r.entry.priority }}</span>
          </div>
          <div class="match-content">{{ r.entry.injectContent }}</div>
          <div
            class="match-hit-text"
            v-html="highlightWorldBookMatch(r.hitText, r.entry.matchPattern)"
          ></div>
        </div>
      </div>
      <div
        v-if="testText && tested && testResults.length === 0"
        class="no-match"
      >
        无规则命中
      </div>
    </div>

    <el-dialog
      v-model="showAddForm"
      title="新增规则"
      width="500px"
      align-center
      destroy-on-close
      @closed="showAddForm = false"
    >
      <el-form label-width="80px">
        <pre v-if="operationError" class="worldbook-error" role="alert" aria-label="世界书原始错误" tabindex="0">{{ operationError }}</pre>
        <el-form-item label="作用域">
          <el-select v-model="form.scope" style="width: 100%">
            <el-option label="当前角色" value="role" />
            <el-option label="全局世界书" value="global" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配类型">
          <el-select v-model="form.matchType" style="width: 100%">
            <el-option label="正则匹配" value="regex" />
            <el-option label="精确匹配" value="exact" />
            <el-option label="关键词匹配" value="keyword" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配模式">
          <el-input
            v-model="form.matchPattern"
            placeholder="正则表达式/精确文本/关键词(逗号分隔)"
          />
        </el-form-item>
        <el-form-item label="匹配范围">
          <el-select v-model="form.matchScope" style="width: 100%">
            <el-option label="全部上下文" value="full_context" />
            <el-option label="仅用户消息" value="user_message" />
            <el-option label="仅AI回复" value="assistant_reply" />
          </el-select>
        </el-form-item>
        <el-form-item label="注入内容">
          <el-input
            v-model="form.injectContent"
            type="textarea"
            placeholder="匹配命中后注入到上下文的记忆内容"
            :rows="3"
          />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number
            v-model="form.priority"
            :min="0"
            :max="10"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddForm = false">取消</el-button>
        <el-button type="primary" :loading="mutationLoading" :disabled="mutationLoading" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="importDialogVisible"
      title="JSON 导入世界书"
      width="780px"
      align-center
      destroy-on-close
      @closed="resetImportState"
    >
      <div class="json-import-content">
        <pre v-if="operationError" class="worldbook-error" role="alert" aria-label="世界书原始错误" tabindex="0">{{ operationError }}</pre>
        <div
          class="json-import-drop-zone"
          :class="{ 'has-file': !!importFile }"
          @dragover.prevent
          @drop.prevent="onImportDrop"
        >
          <el-icon class="upload-icon"><UploadFilled /></el-icon>
          <template v-if="importFile">
            <strong>{{ importFile.name }}</strong>
            <span>{{ importParsing ? "正在解析..." : `已解析 ${importRows.length} 条规则` }}</span>
          </template>
          <template v-else>
            <strong>拖入 JSON 文件，或点击选择文件</strong>
            <span>JSON 顶层必须是数组，每条记录对应一条世界书规则</span>
          </template>
          <el-button :loading="importParsing" @click="triggerImportFile">
            {{ importFile ? "重新选择" : "选择 JSON" }}
          </el-button>
          <input
            ref="importInput"
            class="hidden-input"
            type="file"
            accept=".json,application/json"
            @change="handleImport"
          />
        </div>

        <el-alert
          v-if="importParseError"
          :title="importParseError"
          type="error"
          show-icon
          :closable="false"
          role="alert"
        />

        <div v-if="importRows.length > 0" class="import-summary">
          <span>共 {{ importRows.length }} 条</span>
          <span class="valid">可导入 {{ importValidCount }} 条</span>
          <span v-if="importInvalidCount > 0" class="invalid">
            需修正 {{ importInvalidCount }} 条
          </span>
        </div>

        <el-table
          v-if="importRows.length > 0"
          :data="importRows"
          border
          size="small"
          max-height="240"
          empty-text="暂无导入数据"
        >
          <el-table-column type="index" label="#" width="52" />
          <el-table-column label="状态" width="76">
            <template #default="{ row }">
              <el-tag :type="row.valid ? 'success' : 'danger'" size="small">
                {{ row.valid ? "有效" : "错误" }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="matchType" label="匹配类型" width="100" />
          <el-table-column prop="matchPattern" label="匹配内容" min-width="170" show-overflow-tooltip />
          <el-table-column prop="message" label="说明" min-width="240" show-overflow-tooltip />
        </el-table>

        <section class="example-panel" aria-labelledby="worldbook-json-example-title">
          <div class="example-panel-header">
            <div>
              <h3 id="worldbook-json-example-title">示例 JSON</h3>
            </div>
            <el-button text :icon="DocumentCopy" @click="copyImportExample">
              复制
            </el-button>
          </div>
          <pre class="example-code"><code>{{ importExampleJson }}</code></pre>
        </section>
      </div>

      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="importLoading"
          :disabled="!canImportJson"
          @click="confirmImport"
        >
          导入 {{ importValidCount }} 条
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="editVisible"
      title="编辑规则"
      width="500px"
      align-center
      destroy-on-close
      @closed="editingEntry = null"
    >
      <el-form label-width="80px">
        <pre v-if="operationError" class="worldbook-error" role="alert" aria-label="世界书原始错误" tabindex="0">{{ operationError }}</pre>
        <el-form-item label="作用域">
          <el-select v-model="editForm.scope" style="width: 100%">
            <el-option label="当前角色" value="role" />
            <el-option label="全局世界书" value="global" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配类型">
          <el-select v-model="editForm.matchType" style="width: 100%">
            <el-option label="正则匹配" value="regex" />
            <el-option label="精确匹配" value="exact" />
            <el-option label="关键词匹配" value="keyword" />
          </el-select>
        </el-form-item>
        <el-form-item label="匹配模式">
          <el-input v-model="editForm.matchPattern" />
        </el-form-item>
        <el-form-item label="匹配范围">
          <el-select v-model="editForm.matchScope" style="width: 100%">
            <el-option label="全部上下文" value="full_context" />
            <el-option label="仅用户消息" value="user_message" />
            <el-option label="仅AI回复" value="assistant_reply" />
          </el-select>
        </el-form-item>
        <el-form-item label="注入内容">
          <el-input
            v-model="editForm.injectContent"
            type="textarea"
            :rows="3"
          />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number
            v-model="editForm.priority"
            :min="0"
            :max="10"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button
          @click="
            editVisible = false;
            editingEntry = null;
          "
          >取消</el-button
        >
        <el-button type="primary" :loading="mutationLoading" :disabled="mutationLoading" @click="handleUpdate">保存</el-button>
      </template>
    </el-dialog>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="rules-list">
      <div v-for="rule in rules" :key="rule.id" class="rule-card">
        <div class="rule-meta">
          <span class="match-type-badge" :class="'badge-' + rule.matchType">{{
            matchTypeLabel(rule.matchType)
          }}</span>
          <span class="match-scope"
            >范围: {{ scopeLabel(rule.matchScope) }}</span
          >
          <span class="priority">优先级: {{ rule.priority }}</span>
          <span class="hit-count">命中: {{ rule.hitCount }}</span>
        </div>
        <div class="rule-pattern">匹配: {{ rule.matchPattern }}</div>
        <div class="rule-content">注入: {{ rule.injectContent }}</div>
        <div v-if="canWrite" class="rule-actions">
          <el-button size="small" :disabled="mutationLoading" @click="startEdit(rule)">编辑</el-button>
          <el-button size="small" type="danger" :disabled="mutationLoading" @click="handleDelete(rule.id)"
            >删除</el-button
          >
        </div>
      </div>
      <div v-if="rules.length === 0" class="empty">暂无世界书规则</div>
    </div>

    <div v-if="totalPages > 1" class="pagination">
      <el-pagination
        v-model:current-page="page"
        :page-size="20"
        :total="total"
        layout="prev, pager, next"
        size="small"
        @current-change="changePage"
      />
    </div>

    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, watch, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { DocumentCopy, UploadFilled } from "@element-plus/icons-vue";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";
import { useWorldBook, type WorldBookEntry } from "@/composables/useWorldBook";
import { useCoreDataAccess } from "@/composables/useCoreDataAccess";
import {
  importExampleJson,
  parseWorldBookImport,
  type WorldBookImportRow,
} from "./worldBookImport";
import { highlightWorldBookMatch } from "./worldBookHighlight";
import { originalErrorText } from "@/runtime/original-error";

const operationError = ref("");

function reportError(error: unknown) {
  operationError.value = originalErrorText(error);
}

const {
  access,
  loadedContext,
  rules,
  loading,
  total,
  page,
  totalPages,
  fetchRules,
  createRule,
  createRules,
  updateRule,
  deleteRule,
  testMatch,
  matchTypeLabel,
  scopeLabel,
} = useWorldBook();
const coreData = useCoreDataAccess();
const canWrite = computed(() => coreData.access.value.write);
const editorContext = ref("");
watch(loadedContext, (current) => {
  if (current !== editorContext.value) {
    showAddForm.value = false;
    editVisible.value = false;
    importDialogVisible.value = false;
    invalidateTestState();
  }
});

const filterType = ref("");
const testPanelOpen = ref(false);
const testText = ref("");
const testResults = ref<any[]>([]);
const tested = ref(false);
const testing = ref(false);
let testRequestVersion = 0;
const mutationLoading = ref(false);
function invalidateTestState() {
  testRequestVersion += 1;
  testing.value = false;
  tested.value = false;
  testResults.value = [];
}
watch(rules, () => {
  invalidateTestState();
}, { deep: true });
watch(testText, invalidateTestState);
watch(testPanelOpen, (open) => {
  if (!open) invalidateTestState();
});
const showAddForm = ref(false);
watch(showAddForm, (open) => { if (open) editorContext.value = loadedContext.value; });
const editVisible = ref(false);
const editingEntry = ref<any>(null);
const importInput = ref<HTMLInputElement | null>(null);
const importDialogVisible = ref(false);
const importFile = ref<File | null>(null);
const importParsing = ref(false);
const importLoading = ref(false);
const importParseError = ref("");
const importRows = ref<WorldBookImportRow[]>([]);

const importValidCount = computed(
  () => importRows.value.filter((row) => row.valid).length,
);
const importInvalidCount = computed(
  () => importRows.value.filter((row) => !row.valid).length,
);
const canImportJson = computed(
  () =>
    canWrite.value
    && importRows.value.length > 0
    && importInvalidCount.value === 0
    && !importParsing.value
    && !importLoading.value,
);

const form = reactive({
  scope: "role" as "role" | "global",
  matchType: "keyword",
  matchPattern: "",
  matchScope: "full_context",
  injectContent: "",
  priority: 0,
});
const editForm = reactive({
  scope: "role" as "role" | "global",
  matchType: "",
  matchPattern: "",
  matchScope: "",
  injectContent: "",
  priority: 0,
});

onMounted(() => {
  loadRules();
});

async function loadRules(filters?: Parameters<typeof fetchRules>[0]) {
  operationError.value = "";
  try {
    await fetchRules(filters);
  } catch (error) {
    reportError(error);
  }
}

function onFilterChange() {
  loadRules({ matchType: filterType.value || undefined });
}

async function runTest() {
  if (!canWrite.value) return;
  const version = ++testRequestVersion;
  const text = testText.value;
  operationError.value = "";
  tested.value = false;
  testResults.value = [];
  testing.value = true;
  try {
    const matches = (await testMatch(text))?.matches || [];
    if (version !== testRequestVersion || text !== testText.value) return;
    testResults.value = matches;
    tested.value = true;
  } catch (error) {
    if (version !== testRequestVersion || text !== testText.value) return;
    reportError(error);
  } finally {
    if (version === testRequestVersion) testing.value = false;
  }
}

async function handleCreate() {
  if (!canWrite.value || mutationLoading.value) return;
  operationError.value = "";
  mutationLoading.value = true;
  try {
    await createRule(form, editorContext.value);
    showAddForm.value = false;
    form.matchType = "keyword";
    form.scope = "role";
    form.matchPattern = "";
    form.matchScope = "full_context";
    form.injectContent = "";
    form.priority = 0;
  } catch (error) {
    reportError(error);
  } finally {
    mutationLoading.value = false;
  }
}

function startEdit(rule: any) {
  if (!canWrite.value) return;
  editorContext.value = loadedContext.value;
  editingEntry.value = rule;
  editForm.scope = rule.scope || "role";
  editForm.matchType = rule.matchType;
  editForm.matchPattern = rule.matchPattern;
  editForm.matchScope = rule.matchScope;
  editForm.injectContent = rule.injectContent;
  editForm.priority = rule.priority;
  editVisible.value = true;
}

async function handleUpdate() {
  if (!canWrite.value || mutationLoading.value) return;
  operationError.value = "";
  mutationLoading.value = true;
  try {
    await updateRule(editingEntry.value.id, { ...editForm }, editorContext.value);
    editVisible.value = false;
    editingEntry.value = null;
  } catch (error) {
    reportError(error);
  } finally {
    mutationLoading.value = false;
  }
}

async function handleDelete(id: string) {
  if (!canWrite.value || mutationLoading.value) return;
  const context = loadedContext.value;
  try {
    await ElMessageBox.confirm("确定删除这条规则？", "删除确认", {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch (error) {
    if (error !== "cancel" && error !== "close") reportError(error);
    return;
  }
  if (mutationLoading.value) return;
  operationError.value = "";
  try {
    mutationLoading.value = true;
    await deleteRule(id, context);
  } catch (error) {
    reportError(error);
  } finally {
    mutationLoading.value = false;
  }
}

function changePage(p: number) {
  loadRules({ page: p, matchType: filterType.value || undefined });
}

function openImportDialog() {
  editorContext.value = loadedContext.value;
  resetImportState();
  importDialogVisible.value = true;
}

function resetImportState() {
  importFile.value = null;
  importParsing.value = false;
  importLoading.value = false;
  importParseError.value = "";
  operationError.value = "";
  importRows.value = [];
  if (importInput.value) importInput.value.value = "";
}

function triggerImportFile() {
  importInput.value?.click();
}

async function handleImport(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (file) await loadImportFile(file);
}

async function onImportDrop(e: DragEvent) {
  const file = e.dataTransfer?.files?.[0];
  if (file) await loadImportFile(file);
}

async function loadImportFile(file: File) {
  if (!file.name.toLowerCase().endsWith(".json")) {
    ElMessage.warning("请选择 JSON 文件");
    return;
  }
  importFile.value = file;
  importParsing.value = true;
  importParseError.value = "";
  importRows.value = [];
  try {
    const text = await file.text();
    importRows.value = parseWorldBookImport(text);
  } catch (error: any) {
    importParseError.value = originalErrorText(error);
    reportError(error);
  } finally {
    importParsing.value = false;
  }
}

async function confirmImport() {
  if (!canImportJson.value || !canWrite.value) return;
  const items = importRows.value
    .filter((row) => row.valid && row.item)
    .map((row) => row.item as Partial<WorldBookEntry>);
  importLoading.value = true;
  operationError.value = "";
  try {
    await createRules(items, editorContext.value);
    ElMessage.success(`导入完成：成功 ${items.length} 条`);
    importDialogVisible.value = false;
  } catch (error: any) {
    reportError(error);
  } finally {
    importLoading.value = false;
  }
}

async function copyImportExample() {
  try {
    await navigator.clipboard.writeText(importExampleJson);
    ElMessage.success("示例 JSON 已复制");
  } catch (error) {
    reportError(error);
  }
}

async function exportRules() {
  operationError.value = "";
  try {
  const context = loadedContext.value;
  await access.requireAccess(context);
  const data = rules.value.map((r) => ({
    matchType: r.matchType,
    matchPattern: r.matchPattern,
    matchScope: r.matchScope,
    injectContent: r.injectContent,
    priority: r.priority,
    characterId: r.characterId,
  }));
  const blob = new Blob([JSON.stringify(data, null, 2)], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "world_book.json";
  a.click();
  URL.revokeObjectURL(url);
  } catch (error) {
    reportError(error);
  }
}
</script>

<style scoped>
.worldbook-error {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  user-select: text;
  color: var(--el-color-danger);
  max-height: 240px;
  overflow: auto;
}
.worldbook-page {
  max-width: 100%;
}
.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.hidden-input {
  display: none;
}
.json-import-content {
  max-height: 68vh;
  overflow-y: auto;
  padding-right: 4px;
}
.json-import-drop-zone {
  min-height: 176px;
  padding: 22px;
  border: 1px dashed var(--ac-color-border);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  background: var(--ac-color-bg-secondary);
  transition: border-color 160ms ease, background 160ms ease;
}
.json-import-drop-zone:hover,
.json-import-drop-zone.has-file {
  border-color: var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
}
.json-import-drop-zone .upload-icon {
  margin-bottom: 4px;
  font-size: 34px;
  color: var(--el-color-primary);
}
.json-import-drop-zone strong {
  font-size: 14px;
}
.json-import-drop-zone span {
  margin-bottom: 6px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.import-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 14px 0 10px;
  font-size: 13px;
}
.import-summary .valid {
  color: var(--el-color-success);
}
.import-summary .invalid {
  color: var(--el-color-danger);
}
.example-panel {
  margin-top: 18px;
  padding: 14px;
  border: 1px solid var(--ac-color-border);
  border-radius: 12px;
  background: var(--ac-color-bg-secondary);
}
.example-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 10px;
}
.example-panel-header h3 {
  margin: 0;
  font-size: 15px;
}
.example-panel-header p {
  margin: 4px 0 0;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.example-code {
  max-height: 320px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border: 1px solid var(--ac-color-border);
  border-radius: 10px;
  background: var(--ac-color-surface);
  color: var(--ac-color-text-primary);
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}

.filter-bar {
  margin-bottom: 16px;
}

.test-panel {
  background: var(--ac-color-bg-secondary);
  border: 1px solid var(--ac-color-border);
  border-radius: 10px;
  padding: 16px;
  margin-bottom: 20px;
}
.test-panel h3 {
  margin: 0 0 12px;
}

.test-results {
  margin-top: 12px;
}
.test-results h4 {
  margin: 0 0 8px;
}
.test-match-item {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border);
  border-radius: 8px;
  padding: 10px;
  margin-bottom: 8px;
}
.match-header {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 4px;
  font-size: 13px;
}
.match-pattern {
  font-family: monospace;
  color: var(--ac-color-text);
}
.match-priority {
  color: var(--ac-color-text-muted);
  font-size: 12px;
}
.match-content {
  color: var(--ac-color-text-secondary);
  font-size: 14px;
  margin-top: 4px;
}
.match-hit-text {
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  margin-top: 4px;
  font-family: monospace;
  background: var(--ac-color-warning-bg);
  padding: 4px 8px;
  border-radius: 4px;
}
.match-hit-text :deep(mark) {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-text);
  padding: 0 2px;
}
.no-match {
  color: var(--ac-color-text-muted);
  text-align: center;
  padding: 16px;
}
.match-type-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  color: var(--tp-text-on-status);
}
.badge-regex {
  background: var(--ac-color-primary);
}
.badge-exact {
  background: var(--ac-color-warning);
}
.badge-keyword {
  background: var(--ac-color-success);
}
.rules-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.rule-card {
  background: var(--ac-color-bg-secondary);
  border: 1px solid var(--ac-color-border);
  border-radius: 10px;
  padding: 14px;
  box-shadow: var(--ac-shadow-sm);
}
.rule-meta {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 8px;
  font-size: 12px;
}
.match-scope {
  color: var(--ac-color-text-muted);
}
.priority {
  color: var(--ac-color-warning);
}
.hit-count {
  color: var(--ac-color-success);
}
.rule-pattern {
  font-family: monospace;
  font-size: 14px;
  color: var(--ac-color-text-primary);
  margin-bottom: 4px;
}
.rule-content {
  font-size: 14px;
  color: var(--ac-color-text-primary);
  margin-bottom: 8px;
}
.rule-actions {
  display: flex;
  gap: 8px;
}
.loading,
.empty {
  text-align: center;
  padding: 48px;
  color: var(--ac-color-text-muted);
}
.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
}
</style>
