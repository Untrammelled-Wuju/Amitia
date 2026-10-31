<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAppStore } from "@/stores/app";
import { isDesktopShell } from "@/runtime/runtime-capabilities";
import { setWindowLayerFallback, useWindowLayerState } from "@/composables/useWindowLayer";
import AmitiaIcon from "@/components/icons/AmitiaIcon.vue";

const props = defineProps<{ transparent?: boolean }>();
const appStore = useAppStore();
const router = useRouter();
const route = useRoute();
const windowApi = window.electronWindowApi;
const desktop = isDesktopShell();
const platform = desktop ? windowApi?.platform ?? "" : "web";
const windowLayerBlocked = useWindowLayerState();
const usesNativeControls = desktop && (platform === "darwin" || platform === "win32");
const showCustomWindowActions = desktop && !usesNativeControls;
document.documentElement.dataset.desktopPlatform = platform;
document.documentElement.style.setProperty("--desktop-titlebar-height", "40px");

let stopWindowState: (() => void) | undefined;
let stopAfterEach: (() => void) | undefined;
let appearanceObserver: MutationObserver | undefined;
let overlayObserver: MutationObserver | undefined;
let appearanceProbe: HTMLSpanElement | undefined;
let documentObserver: MutationObserver | null = null;
let previousCaptionColors = "";
const historyStack: string[] = [];
let currentIndex = 0;

const isMaximized = ref(false);
const hasOnboardingWorld = ref(false);
const isFullscreen = ref(false);
const canGoBack = ref(false);
const canGoForward = ref(false);
const isChatPage = computed(() => route.path === "/chat");
const hasWorkspaceSidebar = computed(() => !appStore.sidebarCollapsed);

function applyWindowState(state: { fullscreen: boolean; maximized: boolean }) {
  isFullscreen.value = state.fullscreen;
  isMaximized.value = state.maximized;
}

function syncCaptionColors() {
  if (!appearanceProbe || !windowApi) return;
  const style = getComputedStyle(appearanceProbe);
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 1;
  const context = canvas.getContext("2d");
  if (!context) return;
  const toHex = (color: string) => {
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = color;
    context.fillRect(0, 0, 1, 1);
    return "#" + Array.from(context.getImageData(0, 0, 1, 1).data.slice(0, 3), value => value.toString(16).padStart(2, "0")).join("");
  };
  const colors = { color: toHex(style.backgroundColor), symbolColor: toHex(style.color) };
  const key = JSON.stringify(colors);
  if (key === previousCaptionColors) return;
  previousCaptionColors = key;
  void windowApi.syncCaptionColors(colors);
}

function syncOnboardingWorld() {
  hasOnboardingWorld.value = document.querySelector(".onboarding-world, .login-world") !== null;
}

function syncWindowOverlayState() {
  const active = Array.from(document.querySelectorAll<HTMLElement>(".el-overlay, .search-overlay")).some((element) => {
    const style = getComputedStyle(element);
    return style.display !== "none" && style.visibility !== "hidden";
  });
  setWindowLayerFallback(active);
}

function updateNavigationState() {
  canGoBack.value = currentIndex > 0;
  canGoForward.value = currentIndex < historyStack.length - 1;
}

onMounted(async () => {
  document.documentElement.classList.add("amitia-titlebar-shell");
  if (desktop) document.documentElement.classList.add("amitia-desktop-shell");
  syncOnboardingWorld();
  documentObserver = new MutationObserver(syncOnboardingWorld);
  documentObserver.observe(document.body, { childList: true, subtree: true });
  overlayObserver = new MutationObserver(syncWindowOverlayState);
  overlayObserver.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["class", "style"],
  });
  syncWindowOverlayState();
  stopWindowState = windowApi?.onWindowStateChanged(applyWindowState);
  if (windowApi) applyWindowState(await windowApi.getWindowState());

  if (platform === "win32") {
    appearanceProbe = document.createElement("span");
    appearanceProbe.style.cssText = "position:fixed;visibility:hidden;pointer-events:none;background:var(--workbench-sidebar-bg);color:var(--ac-color-text)";
    document.body.append(appearanceProbe);
    appearanceObserver = new MutationObserver(syncCaptionColors);
    appearanceObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["class", "style", "data-theme"] });
    appearanceObserver.observe(document.body, { attributes: true, attributeFilter: ["class", "style"] });
    appearanceObserver.observe(document.head, { childList: true, subtree: true, characterData: true });
    document.head.addEventListener("load", syncCaptionColors, true);
    syncCaptionColors();
  }

  historyStack.push(router.currentRoute.value.path);
  updateNavigationState();
  stopAfterEach = router.afterEach((to) => {
    const path = to.path;
    if (historyStack[currentIndex] !== path) {
      historyStack.splice(currentIndex + 1);
      historyStack.push(path);
      currentIndex = historyStack.length - 1;
    }
    updateNavigationState();
  });
});

onUnmounted(() => {
  document.documentElement.classList.remove("amitia-titlebar-shell");
  if (desktop) document.documentElement.classList.remove("amitia-desktop-shell");
  documentObserver?.disconnect();
  documentObserver = null;
  overlayObserver?.disconnect();
  overlayObserver = undefined;
  setWindowLayerFallback(false);
  stopWindowState?.();
  stopAfterEach?.();
  appearanceObserver?.disconnect();
  appearanceProbe?.remove();
  document.head.removeEventListener("load", syncCaptionColors, true);
});

function handleGoBack() {
  if (currentIndex > 0) {
    currentIndex--;
    router.go(-1);
  }
}

function handleGoForward() {
  if (currentIndex < historyStack.length - 1) {
    currentIndex++;
    router.go(1);
  }
}

async function handleMinimize() {
  await window.electronWindowApi?.minimize("main");
}

