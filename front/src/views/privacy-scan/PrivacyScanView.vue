<template>
  <div class="privacy-scan-view">
    <div class="page-header">
      <h2>敏感数据扫描</h2>
      <p class="page-desc">扫描历史记录、记忆和导入数据中的敏感信息，进行脱敏处理</p>
    </div>

    <!-- Notice -->
    <el-alert
      title="不会自动删除数据"
      type="info"
      :closable="false"
      show-icon
      class="notice-alert"
    >
      <template #default>
        <p>扫描仅检测敏感信息，不会自动修改或删除你的数据。脱敏操作需要你手动确认。</p>
      </template>
    </el-alert>

    <!-- Step 1: Select Scan Scope -->
    <el-card class="section-card">
      <template #header>
        <span class="card-title">选择扫描范围</span>
      </template>
      <el-checkbox-group v-model="scope" class="scope-group">
        <el-checkbox label="messages">聊天消息</el-checkbox>
        <el-checkbox label="memories">记忆数据</el-checkbox>
        <el-checkbox label="import_items">导入内容</el-checkbox>
        <el-checkbox label="import_batches">导入批次</el-checkbox>
        <el-checkbox label="logs">操作日志（可选）</el-checkbox>
      </el-checkbox-group>
      <div style="margin-top: 12px">
        <el-button type="primary" :loading="scanning" @click="runScan">
          开始扫描
        </el-button>
      </div>
    </el-card>

    <!-- Scan Summary -->
    <el-card v-if="scanSummary" class="section-card summary-card">
      <template #header>
        <span class="card-title">扫描结果</span>
      </template>
      <div class="summary-stats">
        <div class="summary-stat">
          <span class="summary-stat-label">总计发现</span>
          <span class="summary-stat-value">{{ scanSummary.totalFound }}</span>
        </div>
        <div class="summary-stat high">
          <span class="summary-stat-label">高风险</span>
          <span class="summary-stat-value">{{ scanSummary.highRisk }}</span>
        </div>
        <div class="summary-stat medium">
          <span class="summary-stat-label">中风险</span>
          <span class="summary-stat-value">{{ scanSummary.mediumRisk }}</span>
        </div>
      </div>
    </el-card>

    <!-- Step 2: Filter & View Results -->
    <el-card v-if="scanSummary" class="section-card">
      <template #header>
        <span class="card-title">详细结果</span>
      </template>

      <div class="filter-bar">
        <el-select
          v-model="filter.riskLevel"
          placeholder="风险等级"
          clearable
          style="width: 130px"
          @change="loadResults"
        >
          <el-option label="高风险" value="high" />
          <el-option label="中风险" value="medium" />
          <el-option label="低风险" value="low" />
        </el-select>
        <el-select
          v-model="filter.sourceTable"
          placeholder="数据来源"
          clearable
          style="width: 130px"
          @change="loadResults"
        >
          <el-option
            v-for="st in sourceTables"
            :key="st.source_table"
            :label="sourceTableLabel(st.source_table) + ' (' + st.cnt + ')'"
            :value="st.source_table"
          />
        </el-select>
        <el-select
          v-model="filter.riskType"
          placeholder="风险类型"
          clearable
          style="width: 150px"
          @change="loadResults"
        >
          <el-option
            v-for="rt in riskTypes"
            :key="rt.risk_type"
            :label="rt.risk_type + ' (' + rt.cnt + ')'"
            :value="rt.risk_type"
          />
        </el-select>
      </div>

      <!-- Results Table -->
      <el-table
        :data="results"
        style="width: 100%; margin-top: 12px"
        @selection-change="onSelectionChange"
        size="small"
      >
        <el-table-column type="selection" width="40" />
        <el-table-column prop="risk_level" label="等级" width="70">
          <template #default="{ row }">
            <span :class="['risk-tag', row.risk_level]">
              {{ row.risk_level === 'high' ? '高' : row.risk_level === 'medium' ? '中' : '低' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="risk_type" label="类型" width="100" />
        <el-table-column prop="source_table" label="来源" width="90">
          <template #default="{ row }">
            {{ sourceTableLabel(row.source_table) }}
          </template>
        </el-table-column>
        <el-table-column prop="snippet" label="上下文片段" min-width="200" show-overflow-tooltip />
        <el-table-column prop="masked" label="状态" width="70">
          <template #default="{ row }">
            <span v-if="row.masked" class="masked-yes">已脱敏</span>
            <span v-else class="masked-no">待处理</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="!row.masked"
              type="danger"
              size="small"
              text
              @click="maskSingle(row.id)"
            >
              脱敏
            </el-button>
            <span v-else class="masked-done">—</span>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination-wrap" v-if="totalResults > filter.pageSize">
        <el-pagination
          v-model:current-page="filter.page"
          :page-size="filter.pageSize"
          :total="totalResults"
          layout="prev, pager, next"
          @current-change="loadResults"
          background
          small
        />
      </div>

      <!-- Batch Mask -->
      <div class="batch-actions" v-if="selectedIds.length > 0">
        <span class="batch-info">已选择 {{ selectedIds.length }} 条</span>
        <el-button
          type="danger"
          :disabled="maskConfirmText !== '确认脱敏'"
          :loading="masking"
          @click="batchMask"
        >
          批量脱敏
        </el-button>
        <el-input
          v-model="maskConfirmText"
          placeholder='输入"确认脱敏"'
          style="width: 160px"
          size="small"
        />
      </div>
    </el-card>

    <!-- Mask Result -->
    <el-card v-if="maskResult" class="section-card result-card">
      <template #header>
        <span class="card-title">脱敏完成</span>
      </template>
      <div class="mask-report">
        <div class="report-row">
          <span>脱敏记录数：</span>
          <strong>{{ maskResult.maskedCount }}</strong>
        </div>
        <div class="report-row">
          <span>更新源数据处：</span>
          <strong>{{ maskResult.updatedSourceRecords }}</strong>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { apiClient } from "../../composables/useApi"

// ---- Scan ----
const scope = ref<string[]>(["messages", "memories", "import_items"])
const scanning = ref(false)
const scanSummary = ref<any>(null)

// ---- Filters ----
const filter = reactive({
  riskLevel: "" as string,
  riskType: "" as string,
  sourceTable: "" as string,
  page: 1,
  pageSize: 50,
})

// ---- Results ----
const results = ref<any[]>([])
const totalResults = ref(0)
const riskTypes = ref<any[]>([])
const sourceTables = ref<any[]>([])
const selectedIds = ref<number[]>([])

// ---- Mask ----
const masking = ref(false)
const maskConfirmText = ref("")
const maskResult = ref<any>(null)

function sourceTableLabel(table: string): string {
  const map: Record<string, string> = {
    messages: "聊天消息",
    memories: "记忆",
    import_items: "导入内容",
    import_batches: "导入批次",
    operation_logs: "日志",
  }
  return map[table] || table
}

function onSelectionChange(rows: any[]) {
  selectedIds.value = rows.map(r => r.id)
}

async function runScan() {
  scanning.value = true
  scanSummary.value = null
  results.value = []
  maskResult.value = null
  selectedIds.value = []
  maskConfirmText.value = ""
  filter.page = 1

  try {
    const res = await apiClient.post("/api/privacy/scan", { scope: scope.value })
    const d = res.data?.data || res.data
    scanSummary.value = d
    ElMessage.success(d.message || "扫描完成")
    await loadResults()
  } catch (err: any) {
    ElMessage.error("扫描失败: " + (err.response?.data?.message || err.message))
  } finally {
    scanning.value = false
  }
}

async function loadResults() {
  try {
    const params: any = { page: filter.page, pageSize: filter.pageSize }
    if (filter.riskLevel) params.riskLevel = filter.riskLevel
    if (filter.riskType) params.riskType = filter.riskType
    if (filter.sourceTable) params.sourceTable = filter.sourceTable

    const res = await apiClient.get("/api/privacy/scan-results", { params })
    const d = res.data?.data || res.data
    results.value = d.items || []
    totalResults.value = d.total || 0
    riskTypes.value = d.riskTypes || []
    sourceTables.value = d.sourceTables || []
  } catch (err: any) {
    ElMessage.error("加载结果失败: " + (err.response?.data?.message || err.message))
  }
}

async function maskSingle(id: number) {
  try {
    await ElMessageBox.confirm(
      "将此条记录中的敏感信息替换为 [已脱敏]。此操作不可撤销。",
      "确认脱敏",
      { confirmButtonText: "确认脱敏", cancelButtonText: "取消", type: "warning" }
    )
    const res = await apiClient.post("/api/privacy/mask", { ids: [id], confirmToken: "确认脱敏" })
    const d = res.data?.data || res.data
    maskResult.value = d
    ElMessage.success("已脱敏")
    await loadResults()
  } catch (err: any) {
    if (err !== "cancel" && err !== "close") {
      ElMessage.error("脱敏失败: " + (err.response?.data?.message || err.message))
    }
  }
}

async function batchMask() {
  if (maskConfirmText.value !== "确认脱敏") return
  masking.value = true
  try {
    const res = await apiClient.post("/api/privacy/mask", {
      ids: selectedIds.value,
      confirmToken: "确认脱敏",
    })
    const d = res.data?.data || res.data
    maskResult.value = d
    ElMessage.success(`已脱敏 ${d.maskedCount} 条`)
    selectedIds.value = []
    maskConfirmText.value = ""
    await loadResults()
  } catch (err: any) {
    ElMessage.error("批量脱敏失败: " + (err.response?.data?.message || err.message))
  } finally {
    masking.value = false
  }
}
</script>

<style scoped>
.privacy-scan-view {
  padding: 20px;
  max-width: 900px;
}

.page-header {
  margin-bottom: 16px;
}
.page-header h2 {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: var(--el-text-color-primary);
}
.page-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin: 0;
}

.notice-alert {
  margin-bottom: 16px;
}
.notice-alert p {
  margin: 0;
  font-size: 13px;
}

/* Section Card */
.section-card {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-light);
}
.card-title {
  font-size: 15px;
  font-weight: 600;
}

