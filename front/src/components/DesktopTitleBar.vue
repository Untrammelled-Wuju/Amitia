<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useAppStore } from "@/stores/app";
import { useRouter } from "vue-router";
import { ElMessageBox } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { forceCleanupSession } from "@/stores/refresh-coordinator";

const props = defineProps<{ transparent?: boolean }>();
const appStore = useAppStore();
const router = useRouter();

const isMaximized = ref(false);
const hasOnboardingWorld = ref(false);
const editMenuOpen = ref(false);
const fileMenuOpen = ref(false);
const viewMenuOpen = ref(false);
const helpMenuOpen = ref(false);
const zoomLevel = ref(100);
const isFullscreen = ref(false);
const appVersion = ref('');
let documentObserver: MutationObserver | null = null;

function toggleEditMenu() {
  fileMenuOpen.value = false;
  viewMenuOpen.value = false;
  helpMenuOpen.value = false;
  editMenuOpen.value = !editMenuOpen.value;
}

function handleUndo() {
  document.dispatchEvent(new CustomEvent('global-undo'));
  editMenuOpen.value = false;
}

function handleRedo() {
  document.dispatchEvent(new CustomEvent('global-redo'));
  editMenuOpen.value = false;
}

function handleCut() {
  document.execCommand('cut');
  editMenuOpen.value = false;
}

function handleCopy() {
  document.execCommand('copy');
  editMenuOpen.value = false;
}

function handlePaste() {
  document.execCommand('paste');
  editMenuOpen.value = false;
}

function handleDelete() {
  const activeEl = document.activeElement as HTMLInputElement | HTMLTextAreaElement | null;
  if (activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA')) {
    const start = activeEl.selectionStart ?? 0;
    const end = activeEl.selectionEnd ?? 0;
    if (start !== end) {
      activeEl.setRangeText('', start, end, 'select');
    } else if (start > 0) {
      activeEl.setRangeText('', start - 1, start, 'select');
    }
  } else {
    document.execCommand('delete');
  }
  editMenuOpen.value = false;
}

function handleSelectAll() {
  const activeEl = document.activeElement as HTMLInputElement | HTMLTextAreaElement | null;
  if (activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA')) {
    activeEl.select();
  } else {
    document.execCommand('selectAll');
  }
  editMenuOpen.value = false;
}

function handleSettings() {
  router.push('/settings');
  editMenuOpen.value = false;
}

function handleGlobalShortcut(event: KeyboardEvent) {
  const ctrl = event.ctrlKey || event.metaKey;
  if (!ctrl) return;
  const key = event.key.toLowerCase();
  if (key === 'z' && !event.shiftKey) {
    event.preventDefault();
    handleUndo();
  } else if ((key === 'z' && event.shiftKey) || key === 'y') {
    event.preventDefault();
    handleRedo();
  } else if (key === 'x') {
    event.preventDefault();
    handleCut();
  } else if (key === 'c') {
    event.preventDefault();
    handleCopy();
  } else if (key === 'v') {
    event.preventDefault();
    handlePaste();
  } else if (key === 'a') {
    event.preventDefault();
    handleSelectAll();
  }
}

function toggleFileMenu() {
  editMenuOpen.value = false;
  viewMenuOpen.value = false;
  helpMenuOpen.value = false;
  fileMenuOpen.value = !fileMenuOpen.value;
}

function toggleViewMenu() {
  editMenuOpen.value = false;
  fileMenuOpen.value = false;
  helpMenuOpen.value = false;
  viewMenuOpen.value = !viewMenuOpen.value;
}

async function handleZoomIn() {
  if (window.amitiaDesktop?.zoomIn) {
    await window.amitiaDesktop.zoomIn();
    await syncZoomLevel();
  }
  viewMenuOpen.value = false;
}

async function handleZoomOut() {
  if (window.amitiaDesktop?.zoomOut) {
    await window.amitiaDesktop.zoomOut();
    await syncZoomLevel();
  }
  viewMenuOpen.value = false;
}

