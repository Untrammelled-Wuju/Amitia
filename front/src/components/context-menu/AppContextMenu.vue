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

        <template v-for="(item, index) in contextMenuState.items" :key="item.id">
          <div v-if="item.separatorBefore" class="context-menu-divider"></div>
          <div
            class="context-menu-group"
            :class="{ 'opens-left': submenuSide === 'left' }"
            @mouseenter="handleRootItemEnter(item, index, $event)"
            @mouseleave="scheduleCloseSubmenu(item)"
          >
            <button
              type="button"
              class="context-menu-item"
              :class="{ 'is-danger': item.danger, 'has-children': item.children?.length, 'is-active': activeRootIndex === index }"
              :disabled="item.disabled"
              :tabindex="activeLevel === 'root' && activeRootIndex === index ? 0 : -1"
              :data-context-root-index="index"
              :aria-haspopup="item.children?.length ? 'menu' : undefined"
              :aria-expanded="item.children?.length ? activeSubmenu === item.id : undefined"
              :aria-keyshortcuts="menuItemAriaShortcuts(item)"
              role="menuitem"
              @click="activateItem(item)"
            >
              <component :is="item.icon" v-if="item.icon" class="context-menu-icon" />
              <span class="context-menu-label">{{ item.label }}</span>
              <span class="context-menu-meta" aria-hidden="true">
                <span v-if="item.shortcut" class="context-menu-shortcut">{{ displayShortcut(item.shortcut) }}</span>
                <kbd v-if="item.accessKey" class="context-menu-access-key">{{ item.accessKey }}</kbd>
                <span v-if="item.children?.length" class="context-menu-chevron">›</span>
              </span>
            </button>
            <div
              v-if="item.children?.length && activeSubmenu === item.id"
              class="context-submenu"
              :style="{ maxHeight: `${submenuMaxHeight}px` }"
              role="menu"
            >
              <button
                v-for="(child, childIndex) in item.children"
                :key="child.id"
                type="button"
                class="context-menu-item"
                :class="{ 'is-danger': child.danger, 'is-active': activeLevel === 'submenu' && activeChildIndex === childIndex }"
                :disabled="child.disabled"
                :tabindex="activeLevel === 'submenu' && activeChildIndex === childIndex ? 0 : -1"
                :data-context-child-index="childIndex"
                :aria-keyshortcuts="menuItemAriaShortcuts(child)"
                role="menuitem"
                @mouseenter="handleChildItemEnter(childIndex)"
                @click="activateChildItem(child)"
              >
                <component :is="child.icon" v-if="child.icon" class="context-menu-icon" />
                <span class="context-menu-label">{{ child.label }}</span>
                <span class="context-menu-meta" aria-hidden="true">
                  <span v-if="child.shortcut" class="context-menu-shortcut">{{ displayShortcut(child.shortcut) }}</span>
                  <kbd v-if="child.accessKey" class="context-menu-access-key">{{ child.accessKey }}</kbd>
                </span>
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
const activeLevel = ref<"root" | "submenu">("root");
const activeRootIndex = ref(-1);
const activeChildIndex = ref(-1);
const submenuSide = ref<"left" | "right">("right");
const submenuMaxHeight = ref(420);
let submenuTimer: ReturnType<typeof setTimeout> | null = null;
let focusBeforeOpen: HTMLElement | null = null;
const theme = useTheme();
const isMacHost =
  typeof navigator !== "undefined" &&
  /Mac/i.test(`${navigator.platform} ${navigator.userAgent}`);

function displayShortcut(shortcut: string) {
  if (!isMacHost) return shortcut;
  return shortcut
    .replace(/\bCmdOrCtrl\+/g, "⌘")
    .replace(/\bCtrl\+/g, "⌘")
    .replace(/\bMeta\+/g, "⌘")
    .replace(/\bShift\+/g, "⇧")
    .replace(/\bAlt\+/g, "⌥");
}

