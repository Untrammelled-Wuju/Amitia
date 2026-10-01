<template>
  <div class="amrp-audio">
    <div class="amrp-media-head">
      <span>{{ block.title || "语音" }}</span>
      <span>{{ formatDuration(block.duration) }}</span>
    </div>
    <div v-if="status === 'failed'" class="amrp-media-error">
      <span>{{ errorMessage || "音频加载失败" }}</span>
      <button type="button" @click="retry">重试</button>
    </div>
    <div v-else class="amrp-audio-line">
      <audio
        v-if="resolvedUrl"
        ref="audioEl"
        :src="resolvedUrl"
        controls
        preload="metadata"
        @loadedmetadata="status = 'ready'"
        @error="onError"
      ></audio>
      <span v-else class="amrp-audio-loading">加载中</span>
      <button type="button" @click="cycleSpeed">{{ speed }}×</button>
      <button type="button" :disabled="downloading" @click="download">
        {{ downloading ? "下载中" : "下载" }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { AudioBlock } from "../types";
import { formatDuration } from "../utils";
import {
  downloadConversationMedia,
  useResolvedConversationMediaUrl,
} from "../media";

const props = defineProps<{
  block: AudioBlock;
}>();

const audioEl = ref<HTMLAudioElement>();
const errorMessage = ref("");
const status = ref(props.block.status ?? "loading");
const speeds = [1, 1.25, 1.5, 2];
const speed = ref(1);
const downloading = ref(false);
const { resolved: resolvedUrl, error: resolveError } =
  useResolvedConversationMediaUrl(computed(() => props.block.url));

watch(
  () => props.block.status,
  (value) => {
    if (value) status.value = value;
  },
);

watch(resolveError, (value) => {
  if (!value) return;
  status.value = "failed";
  errorMessage.value = value;
});

function cycleSpeed() {
  const index = speeds.indexOf(speed.value);
  speed.value = speeds[(index + 1) % speeds.length];
  if (audioEl.value) audioEl.value.playbackRate = speed.value;
}

function onError() {
  status.value = "failed";
  errorMessage.value = "音频无法播放";
}

function retry() {
  status.value = "loading";
  errorMessage.value = "";
  if (audioEl.value) audioEl.value.load();
}

async function download() {
  if (downloading.value) return;
  downloading.value = true;
  try {
    const saved = await downloadConversationMedia(
      props.block.downloadUrl || props.block.url,
      props.block.title || "audio",
    );
    if (saved) ElMessage.success("语音已保存");
  } catch (reason) {
    ElMessage.error(reason instanceof Error ? reason.message : "语音保存失败");
  } finally {
    downloading.value = false;
  }
}
</script>

<style scoped>
.amrp-audio {
  max-width: 460px;
  margin: 10px 0;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  padding: 9px 11px;
  background: var(--amrp-surface);
}

.amrp-media-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 7px;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-audio-line {
  display: flex;
  align-items: center;
  gap: 7px;
}

audio,
.amrp-audio-loading {
  min-width: 0;
  flex: 1;
  height: 36px;
}

.amrp-audio-loading {
  display: inline-flex;
  align-items: center;
  color: var(--amrp-muted);
  font-size: 10.5px;
}

button,
a {
  border: 0;
  border-radius: 6px;
  padding: 5px 7px;
  background: var(--amrp-soft);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  text-decoration: none;
  cursor: pointer;
}

button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.amrp-media-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--amrp-danger);
  font-size: 11px;
}
</style>
