<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
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

    <!-- 主题设置 -->
    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>主题设置</span></template>
      <div class="theme-preset-list">
        <div
          v-for="p in presets"
          :key="p.id"
          class="theme-preset-item"
          :class="{ active: themeState.preset === p.id }"
          @click="setPreset(p.id)"
        >
          <div class="theme-preset-preview" :class="'preview-' + p.id"></div>
          <div class="theme-preset-info">
            <span class="theme-preset-name">{{ p.name }}</span>
            <span class="theme-preset-desc">{{ p.description }}</span>
          </div>
          <el-icon v-if="themeState.preset === p.id" color="var(--ac-color-primary)"><Check /></el-icon>
        </div>
      </div>
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
        <el-descriptions-item label="项目">{{ aboutInfo.name }} / {{ aboutInfo.displayName }}</el-descriptions-item>
        <el-descriptions-item label="许可证">{{ aboutInfo.license }}</el-descriptions-item>
        <el-descriptions-item label="版权">{{ aboutInfo.copyright }}</el-descriptions-item>
      </el-descriptions>
      <div class="legal-links">
        <el-link :href="legalLinks.sourceCode" target="_blank" type="primary">Source Code</el-link>
        <el-link :href="legalLinks.commercialLicensing" target="_blank" type="primary">Commercial Licensing</el-link>
        <el-link :href="legalLinks.thirdPartyNotices" target="_blank" type="primary">Third-Party Notices</el-link>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useTheme } from '../../composables/useTheme'
import axios from "axios"
import { ElMessage } from "element-plus"
import { Check } from '@element-plus/icons-vue'

const API = "http://127.0.0.1:8899"
const savingPrompt = ref(false)

const { state: themeState, setPreset, presets } = useTheme()

const DEFAULT_STYLE_PROMPT = "你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。\n回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。\n用户随口聊，你就自然接话；用户认真问问题，你再认真回答。\n不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。\n回复格式要像微信连续消息：\n用户发一句话时，你可以回复 1 到 4 句短句。\n不要写成一整段长文。\n整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。\n回复中不要使用任何emoji表情符号。\n不能使用markdown格式。"
const DEFAULT_SOURCE_CODE_URL = "https://gitee.com/Untrammelled/Amitia"
const DEFAULT_COMMERCIAL_LICENSE_URL = "mailto:3151508592@qq.com"

const styleForm = reactive({
  prompt: DEFAULT_STYLE_PROMPT,
})

const aboutInfo = reactive({
  name: "Amitia",
  displayName: "阿米提亚",
  license: "AGPL-3.0-only",
  copyright: "Copyright © 2026 彭旭",
})

const legalLinks = reactive({
  sourceCode: ((import.meta as any).env?.VITE_AMITIA_SOURCE_CODE_URL || DEFAULT_SOURCE_CODE_URL) as string,
  commercialLicensing: ((import.meta as any).env?.VITE_AMITIA_COMMERCIAL_LICENSE_URL || DEFAULT_COMMERCIAL_LICENSE_URL) as string,
  thirdPartyNotices: ((import.meta as any).env?.VITE_AMITIA_THIRD_PARTY_NOTICES_URL || `${DEFAULT_SOURCE_CODE_URL}/blob/master/THIRD_PARTY_NOTICES.md`) as string,
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
  loadAbout()
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

async function loadAbout() {
  try {
    const { data } = await axios.get(API + "/api/about")
    const about = data?.data
    if (!about) return
    aboutInfo.name = about.name || aboutInfo.name
    aboutInfo.displayName = about.displayName || aboutInfo.displayName
    aboutInfo.license = about.license || aboutInfo.license
    aboutInfo.copyright = about.copyright?.replace("(C)", "©") || aboutInfo.copyright
    legalLinks.sourceCode = (import.meta as any).env?.VITE_AMITIA_SOURCE_CODE_URL || about.sourceCodeUrl || legalLinks.sourceCode
    legalLinks.commercialLicensing = (import.meta as any).env?.VITE_AMITIA_COMMERCIAL_LICENSE_URL || about.commercialLicensingUrl || legalLinks.commercialLicensing
    legalLinks.thirdPartyNotices = (import.meta as any).env?.VITE_AMITIA_THIRD_PARTY_NOTICES_URL || about.thirdPartyNoticesUrl || legalLinks.thirdPartyNotices
  } catch {}
}
</script>

<style scoped>
.settings-view { padding: 20px; max-width: 720px; }
.page-header { margin-bottom: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.settings-card { margin-bottom: 16px; }
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
.legal-links { display: flex; gap: 14px; flex-wrap: wrap; margin-top: 12px; }

/* 主题预设卡片 */
.theme-preset-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.theme-preset-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border: 2px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  cursor: pointer;
  transition: all 0.2s;
  background: var(--ac-color-surface);
}

.theme-preset-item:hover {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-surface-hover);
}

.theme-preset-item.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}

.theme-preset-preview {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  flex-shrink: 0;
  border: 1px solid var(--ac-color-border);
}

.preview-light {
  background: linear-gradient(135deg, #F8FAF4 0%, #7DAA84 100%);
}
.preview-dark {
  background: #1A1B1E;
}

.preview-system {
  background: linear-gradient(135deg, #F8FAF4 50%, #1A1B1E 50%);
}

.preview-calm-blue {
  background: linear-gradient(135deg, #3B82F6 0%, #DBEAFE 100%);
}

.preview-warm-gray {
  background: linear-gradient(135deg, #8A8178 0%, #EDE8E2 100%);
}

.preview-mint {
  background: linear-gradient(135deg, #089B8A 0%, #F6FDF9 100%);
}

.preview-navy {
  background: linear-gradient(135deg, #0F172A 0%, #60A5FA 100%);
}
.theme-preset-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.theme-preset-name {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  color: var(--ac-color-text);
}

.theme-preset-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
</style>
