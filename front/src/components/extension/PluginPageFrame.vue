<script setup lang="ts">
defineProps<{ title: string; description?: string }>();
</script>

<template>
  <section class="plugin-page-frame" data-amitia-ui="extension.page">
    <header class="plugin-page-frame__header">
      <div class="plugin-page-frame__identity">
        <h2 class="plugin-page-frame__title">{{ title }}</h2>
        <p v-if="description" class="plugin-page-frame__description">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="plugin-page-frame__actions"><slot name="actions" /></div>
    </header>
    <div class="plugin-page-frame__content"><slot /></div>
  </section>
</template>

<style scoped>
.plugin-page-frame {
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  max-width: 100%;
  min-width: 0;
  min-height: calc(100dvh - 80px);
  display: flex;
  flex-direction: column;
  padding: 20px 24px;
  color: var(--ac-color-text, inherit);
  font-family: var(--ac-font-family, inherit);
  font-size: var(--ac-font-size-base, 14px);
  line-height: 1.5;
}
:global(.content-scroll-inner:not(.no-padding)) .plugin-page-frame {
  padding: 0;
}
.plugin-page-frame__header {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.plugin-page-frame__identity {
  min-width: 0;
}
.plugin-page-frame__title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.plugin-page-frame__description {
  margin: 4px 0 0;
  color: var(--ac-color-text-secondary, inherit);
  font-size: var(--ac-font-size-sm, 13px);
}
.plugin-page-frame__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.plugin-page-frame__content {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  font-size: inherit;
}
.plugin-page-frame__content :deep(.schema-ui-renderer.is-loading) {
  position: static;
  min-height: 0;
}
.plugin-page-frame__content :deep(.schema-ui-renderer) {
  font-size: inherit;
  --el-font-size-base: var(--ac-font-size-base, 14px);
  --el-font-size-small: var(--ac-font-size-sm, 13px);
}
</style>
