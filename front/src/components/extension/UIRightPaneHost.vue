<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, watch } from "vue";
import ExtensionSlot from "./ExtensionSlot.vue";
import { rightPaneRuntime } from "@/ui-runtime/rightPaneRuntime";

const snapshot = shallowRef(rightPaneRuntime.getSnapshot());
const stop = rightPaneRuntime.subscribe(() => {
  snapshot.value = rightPaneRuntime.getSnapshot();
});
onScopeDispose(stop);
const menuOpen = ref(false);
const pinnedGuideEntries = ref(new Set<string>());
watch(snapshot, () => {
  menuOpen.value = false;
});

function isGuideAddress(address: string | undefined): boolean {
  return Boolean(address?.startsWith("dsh-resource://guide/") || address?.startsWith("amitia://guide/"));
}

function isFilesAddress(address: string | undefined): boolean {
  return Boolean(address?.startsWith("dsh-resource://file/") || address?.startsWith("amitia://workspace/") || address?.startsWith("file://"));
}

const guideEntries = computed<Array<Record<string, unknown>>>(() => {
  const configured = snapshot.value.params?.guideEntries;
  if (Array.isArray(configured)) {
    return configured.filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === "object").map((entry) => ({ ...entry }));
  }
  return [{
    entryId: "files",
    kind: "files",
    title: "文件",
    description: "浏览当前会话工作区",
  }];
});
const visibleGuideActions = computed(() => guideEntries.value.filter((entry) => entry.kind !== "files"));
const guide = computed(() => isGuideAddress(snapshot.value.address) || snapshot.value.params?.tabKind === "guide");
const files = computed(() => isFilesAddress(snapshot.value.address) || snapshot.value.params?.tabKind === "files");
const absolutePath = computed(() => String(snapshot.value.params?.absolutePath || snapshot.value.address || ""));
const sessionId = computed(() => snapshot.value.sessionId);
const tabMenuOwner = computed(() => ({
  tab: {
    id: snapshot.value.address || "",
    address: snapshot.value.address || "",
    title: snapshot.value.title || "",
    kind: String(snapshot.value.params?.tabKind || (guide.value ? "guide" : files.value ? "files" : "resource")),
  },
  dismiss: dismissMenu,
}));

function dismissMenu() {
  menuOpen.value = false;
}

function openGuideEntry(entry: Record<string, unknown>) {
  const entryId = String(entry.entryId || entry.id || entry.kind || "").trim();
  if (!entryId) throw new Error("right pane guide entry id is required");
  rightPaneRuntime.open(`dsh-resource://guide/${encodeURIComponent(entryId)}`, {
    title: String(entry.title || entryId),
    sessionId: sessionId.value,
    params: {
      ...snapshot.value.params,
      tabKind: "guide",
      guideEntryId: entryId,
    },
  });
  dismissMenu();
}

function changeGuidePinned(entryId: string, pinned: boolean) {
  const next = new Set(pinnedGuideEntries.value);
  if (pinned) next.add(entryId);
  else next.delete(entryId);
  pinnedGuideEntries.value = next;
}

function reloadFiles() {
  window.dispatchEvent(new CustomEvent("amitia:right-pane:reload", {
    detail: {
      address: snapshot.value.address || "",
      absolutePath: absolutePath.value,
      sessionId: sessionId.value,
    },
  }));
}

const context = computed(() => ({
  ...(snapshot.value.address ? { address: snapshot.value.address } : {}),
  ...(snapshot.value.params ? { params: snapshot.value.params } : {}),
  ...(snapshot.value.sessionId ? { sessionId: snapshot.value.sessionId } : {}),
  rightPane: snapshot.value,
}));
const filesActionsContext = computed(() => ({
  ...context.value,
  absolutePath: absolutePath.value,
}));
const guideActionsContext = computed(() => ({
  ...context.value,
  entries: visibleGuideActions.value,
  onOpen: (entryId: string) => {
    const entry = guideEntries.value.find((item) => String(item.entryId || item.id || item.kind || "") === entryId);
    if (!entry) throw new Error(`right pane guide entry ${entryId} does not exist`);
    openGuideEntry(entry);
  },
  onPinnedChange: changeGuidePinned,
  onDismiss: dismissMenu,
}));
const rightbarContext = computed(() => ({
  ...context.value,
  width: Math.min(520, Math.max(0, (globalThis.innerWidth || 0) - 24)),
  viewportWidth: globalThis.innerWidth || 0,
  canShow: (globalThis.innerWidth || 0) >= 720,
}));
const rightbarSessionContext = computed(() => ({
  ...rightbarContext.value,
  ...tabMenuOwner.value.tab,
}));
</script>

