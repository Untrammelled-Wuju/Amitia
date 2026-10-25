<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue"
import { createChannelClient, createHostBridge, statusLabel } from "./session.mjs"

const props = defineProps({
  uiContext: { type: Object, default: () => ({}) },
  provider: { type: Object, default: null },
  uiActions: { type: Object, default: () => ({}) },
  uiNavigate: { type: Function, default: null },
})

const detail = ref({ status: "disconnected", qqOnline: false })
const config = ref({ token: "", webuiToken: "", webuiError: "", reverseWebSocketUrl: "", groupTrigger: "mention", triggerPrefix: "Amitia" })
const form = ref({ token: "", groupTrigger: "mention", triggerPrefix: "Amitia" })
const ready = ref(false)
const showWebuiToken = ref(false)
const busy = ref("")
const error = ref("")
const notice = ref("")
let invoke
let disposed = false
let timer

const connected = computed(() => detail.value.qqOnline === true || detail.value.status === "online")
const label = computed(() => statusLabel(connected.value ? "online" : detail.value.status))

function failure(reason) {
  return reason instanceof Error ? reason.message : String(reason || "操作失败")
}

function copyWithSelection(text) {
  const area = document.createElement("textarea")
  area.value = text
  area.setAttribute("readonly", "")
  area.style.position = "fixed"
  area.style.left = "-9999px"
  area.style.top = "0"
  area.style.opacity = "0"
  document.body.appendChild(area)
  area.focus()
  area.select()
  area.setSelectionRange(0, area.value.length)
  try {
    if (!document.execCommand("copy")) throw new Error("浏览器复制命令不可用")
  } finally {
    area.remove()
  }
}

function applyConfig(next) {
  config.value = {
    ...config.value,
    ...(next || {}),
    webuiToken: String(next?.webuiToken || ""),
    webuiError: String(next?.webuiError || ""),
  }
  form.value = {
    token: String(next?.token || ""),
    groupTrigger: String(next?.groupTrigger || "mention"),
    triggerPrefix: String(next?.triggerPrefix || "Amitia"),
  }
}

async function refresh(silent = false) {
  if (!invoke || disposed || busy.value) return
  if (!silent) busy.value = "refresh"
  try {
    detail.value = await invoke("status")
    error.value = ""
  } catch (reason) {
    if (!silent) error.value = failure(reason)
  } finally {
    if (!disposed && !silent) busy.value = ""
    schedule()
  }
}

function schedule() {
  clearTimeout(timer)
  if (!disposed) timer = setTimeout(() => void refresh(true), connected.value ? 4000 : 8000)
}

