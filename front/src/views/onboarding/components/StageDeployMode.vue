<template>
  <div class="ob-stage-inner">
    <div class="ob-choice-cloud">
      <div class="ob-choice-cloud-scale">
        <button
          class="ob-path-choice local"
          :class="{ selected: deployMode === 'local' }"
          @click="$emit('update:deployMode', 'local')"
        >
          <div class="ob-path-node">⌂</div>
          <div class="ob-choice-title">在这台设备上运行</div>
          <div class="ob-choice-desc">
            由 Amitia 自动启动本地服务，角色数据、记忆和配置默认保存在当前设备。
          </div>
          <div class="ob-choice-note">适合个人长期使用 · 无需准备服务器</div>
        </button>
        <button
          class="ob-path-choice remote"
          :class="{ selected: deployMode === 'remote' }"
          @click="$emit('update:deployMode', 'remote')"
        >
          <div class="ob-path-node">
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
          <div class="ob-choice-title">连接已有服务</div>
          <div class="ob-choice-desc">
            连接已经部署好的 Amitia 服务。本设备仅作为使用端，不会启动本地服务。
          </div>
          <div class="ob-remote-swap">
            <div class="ob-choice-note ob-remote-note">需要配置远程地址</div>
            <div class="ob-remote-inline">
              <input
                class="ob-field"
                :class="{ 'ob-field-warn': urlWarn }"
                :value="serverURL"
                @input="onInput"
                placeholder="https://amitia.example.com"
              />
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
          </div>
          <div v-if="urlWarn" class="ob-url-warn">{{ urlWarnMsg }}</div>
        </button>
      </div>
    </div>
    <button class="ob-stage-action" @click="handleNext">继续</button>
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
      urlWarnMsg.value = "请输入远程地址";
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
</script>
