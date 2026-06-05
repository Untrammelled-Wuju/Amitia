<template>
  <div class="page">
    <h2 class="page-title">模型配置</h2>

    <!-- Privacy notice -->
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:14px">
      <template #title>
        使用云端模型时，聊天内容会发送到模型服务商。请勿在聊天中发送密码、验证码、银行卡等敏感信息。
      </template>
    </el-alert>

    <!-- Toolbar -->
    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showDialog(null)">新增配置</el-button>
    </div>

    <!-- Config Cards -->
    <div class="config-cards" v-if="configs.length > 0">
      <div
        v-for="cfg in configs"
        :key="cfg.id"
        class="config-card"
        :class="{ 'is-active': cfg.isActive }"
      >
        <div class="card-top">
          <div class="card-header">
            <span class="card-name">{{ cfg.name }}</span>
            <el-tag v-if="cfg.isActive" type="success" size="small" effect="dark">当前</el-tag>
          </div>
          <div class="card-type">
              <el-tag size="small" :type="cfg.apiType === 'ollama' ? 'info' : 'primary'">
              {{ providerName(cfg.apiType) }}
            </el-tag>
            <span class="card-model">{{ cfg.modelName }}</span>
          </div>
        </div>

        <div class="card-details">
          <div class="detail-row">
            <span class="dl">Base URL</span>
            <span class="dv">{{ cfg.baseUrl || "未设置" }}</span>
          </div>
          <div class="detail-row">
            <span class="dl">API Key</span>
            <span class="dv">{{ cfg.hasApiKey ? "已设置" : "未设置" }}</span>
          </div>
          <div class="detail-row">
            <span class="dl">温度</span>
            <span class="dv">{{ cfg.temperature ?? 0.7 }}</span>
          </div>
          <div class="detail-row">
            <span class="dl">最大 Token</span>
            <span class="dv">{{ cfg.maxTokens ?? 4096 }}</span>
          </div>
        </div>

        <!-- Test result -->
        <div class="card-test" v-if="cfg.lastTestStatus">
          <div class="test-indicator" :class="cfg.lastTestStatus">
            <span class="test-dot"></span>
            <span>上次测试: {{ cfg.lastTestStatus === "success" ? "通过" : "失败" }}</span>
          </div>
          <div class="test-msg" v-if="cfg.lastTestMessage">{{ cfg.lastTestMessage }}</div>
          <div class="test-time" v-if="cfg.lastTestAt">{{ fmtDate(cfg.lastTestAt) }}</div>
        </div>

        <div class="card-actions">
          <el-button
            size="small"
            :loading="testingId === cfg.id"
            @click="testConfig(cfg.id)"
          >
            {{ testingId === cfg.id ? "测试中..." : "测试连接" }}
          </el-button>
          <el-button size="small" @click="showDialog(cfg)">编辑</el-button>
          <el-button
            v-if="!cfg.isActive"
            size="small"
            type="primary"
            @click="setActive(cfg.id)"
          >
            设为默认
          </el-button>
          <el-button
            size="small"
            type="danger"
            :disabled="cfg.isActive && configs.length <= 1"
            @click="delConfig(cfg.id)"
          >
            删除
          </el-button>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <el-empty v-else description="还没有模型配置" :image-size="80">
      <el-button type="primary" @click="showDialog(null)">新增配置</el-button>
    </el-empty>

    
    <!-- Scenario Route Assignment -->
    <div class="scenario-section" v-if="configs.length > 0">
      <h3 class="section-title">用途分配</h3>
      <p class="section-desc">为不同场景指定使用的模型。未分配时回退到默认模型。</p>

      <div class="scenario-grid">
        <div
          v-for="route in scenarioRoutes"
          :key="route.scenario"
          class="scenario-card"
        >
          <div class="sc-card-header">
            <span class="sc-label">{{ scenarioLabel(route.scenario) }}</span>
            <el-tag v-if="!route.modelConfigId" size="small" type="info">使用默认</el-tag>
            <el-tag v-else size="small" type="success">已分配</el-tag>
          </div>
          <div class="sc-card-body">
            <span class="sc-desc">{{ scenarioDesc(route.scenario) }}</span>
          </div>
          <div class="sc-card-select">
            <el-select
              v-model="routeAssignments[route.scenario]"
              :placeholder="'使用默认模型'"
              clearable
              size="small"
              style="width:100%"
              @change="(val: number|null) => assignRoute(route.scenario, val)"
            >
              <el-option
                v-for="cfg in configs"
                :key="cfg.id"
                :label="cfg.name + ' (' + cfg.modelName + ')'"
                :value="cfg.id"
              />
            </el-select>
          </div>
        </div>
      </div>
    </div>


    <!-- Add/Edit dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑模型配置' : '新增模型配置'"
      width="520px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent
      >
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如: 我的 GPT、本地 Ollama" />
        </el-form-item>

        <el-form-item label="类型" prop="apiType">
          <el-select v-model="form.apiType" style="width:100%" @change="onProviderChange">
            <el-option
              v-for="p in providers"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            >
              <span>{{ p.name }}</span>
              <span style="float:right;font-size:11px;color:var(--el-text-color-secondary)">{{ p.id }}</span>
            </el-option>
          </el-select>
          <div class="form-hint" v-if="currentProviderSchema">
            {{ currentProviderSchema.description || '' }}
          </div>
          <!-- Capability tags -->
          <div v-if="currentProviderSchema" style="margin-top:6px;display:flex;gap:4px;flex-wrap:wrap">
            <el-tag
              v-for="(val, key) in currentProviderSchema.capabilities"
              :key="key"
              size="small"
              :type="val ? 'success' : 'info'"
              :disable-transitions="true"
            >
              {{ capLabel(key) }}{{ val ? '' : ' x' }}
            </el-tag>
          </div>
        </el-form-item>

        <el-form-item label="Base URL" prop="baseUrl">
          <el-input v-model="form.baseUrl" placeholder="http://127.0.0.1:11434" />
        </el-form-item>

        <el-form-item label="API Key" prop="apiKey">
          <el-input
            v-model="form.apiKey"
            type="password"
            show-password
            :placeholder="editingId ? '留空则保留现有 Key' : 'sk-...'"
          />
          <div class="form-hint">
            Ollama 本地模式可以不填。编辑时留空则保留现有 Key。
          </div>
        </el-form-item>

        <el-form-item label="模型名称" prop="modelName">
          <div class="model-detect-wrap">
        <div class="model-detect-row">
          <el-input v-model="form.modelName" placeholder="gpt-4o-mini / qwen2.5:7b / deepseek-chat" class="model-input" />
          <el-button type="success" size="small" :loading="detectingModels" @click="detectModels" :disabled="!form.baseUrl">
            {{ detectingModels ? '检测中' : '检测可用模型' }}
          </el-button>
        </div>
        <div v-if="detectError" class="detect-error">{{ detectError }}</div>
        <div v-if="detectedModels.length > 0" class="detect-dropdown">
          <div class="detect-hint">已检测到 {{ detectedModels.length }} 个模型，点击选择：</div>
          <div v-for="m in detectedModels" :key="m.id" class="detect-option" :class="{ active: form.modelName === m.id }" @click="form.modelName = m.id; detectError = ''">
            {{ m.id }}
          </div>
        </div>
        </div>
        </el-form-item>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="温度">
              <el-input-number
                v-model="form.temperature"
                :min="0"
                :max="2"
                :step="0.1"
                :precision="1"
                controls-position="right"
                style="width:100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="最大 Token">
              <el-input-number
                v-model="form.maxTokens"
                :min="256"
                :max="131072"
                :step="1024"
                controls-position="right"
                style="width:100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="超时(秒)">
              <el-input-number
                v-model="form.timeoutSeconds"
                :min="5"
                :max="300"
                controls-position="right"
                style="width:100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="重试次数">
              <el-input-number
                v-model="form.retryCount"
                :min="0"
                :max="5"
                controls-position="right"
                style="width:100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveConfig">保存</el-button>
      </template>
    </el-dialog>

    <!-- Test result dialog -->
    <el-dialog v-model="testResultVisible" title="测试结果" width="440px">
      <div v-if="testResult" class="test-result">
        <div class="tr-status" :class="testResult.success ? 'success' : 'fail'">
          <el-icon :size="20">
            <CircleCheckFilled v-if="testResult.success" />
            <CircleCloseFilled v-else />
          </el-icon>
          <span>{{ testResult.success ? "连接成功" : "连接失败" }}</span>
        </div>
        <div class="tr-meta">
          <div class="tr-row">
            <span class="trl">延迟</span>
            <span class="trv">{{ testResult.latencyMs }}ms</span>
          </div>
          <div class="tr-row" v-if="testResult.message">
            <span class="trl">信息</span>
            <span class="trv">{{ testResult.message }}</span>
          </div>
          <div class="tr-row" v-if="testResult.reply">
            <span class="trl">模型回复</span>
            <span class="trv reply">{{ testResult.reply }}</span>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, inject } from "vue"
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus"
import { Plus, CircleCheckFilled, CircleCloseFilled } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const { get, post, put, del } = useApi()
const refreshHealth = inject<() => void>("refreshHealth", () => {})