async function handleToggleMaximize() {
  if (!window.electronWindowApi) return;
  isMaximized.value = await window.electronWindowApi.toggleMaximize();
}

async function handleClose() {
  await window.electronWindowApi?.close("main");
}
</script>

<template>
  <div
    id="WindowControlButtons"
    class="drag"
    :class="{ 'is-onboarding': props.transparent || hasOnboardingWorld, 'is-web': !desktop, 'is-macos': platform === 'darwin', 'is-windows': platform === 'win32', 'is-fullscreen': isFullscreen, 'is-chat-page': isChatPage, 'has-workspace-sidebar': hasWorkspaceSidebar, 'is-overlay-blocked': windowLayerBlocked }"
    :inert="windowLayerBlocked || undefined"
    :aria-hidden="windowLayerBlocked ? 'true' : undefined"
  >
    <div class="window-drag-region" aria-hidden="true"></div>
    <div class="titlebar-left">
      <button
        v-if="!hasOnboardingWorld"
        type="button"
        class="sidebar-toggle-btn no-drag"
        :title="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
        :aria-label="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
        @click="appStore.toggleSidebar"
      >
        <AmitiaIcon
          :name="appStore.sidebarCollapsed ? 'panelLeft' : 'panelLeftClose'"
          :stroke-width="1.6"
        />
      </button>
      <div v-if="!hasOnboardingWorld" class="nav-buttons">
        <button
          type="button"
          class="nav-icon-btn no-drag"
          title="后退"
          aria-label="后退"
          :disabled="!canGoBack"
          @click="handleGoBack"
        >
          <AmitiaIcon name="back" :stroke-width="1.6" />
        </button>
        <button
          type="button"
          class="nav-icon-btn no-drag"
          title="前进"
          aria-label="前进"
          :disabled="!canGoForward"
          @click="handleGoForward"
        >
          <AmitiaIcon name="chevronRight" :stroke-width="1.6" />
        </button>
      </div>
    </div>

    <div id="window-chat-slot" class="window-chat-slot" :aria-hidden="!isChatPage"></div>

    <div v-if="showCustomWindowActions" class="window-actions">
      <div class="icon no-drag" title="最小化" aria-label="最小化" @click="handleMinimize">
        <AmitiaIcon name="minimize" :stroke-width="1.5" />
      </div>
      <div
        class="icon no-drag"
        :title="isMaximized ? '窗口化' : '最大化'"
        :aria-label="isMaximized ? '窗口化' : '最大化'"
        @click="handleToggleMaximize"
      >
        <AmitiaIcon
          :name="isMaximized ? 'restore' : 'maximize'"
          :stroke-width="1.5"
        />
      </div>
      <div class="icon no-drag close" title="关闭" aria-label="关闭" @click="handleClose">
        <AmitiaIcon name="close" :stroke-width="1.5" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.drag { -webkit-app-region: no-drag; }
.no-drag { -webkit-app-region: no-drag; }
#WindowControlButtons {
  position: relative;
  height: var(--desktop-titlebar-height, 40px);
  flex: none;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--workbench-sidebar-bg);
  color: var(--ac-color-text);
  font: 12px/1.2 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  user-select: none;
  -webkit-app-region: no-drag;
  --window-controls-safe-right: 8px;
}
.window-drag-region {
  position: absolute;
  inset: 0;
  z-index: 0;
  -webkit-app-region: drag;
}
#WindowControlButtons.is-onboarding {
  background: transparent;
  -webkit-backdrop-filter: none;
  backdrop-filter: none;
}
#WindowControlButtons .titlebar-left {
  position: relative;
  z-index: 2;
  height: 100%;
  display: flex;
  align-items: center;
  padding-left: 8px;
  -webkit-app-region: no-drag;
}
#WindowControlButtons.is-macos:not(.is-fullscreen) .titlebar-left { padding-left: 88px; }
#WindowControlButtons.is-windows:not(.is-fullscreen) {
  --window-controls-safe-right: 144px;
  padding-right: max(144px, calc(100vw - env(titlebar-area-width, 100vw)));
}
.window-chat-slot {
  position: absolute;
  top: 0;
  right: var(--window-controls-safe-right);
  bottom: 0;
  left: calc(var(--workspace-navigation-rail-width, 64px) + var(--workspace-sidebar-effective-width, 0px));
  z-index: 2;
  display: none;
  min-width: 0;
  box-sizing: border-box;
  -webkit-app-region: drag;
}
.window-chat-slot::before {
  position: absolute;
  top: 50%;
  left: 0;
  width: 1px;
  height: 50%;
  background: var(--surface-border);
  content: "";
  transform: translateY(-50%);
  transition: background-color 0.16s ease;
}
#WindowControlButtons.is-chat-page .window-chat-slot { display: block; }
#WindowControlButtons.is-chat-page:not(.has-workspace-sidebar) .window-chat-slot::before { display: none; }
.sidebar-toggle-btn,
.nav-icon-btn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}
.sidebar-toggle-btn:hover,
.nav-icon-btn:hover:not(:disabled) {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.sidebar-toggle-btn svg { width: 20px; height: 20px; }
.sidebar-toggle-btn + .nav-buttons { margin-left: 4px; }
.nav-buttons { display: flex; align-items: center; gap: 2px; }
.nav-icon-btn:disabled { opacity: 0.35; cursor: default; }
.nav-icon-btn svg { width: 18px; height: 18px; }
#WindowControlButtons .window-actions {
  position: relative;
  z-index: 2;
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
  fill: none;
}
#WindowControlButtons.is-overlay-blocked {
  pointer-events: none;
}
#WindowControlButtons.is-overlay-blocked .window-drag-region {
  display: none;
}
@media (max-width: 899px) {
  .window-chat-slot { display: none !important; }
}
</style>
