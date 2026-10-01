<template>
  <div class="amrp-video">
    <div class="amrp-media-head">
      <span>{{ block.title || "视频" }}</span>
      <span>{{ formatDuration(block.duration) }}</span>
      <button type="button" :disabled="downloading" @click="download">
        {{ downloading ? "下载中" : "下载" }}
      </button>
    </div>
    <div v-if="status === 'failed'" class="amrp-media-error">
      <span>{{ errorMessage || "视频加载失败" }}</span>
      <button type="button" @click="retry">重试</button>
    </div>
    <video
      v-else-if="resolvedUrl"
      ref="videoEl"
      :src="resolvedUrl"
      :poster="block.poster"
      controls
      playsinline
      preload="metadata"
      @loadedmetadata="status = 'ready'"
      @error="onError"
    ></video>
    <div v-else class="amrp-media-loading">加载中</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { VideoBlock } from "../types";
import { formatDuration } from "../utils";
import {
  downloadConversationMedia,
  useResolvedConversationMediaUrl,
} from "../media";

const props = defineProps<{
  block: VideoBlock;
}>();

const videoEl = ref<HTMLVideoElement>();
const errorMessage = ref("");
const status = ref(props.block.status ?? "loading");
const downloading = ref(false);
const { resolved: resolvedUrl, error: resolveError } =
  useResolvedConversationMediaUrl(computed(() => props.block.url));

watch(
  () => props.block.status,
  (value) => {
    if (value) status.value = value;
  },
);

function onError() {
  status.value = "failed";
  errorMessage.value = "视频无法播放";
}

function retry() {
  status.value = "loading";
  errorMessage.value = "";
  videoEl.value?.load();
}

async function download() {
  if (downloading.value) return;
  downloading.value = true;
  try {
    const saved = await downloadConversationMedia(
      props.block.downloadUrl || props.block.url,
      props.block.title || "video",
    );
    if (saved) ElMessage.success("视频已保存");
  } catch (reason) {
    ElMessage.error(reason instanceof Error ? reason.message : "视频保存失败");
  } finally {
    downloading.value = false;
  }
}

watch(resolveError, (value) => {
  if (!value) return;
  status.value = "failed";
  errorMessage.value = value;
});
</script>

<style scoped>
.amrp-video {
  width: 100%;
  max-width: 700px;
  margin: 12px 0 16px;
  overflow: hidden;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  background: var(--amrp-surface);
}

.amrp-media-head {
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 10px;
  border-bottom: 1px solid var(--amrp-line);
  background: var(--amrp-soft);
  font-size: 11px;
}

.amrp-media-head span:nth-child(2) {
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-media-head button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-accent);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-media-head button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.amrp-media-loading {
  min-height: 180px;
  display: grid;
  place-items: center;
  color: var(--amrp-muted);
  font-size: 11px;
}

video {
  display: block;
  width: 100%;
  max-height: 420px;
  background: #111216;
}

.amrp-media-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  color: var(--amrp-danger);
  font-size: 11px;
}

.amrp-media-error button {
  border: 0;
  border-radius: 6px;
  padding: 5px 8px;
  background: var(--amrp-soft);
  color: var(--amrp-accent);
  cursor: pointer;
}
</style>