// State
const configs = ref<any[]>([])
// Provider metadata
const providers = ref<any[]>([])
const currentProviderSchema = ref<any>(null)

const dialogVisible = ref(false)
const detectingModels = ref(false)
const detectedModels = ref<{id: string; owned_by?: string}[]>([])
const detectError = ref("")
const editingId = ref<number | null>(null)
const saving = ref(false)
const showApiKey = ref(false)
const showKeyId = ref<number | null>(null)
const originalApiKey = ref("")
const testingId = ref<number | null>(null)
const testResultVisible = ref(false)
const testResult = ref<any>(null)
const formRef = ref<FormInstance>()
const scenarioRoutes = ref<any[]>([])
const routeAssignments = ref<Record<string, number | null>>({})

const form = reactive({
  name: "",
  apiType: "openai-compatible" as string,
  baseUrl: "",
  apiKey: "",
  modelName: "",
  temperature: 0.7,
  maxTokens: 4096,
  timeoutSeconds: 60,
  retryCount: 1,
})

const rules: FormRules = {
  name: [{ required: true, message: "请输入名称", trigger: "blur" }],
  apiType: [{ required: true, message: "请选择类型", trigger: "change" }],
  baseUrl: [{ required: true, message: "请输入 Base URL", trigger: "blur" }],
  modelName: [{ required: true, message: "请输入模型名称", trigger: "blur" }],
}