async function handleZoomReset() {
  if (window.amitiaDesktop?.zoomReset) {
    await window.amitiaDesktop.zoomReset();
    await syncZoomLevel();
  }
  viewMenuOpen.value = false;
}

async function syncZoomLevel() {
  if (window.amitiaDesktop?.getZoomFactor) {
    const factor = await window.amitiaDesktop.getZoomFactor();
    zoomLevel.value = Math.round(factor * 100);
  }
}

async function handleFullscreen() {
  if (!document.fullscreenElement) {
    await document.documentElement.requestFullscreen();
    isFullscreen.value = true;
  } else {
    await document.exitFullscreen();
    isFullscreen.value = false;
  }
  viewMenuOpen.value = false;
}

function toggleHelpMenu() {
  editMenuOpen.value = false;
  fileMenuOpen.value = false;
  viewMenuOpen.value = false;
  helpMenuOpen.value = !helpMenuOpen.value;
}

function handleAbout() {
  router.push('/settings/about');
  helpMenuOpen.value = false;
}

async function handleCheckUpdate() {
  helpMenuOpen.value = false;
  try {
    await window.amitiaDesktop?.checkUpdate();
  } catch (e) {
    console.error('检查更新失败:', e);
  }
}

async function handleFileClose() {
  fileMenuOpen.value = false;
  await window.electronWindowApi!.close("main");
}

async function handleLogout() {
  fileMenuOpen.value = false;
  try {
    await ElMessageBox.confirm("退出后需要重新登录，确定继续吗？", "注销", {
      confirmButtonText: "注销",
      cancelButtonText: "取消",
      type: "warning",
      confirmButtonClass: "el-button--danger",
    });
  } catch {
    return;
  }
  try {
    await apiClient.post("/api/auth/logout");
  } catch {}
  forceCleanupSession();
  await router.replace("/login");
}

async function handleExit() {
  fileMenuOpen.value = false;
  try {
    await ElMessageBox.confirm("确定要退出应用吗？", "退出", {
      confirmButtonText: "退出",
      cancelButtonText: "取消",
      type: "warning",
      confirmButtonClass: "el-button--danger",
    });
  } catch {
    return;
  }
  if (window.amitiaDesktop?.quitApp) {
    await window.amitiaDesktop.quitApp();
  }
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement;
  if (!target.closest('.edit-menu-container')) {
    editMenuOpen.value = false;
  }
  if (!target.closest('.file-menu-container')) {
    fileMenuOpen.value = false;
  }
  if (!target.closest('.view-menu-container')) {
    viewMenuOpen.value = false;
  }
  if (!target.closest('.help-menu-container')) {
    helpMenuOpen.value = false;
  }
}

function handleFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement;
}

function syncOnboardingWorld() {
  hasOnboardingWorld.value =
    document.querySelector(".onboarding-world, .login-world") !== null;
}

onMounted(async () => {
  document.documentElement.classList.add("amitia-desktop-shell");
  syncOnboardingWorld();
  documentObserver = new MutationObserver(syncOnboardingWorld);
  documentObserver.observe(document.body, { childList: true, subtree: true });
  isMaximized.value = await window.electronWindowApi!.isMaximized();
  document.addEventListener('click', handleClickOutside);
  document.addEventListener('keydown', handleGlobalShortcut);
  document.addEventListener('fullscreenchange', handleFullscreenChange);
  try {
    appVersion.value = await window.amitiaDesktop?.getVersion() ?? '';
  } catch {}
  await syncZoomLevel();
});

