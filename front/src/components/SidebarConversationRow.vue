<template>
  <div
    class="thread-row"
    data-amitia-ui="app.sidebar.item"
    :class="{ compact }"
  >
    <div
      class="thread-main"
      role="button"
      tabindex="0"
      :aria-current="active ? 'page' : undefined"
      @click="emit('select', conversation)"
      @keydown.enter.prevent="emit('select', conversation)"
      @keydown.space.prevent="emit('select', conversation)"
      @contextmenu="openConversationContextMenu"
    >
      <span class="thread-avatar-wrap">
        <el-avatar :size="compact ? 38 : 42" :src="role?.avatar || undefined" fit="cover">{{ avatarInitial }}</el-avatar>
        <span v-if="unreadCount > 0" class="thread-unread" :aria-label="`${unreadCount} 条未读`">{{ unreadLabel }}</span>
      </span>
      <span class="thread-copy">
        <span class="thread-heading">
          <el-icon v-if="isPinned" class="thread-pin"><CollectionTag /></el-icon>
          <input
            v-if="editing"
            ref="inputRef"
            v-model="draftTitle"
            class="thread-name-input"
            @click.stop
            @keydown.enter.prevent="saveRename"
            @keydown.esc.prevent="cancelRename"
            @blur="saveRename"
          />
          <span v-else class="thread-title" :title="conversation.title || '新对话'" @dblclick.stop.prevent="beginRename">{{ conversation.title || "新对话" }}</span>
          <span class="thread-meta">
            <time>{{ timeLabel }}</time>
            <span v-if="conversation.isGenerating" class="thread-spinner" aria-label="正在生成"></span>
          </span>
        </span>
        <span class="thread-preview" :title="previewText">{{ previewText }}</span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import {
  Box,
  ChatLineRound,
  CircleCheck,
  CollectionTag,
  Delete,
  EditPen,
  FolderOpened,
} from "@element-plus/icons-vue";
import type { ConversationItem, ProjectItem } from "@/stores/chat";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { isDesktopShell } from "@/runtime/runtime-capabilities";
import {
  openContextMenu,
  type ContextMenuAction,
} from "./context-menu/contextMenuStore";

const props = defineProps<{
  conversation: ConversationItem;
  active: boolean;
  compact?: boolean;
  projects?: ProjectItem[];
}>();

const emit = defineEmits<{
  select: [conversation: ConversationItem];
  rename: [conversation: ConversationItem, title: string];
  togglePin: [conversation: ConversationItem];
  archive: [conversation: ConversationItem];
  move: [conversation: ConversationItem, projectId: string];
  open: [conversation: ConversationItem];
  markRead: [conversation: ConversationItem];
  delete: [conversation: ConversationItem];
}>();

const editing = ref(false);
const draftTitle = ref("");
const inputRef = ref<HTMLInputElement | null>(null);
const owned = useDeviceOwnedConversation();
const isPinned = computed(() => Boolean(props.conversation.pinnedAt));
const availableProjects = computed(() => (props.projects || []).filter((project) => !project.readOnly && !project.hiddenAt));
const role = computed(() => owned.roles.value.find((item) => item.id === props.conversation.characterId));
const avatarInitial = computed(() => (role.value?.name || props.conversation.title || "AI").trim().slice(0, 1));
const unreadCount = computed(() => Math.max(0, Number(props.conversation.unreadCompletedCount || (props.conversation.hasUnread ? 1 : 0))));
const unreadLabel = computed(() => unreadCount.value > 99 ? "99+" : String(unreadCount.value));
const previewText = computed(() => {
  if (props.conversation.isGenerating && props.conversation.activityLabel) return props.conversation.activityLabel;
  const content = String(props.conversation.lastMessage || props.conversation.lastMessagePreview || "").trim().replace(/\s+/g, " ");
  if (content) return content;
  const type = String(props.conversation.lastMessageType || "").toLowerCase();
  if (type === "image") return "[图片]";
  if (type === "video") return "[视频]";
  if (type === "audio" || type === "voice") return "[语音]";
  if (type === "file") return "[文件]";
  return props.conversation.isGenerating ? "正在生成回复…" : "暂无消息";
});
const timeLabel = computed(() => {
  const time = new Date(String(props.conversation.updatedAt || ""));
  if (Number.isNaN(time.getTime())) return "";
  const now = new Date();
  if (time.toDateString() === now.toDateString()) return `${String(time.getHours()).padStart(2, "0")}:${String(time.getMinutes()).padStart(2, "0")}`;
  return `${time.getMonth() + 1}/${time.getDate()}`;
});

function moveTargets(): ContextMenuAction[] {
  const targets: ContextMenuAction[] = [];
  if (props.conversation.projectId) {
    targets.push({
      id: "move-recent",
      label: "移回聊天",
      icon: ChatLineRound,
      action: () => emit("move", props.conversation, ""),
    });
  }
  for (const project of availableProjects.value) {
    targets.push({
      id: `move-project-${project.id}`,
      label: `移入项目：${project.name}`,
      icon: FolderOpened,
      disabled: project.id === props.conversation.projectId,
      action: () => emit("move", props.conversation, project.id),
    });
  }
  if (!targets.length) {
    targets.push({
      id: "move-empty",
      label: props.conversation.projectId ? "暂无其他项目" : "暂无可用项目",
      icon: FolderOpened,
      disabled: true,
    });
  }
  return targets;
}

