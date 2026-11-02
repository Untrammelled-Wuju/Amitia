<template>
  <aside class="workbench-dock" :class="{ 'workbench-dock--mobile': mobile }" :style="mobile ? undefined : { width: `${store.width}px` }" aria-label="对话工作区" @keydown.esc.stop="store.toggleVisible(false)">
    <div v-if="!mobile" class="resize-handle" role="separator" aria-label="调整工作区宽度" aria-orientation="vertical" tabindex="0" @pointerdown="startResize" @keydown.left.prevent="store.setWidth(store.width + 24)" @keydown.right.prevent="store.setWidth(store.width - 24)"></div>
    <header class="dock-header">
      <span class="dock-label">工作区</span>
      <div class="dock-actions">
        <button type="button" title="打开文件" aria-label="打开文件" @click="store.openTab({ kind: 'files', forceNew: true })"><el-icon><Plus /></el-icon></button>
        <button type="button" title="Git 变更" aria-label="Git 变更" @click="store.openTab({ kind: 'git', forceNew: true })"><el-icon><GitBranch /></el-icon></button>
        <button type="button" title="打开浏览器" aria-label="打开浏览器" @click="store.openTab({ kind: 'browser', forceNew: true })"><el-icon><BrowserIcon /></el-icon></button>
        <button type="button" title="任务状态" aria-label="任务状态" @click="store.openTab({ kind: 'tasks' })"><el-icon><List /></el-icon></button>
        <button type="button" title="交互终端" aria-label="交互终端" @click="store.openTab({ kind: 'terminal', forceNew: true })"><el-icon><TerminalIcon /></el-icon></button>
        <button type="button" title="关闭未固定标签" aria-label="关闭未固定标签" @click="closeUnpinned"><el-icon><Minus /></el-icon></button>
        <button type="button" title="收起工作区" aria-label="收起工作区" @click="store.toggleVisible(false)"><el-icon><Close /></el-icon></button>
      </div>
    </header>
    <div class="tabs" role="tablist" aria-label="工作区标签" @keydown="onTabKeydown">
      <div v-for="tab in store.tabs" :key="tab.id" class="tab-item" :class="{ 'tab-item--active': store.activeTab?.id === tab.id }" @contextmenu.prevent="showTabMenu($event, tab.id)">
        <button :id="`workbench-tab-${tab.id}`" type="button" role="tab" :tabindex="store.activeTab?.id === tab.id ? 0 : -1" :aria-selected="store.activeTab?.id === tab.id" :aria-controls="`workbench-panel-${tab.id}`" @click="store.activateTab(tab.id)" @auxclick.middle.prevent="closeTab(tab.id)">
          <el-icon class="tab-icon"><component :is="tabIcon(tab.kind)" /></el-icon>
          <span>{{ tab.title }}</span>
          <el-icon v-if="tab.pinned" class="tab-pinned"><CollectionTag /></el-icon>
        </button>
        <button type="button" class="tab-close" :title="tab.pinned ? '先取消固定再关闭' : '关闭标签'" :disabled="tab.pinned" :aria-label="`关闭 ${tab.title}`" @click.stop="closeTab(tab.id)"><el-icon><Close /></el-icon></button>
      </div>
    </div>
    <div v-if="store.activeTab" :id="`workbench-panel-${store.activeTab.id}`" class="tab-panel" role="tabpanel" :aria-labelledby="`workbench-tab-${store.activeTab.id}`">
      <KeepAlive :max="8"><WorkbenchResourceView :key="`${store.conversationId}:${store.activeTab.id}`" :tab="store.activeTab" /></KeepAlive>
    </div>
    <div v-else class="empty">从对话中打开文件、Git 变更或网页。</div>
    <Teleport to="body">
      <div v-if="contextMenu" ref="contextMenuElement" class="tab-context-menu" :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }" role="menu" @contextmenu.prevent>
        <button type="button" role="menuitem" @click="togglePinnedFromMenu">{{ contextMenuTab?.pinned ? '取消固定' : '固定标签' }}</button>
        <button type="button" role="menuitem" :disabled="contextMenuTab?.pinned" @click="closeFromMenu">关闭</button>
        <button type="button" role="menuitem" @click="closeOtherTabs">关闭其他</button>
        <button type="button" role="menuitem" @click="closeTabsToRight">关闭右侧标签</button>
        <div class="menu-separator"></div>
        <button type="button" role="menuitem" @click="closeUnpinnedFromMenu">关闭未固定标签</button>
      </div>
    </Teleport>
  </aside>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { Browser as BrowserIcon, Close, CollectionTag, Files, GitBranch, List, Minus, Plus, Terminal as TerminalIcon } from '@/components/icons/element-plus-icons';
