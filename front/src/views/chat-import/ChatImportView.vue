<template>
  <div class="import-page">
    <h2 class="page-title">导入聊天记录</h2>

    <!-- Privacy notice -->
    <el-alert type="warning" :closable="true" show-icon style="margin-bottom:12px">
      <template #title>
        导入内容可能包含隐私信息，需自行移除验证码、密码、银行卡号和身份证号再导入。
      </template>
    </el-alert>

    <!-- 步骤1: 输入 -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <span class="step-badge">1</span> 粘贴聊天记录
      </template>

      <div class="input-area">
        <el-input
          v-model="rawText"
          type="textarea"
          :rows="8"
          placeholder="Paste your chat history. Examples:

User: I'm a bit tired today
AI: Take a rest then.

[2026-05-18 12:00] Me: What's for dinner?
[2026-05-18 12:01] You: Whatever you want

2026/05/18 12:00 Zhang San
Hello!
2026/05/18 12:01 Li Si
Hey there!"
        />

        <div class="input-options">
          <el-input v-model="batchTitle" placeholder="标题（可选）" size="small" style="width:200px" />

          <div class="format-picker">
            <span class="fp-label">格式：</span>
            <el-radio-group v-model="parseFormat" size="small">
              <el-radio-button value="auto">自动</el-radio-button>
              <el-radio-button value="standard">标准</el-radio-button>
              <el-radio-button value="timestamp">时间戳</el-radio-button>
              <el-radio-button value="multiline">多行</el-radio-button>
              <el-radio-button value="wechat">微信</el-radio-button>
            </el-radio-group>
          </div>
        </div>

        <!-- Custom speaker names -->
        <el-collapse v-model="showSpeakerOptions" style="border:none">
          <el-collapse-item title="自定义发言者名称映射" name="options">
            <div class="speaker-options">
              <div class="so-group">
                <span class="so-label">用户发言者（逗号分隔）：</span>
                <el-input
                  v-model="userSpeakerInput"
                  placeholder="例如：张三、我、自己"
                  size="small"
                />
              </div>
              <div class="so-group">
                <span class="so-label">AI 发言者（逗号分隔）：</span>
                <el-input
                  v-model="assistantSpeakerInput"
                  placeholder="例如：AI、李四、Bot"
                  size="small"
                />
              </div>
            </div>
          </el-collapse-item>
        </el-collapse>

        <div class="input-actions">
          <el-button type="primary" :icon="Reading" :loading="parsing" @click="handleParse" :disabled="!rawText.trim()">
            Parse
          </el-button>
          <el-upload :auto-upload="false" :show-file-list="false" :on-change="onFileChange" accept=".txt,.md">
            <el-button :icon="Upload">上传 .txt / .md</el-button>
          </el-upload>
        </div>
      </div>
    </el-card>

    <!-- 步骤2: 预览与编辑 -->
    <el-card shadow="never" class="section-card" v-if="parseResult">
      <template #header>
        <span class="step-badge">2</span> 预览与编辑（{{ parseResult.items?.length || 0 }} 条消息）
        <span class="detected-tag">
          检测到： <el-tag size="small" type="info">{{ parseResult.detectedFormat || 'auto' }}</el-tag>
        </span>
      </template>

      <!-- Warnings -->
      <div v-if="parseResult.warnings?.length" class="warnings-block">
        <el-alert
          v-for="(w, i) in parseResult.warnings"
          :key="i"
          :title="w.message || w"
          :type="warningType(w)"
          :closable="false"
          show-icon
          style="margin-bottom:4px"
        />
      </div>

      <!-- High-risk warning -->
      <el-alert
        v-if="parseResult.hasHighRisk"
        type="error"
        :closable="false"
        show-icon
        style="margin-bottom:8px"
      >
        <template #title>检测到高风险敏感数据，确认前请仔细检查。</template>
      </el-alert>

      <!-- Editable table with confidence -->
      <el-table :data="editableItems" stripe size="small" max-height="400">
        <el-table-column prop="lineNo" label="#" width="50" />
        <el-table-column label="发言者" width="100">
          <template #default="{row, $index}">
            <el-input v-model="editableItems[$index].speaker" size="small" />
          </template>
        </el-table-column>
        <el-table-column label="角色" width="90">
          <template #default="{row, $index}">
            <el-select v-model="editableItems[$index].role" size="small">
              <el-option label="用户" value="user" />
              <el-option label="AI" value="assistant" />
              <el-option label="系统" value="system" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="内容">
          <template #default="{row, $index}">
            <el-input
              v-model="editableItems[$index].content"
              size="small"
              :class="{ 'is-sensitive': row._sensitive }"
            />
          </template>
        </el-table-column>
        <el-table-column label="置信度" width="90" align="center">
          <template #default="{row}">
            <el-progress
              :percentage="Math.round((row.confidence || 0) * 100)"
              :stroke-width="6"
              :show-text="true"
              :color="confidenceColor(row.confidence)"
            />
          </template>
        </el-table-column>
        <el-table-column label="时间" width="100">
          <template #default="{row, $index}">
            <el-input
              v-model="editableItems[$index].timestamp"
              size="small"
              placeholder="HH:MM"
            />
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 步骤3: 确认选项 -->
    <el-card shadow="never" class="section-card" v-if="parseResult">
      <template #header>
        <span class="step-badge">3</span> 确认导入
      </template>

      <div class="confirm-options">
        <el-checkbox v-model="genSummary">生成会话摘要</el-checkbox>
        <el-checkbox v-model="extractMemories">提取记忆候选项</el-checkbox>
        <span class="confirm-hint">
          将从导入的消息创建一个新的会话。
        </span>
      </div>

      <div style="margin-top:12px">
        <el-button
          type="primary"
          size="large"
          :loading="confirming"
          :disabled="editableItems.length === 0"
          @click="handleConfirm"
        >
          确认导入（{{ editableItems.length }} 条消息）
        </el-button>
      </div>
    </el-card>

    <!-- 导入后处理操作 -->
    <el-card shadow="never" class="section-card" v-if="importedBatchId">
      <template #header>
        <span class="step-badge">4</span> 导入后处理
      </template>

      <div class="post-actions">
        <el-button :loading="genSummaryLoading" @click="handleGenSummary" v-if="genSummary">
          Generate Summary
        </el-button>
        <el-button :loading="extractLoading" @click="handleExtractMemories" v-if="extractMemories">
          Extract Memories
        </el-button>
        <router-link v-if="importedConvId" :to="'/logs'" class="inline-link">
          查看已导入会话
        </router-link>
      </div>

      <div v-if="memCandidates.length > 0" style="margin-top:10px">
        <div v-for="c in memCandidates.slice(0, 5)" :key="c.key" class="mem-candidate">
          <el-tag size="small">{{ c.key }}</el-tag>
          <span class="mc-val">{{ c.value }}</span>
          <span class="mc-imp">重要性： {{ c.importance }}/10</span>
        </div>
        <el-button text size="small" @click="router.push('/memory')" v-if="memCandidates.length > 0">
          在记忆中管理
        </el-button>
      </div>
    </el-card>

    <!-- 导入历史 -->
    <el-card shadow="never" class="section-card">
      <template #header>导入历史</template>
      <el-table :data="batches" size="small" v-if="batches.length > 0">
        <el-table-column prop="fileName" label="名称" show-overflow-tooltip />
        <el-table-column label="状态" width="90">
          <template #default="{row}">
            <el-tag :type="row.status === 'completed' ? 'success' : 'info'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="totalItems" label="条目数" width="60" />
        <el-table-column label="日期" width="140">
          <template #default="{row}">{{ fmtDate(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160">
          <template #default="{row}">
            <el-button text size="small" @click="viewBatch(row.id)">查看</el-button>
            <el-button text size="small" type="danger" @click="delBatch(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="暂无导入历史" :image-size="50" />
    </el-card>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailVisible" title="批次详情" width="600px">
      <el-table :data="detailItems" size="small" max-height="350">
        <el-table-column prop="lineNo" label="#" width="50" v-if="false" />
        <el-table-column prop="senderName" label="发言者" width="80" />
        <el-table-column prop="role" label="角色" width="70" />
        <el-table-column prop="content" label="内容" show-overflow-tooltip />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import { Reading, Upload, Delete, Collection, Switch } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const router = useRouter()
