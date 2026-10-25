<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from "vue";

interface WebviewElement extends HTMLElement {
  loadURL(url: string): Promise<void>;
  getURL(): string;
  canGoBack(): boolean;
  canGoForward(): boolean;
  goBack(): void;
  goForward(): void;
  reload(): void;
}

const props = defineProps<{
  providerId: string;
  extensionId: string;
  title: string;
  url: string;
  allowedOrigins: string[];
  storageKey: string;
  persistent?: boolean;
}>();

const host = ref<HTMLElement | null>(null);
const webview = ref<WebviewElement | null>(null);
const lease = ref("");
const loading = ref(true);
const error = ref("");
const currentURL = ref(props.url);
const canGoBack = ref(false);
const canGoForward = ref(false);
const clearing = ref(false);
let disposed = false;

function syncNavigationState() {
  const element = webview.value;
  if (!element) return;
  try {
    currentURL.value = element.getURL() || currentURL.value;
    canGoBack.value = element.canGoBack();
    canGoForward.value = element.canGoForward();
  } catch {
  }
}

function externalOpen() {
  const target = currentURL.value || props.url;
  if (/^https?:\/\//i.test(target)) window.open(target, "_blank", "noopener,noreferrer");
}

function goBack() {
  if (canGoBack.value) webview.value?.goBack();
}

function goForward() {
  if (canGoForward.value) webview.value?.goForward();
}

function reload() {
  if (webview.value) {
    webview.value.reload();
    return;
  }
  if (lease.value) void mountGuest();
}

function acquireRequest() {
  return {
    providerId: String(props.providerId),
    extensionId: String(props.extensionId),
    url: String(props.url),
    allowedOrigins: props.allowedOrigins.map(String),
    storageKey: String(props.storageKey),
    persistent: props.persistent === true,
  };
}

async function clearStorage() {
  if (clearing.value || !window.confirm("确定清除此外部 WebUI 的登录状态和站点数据吗？")) return;
  const desktop = window.amitiaDesktop;
  if (!desktop?.clearExternalWebUI) return;
  clearing.value = true;
  error.value = "";
  try {
    await desktop.clearExternalWebUI(acquireRequest());
    reload();
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : String(reason);
  } finally {
    clearing.value = false;
  }
}

async function mountGuest() {
  if (disposed || !host.value) return;
  const desktop = window.amitiaDesktop;
  if (!desktop?.acquireExternalWebUI) {
    loading.value = false;
    error.value = "当前客户端不支持嵌入外部 WebUI";
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const reservation = await desktop.acquireExternalWebUI(acquireRequest());
    if (disposed || !host.value) {
      await desktop.releaseExternalWebUI?.(reservation.lease);
      return;
    }
    lease.value = reservation.lease;
    const element = document.createElement("webview") as WebviewElement;
    element.className = "external-web-ui__guest";
    element.setAttribute("partition", reservation.partition);
    element.setAttribute("src", `about:blank#${reservation.lease}`);
    element.style.width = "100%";
    element.style.height = "100%";
    element.style.display = "flex";
    element.addEventListener("dom-ready", () => {
      currentURL.value = props.url;
      void element.loadURL(props.url).catch((reason: unknown) => {
        loading.value = false;
        error.value = reason instanceof Error ? reason.message : String(reason);
      });
    }, { once: true });
    element.addEventListener("did-start-loading", () => {
      loading.value = true;
      error.value = "";
    });
    element.addEventListener("did-stop-loading", () => {
      loading.value = false;
      syncNavigationState();
    });
    element.addEventListener("did-navigate", syncNavigationState);
    element.addEventListener("did-navigate-in-page", syncNavigationState);
    element.addEventListener("did-fail-load", (event) => {
      const failure = event as Event & { errorCode?: number; errorDescription?: string; isMainFrame?: boolean };
      if (failure.isMainFrame === false || failure.errorCode === -3) return;
      loading.value = false;
      error.value = failure.errorDescription || "外部 WebUI 加载失败";
    });
    element.addEventListener("render-process-gone", () => {
      loading.value = false;
      error.value = "外部 WebUI 进程已退出";
    });
    host.value.replaceChildren(element);
    webview.value = element;
    await nextTick();
  } catch (reason) {
    loading.value = false;
    error.value = reason instanceof Error ? reason.message : String(reason);
  }
}

function releaseGuest() {
  const currentLease = lease.value;
  lease.value = "";
  webview.value = null;
  if (host.value) host.value.replaceChildren();
  if (currentLease) void window.amitiaDesktop?.releaseExternalWebUI?.(currentLease).catch(() => undefined);
}

onMounted(() => {
  void mountGuest();
});

onBeforeUnmount(() => {
  disposed = true;
  releaseGuest();
});
</script>

<template>
  <section class="external-web-ui" data-amitia-ui="external.web-ui">
    <header class="external-web-ui__toolbar">
      <button type="button" aria-label="后退" :disabled="!canGoBack" @click="goBack">←</button>
      <button type="button" aria-label="前进" :disabled="!canGoForward" @click="goForward">→</button>
      <button type="button" aria-label="刷新" @click="reload">刷新</button>
      <span class="external-web-ui__title">{{ title }}</span>
      <span v-if="loading" class="external-web-ui__status">加载中</span>
      <button v-if="persistent" type="button" :disabled="clearing" @click="clearStorage">
        {{ clearing ? "清理中" : "清除数据" }}
      </button>
      <button type="button" aria-label="在浏览器中打开" @click="externalOpen">外部打开</button>
    </header>
    <div v-if="error" class="external-web-ui__error" role="alert">
      <span>{{ error }}</span>
      <button type="button" @click="reload">重试</button>
    </div>
    <div ref="host" class="external-web-ui__guest-host" aria-label="外部 WebUI" />
  </section>
</template>

<style scoped>
.external-web-ui {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--workbench-bg);
}
.external-web-ui__toolbar {
  flex: 0 0 auto;
  min-height: 42px;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--surface-border);
}
.external-web-ui__toolbar button,
.external-web-ui__error button {
  padding: 5px 10px;
  border: 1px solid var(--surface-border);
  border-radius: 6px;
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
}
.external-web-ui__toolbar button:disabled {
  cursor: default;
  opacity: 0.4;
}
.external-web-ui__title {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
.external-web-ui__status {
  color: var(--text-secondary);
  font-size: var(--ac-font-size-sm);
}
.external-web-ui__error {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  color: rgb(190 55 55);
  background: rgba(220 60 60 / 8%);
}
.external-web-ui__guest-host {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}
.external-web-ui__guest-host :deep(.external-web-ui__guest) {
  flex: 1;
  min-width: 0;
  min-height: 0;
}
</style>
