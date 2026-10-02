<template>
  <div v-if="status" class="automation-indicator" :class="status.phase">
    <div class="automation-frame" aria-hidden="true"></div>
    <div class="automation-status" role="status" aria-live="polite" aria-atomic="true">
      <span class="automation-dot" aria-hidden="true"></span>
      <span class="automation-copy">
        <strong>{{ status.label }}</strong>
        <span>{{ status.detail }}<template v-if="status.count > 1"> · {{ status.count }} 项操作</template></span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { AutomationStatus } from "@/conversation/runtime/automationStatus";
defineProps<{ status: AutomationStatus | null }>();
</script>

<style scoped>
.automation-indicator {
  position: absolute;
  inset: 0;
  z-index: 20;
  pointer-events: none;
  color: var(--tp-primary);
}
.automation-frame {
  position: absolute;
  inset: 2px;
  border: 2px solid currentColor;
  border-radius: var(--radius-lg);
  box-shadow: inset 0 0 18px color-mix(in srgb, currentColor 15%, transparent);
  animation: automation-breathe 2.4s ease-in-out infinite;
}
.automation-status {
  position: absolute;
  top: 64px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 10px;
  width: max-content;
  max-width: calc(100% - 32px);
  box-sizing: border-box;
  padding: 10px 14px;
  border: 1px solid color-mix(in srgb, currentColor 24%, transparent);
  border-radius: 16px;
  background: var(--chat-surface-bg);
  box-shadow: 0 4px 18px color-mix(in srgb, var(--text-primary) 8%, transparent);
}
.automation-dot {
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  border-radius: 50%;
  background: currentColor;
}
.automation-copy { display: grid; gap: 3px; min-width: 0; }
.automation-copy strong { color: var(--text-primary); font-size: 13px; font-weight: 600; }
.automation-copy > span { color: var(--text-secondary); font-size: 11px; line-height: 1.5; }
.waiting .automation-frame, .cancelling .automation-frame { animation: none; opacity: .55; }
@keyframes automation-breathe { 0%, 100% { opacity: .6; } 50% { opacity: 1; } }
@media (prefers-reduced-motion: reduce) { .automation-frame { animation: none; } }
:global(html[data-reduce-motion="true"]) .automation-frame { animation: none; }
</style>
