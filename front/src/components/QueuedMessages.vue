<template>
  <section v-if="items.length || error" class="queue" aria-label="排队消息">
    <div v-for="item in items" :key="item.requestId" class="queue-item"
      :class="{ 'is-editing': item.requestId === editingId, 'is-expanded': isExpanded(item.requestId) }"
      draggable="true" tabindex="0" :aria-label="`${statusLabel(item.status)}：${item.message}`"
      @dragstart="startDrag($event, item.requestId)" @dragend="endDrag"
      @dragover.prevent @drop.prevent="drop(item.requestId)"
      @keydown.alt.up.prevent="moveByKey(item.requestId, -1)"
      @keydown.alt.down.prevent="moveByKey(item.requestId, 1)"
      @keydown.enter="onRowKey($event, item.requestId)"
      @keydown.space="onRowKey($event, item.requestId)">
      <div class="queue-head" @click="toggleFromRow(item.requestId)">
        <AmitiaIcon class="queue-icon" :class="`is-${item.status}`" name="timer" />
        <span class="queue-text" :class="{ 'has-error': item.error }">{{ item.message }}</span>
        <span v-if="item.status !== 'queued'" class="queue-status" :class="`is-${item.status}`">{{ statusLabel(item.status) }}</span>
        <div class="queue-actions" @click.stop>
          <button type="button" class="queue-steer" :disabled="busy || !canSteer || item.status !== 'queued'"
            :title="canSteer ? '将消息用于引导当前任务' : '当前没有可引导的任务'" @click="$emit('steer', item.requestId)">引导</button>
          <button type="button" class="queue-more" aria-label="排队消息操作" aria-haspopup="menu" title="更多操作"
            @click="openMenu($event, item)">⋯</button>
          <button type="button" class="queue-remove" title="删除排队消息" aria-label="删除排队消息"
            :disabled="busy" @click="$emit('remove', item.requestId)">×</button>
        </div>
        <button type="button" class="queue-expand" :aria-expanded="isExpanded(item.requestId)"
          :aria-label="isExpanded(item.requestId) ? '收起排队消息详情' : '展开排队消息详情'"
          :title="isExpanded(item.requestId) ? '收起详情' : '展开详情'"
          @click.stop="toggle(item.requestId)">
          <AmitiaIcon name="chevronRight" />
        </button>
      </div>
      <div class="queue-body">
        <div v-if="item.quote || (item.attachments && item.attachments.length)" class="queue-meta">
          <span v-if="item.quote" class="queue-tag">引用 {{ item.quote.messageId }}</span>
          <span v-for="attachment in item.attachments || []" :key="attachment.name" class="queue-tag">{{ attachment.name }}</span>
        </div>
        <details v-if="item.error" class="queue-error" open><summary>查看完整原始错误</summary><pre>{{ item.error }}</pre></details>
      </div>
    </div>
    <details v-if="error" class="queue-error queue-error--section"><summary>排队消息同步失败，查看原始错误</summary><pre>{{ error }}</pre></details>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { openContextMenu } from "@/components/context-menu/contextMenuStore";
import AmitiaIcon from "./icons/AmitiaIcon.vue";
import type { QueuedMessage } from "@/composables/useConversationQueue";

const props = defineProps<{ items: QueuedMessage[]; busy: boolean; canSteer: boolean; error: string; editingId?: string }>();
const emit = defineEmits<{ reorder: [ids: string[]]; remove: [id: string]; steer: [id: string]; edit: [id: string]; openSide: [id: string] }>();
const dragged = ref("");
const dragging = ref(false);
const expandedIds = ref<Set<string>>(new Set());
const lastStatus = new Map<string, string>();

watch(
  () => props.items,
  (items) => {
    const next = new Set(expandedIds.value);
    const alive = new Set(items.map((item) => item.requestId));
    for (const id of [...next]) {
      if (!alive.has(id)) next.delete(id);
    }
    for (const item of items) {
      const previous = lastStatus.get(item.requestId);
      const needsAttention = item.status === "failed" || item.status === "uncertain";
      if (needsAttention && previous !== item.status) next.add(item.requestId);
      lastStatus.set(item.requestId, item.status);
    }
    for (const id of [...lastStatus.keys()]) {
      if (!alive.has(id)) lastStatus.delete(id);
    }
    expandedIds.value = next;
  },
  { deep: true, immediate: true },
);

function isExpanded(id: string) {
  return expandedIds.value.has(id);
}

function toggle(id: string) {
  const next = new Set(expandedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  expandedIds.value = next;
}

function toggleFromRow(id: string) {
  if (dragging.value) return;
  toggle(id);
}

function onRowKey(event: KeyboardEvent, id: string) {
  if (event.target !== event.currentTarget) return;
  event.preventDefault();
  toggle(id);
}

function statusLabel(status: string) {
  if (status === "running") return "执行中";
  if (status === "failed") return "失败";
  if (status === "uncertain") return "状态不确定";
  if (status === "steering") return "引导确认中";
  if (status === "steered") return "已引导";
  return "等待";
}

function startDrag(event: DragEvent, id: string) {
  dragged.value = id;
  dragging.value = true;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
  }
}

function endDrag() {
  dragged.value = "";
  window.setTimeout(() => {
    dragging.value = false;
  }, 0);
}

function moveTo(from: number, to: number) {
  const ids = props.items.map((item) => item.requestId);
  if (from < 0 || to < 0 || to >= ids.length || from === to) return;
  const [id] = ids.splice(from, 1);
  ids.splice(to, 0, id);
  emit("reorder", ids);
}

function moveByKey(id: string, offset: number) {
  const ids = props.items.map((item) => item.requestId);
  const from = ids.indexOf(id);
  if (props.busy || from < 0) return;
  moveTo(from, from + offset);
}

