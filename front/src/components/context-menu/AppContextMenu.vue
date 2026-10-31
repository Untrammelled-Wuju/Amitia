<template>
  <Teleport to="body">
    <Transition name="context-menu-fade">
      <div
        v-if="contextMenuState.visible"
        ref="menuRef"
        class="app-context-menu"
        :style="{ left: `${contextMenuState.x}px`, top: `${contextMenuState.y}px` }"
        role="menu"
        aria-label="右键菜单"
        data-amitia-context-menu
        @contextmenu.prevent.stop
        @mousedown.stop
        @click.stop
      >
        <div v-if="contextMenuState.title" class="context-menu-title">{{ contextMenuState.title }}</div>

        <template v-for="item in contextMenuState.items" :key="item.id">
          <div v-if="item.separatorBefore" class="context-menu-divider"></div>
          <div
            class="context-menu-group"
            :class="{ 'opens-left': submenuSide === 'left' }"
            @mouseenter="openSubmenu(item, $event)"
            @mouseleave="scheduleCloseSubmenu(item)"
          >
            <button
              type="button"
              class="context-menu-item"
              :class="{ 'is-danger': item.danger, 'has-children': item.children?.length }"
              :disabled="item.disabled"
              role="menuitem"
              @click="activateItem(item)"
            >
              <component :is="item.icon" v-if="item.icon" class="context-menu-icon" />
              <span class="context-menu-label">{{ item.label }}</span>
              <span v-if="item.shortcut" class="context-menu-shortcut">{{ item.shortcut }}</span>
              <span v-else-if="item.children?.length" class="context-menu-chevron">›</span>
            </button>
            <div
              v-if="item.children?.length && activeSubmenu === item.id"
              class="context-submenu"
              :style="{ maxHeight: `${submenuMaxHeight}px` }"
              role="menu"
            >
              <button
                v-for="child in item.children"
                :key="child.id"
                type="button"
                class="context-menu-item"
                :class="{ 'is-danger': child.danger }"
                :disabled="child.disabled"
                role="menuitem"
                @click="runAction(child.action)"
              >
                <component :is="child.icon" v-if="child.icon" class="context-menu-icon" />
                <span class="context-menu-label">{{ child.label }}</span>
              </button>
            </div>
          </div>
        </template>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  Brush,
  CopyDocument,
  DocumentCopy,
  Download,
  Link,
  Picture,
  View,
} from "@element-plus/icons-vue";
import { isDesktopShell } from "@/runtime/runtime-capabilities";
import { useTheme } from "@/composables/useTheme";
import {
  closeContextMenu,
  contextMenuState,
  openContextMenu,
  setContextMenuPosition,
  type ContextMenuAction,
} from "./contextMenuStore";

const menuRef = ref<HTMLElement | null>(null);
const activeSubmenu = ref("");
const submenuSide = ref<"left" | "right">("right");
const submenuMaxHeight = ref(420);
let submenuTimer: ReturnType<typeof setTimeout> | null = null;
const theme = useTheme();

function isEditable(target: Element | null): target is HTMLElement {
  return Boolean(
    target &&
      (target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        (target as HTMLElement).isContentEditable),
  );
}

async function copyText(value: string, message = "已复制") {
  if (!value) return;
  try {
    await navigator.clipboard.writeText(value);
    ElMessage.success(message);
  } catch (error) {
  console.error("[original-error]", error);
const textarea = document.createElement("textarea");
    textarea.value = value;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    textarea.remove();
    ElMessage.success(message);
  }
}

async function pasteText(target: Element | null) {
  if (!isEditable(target)) return;
  try {
    const text = await navigator.clipboard.readText();
    target.focus();
    if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
      const start = target.selectionStart ?? target.value.length;
      const end = target.selectionEnd ?? start;
      target.setRangeText(text, start, end, "end");
    } else {
      document.execCommand("insertText", false, text);
    }
    target.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertFromPaste", data: text }));
  } catch (error) {
  console.error("[original-error]", error);
ElMessage.warning(error instanceof Error ? error.message : String(error));
  }
}