function menuItemAriaShortcuts(item: ContextMenuAction) {
  const shortcut = item.shortcut?.replace(/\bCtrl\+/g, "Control+");
  return [item.accessKey, shortcut].filter(Boolean).join(" ") || undefined;
}

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
      accessKey: "C",
      action: () => copyText(selection, "已复制选中内容"),
    });
  }
  if (isEditable(target) || target?.closest("[contenteditable='true']")) {
    actions.push({
      id: "paste",
      label: "粘贴",
      icon: DocumentCopy,
      shortcut: "Ctrl+V",
      accessKey: "V",
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
        accessKey: "O",
        separatorBefore: actions.length > 0,
        action: () => {
          window.open(url, "_blank", "noopener,noreferrer");
        },
      },
      {
        id: "navigate-link",
        label: "转到链接",
        icon: Link,
        accessKey: "G",
        action: () => {
          window.open(url, "_self");
        },
      },
      {
        id: "copy-link",
        label: "复制链接地址",
        icon: Link,
        accessKey: "L",
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
        accessKey: "S",
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
        accessKey: "I",
        action: () => {
          window.open(url, "_blank", "noopener,noreferrer");
        },
      },
      {
        id: "copy-image-url",
        label: "复制图片地址",
        icon: Link,
        accessKey: "U",
        action: () => copyText(url, "图片地址已复制"),
      },
    );
  }

  actions.push({
    id: "toggle-theme",
    label: theme.resolvedMode.value === "dark" ? "切换为浅色模式" : "切换为深色模式",
    icon: Brush,
    accessKey: "T",
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

function openSubmenu(
  item: ContextMenuAction,
  index: number,
  group: HTMLElement | null,
  focusFirstChild = false,
) {
  if (!item.children?.length || item.disabled) return;
  if (submenuTimer) clearTimeout(submenuTimer);
  activeSubmenu.value = item.id;
  activeLevel.value = "submenu";
  activeRootIndex.value = index;
  activeChildIndex.value = -1;
  const menu = menuRef.value;
  if (!group || !menu) return;
  const groupRect = group.getBoundingClientRect();
  const menuRect = menu.getBoundingClientRect();
  submenuSide.value = menuRect.right + 218 > window.innerWidth - 8 ? "left" : "right";
  submenuMaxHeight.value = Math.max(120, Math.min(420, window.innerHeight - groupRect.top - 8));
  if (focusFirstChild) {
    void nextTick(() => {
      const firstEnabled = item.children?.findIndex((child) => !child.disabled) ?? -1;
      if (firstEnabled >= 0) focusChildItem(firstEnabled, index);
    });
  }
}

function handleRootItemEnter(item: ContextMenuAction, index: number, event: MouseEvent) {
  activeLevel.value = "root";
  activeRootIndex.value = index;
  activeChildIndex.value = -1;
  if (!item.children?.length || item.disabled) return;
  const group = event.currentTarget instanceof HTMLElement ? event.currentTarget : null;
  openSubmenu(item, index, group);
}

function handleChildItemEnter(index: number) {
  activeLevel.value = "submenu";
  activeChildIndex.value = index;
}

function scheduleCloseSubmenu(item: ContextMenuAction) {
  if (!item.children?.length) return;
  submenuTimer = setTimeout(() => {
    if (activeSubmenu.value !== item.id) return;
    activeSubmenu.value = "";
    activeLevel.value = "root";
    activeChildIndex.value = -1;
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
  if (item.disabled) return;
  if (item.children?.length) {
    const index = contextMenuState.items.findIndex((candidate) => candidate.id === item.id);
    const group = rootButton(index)?.parentElement || null;
    openSubmenu(item, index, group, true);
    return;
  }
  void runAction(item.action);
}

function activateChildItem(item: ContextMenuAction) {
  if (item.disabled) return;
  void runAction(item.action);
}

function rootButton(index: number) {
  return menuRef.value?.querySelector<HTMLButtonElement>(`[data-context-root-index="${index}"]`) || null;
}

function childButton(index: number) {
  return menuRef.value?.querySelector<HTMLButtonElement>(`[data-context-child-index="${index}"]`) || null;
}

function focusRootItem(index: number) {
  if (index < 0) return;
  activeLevel.value = "root";
  activeRootIndex.value = index;
  activeChildIndex.value = -1;
  rootButton(index)?.focus();
}

function focusChildItem(index: number, rootIndex = activeRootIndex.value) {
  if (index < 0) return;
  activeLevel.value = "submenu";
  activeRootIndex.value = rootIndex;
  activeChildIndex.value = index;
  childButton(index)?.focus();
}

function enabledIndexes(items: ContextMenuAction[]) {
  return items.flatMap((item, index) => (item.disabled ? [] : [index]));
}

function activeItems() {
  if (activeLevel.value === "submenu") {
    return contextMenuState.items.find((item) => item.id === activeSubmenu.value)?.children || [];
  }
  return contextMenuState.items;
}

function moveActiveFocus(direction: 1 | -1) {
  const items = activeItems();
  const indexes = enabledIndexes(items);
  if (!indexes.length) return;
  const current = activeLevel.value === "submenu" ? activeChildIndex.value : activeRootIndex.value;
  const position = indexes.indexOf(current);
  const nextPosition =
    position < 0
      ? direction > 0
        ? 0
        : indexes.length - 1
      : (position + direction + indexes.length) % indexes.length;
  if (activeLevel.value === "submenu") focusChildItem(indexes[nextPosition]);
  else focusRootItem(indexes[nextPosition]);
}

function moveActiveFocusToEdge(edge: "first" | "last") {
  const indexes = enabledIndexes(activeItems());
  if (!indexes.length) return;
  const index = edge === "first" ? indexes[0] : indexes[indexes.length - 1];
  if (activeLevel.value === "submenu") focusChildItem(index);
  else focusRootItem(index);
}

function openActiveSubmenu() {
  if (activeLevel.value !== "root") return false;
  const item = contextMenuState.items[activeRootIndex.value];
  if (!item?.children?.length || item.disabled) return false;
  openSubmenu(item, activeRootIndex.value, rootButton(activeRootIndex.value)?.parentElement || null, true);
  return true;
}

function closeActiveSubmenu(restoreRootFocus: boolean) {
  const rootIndex = activeRootIndex.value;
  activeSubmenu.value = "";
  activeLevel.value = "root";
  activeChildIndex.value = -1;
  if (restoreRootFocus) focusRootItem(rootIndex);
}

function activateSelectedItem() {
  const item =
    activeLevel.value === "submenu"
      ? contextMenuState.items.find((candidate) => candidate.id === activeSubmenu.value)?.children?.[activeChildIndex.value]
      : contextMenuState.items[activeRootIndex.value];
  if (!item || item.disabled) return;
  if (activeLevel.value === "root" && item.children?.length) {
    openActiveSubmenu();
    return;
  }
  void runAction(item.action);
}

function activateByAccessKey(key: string) {
  const normalizedKey = key.toUpperCase();
  const item = activeItems().find((candidate) => candidate.accessKey === normalizedKey);
  if (!item) return false;
  if (item.disabled) return true;
  if (activeLevel.value === "root" && item.children?.length) {
    const index = contextMenuState.items.findIndex((candidate) => candidate.id === item.id);
    openSubmenu(item, index, rootButton(index)?.parentElement || null, true);
    return true;
  }
  void runAction(item.action);
  return true;
}

function dismissMenu(restoreFocus: boolean) {
  const focusTarget = focusBeforeOpen;
  focusBeforeOpen = null;
  closeContextMenu();
  if (restoreFocus && focusTarget?.isConnected) {
    void nextTick(() => focusTarget.focus());
  }
}

function handleMenuKeydown(event: KeyboardEvent) {
  if (!contextMenuState.visible || event.isComposing) return;
  let handled = false;
  if (event.key === "Escape") {
    if (activeSubmenu.value) closeActiveSubmenu(true);
    else dismissMenu(true);
    handled = true;
  } else if (event.key === "ArrowDown") {
    moveActiveFocus(1);
    handled = true;
  } else if (event.key === "ArrowUp") {
    moveActiveFocus(-1);
    handled = true;
  } else if (event.key === "Home") {
    moveActiveFocusToEdge("first");
    handled = true;
  } else if (event.key === "End") {
    moveActiveFocusToEdge("last");
    handled = true;
  } else if (event.key === "ArrowRight") {
    handled = activeSubmenu.value ? true : openActiveSubmenu();
  } else if (event.key === "ArrowLeft") {
    if (activeSubmenu.value) {
      closeActiveSubmenu(true);
      handled = true;
    }
  } else if (event.key === "Enter" || event.key === " ") {
    activateSelectedItem();
    handled = true;
  } else if (
    event.key.length === 1 &&
    /^[a-z0-9]$/i.test(event.key) &&
    !event.ctrlKey &&
    !event.metaKey &&
    !event.altKey
  ) {
    handled = activateByAccessKey(event.key);
  }
  if (handled) {
    event.preventDefault();
    event.stopPropagation();
  }
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
  dismissMenu(false);
}

function closeOnViewportChange() {
  dismissMenu(false);
}

watch(
  () => contextMenuState.visible,
  async (visible) => {
    activeSubmenu.value = "";
    activeLevel.value = "root";
    activeRootIndex.value = -1;
    activeChildIndex.value = -1;
    if (!visible) return;
    focusBeforeOpen = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    await nextTick();
    clampPosition();
    const firstEnabled = enabledIndexes(contextMenuState.items)[0];
    if (firstEnabled !== undefined) focusRootItem(firstEnabled);
  },
);

onMounted(() => {
  document.addEventListener("contextmenu", handleContextMenu);
  document.addEventListener("mousedown", closeOnOutside);
  document.addEventListener("click", closeOnOutside);
  document.addEventListener("wheel", closeOnViewportChange, { passive: true });
  window.addEventListener("keydown", handleMenuKeydown, true);
  window.addEventListener("blur", closeOnViewportChange);
  window.addEventListener("resize", closeOnViewportChange);
});

onUnmounted(() => {
  if (submenuTimer) clearTimeout(submenuTimer);
  document.removeEventListener("contextmenu", handleContextMenu);
  document.removeEventListener("mousedown", closeOnOutside);
  document.removeEventListener("click", closeOnOutside);
  document.removeEventListener("wheel", closeOnViewportChange);
  window.removeEventListener("keydown", handleMenuKeydown, true);
  window.removeEventListener("blur", closeOnViewportChange);
  window.removeEventListener("resize", closeOnViewportChange);
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
.context-menu-item.is-active,
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
.context-menu-meta {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 5px;
}
.context-menu-shortcut,
.context-menu-chevron {
  flex: 0 0 auto;
  color: var(--text-muted);
  font-size: 11px;
}
.context-menu-access-key {
  display: inline-grid;
  min-width: 18px;
  height: 18px;
  place-items: center;
  padding: 0 4px;
  border: 1px solid var(--border-subtle);
  border-radius: 5px;
  background: color-mix(in srgb, var(--surface-bg-elevated) 72%, var(--control-hover-bg));
  color: var(--text-muted);
  font: inherit;
  font-size: 10px;
  line-height: 1;
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
