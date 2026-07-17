<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="wechat-page">
    <h2 class="page-title">QQ 连接</h2>
    <div v-if="!pageReady" style="padding:40px 0;color:#909399">
      <el-icon class="is-loading" :size="24"><Loading /></el-icon>
      <p style="margin-top:8px">检测连接状态...</p>
    </div>
    <template v-if="pageReady">

    <template v-if="qqOnline">
      <el-alert type="success" :closable="false" show-icon style="margin-bottom: 14px">
        <template #title>QQ Bot 已成功连接</template>
      </el-alert>
      <el-card shadow="never" class="section-card">
        <template #header>
          <div class="card-header-row">
            <span class="card-header-title">连接状态</span>
            <div class="header-actions">
              <el-button size="small" @click="refreshStatus" :loading="loading">刷新</el-button>
              <el-button size="small" type="success" @click="doReconnect" :loading="reconnecting">重新连接</el-button>
              <el-button size="small" type="danger" @click="doDisconnect" :loading="disconnecting">断开</el-button>
            </div>
          </div>
        </template>
        <div class="status-main">
          <div class="status-row">
            <div class="status-indicator ok"></div>
            <span class="status-label">已连接</span>
          </div>
          <div class="status-detail-grid">
            <div class="sd-item">
              <span class="sd-label">消息数</span>
              <span class="sd-value">{{ statusMessageCount }}</span>
            </div>
            <div class="sd-item">
              <span class="sd-label">回复数</span>
              <span class="sd-value">{{ statusReplyCount }}</span>
            </div>
            <div class="sd-item" v-if="accountId">
              <span class="sd-label">Bot ID</span>
              <span class="sd-value">{{ accountId.slice(0, 12) }}...</span>
            </div>
            <div class="sd-item">
              <span class="sd-label">协议</span>
              <span class="sd-value">QQBot (WebSocket)</span>
            </div>
            <div class="sd-item" v-if="startedAt">
              <span class="sd-label">连接时间</span>
              <span class="sd-value">{{ formatStartedAt(startedAt) }}</span>
            </div>
          </div>
        </div>
      </el-card>

      <el-alert type="warning" :closable="false" show-icon style="margin-top: 12px">
        <template #title>主动推送须知</template>
        添加QQ好友后，必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
      </el-alert>
    </template>

    <template v-if="!qqOnline">
      <el-card shadow="never" class="section-card">
        <template #header><span class="card-header-title">QQBot 配置</span></template>
        <div class="pwd-login">
          <el-form label-width="70px" @submit.prevent="doConnect">
            <el-form-item label="AppID">
              <el-input v-model="appId" placeholder="输入Bot AppID" style="width:100%;max-width:400px" />
            </el-form-item>
            <el-form-item label="Token">
              <el-input v-model="token" type="password" placeholder="输入Bot Token" style="width:100%;max-width:400px" show-password />
            </el-form-item>
            <el-form-item label="沙箱模式">
              <el-switch v-model="sandbox" />
              <span style="margin-left:8px;font-size:12px;color:#909399">{{ sandbox ? "沙箱环境" : "正式环境" }}</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="connecting" @click="doConnect">连接</el-button>
              <span v-if="loginStatus === 'connecting'" style="margin-left:10px;font-size:12px;color:#e6a23c">
                <el-icon class="is-loading"><Loading /></el-icon> 连接中...
              </span>

            </el-form-item>
          </el-form>
        </div>
      </el-card>

      <el-card shadow="never" class="section-card" style="margin-top:12px">
        <template #header><span class="card-header-title">使用说明</span></template>
        <div style="font-size:13px;color:#606266;line-height:1.8">
          <p>1. 前往 <a href="https://q.qq.com/" target="_blank" style="color:#409eff">QQ开放平台</a> 创建机器人</p>
          <p>2. 获取 AppID 和 Token</p>
          <p>3. 填入上方表单，点击"连接"</p>
          <p>4. 连接成功后，在QQ中 @机器人 即可对话</p>
        </div>
      </el-card>

      <el-alert type="warning" :closable="false" show-icon style="margin-top: 12px">
        <template #title>主动推送须知</template>
        连接成功后，添加QQ好友必须主动给机器人发一条消息，系统才能记录你的用户ID用于主动推送。用户ID每7天自动刷新，届时需重新发送一条消息。
      </el-alert>
    </template>

    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject } from "vue"
import { Loading } from "@element-plus/icons-vue"
import { ElMessage, ElMessageBox } from "element-plus"
import axios from "axios"
import { getQQApiBaseURL } from "../../runtime/runtime-adapter"
import { useApi } from "../../composables/useApi"

const { get } = useApi()

const refreshHealth = inject<() => Promise<void>>("refreshHealth")

const qqApiBaseUrl = ref("")

const pageReady = ref(false)
const loading = ref(false)
const qqOnline = ref(false)
const accountId = ref<string | null>(null)
const loginStatus = ref("")
const connecting = ref(false)
const disconnecting = ref(false)
const reconnecting = ref(false)
const statusMessageCount = ref(0)
const statusReplyCount = ref(0)
const startedAt = ref("")

const appId = ref("")
const token = ref("")
const sandbox = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

let connectPollTimer: ReturnType<typeof setInterval> | null = null

let lastShownError: string = ""

function stopConnectPoll() {
  if (connectPollTimer) { clearInterval(connectPollTimer); connectPollTimer = null }
}

function formatStartedAt(iso: string): string {
  if (!iso) return ""
  try {
    const d = new Date(iso)
    const pad = (n: number) => String(n).padStart(2, "0")
    return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return iso
  }
}

