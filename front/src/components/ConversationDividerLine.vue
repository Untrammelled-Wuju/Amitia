<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="conversation-divider-line">
    <span class="conversation-divider-rule"></span>
    <span class="conversation-divider-text">
      <slot>{{ text }}</slot>
    </span>
    <span class="conversation-divider-rule"></span>
  </div>
</template>

<script setup lang="ts">
defineProps<{ text?: string }>();
</script>

<style scoped>
.conversation-divider-line {
  width: min(100%, 820px);
  display: flex;
  align-items: center;
  gap: 10px;
  margin-inline: auto;
  color: var(--ac-color-text-muted);
  font-size: 11px;
}
.conversation-divider-rule {
  flex: 1 1 0;
  height: 1px;
  background: var(--ac-color-border);
}
.conversation-divider-text {
  flex: 0 1 auto;
  max-width: calc(100% * 4 / 6);
  min-width: 0;
  text-align: center;
  overflow-wrap: anywhere;
}
</style>
