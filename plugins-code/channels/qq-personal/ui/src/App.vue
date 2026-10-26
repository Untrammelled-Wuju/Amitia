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
  reverseWebSocketWarning: "",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  platform: "",
  runtimeMode: "docker",
  snowlumaRoot: "",
  snowlumaInstall: {},
  webuiPort: 5099,
  novncPort: 6081,
  novncEnabled: false,
  containerName: "snowluma",
  dockerBin: "",
  webuiUrl: "",
  novncUrl: "",
  webuiUser: "admin",
  webuiPassword: "",
  webuiPasswordState: "unavailable",
  novncPassword: "",
  snowlumaError: "",
  runtimeStatus: {},
  runtimeLogLines: [],
})
const form = ref({
  token: "",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  runtimeMode: "docker",
  snowlumaRoot: "",
  webuiPort: 5099,
  novncPort: 6081,
  containerName: "snowluma",
  dockerBin: "",
})
const ready = ref(false)
const selectedModeId = ref("docker")
const showWebuiPassword = ref(false)
const showNovncPassword = ref(false)
const credentialsOpen = ref(false)
const busy = ref("")
const error = ref("")
const notice = ref("")
const passwordPanel = ref("")
const modePrompt = ref(null)
const passwordForm = ref({ oldPassword: "", newPassword: "", confirmPassword: "" })
const recordPassword = ref("")
let invoke
let disposed = false
let timer

const connected = computed(() => detail.value.qqOnline === true || detail.value.status === "online")
const label = computed(() => statusLabel(connected.value ? "online" : detail.value.status))
const platformLabel = computed(() => ({
  win32: "Windows",
  linux: "Linux",
  darwin: "macOS",
  android: "Android",
})[config.value.platform] || config.value.platform || "当前系统")
const nativeModeLabel = computed(() => ({
  win32: "Windows 原生",
  linux: "Linux 原生",
})[config.value.platform] || "本机原生")
const runtimeModeLabel = computed(() => config.value.runtimeMode === "native" ? nativeModeLabel.value : "Docker")
const runtimeModes = computed(() => {
  return [{
    id: "docker",
    mode: "docker",
    label: "Docker",
    available: true,
    unavailableTip: "",
  }, {
    id: "windows-native",
    mode: "native",
    label: "Windows 原生",
    available: config.value.platform === "win32",
    unavailableTip: "当前系统不支持 Windows 原生运行",
  }, {
    id: "linux-native",
    mode: "native",
    label: "Linux 原生",
    available: config.value.platform === "linux",
    unavailableTip: "当前系统不支持 Linux 原生运行",
  }]
})
const runtimeModeOptions = computed(() => runtimeModes.value)
const novncEnabled = computed(() => form.value.runtimeMode === "docker")
const nativeMode = computed(() => form.value.runtimeMode === "native")
const dockerMode = computed(() => form.value.runtimeMode === "docker")
const modePreviewUnsupported = computed(() => runtimeModes.value.find((mode) => mode.id === selectedModeId.value)?.available === false)
const connectionDetail = computed(() => {
  if (connected.value) return detail.value.accountId ? `QQ ${detail.value.accountId}` : "OneBot 已连接"
  if (modePreviewUnsupported.value) return "当前平台不可用，仅预览配置"
  return dockerMode.value ? "等待容器连接" : "等待原生进程连接"
})
const snowlumaInstall = computed(() => config.value.snowlumaInstall || {})
const snowlumaInstallRunning = computed(() => ["checking", "downloading", "verifying", "extracting", "validating", "migrating", "starting"].includes(String(snowlumaInstall.value.phase || "")))
const snowlumaInstalled = computed(() => snowlumaInstall.value.installed === true)
const snowlumaInstallPercent = computed(() => Math.max(0, Math.min(100, Number(snowlumaInstall.value.percent || 0))))
const snowlumaInstallLabel = computed(() => {
  if (snowlumaInstallRunning.value) return snowlumaInstall.value.message || "正在安装 SnowLuma"
  if (snowlumaInstall.value.phase === "failed") return snowlumaInstall.value.error || "SnowLuma 安装失败"
  if (snowlumaInstalled.value) return `SnowLuma ${snowlumaInstall.value.installedVersion} 已安装`
  return "SnowLuma 尚未安装"
})
const saveButtonLabel = computed(() => nativeMode.value ? "保存并启动" : "保存并连接")
const modePromptImpact = computed(() => modePrompt.value?.mode === "native"
  ? "将保存当前配置并启动插件托管的 SnowLuma 进程。"
  : "将保存当前配置并切换到 Docker 容器连接。")