.scope-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
}

/* Summary Stats */
.summary-stats {
  display: flex;
  gap: 20px;
}
.summary-stat {
  padding: 10px 20px;
  background: var(--el-fill-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  text-align: center;
  min-width: 100px;
}
.summary-stat.high {
  border-color: #dc2626;
  background: #fef2f2;
}
.summary-stat.high .summary-stat-value {
  color: #dc2626;
}
.summary-stat.medium {
  border-color: #d97706;
  background: #fffbeb;
}
.summary-stat.medium .summary-stat-value {
  color: #d97706;
}
.summary-stat-label {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
.summary-stat-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

/* Filter Bar */
.filter-bar {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

/* Risk Tags */
.risk-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}
.risk-tag.high {
  background: #fef2f2;
  color: #dc2626;
  border: 1px solid #fecaca;
}
.risk-tag.medium {
  background: #fffbeb;
  color: #d97706;
  border: 1px solid #fde68a;
}
.risk-tag.low {
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
  border: 1px solid var(--el-border-color-light);
}

.masked-yes {
  color: var(--el-color-success);
  font-size: 12px;
}
.masked-no {
  color: var(--el-color-danger);
  font-size: 12px;
  font-weight: 500;
}
.masked-done {
  color: var(--el-text-color-placeholder);
}

/* Batch Actions */
.batch-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-extra-light);
}
.batch-info {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* Pagination */
.pagination-wrap {
  display: flex;
  justify-content: center;
  margin-top: 12px;
}

/* Mask Result */
.mask-report {
  font-size: 14px;
}
.report-row {
  padding: 6px 0;
}
.report-row strong {
  color: var(--el-text-color-primary);
}

.result-card {
  border-color: var(--el-color-success-light-5);
}

/* Mobile */
@media (max-width: 600px) {
  .privacy-scan-view {
    padding: 12px;
    max-width: 100%;
  }
  .summary-stats {
    flex-wrap: wrap;
    gap: 8px;
  }
  .summary-stat {
    flex: 1;
    min-width: 80px;
  }
  .filter-bar {
    flex-direction: column;
  }
  .filter-bar .el-select {
    width: 100% !important;
  }
  .batch-actions {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
