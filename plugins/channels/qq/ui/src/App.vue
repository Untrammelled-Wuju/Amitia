<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue"
import { createChannelClient, statusLabel } from "./session.mjs"

const detail = ref({ status: "disconnected", qqOnline: false })
const form = ref({ appId: "", token: "", sandbox: false })
const ready = ref(false)
const busy = ref("")
const error = ref("")
const confirmation = ref("")
const connected = computed(() => detail.value.qqOnline === true || detail.value.status === "online")
const label = computed(() => statusLabel(connected.value ? "online" : detail.value.status))
let invoke
let disposed = false
let pollTimer
let refreshing = false
let revision = 0

function failure(reason) {
  return reason instanceof Error ? reason.message : String(reason || "操作失败，请重试")
}

function applyStatus(next) {
  detail.value = next && typeof next === "object" ? next : { status: "disconnected", qqOnline: false }
}

async function refresh(manual = false) {
  if (!invoke || disposed || refreshing || busy.value) return
  refreshing = true
  const current = revision
  if (manual) busy.value = "refresh"
  try {
    const next = await invoke("status")
    if (!disposed && current === revision) {
      applyStatus(next)
      error.value = ""
    }
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason)
  } finally {
    refreshing = false
    if (manual && !disposed) busy.value = ""
    schedulePoll()
  }
}

function schedulePoll() {
  clearTimeout(pollTimer)
  if (disposed || document.hidden || !invoke) return
  pollTimer = setTimeout(() => {
    void refresh()
  }, invoke.nextPollDelay(connected.value))
}

function onVisibilityChange() {
  clearTimeout(pollTimer)
  if (!document.hidden && !disposed) void refresh()
}

async function loadConfig() {
  if (!invoke) return
  try {
    const config = await invoke("config")
    form.value = {
      appId: String(config?.appId || ""),
      token: String(config?.token || ""),
      sandbox: config?.sandbox === true,
    }
  } catch {}
}

async function connect() {
  if (!invoke || busy.value) return
  const appId = form.value.appId.trim()
  const token = form.value.token.trim()
  if (!appId || !token) {
    error.value = "请输入 QQ Bot 的 AppID 和 Secret"
    return
  }
  const current = ++revision
  busy.value = "connect"
  error.value = ""
  confirmation.value = ""
  try {
    await invoke("connect", {
      config: { appId, token, sandbox: form.value.sandbox === true },
    })
    if (disposed || current !== revision) return
    const deadline = Date.now() + 30000
    while (!disposed && current === revision && Date.now() < deadline) {
      const next = await invoke("status")
      applyStatus(next)
      if (next?.qqOnline === true || next?.status === "online") return
      await new Promise((resolve) => setTimeout(resolve, 2000))
    }
    throw new Error("连接超时，请检查 AppID、Secret 和网络")
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason)
  } finally {
    if (!disposed && current === revision) {
      busy.value = ""
      schedulePoll()
    }
  }
}

async function reconnect() {
  if (!invoke || busy.value) return
  const current = ++revision
  busy.value = "reconnect"
  error.value = ""
  confirmation.value = ""
  try {
    await invoke("disconnect")
    await new Promise((resolve) => setTimeout(resolve, 800))
    await connect()
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason)
  } finally {
    if (!disposed && current === revision) busy.value = ""
  }
}

async function disconnect() {
  if (!invoke || busy.value) return
  const current = ++revision
  busy.value = "disconnect"
  confirmation.value = ""
  error.value = ""
  try {
    await invoke("disconnect")
    if (!disposed && current === revision) applyStatus({ status: "disconnected", qqOnline: false })
  } catch (reason) {
    if (!disposed && current === revision) error.value = failure(reason)
  } finally {
    if (!disposed && current === revision) busy.value = ""
  }
}

async function openChannelMessages() {
  const bridge = window.amitiaUI
  if (typeof bridge?.navigate !== "function") {
    error.value = "当前宿主不支持页面跳转"
    return
  }
  try {
    await bridge.navigate("/channel-messages", "page")
  } catch (reason) {
    error.value = failure(reason)
  }
}

async function initialize() {
  if (busy.value) return
  error.value = ""
  try {
    const bridge = window.amitiaUI
    if (typeof bridge?.ready === "function") await bridge.ready()
    if (disposed) return
    invoke = createChannelClient(bridge)
    ready.value = true
    await Promise.all([refresh(), loadConfig()])
  } catch (reason) {
    if (!disposed) error.value = failure(reason)
  }
}

onMounted(() => {
  document.addEventListener("visibilitychange", onVisibilityChange)
  void initialize()
})

