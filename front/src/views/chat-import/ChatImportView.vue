<template>
  <div class="import-page">
    <h2 class="page-title">Import Chat History</h2>

    <!-- Privacy notice -->
    <el-alert type="warning" :closable="true" show-icon style="margin-bottom:12px">
      <template #title>
        Imported content may contain private info. Please remove verification codes, passwords, bank cards, and ID numbers before importing.
      </template>
    </el-alert>

    <!-- Step 1: Input -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <span class="step-badge">1</span> Paste Chat History
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
          <el-input v-model="batchTitle" placeholder="Title (optional)" size="small" style="width:200px" />

          <div class="format-picker">
            <span class="fp-label">Format:</span>
            <el-radio-group v-model="parseFormat" size="small">
              <el-radio-button value="auto">Auto</el-radio-button>
              <el-radio-button value="standard">Standard</el-radio-button>
              <el-radio-button value="timestamp">Timestamps</el-radio-button>
              <el-radio-button value="multiline">Multi-line</el-radio-button>
              <el-radio-button value="wechat">WeChat</el-radio-button>
            </el-radio-group>
          </div>
        </div>

        <!-- Custom speaker names -->
        <el-collapse v-model="showSpeakerOptions" style="border:none">
          <el-collapse-item title="Custom speaker name mapping" name="options">
            <div class="speaker-options">
              <div class="so-group">
                <span class="so-label">User speakers (comma-separated):</span>
                <el-input
                  v-model="userSpeakerInput"
                  placeholder="e.g. Zhang San, Me, Myself"
                  size="small"
                />
              </div>
              <div class="so-group">
                <span class="so-label">AI speakers (comma-separated):</span>
                <el-input
                  v-model="assistantSpeakerInput"
                  placeholder="e.g. AI, Li Si, Bot"
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
            <el-button :icon="Upload">Upload .txt / .md</el-button>
          </el-upload>
        </div>
      </div>
    </el-card>

    <!-- Step 2: Preview & Edit -->
    <el-card shadow="never" class="section-card" v-if="parseResult">
      <template #header>
        <span class="step-badge">2</span> Preview & Edit ({{ parseResult.items?.length || 0 }} messages)
        <span class="detected-tag">
          Detected: <el-tag size="small" type="info">{{ parseResult.detectedFormat || 'auto' }}</el-tag>
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
        <template #title>High-risk sensitive data detected. Please review carefully before confirming.</template>
      </el-alert>

      <!-- Editable table with confidence -->
      <el-table :data="editableItems" stripe size="small" max-height="400">
        <el-table-column prop="lineNo" label="#" width="50" />
        <el-table-column label="Speaker" width="100">
          <template #default="{row, $index}">
            <el-input v-model="editableItems[$index].speaker" size="small" />
          </template>
        </el-table-column>
        <el-table-column label="Role" width="90">
          <template #default="{row, $index}">
            <el-select v-model="editableItems[$index].role" size="small">
              <el-option label="User" value="user" />
              <el-option label="AI" value="assistant" />
              <el-option label="System" value="system" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="Content">
          <template #default="{row, $index}">
            <el-input
              v-model="editableItems[$index].content"
              size="small"
              :class="{ 'is-sensitive': row._sensitive }"
            />
          </template>
        </el-table-column>
        <el-table-column label="Confidence" width="90" align="center">
          <template #default="{row}">
            <el-progress
              :percentage="Math.round((row.confidence || 0) * 100)"
              :stroke-width="6"
              :show-text="true"
              :color="confidenceColor(row.confidence)"
            />
          </template>
        </el-table-column>
        <el-table-column label="Time" width="100">
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

    <!-- Step 3: Options & Confirm -->
    <el-card shadow="never" class="section-card" v-if="parseResult">
      <template #header>
        <span class="step-badge">3</span> Confirm Import
      </template>

      <div class="confirm-options">
        <el-checkbox v-model="genSummary">Generate conversation summary</el-checkbox>
        <el-checkbox v-model="extractMemories">Extract memory candidates</el-checkbox>
        <span class="confirm-hint">
          A new conversation will be created from the imported messages.
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
          Confirm Import ({{ editableItems.length }} messages)
        </el-button>
      </div>
    </el-card>

    <!-- Post-import actions -->
    <el-card shadow="never" class="section-card" v-if="importedBatchId">
      <template #header>
        <span class="step-badge">4</span> Post-Import
      </template>

      <div class="post-actions">
        <el-button :loading="genSummaryLoading" @click="handleGenSummary" v-if="genSummary">
          Generate Summary
        </el-button>
        <el-button :loading="extractLoading" @click="handleExtractMemories" v-if="extractMemories">
          Extract Memories
        </el-button>
        <router-link v-if="importedConvId" :to="'/logs'" class="inline-link">
          View imported conversation
        </router-link>
      </div>

      <div v-if="memCandidates.length > 0" style="margin-top:10px">
        <div v-for="c in memCandidates.slice(0, 5)" :key="c.key" class="mem-candidate">
          <el-tag size="small">{{ c.key }}</el-tag>
          <span class="mc-val">{{ c.value }}</span>
          <span class="mc-imp">importance: {{ c.importance }}/10</span>
        </div>
        <el-button text size="small" @click="router.push('/memory')" v-if="memCandidates.length > 0">
          Manage in Memory
        </el-button>
      </div>
    </el-card>

    <!-- Batch History -->
    <el-card shadow="never" class="section-card">
      <template #header>Import History</template>
      <el-table :data="batches" size="small" v-if="batches.length > 0">
        <el-table-column prop="fileName" label="Name" show-overflow-tooltip />
        <el-table-column label="Status" width="90">
          <template #default="{row}">
            <el-tag :type="row.status === 'completed' ? 'success' : 'info'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="totalItems" label="Items" width="60" />
        <el-table-column label="Date" width="140">
          <template #default="{row}">{{ fmtDate(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="Actions" width="160">
          <template #default="{row}">
            <el-button text size="small" @click="viewBatch(row.id)">View</el-button>
            <el-button text size="small" type="danger" @click="delBatch(row.id)">Delete</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No import history" :image-size="50" />
    </el-card>

    <!-- Detail dialog -->
    <el-dialog v-model="detailVisible" title="Batch Detail" width="600px">
      <el-table :data="detailItems" size="small" max-height="350">
        <el-table-column prop="lineNo" label="#" width="50" v-if="false" />
        <el-table-column prop="senderName" label="Speaker" width="80" />
        <el-table-column prop="role" label="Role" width="70" />
        <el-table-column prop="content" label="Content" show-overflow-tooltip />
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