const passwordStateLabel = computed(() => ({
  stored: "已记录",
  temporary: "临时密码",
  changed: "未记录",
  unavailable: "未读取到",
})[config.value.webuiPasswordState] || "未记录")
const passwordRules = computed(() => {
  const value = String(passwordForm.value.newPassword || "")
  return [
    { id: "length", label: "至少 10 位", ok: value.length >= 10 },
    { id: "lower", label: "小写字母", ok: /[a-z]/.test(value) },
    { id: "upper", label: "大写字母", ok: /[A-Z]/.test(value) },
    { id: "special", label: "特殊符号", ok: /[^A-Za-z0-9\s]/.test(value) },
    { id: "no-space", label: "不含空格", ok: value.length > 0 && !/\s/.test(value) },
  ]
})
const passwordRulesPassed = computed(() => passwordRules.value.every((rule) => rule.ok))
const webuiPasswordText = computed(() => {
  if (config.value.webuiPassword) return config.value.webuiPassword
  if (config.value.webuiPasswordState === "changed") return "已改密，请录入当前密码"
  return "未读取到"
})
const novncPasswordText = computed(() => config.value.novncPassword || "未读取到")
const webuiMissing = computed(() => !config.value.webuiPassword)
const novncMissing = computed(() => !config.value.novncPassword)
const diagnosticErrors = computed(() => {
  const entries = []
  if (config.value.snowlumaError) entries.push({ label: "SnowLuma", message: String(config.value.snowlumaError) })
  if (nativeMode.value && config.value.runtimeStatus?.error) entries.push({ label: "原生进程", message: String(config.value.runtimeStatus.error) })
  if (detail.value.lastError) entries.push({ label: "渠道连接", message: String(detail.value.lastError) })
  return entries.filter((entry, index) => entries.findIndex((item) => item.message === entry.message) === index)
})
const runtimeIssueSummary = computed(() => {
  const raw = diagnosticErrors.value.map((entry) => entry.message).join(" ")
  if (!raw) return ""
  if (/docker\.sock|docker api|docker daemon|failed to connect/i.test(raw)) {
    return "Docker 当前不可用，请启动 Docker Desktop 并检查访问权限"
  }
  if (/不支持当前平台|does not support current platform/i.test(raw)) {
    return "当前平台不支持 SnowLuma 原生运行"
  }
  return "SnowLuma 运行环境暂不可用"
})
const dockerStartRecommended = computed(() => dockerMode.value && !connected.value && !modePreviewUnsupported.value)

function failure(reason) {
  const message = reason instanceof Error ? reason.message : String(reason || "操作失败")
  if (/docker\.sock|docker api|docker daemon|failed to connect/i.test(message)) {
    return "Docker 当前不可用，请启动 Docker Desktop 并检查访问权限"
  }
  if (/不支持当前平台|does not support current platform/i.test(message)) {
    return "当前平台不支持 SnowLuma 原生运行"
  }
  if (/operation not permitted|failed to fire hook|unable to create a log file|LSOpenURLsWithCompletionHandler|EACCES|EPERM/i.test(message)) {
    return "Docker Desktop 启动权限受限，请检查系统权限"
  }
  if (/^Command failed:/i.test(message)) return "命令执行失败，请展开运行详情查看诊断"
  return message
}

function chooseRuntimeMode(mode) {
  if (busy.value || mode.id === selectedModeId.value) return
  error.value = ""
  notice.value = ""
  if (!mode.available) {
    selectedModeId.value = mode.id
    form.value.runtimeMode = mode.mode
    notice.value = mode.unavailableTip
    modePrompt.value = null
    return
  }
  if (mode.mode === config.value.runtimeMode) {
    selectedModeId.value = mode.id
    form.value.runtimeMode = mode.mode
    return
  }
  modePrompt.value = mode
}

function handleModeSelect(event) {
  const mode = runtimeModes.value.find((item) => item.id === event.target.value)
  if (mode) chooseRuntimeMode(mode)
}

function cancelModeSwitch() {
  if (busy.value === "mode-switch") return
  modePrompt.value = null
}