// ---- Fetch ----
async function fetchConfigs() {
  configs.value = (await get<any[]>('/api/model/configs') || []).map(c => ({ ...c, isActive: !!c.isActive }))
}

async function loadProviders() {
  try {
    providers.value = await get<any[]>("/api/model/providers") || []
  } catch {
    providers.value = [
      { id: "openai-compatible", name: "OpenAI Compatible" },
      { id: "ollama", name: "Ollama" },
      { id: "custom-http", name: "Custom HTTP" },
    ]
  }
  onProviderChange(form.apiType)
}

function onProviderChange(apiType = form.apiType) {
  currentProviderSchema.value = providers.value.find((p: any) => p.id === apiType) || null
  detectedModels.value = []
  detectError.value = ""
}

// ---- Detect models ----
async function detectModels() {
  detectError.value = ""
  detectedModels.value = []
  detectingModels.value = true
  try {
    const res = await post<any>("/api/model/detect-models", {
      baseUrl: form.baseUrl,
      apiKey: form.apiKey,
      apiType: form.apiType,
    })
    detectedModels.value = res?.models || []
    if (detectedModels.value.length === 0) {
      detectError.value = "未检测到可用模型"
    }
  } catch (err: any) {
    detectError.value = err?.message || "检测失败，请检查 Base URL 和 API Key"
  } finally {
    detectingModels.value = false
  }
}

// ---- Mask API Key ----
function providerName(apiType: string): string {
  const p = providers.value.find((pr: any) => pr.id === apiType)
  return p?.name || apiType
}

function capLabel(key: string | number): string {
  const labels: Record<string, string> = {
    chat: "聊天",
    stream: "流式",
    vision: "视觉",
    tools: "工具",
    embeddings: "嵌入",
    local: "本地",
    remote: "远程",
  }
  const name = String(key)
  return labels[name] || name.replace(/_/g, " ")
}

function maskKey(key: string): string {
  if (!key) return "未设置"
  if (key.length <= 8) return "****"
  return key.slice(0, 4) + "****" + key.slice(-4)
}

function toggleKey(id: number) {
  showKeyId.value = showKeyId.value === id ? null : id
}

