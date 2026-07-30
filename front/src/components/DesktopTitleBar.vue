<script setup lang="ts">
import { onMounted, ref } from "vue";

const isMaximized = ref(false);

onMounted(async () => {
  document.documentElement.classList.add("amitia-desktop-shell");
  isMaximized.value = await window.electronWindowApi!.isMaximized();
});

async function handleMinimize() {
  await window.electronWindowApi!.minimize("main");
}

async function handleToggleMaximize() {
  isMaximized.value = await window.electronWindowApi!.toggleMaximize();
}

async function handleClose() {
  await window.electronWindowApi!.close("main");
}
</script>

<template>
  <div id="WindowControlButtons" class="drag">
    <div class="window-actions">
      <div
        class="icon no-drag"
        title="最小化"
        aria-label="最小化"
        @click="handleMinimize"
      >
        <svg viewBox="0 0 1024 1024">
          <path
            d="M863.7 552.5H160.3c-10.6 0-19.2-8.6-19.2-19.2v-41.7c0-10.6 8.6-19.2 19.2-19.2h703.3c10.6 0 19.2 8.6 19.2 19.2v41.7c0 10.6-8.5 19.2-19.1 19.2z"
          ></path>
        </svg>
      </div>
      <div
        class="icon no-drag"
        :title="isMaximized ? '窗口化' : '最大化'"
        :aria-label="isMaximized ? '窗口化' : '最大化'"
        @click="handleToggleMaximize"
      >
        <svg v-if="!isMaximized" viewBox="0 0 1024 1024">
          <path
            d="M770.9 923.3H253.1c-83.8 0-151.9-68.2-151.9-151.9V253.6c0-83.8 68.2-151.9 151.9-151.9h517.8c83.8 0 151.9 68.2 151.9 151.9v517.8c0 83.8-68.1 151.9-151.9 151.9zM253.1 181.7c-39.7 0-71.9 32.3-71.9 71.9v517.8c0 39.7 32.3 71.9 71.9 71.9h517.8c39.7 0 71.9-32.3 71.9-71.9V253.6c0-39.7-32.3-71.9-71.9-71.9H253.1z"
          ></path>
        </svg>
        <svg v-if="isMaximized" viewBox="0 0 1024 1024">
          <path
            d="M812.2 65H351.6c-78.3 0-142.5 61.1-147.7 138.1-77 5.1-138.1 69.4-138.1 147.7v460.6c0 81.6 66.4 148 148 148h460.6c78.3 0 142.5-61.1 147.7-138.1 77-5.1 138.1-69.4 138.1-147.7V213c0-81.6-66.4-148-148-148z m-45.8 746.3c0 50.7-41.3 92-92 92H213.8c-50.7 0-92-41.3-92-92V350.7c0-50.7 41.3-92 92-92h460.6c50.7 0 92 41.3 92 92v460.6z m137.8-137.7c0 47.3-35.8 86.3-81.8 91.4V350.7c0-81.6-66.4-148-148-148H260.2c5.1-45.9 44.2-81.8 91.4-81.8h460.6c50.7 0 92 41.3 92 92v460.7z"
          ></path>
        </svg>
      </div>
      <div
        class="icon no-drag close"
        title="关闭"
        aria-label="关闭"
        @click="handleClose"
      >
        <svg viewBox="0 0 1024 1024">
          <path
            d="M897.6 183.5L183 898.1c-7.5 7.5-19.6 7.5-27.1 0l-29.5-29.5c-7.5-7.5-7.5-19.6 0-27.1L841 126.9c7.5-7.5 19.6-7.5 27.1 0l29.5 29.5c7.5 7.4 7.5 19.6 0 27.1z"
          ></path>
          <path
            d="M183 126.9l714.7 714.7c7.5 7.5 7.5 19.6 0 27.1l-29.5 29.5c-7.5 7.5-19.6 7.5-27.1 0L126.4 183.5c-7.5-7.5-7.5-19.6 0-27.1l29.5-29.5c7.4-7.5 19.6-7.5 27.1 0z"
          ></path>
        </svg>
      </div>
    </div>
  </div>
</template>

<style scoped>
.drag {
  -webkit-app-region: drag;
}
.no-drag {
  -webkit-app-region: no-drag;
}
#WindowControlButtons {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10000;
  height: 34px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  background: var(--tp-glass-bg-strong);
  border-bottom: 1px solid var(--tp-glass-border);
  color: var(--ac-color-text);
  font:
    12px/1.2 system-ui,
    -apple-system,
    BlinkMacSystemFont,
    "Segoe UI",
    sans-serif;
  user-select: none;
  -webkit-app-region: drag;
}

:global(html[data-theme="dark"]) #WindowControlButtons {
  background: var(--tp-glass-bg-strong);
  border-bottom-color: var(--tp-glass-border);
  -webkit-backdrop-filter: blur(var(--tp-glass-blur))
    saturate(var(--tp-glass-saturate));
  backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
}
#WindowControlButtons .window-actions {
  height: 100%;
  display: flex;
  -webkit-app-region: no-drag;
}
#WindowControlButtons .icon {
  width: 46px;
  height: 100%;
  border: 0;
  border-radius: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: var(--ac-color-text-secondary);
  cursor: pointer;
  -webkit-app-region: no-drag;
}
#WindowControlButtons .icon:hover {
  background: var(--tp-control-hover);
  color: var(--ac-color-text);
}
#WindowControlButtons .icon.close:hover {
  background: var(--ac-color-danger);
  color: var(--tp-text-on-status);
}
#WindowControlButtons .icon svg {
  width: 14px;
  height: 14px;
  fill: currentColor;
}
</style>
