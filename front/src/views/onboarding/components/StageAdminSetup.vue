<template>
  <div class="ob-stage-inner ob-setup-stage-inner">
    <div
      class="ob-boot-panel ob-setup-step-panel"
      :class="panelClass('environment')"
    >
      <h2 class="ob-boot-title">
        {{ deployMode === 'local' ? '正在检查运行环境' : '正在检查远程服务' }}
      </h2>
      <div class="ob-boot-copy">
        {{ deployMode === 'local'
          ? '系统将检查后端服务、侧车服务以及各数据库组件的启动状态。'
          : '本设备不会启动本地服务，只会验证远程地址、服务版本和接口兼容性。' }}
      </div>
      <div class="ob-boot-list">
        <div
          v-for="(row, idx) in bootRows"
          :key="idx"
          class="ob-boot-row"
          :class="row.state"
        >
          <span class="ob-boot-dot"></span>
          <span class="ob-boot-name">{{ row.name }}</span>
          <span class="ob-boot-state">{{ row.stateText }}</span>
        </div>
      </div>
    </div>

    <div
      class="ob-account-sheet ob-setup-step-panel"
      :class="panelClass('account')"
    >
      <div class="kicker">{{ isLogin ? '账号登录' : '账号注册' }}</div>
      <div class="ob-sheet-title">
        {{ deployMode === 'remote'
          ? '登录远程管理账号'
          : (isLogin ? '登录管理账号' : '注册管理账号') }}
      </div>
      <div class="ob-account-sheet-desc">
        {{ deployMode === 'remote'
          ? '使用远程 Amitia 服务已有的管理账号登录。本设备不会创建新的本地账号。'
          : (isLogin
            ? '管理账号已经创建。请输入管理员名称和密码，登录后继续设置。'
            : '用于进入管理与设置页面，并保护配置、聊天记录和记忆数据。') }}
      </div>
      <div class="ob-form-stack">
        <label class="ob-input-label">
          账号
          <input v-model="username" autocomplete="username" placeholder="请输入账号" />
        </label>
        <label class="ob-input-label">
          密码
          <input v-model="password" type="password" autocomplete="current-password" placeholder="请输入密码" />
        </label>
        <label v-if="!isLogin" class="ob-input-label">
          确认密码
          <input v-model="password2" type="password" autocomplete="new-password" placeholder="再次输入密码" />
        </label>
      </div>
      <div class="ob-error">{{ errorMsg }}</div>
      <button class="ob-stage-action ob-setup-inline-action" @click="handleSubmit">
        {{ deployMode === 'remote' ? '登录远程服务并继续'
          : (isLogin ? '登录并继续' : '注册账号并继续') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from "vue"
import { getApiBaseURL } from "@/runtime/runtime-adapter"

const props = defineProps<{
  deployMode: string
  step: string
  isLogin: boolean
  accountName: string
  serverURL: string
}>()

const emit = defineEmits<{
  'update:step': [value: string]
  healthCheckDone: []
  submit: [data: { username: string; password: string; password2: string; isLogin: boolean; deployMode: string }]
}>()

const username = ref(props.accountName || "")
const password = ref("")
const password2 = ref("")
const errorMsg = ref("")

const bootRows = ref([
  { name: "检查后端服务", state: "", stateText: "等待" },
  { name: "检查微信侧车", state: "", stateText: "等待" },
  { name: "检查QQ侧车", state: "", stateText: "等待" },
  { name: "检查Qdrant", state: "", stateText: "等待" },
  { name: "检查SurrealDB", state: "", stateText: "等待" },
])

let abortController: AbortController | null = null
let flowTransitionToken = 0

const visibleStep = ref(props.step)
const panelPhase = ref<Record<string, string>>({})

function panelClass(panel: string) {
  const phase = panelPhase.value[panel] || ""
  if (phase) return phase
  if (visibleStep.value !== panel) return "setup-panel-entering"
  return ""
}

function switchSetupPanel(step: string) {
  if (visibleStep.value === step && !panelPhase.value[step]) return

  const token = ++flowTransitionToken
  const outgoing = visibleStep.value
  const incoming = step

  if (outgoing !== incoming) {
    panelPhase.value = { [outgoing]: "setup-panel-leaving" }
  }

  setTimeout(() => {
    if (token !== flowTransitionToken) return

    panelPhase.value = {}
    visibleStep.value = incoming

    nextTick(() => {
      if (token !== flowTransitionToken) return
      panelPhase.value = { [incoming]: "setup-panel-entering" }

      requestAnimationFrame(() => {
        if (token !== flowTransitionToken) return
        panelPhase.value = {}

        if (incoming === "environment") {
          updateBootLabels()
          startRealChecks()
        }
      })
    })
  }, 320)
}

watch(() => props.deployMode, () => {
  if (props.step === "environment") {
    updateBootLabels()
  }
})

onMounted(() => {
  visibleStep.value = props.step
  if (props.step === "environment") {
    updateBootLabels()
    startRealChecks()
  }
})

onUnmounted(() => {
  if (abortController) {
    abortController.abort()
    abortController = null
  }
})

watch(() => props.step, (s) => {
  if (s !== visibleStep.value) {
    switchSetupPanel(s)
  }
})

function updateBootLabels() {
  const local = props.deployMode === "local"
  const names = local
    ? ["检查后端服务", "检查微信侧车", "检查QQ侧车", "检查Qdrant", "检查SurrealDB"]
    : ["检查服务地址", "验证服务版本", "确认接口兼容性", "建立远程连接"]
  bootRows.value = names.map((name) => ({ name, state: "", stateText: "等待" }))
}

async function startRealChecks() {
  if (abortController) {
    abortController.abort()
  }
  abortController = new AbortController()
  const signal = abortController.signal

  const rows = bootRows.value
  rows.forEach((row) => { row.state = ""; row.stateText = "等待" })

  const isLocal = props.deployMode === "local"

  try {
    if (isLocal) {
      await runLocalChecks(rows, signal)
    } else {
      await runRemoteChecks(rows, signal)
    }

    if (!signal.aborted) {
      const allDone = rows.every((r) => r.state === "done")
      if (allDone) {
        setTimeout(() => {
          if (!signal.aborted) {
            emit("healthCheckDone")
          }
        }, 360)
      }
    }
  } catch (e: any) {
    if (signal.aborted) return
    const failedRow = rows.find((r) => r.state === "running")
    if (failedRow) {
      failedRow.state = "error"
      failedRow.stateText = "失败"
    }
  }
}

async function fetchWithTimeout(url: string, signal: AbortSignal, timeoutMs = 10000): Promise<Response> {
  const controller = new AbortController()
  const linkedSignal = controller.signal

  signal.addEventListener("abort", () => controller.abort())

  const timeout = setTimeout(() => controller.abort(), timeoutMs)

  try {
    const res = await fetch(url, { signal: linkedSignal })
    return res
  } finally {
    clearTimeout(timeout)
  }
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new DOMException("Aborted", "AbortError")); return }
    const t = setTimeout(resolve, ms)
    signal.addEventListener("abort", () => { clearTimeout(t); reject(new DOMException("Aborted", "AbortError")) })
  })
}

