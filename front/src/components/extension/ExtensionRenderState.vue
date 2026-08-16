<script setup lang="ts">
defineProps<{
  state: "loading" | "error" | "empty" | "suspended";
  detail?: string;
}>();

defineEmits<{ retry: [] }>();
</script>

<template>
  <div class="extension-render-state" :data-state="state" role="status">
    <span class="extension-render-state__title">
      {{ state === "loading" ? "加载扩展界面" : state === "error" ? "扩展界面加载失败" : state === "suspended" ? "扩展已暂停" : "暂无扩展内容" }}
    </span>
    <span v-if="detail" class="extension-render-state__detail">{{ detail }}</span>
    <button v-if="state === 'error'" type="button" @click="$emit('retry')">重试</button>
  </div>
</template>

<style scoped>
.extension-render-state { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; width: 100%; min-width: 0; padding: 10px 12px; border: 1px solid var(--plugin-surface-border); border-radius: var(--radius-sm); background: var(--plugin-muted-bg); color: var(--text-secondary); font-size: 12px; }
.extension-render-state[data-state="error"] { border-color: color-mix(in srgb, var(--ac-color-danger) 38%, var(--plugin-surface-border)); color: var(--ac-color-danger); }
.extension-render-state__title { font-weight: 600; }
.extension-render-state__detail { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.extension-render-state button { border: 1px solid currentColor; border-radius: var(--radius-xs); background: transparent; color: inherit; cursor: pointer; padding: 3px 8px; }
</style>
