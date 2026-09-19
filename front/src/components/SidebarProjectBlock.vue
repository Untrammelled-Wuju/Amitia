<template>
  <div class="project-block">
    <div class="project-row" :class="{ active: active }">
      <button
        type="button"
        class="project-main"
        :title="project.statusReason || project.name"
        @click="expanded = !expanded"
      >
        <el-icon><FolderOpened /></el-icon>
        <span>{{ project.name }}</span>
        <small v-if="!project.available">不可用</small>
      </button>
      <button
        type="button"
        class="project-action project-create-action"
        title="在项目中新建对话"
        aria-label="在项目中新建对话"
        :disabled="!project.available"
        @click.stop="emit('createConversation', project)"
      >
        <el-icon><Plus /></el-icon>
      </button>
      <el-dropdown
        class="project-action-wrap"
        trigger="click"
        @command="(command: string | number | object) => emit('command', project, command)"
      >
        <button type="button" class="project-action" title="项目操作" aria-label="项目操作" @click.stop>
          <el-icon><MoreFilled /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="newChat" :disabled="!project.available">新建对话</el-dropdown-item>
            <el-dropdown-item command="rename">重命名项目</el-dropdown-item>
            <el-dropdown-item command="changeRoot">更换根目录</el-dropdown-item>
            <el-dropdown-item command="pin">{{ project.pinnedAt ? "取消置顶" : "置顶" }}</el-dropdown-item>
            <el-dropdown-item command="open" :disabled="!project.available">在资源管理器中打开</el-dropdown-item>
            <el-dropdown-item command="remove" divided>移除项目</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
    <Transition name="project-threads">
      <div v-if="expanded" class="project-threads">
        <SidebarConversationRow
          v-for="conversation in visibleConversations"
          :key="conversation.id"
          :conversation="conversation"
          :active="conversation.id === activeConversationId"
          compact
          @select="emit('select', $event)"
          @rename="(item, title) => emit('renameConversation', item, title)"
          @toggle-pin="emit('togglePinConversation', $event)"
          @archive="emit('archiveConversation', $event)"
        />
        <button
          v-if="project.conversations.length > 5"
          type="button"
          class="thread-expand"
          @click="expandedConversations = !expandedConversations"
        >
          {{ expandedConversations ? "收起" : "展开显示" }}
        </button>
        <div v-if="project.conversations.length === 0" class="thread-empty">暂无对话</div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { FolderOpened, MoreFilled, Plus } from "@element-plus/icons-vue";
import type { ConversationItem, ProjectItem } from "@/stores/chat";
import SidebarConversationRow from "./SidebarConversationRow.vue";

const props = defineProps<{
  project: ProjectItem;
  active: boolean;
  activeConversationId: string;
}>();

const emit = defineEmits<{
  select: [conversation: ConversationItem];
  createConversation: [project: ProjectItem];
  command: [project: ProjectItem, command: string | number | object];
  renameConversation: [conversation: ConversationItem, title: string];
  togglePinConversation: [conversation: ConversationItem];
  archiveConversation: [conversation: ConversationItem];
}>();

const expanded = ref(false);
const expandedConversations = ref(false);
const visibleConversations = computed(() =>
  expandedConversations.value
    ? props.project.conversations
    : props.project.conversations.slice(0, 5),
);
</script>

<style scoped>
.project-block {
  display: grid;
  gap: 1px;
}
.project-row {
  display: flex;
  align-items: center;
  gap: 2px;
  min-height: 32px;
  padding: 0 3px 0 7px;
  border-radius: 7px;
  color: var(--text-secondary);
}
.project-row:hover,
.project-row.active {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.project-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
  height: 32px;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
  text-align: left;
}
.project-main:focus-visible {
  outline: 1px solid var(--ac-color-primary);
  outline-offset: -1px;
}
.project-main span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.project-main small {
  margin-left: auto;
  color: var(--text-muted);
  font-size: 9px;
}
.project-action {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}
.project-action:hover {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.project-action:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}
.project-create-action,
.project-action-wrap {
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.16s ease;
}
.project-row:hover .project-create-action,
.project-row:hover .project-action-wrap,
.project-row:focus-within .project-create-action,
.project-row:focus-within .project-action-wrap {
  opacity: 1;
  pointer-events: auto;
}
.project-threads {
  display: grid;
  gap: 1px;
  padding-left: 18px;
}
.project-threads-enter-active,
.project-threads-leave-active {
  overflow: hidden;
  transition:
    max-height 0.2s ease,
    opacity 0.16s ease,
    transform 0.2s ease;
}
.project-threads-enter-from,
.project-threads-leave-to {
  max-height: 0;
  opacity: 0;
  transform: translateY(-4px);
}
.project-threads-enter-to,
.project-threads-leave-from {
  max-height: 1200px;
  opacity: 1;
  transform: translateY(0);
}
@media (prefers-reduced-motion: reduce) {
  .project-threads-enter-active,
  .project-threads-leave-active {
    transition: none;
  }
}
.thread-expand {
  align-self: flex-start;
  margin: 2px 0 2px 12px;
  padding: 3px 7px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--ac-color-primary);
  cursor: pointer;
  font: inherit;
  font-size: 11px;
}
.thread-expand:hover {
  background: var(--workbench-sidebar-hover);
}
.thread-empty {
  padding: 5px 9px;
  color: var(--text-muted);
  font-size: 10px;
}
</style>
