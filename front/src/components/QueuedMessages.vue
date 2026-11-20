<template>
  <section v-if="items.length || error" class="queue" aria-label="排队消息">
    <div v-if="items.length" class="queue-title">消息队列 <span>{{ items.length }}</span></div>
    <div v-for="(item, index) in items" :key="item.requestId" class="queue-item"
      draggable="true" tabindex="0" :aria-label="`${statusLabel(item.status)}：${item.message}`"
      @dragstart="startDrag($event, item.requestId)" @dragend="dragged = ''"
      @dragover.prevent @drop.prevent="drop(item.requestId)">
      <span class="queue-handle" aria-hidden="true">⠿</span>
      <div class="queue-message" :class="{ 'has-error': item.error }">
        <div class="queue-copy">
          <span class="queue-status">{{ statusLabel(item.status) }}</span>
          <span class="queue-text">{{ item.message }}</span>
          <span v-if="item.quote" class="queue-tag">引用 {{ item.quote.messageId }}</span>
          <span v-for="attachment in item.attachments || []" :key="attachment.name" class="queue-tag">{{ attachment.name }}</span>
        </div>
        <details v-if="item.error" class="queue-error"><summary>查看完整原始错误</summary><pre>{{ item.error }}</pre></details>
      </div>
      <div class="queue-actions">
        <button type="button" :disabled="busy || index === 0" title="上移" aria-label="上移排队消息" @click="move(index, -1)">↑</button>
        <button type="button" :disabled="busy || index === items.length - 1" title="下移" aria-label="下移排队消息" @click="move(index, 1)">↓</button>
        <button type="button" :disabled="busy || !canSteer || item.status !== 'queued'" :title="canSteer ? '将消息用于引导当前任务' : '当前没有可引导的任务'" @click="$emit('steer', item.requestId)">引导</button>
        <button type="button" :disabled="busy" title="删除排队消息" aria-label="删除排队消息" @click="$emit('remove', item.requestId)">×</button>
      </div>
    </div>
    <details v-if="error" class="queue-error"><summary>排队消息同步失败，查看原始错误</summary><pre>{{ error }}</pre></details>
  </section>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { QueuedMessage } from "@/composables/useConversationQueue";

const props = defineProps<{ items: QueuedMessage[]; busy: boolean; canSteer: boolean; error: string }>();
const emit = defineEmits<{ reorder: [ids: string[]]; remove: [id: string]; steer: [id: string] }>();
const dragged = ref("");

function statusLabel(status: string) {
  if (status === "failed") return "失败";
  if (status === "uncertain") return "状态不确定";
  if (status === "steering") return "引导确认中";
  return "等待";
}

function startDrag(event: DragEvent, id: string) {
  dragged.value = id;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
  }
}

function move(from: number, offset: number) {
  const ids = props.items.map((item) => item.requestId);
  const to = from + offset;
  if (from < 0 || to < 0 || to >= ids.length) return;
  const [id] = ids.splice(from, 1);
  ids.splice(to, 0, id);
  emit("reorder", ids);
}

function drop(target: string) {
  const from = props.items.findIndex((item) => item.requestId === dragged.value);
  const to = props.items.findIndex((item) => item.requestId === target);
  dragged.value = "";
  if (!props.busy && from >= 0 && to >= 0 && from !== to) move(from, to - from);
}
</script>

<style scoped>
.queue { margin: 0 auto 8px; max-width: 860px; padding: 8px 12px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-bg-color); color: var(--el-text-color-primary); }
.queue-title { padding: 2px 4px 7px; font-size: 12px; font-weight: 600; color: var(--el-text-color-secondary); }
.queue-title span { margin-left: 4px; }
.queue-item { display: flex; align-items: center; gap: 8px; min-height: 34px; border-top: 1px solid var(--el-border-color-lighter); }
.queue-item:focus-visible { outline: 2px solid var(--el-color-primary); outline-offset: -2px; }
.queue-handle { cursor: grab; color: var(--el-text-color-secondary); }
.queue-message { min-width: 0; flex: 1; overflow: hidden; font-size: 13px; }
.queue-copy { display: flex; align-items: center; gap: 5px; min-width: 0; overflow: hidden; white-space: nowrap; }
.queue-text { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.queue-status { flex: none; font-size: 11px; color: var(--el-text-color-secondary); }
.queue-tag { flex: none; max-width: 140px; overflow: hidden; text-overflow: ellipsis; padding: 1px 5px; border-radius: 4px; background: var(--el-fill-color-light); color: var(--el-text-color-secondary); font-size: 11px; }
.queue-actions { display: flex; align-items: center; gap: 2px; }
.queue-actions button { border: 0; border-radius: 5px; padding: 4px 6px; background: transparent; color: var(--el-text-color-secondary); cursor: pointer; }
.queue-actions button:hover:not(:disabled) { background: var(--el-fill-color-light); color: var(--el-text-color-primary); }
.queue-actions button:disabled { opacity: .35; cursor: default; }
.queue-error { color: var(--el-color-danger); font-size: 12px; white-space: normal; }
.queue-error pre { max-height: 100px; overflow: auto; white-space: pre-wrap; }
</style>