// ---- Input state ----
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

// ---- Parse result ----
const parseResult = ref<any>(null)
const editableItems = ref<any[]>([])
const confirming = ref(false)
const genSummary = ref(false)
const extractMemories = ref(false)

// ---- Post-import ----
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

// ---- Batch history ----
const batches = ref<any[]>([])
const batchTotal = ref(0)
const batchPage = ref(1)
const detailVisible = ref(false)
const detailItems = ref<any[]>([])

// ---- Warning type helper ----
function warningType(w: any): "error" | "warning" | "info" {
  const type = typeof w === "string" ? "" : (w.type || "")
  if (type === "sensitive_data" || type === "empty_content") return "error"
  if (type === "low_confidence") return "warning"
  if (type === "unknown_speaker") return "info"
  return "warning"
}

// ---- Confidence color ----
function confidenceColor(conf: number): string {
  if (conf >= 0.8) return "#67c23a"
  if (conf >= 0.5) return "#e6a23c"
  return "#f56c6c"
}

// ---- Parse ----
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
    // handled by interceptor
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

// ---- Confirm ----
async function handleConfirm() {
  if (editableItems.value.length === 0) return

  await ElMessageBox.confirm(
    `Confirm importing ${editableItems.value.length} messages? A new conversation will be created.`,
    "Confirm Import",
    { type: "warning", confirmButtonText: "Confirm" }
  )

  confirming.value = true
  try {
    const result = await post<any>("/api/imports/confirm", {
      batchId: parseResult.value?.batchId,
      title: batchTitle.value || "Imported Chat",
    })
    importedBatchId.value = result?.batchId || parseResult.value?.batchId
    importedConvId.value = result?.conversationId || ""
    ElMessage.success(`Successfully imported ${result?.messageCount || editableItems.value.length} messages`)
    await fetchBatches()

    if (genSummary.value) await handleGenSummary()
    if (extractMemories.value) await handleExtractMemories()
  } catch {
    // handled by interceptor
  } finally {
    confirming.value = false
  }
}

// ---- Post-import actions ----
async function handleGenSummary() {
  if (!importedBatchId.value) return
  genSummaryLoading.value = true
  showCloudWarning.value = true
  try {
    const result = await post<any>(`/api/imports/batches/${importedBatchId.value}/generate-summary`)
    if (result?.summary) {
      summaryData.value = result.summary
    }
    // Also try GET to fetch the saved summary
    try {
      const saved = await get<any>(`/api/imports/batches/${importedBatchId.value}/summary`)
      if (saved?.summary) {
        summaryData.value = saved.summary
      }
    } catch {}
    ElMessage.success("Summary generated successfully")
  } catch (err: any) {
    ElMessage.error(err?.message || "Failed to generate summary")
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
      // Auto-select low and medium risk, leave high risk unchecked
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
    ElMessage.success(`Saved ${result?.savedCount || 0} memories`)
    // Clear candidates after confirmation
    memCandidates.value = []
    selectedCandidateIds.value = []
  } catch (err: any) {
    ElMessage.error(err?.message || "Failed to save memories")
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
  await ElMessageBox.confirm("Delete this batch?", "Confirm", { type: "warning" })
  try {
    await del(`/api/imports/batches/${id}`)
    ElMessage.success("Deleted")
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