function openConversationContextMenu(event: MouseEvent) {
  if (!isDesktopShell()) return;
  const target = event.target instanceof Element ? event.target : null;
  if (target?.closest("input, textarea, [contenteditable='true']")) return;
  event.preventDefault();
  event.stopPropagation();
  openContextMenu({
    x: event.clientX,
    y: event.clientY,
    title: props.conversation.title || "新对话",
    items: [
      {
        id: "open-conversation",
        label: "打开对话",
        icon: ChatLineRound,
        action: () => emit("open", props.conversation),
      },
      {
        id: "rename-conversation",
        label: "重命名",
        icon: EditPen,
        action: beginRename,
      },
      {
        id: "pin-conversation",
        label: isPinned.value ? "取消置顶" : "置顶",
        icon: CollectionTag,
        separatorBefore: true,
        action: () => emit("togglePin", props.conversation),
      },
      {
        id: "mark-conversation-read",
        label: "标为已读",
        icon: CircleCheck,
        disabled: unreadCount.value <= 0,
        action: () => emit("markRead", props.conversation),
      },
      {
        id: "archive-conversation",
        label: "归档",
        icon: Box,
        action: () => emit("archive", props.conversation),
      },
      {
        id: "move-conversation",
        label: "移动到项目",
        icon: FolderOpened,
        children: moveTargets(),
      },
      {
        id: "delete-conversation",
        label: "删除对话",
        icon: Delete,
        danger: true,
        separatorBefore: true,
        action: () => emit("delete", props.conversation),
      },
    ],
  });
}

async function beginRename() {
  editing.value = true;
  draftTitle.value = props.conversation.title || "";
  await nextTick();
  inputRef.value?.focus();
  inputRef.value?.select();
}

function saveRename() {
  if (!editing.value) return;
  const title = draftTitle.value.trim();
  editing.value = false;
  if (!title || title === props.conversation.title) return;
  emit("rename", props.conversation, title);
}

function cancelRename() {
  editing.value = false;
  draftTitle.value = "";
}
</script>

<style scoped>
.thread-row {
  position: relative;
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  min-height: 58px;
  border-radius: 6px;
  color: var(--text-secondary);
}
.thread-row:hover {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.thread-row:has([aria-current="page"]) {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.thread-main {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  flex: 1;
  width: 100%;
  min-height: 58px;
  line-height: 1.35;
  padding: 6px 7px;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  font-size: var(--ac-font-size-sm);
  text-align: left;
}
.thread-main:focus-visible {
  outline: 2px solid var(--ac-color-primary);
  outline-offset: -2px;
}
.thread-avatar-wrap {
  position: relative;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
}
.thread-avatar-wrap :deep(.el-avatar) {
  background: color-mix(in srgb, var(--ac-color-primary) 16%, var(--surface-bg-elevated));
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}
.thread-unread {
  position: absolute;
  top: -4px;
  right: -5px;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  display: grid;
  place-items: center;
  border: 2px solid var(--surface-bg);
  border-radius: 9px;
  background: var(--el-color-danger);
  color: #fff;
  font-size: 9px;
  font-weight: 600;
  line-height: 1;
}
.thread-copy {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
  padding-right: 48px;
}
.thread-heading {
  width: 100%;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 5px;
}
.thread-main .thread-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.thread-pin {
  flex: 0 0 auto;
  color: var(--ac-color-primary);
  font-size: 12px;
}
.thread-meta {
  position: absolute;
  top: 1px;
  right: 0;
  width: 42px;
  min-height: 28px;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  gap: 3px;
  color: var(--text-muted);
  font-size: 10px;
  line-height: 1;
  text-align: right;
  margin-left: auto;
}
.thread-spinner {
  width: 11px;
  height: 11px;
  border: 1.6px solid color-mix(in srgb, var(--ac-color-primary) 28%, transparent);
  border-top-color: var(--ac-color-primary);
  border-radius: 50%;
  animation: conversation-activity-spin 0.75s linear infinite;
}
.thread-preview {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: var(--ac-font-size-xs);
  line-height: 1.3;
}
@keyframes conversation-activity-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .thread-spinner {
    animation-duration: 1.6s;
  }
}
.thread-name-input {
  flex: 1;
  min-width: 0;
  width: 100%;
  min-height: 24px;
  padding: 3px 6px;
  border: 1px solid var(--ac-color-primary);
  border-radius: 5px;
  background: var(--surface-bg);
  color: var(--text-primary);
  font: inherit;
  outline: none;
}
.thread-row.compact {
  min-height: 52px;
  font-size: var(--ac-font-size-xs);
}
.thread-row.compact .thread-main {
  min-height: 52px;
  line-height: 1.35;
}
button, .thread-main, .thread-name-input { line-height: 1.45; }
button:not(:disabled):focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
</style>
