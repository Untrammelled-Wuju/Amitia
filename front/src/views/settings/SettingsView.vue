<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="settings-view">
    <el-tabs v-model="activeTab" type="border-card">
      <el-tab-pane label="运行维护" name="maintenance">
        <StatusPanel ref="statusPanelRef" />
        <ManualActionsPanel @action-completed="refreshStatus" />
        <ConfigPanel />

        <el-card shadow="never" class="section-card">
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
      </el-tab-pane>

      <el-tab-pane label="AI 配置" name="ai-config">
        <el-card shadow="never" class="section-card">
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
      </el-tab-pane>

      <el-tab-pane label="系统设置" name="system">
        <el-card shadow="never" class="section-card">
          <template #header><span>主题设置</span></template>
          <div class="nav-entry" @click="router.push('/settings/theme')">
            <el-icon size="22" color="var(--ac-color-primary)"><Brush /></el-icon>
            <div class="nav-entry-body">
              <div class="nav-entry-title">主题设置</div>
              <div class="nav-entry-desc">切换亮色/暗色主题，自定义外观</div>
            </div>
            <el-icon><ArrowRight /></el-icon>
          </div>
        </el-card>

        <el-card shadow="never" class="section-card">
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

        <el-card shadow="never" class="section-card">
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
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter } from "vue-router"
import axios from "axios"
import { ElMessage } from "element-plus"
import {
  Brush, ArrowRight, Loading, FolderAdd, Delete, Download
} from '@element-plus/icons-vue'
import { getApiBaseURL, getDeploymentConfig } from "../../runtime/runtime-adapter"
import StatusPanel from "../long-running/components/StatusPanel.vue"
import ManualActionsPanel from "../long-running/components/ManualActionsPanel.vue"
import ConfigPanel from "../long-running/components/ConfigPanel.vue"

const router = useRouter()
const apiBaseUrl = ref("")
const runtimeModeLabel = ref("浏览器")
const savingPrompt = ref(false)
const activeTab = ref("maintenance")
const statusPanelRef = ref<InstanceType<typeof StatusPanel> | null>(null)

const DEFAULT_STYLE_PROMPT = "你和用户是比较熟悉的长期对话关系，不需要像客服或正式助手一样说话。回复要自然、有反应、有一点态度，可以适当使用「嗯？、喔、奥奥、ok、好、行、确实、懂了」等语气词。用户随口聊，你就自然接话；用户认真问问题，你再认真回答。不要客服腔，不要过度正式，不要每次都完整总结，也不要动不动分点讲大道理。回复格式要像微信连续消息：用户发一句话时，你可以回复 1 到 4 句短句。不要写成一整段长文。整体目标是：像一个熟悉用户、说话自然、有判断力的人。该短就短，该认真就认真，不端着，也不表演过头。回复中不要使用任何emoji表情符号。不能使用markdown格式。"
const DEFAULT_SOURCE_CODE_URL = "https://gitee.com/Untrammelled/Amitia"
const DEFAULT_COMMERCIAL_LICENSE_URL = "mailto:3151508592@qq.com"

onMounted(() => {
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

function goStorage() { router.push('/storage') }

function refreshStatus() {
  statusPanelRef.value?.refresh()
}

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
.settings-view { }
.section-card { margin-bottom: 12px; border: 1px solid var(--ac-color-border-light); }
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
.legal-links { display: flex; gap: 14px; flex-wrap: wrap; margin-top: 12px; }

.nav-entry {
  display: flex; align-items: center; gap: 12px; padding: 12px 14px;
  border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md);
  cursor: pointer; transition: all 0.15s; background: var(--ac-color-surface);
}
.nav-entry:hover { border-color: var(--ac-color-primary); background: var(--ac-color-surface-hover); }
.nav-entry-body { flex: 1; min-width: 0; }
.nav-entry-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); margin-bottom: 2px; }
.nav-entry-desc { font-size: 12px; color: var(--ac-color-text-muted); line-height: 1.4; }
</style>
