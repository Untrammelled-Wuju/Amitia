<template>
  <div class="bubble-content" :class="{ 'bubble-content--user': message.role === 'user' }">
    <div v-for="item in items" :key="item.key" class="bubble-entry">
      <slot name="avatar" />
      <div class="bubble-entry-body">
      <slot name="identity" />
      <div class="message-piece" :data-bubble-kind="item.kind === 'block' ? item.block.kind : item.kind">
      <MarkdownContent v-if="item.kind === 'text'" :source="item.content" :streaming="false" :citation-ids="citationIds" @citation="emit('citation', $event)" />
      <AmitiaThinkingBlock v-else-if="item.kind === 'thinking'" :content="item.content" :state="item.complete ? 'completed' : 'streaming'" :duration="item.duration" />
      <AssistantTurnTimeline v-else-if="item.kind === 'turn'" :turn="item.turn" :citation-ids="citationIds" @citation="emit('citation', $event)" />
      <RichBlockRenderer v-else :block="item.block" />
      </div>
      </div>
    </div>
    <div v-if="pending && !items.some(item => item.kind === 'thinking' && !item.complete)" class="bubble-entry">
      <slot name="avatar" />
      <div class="bubble-entry-body">
      <slot name="identity" />
      <div class="message-piece message-piece--pending" role="status">正在回复…</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { AIMessageData, AssistantTurnData } from "./types";
import { projectBubbleContent } from "./bubbleMessageProjection";
import MarkdownContent from "./markdown/MarkdownContent.vue";
import AmitiaThinkingBlock from "./blocks/AmitiaThinkingBlock.vue";
import AssistantTurnTimeline from "./AssistantTurnTimeline.vue";
import RichBlockRenderer from "./blocks/RichBlockRenderer.vue";

const props = defineProps<{ message: AIMessageData; turn?: AssistantTurnData | null; citationIds?: string[]; pending?: boolean }>();
const emit = defineEmits<{ citation: [id: string] }>();
const items = computed(() => projectBubbleContent(props.message, props.turn));
</script>

<style scoped>
.bubble-content { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; min-width: 0; }
.bubble-content--user { align-items: flex-end; }
.bubble-entry { display: flex; align-items: flex-start; gap: 8px; max-width: 100%; min-width: 0; }
.bubble-entry-body { min-width: 0; max-width: 100%; }
.message-piece { box-sizing: border-box; max-width: 100%; min-width: 0; padding: 11px 14px; border: 1px solid var(--ac-color-border-light); border-radius: 4px 16px 16px 16px; background: var(--ac-color-surface); color: var(--ac-color-text); overflow-wrap: anywhere; }
.bubble-content--user .message-piece { border: 0; background: var(--ac-color-primary-bg); border-radius: 16px 4px 16px 16px; }
.message-piece--pending { color: var(--ac-color-text-secondary); font-size: 13px; }
.message-piece :deep(audio), .message-piece :deep(video) { max-width: 100%; }
</style>
