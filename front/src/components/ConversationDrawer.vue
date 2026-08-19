<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-drawer
    :model-value="visible"
    @update:model-value="(val: boolean) => emit('update:visible', val)"
    :title="drawerTitle"
    :size="isMobile ? '100%' : '360px'"
    direction="ltr"
    :close-on-click-modal="true"
    :with-header="!!isMobile"
    modal-class="workbench-drawer-modal"
  >
    <div :class="{ 'mobile-drawer-body': isMobile }">
      <div v-if="wechatOnline || qqOnline" class="drawer-section">
        <div class="section-label">频道对话</div>
        <button
          v-if="wechatOnline"
          type="button"
          class="drawer-row"
          :class="{ active: isWechatActive }"
          @click.stop="onSelectWechat"
        >
          <el-icon class="channel-icon"><ChatDotRound /></el-icon>
          <span class="row-copy"><strong>微信对话</strong><small>{{ wechatMsgCount || 0 }} 条消息</small></span>
          <span class="row-tag">微信</span>
        </button>
        <button
          v-if="qqOnline"
          type="button"
          class="drawer-row"
          :class="{ active: isQQActive }"
          @click.stop="onSelectQQ"
        >
          <el-icon class="channel-icon"><ChatDotSquare /></el-icon>
          <span class="row-copy"><strong>QQ 对话</strong><small>{{ qqMsgCount || 0 }} 条消息</small></span>
          <span class="row-tag">QQ</span>
        </button>
      </div>

      <div v-if="webConversations.length" class="drawer-section">
        <div class="section-label">最近对话</div>
        <button
          v-for="conversation in webConversations"
          :key="conversation.id"
          type="button"
          class="drawer-row conversation-row"
          :class="{ active: conversation.id === activeConvId && !isWechatActive && !isQQActive }"
          @click="$emit('selectConv', conversation)"
        >
          <el-icon><ChatLineRound /></el-icon>
          <span class="row-copy">
            <strong>{{ conversation.title || "新对话" }}</strong>
            <small>{{ conversation.messageCount || 0 }} 条消息</small>
          </span>
        </button>
      </div>

      <div class="drawer-section">
        <div class="section-label">陪伴角色</div>
        <div class="char-list" v-if="characters.length > 0">
          <button
            v-for="c in characters"
            :key="c.id"
            type="button"
            class="char-item"
            :class="{ active: c.id === activeCharId && !isWechatActive && !isQQActive }"
            @click="$emit('selectChar', c)"
          >
            <el-avatar :size="30" :src="c.avatar || undefined">{{ c.name?.charAt(0) }}</el-avatar>
            <span class="char-info">
              <strong>{{ c.name }}</strong>
              <small>{{ c.identity || c.personality || "未设置" }}</small>
            </span>
            <span v-if="!!c.isDefault" class="row-tag">默认</span>
          </button>
        </div>
        <el-empty v-else description="还没有配置角色" :image-size="60" />
      </div>

      <div v-if="importBatches.length > 0" class="drawer-section">
        <div class="section-label">从导入记录继续聊天</div>
        <button
          v-for="batch in importBatches"
          :key="batch.id"
          type="button"
          class="drawer-row"
          @click="$emit('continueImport', batch)"
        >
          <el-icon><Upload /></el-icon>
          <span class="row-copy"><strong>{{ batch.title }}</strong><small>{{ batch.itemCount || batch.totalItems || batch.messageCount || 0 }} 条记录</small></span>
        </button>
      </div>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { Upload, ChatDotRound, ChatDotSquare, ChatLineRound } from "@element-plus/icons-vue";

const props = defineProps<{
  visible: boolean;
  characters: any[];
  conversations?: any[];
  importBatches: any[];
  activeCharId: string;
  activeConvId?: string;
  wechatMsgCount: number;
  isWechatActive: boolean;
  wechatOnline: boolean;
  qqMsgCount: number;
  isQQActive: boolean;
  qqOnline: boolean;
}>();

const emit = defineEmits<{
  "update:visible": [val: boolean];
  selectChar: [char: any];
  selectConv: [conversation: any];
  selectWechat: [];
  selectQQ: [];
  continueImport: [batch: any];
}>();

const isMobile = computed(() => window.innerWidth < 768);
const drawerTitle = computed(() => (isMobile.value ? "对话与角色" : ""));
const webConversations = computed(() =>
  (props.conversations || [])
    .filter((item: any) => !item.channel || item.channel === "web")
    .slice(0, 20),
);

function onSelectWechat() { emit("selectWechat"); }
function onSelectQQ() { emit("selectQQ"); }
</script>

<style scoped>
.drawer-section { padding: 0 0 14px; margin-bottom: 12px; border-bottom: 1px solid var(--surface-border); }
.drawer-section:last-child { border-bottom: 0; }
.section-label { padding: 0 6px 6px; color: var(--text-muted); font-size: 11px; font-weight: 550; }
.drawer-row, .char-item { display: flex; align-items: center; gap: 9px; width: 100%; min-height: 42px; padding: 6px 8px; border: 0; border-radius: 8px; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; text-align: left; }
.drawer-row:hover, .char-item:hover, .drawer-row.active, .char-item.active { background: var(--control-hover-bg); color: var(--text-primary); }
.drawer-row.active, .char-item.active { background: var(--control-active-bg); }
.row-copy, .char-info { min-width: 0; flex: 1; }
.row-copy strong, .row-copy small, .char-info strong, .char-info small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-copy strong, .char-info strong { color: var(--text-primary); font-size: 12px; font-weight: 550; }
.row-copy small, .char-info small { margin-top: 2px; color: var(--text-muted); font-size: 10px; }
.row-tag { flex: 0 0 auto; padding: 2px 6px; border-radius: 999px; background: var(--control-active-bg); color: var(--ac-color-primary); font-size: 9px; }
.channel-icon { color: var(--ac-color-primary); }
.char-list { display: flex; flex-direction: column; gap: 2px; }
@media (max-width: 768px) { .mobile-drawer-body { padding: 0 4px; } .drawer-row, .char-item { min-height: 48px; } }
</style>
