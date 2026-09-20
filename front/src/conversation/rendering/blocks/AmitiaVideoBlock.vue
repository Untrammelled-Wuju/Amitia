<template>
  <div class="amrp-video">
    <div class="amrp-media-head">
      <span>{{ block.title || "视频" }}</span>
      <span>{{ formatDuration(block.duration) }}</span>
    </div>
    <div v-if="status === 'failed'" class="amrp-media-error">
      <span>{{ errorMessage || "视频加载失败" }}</span>
      <button type="button" @click="retry">重试</button>
    </div>
    <video
      v-else
      ref="videoEl"
      :src="block.url"
      :poster="block.poster"
      controls
      playsinline
      preload="metadata"
      @loadedmetadata="status = 'ready'"
      @error="onError"
    ></video>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import type { VideoBlock } from "../types";
import { formatDuration } from "../utils";

const props = defineProps<{
  block: VideoBlock;
}>();

const videoEl = ref<HTMLVideoElement>();
const errorMessage = ref("");
const status = ref(props.block.status ?? "loading");

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

.amrp-media-head span:last-child {
  color: var(--amrp-muted);
  font-size: 10px;
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
