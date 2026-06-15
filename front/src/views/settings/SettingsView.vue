<template>
  <div class="settings-view">
    <div class="page-header"><h2>系统设置</h2></div>
    
    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>AI回复风格提示词</span></template>
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
      >
        <template #title>
          此提示词影响 AI 的回复风格。默认配置经过优化，<strong>如非必要请勿修改</strong>，修改不当可能导致回复质量下降。
        </template>
      </el-alert>
      <el-form :model="styleForm" label-width="0">
        <el-form-item>
          <el-input
            v-model="styleForm.prompt"
            type="textarea"
            :rows="12"
            placeholder="AI回复风格提示词..."
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveStylePrompt" :loading="savingPrompt">保存</el-button>
          <el-button @click="resetStylePrompt">恢复默认</el-button>
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
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import axios from "axios"
import { ElMessage } from "element-plus"

const API = "http://127.0.0.1:8899"
const savingPrompt = ref(false)

const DEFAULT_STYLE_PROMPT = "你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。\n回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。\n用户随口聊，你就自然接话；用户认真问问题，你再认真回答。\n不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。\n回复格式要像微信连续消息：\n用户发一句话时，你可以回复 1 到 4 句短句。\n不要写成一整段长文。\n整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。\n回复中不要使用任何emoji表情符号。\n不能使用markdown格式。"

const styleForm = reactive({
  prompt: DEFAULT_STYLE_PROMPT,
})

async function loadStylePrompt() {
  try {
    const { data } = await axios.get(API + "/api/config")
    if (data?.data?.settings?.wechat_style_prompt) {
      styleForm.prompt = data.data.settings.wechat_style_prompt
    }
  } catch {}
}

const timingOverview = ref<any>({ enabled: false, bufferCounts: {} })

onMounted(async () => {
  loadTimingOverview()
  loadStylePrompt()
})

async function saveStylePrompt() {
  savingPrompt.value = true
  try {
    await axios.put(API + "/api/config", { settings: { wechat_style_prompt: styleForm.prompt } })
    ElMessage.success("AI回复风格提示词已保存")
  } catch (err: any) {
    ElMessage.error("保存失败: " + err.message)
  } finally {
    savingPrompt.value = false
  }
}

function resetStylePrompt() {
  styleForm.prompt = DEFAULT_STYLE_PROMPT
}

async function loadTimingOverview() {
  try {
    const { data } = await axios.get(API + "/api/reply-timing/overview")
    if (data?.data) timingOverview.value = data.data
  } catch {}
}
</script>

<style scoped>
.settings-view { padding: 20px; max-width: 720px; }
.page-header { margin-bottom: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.settings-card { margin-bottom: 16px; }
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
</style>