onUnmounted(() => {
  documentObserver?.disconnect();
  documentObserver = null;
  document.removeEventListener('click', handleClickOutside);
  document.removeEventListener('keydown', handleGlobalShortcut);
  document.removeEventListener('fullscreenchange', handleFullscreenChange);
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
  <div
    id="WindowControlButtons"
    class="drag"
    :class="{ 'is-onboarding': props.transparent || hasOnboardingWorld }"
  >
    <div class="titlebar-left">
      <button
        v-if="!hasOnboardingWorld"
        type="button"
        class="sidebar-toggle-btn no-drag"
        :title="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
        :aria-label="appStore.sidebarCollapsed ? '展开导航' : '收起导航'"
        @click="appStore.toggleSidebar"
      >
        <svg v-if="appStore.sidebarCollapsed" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect x="3.5" y="4" width="17" height="16" rx="3" stroke="currentColor" stroke-width="1.25"/>
          <path d="M9 4.8V19.2" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
          <path d="M12.6 8.9L15.4 12L12.6 15.1" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect x="3.5" y="4" width="17" height="16" rx="3" stroke="currentColor" stroke-width="1.25"/>
          <path d="M9 4.8V19.2" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
          <path d="M15.4 8.9L12.6 12L15.4 15.1" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
      <div v-if="!hasOnboardingWorld" class="file-menu-container">
        <button
          type="button"
          class="text-btn no-drag"
          title="文件"
          aria-label="文件"
          @click.stop="toggleFileMenu"
        >
          文件
        </button>
        <div v-if="fileMenuOpen" class="file-menu-card">
          <button type="button" class="edit-menu-item" @click="handleLogout">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M15 4H18C19.1046 4 20 4.89543 20 6V18C20 19.1046 19.1046 20 18 20H15" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M11 16L15 12L11 8" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M15 12H4" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">注销</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleFileClose">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M7 7L17 17" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M17 7L7 17" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">关闭窗口</span>
          </button>
          <div class="menu-divider"></div>
          <button type="button" class="edit-menu-item edit-menu-item--danger" @click="handleExit">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M9 21H5C3.89543 21 3 20.1046 3 19V5C3 3.89543 3.89543 3 5 3H9" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M16 17L21 12L16 7" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M21 12H9" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">关闭程序</span>
          </button>
        </div>
      </div>
      <div v-if="!hasOnboardingWorld" class="edit-menu-container">
        <button
          type="button"
          class="text-btn no-drag"
          title="编辑"
          aria-label="编辑"
          @click.stop="toggleEditMenu"
        >
          编辑
        </button>
        <div v-if="editMenuOpen" class="edit-menu-card">
          <button type="button" class="edit-menu-item" @click="handleUndo">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M9.5 8.5H17C18.1046 8.5 19 9.39543 19 10.5V11.5C19 12.6046 18.1046 13.5 17 13.5H12" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M11 10.5L8 13.5L11 16.5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span class="menu-item-label">撤销</span>
            <span class="menu-item-shortcut">Ctrl+Z</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleRedo">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M14.5 8.5H7C5.89543 8.5 5 9.39543 5 10.5V11.5C5 12.6046 5.89543 13.5 7 13.5H12" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M13 10.5L16 13.5L13 16.5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span class="menu-item-label">重做</span>
            <span class="menu-item-shortcut">Ctrl+Y</span>
          </button>
          <div class="menu-divider"></div>
          <button type="button" class="edit-menu-item" @click="handleCut">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="7" cy="7" r="2.5" stroke="currentColor" stroke-width="1.35"/>
              <circle cx="17" cy="7" r="2.5" stroke="currentColor" stroke-width="1.35"/>
              <path d="M5 19L12 9L19 19" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M9 7L15 7" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">剪切</span>
            <span class="menu-item-shortcut">Ctrl+X</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleCopy">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="8" y="8" width="12" height="12" rx="2" stroke="currentColor" stroke-width="1.35"/>
              <path d="M16 8V6C16 4.89543 15.1046 4 14 4H6C4.89543 4 4 4.89543 4 6V14C4 15.1046 4.89543 16 6 16H8" stroke="currentColor" stroke-width="1.35"/>
            </svg>
            <span class="menu-item-label">复制</span>
            <span class="menu-item-shortcut">Ctrl+C</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handlePaste">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="6" y="4" width="12" height="16" rx="2" stroke="currentColor" stroke-width="1.35"/>
              <path d="M9 4H15V3C15 2.44772 14.5523 2 14 2H10C9.44772 2 9 2.44772 9 3V4Z" stroke="currentColor" stroke-width="1.35"/>
              <path d="M9 11H15" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M9 15H12" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">粘贴</span>
            <span class="menu-item-shortcut">Ctrl+V</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleDelete">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M5 7H19" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M10 11V17" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M14 11V17" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M9 7V5C9 4.44772 9.44772 4 10 4H14C14.5523 4 15 4.44772 15 5V7" stroke="currentColor" stroke-width="1.35"/>
              <path d="M7 7L8.5 19C8.5 19.5523 8.94772 20 9.5 20H14.5C15.0523 20 15.5 19.5523 15.5 19L17 7" stroke="currentColor" stroke-width="1.35"/>
            </svg>
            <span class="menu-item-label">删除</span>
            <span class="menu-item-shortcut">Del</span>
          </button>
          <div class="menu-divider"></div>
          <button type="button" class="edit-menu-item" @click="handleSelectAll">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="5" y="5" width="14" height="14" rx="2" stroke="currentColor" stroke-width="1.35"/>
              <path d="M9 5V3" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M15 5V3" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M9 21V19" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M15 21V19" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M3 9H5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M3 15H5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M19 9H21" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M19 15H21" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">全选</span>
            <span class="menu-item-shortcut">Ctrl+A</span>
          </button>
          <div class="menu-divider"></div>
          <button type="button" class="edit-menu-item" @click="handleSettings">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.35"/>
              <path d="M12 2V5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M12 19V22" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M4.93 4.93L7.05 7.05" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M16.95 16.95L19.07 19.07" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M2 12H5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M19 12H22" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M4.93 19.07L7.05 16.95" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M16.95 7.05L19.07 4.93" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">设置</span>
            <span class="menu-item-shortcut">,</span>
          </button>
        </div>
      </div>
      <div class="view-menu-container">
        <button
          type="button"
          class="text-btn no-drag"
          title="视图"
          aria-label="视图"
          @click.stop="toggleViewMenu"
        >
          视图
        </button>
        <div v-if="viewMenuOpen" class="view-menu-card">
          <button type="button" class="edit-menu-item" @click="handleZoomIn">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="1.35"/>
              <path d="M11 8V14" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M8 11H14" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M16 16L20 20" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span class="menu-item-label">放大</span>
            <span class="menu-item-shortcut">Ctrl++</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleZoomOut">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="1.35"/>
              <path d="M8 11H14" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M16 16L20 20" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span class="menu-item-label">缩小</span>
            <span class="menu-item-shortcut">Ctrl+-</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleZoomReset">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.35"/>
              <path d="M9 9H15" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M9 12H15" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M9 15H12" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">实际大小</span>
            <span class="menu-item-shortcut">Ctrl+0</span>
          </button>
          <div class="menu-divider"></div>
          <button type="button" class="edit-menu-item" @click="handleFullscreen">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M4 9V5C4 4.44772 4.44772 4 5 4H9" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M20 9V5C20 4.44772 19.5523 4 19 4H15" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M4 15V19C4 19.5523 4.44772 20 5 20H9" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M20 15V19C20 19.5523 19.5523 20 19 20H15" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span class="menu-item-label">{{ isFullscreen ? '退出全屏' : '全屏' }}</span>
            <span class="menu-item-shortcut">F11</span>
          </button>
        </div>
      </div>
      <div class="help-menu-container">
        <button
          type="button"
          class="text-btn no-drag"
          title="帮助"
          aria-label="帮助"
          @click.stop="toggleHelpMenu"
        >
          帮助
        </button>
        <div v-if="helpMenuOpen" class="help-menu-card">
          <button type="button" class="edit-menu-item" @click="handleAbout">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.35"/>
              <path d="M12 8V8.5" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
              <path d="M12 11V16" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">关于</span>
          </button>
          <button type="button" class="edit-menu-item" @click="handleCheckUpdate">
            <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M4 12C4 7.58172 7.58172 4 12 4C16.4183 4 20 7.58172 20 12C20 16.4183 16.4183 20 12 20C9.5 20 7.3 19 5.8 17.3" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M12 8V12L14 14" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M4 12H2" stroke="currentColor" stroke-width="1.35" stroke-linecap="round"/>
            </svg>
            <span class="menu-item-label">检查更新</span>
          </button>
        </div>
      </div>
    </div>
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
  justify-content: space-between;
  background: var(--workbench-sidebar-bg);
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

:global(html[data-theme="dark"]) #WindowControlButtons:not(.is-onboarding) {
  background: var(--workbench-sidebar-bg);
  -webkit-backdrop-filter: blur(var(--tp-glass-blur))
    saturate(var(--tp-glass-saturate));
  backdrop-filter: blur(var(--tp-glass-blur)) saturate(var(--tp-glass-saturate));
}

#WindowControlButtons.is-onboarding {
  background: transparent;
  -webkit-backdrop-filter: none;
  backdrop-filter: none;
}
#WindowControlButtons .titlebar-left {
  height: 100%;
  display: flex;
  align-items: center;
  padding-left: 8px;
  -webkit-app-region: no-drag;
}
.sidebar-toggle-btn {
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
.sidebar-toggle-btn:hover {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.sidebar-toggle-btn svg {
  width: 20px;
  height: 20px;
}
.sidebar-toggle-btn + .sidebar-toggle-btn,
.sidebar-toggle-btn + .file-menu-container,
.sidebar-toggle-btn + .edit-menu-container,
.sidebar-toggle-btn + .view-menu-container,
.sidebar-toggle-btn + .help-menu-container {
  margin-left: 4px;
}
.text-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 28px;
  padding: 0 12px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
}
.text-btn:hover {
  background: var(--workbench-sidebar-hover);
  color: var(--text-primary);
}
.text-btn + .text-btn,
.text-btn + .sidebar-toggle-btn,
.text-btn + .file-menu-container,
.text-btn + .edit-menu-container,
.text-btn + .view-menu-container,
.text-btn + .help-menu-container {
  margin-left: 4px;
}
.file-menu-container {
  position: relative;
}
.file-menu-card {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  min-width: 160px;
  padding: 6px;
  border-radius: 10px;
  background: var(--workbench-sidebar-bg);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 1px rgba(0, 0, 0, 0.06);
  z-index: 10001;
}
.edit-menu-container {
  position: relative;
}
.view-menu-container {
  position: relative;
}
.view-menu-card {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  min-width: 170px;
  padding: 6px;
  border-radius: 10px;
  background: var(--workbench-sidebar-bg);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 1px rgba(0, 0, 0, 0.06);
  z-index: 10001;
}
.help-menu-container {
  position: relative;
}
.help-menu-card {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  min-width: 150px;
  padding: 6px;
  border-radius: 10px;
  background: var(--workbench-sidebar-bg);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 1px rgba(0, 0, 0, 0.06);
  z-index: 10001;
}
.edit-menu-card {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  min-width: 140px;
  padding: 6px;
  border-radius: 10px;
  background: var(--workbench-sidebar-bg);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 1px rgba(0, 0, 0, 0.06);
  z-index: 10001;
}
.edit-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-primary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}
.edit-menu-item:hover {
  background: var(--workbench-sidebar-hover);
}
.edit-menu-item svg {
  width: 18px;
  height: 18px;
  color: var(--text-muted);
  flex-shrink: 0;
}
.menu-item-label {
  flex: 1;
}
.menu-item-shortcut {
  font-size: 11px;
  color: var(--text-muted);
  opacity: 0.7;
}
.menu-divider {
  height: 1px;
  margin: 4px 8px;
  background: var(--workbench-sidebar-hover);
}
.edit-menu-item--danger {
  color: var(--ac-color-danger, #e5484d);
}
.edit-menu-item--danger svg {
  color: var(--ac-color-danger, #e5484d);
}
.edit-menu-item--danger:hover {
  background: rgba(229, 72, 77, 0.1);
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
