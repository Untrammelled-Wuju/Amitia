<template>
  <div class="general-settings">
    <h2>通用</h2>
    <p>调整对话输入方式和执行时限。</p>
    <ChatInputSettings />
    <TimeoutSettings />
  </div>
</template>

<script setup lang="ts">
import ChatInputSettings from "@/components/ChatInputSettings.vue";
import TimeoutSettings from "@/components/TimeoutSettings.vue";
</script>

<style scoped>
p { margin: 8px 0 24px; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
</style>
