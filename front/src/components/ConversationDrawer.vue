<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-drawer
    :model-value="visible" @update:model-value="(val: boolean) => emit('update:visible', val)"
    :title="drawerTitle"
    :size="isMobile ? '100%' : '360px'"
    direction="ltr"
    :close-on-click-modal="true"
    :with-header="!!isMobile"
  >
    <div :class="{ 'mobile-drawer-body': isMobile }">

    <div class="section-label">频道对话</div>
    <div v-if="wechatOnline" class="channel-pinned" :class="{ active: isWechatActive }" @click="$emit('selectWechat')">
      <el-icon class="channel-icon wechat-icon"><ChatDotRound /></el-icon>
      <div class="channel-info">
        <div class="channel-name">微信对话</div>
        <div class="channel-meta">{{ wechatMsgCount || 0 }} 条消息</div>
      </div>
      <el-tag type="success" size="small" effect="plain">微信</el-tag>
    </div>
    <div v-if="qqOnline" class="channel-pinned" :class="{ active: isQQActive }" @click="$emit('selectQQ')">
      <el-icon class="channel-icon qq-icon"><ChatDotSquare /></el-icon>
      <div class="channel-info">
        <div class="channel-name">QQ对话</div>
        <div class="channel-meta">{{ qqMsgCount || 0 }} 条消息</div>
      </div>
      <el-tag type="primary" size="small" effect="plain">QQ</el-tag>
    </div>

    <div class="divider"></div>

    <div class="section-label">陪伴角色</div>
    <div class="char-list" v-if="characters.length > 0">
      <div
        v-for="c in characters"
        :key="c.id"
        class="char-item"
        :class="{ active: c.id === activeCharId && !isWechatActive && !isQQActive }"
        @click="$emit('selectChar', c)"
      >
        <el-avatar :size="32">{{ c.name?.charAt(0) }}</el-avatar>
        <div class="char-info">
          <div class="char-name">{{ c.name }}</div>
          <div class="char-desc">{{ c.identity || c.personality || '未设置' }}</div>
        </div>
        <el-tag v-if="!!c.isDefault" size="small" type="success" effect="plain">默认角色</el-tag>
      </div>
    </div>
    <el-empty v-else description="还没有配置角色" :image-size="60" />

    <div class="divider" v-if="importBatches.length > 0"></div>

    <div v-if="importBatches.length > 0" class="import-section">
      <div class="section-label">从导入记录继续聊天</div>
      <div
        v-for="batch in importBatches"
        :key="batch.id"
        class="import-item"
        @click="$emit('continueImport', batch)"
      >
        <el-icon><Upload /></el-icon>
        <span class="import-title">{{ batch.title }}</span>
        <span class="import-count">{{ batch.itemCount || 0 }}条</span>
      </div>
    </div>

    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Upload, ChatDotRound, ChatDotSquare } from "@element-plus/icons-vue"

const props = defineProps<{
  visible: boolean
  characters: any[]
  importBatches: any[]
  activeCharId: string
  wechatMsgCount: number
  isWechatActive: boolean
  wechatOnline: boolean
  qqMsgCount: number
  isQQActive: boolean
  qqOnline: boolean
}>()

const emit = defineEmits<{
  "update:visible": [val: boolean]
  selectChar: [char: any]
  selectWechat: []
  selectQQ: []
  continueImport: [batch: any]
}>()

const isMobile = computed(() => window.innerWidth < 768)
const drawerTitle = computed(() => isMobile.value ? "角色" : "陪伴角色")
</script>

<style scoped>
.section-label {
  font-size: var(--ac-font-size-xs);
  font-weight: 600;
  color: var(--ac-color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
  padding: 0 2px;
}

.channel-pinned {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
}

.channel-pinned:hover {
  background: var(--ac-color-surface-hover);
}

.channel-pinned.active {
  background: var(--ac-color-primary-bg);
  border-left: 3px solid var(--ac-color-primary);
}

.channel-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.channel-icon.wechat-icon {
  color: var(--ac-color-success, #67c23a);
}

.channel-icon.qq-icon {
  color: var(--ac-color-primary, #409eff);
}

.channel-info {
  flex: 1;
  min-width: 0;
}

.channel-name {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  color: var(--ac-color-text);
}

.channel-meta {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-top: 2px;
}

.char-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.char-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
}

.char-item:hover {
  background: var(--ac-color-surface-hover);
}

.char-item.active {
  background: var(--ac-color-primary-bg);
  border-left: 3px solid var(--ac-color-primary);
}

.char-info {
  flex: 1;
  min-width: 0;
}

.char-name {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  color: var(--ac-color-text);
}

.char-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
}

.divider {
  height: 1px;
  background: var(--ac-color-border-light);
  margin: 14px 0;
}

.import-section {
  margin-bottom: 8px;
}

.import-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
  font-size: var(--ac-font-size-sm);
}

.import-item:hover {
  background: var(--ac-color-primary-bg);
}

.import-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.import-count {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}

@media (max-width: 768px) {
  .mobile-drawer-body {
    padding: 0 4px;
  }

  .char-item {
    padding: 12px 10px;
  }

  .channel-pinned {
    padding: 12px 10px;
  }

  .import-item {
    padding: 12px 10px;
  }
}
</style>