const { get, post, del, loading: apiLoading } = useApi()

// ---- 输入状态 ----
const rawText = ref("")
const batchTitle = ref("")
const parseFormat = ref("auto")
const parsing = ref(false)
const showSpeakerOptions = ref<string[]>([])
const userSpeakerInput = ref("")
const assistantSpeakerInput = ref("")

function parseSpeakerNames(input: string): string[] {
  if (!input.trim()) return []
  return input.split(/[,;，；]/).map(s => s.trim()).filter(Boolean)
}

// ---- 解析结果 ----
const parseResult = ref<any>(null)
const editableItems = ref<any[]>([])
const confirming = ref(false)
const genSummary = ref(false)
const extractMemories = ref(false)

// ---- 导入后处理 ----
const importedBatchId = ref("")
const importedConvId = ref("")
const genSummaryLoading = ref(false)
const extractLoading = ref(false)
const confirmMemLoading = ref(false)
const continueLoading = ref(false)
const showCloudWarning = ref(false)
const summaryData = ref<any>(null)
const selectedCandidateIds = ref<string[]>([])
const highRiskCount = ref(0)
const memCandidates = ref<any[]>([])

// ---- 导入历史 ----
const batches = ref<any[]>([])
const batchTotal = ref(0)
const batchPage = ref(1)
const detailVisible = ref(false)
const detailItems = ref<any[]>([])