onUnmounted(() => {
  disposed = true
  revision++
  clearTimeout(pollTimer)
  document.removeEventListener("visibilitychange", onVisibilityChange)
})
</script>

<template>
  <main class="qq-page" aria-labelledby="qq-title">
    <header class="page-heading">
      <div>
        <p class="eyebrow">QQ BOT · 官方渠道</p>
        <h1 id="qq-title">QQ 渠道</h1>
        <p class="subtitle">使用 QQ 开放平台的 AppID 与 Secret 连接机器人，接收私聊和群聊消息。</p>
      </div>
      <span class="connection-badge" :class="{ connected }" role="status">{{ ready ? label : "正在连接插件…" }}</span>
    </header>

    <div v-if="error" class="error-message" role="alert">
      <span>{{ error }}</span>
      <button type="button" :disabled="!!busy" @click="ready ? refresh(true) : initialize()">{{ ready ? "重新检查" : "重试" }}</button>
    </div>

    <section v-if="!ready && !error" class="loading-panel" aria-busy="true">正在检测 QQ 连接状态…</section>

    <section v-if="ready" class="connection-panel" :aria-busy="!!busy">
      <div class="connection-info">
        <template v-if="connected">
          <h2>QQ Bot 已连接</h2>
          <p class="section-description">渠道服务正在运行，可以接收 QQ 私聊与群聊消息，并将角色回复发送回 QQ。</p>
          <dl class="account-details">
            <div v-if="detail.accountId"><dt>Bot AppID</dt><dd>{{ detail.accountId }}</dd></div>
            <div><dt>接收消息</dt><dd>{{ detail.messageCount ?? 0 }}</dd></div>
            <div><dt>发送回复</dt><dd>{{ detail.replyCount ?? 0 }}</dd></div>
            <div v-if="detail.startedAt"><dt>连接时间</dt><dd>{{ new Date(detail.startedAt).toLocaleString() }}</dd></div>
          </dl>
          <div class="actions">
            <button class="primary-button" type="button" :disabled="!!busy" @click="openChannelMessages">查看渠道消息</button>
            <button type="button" :disabled="!!busy" @click="refresh(true)">{{ busy === "refresh" ? "刷新中…" : "刷新状态" }}</button>
            <button type="button" :disabled="!!busy" @click="reconnect">{{ busy === "reconnect" ? "重连中…" : "重新连接" }}</button>
            <button class="danger-button" type="button" :disabled="!!busy" @click="confirmation = 'disconnect'">断开连接</button>
          </div>
        </template>
        <template v-else>
          <h2>连接 QQ Bot</h2>
          <p class="section-description">QQ 官方 Bot 使用 AppID 与 Secret 建立 WebSocket 连接，不提供账号扫码登录。</p>
          <form class="credential-form" @submit.prevent="connect">
            <label>
              <span>AppID</span>
              <input v-model="form.appId" autocomplete="off" placeholder="填写 QQ 开放平台 Bot AppID" />
            </label>
            <label>
              <span>Secret</span>
              <input v-model="form.token" type="password" autocomplete="new-password" placeholder="填写 Bot Secret" />
            </label>
            <label class="switch-row">
              <input v-model="form.sandbox" type="checkbox" />
              <span>使用沙箱环境</span>
            </label>
            <div class="actions">
              <button class="primary-button" type="submit" :disabled="!!busy">{{ busy === "connect" ? "正在连接…" : "连接 QQ Bot" }}</button>
              <button type="button" :disabled="!!busy" @click="openChannelMessages">查看渠道消息</button>
            </div>
          </form>
        </template>
        <div v-if="confirmation" class="confirmation" role="alert">
          <p>断开后将停止接收和回复 QQ 消息，是否继续？</p>
          <div class="actions">
            <button class="primary-button" type="button" :disabled="!!busy" @click="disconnect">确认断开</button>
            <button type="button" @click="confirmation = ''">取消</button>
          </div>
        </div>
      </div>

      <aside class="side-panel">
        <h2>连接说明</h2>
        <ol>
          <li>在 QQ 开放平台创建机器人并获取 AppID、Secret。</li>
          <li>填写凭证后点击连接，页面会自动等待 Bot 上线。</li>
          <li>在 QQ 中 @机器人 或在私聊中发送消息即可对话。</li>
        </ol>
        <a href="https://q.qq.com/" target="_blank" rel="noreferrer">打开 QQ 开放平台</a>
        <p v-if="detail.error" class="service-error">{{ detail.error }}</p>
      </aside>
    </section>
  </main>
</template>
