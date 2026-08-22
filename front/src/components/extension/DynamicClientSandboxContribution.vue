<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { DynamicClientSandboxCode } from "@/ui-runtime/dynamicClientSandbox";

const props = withDefaults(defineProps<{
  clientCode: DynamicClientSandboxCode;
  title?: string;
  slotId?: string;
  pluginId?: string;
  contributionId?: string;
  context?: Record<string, unknown>;
}>(), {
  title: "Dynamic UI",
  slotId: "",
  pluginId: "",
  contributionId: "",
  context: () => ({}),
});

const iframeRef = ref<HTMLIFrameElement>();
const measuredHeight = ref(0);
const channelToken = cryptoToken();

const minHeight = computed(() => clampHeight(props.clientCode?.minHeight, 32, 1200, 96));
const maxHeight = computed(() => clampHeight(props.clientCode?.maxHeight, minHeight.value, 2400, 960));
const frameHeight = computed(() => Math.min(maxHeight.value, Math.max(minHeight.value, measuredHeight.value || minHeight.value)));

const srcdoc = computed(() => buildSandboxDocument({
  code: props.clientCode ?? {},
  token: channelToken,
  metadata: {
    title: props.title,
    slotId: props.slotId,
    pluginId: props.pluginId,
    contributionId: props.contributionId,
    context: sanitizeContext(props.context),
  },
}));

function onMessage(event: MessageEvent) {
  if (!iframeRef.value || event.source !== iframeRef.value.contentWindow) return;
  const data = event.data as { channel?: string; type?: string; height?: unknown } | null;
  if (!data || data.channel !== channelToken || data.type !== "resize") return;
  const height = Number(data.height);
  if (!Number.isFinite(height)) return;
  measuredHeight.value = Math.round(Math.min(maxHeight.value, Math.max(minHeight.value, height)));
}

onMounted(() => window.addEventListener("message", onMessage));
onBeforeUnmount(() => window.removeEventListener("message", onMessage));
watch(srcdoc, () => { measuredHeight.value = 0; });

function clampHeight(value: unknown, minimum: number, maximum: number, fallback: number): number {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return fallback;
  return Math.round(Math.min(maximum, Math.max(minimum, numeric)));
}

function cryptoToken(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID();
  const random = Math.random().toString(36).slice(2);
  return `amitia-sandbox-${Date.now().toString(36)}-${random}`;
}

function sanitizeContext(value: Record<string, unknown> | undefined): Record<string, unknown> {
  if (!value) return {};
  try {
    return JSON.parse(JSON.stringify(value)) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function htmlEscape(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapeStyle(value: string): string {
  return value.replace(/<\/style/gi, "<\\/style");
}

function escapeScript(value: string): string {
  return value.replace(/<\/script/gi, "<\\/script");
}

function jsonForInlineScript(value: unknown): string {
  return JSON.stringify(value)
    .replaceAll("<", "\\u003c")
    .replaceAll(">", "\\u003e")
    .replaceAll("&", "\\u0026")
    .replaceAll("\u2028", "\\u2028")
    .replaceAll("\u2029", "\\u2029");
}

function buildSandboxDocument(input: {
  code: DynamicClientSandboxCode;
  token: string;
  metadata: Record<string, unknown>;
}): string {
  const html = input.code.html?.slice(0, 65_536) ?? "";
  const css = escapeStyle(input.code.css?.slice(0, 65_536) ?? "");
  const script = escapeScript(input.code.script?.slice(0, 131_072) ?? "");
  const metadata = jsonForInlineScript(input.metadata);
  const token = jsonForInlineScript(input.token);
  const title = htmlEscape(String(input.metadata.title ?? "Dynamic UI"));
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; media-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; object-src 'none'; worker-src 'none'; base-uri 'none'; form-action 'none'">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>${title}</title>
<style>
:root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
html, body { margin: 0; padding: 0; width: 100%; min-width: 0; background: transparent; color: inherit; }
*, *::before, *::after { box-sizing: border-box; }
#amitia-root { width: 100%; min-width: 0; }
${css}
</style>
</head>
<body>
<div id="amitia-root">${html}</div>
<script>
(() => {
  'use strict';
  const channel = ${token};
  const metadata = ${metadata};
  Object.defineProperty(window, 'amitiaSandbox', {
    configurable: false,
    enumerable: true,
    writable: false,
    value: Object.freeze({
      metadata: Object.freeze(metadata),
      root: document.getElementById('amitia-root'),
      resize(height) {
        const numeric = Number(height);
        if (!Number.isFinite(numeric)) return;
        parent.postMessage({ channel, type: 'resize', height: numeric }, '*');
      },
    }),
  });
  const reportSize = () => {
    const height = Math.max(document.documentElement.scrollHeight, document.body.scrollHeight, document.getElementById('amitia-root')?.scrollHeight || 0);
    parent.postMessage({ channel, type: 'resize', height }, '*');
  };
  const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(reportSize) : null;
  if (observer) observer.observe(document.documentElement);
  addEventListener('load', reportSize, { once: true });
  queueMicrotask(reportSize);
})();
<\/script>
<script>
${script}
<\/script>
</body>
</html>`;
}
</script>

<template>
  <iframe
    ref="iframeRef"
    class="dynamic-client-sandbox"
    sandbox="allow-scripts"
    :title="title"
    :srcdoc="srcdoc"
    :style="{ height: `${frameHeight}px` }"
    referrerpolicy="no-referrer"
  />
</template>

<style scoped>
.dynamic-client-sandbox {
  display: block;
  width: 100%;
  min-width: 0;
  border: 0;
  background: transparent;
  overflow: hidden;
}
</style>
