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
          ? '系统将启动本地核心服务，并检查数据目录、运行组件和服务状态。'
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
          管理员名称
          <input v-model="username" autocomplete="username" placeholder="例如：无拘" />
        </label>
        <label class="ob-input-label">
          管理密码
          <input v-model="password" type="password" autocomplete="current-password" placeholder="至少 6 位" />
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
import { ref, watch, onMounted, nextTick } from "vue"

const props = defineProps<{
  deployMode: string
  step: string
  isLogin: boolean
  accountName: string
}>()

const emit = defineEmits<{
  'update:step': [value: string]
  submit: [data: { username: string; password: string; password2: string; isLogin: boolean; deployMode: string }]
}>()

const username = ref(props.accountName || "")
const password = ref("")
const password2 = ref("")
const errorMsg = ref("")

const bootRows = ref([
  { name: "检查运行组件", state: "", stateText: "等待" },
  { name: "准备数据目录", state: "", stateText: "等待" },
  { name: "启动核心服务", state: "", stateText: "等待" },
  { name: "确认服务状态", state: "", stateText: "等待" },
])

let bootTimers: ReturnType<typeof setTimeout>[] = []
let flowTransitionToken = 0

const visibleStep = ref(props.step)
const panelPhase = ref<Record<string, string>>({})

function panelClass(panel: string) {
  const phase = panelPhase.value[panel] || ""
  if (phase) return phase
  if (visibleStep.value !== panel) return "setup-panel-entering"
  return ""
}

function panelVisible(panel: string) {
  return visibleStep.value === panel || panelPhase.value[panel]
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
          startBootAnimation()
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
    startBootAnimation()
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
    ? ["检查运行组件", "准备数据目录", "启动核心服务", "确认服务状态"]
    : ["检查服务地址", "验证服务版本", "确认接口兼容性", "建立远程连接"]
  bootRows.value = names.map((name) => ({ name, state: "", stateText: "等待" }))
}

function startBootAnimation() {
  bootTimers.forEach(clearTimeout)
  bootTimers = []
  const rows = bootRows.value
  rows.forEach((row) => { row.state = ""; row.stateText = "等待" })

  rows.forEach((row, index) => {
    bootTimers.push(setTimeout(() => {
      if (index > 0) {
        rows[index - 1].state = "done"
        rows[index - 1].stateText = "已完成"
      }
      row.state = "running"
      row.stateText = "处理中"

      if (index === rows.length - 1) {
        bootTimers.push(setTimeout(() => {
          row.state = "done"
          row.stateText = "已就绪"
          bootTimers.push(setTimeout(() => {
            emit("update:step", "account")
          }, 360))
        }, 650))
      }
    }, index * 650))
  })
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