function buildGenericActions(target: Element | null): ContextMenuAction[] {
  const actions: ContextMenuAction[] = [];
  const selection = window.getSelection()?.toString().trim() || "";
  if (selection) {
    actions.push({
      id: "copy-selection",
      label: "复制",
      icon: CopyDocument,
      shortcut: "Ctrl+C",
      action: () => copyText(selection, "已复制选中内容"),
    });
  }
  if (isEditable(target) || target?.closest("[contenteditable='true']")) {
    actions.push({
      id: "paste",
      label: "粘贴",
      icon: DocumentCopy,
      shortcut: "Ctrl+V",
      action: () => pasteText(target),
    });
  }

  const anchor = target?.closest("a[href]") as HTMLAnchorElement | null;
  if (anchor?.href) {
    const url = anchor.href;
    actions.push(
      {
        id: "open-link",
        label: "在新窗口打开链接",
        icon: View,
        separatorBefore: actions.length > 0,
        action: () => {
          window.open(url, "_blank", "noopener,noreferrer");
        },
      },
      {
        id: "navigate-link",
        label: "转到链接",
        icon: Link,
        action: () => {
          window.open(url, "_self");
        },
      },
      {
        id: "copy-link",
        label: "复制链接地址",
        icon: Link,
        action: () => copyText(url, "链接已复制"),
      },
    );
  }

  const image = target?.closest("img[src]") as HTMLImageElement | null;
  if (image?.src) {
    const url = image.src;
    actions.push(
      {
        id: "save-image",
        label: "保存图片",
        icon: Download,
        separatorBefore: actions.length > 0,
        action: () => {
          const anchor = document.createElement("a");
          anchor.href = url;
          anchor.download = image.alt || "image";
          anchor.click();
        },
      },
      {
        id: "open-image",
        label: "在新窗口打开图片",
        icon: Picture,
        action: () => {
          window.open(url, "_blank", "noopener,noreferrer");
        },
      },
      {
        id: "copy-image-url",
        label: "复制图片地址",
        icon: Link,
        action: () => copyText(url, "图片地址已复制"),
      },
    );
  }

  actions.push({
    id: "toggle-theme",
    label: theme.resolvedMode.value === "dark" ? "切换为浅色模式" : "切换为深色模式",
    icon: Brush,
    separatorBefore: actions.length > 0,
    action: () => theme.toggleLightDark(),
  });
  return actions;
}

function handleContextMenu(event: MouseEvent) {
  if (!isDesktopShell() || event.defaultPrevented) return;
  const target = event.target instanceof Element ? event.target : null;
  if (target?.closest("[data-amitia-context-menu]")) return;
  event.preventDefault();
  openContextMenu({
    x: event.clientX,
    y: event.clientY,
    items: buildGenericActions(target),
  });
}

function openSubmenu(item: ContextMenuAction, event: MouseEvent) {
  if (!item.children?.length || item.disabled) return;
  if (submenuTimer) clearTimeout(submenuTimer);
  activeSubmenu.value = item.id;
  const group = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
  const menu = menuRef.value;
  if (!group || !menu) return;
  const groupRect = group.getBoundingClientRect();
  const menuRect = menu.getBoundingClientRect();
  submenuSide.value = menuRect.right + 218 > window.innerWidth - 8 ? "left" : "right";
  submenuMaxHeight.value = Math.max(120, Math.min(420, window.innerHeight - groupRect.top - 8));
}

function scheduleCloseSubmenu(item: ContextMenuAction) {
  if (!item.children?.length) return;
  submenuTimer = setTimeout(() => {
    if (activeSubmenu.value === item.id) activeSubmenu.value = "";
  }, 160);
}

async function runAction(action?: () => void | Promise<void>) {
  if (!action) return;
  closeContextMenu();
  try {
    await action();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  }
}

function activateItem(item: ContextMenuAction) {
  if (item.disabled || item.children?.length) return;
  void runAction(item.action);
}