import { useWorkbenchStore } from '@/stores/workbench';
import type { WorkbenchViewKind } from '@/stores/workbench';
import WorkbenchResourceView from './WorkbenchResourceView.vue';

defineProps<{ mobile: boolean }>();
const store = useWorkbenchStore();
const tabIcons: Record<WorkbenchViewKind, object> = {
  files: Files,
  file: Files,
  git: GitBranch,
  diff: GitBranch,
  browser: BrowserIcon,
  terminal: TerminalIcon,
  tasks: List,
};
const contextMenu = ref<{ x: number; y: number; tabId: string } | null>(null);
const contextMenuElement = ref<HTMLElement>();
const contextMenuTab = computed(() => store.tabs.find(tab => tab.id === contextMenu.value?.tabId) || null);
function tabIcon(kind: WorkbenchViewKind) {
  return tabIcons[kind] || Files;
}
function closeTab(id: string) {
  const tab = store.tabs.find(item => item.id === id);
  if (!tab || tab.pinned) return;
  store.closeTab(id);
  contextMenu.value = null;
}
function closeUnpinned() {
  store.closeAll();
  contextMenu.value = null;
}
function showTabMenu(event: MouseEvent, tabId: string) {
  const width = 176;
  const height = 208;
  contextMenu.value = {
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - width - 8)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - height - 8)),
    tabId,
  };
}
function togglePinnedFromMenu() {
  const tab = contextMenuTab.value;
  if (!tab) return;
  store.setPinned(tab.id, !tab.pinned);
  contextMenu.value = null;
}
function closeFromMenu() {
  const tab = contextMenuTab.value;
  if (tab) closeTab(tab.id);
}
function closeOtherTabs() {
  const keep = contextMenu.value?.tabId;
  for (const tab of [...store.tabs]) {
    if (tab.id !== keep && !tab.pinned) store.closeTab(tab.id);
  }
  contextMenu.value = null;
}
function closeTabsToRight() {
  const tabId = contextMenu.value?.tabId;
  const index = store.tabs.findIndex(tab => tab.id === tabId);
  if (index >= 0) {
    for (const tab of store.tabs.slice(index + 1)) {
      if (!tab.pinned) store.closeTab(tab.id);
    }
  }
  contextMenu.value = null;
}
function closeUnpinnedFromMenu() {
  closeUnpinned();
}
function dismissContextMenu(event: PointerEvent) {
  if (!contextMenu.value || contextMenuElement.value?.contains(event.target as Node)) return;
  contextMenu.value = null;
}
function dismissContextMenuOnEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') contextMenu.value = null;
}
let dragStart = 0;
let widthStart = 0;
let dragging = false;
function startResize(event: PointerEvent) {
  if (event.button !== 0) return;
  event.preventDefault();
  dragging = true;
  dragStart = event.clientX;
  widthStart = store.width;
  (event.target as HTMLElement).setPointerCapture?.(event.pointerId);
  window.addEventListener('pointermove', resize);
  window.addEventListener('pointerup', stopResize);
  window.addEventListener('pointercancel', stopResize);
}
function resize(event: PointerEvent) {
  if (!dragging) return;
  store.setWidth(Math.max(350, Math.min(window.innerWidth - 340, widthStart + dragStart - event.clientX)));
}
function stopResize() {
  dragging = false;
  window.removeEventListener('pointermove', resize);
  window.removeEventListener('pointerup', stopResize);
  window.removeEventListener('pointercancel', stopResize);
}
function onTabKeydown(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
  event.preventDefault();
  const tabs = store.tabs;
  if (!tabs.length) return;
  const current = tabs.findIndex((tab) => tab.id === store.activeTab?.id);
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (current + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
  store.activateTab(tabs[next].id);
  requestAnimationFrame(() => document.getElementById(`workbench-tab-${tabs[next].id}`)?.focus());
}
onMounted(() => {
  window.addEventListener('pointerdown', dismissContextMenu);
  window.addEventListener('keydown', dismissContextMenuOnEscape);
});
onBeforeUnmount(() => {
  stopResize();
  window.removeEventListener('pointerdown', dismissContextMenu);
  window.removeEventListener('keydown', dismissContextMenuOnEscape);
});
</script>
<style scoped>
.workbench-dock{height:100%;min-width:0;max-width:calc(100% - 360px);flex-shrink:0;position:relative;display:flex;flex-direction:column;border-left:1px solid var(--surface-border);background:var(--workbench-bg);color:var(--el-text-color-primary);overflow:hidden}.resize-handle{position:absolute;width:7px;left:0;top:0;bottom:0;cursor:col-resize;z-index:5;touch-action:none}.resize-handle:hover,.resize-handle:focus-visible{background:var(--el-color-primary-light-7)}.dock-header{flex-shrink:0;display:flex;justify-content:space-between;align-items:center;gap:8px;min-height:48px;padding:0 8px 0 14px;border-bottom:1px solid var(--surface-border);box-sizing:border-box}.dock-label{font-size:13px;font-weight:600;letter-spacing:.01em}.dock-actions{display:flex;align-items:center;gap:1px}.dock-actions button,.tab-close{width:28px;height:28px;border:0;background:transparent;color:var(--el-text-color-secondary);border-radius:6px;cursor:pointer;display:inline-grid;place-items:center;padding:0;font-size:15px}.dock-actions button:hover,.tab-close:hover:not(:disabled){background:var(--el-fill-color-light);color:var(--el-text-color-primary)}.tabs{display:flex;overflow-x:auto;align-items:stretch;min-height:38px;flex-shrink:0;border-bottom:1px solid var(--surface-border);scrollbar-width:thin}.tab-item{display:flex;align-items:center;max-width:220px;min-width:88px;flex-shrink:0;border-right:1px solid var(--surface-border);position:relative}.tab-item--active{background:var(--el-fill-color-light)}.tab-item--active::after{content:"";position:absolute;left:8px;right:8px;bottom:-1px;height:2px;border-radius:2px;background:var(--el-color-primary)}.tab-item button[role="tab"]{border:0;color:var(--el-text-color-secondary);background:none;cursor:pointer;min-width:0;overflow:hidden;white-space:nowrap;font-size:12px;padding:0 3px 0 9px;height:37px;flex:1;display:flex;align-items:center;gap:6px;text-align:left}.tab-item--active button[role="tab"]{color:var(--el-text-color-primary)}.tab-item button[role="tab"] span{overflow:hidden;text-overflow:ellipsis}.tab-icon{flex:none;font-size:14px}.tab-pinned{font-size:10px;flex:none}.tab-close{width:24px;height:24px;font-size:13px;margin-right:4px;flex:none}.tab-close:disabled{opacity:.24;cursor:not-allowed}.tab-panel{flex:1;min-height:0;overflow:hidden}.empty{font-size:13px;color:var(--el-text-color-secondary);padding:20px}.tab-context-menu{position:fixed;z-index:10000;width:176px;padding:5px;border:1px solid var(--surface-border);border-radius:8px;background:var(--el-bg-color-overlay,var(--workbench-bg));box-shadow:0 12px 36px #0005}.tab-context-menu button{display:block;width:100%;border:0;border-radius:5px;background:transparent;color:var(--el-text-color-primary);text-align:left;font:inherit;font-size:12px;padding:7px 9px;cursor:pointer}.tab-context-menu button:hover:not(:disabled){background:var(--el-fill-color-light)}.tab-context-menu button:disabled{opacity:.4;cursor:not-allowed}.menu-separator{height:1px;margin:4px 2px;background:var(--surface-border)}.workbench-dock--mobile{position:absolute;inset:0;width:100%;max-width:none;z-index:20;border:0;box-shadow:0 0 25px #0003}.workbench-dock--mobile .dock-header{padding-top:env(safe-area-inset-top);min-height:calc(48px + env(safe-area-inset-top))}.workbench-dock--mobile .tabs{min-height:42px}.workbench-dock--mobile .tab-item button[role="tab"]{height:41px;font-size:13px;padding-left:10px}
</style>