function drop(target: string) {
  const ids = props.items.map((item) => item.requestId);
  const from = ids.indexOf(dragged.value);
  const to = ids.indexOf(target);
  dragged.value = "";
  if (!props.busy && from >= 0 && to >= 0) moveTo(from, to);
}

function openMenu(event: MouseEvent, item: QueuedMessage) {
  const editable = item.status === "queued";
  event.preventDefault();
  event.stopPropagation();
  openContextMenu({
    x: event.clientX,
    y: event.clientY,
    title: item.message.length > 42 ? `${item.message.slice(0, 42)}…` : item.message,
    items: [
      {
        id: "edit",
        label: "编辑消息",
        disabled: props.busy || !editable,
        action: () => emit("edit", item.requestId),
      },
      {
        id: "side",
        label: "在侧边工作区中打开",
        disabled: props.busy,
        action: () => emit("openSide", item.requestId),
      },
      {
        id: "close",
        label: "关闭排队",
        danger: true,
        disabled: props.busy,
        separatorBefore: true,
        action: () => emit("remove", item.requestId),
      },
    ],
  });
}
</script>

<style scoped>
.queue {
  position: absolute;
  left: 50%;
  bottom: calc(100% - 2px);
  z-index: 4;
  width: min(760px, calc(100% - 48px));
  max-height: min(42vh, 420px);
  overflow-y: auto;
  transform: translateX(-50%);
  padding: 3px;
  border: 1px solid var(--composer-border, var(--el-border-color-light));
  border-radius: 12px;
  background: var(--composer-bg, var(--el-bg-color));
  color: var(--text-primary, var(--el-text-color-primary));
  scrollbar-width: thin;
}
.queue-item {
  position: relative;
  border-radius: 9px;
  cursor: grab;
  transition: background-color .12s ease;
}
.queue-item + .queue-item { margin-top: 1px; }
.queue-item:hover { background: var(--control-hover-bg, var(--el-fill-color-light)); }
.queue-item.is-editing { background: color-mix(in srgb, var(--ac-color-primary, var(--el-color-primary)) 14%, transparent); }
.queue-item:focus-visible { outline: 2px solid var(--ac-color-primary, var(--el-color-primary)); outline-offset: -2px; }
.queue-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 0 4px 0 9px;
  border-radius: 9px;
}
.queue-icon {
  width: 14px;
  height: 14px;
  flex: none;
  color: var(--text-secondary, var(--el-text-color-secondary));
}
.queue-icon.is-running { color: var(--ac-color-primary, var(--el-color-primary)); }
.queue-icon.is-failed { color: var(--el-color-danger); }
.queue-icon.is-uncertain { color: var(--el-color-warning); }
.queue-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 12.5px;
  line-height: 20px;
  color: var(--text-primary, var(--el-text-color-primary));
  text-overflow: ellipsis;
  white-space: nowrap;
}
.queue-text.has-error { color: var(--text-secondary, var(--el-text-color-regular)); }
.queue-item.is-expanded .queue-text { white-space: normal; overflow-wrap: anywhere; }
.queue-status {
  flex: none;
  font-size: 10.5px;
  line-height: 16px;
  letter-spacing: .02em;
  color: var(--text-secondary, var(--el-text-color-placeholder));
}
.queue-status.is-failed { color: var(--el-color-danger); }
.queue-status.is-uncertain { color: var(--el-color-warning); }
.queue-status.is-running { color: var(--ac-color-primary, var(--el-color-primary)); }
.queue-actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 1px;
  opacity: 0;
  pointer-events: none;
  transition: opacity .12s ease;
}
.queue-item:hover .queue-actions,
.queue-item:focus-within .queue-actions { opacity: 1; pointer-events: auto; }
@media (hover: none) {
  .queue-actions { opacity: 1; pointer-events: auto; }
}
.queue-actions button {
  border: 0;
  border-radius: 6px;
  padding: 3px 6px;
  background: transparent;
  color: var(--text-secondary, var(--el-text-color-secondary));
  font-size: 11px;
  line-height: 16px;
  cursor: pointer;
}
.queue-actions button:hover:not(:disabled) { background: var(--el-fill-color); color: var(--text-primary, var(--el-text-color-primary)); }
.queue-actions button:disabled { opacity: .35; cursor: default; }
.queue-actions .queue-more { font-size: 13px; letter-spacing: 1px; }
.queue-expand {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  flex: none;
  border: 0;
  border-radius: 6px;
  padding: 0;
  background: transparent;
  color: var(--text-secondary, var(--el-text-color-secondary));
  font-size: 12px;
  cursor: pointer;
  transition: transform .15s ease, background-color .12s ease;
}
.queue-expand:hover { background: var(--el-fill-color); color: var(--text-primary, var(--el-text-color-primary)); }
.queue-item.is-expanded .queue-expand { transform: rotate(90deg); }
.queue-body { display: none; padding: 0 10px 8px 31px; }
.queue-item.is-expanded .queue-body { display: block; }
.queue-meta { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 2px; }
.queue-tag {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 5px;
  border-radius: 4px;
  background: var(--el-fill-color-light);
  color: var(--text-secondary, var(--el-text-color-secondary));
  font-size: 10px;
  line-height: 16px;
  white-space: nowrap;
}
.queue-error { color: var(--el-color-danger); font-size: 11px; white-space: normal; }
.queue-error summary { cursor: pointer; opacity: .85; }
.queue-error pre { max-height: 120px; margin: 4px 0 0; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; }
.queue-error--section { padding: 4px 9px 6px; border-top: 1px solid var(--composer-border, var(--el-border-color-lighter)); }
</style>