async function doConnect() {
  if (!appId.value || !token.value) return
  connecting.value = true
  loginStatus.value = ""
  try {
    await axios.post(qqApiBaseUrl.value + "/connect", {
      appId: appId.value,
      token: token.value,
      sandbox: sandbox.value,
    })
    loginStatus.value = "connecting"
    stopConnectPoll()
    const startTime = Date.now()
    connectPollTimer = setInterval(async () => {
      await refreshStatus()
      if (qqOnline.value) {
        stopConnectPoll()
        connecting.value = false
        loginStatus.value = ""
        ElMessage.success("QQ Bot 连接成功")
        refreshHealth?.()
        return
      }
      if (Date.now() - startTime > 30000) {
        stopConnectPoll()
        connecting.value = false
        loginStatus.value = ""
        ElMessage.error("连接超时，请检查AppID和Token是否有效")
      }
    }, 2000)
  } catch (e: any) {
    const msg = e?.response?.data?.error || "连接失败，请检查AppID和Token"
    ElMessage.error(msg)
    connecting.value = false
  }
}

async function doReconnect() {
  reconnecting.value = true
  try {
    const cfg = await axios.get(qqApiBaseUrl.value + "/config")
    if (!cfg.data?.appId || !cfg.data?.token) {
      ElMessage.warning("未找到已保存的凭证，请手动重新连接")
      reconnecting.value = false
      return
    }
    await axios.post(qqApiBaseUrl.value + "/disconnect")
    await new Promise(r => setTimeout(r, 1000))
    await axios.post(qqApiBaseUrl.value + "/connect", {
      appId: cfg.data.appId,
      token: cfg.data.token,
      sandbox: cfg.data.sandbox || false,
    })
    loginStatus.value = "connecting"
    stopConnectPoll()
    const startTime = Date.now()
    connectPollTimer = setInterval(async () => {
      await refreshStatus()
      if (qqOnline.value) {
        stopConnectPoll()
        reconnecting.value = false
        loginStatus.value = ""
        ElMessage.success("QQ Bot 重新连接成功")
        refreshHealth?.()
        return
      }
      if (Date.now() - startTime > 30000) {
        stopConnectPoll()
        reconnecting.value = false
        loginStatus.value = ""
        ElMessage.error("重新连接超时")
      }
    }, 2000)
  } catch (e: any) {
    ElMessage.error("重新连接失败")
    reconnecting.value = false
  }
}

async function doDisconnect() {
  disconnecting.value = true
  try {
    await axios.post(qqApiBaseUrl.value + "/disconnect")
    qqOnline.value = false
    accountId.value = null
    startedAt.value = ""
    loginStatus.value = ""
    refreshHealth?.()
  } catch (e: any) {}
  disconnecting.value = false
}

async function refreshStatus() {
  try {
    const res = await axios.get(qqApiBaseUrl.value + "/status")
    const data = res.data?.data || res.data
    qqOnline.value = !!data?.qqOnline
    accountId.value = data?.accountId || null
    loginStatus.value = data?.status || ""
    startedAt.value = data?.startedAt || ""
    statusMessageCount.value = data?.messageCount ?? 0
		statusReplyCount.value = data?.replyCount ?? 0

    if (qqOnline.value && loginStatus.value !== "connecting") {
      if (loginStatus.value === "connecting") {
        ElMessage.success("QQ Bot 连接成功")
      }
      loginStatus.value = ""
    }
    const err = data?.error || ""
    if (err && loginStatus.value !== "connecting" && loginStatus.value !== "online") {
      if (err !== lastShownError) {
        lastShownError = err
        ElMessage.error(err)
      }
    }
    if (!err) {
      lastShownError = ""
    }
    if (!qqOnline.value) {
      try {
        const cfg = await axios.get(qqApiBaseUrl.value + "/config")
        if (cfg.data?.appId) {
          appId.value = cfg.data.appId
          token.value = cfg.data.token || ""
          sandbox.value = cfg.data.sandbox || false
        }
      } catch {}
    }
  } catch {
    qqOnline.value = false
  } finally {
    pageReady.value = true
  }
}

onMounted(async () => {
  qqApiBaseUrl.value = await getQQApiBaseURL()
  await refreshStatus()
  pollTimer = setInterval(refreshStatus, 3000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  stopConnectPoll()
})
</script>

<style scoped>
.wechat-page { margin: 0; }
.page-title { font-size: 24px; font-weight: 700; color: var(--ac-color-text); margin-bottom: 16px; }
.section-card { margin-bottom: 16px; }
.card-header-row { display: flex; align-items: center; justify-content: space-between; }
.header-actions { display: flex; gap: 8px; }
.card-header-title { font-size: 14px; font-weight: 600; color: var(--ac-color-text); }
.status-main { display: flex; flex-direction: column; gap: 12px; }
.status-row { display: flex; align-items: center; gap: 10px; }
.status-indicator { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.status-indicator.ok { background: #5a9e6f; }
.status-label { font-size: 16px; font-weight: 600; color: var(--ac-color-text); }
.status-detail-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 8px; }
.sd-item { padding: 8px 12px; background: var(--ac-color-bg-secondary); border-radius: 4px; }
.sd-label { font-size: 11px; color: var(--ac-color-text-muted); display: block; }
.sd-value { font-size: 14px; font-weight: 600; color: var(--ac-color-text); }
.pwd-login { padding: 8px 0; }
</style>
