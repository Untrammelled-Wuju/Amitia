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
const config = ref({
  token: "",
  reverseWebSocketUrl: "",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  webuiPort: 5099,
  novncPort: 6081,
  containerName: "snowluma",
  dockerBin: "",
  webuiUrl: "",
  novncUrl: "",
  webuiUser: "admin",
  webuiPassword: "",
  webuiPasswordState: "unavailable",
  novncPassword: "",
  snowlumaError: "",
})
const form = ref({
  token: "",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  webuiPort: 5099,
  novncPort: 6081,
  containerName: "snowluma",
  dockerBin: "",
})
const ready = ref(false)
const showWebuiPassword = ref(false)
const showNovncPassword = ref(false)
const busy = ref("")
const error = ref("")
const notice = ref("")
const passwordPanel = ref("")
const passwordForm = ref({ oldPassword: "", newPassword: "", confirmPassword: "" })
const recordPassword = ref("")
let invoke
let disposed = false
let timer

const connected = computed(() => detail.value.qqOnline === true || detail.value.status === "online")
const label = computed(() => statusLabel(connected.value ? "online" : detail.value.status))
const passwordStateLabel = computed(() => ({
  stored: "已记录",
  temporary: "临时密码",
  changed: "未记录",
  unavailable: "未读取到",
})[config.value.webuiPasswordState] || "未记录")
const passwordRules = computed(() => {
  const value = String(passwordForm.value.newPassword || "")
  return [
    { id: "length", label: "长度不少于 10 位", ok: value.length >= 10 },
    { id: "lower", label: "包含小写字母", ok: /[a-z]/.test(value) },
    { id: "upper", label: "包含大写字母", ok: /[A-Z]/.test(value) },
    { id: "special", label: "包含特殊符号 (!@#$%…)", ok: /[^A-Za-z0-9\s]/.test(value) },
    { id: "no-space", label: "不包含空格", ok: value.length > 0 && !/\s/.test(value) },
  ]
})
const passwordRulesPassed = computed(() => passwordRules.value.every((rule) => rule.ok))
const webuiPasswordText = computed(() => {
  if (config.value.webuiPassword) return config.value.webuiPassword
  if (config.value.webuiPasswordState === "changed") return "已改密，请录入当前密码"
  return config.value.snowlumaError || "未读取到 WebUI 密码"
})
const novncPasswordText = computed(() => config.value.novncPassword || config.value.snowlumaError || "未读取到 noVNC 密码")
const webuiMissing = computed(() => !config.value.webuiPassword)
const novncMissing = computed(() => !config.value.novncPassword)

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
  config.value = { ...config.value, ...(next || {}) }
  form.value = {
    token: String(next?.token || ""),
    groupTrigger: String(next?.groupTrigger || "mention"),
    triggerPrefix: String(next?.triggerPrefix || "Amitia"),
    webuiPort: Number(next?.webuiPort || 5099),
    novncPort: Number(next?.novncPort || 6081),
    containerName: String(next?.containerName || "snowluma"),
    dockerBin: String(next?.dockerBin || ""),
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

async function reconnectConfig() {
  if (!invoke) return
  try {
    applyConfig(await invoke("config"))
  } catch (reason) {
    error.value = failure(reason)
  }
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
    notice.value = "已保存，SnowLuma 会通过反向 WebSocket 连接本插件"
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

function togglePasswordPanel(panel) {
  passwordPanel.value = passwordPanel.value === panel ? "" : panel
  if (passwordPanel.value === "change") {
    passwordForm.value = {
      oldPassword: String(config.value.webuiPassword || ""),
      newPassword: "",
      confirmPassword: "",
    }
  }
  if (passwordPanel.value === "record") {
    recordPassword.value = ""
  }
}

async function changePassword() {
  if (!invoke || busy.value) return
  const oldPassword = String(passwordForm.value.oldPassword || "").trim()
  const newPassword = String(passwordForm.value.newPassword || "")
  if (!oldPassword) {
    error.value = "请填写当前密码"
    return
  }
  if (!passwordRulesPassed.value) {
    error.value = "新密码不符合 SnowLuma 密码规则"
    return
  }
  if (newPassword !== passwordForm.value.confirmPassword) {
    error.value = "两次输入的新密码不一致"
    return
  }
  busy.value = "change-password"
  error.value = ""
  notice.value = ""
  try {
    const next = await invoke("change-webui-password", {
      action: "change-webui-password",
      input: { oldPassword, newPassword },
    })
    applyConfig(next)
    passwordForm.value = { oldPassword: "", newPassword: "", confirmPassword: "" }
    passwordPanel.value = ""
    showWebuiPassword.value = true
    notice.value = "WebUI 密码已更新，并已加密记录在插件里"
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

async function saveRecordedPassword() {
  if (!invoke || busy.value) return
  const password = String(recordPassword.value || "").trim()
  if (!password) {
    error.value = "请填写当前 WebUI 密码"
    return
  }
  busy.value = "record-password"
  error.value = ""
  notice.value = ""
  try {
    const next = await invoke("record-webui-password", {
      action: "record-webui-password",
      input: { password },
    })
    applyConfig(next)
    recordPassword.value = ""
    passwordPanel.value = ""
    notice.value = "密码已加密记录"
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

async function clearRecordedPassword() {
  if (!invoke || busy.value) return
  if (!window.confirm("确定清除已记录的 WebUI 密码？清除后将无法再显示和复制。")) return
  busy.value = "clear-password"
  error.value = ""
  notice.value = ""
  try {
    const next = await invoke("clear-webui-password", { action: "clear-webui-password" })
    applyConfig(next)
    notice.value = "已清除记录的密码"
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
  if (!text) {
    error.value = "没有可复制的内容"
    return
  }
  try {
    try {
      copyWithSelection(text)
    } catch {
      await invoke("copy-text", { text })
    }
    const labels = {
      onebot: "OneBot Token",
      url: "反向 WebSocket 地址",
      "webui-password": "SnowLuma WebUI 密码",
      "novnc-password": "noVNC 密码",
      "webui-url": "SnowLuma WebUI 地址",
      "novnc-url": "noVNC 地址",
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
        <p class="description">SnowLuma 主动连接 Amitia，消息通过统一渠道管线进入角色对话。</p>
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
        <div class="field-row">
          <label>
            <span>SnowLuma WebUI 端口</span>
            <input v-model.number="form.webuiPort" type="number" min="1" max="65535" />
          </label>
          <label>
            <span>noVNC 端口</span>
            <input v-model.number="form.novncPort" type="number" min="1" max="65535" />
          </label>
        </div>
        <div class="field-row">
          <label>
            <span>SnowLuma 容器名</span>
            <input v-model="form.containerName" placeholder="snowluma" />
          </label>
          <label>
            <span>docker 路径（可留空）</span>
            <input v-model="form.dockerBin" placeholder="自动探测" />
          </label>
        </div>
        <div class="actions">
          <button class="primary" type="button" :disabled="!!busy" @click="connect">{{ busy === "connect" ? "启用中…" : "保存并启用" }}</button>
          <button type="button" :disabled="!!busy" @click="refresh()">{{ busy === "refresh" ? "刷新中…" : "刷新" }}</button>
          <button class="danger" type="button" :disabled="!!busy || !config.enabled" @click="disconnect">{{ busy === "disconnect" ? "断开中…" : "断开" }}</button>
        </div>
      </article>

      <article class="panel">
        <h2>SnowLuma 反向 WebSocket</h2>
        <p class="hint">在 SnowLuma 网络配置中新建 WebSocket 客户端，填写下面的地址与 Token：</p>
        <div class="endpoint">{{ config.reverseWebSocketUrl }}</div>
        <button type="button" @click="copy(config.reverseWebSocketUrl, 'url')">复制地址</button>
        <p class="hint">Token 使用左侧 OneBot Token。连接成功后状态会变为“已连接”。</p>
        <div class="token-section">
          <h3>运行环境</h3>
          <p class="hint">以下账号密码由插件读取 SnowLuma 容器得到，修改密码后需要重新登录一次才能刷新。</p>
          <button type="button" :disabled="!!busy" @click="reconnectConfig">重新读取</button>
        </div>
      </article>
    </section>

    <section class="grid env-grid">
      <article class="panel">
        <h2>SnowLuma WebUI</h2>
        <a class="env-link" :href="config.webuiUrl" target="_blank" rel="noreferrer">{{ config.webuiUrl }}</a>
        <div class="copy-row env-copy">
          <button type="button" @click="copy(config.webuiUrl, 'webui-url')">复制网址</button>
        </div>
        <dl class="env-dl">
          <div>
            <dt>账号</dt>
            <dd>{{ config.webuiUser || "admin" }}</dd>
          </div>
          <div>
            <dt>密码</dt>
            <dd class="secret">
              <span v-if="!webuiMissing">{{ showWebuiPassword ? webuiPasswordText : "••••••••" }}</span>
              <span v-else class="secret-missing">{{ webuiPasswordText }}</span>
            </dd>
          </div>
        </dl>
        <div class="copy-row">
          <button v-if="!webuiMissing" type="button" @click="showWebuiPassword = !showWebuiPassword">{{ showWebuiPassword ? "隐藏密码" : "显示密码" }}</button>
          <button v-if="!webuiMissing" type="button" @click="copy(config.webuiPassword, 'webui-password')">复制密码</button>
          <span class="password-state" :data-state="config.webuiPasswordState">{{ passwordStateLabel }}</span>
        </div>
        <div class="copy-row env-actions">
          <button type="button" :disabled="!!busy" @click="togglePasswordPanel('change')">修改密码</button>
          <button type="button" :disabled="!!busy" @click="togglePasswordPanel('record')">记录当前密码</button>
          <button v-if="config.webuiPasswordState === 'stored'" class="danger" type="button" :disabled="!!busy" @click="clearRecordedPassword">清除记录</button>
        </div>

        <div v-if="passwordPanel === 'change'" class="password-form">
          <label>
            <span>当前密码</span>
            <input v-model="passwordForm.oldPassword" type="password" autocomplete="off" />
          </label>
          <label>
            <span>新密码</span>
            <input v-model="passwordForm.newPassword" type="password" autocomplete="new-password" />
          </label>
          <label>
            <span>确认新密码</span>
            <input v-model="passwordForm.confirmPassword" type="password" autocomplete="new-password" />
          </label>
          <ul class="password-rules">
            <li v-for="rule in passwordRules" :key="rule.id" :class="{ ok: rule.ok }">{{ rule.label }}</li>
          </ul>
          <div class="copy-row">
            <button class="primary" type="button" :disabled="!!busy || !passwordRulesPassed" @click="changePassword">
              {{ busy === "change-password" ? "提交中…" : "提交修改" }}
            </button>
            <button type="button" :disabled="!!busy" @click="passwordPanel = ''">取消</button>
          </div>
          <p class="hint">改密由 SnowLuma 官方接口执行，成功后所有已登录的 WebUI 会话会退出，需要重新登录。新密码会加密记录在插件里。</p>
        </div>

        <div v-if="passwordPanel === 'record'" class="password-form">
          <label>
            <span>当前 WebUI 密码</span>
            <input v-model="recordPassword" type="password" autocomplete="off" />
          </label>
          <div class="copy-row">
            <button class="primary" type="button" :disabled="!!busy" @click="saveRecordedPassword">
              {{ busy === "record-password" ? "保存中…" : "保存记录" }}
            </button>
            <button type="button" :disabled="!!busy" @click="passwordPanel = ''">取消</button>
          </div>
          <p class="hint">如果你在 SnowLuma WebUI 里自己改过密码，在这里录入一次，之后就可以直接显示和复制。</p>
        </div>

        <p class="hint">首次启动的临时密码只在容器日志里出现一次，插件会自动读取最新值。</p>
      </article>

      <article class="panel">
        <h2>noVNC 远程桌面</h2>
        <a class="env-link" :href="config.novncUrl" target="_blank" rel="noreferrer">{{ config.novncUrl }}</a>
        <div class="copy-row env-copy">
          <button type="button" @click="copy(config.novncUrl, 'novnc-url')">复制网址</button>
        </div>
        <dl class="env-dl">
          <div>
            <dt>用途</dt>
            <dd>QQ 扫码登录</dd>
          </div>
          <div>
            <dt>密码</dt>
            <dd class="secret">
              <span v-if="!novncMissing">{{ showNovncPassword ? novncPasswordText : "••••••••" }}</span>
              <span v-else class="secret-missing">{{ novncPasswordText }}</span>
            </dd>
          </div>
        </dl>
        <div class="copy-row">
          <button v-if="!novncMissing" type="button" @click="showNovncPassword = !showNovncPassword">{{ showNovncPassword ? "隐藏密码" : "显示密码" }}</button>
          <button v-if="!novncMissing" type="button" @click="copy(config.novncPassword, 'novnc-password')">复制密码</button>
        </div>
        <p class="hint">容器重建后登录态仍在数据卷里，一般不需要重新扫码。</p>
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