<template>
  <aside v-if="snapshot.open" class="amitia-right-pane" data-amitia-ui="right-pane">
    <ExtensionSlot
      slot-id="app.rightbar"
      :context="rightbarContext"
      fallback="default"
      layout="panel"
      surface-role="main"
      bare
    >
      <div class="amitia-right-pane__frame">
        <ExtensionSlot
          slot-id="app.rightbar.session"
          :context="rightbarSessionContext"
          fallback="default"
          layout="panel"
          surface-role="main"
          bare
        >
          <header class="amitia-right-pane__header">
            <div class="amitia-right-pane__title">
              <ExtensionSlot
                slot-id="ui.sidebar.right.pane.tab.title"
                :context="context"
                fallback="default"
                layout="inline"
                surface-role="header"
                bare
              >
                <span>{{ snapshot.title || "资源" }}</span>
              </ExtensionSlot>
            </div>
            <div class="amitia-right-pane__actions">
              <template v-if="files">
                <button type="button" class="amitia-right-pane__button" aria-label="刷新文件目录" @click="reloadFiles">刷新</button>
                <ExtensionSlot
                  slot-id="ui.sidebar.right.pane.files.actions"
                  :context="filesActionsContext"
                  fallback="none"
                  layout="inline"
                  surface-role="header"
                  bare
                />
              </template>
              <button type="button" class="amitia-right-pane__button" aria-label="打开标签页操作菜单" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen">⋯</button>
              <div v-if="menuOpen" class="amitia-right-pane__menu" role="menu">
                <ExtensionSlot
                  v-if="guide"
                  slot-id="ui.sidebar.right.pane.guide.actions"
                  :context="guideActionsContext"
                  fallback="none"
                  layout="inline"
                  surface-role="main"
                  bare
                />
                <ExtensionSlot
                  slot-id="ui.sidebar.right.pane.tab.menu.item"
                  :context="tabMenuOwner"
                  fallback="none"
                  layout="inline"
                  surface-role="main"
                  bare
                />
              </div>
              <button type="button" class="amitia-right-pane__close" aria-label="关闭右侧面板" @click="rightPaneRuntime.close()">×</button>
            </div>
          </header>
          <div class="amitia-right-pane__body">
            <ExtensionSlot
              v-if="guide"
              slot-id="ui.sidebar.right.pane.guide"
              :context="context"
              fallback="default"
              layout="stack"
              surface-role="main"
              bare
            >
              <div class="amitia-right-pane__guide">
                <ExtensionSlot
                  v-for="entry in guideEntries"
                  :key="String(entry.entryId || entry.id || entry.kind)"
                  slot-id="ui.sidebar.right.pane.guide.entry"
                  :context="{
                    ...context,
                    entryId: String(entry.entryId || entry.id || entry.kind || ''),
                    kind: String(entry.kind || ''),
                    title: String(entry.title || entry.entryId || entry.id || ''),
                    description: entry.description == null ? undefined : String(entry.description),
                  }"
                  :dispatch-key="String(entry.entryId || entry.id || entry.kind || '')"
                  fallback="default"
                  layout="stack"
                  surface-role="main"
                  bare
                >
                  <button type="button" class="amitia-right-pane__guide-card" @click="openGuideEntry(entry)">
                    <strong>{{ entry.title || entry.entryId || entry.id || entry.kind }}</strong>
                    <span v-if="entry.description">{{ entry.description }}</span>
                  </button>
                </ExtensionSlot>
              </div>
            </ExtensionSlot>
            <ExtensionSlot
              v-else
              slot-id="ui.sidebar.right.pane.tab"
              :context="context"
              fallback="none"
              layout="stack"
              surface-role="sidebar"
              bare
            />
          </div>
        </ExtensionSlot>
      </div>
    </ExtensionSlot>
  </aside>
</template>

<style scoped>
.amitia-right-pane {
  position: fixed;
  top: 48px;
  right: 12px;
  bottom: 12px;
  z-index: 30;
  display: flex;
  width: min(520px, calc(100vw - 24px));
  min-width: 320px;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--surface-border);
  border-radius: 12px;
  background: var(--el-bg-color);
  box-shadow: 0 20px 60px rgb(0 0 0 / 18%);
}

.amitia-right-pane__frame,
.amitia-right-pane__frame > :deep(.extension-slot),
.amitia-right-pane__frame > :deep(.extension-slot) > div {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
}

.amitia-right-pane__header {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  min-height: 44px;
  padding: 0 10px 0 14px;
  border-bottom: 1px solid var(--surface-border);
}

.amitia-right-pane__title {
  min-width: 0;
  overflow: hidden;
  color: var(--el-text-color-primary);
  font-size: 13px;
  font-weight: 600;
}

.amitia-right-pane__actions {
  position: relative;
  display: flex;
  flex: none;
  align-items: center;
  gap: 3px;
}

.amitia-right-pane__button,
.amitia-right-pane__close {
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--el-text-color-secondary);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
}

.amitia-right-pane__button {
  width: auto;
  min-width: 28px;
  padding: 0 8px;
  font-size: 13px;
}

.amitia-right-pane__button:hover,
.amitia-right-pane__close:hover {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.amitia-right-pane__menu {
  position: absolute;
  top: 34px;
  right: 32px;
  z-index: 2;
  min-width: 180px;
  padding: 6px;
  border: 1px solid var(--surface-border);
  border-radius: 8px;
  background: var(--el-bg-color-overlay, var(--el-bg-color));
  box-shadow: 0 12px 36px rgb(0 0 0 / 18%);
}

.amitia-right-pane__guide {
  display: grid;
  gap: 10px;
  padding: 14px;
}

.amitia-right-pane__guide-card {
  display: grid;
  width: 100%;
  gap: 5px;
  padding: 14px;
  border: 1px solid var(--surface-border);
  border-radius: 10px;
  background: var(--el-fill-color-blank);
  color: var(--el-text-color-primary);
  cursor: pointer;
  text-align: left;
}

.amitia-right-pane__guide-card span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.amitia-right-pane__body {
  min-height: 0;
  flex: 1;
  overflow: auto;
}
</style>