// ---- 警告类型辅助 ----
function warningType(w: any): "error" | "warning" | "info" {
  const type = typeof w === "string" ? "" : (w.type || "")
  if (type === "sensitive_data" || type === "empty_content") return "error"
  if (type === "low_confidence") return "warning"
  if (type === "unknown_speaker") return "info"
  return "warning"
}

// ---- 置信度颜色 ----
function confidenceColor(conf: number): string {
  if (conf >= 0.8) return "#67c23a"
  if (conf >= 0.5) return "#e6a23c"
  return "#f56c6c"
}

// ---- 解析 ----
async function handleParse() {
  if (!rawText.value.trim()) return
  parsing.value = true
  try {
    const body: any = {
      rawText: rawText.value,
      format: parseFormat.value,
      title: batchTitle.value || undefined,
    }

    const userNames = parseSpeakerNames(userSpeakerInput.value)
    const assistantNames = parseSpeakerNames(assistantSpeakerInput.value)
    if (userNames.length > 0) body.userSpeakerNames = userNames
    if (assistantNames.length > 0) body.assistantSpeakerNames = assistantNames

    const result = await post<any>("/api/imports/parse-text", body)
    parseResult.value = result
    editableItems.value = (result?.items || []).map((item: any, idx: number) => ({
      ...item,
      lineNo: idx + 1,
      _sensitive: result?.sensitiveMatches?.some((m: any) => m.lineNo === item.lineNo) || false,
    }))
    importedBatchId.value = ""
    importedConvId.value = ""
  } catch {
    // 由拦截器处理
  } finally {
    parsing.value = false
  }
}

function onFileChange(file: any) {
  const reader = new FileReader()
  reader.onload = (e) => {
    rawText.value = (e.target?.result as string) || ""
    batchTitle.value = file.name.replace(/\.[^.]+$/, "")
    handleParse()
  }
  reader.readAsText(file.raw)
}

