<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div v-if="visible" class="runtime-recovery">
    <div class="runtime-recovery__body">
      <div class="runtime-recovery__title">
        {{ title }}
      </div>
      <div class="runtime-recovery__detail" :class="{ 'is-collapsed': !expanded }">
        {{ message }}
      </div>
      <div class="runtime-recovery__actions">
        <el-button
          size="small"
          type="primary"
          :loading="restarting"
          @click="restart"
        >
          重启本地服务
        </el-button>
        <el-button size="small" @click="expanded = !expanded">
          {{ expanded ? "收起原始错误" : "查看原始错误" }}
        </el-button>
        <el-button size="small" text @click="copyMessage"> 复制错误 </el-button>
        <el-button size="small" text @click="openLogs"> 打开日志目录 </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage } from "element-plus";

interface RuntimeStatusLike {
  state?: string;
  message?: string;
}

const status = ref<RuntimeStatusLike | null>(null);
const restarting = ref(false);
const expanded = ref(false);
const recovered = ref(false);
let unsubscribe: (() => void) | null = null;

const bridge = computed(() => window.amitiaDesktop);

const visible = computed(() => {
  if (recovered.value) return false;
  const state = String(status.value?.state || "");
  return state === "failed";
});

const message = computed(() => {
  const raw = String(status.value?.message || "").trim();
  return raw || "本地服务未在运行，请重启本地服务后继续使用。";
});

const title = computed(() => {
  if (restarting.value) return "正在重启本地服务";
  return "本地服务已停止";
});

function applyStatus(next: RuntimeStatusLike | null) {
  const previous = String(status.value?.state || "");
  status.value = next;
  const current = String(next?.state || "");
  if (previous === "failed" && current === "ready") {
    recovered.value = true;
    ElMessage.success("本地服务已恢复，正在重新加载数据");
    window.setTimeout(() => window.location.reload(), 800);
  }
}

async function refresh() {
  const api = bridge.value;
  if (!api?.getRuntimeStatus) return;
  try {
    applyStatus(await api.getRuntimeStatus());
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(
      `读取本地服务状态失败: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

async function restart() {
  const api = bridge.value;
  if (!api?.restartRuntime) return;
  restarting.value = true;
  try {
    const next = await api.restartRuntime();
    applyStatus(next);
    if (String(next?.state || "") !== "ready") {
      ElMessage.error(
        `本地服务仍未恢复: ${String(next?.message || "未知原因").slice(0, 300)}`,
      );
    }
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(
      `重启本地服务失败: ${error instanceof Error ? error.message : String(error)}`,
    );
  } finally {
    restarting.value = false;
  }
}

async function copyMessage() {
  try {
    await navigator.clipboard.writeText(message.value);
    ElMessage.success("错误信息已复制");
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(
      `复制失败: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

function openLogs() {
  const api = bridge.value;
  if (api?.openLogsDirectory) {
    void api.openLogsDirectory().catch((error: unknown) => {
      console.error("[original-error]", error);
      ElMessage.error(
        `打开日志目录失败: ${error instanceof Error ? error.message : String(error)}`,
      );
    });
  }
}

onMounted(() => {
  void refresh();
  const api = bridge.value;
  if (api?.onRuntimeStatusChanged) {
    unsubscribe = api.onRuntimeStatusChanged((next: RuntimeStatusLike) =>
      applyStatus(next),
    );
  }
});

onUnmounted(() => {
  unsubscribe?.();
});
</script>

<style scoped>
.runtime-recovery {
  position: fixed;
  top: 48px;
  left: 50%;
  z-index: 3000;
  width: min(720px, calc(100vw - 48px));
  transform: translateX(-50%);
  border: 1px solid var(--el-color-danger-light-5, #f3b7b7);
  border-radius: 10px;
  background: var(--el-color-danger-light-9, #fef0f0);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.runtime-recovery__body {
  padding: 12px 16px;
}

.runtime-recovery__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-color-danger, #f56c6c);
}

.runtime-recovery__detail {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-regular, #606266);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 40vh;
  overflow: auto;
}

.runtime-recovery__detail.is-collapsed {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.runtime-recovery__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 10px;
}
</style>
