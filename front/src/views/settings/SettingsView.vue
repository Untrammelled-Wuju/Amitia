<template>
  <div class="settings-view">
    <div class="page-header"><h2>系统设置</h2></div>

    <el-card class="settings-card">
      <template #header><span>大模型 API 配置</span></template>
      <el-form :model="llmForm" label-width="120px">
        <el-form-item label="API 地址">
          <el-input v-model="llmForm.apiUrl" placeholder="例如: https://api.openai.com" />
          <div class="form-tip">兼容 OpenAI API 格式的地址</div>
        </el-form-item>
        <el-form-item label="API Key">
          <el-input v-model="llmForm.apiKey" type="password" show-password placeholder="sk-..." />
        </el-form-item>
        <el-form-item label="模型名称">
          <el-input v-model="llmForm.model" placeholder="gpt-4o-mini" />
        </el-form-item>
        <el-form-item label="Temperature">
          <el-slider v-model="llmForm.temperature" :min="0" :max="2" :step="0.1" show-input />
        </el-form-item>
        <el-form-item label="Max Tokens">
          <el-input-number v-model="llmForm.maxTokens" :min="100" :max="128000" :step="100" />
        </el-form-item>
        <el-form-item label="Top P">
          <el-slider v-model="llmForm.topP" :min="0" :max="1" :step="0.05" show-input />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveLLMConfig" :loading="saving">保存配置</el-button>
          <el-button @click="testConnection" :loading="testing">测试连接</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    
    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>微信回复风格提示词</span></template>
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
      >
        <template #title>
          此提示词影响 AI 在微信中的回复风格。默认配置经过优化，<strong>如非必要请勿修改</strong>，修改不当可能导致回复质量下降。
        </template>
      </el-alert>
      <el-form :model="wechatForm" label-width="0">
        <el-form-item>
          <el-input
            v-model="wechatForm.prompt"
            type="textarea"
            :rows="12"
            placeholder="微信风格提示词..."
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveWechatPrompt" :loading="savingPrompt">保存</el-button>
          <el-button @click="resetWechatPrompt">恢复默认</el-button>
        </el-form-item>
      </el-form>
    </el-card>


    <!-- 模型分配 -->
    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>模型分配（用途分配）</span></template>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        <template #title>
          为不同用途分配独立模型。未分配时自动使用默认聊天模型。
        </template>
      </el-alert>
      <el-form label-width="140px">
        <el-form-item
          v-for="item in routeList"
          :key="item.scenario"
          :label="scenarioLabel(item.scenario)"
        >
          <el-select
            v-model="routeForm[item.scenario]"
            placeholder="选择模型（留空则使用默认）"
            clearable
            style="width: 320px"
            @change="onRouteChange(item.scenario, $event)"
          >
            <el-option
              v-for="cfg in modelConfigs"
              :key="cfg.id"
              :label="cfg.name + ' (' + cfg.modelName + ')'"
              :value="cfg.id"
            />
          </el-select>
          <div class="form-tip" v-if="scenarioTip(item.scenario)">
            {{ scenarioTip(item.scenario) }}
          </div>
          <div class="form-tip" v-if="item.scenario === 'reply_timing_check' && !routeForm[item.scenario]">
            <el-icon style="vertical-align: middle; margin-right: 2px"><InfoFilled /></el-icon>
            未单独配置时，将使用默认聊天模型。
          </div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveRoutes" :loading="savingRoutes">保存分配</el-button>
        </el-form-item>
      </el-form>
    </el-card>


    <!-- 回复时机判断 -->
    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>回复时机判断</span></template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="功能状态">
          <el-tag :type="timingOverview.enabled ? 'success' : 'info'" size="small">
            {{ timingOverview.enabled ? '已启用' : '已禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="模型判断">
          <el-tag :type="timingOverview.useModelCheck ? 'success' : 'warning'" size="small">
            {{ timingOverview.useModelCheck ? '已启用' : '仅规则' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Web 等待">{{ timingOverview.webWaitMs }}ms</el-descriptions-item>
        <el-descriptions-item label="微信等待">{{ timingOverview.wechatWaitMs }}ms</el-descriptions-item>
        <el-descriptions-item label="最大等待">{{ timingOverview.maxWaitMs }}ms</el-descriptions-item>
        <el-descriptions-item label="缓冲区总数">{{ timingOverview.bufferCounts?.total || 0 }}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top: 8px; display: flex; gap: 8px; flex-wrap: wrap">
        <el-tag size="small" type="info">等待中: {{ timingOverview.bufferCounts?.waiting || 0 }}</el-tag>
        <el-tag size="small" type="warning">检查中: {{ timingOverview.bufferCounts?.checking || 0 }}</el-tag>
        <el-tag size="small" type="primary">回复中: {{ timingOverview.bufferCounts?.replying || 0 }}</el-tag>
        <el-tag size="small" type="danger">已暂停: {{ timingOverview.bufferCounts?.paused || 0 }}</el-tag>
        <el-tag size="small" type="danger">失败: {{ timingOverview.bufferCounts?.failed || 0 }}</el-tag>
      </div>
      <div v-if="timingOverview.recentFailures?.length" style="margin-top: 12px">
        <div class="form-tip" style="font-weight: 600; margin-bottom: 4px">最近失败记录：</div>
        <div v-for="(f, i) in timingOverview.recentFailures.slice(0, 5)" :key="i" class="form-tip">
          {{ f.created_at?.slice(0, 19) }} {{ f.details?.slice(0, 80) }}
        </div>
      </div>
    </el-card>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>服务器信息</span></template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="Core 地址">http://127.0.0.1:8899</el-descriptions-item>
        <el-descriptions-item label="模式">本地</el-descriptions-item>
        <el-descriptions-item label="数据库">data/ai-companion.db</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>

  <!-- 开发者选项 -->
  <el-card class="settings-card" shadow="hover">
    <template #header><span>开发者选项</span></template>
    <div style="display:flex;gap:12px;flex-wrap:wrap">
      <el-button @click="openDebugPanel" :icon="Monitor">AI 生活状态调试面板</el-button>
      <el-button @click="$router.push('/character/life-rules')">AI 生活规则配置</el-button>
    </div>
  </el-card>

  <!-- 调试面板弹窗 -->
  <el-dialog v-model="debugVisible" title="AI 生活状态调试" width="95%" top="2vh" destroy-on-close>
    <CompanionDebugView v-if="debugVisible" />
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import axios from "axios"
import type { LLMConfig, ApiResponse } from "@/types"
import { ElMessage } from "element-plus"
import { InfoFilled } from "@element-plus/icons-vue"
import { Monitor } from "@element-plus/icons-vue"
import CompanionDebugView from "@/views/companion-debug/CompanionDebugView.vue"
const debugVisible = ref(false)
function openDebugPanel() { debugVisible.value = true }

const API = "http://127.0.0.1:8899"
const saving = ref(false)
const testing = ref(false)
const savingPrompt = ref(false)
const savingRoutes = ref(false)

const DEFAULT_WECHAT_PROMPT = `【微信聊天风格 — 必须严格遵守】
你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。
回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。
用户随口聊，你就自然接话；用户认真问问题，你再认真回答。
不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。
回复格式要像微信连续消息：
用户发一句话时，你可以回复 1 到 4 句短句。
每句话之间用反斜杠 \\\\ 分割（注意：是反斜杠，不是换行）。
不要写成一整段长文。
整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。`

const SCENARIO_LABELS: Record<string, string> = {
  chat: "普通聊天模型",
  summary: "摘要模型",
  memory_extract: "记忆提取模型",
  safety_rewrite: "安全改写模型",
  import_parse: "导入解析模型",
  reply_timing_check: "完整性判断模型",
}
const SCENARIO_TIPS: Record<string, string> = {
  reply_timing_check: "用于判断用户是否说完，建议选择速度快、成本低的小模型。",
}

const wechatForm = reactive({
  prompt: DEFAULT_WECHAT_PROMPT,
})
const modelConfigs = ref<any[]>([])
const routeList = ref<any[]>([])
const routeForm = reactive<Record<string, number | null>>({})

const llmForm = reactive<LLMConfig>({
  apiUrl: "", apiKey: "", model: "gpt-4o-mini",
  temperature: 0.7, maxTokens: 4096, topP: 1.0,
})

async function loadWechatPrompt() {
  try {
    const { data } = await axios.get(API + "/api/config")
    if (data?.data?.settings?.wechat_style_prompt) {
      wechatForm.prompt = data.data.settings.wechat_style_prompt
    }
  } catch {}
}

const timingOverview = ref<any>({ enabled: false, bufferCounts: {} })

onMounted(async () => {
  loadTimingOverview()
  loadWechatPrompt()
  loadRoutes()
  loadModelConfigs()
  const { data } = await axios.get<ApiResponse<LLMConfig>>(API + "/api/config/llm")
  if (data.success && data.data) Object.assign(llmForm, data.data)
})

async function saveLLMConfig() {
  saving.value = true
  try {
    await axios.put(API + "/api/config/llm", llmForm)
    ElMessage.success("LLM 配置已保存")
  } catch (err: any) {
    ElMessage.error("保存失败: " + err.message)
  } finally {
    saving.value = false
  }
}

async function saveWechatPrompt() {
  savingPrompt.value = true
  try {
    await axios.put(API + "/api/config", { settings: { wechat_style_prompt: wechatForm.prompt } })
    ElMessage.success("微信风格提示词已保存")
  } catch (err: any) {
    ElMessage.error("保存失败: " + err.message)
  } finally {
    savingPrompt.value = false
  }
}

function resetWechatPrompt() {
  wechatForm.prompt = DEFAULT_WECHAT_PROMPT
}

async function loadTimingOverview() {
  try {
    const { data } = await axios.get(API + "/api/reply-timing/overview")
    if (data?.data) timingOverview.value = data.data
  } catch {}
}

async function testConnection() {
  testing.value = true
  try {
    const { data } = await axios.post(API + "/api/chat", { characterId: "test", message: "ping" })
    if (data.success) {
      ElMessage.success("连接测试成功")
    } else {
      ElMessage.warning(data.error || "连接测试返回异常")
    }
  } catch (err: any) {
    ElMessage.error("连接测试失败: " + err.message)
  } finally {
    testing.value = false
  }
}

async function loadRoutes() {
  try {
    const { data } = await axios.get(API + "/api/model/routes")
    if (data?.data) {
      routeList.value = data.data
      for (const r of data.data) {
        routeForm[r.scenario] = r.modelConfigId
      }
    }
  } catch {}
}

async function loadModelConfigs() {
  try {
    const { data } = await axios.get(API + "/api/model/configs")
    if (data?.data) {
      modelConfigs.value = data.data
    }
  } catch {}
}

async function saveRoutes() {
  savingRoutes.value = true
  try {
    const routes: Record<string, number | null> = {}
    for (const key of Object.keys(routeForm)) {
      routes[key] = routeForm[key] ?? null
    }
    await axios.put(API + "/api/model/routes", { routes })
    ElMessage.success("模型分配已保存")
    await loadRoutes()
  } catch (err: any) {
    ElMessage.error("保存失败: " + (err?.response?.data?.message || err.message))
  } finally {
    savingRoutes.value = false
  }
}

function scenarioLabel(scenario: string): string {
  return SCENARIO_LABELS[scenario] || scenario
}

function scenarioTip(scenario: string): string {
  return SCENARIO_TIPS[scenario] || ""
}

function onRouteChange(scenario: string, value: number | null) {
  // no-op: just track change via v-model
}
</script>

<style scoped>
.settings-view { padding: 20px; max-width: 720px; }
.page-header { margin-bottom: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.settings-card { margin-bottom: 16px; }
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
</style>