// ---- 确认 ----
async function handleConfirm() {
  if (editableItems.value.length === 0) return

  await ElMessageBox.confirm(
    `确认导入 ${editableItems.value.length} 条消息？将创建一个新的会话。`,
    "确认导入",
    { type: "warning", confirmButtonText: "确认" }
  )

  confirming.value = true
  try {
    const result = await post<any>("/api/imports/confirm", {
      batchId: parseResult.value?.batchId,
      title: batchTitle.value || "已导入的聊天",
    })
    importedBatchId.value = result?.batchId || parseResult.value?.batchId
    importedConvId.value = result?.conversationId || ""
    ElMessage.success(`成功导入 ${result?.messageCount || editableItems.value.length} 条消息`)
    await fetchBatches()

    if (genSummary.value) await handleGenSummary()
    if (extractMemories.value) await handleExtractMemories()
  } catch {
    // handled by interceptor
  } finally {
    confirming.value = false
  }
}

// ---- 导入后处理操作 ----
async function handleGenSummary() {
  if (!importedBatchId.value) return
  genSummaryLoading.value = true
  showCloudWarning.value = true
  try {
    const result = await post<any>(`/api/imports/batches/${importedBatchId.value}/generate-summary`)
    if (result?.summary) {
      summaryData.value = result.summary
    }
    // 同时尝试GET获取已保存的摘要
    try {
      const saved = await get<any>(`/api/imports/batches/${importedBatchId.value}/summary`)
      if (saved?.summary) {
        summaryData.value = saved.summary
      }
    } catch {}
    ElMessage.success("摘要生成成功")
  } catch (err: any) {
    ElMessage.error(err?.message || "摘要生成失败")
  }
  genSummaryLoading.value = false
}

async function handleExtractMemories() {
  if (!importedBatchId.value) return
  extractLoading.value = true
  showCloudWarning.value = true
  try {
    const result = await post<any>(`/api/imports/batches/${importedBatchId.value}/extract-memory-candidates`)
    if (result?.candidates) {
      memCandidates.value = result.candidates
      highRiskCount.value = result.highRiskCount || result.candidates.filter((c: any) => c.riskLevel === 'high').length
      // 自动选中低中风险，高风险不选中
      selectedCandidateIds.value = result.candidates
        .filter((c: any) => c.riskLevel !== 'high')
        .map((c: any) => c.id)
    }
    ElMessage.success(`Extracted ${memCandidates.value.length} memory candidates (${highRiskCount.value} high-risk)`)
  } catch (err: any) {
    ElMessage.error(err?.message || "Failed to extract memory candidates")
  }
  extractLoading.value = false
}

async function handleContinueChat() {
  if (!importedBatchId.value) return
  continueLoading.value = true
  try {
    const result = await post<any>("/api/web-chat/conversations/from-import", {
      importBatchId: importedBatchId.value,
    })
    if (result?.id) {
      ElMessage.success("Conversation created! Redirecting to chat...")
      router.push(`/chat/${result.id}`)
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "Failed to create conversation")
  }
  continueLoading.value = false
}

async function handleConfirmMemories() {
  if (!importedBatchId.value || selectedCandidateIds.value.length === 0) return

  await ElMessageBox.confirm(
    `Save ${selectedCandidateIds.value.length} selected memories? This will add them to the active character's memory store.`,
    "Confirm Memories",
    { type: "warning", confirmButtonText: "Save" }
  )

  confirmMemLoading.value = true
  try {
    const result = await post<any>(
      `/api/imports/batches/${importedBatchId.value}/confirm-memories`,
      { selectedIds: selectedCandidateIds.value }
    )
    ElMessage.success(`已保存 ${result?.savedCount || 0} 条记忆`)
    // 确认后清除候选项
    memCandidates.value = []
    selectedCandidateIds.value = []
  } catch (err: any) {
    ElMessage.error(err?.message || "保存记忆失败")
  }
  confirmMemLoading.value = false
}