async function runLocalChecks(rows: typeof bootRows.value, signal: AbortSignal) {
  const apiBase = await getApiBaseURL()

  let healthData: any = null
  let breakerData: any = null

  for (let i = 0; i < rows.length; i++) {
    if (signal.aborted) return
    const row = rows[i]

    if (i === 0) {
      row.state = "running"
      row.stateText = "处理中"

      try {
        const res = await fetchWithTimeout(`${apiBase}/api/health`, signal, 8000)
        if (res.ok) {
          healthData = await res.json()
          const ready = healthData?.data?.ready === true || healthData?.ready === true
          row.state = ready ? "done" : "error"
          row.stateText = ready ? "正常运行" : "异常"
        } else {
          row.state = "error"
          row.stateText = "异常"
        }
      } catch {
        row.state = "error"
        row.stateText = "无法连接"
      }
    } else if (i === 1) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"

      const hd = healthData?.data || healthData || {}
      const wechatRunning = hd.wechat_running
      row.state = wechatRunning === true ? "done" : "error"
      row.stateText = wechatRunning === true ? "正常运行" : "未启动"
    } else if (i === 2) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"

      const hd = healthData?.data || healthData || {}
      const qqRunning = hd.qq_running
      row.state = qqRunning === true ? "done" : "error"
      row.stateText = qqRunning === true ? "正常运行" : "未启动"
    } else if (i === 3) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"

      try {
        if (!breakerData) {
          const res = await fetchWithTimeout(`${apiBase}/api/health/circuit-breakers`, signal, 8000)
          if (res.ok) {
            const json = await res.json()
            breakerData = json?.data || json || []
          }
        }
        const qdrant = Array.isArray(breakerData) ? breakerData.find((b: any) => b.name === "qdrant") : null
        row.state = qdrant?.healthy ? "done" : "error"
        row.stateText = qdrant?.healthy ? "正常运行" : "未启动"
      } catch {
        row.state = "error"
        row.stateText = "无法连接"
      }
    } else if (i === 4) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"

      const surrealdb = Array.isArray(breakerData) ? breakerData.find((b: any) => b.name === "surrealdb") : null
      row.state = surrealdb?.healthy ? "done" : "error"
      row.stateText = surrealdb?.healthy ? "正常运行" : "未启动"
    }
  }
}

