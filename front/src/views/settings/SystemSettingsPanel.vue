<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="system-settings-panel">
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
      <div class="nav-entry nav-entry-spaced" @click="router.push('/settings/temporal')">
        <el-icon size="22" color="var(--ac-color-primary)"><Clock /></el-icon>
        <div class="nav-entry-body">
          <div class="nav-entry-title">时间与地区</div>
          <div class="nav-entry-desc">配置时区、安静时段、节日地区和时间锚点</div>
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
        <div style="margin-top:10px;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
          <el-select v-model="exportCharId" placeholder="选择角色(可选)" size="small" style="width:160px" clearable filterable>
            <el-option v-for="c in exportCharacters" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
          <el-button size="small" @click="exportAmitiaData('character')" :disabled="!exportCharId" :loading="exportingAmitia"><el-icon><User /></el-icon> 导出角色数据</el-button>
          <el-button size="small" @click="exportAmitiaData('all')" :loading="exportingAmitia"><el-icon><Download /></el-icon> 导出所有数据</el-button>
        </div>
        <div style="margin-top:10px;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
          <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleImportFile" accept=".json">
            <el-button size="small" :loading="importingAmitia"><el-icon><Upload /></el-icon> 导入数据</el-button>
          </el-upload>
          <span v-if="importResult" style="font-size:12px;color:var(--ac-color-text-muted)">{{ importResult }}</span>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from "vue"
import { useRouter } from "vue-router"
import axios from "axios"
import { ElMessage } from "element-plus"
import {
  Brush, ArrowRight, Clock, Loading, FolderAdd, Delete, Download, User, Upload
} from '@element-plus/icons-vue'
import { getApiBaseURL, getDeploymentConfig } from "../../runtime/runtime-adapter"

const router = useRouter()
const apiBaseUrl = ref("")
const runtimeModeLabel = ref("浏览器")

const DEFAULT_SOURCE_CODE_URL = "https://gitee.com/Untrammelled/Amitia"
const DEFAULT_COMMERCIAL_LICENSE_URL = "mailto:3151508592@qq.com"

const aboutInfo = reactive({ name: "Amitia", displayName: "阿米提亚", license: "AGPL-3.0-only", copyright: "Copyright © 2026 彭旭" })

const legalLinks = reactive({
  sourceCode: ((import.meta as any).env?.VITE_AMITIA_SOURCE_CODE_URL || DEFAULT_SOURCE_CODE_URL) as string,
  commercialLicensing: ((import.meta as any).env?.VITE_AMITIA_COMMERCIAL_LICENSE_URL || DEFAULT_COMMERCIAL_LICENSE_URL) as string,
  thirdPartyNotices: ((import.meta as any).env?.VITE_AMITIA_THIRD_PARTY_NOTICES_URL || DEFAULT_SOURCE_CODE_URL + "/blob/master/THIRD_PARTY_NOTICES.md") as string,
})

const storageLoading = ref(false)
const storageInfo = ref<any>({})
const backupList = ref<any[]>([])
const backupCreating = ref(false)
const exportingData = ref(false)

const exportCharId = ref("")
const exportCharacters = ref<any[]>([])
const exportingAmitia = ref(false)

const importingAmitia = ref(false)
const importResult = ref("")


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

async function exportAmitiaData(scope: string) {
  exportingAmitia.value = true
  try {
    const res = await axios.post(apiBaseUrl.value + "/api/storage/export-amitia", {
      scope,
      characterId: scope === "character" ? exportCharId.value : "",
    })
    const data = res.data?.data || res.data
    if (data?.file) {
      window.open(apiBaseUrl.value + "/api/storage/export-download/" + encodeURIComponent(data.file), "_blank")
    }
    ElMessage.success(data?.message || "导出成功")
  } catch (err: any) {
    ElMessage.error("导出失败: " + (err?.response?.data?.msg || err.message))
  } finally { exportingAmitia.value = false }
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

async function loadExportCharacters() {
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/characters")
    if (Array.isArray(data?.data)) exportCharacters.value = data.data
  } catch {}
}


async function handleImportFile(file: any) {
  importingAmitia.value = true
  importResult.value = ""
  try {
    const formData = new FormData()
    formData.append("file", file.raw)
    const res = await axios.post(apiBaseUrl.value + "/api/storage/import-amitia", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    })
    const data = res.data?.data || res.data
    if (data?.imported) {
      const stats = data.stats || {}
      const tableCount = Object.keys(stats).length
      ElMessage.success("导入成功，共 " + data.totalImported + " 条记录，" + tableCount + " 张表")
      importResult.value = "导入完成：共 " + data.totalImported + " 条记录，涉及 " + tableCount + " 张表"
      loadStorageInfo()
    } else {
      ElMessage.error("导入失败: " + (data?.error || "未知错误"))
    }
  } catch (err: any) {
    ElMessage.error("导入失败: " + (err?.response?.data?.msg || err.message))
  } finally { importingAmitia.value = false }
}

onMounted(async () => {
  apiBaseUrl.value = await getApiBaseURL()
  const deploymentConfig = await getDeploymentConfig()
  if (deploymentConfig?.mode === "local") runtimeModeLabel.value = "桌面本地"
  if (deploymentConfig?.mode === "cloud") runtimeModeLabel.value = "桌面云端"
  loadAbout()
  loadStorageInfo()
  loadExportCharacters()
})
</script>

<style scoped>
.system-settings-panel { }
.section-card { margin-bottom: 12px; border: 1px solid var(--ac-color-border-light); }
.legal-links { display: flex; gap: 14px; flex-wrap: wrap; margin-top: 12px; }

.nav-entry {
  display: flex; align-items: center; gap: 12px; padding: 12px 14px;
  border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md);
  cursor: pointer; transition: all 0.15s; background: var(--ac-color-surface);
}
.nav-entry:hover { border-color: var(--ac-color-primary); background: var(--ac-color-surface-hover); }
.nav-entry-spaced { margin-top: 10px; }
.nav-entry-body { flex: 1; min-width: 0; }
.nav-entry-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); margin-bottom: 2px; }
.nav-entry-desc { font-size: 12px; color: var(--ac-color-text-muted); line-height: 1.4; }
</style>