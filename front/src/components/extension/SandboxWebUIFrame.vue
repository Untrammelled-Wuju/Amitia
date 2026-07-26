<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";

const props = defineProps<{
  contribution: UIContributionSummary;
  context?: Record<string, unknown>;
  slotId: string;
}>();

const iframeRef = ref<HTMLIFrameElement | null>(null);
const sessionId = ref<string>("");
const loading = ref(true);
const error = ref<string | null>(null);
const entryUrl = ref<string>("");

async function createSession() {
  loading.value = true;
  error.value = null;
  try {
    const response = await fetch(`/api/extension/webui/session`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        contributionId: props.contribution.contributionId,
        extensionId: props.contribution.extensionId,
        moduleId: props.contribution.moduleId,
        slotId: props.slotId,
        generation: props.contribution.generation,
        sandbox: props.contribution.sandbox ?? "web_restricted",
        entryPath: props.contribution.entryPath,
      }),
    });
    if (!response.ok) throw new Error(`session create failed: ${response.status}`);
    const data = await response.json();
    sessionId.value = data.sessionId;
    entryUrl.value = data.entryUrl;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

async function destroySession() {
  if (!sessionId.value) return;
  try {
    await fetch(`/api/extension/webui/session/${sessionId.value}`, {
      method: "DELETE",
    });
  } catch {
  }
  sessionId.value = "";
}

function onMessage(event: MessageEvent) {
  if (!event.origin || event.origin !== new URL(entryUrl.value).origin) return;
  const data = event.data;
  if (!data || typeof data !== "object") return;
  if (data.session !== sessionId.value) return;
  if (data.nonce !== sessionId.value) return;
  const origin = event.origin;
  void handleBridgeMessage(data, origin);
}

async function handleBridgeMessage(msg: Record<string, unknown>, origin: string) {
  try {
    const response = await fetch(`/api/extension/webui/bridge/${sessionId.value}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(msg),
    });
    const result = await response.json();
    if (iframeRef.value?.contentWindow) {
      iframeRef.value.contentWindow.postMessage(
        {
          method: msg.method,
          id: msg.id,
          output: result.output,
          error: result.error,
        },
        origin
      );
    }
  } catch (e) {
    if (iframeRef.value?.contentWindow) {
      iframeRef.value.contentWindow.postMessage(
        {
          method: msg.method,
          id: msg.id,
          error: e instanceof Error ? e.message : String(e),
        },
        origin
      );
    }
  }
}

onMounted(async () => {
  await createSession();
  window.addEventListener("message", onMessage);
});

onBeforeUnmount(() => {
  window.removeEventListener("message", onMessage);
  void destroySession();
});

watch(() => props.contribution.contributionId, async () => {
  await destroySession();
  await createSession();
});
</script>

<template>
  <div class="sandbox-webui-frame" :data-contribution-id="contribution.contributionId">
    <template v-if="loading">
      <div class="sandbox-webui-frame__loading">加载扩展界面...</div>
    </template>
    <template v-else-if="error">
      <div class="sandbox-webui-frame__error">{{ error }}</div>
    </template>
    <template v-else>
      <iframe
        ref="iframeRef"
        :src="entryUrl"
        class="sandbox-webui-frame__iframe"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
        :data-session-id="sessionId"
      ></iframe>
    </template>
  </div>
</template>

<style scoped>
.sandbox-webui-frame {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.sandbox-webui-frame__iframe {
  width: 100%;
  min-height: 240px;
  border: none;
  border-radius: 6px;
  background: transparent;
}
.sandbox-webui-frame__loading,
.sandbox-webui-frame__error {
  padding: 12px;
  font-size: 12px;
}
.sandbox-webui-frame__error {
  color: rgb(180, 40, 40);
}
</style>