async function runRemoteChecks(rows: typeof bootRows.value, signal: AbortSignal) {
  const remoteURL = (props.serverURL || "").trim().replace(/\/+$/, "")

  for (let i = 0; i < rows.length; i++) {
    if (signal.aborted) return
    const row = rows[i]

    if (i === 0) {
      row.state = "running"
      row.stateText = "处理中"

      try {
        const res = await fetchWithTimeout(`${remoteURL}/api/health`, signal, 8000)
        if (res.ok) {
          row.state = "done"
          row.stateText = "已完成"
        } else {
          row.state = "error"
          row.stateText = "无法访问"
        }
      } catch {
        row.state = "error"
        row.stateText = "无法连接"
      }
    } else if (i === 1) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"

      try {
        const res = await fetchWithTimeout(`${remoteURL}/api/health`, signal, 8000)
        if (res.ok) {
          const data = await res.json()
          const hasVersion = !!(data?.data?.version || data?.version)
          row.state = hasVersion ? "done" : "error"
          row.stateText = hasVersion ? "已完成" : "版本未知"
        } else {
          row.state = "error"
          row.stateText = "异常"
        }
      } catch {
        row.state = "error"
        row.stateText = "无法连接"
      }
    } else if (i === 2) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"

      try {
        const res = await fetchWithTimeout(`${remoteURL}/api/runtime/health`, signal, 8000)
        if (res.ok) {
          row.state = "done"
          row.stateText = "已完成"
        } else {
          row.state = "error"
          row.stateText = "不兼容"
        }
      } catch {
        row.state = "error"
        row.stateText = "无法连接"
      }
    } else if (i === 3) {
      await delay(650, signal)
      if (signal.aborted) return

      row.state = "running"
      row.stateText = "处理中"
      row.state = "done"
      row.stateText = "已完成"
    }
  }
}

function handleSubmit() {
  const name = username.value.trim()
  const pw = password.value

  if (!name || !pw) {
    errorMsg.value = props.deployMode === "remote"
      ? "请输入远程管理账号和密码。"
      : "请输入管理员名称和密码。"
    return
  }

  if (!props.isLogin && pw !== password2.value) {
    errorMsg.value = "请确认两次密码一致。"
    return
  }

  if (pw.length < 6) {
    errorMsg.value = "密码至少 6 位。"
    return
  }

  errorMsg.value = ""
  emit("submit", {
    username: name,
    password: pw,
    password2: password2.value,
    isLogin: props.isLogin,
    deployMode: props.deployMode,
  })
}
</script>