async function confirmModeSwitch() {
  if (!invoke || !modePrompt.value || busy.value) return
  const target = modePrompt.value
  busy.value = "mode-switch"
  error.value = ""
  notice.value = ""
  let switched = false
  try {
    const next = { ...form.value, runtimeMode: target.mode }
    await invoke("connect", { config: next })
    applyConfig(await invoke("config"))
    modePrompt.value = null
    notice.value = `已切换为${target.label}`
    switched = true
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
  if (switched) await refresh(true)
}

function handleDialogKeydown(event) {
  if (event.key !== "Escape") return
  if (passwordPanel.value) {
    event.preventDefault()
    passwordPanel.value = ""
    return
  }
  if (credentialsOpen.value) {
    event.preventDefault()
    closeCredentials()
    return
  }
  if (modePrompt.value) {
    event.preventDefault()
    cancelModeSwitch()
  }
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
    runtimeMode: String(next?.runtimeMode || "docker"),
    snowlumaRoot: String(next?.snowlumaRoot || ""),
    webuiPort: Number(next?.webuiPort || 5099),
    novncPort: Number(next?.novncPort || 6081),
    containerName: String(next?.containerName || "snowluma"),
    dockerBin: String(next?.dockerBin || ""),
  }
  selectedModeId.value = form.value.runtimeMode === "native" ? (config.value.platform === "linux" ? "linux-native" : "windows-native") : "docker"
}

