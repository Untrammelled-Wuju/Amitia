<template>
  <aside class="workbench-dock" :class="{ 'workbench-dock--mobile': mobile }" :style="mobile ? undefined : { width: `${store.width}px` }" aria-label="对话工作区" @keydown.esc.stop="store.toggleVisible(false)">
    <div v-if="!mobile" class="resize-handle" role="separator" aria-label="调整工作区宽度" aria-orientation="vertical" tabindex="0" @pointerdown="startResize" @keydown.left.prevent="store.setWidth(store.width + 24)" @keydown.right.prevent="store.setWidth(store.width - 24)"></div>
    <header class="dock-header">
      <span class="dock-label">工作区</span>
      <div class="dock-actions">
        <button type="button" title="打开文件" aria-label="打开文件" @click="store.openTab({ kind: 'files', forceNew: true })">＋</button>
        <button type="button" title="Git 变更" aria-label="Git 变更" @click="store.openTab({ kind: 'git', forceNew: true })">⑂</button>
        <button type="button" title="网页预览" aria-label="网页预览" @click="store.openTab({ kind: 'browser', forceNew: true })">◎</button>
        <button type="button" title="任务状态" aria-label="任务状态" @click="store.openTab({ kind: 'tasks' })">☷</button>
        <button type="button" title="交互终端" aria-label="交互终端" @click="store.openTab({ kind: 'terminal', forceNew: true })">▣</button>
        <button type="button" title="关闭未固定标签" aria-label="关闭未固定标签" @click="closeUnpinned">⊟</button>
        <button type="button" title="收起工作区" aria-label="收起工作区" @click="store.toggleVisible(false)">×</button>
      </div>
    </header>
    <div class="tabs" role="tablist" aria-label="工作区标签" @keydown="onTabKeydown">
      <div v-for="tab in store.tabs" :key="tab.id" class="tab-item" :class="{ 'tab-item--active': store.activeTab?.id === tab.id }">
        <button :id="`workbench-tab-${tab.id}`" type="button" role="tab" :tabindex="store.activeTab?.id === tab.id ? 0 : -1" :aria-selected="store.activeTab?.id === tab.id" :aria-controls="`workbench-panel-${tab.id}`" @click="store.activateTab(tab.id)"><span>{{ tab.title }}</span></button>
        <button type="button" class="tab-pin" :title="tab.pinned ? '取消固定' : '固定标签'" :aria-label="`${tab.pinned ? '取消固定' : '固定'} ${tab.title}`" @click="store.setPinned(tab.id, !tab.pinned)">{{ tab.pinned ? '●' : '○' }}</button>
        <button type="button" class="tab-close" :title="tab.pinned ? '先取消固定再关闭' : '关闭标签'" :disabled="tab.pinned" :aria-label="`关闭 ${tab.title}`" @click="closeTab(tab.id)">×</button>
      </div>
    </div>
    <div v-if="store.activeTab" :id="`workbench-panel-${store.activeTab.id}`" class="tab-panel" role="tabpanel" :aria-labelledby="`workbench-tab-${store.activeTab.id}`">
      <KeepAlive :max="8"><WorkbenchResourceView :key="`${store.conversationId}:${store.activeTab.id}`" :tab="store.activeTab" /></KeepAlive>
    </div>
    <div v-else class="empty">从对话中打开文件、Git 变更或网页。</div>
  </aside>
</template>
<script setup lang="ts">
import { onBeforeUnmount } from 'vue';
import { useWorkbenchStore } from '@/stores/workbench';
import WorkbenchResourceView from './WorkbenchResourceView.vue';

defineProps<{ mobile: boolean }>();
const store = useWorkbenchStore();
function closeTab(id: string) {
  const tab = store.tabs.find(item => item.id === id);
  if (!tab || tab.pinned) return;
  store.closeTab(id);
}
function closeUnpinned() {
  store.closeAll();
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
onBeforeUnmount(stopResize);
</script>
<style scoped>
.workbench-dock{height:100%;min-width:0;max-width:calc(100% - 360px);flex-shrink:0;position:relative;display:flex;flex-direction:column;border-left:1px solid var(--surface-border);background:var(--workbench-bg);color:var(--el-text-color-primary);overflow:hidden}.resize-handle{position:absolute;width:7px;left:0;top:0;bottom:0;cursor:col-resize;z-index:5;touch-action:none}.resize-handle:hover,.resize-handle:focus-visible{background:var(--el-color-primary-light-7)}.dock-header{flex-shrink:0;display:flex;justify-content:space-between;align-items:center;gap:8px;padding:9px 10px 9px 16px;border-bottom:1px solid var(--surface-border)}.dock-label{font-size:13px;font-weight:600}.dock-actions{display:flex;gap:4px}.dock-actions button,.tab-close,.tab-pin{width:27px;height:27px;border:0;background:transparent;color:inherit;border-radius:6px;cursor:pointer;font-size:18px}.dock-actions button:hover,.tab-close:hover,.tab-pin:hover{background:var(--el-fill-color-light)}.tabs{display:flex;overflow-x:auto;align-items:center;min-height:39px;flex-shrink:0;border-bottom:1px solid var(--surface-border)}.tab-item{display:flex;align-items:center;gap:2px;max-width:220px;min-width:85px;flex-shrink:0;border-right:1px solid var(--surface-border)}.tab-item--active{background:var(--el-fill-color-light);box-shadow:inset 0 -2px 0 var(--el-color-primary)}.tab-item button[role="tab"]{border:0;color:inherit;background:none;cursor:pointer;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;padding:10px 7px;flex:1;text-align:left}.tab-close,.tab-pin{font-size:13px;width:22px;height:22px}.tab-close:disabled{opacity:.24;cursor:not-allowed}.tab-panel{flex:1;min-height:0;overflow:hidden}.empty{font-size:13px;color:var(--el-text-color-secondary);padding:20px}.workbench-dock--mobile{position:absolute;inset:0;width:100%;max-width:none;z-index:20;border:0;box-shadow:0 0 25px #0003}
</style>