async function connect() {
  if (!invoke || busy.value) return
  busy.value = "connect"
  error.value = ""
  notice.value = ""
  try {
    await invoke("connect", { config: { ...form.value } })
    applyConfig(await invoke("config"))
    await refresh(true)
    notice.value = "桥接已启用，请在 NapCat 中把反向 WebSocket 地址设置为下方地址"
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

async function disconnect() {
  if (!invoke || busy.value) return
  busy.value = "disconnect"
  error.value = ""
  try {
    await invoke("disconnect")
    await refresh(true)
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

async function copy(value, type) {
  error.value = ""
  notice.value = ""
  const text = String(value || "")
  try {
    try {
      copyWithSelection(text)
    } catch {
      await invoke("copy-text", { text })
    }
    const labels = {
      onebot: "OneBot Token",
      webui: "NapCat WebUI Token",
      url: "反向 WebSocket 地址",
    }
    notice.value = `${labels[type] || "内容"}已复制`
  } catch (reason) {
    error.value = "复制失败，请手动选择文本"
    if (String(failure(reason)).trim()) {
      console.warn("[qq-personal] copy failed", failure(reason))
    }
  }
}

async function initialize() {
  try {
    const bridge = createHostBridge(props.uiActions, props.uiNavigate)
    await bridge.ready()
    if (disposed) return
    invoke = createChannelClient(bridge)
    ready.value = true
    const [status, nextConfig] = await Promise.all([invoke("status"), invoke("config")])
    detail.value = status
    applyConfig(nextConfig)
    schedule()
  } catch (reason) {
    error.value = failure(reason)
  }
}

onMounted(() => void initialize())
onUnmounted(() => {
  disposed = true
  clearTimeout(timer)
})
</script>

<template>
  <main class="qq-personal-page">
    <header>
      <div>
        <p class="eyebrow">ONEBOT 11 · 个人 QQ</p>
        <h1>QQ 个人号桥接</h1>
        <p class="description">NapCat 主动连接 Amitia，消息通过统一渠道管线进入角色对话。</p>
      </div>
      <span class="badge" :class="{ online: connected }">{{ ready ? label : "正在加载" }}</span>
    </header>

    <div v-if="error" class="message error">{{ error }}</div>
    <div v-if="notice" class="message success">{{ notice }}</div>

    <section class="grid">
      <article class="panel">
        <h2>桥接配置</h2>
        <label>
          <span>OneBot Token</span>
          <div class="copy-row">
            <input v-model="form.token" type="password" autocomplete="new-password" />
            <button type="button" @click="copy(form.token, 'onebot')">复制</button>
          </div>
        </label>
        <label>
          <span>群聊触发方式</span>
          <select v-model="form.groupTrigger">
            <option value="mention">仅 @机器人</option>
            <option value="all">所有群消息</option>
            <option value="prefix">指定前缀</option>
          </select>
        </label>
        <label v-if="form.groupTrigger === 'prefix'">
          <span>群聊触发前缀</span>
          <input v-model="form.triggerPrefix" />
        </label>
        <div class="actions">
          <button class="primary" type="button" :disabled="!!busy" @click="connect">{{ busy === "connect" ? "启用中…" : "保存并启用" }}</button>
          <button type="button" :disabled="!!busy" @click="refresh()">{{ busy === "refresh" ? "刷新中…" : "刷新" }}</button>
          <button class="danger" type="button" :disabled="!!busy || !config.enabled" @click="disconnect">{{ busy === "disconnect" ? "断开中…" : "断开" }}</button>
        </div>
      </article>

      <article class="panel">
        <h2>NapCat 反向 WebSocket</h2>
        <p class="hint">NapCat 网络配置中选择“WebSockets 客户端”，启用后填写：</p>
        <div class="endpoint">{{ config.reverseWebSocketUrl }}</div>
        <button type="button" @click="copy(config.reverseWebSocketUrl, 'url')">复制地址</button>
        <p class="hint">Token 使用左侧 OneBot Token。连接成功后状态会变为“已连接”。</p>
        <div class="token-section">
          <h3>NapCat WebUI Token</h3>
          <p class="hint">用于登录 NapCat WebUI，不是 OneBot 反向 WebSocket Token。</p>
          <div v-if="config.webuiToken" class="copy-row">
            <input
              :type="showWebuiToken ? 'text' : 'password'"
              :value="config.webuiToken"
              readonly
              autocomplete="off"
            />
            <button type="button" @click="showWebuiToken = !showWebuiToken">{{ showWebuiToken ? "隐藏" : "显示" }}</button>
            <button type="button" @click="copy(config.webuiToken, 'webui')">复制</button>
          </div>
          <p v-else class="message error webui-token-error">{{ config.webuiError || "未检测到 NapCat WebUI Token" }}</p>
        </div>
      </article>
    </section>

    <section class="panel status-panel">
      <h2>运行状态</h2>
      <dl>
        <div><dt>QQ 号</dt><dd>{{ detail.accountId || "未连接" }}</dd></div>
        <div><dt>OneBot 客户端</dt><dd>{{ detail.clients ?? 0 }}</dd></div>
        <div><dt>接收消息</dt><dd>{{ detail.messageCount ?? 0 }}</dd></div>
        <div><dt>发送回复</dt><dd>{{ detail.replyCount ?? 0 }}</dd></div>
        <div><dt>最近活动</dt><dd>{{ detail.lastSeenAt ? new Date(detail.lastSeenAt).toLocaleString() : "暂无" }}</dd></div>
      </dl>
      <p v-if="detail.lastError" class="message error">{{ detail.lastError }}</p>
    </section>
  </main>
</template>
