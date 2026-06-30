<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">语音识别</h2>

    <el-alert v-if="!apiKeyConfigured" type="warning" :closable="false" show-icon style="margin-bottom:14px">
      <template #title>
        请先在 <router-link to="/model">模型配置</router-link> 中设置语音识别 API Key
      </template>
    </el-alert>

    <el-alert v-else type="info" :closable="false" show-icon style="margin-bottom:14px">
      <template #title>
        录音文件识别需要音频的公网URL。本地开发请使用 ngrok 暴露本地服务，或上传音频到云存储后填入URL。
      </template>
    </el-alert>

    <el-card>
      <template #header>API 配置</template>
      <div v-if="apiKeyConfigured">
        <span style="color:var(--el-color-success);font-size:13px">API Key 已配置 (从模型配置中读取)</span>
      </div>
      <div v-else>
        <span style="color:var(--el-color-danger);font-size:13px">未配置 API Key</span>
      </div>
    </el-card>

    <el-card style="margin-top:14px">
      <template #header>上传音频并识别</template>
      <el-form label-position="top">
        <el-form-item label="上传音频文件">
          <el-upload :auto-upload="false" :limit="1" accept=".mp3,.wav,.ogg,.m4a,.aac,.pcm" :on-change="onFileChange" :on-remove="() => resetFile()">
            <el-button size="small">选择音频文件</el-button>
            <template #tip><div class="form-hint">支持 mp3/wav/ogg/m4a/aac/pcm，最大60分钟</div></template>
          </el-upload>
        </el-form-item>
        <el-form-item label="语言">
          <el-select v-model="language" style="width:200px" clearable placeholder="自动识别">
            <el-option value="zh-CN" label="中文普通话" />
            <el-option value="en-US" label="英语" />
            <el-option value="ja-JP" label="日语" />
            <el-option value="ko-KR" label="韩语" />
          </el-select>
        </el-form-item>
        <el-form-item label="音频公网URL">
          <el-input v-model="audioUrl" placeholder="输入音频的公网URL（或先上传文件获取本地URL后再用ngrok暴露）" />
          <div class="form-hint">火山引擎服务器需要能访问到此URL。本地文件先上传后复制地址填入</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="submitTask" :loading="submitting" :disabled="!audioUrl.trim() || !apiKeyConfigured">提交识别</el-button>
          <el-button @click="stopPolling" v-if="pollTimer" :loading="false">停止轮询</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="taskId" style="margin-top:14px">
      <template #header>识别结果</template>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="任务ID">{{ taskId }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="statusTagType">{{ status || '等待查询' }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>
      <div v-if="result" class="result-box">{{ result }}</div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { apiClient } from "../../ui-index"

const audioFile = ref<File | null>(null)
const audioUrl = ref("")
const language = ref("")
const taskId = ref("")
const status = ref("")
const result = ref("")
const submitting = ref(false)
const polling = ref(false)
const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
const apiKeyConfigured = ref(false)

const statusTagType = computed(() => {
  const s = status.value
  if (!s) return "info"
  if (s === "completed" || s === "success") return "success"
  if (s === "failed" || s === "error") return "danger"
  if (s === "processing" || s === "running") return "warning"
  return "info"
})

onMounted(async () => {
  try {
    const resp = await apiClient.get("/api/asr/configs")
    const configs = (resp as any)?.data || resp || []
    const cfg = configs.find((c: any) => c.isActive) || configs[0]
    apiKeyConfigured.value = !!(cfg && cfg.hasApiKey)
  } catch {
    apiKeyConfigured.value = false
  }
})

function resetFile() {
  audioFile.value = null
  audioUrl.value = ""
}

function onFileChange(file: any) {
  audioFile.value = file.raw || file
  uploadFile()
}

async function uploadFile() {
  if (!audioFile.value) return
  const formData = new FormData()
  formData.append("audio", audioFile.value)
  const token = localStorage.getItem("ai-companion-token")
  try {
    const resp = await fetch("/api/asr/upload", {
      method: "POST",
      headers: token ? { Authorization: "Bearer " + token } : {},
      body: formData,
    })
    const json = await resp.json()
    if (json.code === 200) {
      audioUrl.value = json.data?.url || ""
      ElMessage.success("文件已上传，URL已自动填入")
    } else {
      ElMessage.error(json.message || "上传失败")
    }
  } catch (err: any) {
    ElMessage.error("上传失败: " + (err?.message || "未知错误"))
  }
}

async function submitTask() {
  if (!audioUrl.value.trim()) return
  if (!apiKeyConfigured.value) { ElMessage.warning("请先在模型配置中设置语音识别API Key"); return }
  submitting.value = true
  try {
    const formData = new FormData()
    formData.append("audioUrl", audioUrl.value.trim())
    if (language.value) formData.append("language", language.value)
    const token = localStorage.getItem("ai-companion-token")
    const resp = await fetch("/api/asr/submit", {
      method: "POST",
      headers: token ? { Authorization: "Bearer " + token } : {},
      body: formData,
    })
    const json = await resp.json()
    if (json.code !== 200) { ElMessage.error(json.message || "提交失败"); return }
    taskId.value = json.data?.taskId || ""
    status.value = "已提交"
    result.value = ""
    ElMessage.success("任务已提交，自动轮询中...")
    startPolling()
  } catch (err: any) { ElMessage.error(err?.message || "提交失败") }
  finally { submitting.value = false }
}

function startPolling() {
  if (pollTimer.value) return
  pollTimer.value = setInterval(() => pollResult(), 3000)
  pollResult()
}

function stopPolling() {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
}

async function pollResult() {
  if (!taskId.value) { stopPolling(); return }
  polling.value = true
  try {
    const resp = await apiClient.get("/api/asr/query?taskId=" + taskId.value)
    const data = (resp as any)?.data || resp
    status.value = data?.status || "未知"
    if (data?.result) {
      result.value = data.result
      stopPolling()
      ElMessage.success("识别完成")
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "查询失败")
    stopPolling()
  }
  finally { polling.value = false }
}
</script>

<style scoped>
.page { max-width: 640px; margin: 0 auto; padding: 20px 16px; }
.page-title { font-size: 20px; font-weight: 600; margin: 0 0 16px; }
.result-box { margin-top: 12px; padding: 14px; background: var(--el-fill-color-light); border-radius: 8px; white-space: pre-wrap; font-size: 14px; line-height: 1.7; }
.form-hint { font-size: 11px; color: var(--el-text-color-secondary); margin-top: 4px; }
</style>
