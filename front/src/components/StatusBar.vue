<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="status-bar">
    <div class="status-left">
      <span class="status-brand">AI-Amitia</span>
      <el-tag :type="deployTagType" size="small" class="status-tag">
        {{ deployLabel }}
      </el-tag>
    </div>
    <div class="status-center">
      <div class="status-indicators">
        <span class="status-dot" :class="wechatClass" :title="wechatLabel">
          <span class="dot"></span>
          <span class="dot-label">{{ wechatLabel }}</span>
        </span>
        <span class="status-dot" :class="qqClass" :title="qqLabel">
          <span class="dot"></span>
          <span class="dot-label">{{ qqLabel }}</span>
        </span>
        <span class="status-dot" :class="modelClass" :title="modelLabel">
          <span class="dot"></span>
          <span class="dot-label">{{ modelLabel }}</span>
        </span>
        <span class="status-dot" :class="characterClass" :title="characterLabel">
          <span class="dot"></span>
          <span class="dot-label">{{ characterLabel }}</span>
        </span>
      </div>
    </div>
    <div class="status-right">
      <el-button text size="small" @click="$emit('toggleTheme')">
        <el-icon><component :is="themeIcon" /></el-icon>
      </el-button>
      <el-dropdown v-if="username" trigger="click">
        <el-button text size="small">{{ username }}</el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="$emit('logout')">退出登录</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Sunny, Moon } from "@element-plus/icons-vue"

const props = defineProps<{
  deployMode?: string
  wechatStatus?: string
  qqStatus?: string
  modelStatus?: string
  characterName?: string
  theme?: string
  username?: string
}>()

defineEmits<{
  toggleTheme: []
  logout: []
}>()

const deployLabel = computed(() =>
  props.deployMode === "cloud-web" ? "私有云" : "本地"
)
const deployTagType = computed(() =>
  props.deployMode === "cloud-web" ? "warning" : "success"
)

const wechatClass = computed(() =>
  props.wechatStatus === "connected" ? "status-on" : "status-off"
)
const wechatLabel = computed(() =>
  props.wechatStatus === "connected" ? "微信已连" : "微信未连"
)

const qqClass = computed(() =>
  props.qqStatus === "connected" || props.qqStatus === "online" ? "status-on" : "status-off"
)
const qqLabel = computed(() =>
  props.qqStatus === "connected" || props.qqStatus === "online" ? "QQ已连" : "QQ未连"
)

const modelClass = computed(() =>
  props.modelStatus === "configured" ? "status-on" : "status-off"
)
const modelLabel = computed(() =>
  props.modelStatus === "configured" ? "模型已配" : "模型未配"
)

const characterClass = computed(() =>
  props.characterName ? "status-on" : "status-off"
)
const characterLabel = computed(() =>
  props.characterName || "未选角色"
)

const themeIcon = computed(() => {
  if (props.theme === "dark") return Sunny
  return Moon
})

</script>

<style scoped>
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--ac-statusbar-height);
  padding: 0 16px;
  background: var(--ac-color-surface);
  border-bottom: 1px solid var(--ac-color-border-light);
  flex-shrink: 0;
  user-select: none;
}

.status-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-brand {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text);
  white-space: nowrap;
}

.status-tag {
  font-size: var(--ac-font-size-xs);
}

.status-center {
  flex: 1;
  display: flex;
  justify-content: center;
}

.status-indicators {
  display: flex;
  gap: 16px;
}

.status-dot {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-secondary);
  cursor: default;
}

.status-dot .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-on .dot {
  background: var(--ac-color-success);
  box-shadow: 0 0 4px rgba(82, 168, 121, 0.4);
}

.status-off .dot {
  background: var(--ac-color-text-muted);
}

.dot-label {
  white-space: nowrap;
}

.status-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

@media (max-width: 768px) {
  .status-indicators {
    gap: 10px;
  }
  .dot-label {
    display: none;
  }
}
</style>