function clampPosition() {
  const menu = menuRef.value;
  if (!menu) return;
  const margin = 8;
  const width = menu.offsetWidth;
  const height = menu.offsetHeight;
  let x = contextMenuState.x;
  let y = contextMenuState.y;
  if (x + width > window.innerWidth - margin) x = Math.max(margin, window.innerWidth - width - margin);
  if (y + height > window.innerHeight - margin) y = Math.max(margin, window.innerHeight - height - margin);
  setContextMenuPosition(x, y);
}

function closeOnOutside(event: MouseEvent) {
  const target = event.target;
  if (!(target instanceof Node)) return;
  if (menuRef.value?.contains(target)) return;
  closeContextMenu();
}

function closeOnEscape(event: KeyboardEvent) {
  if (event.key === "Escape") closeContextMenu();
}

watch(
  () => contextMenuState.visible,
  async (visible) => {
    activeSubmenu.value = "";
    if (!visible) return;
    await nextTick();
    clampPosition();
  },
);

onMounted(() => {
  document.addEventListener("contextmenu", handleContextMenu);
  document.addEventListener("mousedown", closeOnOutside);
  document.addEventListener("click", closeOnOutside);
  document.addEventListener("wheel", closeContextMenu, { passive: true });
  document.addEventListener("keydown", closeOnEscape);
  window.addEventListener("blur", closeContextMenu);
  window.addEventListener("resize", closeContextMenu);
});

onUnmounted(() => {
  if (submenuTimer) clearTimeout(submenuTimer);
  document.removeEventListener("contextmenu", handleContextMenu);
  document.removeEventListener("mousedown", closeOnOutside);
  document.removeEventListener("click", closeOnOutside);
  document.removeEventListener("wheel", closeContextMenu);
  document.removeEventListener("keydown", closeOnEscape);
  window.removeEventListener("blur", closeContextMenu);
  window.removeEventListener("resize", closeContextMenu);
});
</script>

<style scoped>
.app-context-menu {
  position: fixed;
  z-index: 30000;
  width: 200px;
  padding: 6px;
  border: 1px solid var(--surface-border);
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface-bg-elevated) 96%, transparent);
  box-shadow: 0 16px 44px rgba(0, 0, 0, 0.28);
  color: var(--text-primary);
  backdrop-filter: blur(14px);
}
.context-menu-fade-enter-active,
.context-menu-fade-leave-active {
  transition: opacity 0.14s ease, transform 0.14s ease;
}
.context-menu-fade-enter-from,
.context-menu-fade-leave-to {
  opacity: 0;
  transform: translateY(-3px) scale(0.98);
}
.context-menu-title {
  padding: 5px 10px 7px;
  overflow: hidden;
  color: var(--text-muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.context-menu-group {
  position: relative;
}
.context-menu-divider {
  height: 1px;
  margin: 5px 6px;
  background: var(--border-subtle);
}
.context-menu-item {
  display: flex;
  width: 100%;
  min-height: 34px;
  align-items: center;
  gap: 9px;
  padding: 0 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  text-align: left;
}
.context-menu-item:hover,
.context-menu-item:focus-visible {
  background: var(--control-hover-bg);
  color: var(--text-primary);
  outline: none;
}
.context-menu-item:disabled {
  opacity: 0.38;
  cursor: default;
}
.context-menu-item.is-danger {
  color: var(--el-color-danger);
}
.context-menu-icon {
  width: 16px;
  height: 16px;
  flex: 0 0 16px;
}
.context-menu-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.context-menu-shortcut,
.context-menu-chevron {
  flex: 0 0 auto;
  color: var(--text-muted);
  font-size: 11px;
}
.context-submenu {
  position: absolute;
  top: -6px;
  left: calc(100% + 4px);
  width: 210px;
  max-height: min(420px, calc(100vh - 24px));
  overflow: auto;
  padding: 6px;
  border: 1px solid var(--surface-border);
  border-radius: 11px;
  background: var(--surface-bg-elevated);
  box-shadow: 0 14px 36px rgba(0, 0, 0, 0.24);
}
.context-menu-group.opens-left .context-submenu {
  right: calc(100% + 4px);
  left: auto;
}
@media (prefers-reduced-motion: reduce) {
  .context-menu-fade-enter-active,
  .context-menu-fade-leave-active {
    transition-duration: 0.01ms;
  }
}
</style>
