<template>
  <div class="secondary-workspace">
    <aside class="secondary-sidebar">
      <header class="secondary-header">
        <router-link :to="returnTo" class="workspace-back" aria-label="返回主界面">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m10 5-7 7 7 7M3 12h18" /></svg>
        </router-link>
        <h1>{{ title }}</h1>
      </header>
      <slot name="navigation" />
    </aside>
    <main class="secondary-main"><div class="secondary-content"><slot /></div></main>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; returnTo: string }>();
</script>

<style scoped>
.secondary-workspace {
  display: grid;
  grid-template-columns: 224px minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  width: 100%;
  height: 100%;
  min-height: 0;
  color: var(--text-primary);
  background: var(--workbench-bg);
}
.secondary-sidebar { display: flex; flex-direction: column; min-height: 0; background: var(--workbench-sidebar-bg); border-right: 1px solid var(--surface-border); }
.secondary-header { display: flex; align-items: center; gap: 8px; flex-shrink: 0; padding: 20px 16px 16px; }
.secondary-header h1 { min-width: 0; margin: 0; font-size: var(--ac-font-size-xl); font-weight: 650; line-height: 1.3; letter-spacing: -0.025em; }
.workspace-back { display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; width: 32px; height: 32px; padding: 0; border-radius: var(--ac-radius-sm); color: var(--text-secondary); text-decoration: none; }
.workspace-back svg { width: 16px; height: 16px; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.workspace-back:hover { background: var(--control-hover-bg); color: var(--text-primary); }
.workspace-back:active { background: var(--control-active-bg); }
.workspace-back:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: -2px; }
.secondary-main { min-width: 0; min-height: 0; overflow: auto; overscroll-behavior: contain; }
.secondary-content { width: 100%; max-width: 1100px; margin: 0 auto; padding: 32px 40px; box-sizing: border-box; }
@media (max-width: 767px) {
  .secondary-workspace { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
  .secondary-sidebar { border-right: 0; border-bottom: 1px solid var(--surface-border); }
  .secondary-header { padding: 12px 16px; }
  .secondary-content { padding: 24px 16px; }
}
</style>
