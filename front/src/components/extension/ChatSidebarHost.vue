<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { ElMessage } from "element-plus";
import { Fold, Expand } from "@element-plus/icons-vue";
import { fetchContributions } from "@/api/extension";
import type { UIContributionSummary } from "@/stores/extensionUI";
import SchemaUIRenderer from "./SchemaUIRenderer.vue";
import SandboxWebUIFrame from "./SandboxWebUIFrame.vue";

interface ChatSidebarItem {
  contribution: UIContributionSummary;
  open: boolean;
  width: number;
  resizable: boolean;
}

const props = withDefaults(
  defineProps<{
    context?: Record<string, unknown>;
    defaultWidth?: number;
    minWidth?: number;
    maxWidth?: number;
    slotId?: string;
  }>(),
  {
    defaultWidth: 320,
    minWidth: 240,
    maxWidth: 600,
    slotId: "chat.sidebar.panel",
  }
);

const loading = ref(false);
const sidebarItems = ref<ChatSidebarItem[]>([]);
const containerWidth = ref(props.defaultWidth);

const hasSidebars = computed(() => sidebarItems.value.length > 0);
const openItems = computed(() => sidebarItems.value.filter((i) => i.open));

async function loadSidebars() {
  loading.value = true;
  try {
    const all = await fetchContributions();
    const filtered = all
      .filter((c) => c.kind === "chat_sidebar" && c.visible && c.effective && c.enabled)
      .sort((a, b) => a.ordering - b.ordering);
    sidebarItems.value = filtered.map((c) => ({
      contribution: c,
      open: true,
      width: props.defaultWidth,
      resizable: true,
    }));
  } catch (e: any) {
    ElMessage.error("加载侧边栏扩展失败: " + (e?.message || e));
  } finally {
    loading.value = false;
  }
}

function toggleSidebar(item: ChatSidebarItem) {
  item.open = !item.open;
}

function closeAll() {
  for (const item of sidebarItems.value) {
    item.open = false;
  }
}

function openAll() {
  for (const item of sidebarItems.value) {
    item.open = true;
  }
}

function getRendererType(contribution: UIContributionSummary): "schema-ui" | "web-ui" {
  if (contribution.schemaPath || contribution.sandbox === "schema_renderer") {
    return "schema-ui";
  }
  return "web-ui";
}

let resizeState: { startX: number; startWidth: number } | null = null;

function startResize(e: MouseEvent) {
  e.preventDefault();
  e.stopPropagation();
  resizeState = { startX: e.clientX, startWidth: containerWidth.value };
  document.addEventListener("mousemove", onResize);
  document.addEventListener("mouseup", stopResize);
}

function onResize(e: MouseEvent) {
  if (!resizeState) return;
  const delta = e.clientX - resizeState.startX;
  const newWidth = resizeState.startWidth + delta;
  containerWidth.value = Math.max(props.minWidth, Math.min(props.maxWidth, newWidth));
}

function stopResize() {
  resizeState = null;
  document.removeEventListener("mousemove", onResize);
  document.removeEventListener("mouseup", stopResize);
}

onMounted(() => {
  loadSidebars();
});

onBeforeUnmount(() => {
  document.removeEventListener("mousemove", onResize);
  document.removeEventListener("mouseup", stopResize);
});
</script>

<template>
  <div class="chat-sidebar-host" :style="{ width: containerWidth + 'px' }">
    <div v-if="loading" class="chat-sidebar-host__loading">
      <span class="chat-sidebar-host__spinner"></span>
      <span>加载侧边栏...</span>
    </div>
    <template v-else-if="hasSidebars">
      <div class="chat-sidebar-host__toolbar">
        <span class="chat-sidebar-host__count">侧边栏 ({{ openItems.length }}/{{ sidebarItems.length }})</span>
        <div class="chat-sidebar-host__toolbar-actions">
          <button class="chat-sidebar-host__btn" @click="openAll">全部展开</button>
          <button class="chat-sidebar-host__btn" @click="closeAll">全部折叠</button>
        </div>
      </div>
      <div class="chat-sidebar-host__container">
        <div
          v-for="item in sidebarItems"
          :key="item.contribution.contributionId"
          class="chat-sidebar-host__panel"
          :data-open="item.open"
        >
          <div class="chat-sidebar-host__header" @click="toggleSidebar(item)">
            <span v-if="item.contribution.icon" class="chat-sidebar-host__icon">{{ item.contribution.icon }}</span>
            <span class="chat-sidebar-host__title">{{ item.contribution.title }}</span>
            <el-icon class="chat-sidebar-host__toggle">
              <Fold v-if="item.open" />
              <Expand v-else />
            </el-icon>
          </div>
          <div v-show="item.open" class="chat-sidebar-host__content">
            <SchemaUIRenderer
              v-if="getRendererType(item.contribution) === 'schema-ui'"
              :contribution="item.contribution"
              :context="context ?? {}"
              :slot-id="slotId"
            />
            <SandboxWebUIFrame
              v-else
              :contribution="item.contribution"
              :context="context ?? {}"
              :slot-id="slotId"
            />
          </div>
        </div>
      </div>
    </template>
    <div v-else class="chat-sidebar-host__empty">
      暂无侧边栏扩展
    </div>
    <div class="chat-sidebar-host__resizer" @mousedown="startResize"></div>
  </div>
</template>

<style scoped>
.chat-sidebar-host {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  background: var(--el-bg-color, #fff);
  border-right: 1px solid var(--el-border-color, #e4e7ed);
  overflow: hidden;
}

.chat-sidebar-host__loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px;
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
}

.chat-sidebar-host__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(127, 127, 127, 0.2);
  border-top-color: var(--el-color-primary, #409eff);
  border-radius: 50%;
  animation: chat-sidebar-spin 0.9s linear infinite;
}

@keyframes chat-sidebar-spin {
  to {
    transform: rotate(360deg);
  }
}

.chat-sidebar-host__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5);
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
}

.chat-sidebar-host__toolbar-actions {
  display: flex;
  gap: 6px;
}

.chat-sidebar-host__btn {
  padding: 2px 8px;
  border: 1px solid var(--el-border-color, #dcdfe6);
  border-radius: 4px;
  background: transparent;
  color: var(--el-text-color-regular, #606266);
  font-size: 11px;
  cursor: pointer;
  transition: border-color 0.2s, color 0.2s;
}

.chat-sidebar-host__btn:hover {
  border-color: var(--el-color-primary, #409eff);
  color: var(--el-color-primary, #409eff);
}

.chat-sidebar-host__container {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.chat-sidebar-host__panel {
  border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5);
}

.chat-sidebar-host__panel:last-child {
  border-bottom: none;
}

.chat-sidebar-host__header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}

.chat-sidebar-host__header:hover {
  background: var(--el-fill-color-light, #f5f7fa);
}

.chat-sidebar-host__icon {
  font-size: 14px;
  line-height: 1;
}

.chat-sidebar-host__title {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary, #303133);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-sidebar-host__toggle {
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  flex-shrink: 0;
}

.chat-sidebar-host__content {
  padding: 8px 12px 12px;
  min-height: 0;
}

.chat-sidebar-host__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
}

.chat-sidebar-host__resizer {
  position: absolute;
  top: 0;
  right: -3px;
  width: 6px;
  height: 100%;
  cursor: col-resize;
  z-index: 10;
}

.chat-sidebar-host__resizer:hover {
  background: rgba(64, 158, 255, 0.2);
}
</style>