// ---- Dialog ----
async function showDialog(row: any) {
  editingId.value = row?.id || null
  showApiKey.value = false
  if (row) {
    form.name = row.name || ""
    form.apiType = row.apiType || "openai-compatible"
    form.baseUrl = row.baseUrl || ""
    // Fetch full model to get the real API key
    try {
      const full = await get<any>(`/api/model/configs/${row.id}`)
      form.apiKey = full?.apiKey || ""
      originalApiKey.value = full?.apiKey || ""
    } catch {
      form.apiKey = ""
      originalApiKey.value = ""
    }
    form.modelName = row.modelName || ""
    form.temperature = row.temperature ?? 0.7
    form.maxTokens = row.maxTokens ?? 4096
    form.timeoutSeconds = row.timeoutSeconds ?? 60
    form.retryCount = row.retryCount ?? 1
  } else {
    form.name = ""
    form.apiType = "openai-compatible"
    form.baseUrl = ""
    form.apiKey = ""
    form.modelName = ""
    form.temperature = 0.7
    form.maxTokens = 4096
    form.timeoutSeconds = 60
    form.retryCount = 1
  }
  onProviderChange(form.apiType)
  dialogVisible.value = true
  // Reset validation
  setTimeout(() => formRef.value?.clearValidate(), 0)
}

async function saveConfig() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    if (editingId.value) {
      const payload: any = { ...form }
      if (!payload.apiKey || payload.apiKey === originalApiKey.value) {
        delete payload.apiKey
      }
      await put(`/api/model/configs/${editingId.value}`, payload)
    } else {
      await post("/api/model/configs", { ...form })
    }
    dialogVisible.value = false
    ElMessage.success(editingId.value ? "保存成功" : "新建成功")
    await fetchConfigs()
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false
  }
}

// ---- Test ----
async function testConfig(id: number) {
  testingId.value = id
  try {
    const result = await post<any>(`/api/model/configs/${id}/test`, { configId: id })
    testResult.value = {
      success: result?.test?.success ?? (result?.status === "ok") ?? result?.success ?? false,
      latencyMs: result?.test?.latencyMs ?? result?.latencyMs ?? result?.latency ?? 0,
      message: result?.test?.message ?? result?.message ?? "",
      reply: result?.test?.reply ?? result?.reply ?? "",
    }
    testResultVisible.value = true
    await fetchConfigs()
  } catch {
    testResult.value = { success: false, latencyMs: 0, message: "请求失败", reply: "" }
    testResultVisible.value = true
  } finally {
    testingId.value = null
  }
}

// ---- Set active ----
async function setActive(id: number) {
  try {
    await post(`/api/model/configs/${id}/active`)
    ElMessage.success("已设为默认模型")
    await fetchConfigs()
    refreshHealth()
  } catch {}
}

// ---- Delete ----
async function delConfig(id: number) {
  const cfg = configs.value.find(c => c.id === id)
  if (cfg?.isActive && configs.value.length <= 1) {
    ElMessage.warning("不能删除唯一的激活配置")
    return
  }

  await ElMessageBox.confirm(
    "确定删除此配置？如果是当前激活配置，将自动切换到其他配置。",
    "确认删除",
    { type: "warning", confirmButtonText: "删除" }
  )

  try {
    await del(`/api/model/configs/${id}`)
    ElMessage.success("已删除")
    await fetchConfigs()
  } catch {}
}

// ---- Helpers ----
function fmtDate(dateStr: string): string {
  if (!dateStr) return ""
  try { return new Date(dateStr).toLocaleString("zh-CN") } catch { return dateStr }
}



// ---- Scenario routing ----
async function fetchRoutes() {
  try {
    const data = await get<any[]>("/api/model/routes")
    scenarioRoutes.value = Array.isArray(data) ? data : (data as any)?.data || []
    // Sync assignments
    for (const r of scenarioRoutes.value) {
      routeAssignments.value[r.scenario] = r.modelConfigId
    }
  } catch { /* ignore */ }
}

async function assignRoute(scenario: string, modelConfigId: number | null) {
  try {
    await put("/api/model/routes", {
      routes: { [scenario]: modelConfigId }
    })
    ElMessage.success("用途分配已更新")
    await fetchRoutes()
  } catch {
    // Revert on error
    await fetchRoutes()
  }
}

function scenarioLabel(scenario: string): string {
  const labels: Record<string, string> = {
    chat: "聊天对话",
    summary: "会话摘要",
    memory_extract: "记忆提取",
    safety_rewrite: "安全改写",
    import_parse: "导入解析",
    reply_timing_check:"完整性判断"
  }
  return labels[scenario] || scenario
}

function scenarioDesc(scenario: string): string {
  const descs: Record<string, string> = {
    chat: "日常聊天和对话回复",
    summary: "生成对话历史摘要",
    memory_extract: "从对话中提取用户记忆",
    safety_rewrite: "安全边界内容改写",
    import_parse: "解析导入的聊天记录文本",
    reply_timing_check:"判断回复用户是否发送完成完整信息"
  }
  return descs[scenario] || ""
}

