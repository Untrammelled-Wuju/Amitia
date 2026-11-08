<template>
  <div class="ob-stage-inner ob-flow-stage-inner">
    <div class="kicker">选择统筹中心</div>
    <h2 class="ob-title">选择 Amitia 的统筹中心</h2>
    <p class="ob-sub-title">
      本机可以作为统筹中心，也可以连接另一台设备提供的统筹中心。
    </p>
    <div class="ob-deploy-container">
      <div class="ob-deploy-options">
        <button
          v-if="desktopAvailable"
          class="ob-deploy-option"
          :class="{ selected: deployMode === 'local' }"
          @click="$emit('update:deployMode', 'local')"
        >
          <div class="ob-deploy-icon">⌂</div>
          <div class="ob-deploy-info">
            <div class="ob-deploy-title">以本机作为统筹中心</div>
            <div class="ob-deploy-desc">
              当前设备保存统筹数据和模型配置，并管理其他设备
            </div>
          </div>
        </button>
        <button
          class="ob-deploy-option"
          :class="{ selected: deployMode === 'remote' }"
          @click="$emit('update:deployMode', 'remote')"
        >
          <div class="ob-deploy-icon">
            <svg width="22" height="18" viewBox="0 0 22 18" fill="none">
              <circle cx="11" cy="15" r="1.5" fill="currentColor" />
              <path
                d="M7.5 12a5 5 0 0 1 7 0"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
              <path
                d="M4.2 8.8a10 10 0 0 1 13.6 0"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
              <path
                d="M1 5.6a14.5 14.5 0 0 1 20 0"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
            </svg>
          </div>
          <div class="ob-deploy-info">
            <div class="ob-deploy-title">使用远程统筹中心</div>
            <div class="ob-deploy-desc">
              连接另一台设备提供的统筹中心并完成设备配对
            </div>
          </div>
        </button>
      </div>
      <div v-if="deployMode === 'remote'" class="ob-remote-config">
        <label class="ob-input-label">
          统筹中心地址
          <input
            class="ob-field"
            :class="{ 'ob-field-warn': urlWarn }"
            :value="serverURL"
            @input="onInput"
            placeholder="https://amitia.example.com"
          />
        </label>
        <div class="ob-remote-actions">
          <span
            class="ob-small-action"
            :class="{ spinning: detectingRemote }"
            @click="handleCheckRemote"
          >
            <span v-if="detectingRemote" class="ob-spin-icon"></span>
            {{
              detectingRemote ? "检测中" : remoteStatusText || "检测连接"
            }}
          </span>
        </div>
        <div v-if="urlWarn" class="ob-url-warn">{{ urlWarnMsg }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";

const props = defineProps<{
  deployMode: string;
  serverURL: string;
  detectingRemote: boolean;
  remoteStatusText: string;
}>();

const emit = defineEmits<{
  "update:deployMode": [value: string];
  "update:serverURL": [value: string];
  next: [];
  checkRemote: [];
}>();

const desktopAvailable = typeof window !== "undefined" && Boolean(window.amitiaDesktop);
const urlWarn = ref(false);
const urlWarnMsg = ref("");

function onInput(e: Event) {
  urlWarn.value = false;
  emit("update:serverURL", (e.target as HTMLInputElement).value);
}

function handleCheckRemote() {
  if (!props.serverURL.trim()) {
    urlWarn.value = true;
    urlWarnMsg.value = "请输入远程地址";
    return;
  }
  urlWarn.value = false;
  emit("checkRemote");
}

function handleNext() {
  if (props.deployMode === "remote") {
    if (!props.serverURL.trim()) {
      urlWarn.value = true;
      urlWarnMsg.value = "请输入统筹中心地址";
      return;
    }
    if (props.remoteStatusText !== "连接成功") {
      urlWarn.value = true;
      urlWarnMsg.value = "请先检测连接";
      return;
    }
  }
  emit("next");
}

defineExpose({ continueFlow: handleNext });
</script>
