<template>
  <div class="ob-stage-inner ob-setup-stage-inner ob-flow-stage-inner">
    <div
      v-if="step === 'environment'"
      class="ob-boot-panel ob-setup-step-panel"
    >
      <h2 class="ob-boot-title">
        {{ deployMode === "local" ? "正在检查运行环境" : "正在检查远程服务" }}
      </h2>
      <div class="ob-boot-copy">
        {{
          deployMode === "local"
            ? "系统将检查后端服务、侧车服务以及各数据库组件的启动状态。"
            : "业务连接将直达 Cloud Core；本机 Device Agent 仍会保留，用于设备本地 Runtime 能力。"
        }}
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
      v-else
      class="ob-account-sheet ob-setup-step-panel"
    >
      <div class="kicker">{{ isLogin ? "账号登录" : "账号注册" }}</div>
      <div class="ob-sheet-title">
        {{
          isLogin
            ? deployMode === "remote"
              ? "登录远程管理账号"
              : "登录管理账号"
            : deployMode === "remote"
              ? "初始化远程管理员"
              : "注册管理账号"
        }}
      </div>
      <div class="ob-account-sheet-desc">
        {{
          isLogin
            ? deployMode === "remote"
              ? "使用 Cloud Core 已有的管理账号登录。"
              : "管理账号已经创建。请输入管理员名称和密码，登录后继续设置。"
            : deployMode === "remote"
              ? "该 Cloud Core 尚无管理员。请输入服务器 AMITIA_SETUP_TOKEN 对应的初始化令牌后创建首个管理员。"
              : "用于进入管理与设置页面，并保护配置、聊天记录和记忆数据。"
        }}
      </div>
      <div class="ob-form-stack">
        <label class="ob-input-label">
          账号
          <input
            v-model="username"
            autocomplete="username"
            placeholder="请输入账号"
          />
        </label>
        <label class="ob-input-label">
          密码
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            placeholder="请输入密码"
          />
        </label>
        <label v-if="!isLogin" class="ob-input-label">
          确认密码
          <input
            v-model="password2"
            type="password"
            autocomplete="new-password"
            placeholder="再次输入密码"
          />
        </label>
        <label v-if="deployMode === 'remote' && !isLogin" class="ob-input-label">
          初始化令牌
          <input
            v-model="setupToken"
            type="password"
            autocomplete="off"
            placeholder="AMITIA_SETUP_TOKEN（至少 32 位）"
          />
        </label>
      </div>
      <div class="ob-error">{{ errorMsg }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from "vue";
import { getApiBaseURL } from "@/runtime/runtime-adapter";

const props = defineProps<{
  deployMode: string;
  step: string;
  isLogin: boolean;
  accountName: string;
  serverURL: string;
}>();

const emit = defineEmits<{
  healthCheckDone: [];
  submit: [
    data: {
      username: string;
      password: string;
      password2: string;
      setupToken: string;
      isLogin: boolean;
      deployMode: string;
    },
  ];
}>();

const username = ref(props.accountName || "");
const password = ref("");
const password2 = ref("");
const setupToken = ref("");
const errorMsg = ref("");

const bootRows = ref([
  { name: "检查本地服务", state: "", stateText: "等待" },
  { name: "检查运行时就绪状态", state: "", stateText: "等待" },
  { name: "检查运行时能力", state: "", stateText: "等待" },
]);

let abortController: AbortController | null = null;
watch(
  () => props.deployMode,
  () => {
    if (props.step === "environment") {
      updateBootLabels();
    }
  },
);

onMounted(() => {
  if (props.step === "environment") {
    updateBootLabels();
    startRealChecks();
  }
});

onUnmounted(() => {
  if (abortController) {
    abortController.abort();
    abortController = null;
  }
});

watch(
  () => props.step,
  (s) => {
    if (s === "environment") {
      updateBootLabels();
      startRealChecks();
    }
  },
);

function updateBootLabels() {
  const local = props.deployMode === "local";
  const names = local
    ? ["检查本地服务", "检查运行时就绪状态", "检查运行时能力"]
    : ["检查服务地址", "验证服务版本", "确认接口兼容性", "建立远程连接"];
  bootRows.value = names.map((name) => ({
    name,
    state: "",
    stateText: "等待",
  }));
}

async function startRealChecks() {
  if (abortController) {
    abortController.abort();
  }
  abortController = new AbortController();
  const signal = abortController.signal;

  const rows = bootRows.value;
  rows.forEach((row) => {
    row.state = "";
    row.stateText = "等待";
  });

  const isLocal = props.deployMode === "local";

  try {
    if (isLocal) {
      await runLocalChecks(rows, signal);
    } else {
      await runRemoteChecks(rows, signal);
    }

    if (!signal.aborted) {
      const allDone = rows.every((r) => r.state === "done");
      if (allDone) {
        setTimeout(() => {
          if (!signal.aborted) {
            emit("healthCheckDone");
          }
        }, 360);
      }
    }
  } catch (e: any) {
    if (signal.aborted) return;
    const failedRow = rows.find((r) => r.state === "running");
    if (failedRow) {
      failedRow.state = "error";
      failedRow.stateText = "失败";
    }
  }
}

async function fetchWithTimeout(
  url: string,
  signal: AbortSignal,
  timeoutMs = 10000,
  init: RequestInit = {},
): Promise<Response> {
  const controller = new AbortController();
  const linkedSignal = controller.signal;

  signal.addEventListener("abort", () => controller.abort());

  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const res = await fetch(url, { ...init, signal: linkedSignal });
    return res;
  } finally {
    clearTimeout(timeout);
  }
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const t = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => {
      clearTimeout(t);
      reject(new DOMException("Aborted", "AbortError"));
    });
  });
}