onMounted(async () => {
  await loadProviders(); fetchConfigs(); fetchRoutes() })
</script>

<style scoped>
.page {
  }

.page-title {
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--ac-color-text);
}

.toolbar {
  margin-bottom: 14px;
}

/* ---- Cards ---- */
.config-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.config-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 16px;
  transition: border-color var(--ac-transition-fast);
}

.config-card.is-active {
  border-color: var(--ac-color-primary);
  border-left: 3px solid var(--ac-color-primary);
}

.card-top {
  margin-bottom: 10px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.card-name {
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  color: var(--ac-color-text);
}

.card-type {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-model {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  font-family: monospace;
}

/* ---- Details ---- */
.card-details {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 16px;
  margin-bottom: 10px;
}

.detail-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--ac-font-size-sm);
}

.dl {
  color: var(--ac-color-text-muted);
  white-space: nowrap;
  min-width: 60px;
}

.dv {
  color: var(--ac-color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: flex;
  align-items: center;
}

/* ---- Test result ---- */
.card-test {
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
  padding: 10px 12px;
  margin-bottom: 10px;
  font-size: var(--ac-font-size-sm);
}

.test-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

.test-indicator.success { color: var(--ac-color-success); }
.test-indicator.failed { color: var(--ac-color-danger); }

.test-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.test-indicator.success .test-dot { background: var(--ac-color-success); }
.test-indicator.failed .test-dot { background: var(--ac-color-danger); }

.test-msg {
  margin-top: 4px;
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
}

.test-time {
  margin-top: 2px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}

/* ---- Actions ---- */
.card-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

/* ---- Form hints ---- */
.form-hint {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-top: 4px;
  line-height: 1.4;
}

/* ---- Test result dialog ---- */
.test-result {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tr-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
}

.tr-status.success { color: var(--ac-color-success); }
.tr-status.fail { color: var(--ac-color-danger); }

.tr-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tr-row {
  display: flex;
  gap: 8px;
  font-size: var(--ac-font-size-sm);
}

.trl {
  color: var(--ac-color-text-muted);
  min-width: 60px;
}

.trv {
  color: var(--ac-color-text);
}

.trv.reply {
  font-style: italic;
  color: var(--ac-color-text-secondary);
  background: var(--ac-color-bg-secondary);
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  flex: 1;
}

@media (max-width: 640px) {
  .card-details {
    grid-template-columns: 1fr;
  }
}

/* ---- Scenario Section ---- */
.scenario-section {
  margin-top: 18px;
}

.section-title {
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  margin-bottom: 4px;
  color: var(--ac-color-text);
}

.section-desc {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  margin-bottom: 10px;
}

.scenario-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 10px;
}

.scenario-card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  padding: 12px 14px;
}

.sc-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.sc-label {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text);
}

.sc-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  display: block;
  margin-bottom: 8px;
}

@media (max-width: 640px) {
  .scenario-grid {
    grid-template-columns: 1fr;
  }
}

/* Model detect dropdown */
.model-detect-wrap { position: relative; }
.model-detect-row { display: flex; gap: 8px; }
.model-detect-row .model-input { flex: 1; }
.detect-error { color: var(--ac-color-danger, #f56c6c); font-size: 12px; margin-top: 4px; }
.detect-dropdown { background: var(--ac-color-surface); border: 1px solid var(--ac-color-primary-border); border-radius: var(--ac-radius-sm); box-shadow: var(--ac-shadow-md); max-height: 180px; overflow-y: auto; margin-top: 6px; }
.detect-hint { padding: 6px 10px; font-size: 11px; color: var(--ac-color-text-muted); border-bottom: 1px solid var(--ac-color-border-light); }
.detect-option { padding: 8px 10px; cursor: pointer; font-size: 13px; color: var(--ac-color-text); transition: background .15s; }
.detect-option:hover { background: var(--ac-color-primary-bg); }
.detect-option.active { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); font-weight: 600; }
.dropdown-enter-active, .dropdown-leave-active { transition: all .2s ease; }
.dropdown-enter-from, .dropdown-leave-to { opacity: 0; transform: translateY(-4px); }


.detect-hint { padding: 6px 10px; font-size: 11px; color: var(--ac-color-text-muted); border-bottom: 1px solid var(--ac-color-border-light); }
.detect-option { padding: 8px 10px; cursor: pointer; font-size: 13px; color: var(--ac-color-text); transition: background .15s; }
.detect-option:hover { background: var(--ac-color-primary-bg); }
.detect-option.active { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); font-weight: 600; }

</style>