async function refresh(silent = false) {
  if (!invoke || disposed || busy.value) return
  if (!silent) busy.value = "refresh"
  try {
    const [nextStatus, nextConfig] = silent
      ? [await invoke("status"), null]
      : await Promise.all([invoke("status"), invoke("config")])
    detail.value = nextStatus
    if (detail.value.snowluma) {
      config.value.runtimeStatus = detail.value.snowluma
    }
    if (detail.value.snowlumaInstall) {
      config.value.snowlumaInstall = detail.value.snowlumaInstall
    }
    if (nextConfig) applyConfig(nextConfig)
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
    notice.value = nativeMode.value ? "配置已保存，SnowLuma 已启动" : "配置已保存，正在连接 Docker 容器"
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

async function installSnowLuma() {
  if (!invoke || busy.value || snowlumaInstallRunning.value) return
  busy.value = "install-snowluma"
  error.value = ""
  notice.value = ""
  try {
    const next = await invoke("install-snowluma", {
      action: "install-snowluma",
      input: { config: { ...form.value, runtimeMode: "native" } },
    })
    applyConfig(next)
    notice.value = "SnowLuma 安装任务已启动"
    schedule()
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

function dockerDesktopPath() {
  if (config.value.platform === "darwin") return "/Applications/Docker.app"
  if (config.value.platform === "win32") return "C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe"
  return ""
}

async function openDockerDesktop() {
  const target = dockerDesktopPath()
  const desktop = window.amitiaDesktop
  if (!target || typeof desktop?.openPath !== "function") return false
  try {
    await desktop.openPath(target)
    return true
  } catch {
    return false
  }
}

async function startDocker() {
  if (!invoke || busy.value) return
  busy.value = "start-docker"
  error.value = ""
  notice.value = ""
  let started = false
  try {
    const needsDocker = runtimeIssueSummary.value.startsWith("Docker 当前不可用")
    const hostLaunched = needsDocker ? await openDockerDesktop() : false
    const next = await invoke("start-docker", {
      action: "start-docker",
      input: { skipDockerLaunch: hostLaunched },
    })
    applyConfig(next)
    notice.value = next.dockerService?.message || "Docker 已启动"
    started = true
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
  if (started) await refresh(true)
}

async function disconnect() {
  if (!invoke || busy.value) return
  busy.value = "disconnect"
  error.value = ""
  try {
    await invoke("disconnect")
    await refresh(true)
    notice.value = "连接已断开"
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

function openCredentials() {
  error.value = ""
  notice.value = ""
  passwordPanel.value = ""
  credentialsOpen.value = true
}

function closeCredentials() {
  if (busy.value) return
  credentialsOpen.value = false
  passwordPanel.value = ""
  showWebuiPassword.value = false
  showNovncPassword.value = false
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
    notice.value = "WebUI 密码已更新"
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
    notice.value = "密码已记录"
  } catch (reason) {
    error.value = failure(reason)
  } finally {
    busy.value = ""
  }
}

async function clearRecordedPassword() {
  if (!invoke || busy.value) return
  if (!window.confirm("确定清除已记录的 WebUI 密码？")) return
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
      "webui-password": "WebUI 密码",
      "novnc-password": "noVNC 密码",
      "webui-url": "WebUI 地址",
      "novnc-url": "noVNC 地址",
    }
    notice.value = `${labels[type] || "内容"}已复制`
  } catch (reason) {
    error.value = "复制失败，请手动选择文本"
    if (String(failure(reason)).trim()) console.warn("[qq-personal] copy failed", failure(reason))
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

onMounted(() => {
  window.addEventListener("keydown", handleDialogKeydown)
  void initialize()
})
onUnmounted(() => {
  disposed = true
  clearTimeout(timer)
  window.removeEventListener("keydown", handleDialogKeydown)
})
</script>

<template>
  <main class="qq-personal-page">
    <div v-if="!ready" class="loading-state">正在读取连接配置…</div>
    <template v-else>
      <header class="page-toolbar">
        <div class="connection-state">
          <span class="status-dot" :class="{ online: connected }"></span>
          <div>
            <strong>{{ label }}</strong>
            <span>{{ connectionDetail }}</span>
          </div>
        </div>
        <div class="toolbar-actions">
          <label class="mode-select">
            <span>运行方式</span>
            <select :value="selectedModeId" :disabled="!!busy" @change="handleModeSelect">
              <option v-for="mode in runtimeModeOptions" :key="mode.id" :value="mode.id">
                {{ mode.label }}{{ mode.available ? "" : "（预览）" }}
              </option>
            </select>
          </label>
          <button class="quiet" type="button" :disabled="!!busy" @click="refresh()">
            {{ busy === "refresh" ? "刷新中…" : "刷新" }}
          </button>
        </div>
      </header>

      <div v-if="error && !credentialsOpen && !modePrompt" class="message error">{{ error }}</div>
      <div v-if="notice && !credentialsOpen && !modePrompt" class="message success">{{ notice }}</div>
      <div v-if="runtimeIssueSummary && !connected" class="runtime-callout">
        <span>{{ runtimeIssueSummary }}</span>
        <button v-if="dockerStartRecommended" class="primary compact" type="button" :disabled="!!busy" @click="startDocker">
          {{ busy === "start-docker" ? "正在启动…" : "启动 Docker 与 SnowLuma" }}
        </button>
      </div>

      <section class="metrics-strip" aria-label="运行状态">
        <div><span>QQ 号</span><strong>{{ detail.accountId || "未连接" }}</strong></div>
        <div><span>客户端</span><strong>{{ detail.clients ?? 0 }}</strong></div>
        <div><span>接收</span><strong>{{ detail.messageCount ?? 0 }}</strong></div>
        <div><span>回复</span><strong>{{ detail.replyCount ?? 0 }}</strong></div>
      </section>

      <section class="config-surface">
        <div class="section-heading">
          <h2>连接设置</h2>
        </div>

        <div v-if="nativeMode" class="install-strip">
          <div class="install-copy">
            <strong>SnowLuma 运行组件</strong>
            <span>{{ snowlumaInstallLabel }}</span>
          </div>
          <button class="primary compact" type="button" :disabled="!!busy || snowlumaInstallRunning || modePreviewUnsupported" @click="installSnowLuma">
            {{ snowlumaInstallRunning ? "安装中…" : snowlumaInstalled ? "重新安装" : "安装 SnowLuma" }}
          </button>
          <div v-if="snowlumaInstallRunning || snowlumaInstall.phase === 'failed'" class="install-progress" :data-state="snowlumaInstall.phase">
            <div class="install-progress-track" role="progressbar" :aria-valuenow="snowlumaInstallPercent" aria-valuemin="0" aria-valuemax="100">
              <span :style="{ width: `${snowlumaInstallPercent}%` }"></span>
            </div>
            <small>{{ snowlumaInstallPercent }}%</small>
          </div>
          <p class="install-note">只安装 SnowLuma，使用项目内置 Node；QQ 和图形桌面依赖由系统准备。</p>
        </div>

        <div class="settings-block">
          <div v-if="dockerMode" class="settings-grid settings-grid--three">
            <label>
              <span>容器名</span>
              <input v-model="form.containerName" placeholder="snowluma" />
            </label>
            <label>
              <span>WebUI 端口</span>
              <input v-model.number="form.webuiPort" type="number" min="1" max="65535" />
            </label>
            <label>
              <span>noVNC 端口</span>
              <input v-model.number="form.novncPort" type="number" min="1" max="65535" />
            </label>
          </div>
          <div v-else class="settings-grid">
            <label>
              <span>WebUI 端口</span>
              <input v-model.number="form.webuiPort" type="number" min="1" max="65535" />
            </label>
          </div>
          <details v-if="dockerMode" class="advanced-settings">
            <summary>高级设置</summary>
            <label>
              <span>Docker 可执行文件</span>
              <input v-model="form.dockerBin" placeholder="自动探测" />
            </label>
          </details>
        </div>

        <div class="settings-block">
          <h3>OneBot 接入</h3>
          <label>
            <span>Token</span>
            <div class="inline-field">
              <input v-model="form.token" type="password" autocomplete="new-password" />
              <button class="compact" type="button" @click="copy(form.token, 'onebot')">复制</button>
            </div>
          </label>
          <div class="settings-grid">
            <label>
              <span>群聊触发</span>
              <select v-model="form.groupTrigger">
                <option value="mention">仅 @机器人</option>
                <option value="all">所有群消息</option>
                <option value="prefix">指定前缀</option>
              </select>
            </label>
            <label v-if="form.groupTrigger === 'prefix'">
              <span>触发前缀</span>
              <input v-model="form.triggerPrefix" />
            </label>
          </div>
        </div>

        <div class="form-actions">
          <button class="primary" type="button" :disabled="!!busy || modePreviewUnsupported" @click="connect">
            {{ busy === "connect" ? "保存中…" : saveButtonLabel }}
          </button>
          <button v-if="config.enabled || connected" class="danger quiet" type="button" :disabled="!!busy" @click="disconnect">
            {{ busy === "disconnect" ? "断开中…" : "断开连接" }}
          </button>
        </div>
      </section>

      <section class="access-section">
        <div class="section-heading section-heading--compact">
          <h2>访问入口</h2>
        </div>
        <div class="access-grid" :data-card-count="novncEnabled ? 3 : 2">
          <article class="access-card">
            <h3>反向 WebSocket</h3>
            <code class="endpoint">{{ config.reverseWebSocketUrl }}</code>
            <div class="card-actions">
              <button class="compact" type="button" @click="copy(config.reverseWebSocketUrl, 'url')">复制地址</button>
            </div>
            <p v-if="config.reverseWebSocketWarning" class="inline-warning">{{ config.reverseWebSocketWarning }}</p>
          </article>

          <article class="access-card">
            <h3>SnowLuma WebUI</h3>
            <a class="url-link" :href="config.webuiUrl" target="_blank" rel="noreferrer">{{ config.webuiUrl }}</a>
            <div class="card-actions">
              <a class="button-link" :href="config.webuiUrl" target="_blank" rel="noreferrer">打开</a>
              <button class="compact" type="button" @click="copy(config.webuiUrl, 'webui-url')">复制</button>
              <button class="compact" type="button" @click="openCredentials">凭据</button>
            </div>
          </article>

          <article v-if="novncEnabled" class="access-card">
            <h3>noVNC 远程桌面</h3>
            <a class="url-link" :href="config.novncUrl" target="_blank" rel="noreferrer">{{ config.novncUrl }}</a>
            <div class="card-actions">
              <a class="button-link" :href="config.novncUrl" target="_blank" rel="noreferrer">打开</a>
              <button class="compact" type="button" @click="copy(config.novncUrl, 'novnc-url')">复制</button>
            </div>
          </article>
        </div>
      </section>

      <details class="diagnostic-details">
        <summary>运行详情</summary>
        <div class="diagnostic-meta">
          <span>平台 {{ platformLabel }}</span>
          <span>方式 {{ runtimeModeLabel }}</span>
          <span>进程 {{ config.runtimeStatus?.running ? `运行中${config.runtimeStatus.pid ? ` · PID ${config.runtimeStatus.pid}` : ""}` : "未运行" }}</span>
          <span>最近活动 {{ detail.lastSeenAt ? new Date(detail.lastSeenAt).toLocaleString() : "暂无" }}</span>
        </div>
        <div v-if="diagnosticErrors.length" class="diagnostic-errors">
          <div v-for="entry in diagnosticErrors" :key="entry.label">
            <strong>{{ entry.label }}</strong>
            <pre>{{ entry.message }}</pre>
          </div>
        </div>
      </details>
    </template>

    <div v-if="credentialsOpen" class="dialog-backdrop" @click.self="closeCredentials">
      <section class="dialog credentials-dialog" role="dialog" aria-modal="true" aria-labelledby="credentials-title">
        <div class="dialog-heading">
          <div>
            <h2 id="credentials-title">SnowLuma 凭据</h2>
            <p>{{ dockerMode ? "账号密码从正在运行的容器读取" : "账号密码由插件从本地运行环境读取" }}</p>
          </div>
          <button class="icon-button" type="button" aria-label="关闭" :disabled="!!busy" @click="closeCredentials">×</button>
        </div>
        <div v-if="error" class="message error">{{ error }}</div>
        <div v-if="notice" class="message success">{{ notice }}</div>

        <div class="credential-list">
          <div class="credential-row">
            <span>WebUI 账号</span>
            <strong>{{ config.webuiUser || "admin" }}</strong>
          </div>
          <div class="credential-row">
            <div>
              <span>WebUI 密码</span>
              <strong class="secret" :class="{ missing: webuiMissing }">{{ webuiMissing || showWebuiPassword ? webuiPasswordText : "••••••••" }}</strong>
            </div>
            <div class="credential-actions">
              <span class="password-state" :data-state="config.webuiPasswordState">{{ passwordStateLabel }}</span>
              <button v-if="!webuiMissing" class="compact" type="button" @click="showWebuiPassword = !showWebuiPassword">{{ showWebuiPassword ? "隐藏" : "显示" }}</button>
              <button v-if="!webuiMissing" class="compact" type="button" @click="copy(config.webuiPassword, 'webui-password')">复制</button>
            </div>
          </div>
          <div v-if="novncEnabled" class="credential-row">
            <div>
              <span>noVNC 密码</span>
              <strong class="secret" :class="{ missing: novncMissing }">{{ novncMissing || showNovncPassword ? novncPasswordText : "••••••••" }}</strong>
            </div>
            <div class="credential-actions">
              <button v-if="!novncMissing" class="compact" type="button" @click="showNovncPassword = !showNovncPassword">{{ showNovncPassword ? "隐藏" : "显示" }}</button>
              <button v-if="!novncMissing" class="compact" type="button" @click="copy(config.novncPassword, 'novnc-password')">复制</button>
            </div>
          </div>
        </div>

        <div class="dialog-actions">
          <button class="compact" type="button" :disabled="!!busy" @click="reconnectConfig">重新读取</button>
          <button class="compact" type="button" :disabled="!!busy" @click="togglePasswordPanel('record')">录入当前密码</button>
          <button class="compact" type="button" :disabled="!!busy" @click="togglePasswordPanel('change')">修改密码</button>
          <button v-if="config.webuiPasswordState === 'stored'" class="compact danger quiet" type="button" :disabled="!!busy" @click="clearRecordedPassword">清除记录</button>
        </div>

        <div v-if="passwordPanel === 'record'" class="password-form">
          <label>
            <span>当前 WebUI 密码</span>
            <input v-model="recordPassword" type="password" autocomplete="off" />
          </label>
          <div class="dialog-actions">
            <button type="button" :disabled="!!busy" @click="passwordPanel = ''">取消</button>
            <button class="primary" type="button" :disabled="!!busy" @click="saveRecordedPassword">
              {{ busy === "record-password" ? "保存中…" : "保存" }}
            </button>
          </div>
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
          <div class="dialog-actions">
            <button type="button" :disabled="!!busy" @click="passwordPanel = ''">取消</button>
            <button class="primary" type="button" :disabled="!!busy || !passwordRulesPassed" @click="changePassword">
              {{ busy === "change-password" ? "提交中…" : "修改密码" }}
            </button>
          </div>
        </div>
      </section>
    </div>

    <div v-if="modePrompt" class="dialog-backdrop" @click.self="cancelModeSwitch">
      <section class="dialog" role="dialog" aria-modal="true" aria-labelledby="mode-dialog-title">
        <div class="dialog-heading">
          <div>
            <h2 id="mode-dialog-title">切换运行方式</h2>
            <p>{{ runtimeModeLabel }} → {{ modePrompt.label }}</p>
          </div>
          <button class="icon-button" type="button" aria-label="关闭" :disabled="busy === 'mode-switch'" @click="cancelModeSwitch">×</button>
        </div>
        <p>{{ modePromptImpact }}</p>
        <div v-if="error" class="message error">{{ error }}</div>
        <div class="dialog-actions">
          <button type="button" :disabled="busy === 'mode-switch'" @click="cancelModeSwitch">取消</button>
          <button class="primary" type="button" :disabled="busy === 'mode-switch'" @click="confirmModeSwitch">
            {{ busy === "mode-switch" ? "保存中…" : "确认切换" }}
          </button>
        </div>
      </section>
    </div>
  </main>
</template>