async function runLocalChecks(
  rows: typeof bootRows.value,
  signal: AbortSignal,
) {
  const apiBase = await getApiBaseURL();
  const checks = [
    {
      path: "/api/public/health",
      successText: "正常运行",
      errorText: "服务未就绪",
    },
    {
      path: "/readyz",
      successText: "已就绪",
      errorText: "运行时未就绪",
    },
    {
      path: "/api/public/runtime/capabilities",
      successText: "能力可用",
      errorText: "能力接口不可用",
    },
  ];

  for (let i = 0; i < rows.length; i++) {
    if (signal.aborted) return;
    if (i > 0) {
      await delay(350, signal);
    }

    const row = rows[i];
    const check = checks[i];
    row.state = "running";
    row.stateText = "处理中";

    try {
      const res = await fetchWithTimeout(
        `${apiBase}${check.path}`,
        signal,
        8000,
      );
      row.state = res.ok ? "done" : "error";
      row.stateText = res.ok ? check.successText : check.errorText;
    } catch {
      row.state = "error";
      row.stateText = "无法连接";
    }
  }
}

async function runRemoteChecks(
  rows: typeof bootRows.value,
  signal: AbortSignal,
) {
  const remoteURL = (props.serverURL || "").trim().replace(/\/+$/, "");

  for (let i = 0; i < rows.length; i++) {
    if (signal.aborted) return;
    const row = rows[i];

    if (i === 0) {
      row.state = "running";
      row.stateText = "处理中";

      try {
        const res = await fetchWithTimeout(
          `${remoteURL}/api/public/health`,
          signal,
          8000,
        );
        if (res.ok) {
          row.state = "done";
          row.stateText = "已完成";
        } else {
          row.state = "error";
          row.stateText = "无法访问";
        }
      } catch {
        row.state = "error";
        row.stateText = "无法连接";
      }
    } else if (i === 1) {
      await delay(650, signal);
      if (signal.aborted) return;

      row.state = "running";
      row.stateText = "处理中";

      try {
        const res = await fetchWithTimeout(
          `${remoteURL}/api/public/health`,
          signal,
          8000,
        );
        if (res.ok) {
          const data = await res.json();
          const hasVersion = !!(data?.data?.version || data?.version);
          row.state = hasVersion ? "done" : "error";
          row.stateText = hasVersion ? "已完成" : "版本未知";
        } else {
          row.state = "error";
          row.stateText = "异常";
        }
      } catch {
        row.state = "error";
        row.stateText = "无法连接";
      }
    } else if (i === 2) {
      await delay(650, signal);
      if (signal.aborted) return;

      row.state = "running";
      row.stateText = "处理中";

      try {
        const res = await fetchWithTimeout(
          `${remoteURL}/api/public/runtime/capabilities`,
          signal,
          8000,
        );
        if (res.ok) {
          row.state = "done";
          row.stateText = "已完成";
        } else {
          row.state = "error";
          row.stateText = "不兼容";
        }
      } catch {
        row.state = "error";
        row.stateText = "无法连接";
      }
    } else if (i === 3) {
      await delay(650, signal);
      if (signal.aborted) return;

      row.state = "running";
      row.stateText = "处理中";
      try {
        const res = await fetchWithTimeout(
          `${remoteURL}/api/public/auth/status`,
          signal,
          8000,
        );
        if (!res.ok) {
          row.state = "error";
          row.stateText = "认证接口不可用";
          continue;
        }
        const data = await res.json();
        const status = data?.data ?? data;
        const valid = typeof status?.hasAdmin === "boolean";
        row.state = valid ? "done" : "error";
        row.stateText = valid ? "已完成" : "响应不兼容";
      } catch {
        row.state = "error";
        row.stateText = "无法连接";
      }
    }
  }
}

function handleSubmit() {
  const name = username.value.trim();
  const pw = password.value;

  if (!name || !pw) {
    errorMsg.value =
      props.deployMode === "remote"
        ? "请输入远程管理账号和密码。"
        : "请输入管理员名称和密码。";
    return;
  }

  if (!props.isLogin && pw !== password2.value) {
    errorMsg.value = "请确认两次密码一致。";
    return;
  }

  if (pw.length < 6) {
    errorMsg.value = "密码至少 6 位。";
    return;
  }

  if (props.deployMode === "remote" && !props.isLogin && setupToken.value.trim().length < 32) {
    errorMsg.value = "远程首管理员初始化令牌至少 32 位。";
    return;
  }

  errorMsg.value = "";
  emit("submit", {
    username: name,
    password: pw,
    password2: password2.value,
    setupToken: setupToken.value.trim(),
    isLogin: props.isLogin,
    deployMode: props.deployMode,
  });
}

defineExpose({ submit: handleSubmit });
</script>
