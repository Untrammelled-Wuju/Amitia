<template>
  <div
    class="thread-row"
    :class="{ active, compact }"
  >
    <div
      class="thread-main"
      role="button"
      tabindex="0"
      @click="emit('select', conversation)"
      @keydown.enter.prevent="emit('select', conversation)"
      @keydown.space.prevent="emit('select', conversation)"
    >
      <el-icon v-if="!compact"><ChatLineRound /></el-icon>
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
      <span v-else @dblclick.stop.prevent="beginRename">{{ conversation.title || "新对话" }}</span>
    </div>
    <button
      type="button"
      class="thread-action"
      :class="{ active: isPinned }"
      :title="isPinned ? '取消置顶' : '置顶'"
      :aria-label="isPinned ? '取消置顶' : '置顶'"
      @click.stop="emit('togglePin', conversation)"
    >
      <svg
        class="thread-action-icon pin-icon"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M12 17v5M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"
          stroke="currentColor"
          stroke-width="1.7"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </button>
    <button
      type="button"
      class="thread-action"
      title="归档"
      aria-label="归档"
      @click.stop="emit('archive', conversation)"
    >
      <ArchiveConversationIcon class="thread-action-icon" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { ChatLineRound } from "@element-plus/icons-vue";
import type { ConversationItem } from "@/stores/chat";
import ArchiveConversationIcon from "./ArchiveConversationIcon.vue";

const props = defineProps<{
  conversation: ConversationItem;
  active: boolean;
  compact?: boolean;
}>();

const emit = defineEmits<{
  select: [conversation: ConversationItem];
  rename: [conversation: ConversationItem, title: string];
  togglePin: [conversation: ConversationItem];
  archive: [conversation: ConversationItem];
}>();

const editing = ref(false);
const draftTitle = ref("");
const inputRef = ref<HTMLInputElement | null>(null);
const isPinned = computed(() => Boolean(props.conversation.pinnedAt));

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
  display: flex;
  align-items: center;
  min-height: 30px;
  border-radius: 6px;
  color: var(--text-secondary);
}
.thread-row:hover,
.thread-row.active {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.thread-row.active {
  background: var(--workbench-sidebar-active);
}
.thread-main {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  flex: 1;
  height: 30px;
  padding: 0 4px 0 8px;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
  text-align: left;
}
.thread-main:focus-visible {
  outline: 1px solid var(--ac-color-primary);
  outline-offset: -1px;
}
.thread-main span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.thread-name-input {
  min-width: 0;
  width: 100%;
  height: 24px;
  padding: 0 6px;
  border: 1px solid var(--ac-color-primary);
  border-radius: 5px;
  background: var(--surface-bg);
  color: var(--text-primary);
  font: inherit;
  outline: none;
}
.thread-action {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  flex: 0 0 auto;
  padding: 0;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
  transition:
    opacity 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease;
}
.thread-row:hover .thread-action,
.thread-row:focus-within .thread-action {
  opacity: 1;
  pointer-events: auto;
}
.thread-action:hover,
.thread-action:focus-visible,
.thread-action.active {
  background: var(--control-hover-bg);
  color: var(--ac-color-primary);
  outline: none;
}
.thread-action-icon { width: 15px; height: 15px; }
.thread-action.active .pin-icon path {
  fill: currentColor;
}
.thread-row.compact {
  min-height: 27px;
  font-size: 11px;
}
.thread-row.compact .thread-main {
  height: 27px;
}
.thread-row.compact .thread-action {
  width: 22px;
  height: 22px;
}
</style>