// ---- Batch history ----
async function fetchBatches() {
  try {
    const r = await get<any>("/api/imports/batches", { page: batchPage.value, pageSize: 20 })
    batches.value = r?.items || []
    batchTotal.value = r?.total || 0
  } catch {}
}

async function viewBatch(id: string) {
  try {
    const r = await get<any>(`/api/imports/batches/${id}`)
    detailItems.value = r?.items || r || []
    detailVisible.value = true
  } catch {}
}

async function delBatch(id: string) {
  await ElMessageBox.confirm("确定删除此批次？", "确认", { type: "warning" })
  try {
    await del(`/api/imports/batches/${id}`)
    ElMessage.success("已删除")
    await fetchBatches()
  } catch {}
}

function fmtDate(d: string): string {
  if (!d) return ""
  try { return new Date(d).toLocaleString("zh-CN") } catch { return d }
}

onMounted(fetchBatches)
</script>

<style scoped>
.import-page { }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin-bottom: 14px; color: var(--ac-color-text); }
.section-card { margin-bottom: 12px; }

.step-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 22px; height: 22px; border-radius: 50%;
  background: var(--ac-color-primary); color: #fff;
  font-size: 11px; font-weight: 700; margin-right: 6px; flex-shrink: 0;
}

.detected-tag { margin-left: 12px; font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }

.input-area { display: flex; flex-direction: column; gap: 10px; }
.input-options { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.format-picker { display: flex; align-items: center; gap: 6px; }
.fp-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); white-space: nowrap; }
.input-actions { display: flex; gap: 8px; }

.speaker-options { display: flex; flex-direction: column; gap: 10px; padding: 8px 0; }
.so-group { display: flex; flex-direction: column; gap: 4px; }
.so-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); }

.warnings-block { margin-bottom: 10px; }

:deep(.is-sensitive .el-input__inner) { border-color: var(--ac-color-danger) !important; background: rgba(200,90,90,0.04); }

.confirm-options { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.confirm-hint { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); flex-basis: 100%; margin-top: 4px; }

.post-actions { margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--ac-color-border-light); display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.inline-link { display: inline-block; margin-left: 8px; font-size: var(--ac-font-size-sm); color: var(--ac-color-primary); text-decoration: underline; }

.mem-candidate { display: flex; align-items: center; gap: 10px; padding: 6px 0; }
.mc-val { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mc-imp { font-size: 10px; color: var(--ac-color-text-muted); white-space: nowrap; }

.summary-section { margin-top: 14px; }
.summary-block { margin-bottom: 12px; padding: 10px 14px; background: var(--ac-color-fill); border-radius: 6px; }
.summary-block h4 { margin: 0 0 6px; font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-primary); }
.summary-block p { margin: 0; font-size: var(--ac-font-size-sm); line-height: 1.6; color: var(--ac-color-text); }
.summary-block.overall { background: var(--ac-color-primary-light-9); border-left: 3px solid var(--ac-color-primary); }

.candidate-list { margin-top: 8px; }
.candidate-item {
  padding: 8px 4px;
  border-bottom: 1px solid var(--ac-color-border-lighter);
}
.candidate-item.is-high-risk {
  background: rgba(245, 108, 108, 0.04);
}
.candidate-item:last-child { border-bottom: none; }
.ci-content { margin-left: 4px; flex: 1; }
.ci-header { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.ci-importance { font-size: 10px; color: var(--ac-color-warning); letter-spacing: 2px; }
.ci-text { margin: 0 0 2px; font-size: var(--ac-font-size-sm); color: var(--ac-color-text); line-height: 1.5; }
.ci-reason { margin: 0; font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); font-style: italic; }

.confirm-mem-actions { margin-top: 14px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }

@media (max-width: 768px) {
  .input-options { flex-direction: column; align-items: stretch; }
}
</style>
