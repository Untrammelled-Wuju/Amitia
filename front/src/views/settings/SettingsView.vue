<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="settings-view">
    <div class="page-header"><h2>系统设置</h2></div>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>AI回复风格提示词</span></template>
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px">
        <template #title>此提示词影响 AI 的回复风格。默认配置经过优化，<strong>如非必要请勿修改</strong>，修改不当可能导致回复质量下降。</template>
      </el-alert>
      <el-form :model="styleForm" label-width="0">
        <el-form-item>
          <el-input v-model="styleForm.prompt" type="textarea" :rows="12" placeholder="AI回复风格提示词..." />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveStylePrompt" :loading="savingPrompt">保存</el-button>
          <el-button @click="resetStylePrompt">恢复默认</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>主题设置</span></template>
      <div class="theme-preset-list">
        <div v-for="p in presets" :key="p.id" class="theme-preset-item" :class="{ active: themeState.preset === p.id }" @click="setPreset(p.id)">
          <div class="theme-preset-preview" :class="'preview-' + p.id"></div>
          <div class="theme-preset-info">
            <span class="theme-preset-name">{{ p.name }}</span>
            <span class="theme-preset-desc">{{ p.description }}</span>
          </div>
          <el-icon v-if="themeState.preset === p.id" color="var(--ac-color-primary)"><Check /></el-icon>
        </div>
      </div>
    </el-card>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>回复时机判断</span></template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="功能状态"><el-tag :type="timingOverview.enabled ? 'success' : 'info'" size="small">{{ timingOverview.enabled ? '已启用' : '已禁用' }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="模型判断"><el-tag :type="timingOverview.useModelCheck ? 'success' : 'warning'" size="small">{{ timingOverview.useModelCheck ? '已启用' : '仅规则' }}</el-tag></el-descriptions-item>
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
        <div v-for="(f, i) in timingOverview.recentFailures.slice(0, 5)" :key="i" class="form-tip">{{ f.created_at?.slice(0, 19) }} {{ f.details?.slice(0, 80) }}</div>
      </div>
    </el-card>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>心理状态快照</span></template>
      <div v-if="psycheLoading" style="text-align:center;padding:16px"><el-icon class="is-loading" size="20"><Loading /></el-icon></div>
      <div v-else-if="psycheError" class="form-tip" style="color:var(--el-color-danger)">{{ psycheError }}</div>
      <div v-else>
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item label="情绪正">{{ psycheState.emotion.positive.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="情绪负">{{ psycheState.emotion.negative.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="唤醒度">{{ psycheState.emotion.arousal.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="支配度">{{ psycheState.emotion.dominance.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="心境效价">{{ psycheState.mood.valence.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="心境张力">{{ psycheState.mood.tension.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="压力">{{ psycheState.stress.toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="PAD标签">{{ psycheState.mood.pad || '—' }}</el-descriptions-item>
          <el-descriptions-item label="情绪标签">{{ psycheState.affectLabel || '—' }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="psycheState.needs && Object.keys(psycheState.needs).length" style="margin-top:8px">
          <div class="form-tip" style="font-weight:600;margin-bottom:4px">需求水平：</div>
          <div style="display:flex;gap:6px;flex-wrap:wrap">
            <el-tag v-for="(v,k) in psycheState.needs" :key="k" size="small">{{ k }}: {{ v.toFixed(2) }}</el-tag>
          </div>
        </div>
        <div v-if="psycheState.relationship" style="margin-top:8px">
          <div class="form-tip" style="font-weight:600;margin-bottom:4px">关系状态：</div>
          <div style="display:flex;gap:6px;flex-wrap:wrap">
            <el-tag size="small" type="primary">信任 {{ psycheState.relationship.trust.toFixed(2) }}</el-tag>
            <el-tag size="small" type="success">熟悉 {{ psycheState.relationship.familiarity.toFixed(2) }}</el-tag>
            <el-tag size="small" type="warning">张力 {{ psycheState.relationship.tension.toFixed(2) }}</el-tag>
            <el-tag size="small" type="info">安全感 {{ psycheState.relationship.security.toFixed(2) }}</el-tag>
          </div>
        </div>
        <div v-if="psycheState.beliefs?.length" style="margin-top:8px">
          <div class="form-tip" style="font-weight:600;margin-bottom:4px">活跃信念：</div>
          <div style="display:flex;gap:4px;flex-wrap:wrap">
            <el-tag v-for="b in psycheState.beliefs.slice(0,5)" :key="b.key" :type="b.conflicted?'danger':'success'" size="small">{{ b.key }}={{ b.value }}[{{ (b.confidence*100).toFixed(0) }}%]</el-tag>
          </div>
          <div v-if="psycheState.beliefs.length > 5" class="form-tip">+{{ psycheState.beliefs.length-5 }} 更多</div>
        </div>
        <div class="form-tip" style="margin-top:8px">采样时间: {{ psycheState.collectedAt || '—' }}</div>
      </div>
    </el-card>
    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>隐私与安全</span></template>
      <div class="privacy-grid">
        <div class="privacy-card" @click="goPrivacy">
          <el-icon size="22" color="var(--ac-color-primary)"><Lock /></el-icon>
          <div class="pc-body">
            <div class="pc-title">隐私说明</div>
            <div class="pc-desc">查看数据存储位置、模型 API 数据处理方式以及你的控制权说明</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
        <div class="privacy-card" @click="goBoundary">
          <el-icon size="22" color="var(--ac-color-primary)"><WarningFilled /></el-icon>
          <div class="pc-body">
            <div class="pc-title">使用边界</div>
            <div class="pc-desc">了解 AI 角色的能力限制、安全边界和理性使用建议</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
        <div class="privacy-card" @click="goPrivacyScan">
          <el-icon size="22" color="var(--ac-color-primary)"><Search /></el-icon>
          <div class="pc-body">
            <div class="pc-title">隐私扫描</div>
            <div class="pc-desc">扫描消息记录中的敏感信息，识别潜在隐私泄露风险</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
      </div>
    </el-card>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>数据管理</span></template>
      <div v-if="storageLoading" style="text-align:center;padding:16px"><el-icon class="is-loading" size="20"><Loading /></el-icon></div>
      <template v-else>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="数据库大小">{{ storageInfo.dbSize || '—' }}</el-descriptions-item>
          <el-descriptions-item label="消息总数">{{ storageInfo.messageCount ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="对话数">{{ storageInfo.conversationCount ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="记忆数">{{ storageInfo.memoryCount ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="备份数量">{{ backupList.length }}</el-descriptions-item>
        </el-descriptions>
        <div style="margin-top:12px;display:flex;gap:8px;flex-wrap:wrap">
          <el-button size="small" @click="createBackup" :loading="backupCreating"><el-icon><FolderAdd /></el-icon> 创建备份</el-button>
          <el-button size="small" @click="goStorage"><el-icon><Delete /></el-icon> 存储管理</el-button>
          <el-button size="small" type="danger" plain @click="exportUserData" :loading="exportingData"><el-icon><Download /></el-icon> 导出用户数据</el-button>
        </div>
      </template>
    </el-card>

    <el-card class="settings-card" style="margin-top: 16px">
      <template #header><span>服务器信息</span></template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="Core 地址">{{ apiBaseUrl || "当前页面源" }}</el-descriptions-item>
        <el-descriptions-item label="模式">{{ runtimeModeLabel }}</el-descriptions-item>
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
import { useRouter } from "vue-router"
import { useTheme } from '../../composables/useTheme'
import axios from "axios"
import { ElMessage } from "element-plus"
import {
  Check, Lock, WarningFilled, Search,
  ArrowRight, Loading, FolderAdd, Delete, Download
} from '@element-plus/icons-vue'
import { getApiBaseURL, getDeploymentConfig } from "../../runtime/runtime-adapter"

const router = useRouter()
const apiBaseUrl = ref("")
const runtimeModeLabel = ref("浏览器")
const savingPrompt = ref(false)

const { state: themeState, setPreset, presets } = useTheme()

const DEFAULT_STYLE_PROMPT = "你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。用户随口聊，你就自然接话；用户认真问问题，你再认真回答。不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。回复格式要像微信连续消息：用户发一句话时，你可以回复 1 到 4 句短句。不要写成一整段长文。整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。回复中不要使用任何emoji表情符号。不能使用markdown格式。"
const DEFAULT_SOURCE_CODE_URL = "https://gitee.com/Untrammelled/Amitia"
const DEFAULT_COMMERCIAL_LICENSE_URL = "mailto:3151508592@qq.com"

import type { PsycheStateSnapshot } from "../../types"
const psycheLoading = ref(false)
const psycheError = ref("")
const psycheState = ref<PsycheStateSnapshot>({
  emotion: { positive: 0, negative: 0, arousal: 0, dominance: 0 },
  mood: { valence: 0, tension: 0, pad: "" },
  stress: 0,
  needs: {},
  beliefs: [],
  relationship: { trust: 0.5, familiarity: 0.5, tension: 0, security: 0.5 },
  affectLabel: "",
  collectedAt: "",
})

async function loadPsycheState() {
  psycheLoading.value = true
  psycheError.value = ""
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/runtime/psyche-snapshot")
    if (data?.data) {
      psycheState.value = data.data
    }
  } catch (e: any) {
    psycheError.value = "心理状态不可用: " + (e?.message || "网络错误")
  } finally {
    psycheLoading.value = false
  }
}

onMounted(() => {
  loadPsycheState()
})
const styleForm = reactive({ prompt: DEFAULT_STYLE_PROMPT })

const aboutInfo = reactive({ name: "Amitia", displayName: "阿米提亚", license: "AGPL-3.0-only", copyright: "Copyright © 2026 彭旭" })

const legalLinks = reactive({
  sourceCode: ((import.meta as any).env?.VITE_AMITIA_SOURCE_CODE_URL || DEFAULT_SOURCE_CODE_URL) as string,
  commercialLicensing: ((import.meta as any).env?.VITE_AMITIA_COMMERCIAL_LICENSE_URL || DEFAULT_COMMERCIAL_LICENSE_URL) as string,
  thirdPartyNotices: ((import.meta as any).env?.VITE_AMITIA_THIRD_PARTY_NOTICES_URL || DEFAULT_SOURCE_CODE_URL + "/blob/master/THIRD_PARTY_NOTICES.md") as string,
})

const timingOverview = ref<any>({ enabled: false, bufferCounts: {} })
const storageLoading = ref(false)
const storageInfo = ref<any>({})
const backupList = ref<any[]>([])
const backupCreating = ref(false)
const exportingData = ref(false)

function goPrivacy() { router.push('/privacy') }
function goBoundary() { router.push('/usage-boundary') }
function goPrivacyScan() { router.push('/privacy-scan') }
function goStorage() { router.push('/storage') }

async function loadStorageInfo() {
  storageLoading.value = true
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/storage/info")
    if (data?.data) storageInfo.value = data.data
  } catch {}
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/storage/backups")
    if (Array.isArray(data?.data)) backupList.value = data.data
  } catch {}
  storageLoading.value = false
}

async function createBackup() {
  backupCreating.value = true
  try {
    await axios.post(apiBaseUrl.value + "/api/storage/backup")
    ElMessage.success("备份创建成功")
    await loadStorageInfo()
  } catch (err: any) {
    ElMessage.error("创建备份失败: " + (err?.response?.data?.msg || err.message))
  } finally { backupCreating.value = false }
}

async function exportUserData() {
  exportingData.value = true
  try {
    await axios.post(apiBaseUrl.value + "/api/storage/export-user-data")
    ElMessage.success("导出请求已提交，请查看服务器目录")
  } catch (err: any) {
    ElMessage.error("导出失败: " + (err?.response?.data?.msg || err.message))
  } finally { exportingData.value = false }
}

onMounted(async () => {
  apiBaseUrl.value = await getApiBaseURL()
  const deploymentConfig = await getDeploymentConfig()
  if (deploymentConfig?.mode === "local") runtimeModeLabel.value = "桌面本地"
  if (deploymentConfig?.mode === "cloud") runtimeModeLabel.value = "桌面云端"
  if (deploymentConfig?.mode === "self-hosted") runtimeModeLabel.value = "桌面自建"
  loadTimingOverview()
  loadStylePrompt()
  loadAbout()
  loadStorageInfo()
})

async function saveStylePrompt() {
  savingPrompt.value = true
  try {
    await axios.put(apiBaseUrl.value + "/api/config", { settings: { wechat_style_prompt: styleForm.prompt } })
    ElMessage.success("AI回复风格提示词已保存")
  } catch (err: any) { ElMessage.error("保存失败: " + err.message) }
  finally { savingPrompt.value = false }
}

function resetStylePrompt() { styleForm.prompt = DEFAULT_STYLE_PROMPT }


async function loadStylePrompt() {
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/config")
    if (data?.data?.settings?.wechat_style_prompt) {
      styleForm.prompt = data.data.settings.wechat_style_prompt
    }
  } catch {}
}

async function loadTimingOverview() {
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/reply-timing/overview")
    if (data?.data) timingOverview.value = data.data
  } catch {}
}

async function loadAbout() {
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/about")
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

.theme-preset-list { display: flex; flex-direction: column; gap: 10px; }
.theme-preset-item { display: flex; align-items: center; gap: 14px; padding: 12px 16px; border: 2px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); cursor: pointer; transition: all 0.2s; background: var(--ac-color-surface); }
.theme-preset-item:hover { border-color: var(--ac-color-primary); background: var(--ac-color-surface-hover); }
.theme-preset-item.active { border-color: var(--ac-color-primary); background: var(--ac-color-primary-bg); }
.theme-preset-preview { width: 48px; height: 48px; border-radius: 8px; flex-shrink: 0; border: 1px solid var(--ac-color-border); }
.preview-light { background: linear-gradient(135deg, #F7FAFF 0%, #FFFFFF 52%, #3B82F6 100%); }
.preview-dark { background: linear-gradient(135deg, #070B10 0%, #111820 58%, #3B82F6 100%); }
.preview-system { background: linear-gradient(135deg, #F7FAFF 0 50%, #070B10 50% 100%); }
.theme-preset-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
.theme-preset-name { font-size: var(--ac-font-size-sm); font-weight: 500; color: var(--ac-color-text); }
.theme-preset-desc { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }

.privacy-grid { display: flex; flex-direction: column; gap: 8px; }
.privacy-card { display: flex; align-items: center; gap: 12px; padding: 12px 14px; border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); cursor: pointer; transition: all 0.15s; background: var(--ac-color-surface); }
.privacy-card:hover { border-color: var(--ac-color-primary); background: var(--ac-color-surface-hover); }
.privacy-card .pc-body { flex: 1; min-width: 0; }
.privacy-card .pc-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); margin-bottom: 2px; }
.privacy-card .pc-desc { font-size: 12px; color: var(--ac-color-text-muted); line-height: 1.4; }
</style>




